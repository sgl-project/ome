package placement

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clocktesting "k8s.io/utils/clock/testing"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/irstatus"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workloadcluster"
)

var graceTestNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestMemberReadRetryBackoffDoublesUpToTheMaximum(t *testing.T) {
	policy := MemberReadRetry{MaxAttempts: 7, InitialBackoff: 100 * time.Millisecond, MaxBackoff: time.Second}
	var got []time.Duration
	for n := 1; n <= 6; n++ {
		got = append(got, policy.backoff(n))
	}
	assert.Equal(t, []time.Duration{
		100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond,
		800 * time.Millisecond, time.Second, time.Second,
	}, got)
}

func TestRetryMemberRead(t *testing.T) {
	transient := apierrors.NewServiceUnavailable("member apiserver unavailable")
	retrying := MemberReadRetry{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	for _, tt := range []struct {
		name     string
		policy   MemberReadRetry
		failures []error
		wantErr  error
		attempts int
	}{
		{name: "a transient failure is read again", policy: retrying, failures: []error{transient}, attempts: 2},
		{name: "attempts are bounded", policy: retrying, failures: []error{transient, transient, transient, transient}, wantErr: transient, attempts: 3},
		{name: "zero attempts reads once", failures: []error{transient}, wantErr: transient, attempts: 1},
		{
			name: "NotFound is conclusive", policy: retrying,
			failures: []error{apierrors.NewNotFound(schema.GroupResource{Resource: "inferenceservices"}, "service")},
			wantErr:  apierrors.NewNotFound(schema.GroupResource{Resource: "inferenceservices"}, "service"), attempts: 1,
		},
		{
			name: "an identity failure is conclusive", policy: retrying,
			failures: []error{errors.New("member service ownership is unverified")},
			wantErr:  errors.New("member service ownership is unverified"), attempts: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{MemberReadRetry: tt.policy}
			attempts := 0
			got, err := retryMemberRead(t.Context(), r, "member-a", func(context.Context) (int, error) {
				attempts++
				if attempts <= len(tt.failures) {
					return attempts, tt.failures[attempts-1]
				}
				return attempts, nil
			})
			assert.Equal(t, tt.attempts, attempts)
			assert.Equal(t, tt.attempts, got, "the last attempt's result is returned")
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr.Error())
			}
		})
	}
}

func TestRetryMemberReadGivesEveryAttemptItsOwnDeadline(t *testing.T) {
	r := &Reconciler{
		PlaceTimeout:    20 * time.Millisecond,
		MemberReadRetry: MemberReadRetry{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond},
	}
	attempts := 0
	_, err := retryMemberRead(t.Context(), r, "member-a", func(ctx context.Context) (struct{}, error) {
		attempts++
		if attempts == 1 {
			// The first read hangs until its own deadline passes.
			<-ctx.Done()
			return struct{}{}, &url.Error{Op: "Get", URL: "https://member-a.example", Err: ctx.Err()}
		}
		return struct{}{}, ctx.Err()
	})
	require.NoError(t, err, "a timed-out first read must not use up the second read's deadline")
	assert.Equal(t, 2, attempts)
}

func TestRetryMemberReadStopsWhenTheReconcileIsCancelled(t *testing.T) {
	r := &Reconciler{MemberReadRetry: MemberReadRetry{MaxAttempts: 5, InitialBackoff: time.Hour, MaxBackoff: time.Hour}}
	ctx, cancel := context.WithCancel(t.Context())
	attempts := 0
	_, err := retryMemberRead(ctx, r, "member-a", func(context.Context) (struct{}, error) {
		attempts++
		cancel()
		return struct{}{}, context.DeadlineExceeded
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, attempts)

	waiting, stop := context.WithCancel(t.Context())
	time.AfterFunc(10*time.Millisecond, stop)
	attempts = 0
	_, err = retryMemberRead(waiting, r, "member-a", func(context.Context) (struct{}, error) {
		attempts++
		return struct{}{}, context.DeadlineExceeded
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, attempts, "cancellation during the backoff ends the retries")
}

func TestTransientMemberReadError(t *testing.T) {
	gr := schema.GroupResource{Group: "ome.io", Resource: "inferencereplicas"}
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "request deadline", err: &url.Error{Op: "Get", URL: "https://member.example", Err: context.DeadlineExceeded}, want: true},
		{name: "wrapped deadline", err: fmt.Errorf("list pods: %w", context.DeadlineExceeded), want: true},
		{name: "member informer unavailable", err: fmt.Errorf("read service: %w", workloadcluster.ErrMemberCacheUnavailable), want: true},
		{name: "server timeout", err: apierrors.NewServerTimeout(gr, "list", 1), want: true},
		{name: "gateway timeout", err: apierrors.NewTimeoutError("slow", 1), want: true},
		{name: "throttled", err: apierrors.NewTooManyRequests("slow down", 1), want: true},
		{name: "internal error", err: apierrors.NewInternalError(errors.New("etcd")), want: true},
		{name: "service unavailable", err: apierrors.NewServiceUnavailable("starting"), want: true},
		{name: "bad gateway", err: apierrors.NewGenericServerResponse(502, "get", gr, "ir", "", 0, false), want: true},
		{name: "connection refused", err: &url.Error{Op: "Get", URL: "https://member.example", Err: syscall.ECONNREFUSED}, want: true},
		{name: "connection reset", err: &url.Error{Op: "Get", URL: "https://member.example", Err: syscall.ECONNRESET}, want: true},
		{name: "unexpected EOF", err: &url.Error{Op: "Get", URL: "https://member.example", Err: io.ErrUnexpectedEOF}, want: true},
		{name: "not found", err: apierrors.NewNotFound(gr, "ir")},
		{name: "forbidden", err: apierrors.NewForbidden(gr, "ir", errors.New("denied"))},
		{name: "conflict", err: apierrors.NewConflict(gr, "ir", errors.New("changed"))},
		{name: "first connection pending", err: fmt.Errorf("cluster %q: %w", "member-a", errMemberConnectionPending), want: true},
		{name: "identity check", err: errors.New("cluster \"member-a\" registration identity changed")},
		{name: "connection decided against", err: errors.New("cluster \"member-a\" has no connection for its accepted UID")},
		{name: "cancelled", err: context.Canceled},
		{name: "no error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, transientMemberReadError(tt.err))
		})
	}
}

func TestUnknownCandidateKeepsReadyWithinTheGrace(t *testing.T) {
	since := func(ago time.Duration) *metav1.Time {
		ts := metav1.NewTime(graceTestNow.Add(-ago))
		return &ts
	}
	previous := v1beta1.CandidatePlacement{
		Cluster: "member-a", Phase: v1beta1.CandidatePhaseAdmitted, ObservationKnown: true,
		AdmittedReplicas: 4, ReadyReplicas: 4, AppliedPlanID: "plan-1",
	}
	for _, tt := range []struct {
		name       string
		grace      time.Duration
		wasUnknown bool
		dropped    bool
		since      *metav1.Time
		ready      int32
		wantSince  *metav1.Time
		wantReady  int32
	}{
		{name: "first failed read inside the grace", grace: 5 * time.Minute, ready: 4, wantSince: since(0), wantReady: 4},
		// The start comes only from the stored candidate, so a failure recorded
		// before a controller restart keeps its original grace.
		{name: "failure recorded by an earlier pass", grace: 5 * time.Minute, since: since(4 * time.Minute), ready: 4, wantSince: since(4 * time.Minute), wantReady: 4},
		{name: "grace has passed", grace: 5 * time.Minute, since: since(5 * time.Minute), ready: 4, wantSince: since(5 * time.Minute)},
		{name: "no ready count stays at zero", grace: 5 * time.Minute, since: since(time.Minute), wantSince: since(time.Minute)},
		{name: "grace unset withdraws at once", ready: 4, wantSince: since(0)},
		// A hold or a plan merge can carry a member forward as unknown without a
		// failure start; its first failed read still starts a grace.
		{name: "carried forward as unknown without a start", grace: 5 * time.Minute, wasUnknown: true, ready: 4, wantSince: since(0), wantReady: 4},
		// A stored CRD that drops the start would restart the grace on every
		// pass, so none is granted while it does.
		{name: "stored CRD drops the start", grace: 5 * time.Minute, dropped: true, ready: 4, wantSince: since(0)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{ObservationGrace: tt.grace, ObservationClock: clocktesting.NewFakePassiveClock(graceTestNow)}
			r.failingSinceDropped.Store(tt.dropped)
			in := *previous.DeepCopy()
			in.ObservationKnown = !tt.wasUnknown
			in.ObservationFailingSince, in.ReadyReplicas = tt.since, tt.ready
			got := r.unknownCandidate(srcISVC(""), in)

			want := *in.DeepCopy()
			want.ObservationKnown = false
			want.ObservationFailingSince = tt.wantSince
			want.ReadyReplicas = tt.wantReady
			assert.Equal(t, want, got)
		})
	}
}

func TestGraceWakeRequeuesByTheEarliestGraceEnd(t *testing.T) {
	failing := func(cluster string, ago time.Duration) v1beta1.CandidatePlacement {
		ts := metav1.NewTime(graceTestNow.Add(-ago))
		return v1beta1.CandidatePlacement{Cluster: cluster, ObservationFailingSince: &ts, ReadyReplicas: 2}
	}
	withdrawn := failing("member-c", 10*time.Minute)
	withdrawn.ReadyReplicas = 0
	candidates := []v1beta1.CandidatePlacement{
		failing("member-a", time.Minute),
		failing("member-b", 3*time.Minute),
		withdrawn,
		{Cluster: "member-d", ObservationKnown: true, ReadyReplicas: 2},
	}
	r := &Reconciler{ObservationGrace: 5 * time.Minute, ObservationClock: clocktesting.NewFakePassiveClock(graceTestNow)}
	wake := &graceWake{}
	r.noteGraceEnds(context.WithValue(t.Context(), graceWakeKey{}, wake), candidates)
	assert.Equal(t, graceTestNow.Add(2*time.Minute), wake.at, "member-b's grace ends first; member-c's passed and its count is gone")

	assert.Equal(t, ctrl.Result{RequeueAfter: 2 * time.Minute}, r.wakeForGrace(ctrl.Result{RequeueAfter: 10 * time.Minute}, wake))
	assert.Equal(t, ctrl.Result{RequeueAfter: 2 * time.Minute}, r.wakeForGrace(ctrl.Result{}, wake), "a pass that would not requeue still wakes for the grace")
	assert.Equal(t, ctrl.Result{RequeueAfter: time.Minute}, r.wakeForGrace(ctrl.Result{RequeueAfter: time.Minute}, wake))
	assert.Equal(t, ctrl.Result{RequeueAfter: time.Minute}, r.wakeForGrace(ctrl.Result{RequeueAfter: time.Minute}, &graceWake{}))

	unset := &graceWake{}
	(&Reconciler{}).noteGraceEnds(context.WithValue(t.Context(), graceWakeKey{}, unset), candidates)
	assert.True(t, unset.at.IsZero(), "without a grace no count is retained to withdraw")

	// The grace ran out during the pass after the count was kept, for example
	// while a sibling member was being read: the next pass must come at once.
	expired := &graceWake{}
	r.noteGraceEnds(context.WithValue(t.Context(), graceWakeKey{}, expired), []v1beta1.CandidatePlacement{failing("member-a", 6*time.Minute)})
	assert.Equal(t, graceTestNow, expired.at)
	assert.Equal(t, ctrl.Result{RequeueAfter: time.Nanosecond}, r.wakeForGrace(ctrl.Result{RequeueAfter: time.Minute}, expired))
}

func TestNoteFailingSinceStored(t *testing.T) {
	ts := metav1.NewTime(graceTestNow)
	sent := []v1beta1.CandidatePlacement{
		{Cluster: "member-a", ReadyReplicas: 2},
		{Cluster: "member-b", ObservationFailingSince: &ts, ReadyReplicas: 2},
	}
	r := &Reconciler{ObservationGrace: 5 * time.Minute}

	r.noteFailingSinceStored(sent, []v1beta1.CandidatePlacement{{Cluster: "member-a"}, {Cluster: "member-b"}})
	assert.True(t, r.failingSinceDropped.Load(), "a sent start that did not come back means the CRD lacks the field")
	assert.Zero(t, r.observationGrace())

	r.noteFailingSinceStored(sent[:1], []v1beta1.CandidatePlacement{{Cluster: "member-a"}})
	assert.True(t, r.failingSinceDropped.Load(), "a write that sends no start proves nothing either way")

	r.noteFailingSinceStored(sent, []v1beta1.CandidatePlacement{{Cluster: "member-a"}, {Cluster: "member-b", ObservationFailingSince: &ts}})
	assert.False(t, r.failingSinceDropped.Load(), "a stored start means the CRD has the field again")
	assert.Equal(t, 5*time.Minute, r.observationGrace())
}

// ingressObservation builds a placer over member-a, which serves one ready
// replica. Its ingress readiness is set, its InferenceReplica reads can be
// made to fail, and its stored candidate is known with a ready count of 4.
func ingressObservation(t *testing.T, ingressReady, replicaReadsFail bool) (*Reconciler, plannedObservationFixture) {
	t.Helper()
	f := observationFixture(t)
	status := corev1.ConditionTrue
	if !ingressReady {
		status = corev1.ConditionFalse
	}
	f.member.Status.SetCondition(v1beta1.IngressReady, &apis.Condition{Status: status})
	if replicaReadsFail {
		unavailable := apierrors.NewServiceUnavailable("member API server restarting")
		f.intercept.List = func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*v1beta1.InferenceReplicaList); ok {
				return unavailable
			}
			return cl.List(ctx, list, opts...)
		}
		f.intercept.Get = func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*v1beta1.InferenceReplica); ok {
				return unavailable
			}
			return cl.Get(ctx, key, obj, opts...)
		}
	}
	objects := append([]client.Object{}, f.extraObjects...)
	objects = append(objects, f.member, f.resources.ir)
	for i := range f.resources.pods {
		objects = append(objects, &f.resources.pods[i])
	}
	scheme := testScheme(t)
	worker := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(f.intercept).Build()
	clusters := identifiedTestClusters{fakeClusters: fakeClusters{m: map[string]workloadcluster.SelectivelyCachingClient{"member-a": workloadcluster.NewNeverCachingClient(worker)}}, uid: "member-a-uid"}
	r, _ := newPlacer(scheme, clusters, f.source, plannedTestRegistration())
	r.InstanceStatusDecoder = irstatus.NewDecoder(f.decodeBound)
	r.ObservationGrace = 5 * time.Minute
	r.ObservationClock = clocktesting.NewFakePassiveClock(graceTestNow)
	f.source.Status.Placement.Candidates[0].ObservationKnown = true
	f.source.Status.Placement.Candidates[0].ReadyReplicas = 4
	return r, f
}

func TestStandingReadMarksAReachedMemberKnown(t *testing.T) {
	r, f := ingressObservation(t, true, false)
	previous := *f.source.Status.Placement.Candidates[0].DeepCopy()
	previous.ObservationKnown = false
	member := r.observeMember(t.Context(), f.source, previous, true)
	require.NoError(t, member.err)
	assert.Equal(t, memberPresent, member.state)
	assert.True(t, member.candidate.ObservationKnown, "every path that writes a reached member, a hold included, must record it as known")
}

func TestUnreadableReplicasWithIngressNotReadyKeepNoCount(t *testing.T) {
	for _, tt := range []struct {
		name         string
		ingressReady bool
		wantReady    int32
	}{
		{name: "ingress ready keeps the count for the grace", ingressReady: true, wantReady: 4},
		{name: "ingress not ready withdraws the count", ingressReady: false, wantReady: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("standing read", func(t *testing.T) {
				r, f := ingressObservation(t, tt.ingressReady, true)
				member := r.observeMember(t.Context(), f.source, f.source.Status.Placement.Candidates[0], true)
				require.Error(t, member.err)
				assert.Equal(t, memberUnknown, member.state)
				assert.Equal(t, tt.wantReady, member.candidate.ReadyReplicas)
				assert.NotNil(t, member.candidate.ObservationFailingSince)
			})
			t.Run("planned read", func(t *testing.T) {
				r, f := ingressObservation(t, tt.ingressReady, true)
				got, err := r.observePlannedMember(t.Context(), f.source, f.source.Status.Placement.Candidates[0], f.components)
				require.Error(t, err)
				assert.False(t, got.Candidate.ObservationKnown)
				assert.Equal(t, tt.wantReady, got.Candidate.ReadyReplicas)
			})
		})
	}
}

// failingAllMember fails the first failures inventory lists on one member
// with err, then reads normally.
func failingAllMember(f *backendFixture, name string, failures int, err error) workloadcluster.SelectivelyCachingClient {
	return workloadcluster.NewNeverCachingClient(interceptor.NewClient(f.workers[name], interceptor.Funcs{List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		if _, ok := list.(*v1beta1.InferenceReplicaList); ok && failures > 0 {
			failures--
			return err
		}
		return cl.List(ctx, list, opts...)
	}}))
}

func reconcileResult(t *testing.T, f *backendFixture) (ctrl.Result, *v1beta1.InferenceService) {
	t.Helper()
	res, err := f.reconciler.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.source)})
	require.NoError(t, err)
	got := &v1beta1.InferenceService{}
	require.NoError(t, f.reconciler.Get(t.Context(), client.ObjectKeyFromObject(f.source), got))
	return res, got
}

func TestAllTransientMemberReadFailureKeepsReadyWhenRetried(t *testing.T) {
	const replicas = 3
	transient := &url.Error{Op: "Get", URL: "https://member-a.example", Err: context.DeadlineExceeded}

	t.Run("one failed read is retried", func(t *testing.T) {
		f, _ := settleAllServingMembers(t, replicas)
		f.reconciler.MemberReadRetry = MemberReadRetry{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
		f.connections.m["member-a"] = failingAllMember(f, "member-a", 1, transient)
		_, got := reconcileResult(t, f)
		candidate := candidateOf(t, got, "member-a")
		assert.True(t, candidate.ObservationKnown)
		assert.Equal(t, int32(replicas), candidate.ReadyReplicas)
		assert.Nil(t, candidate.ObservationFailingSince)
	})

	t.Run("without retries the failed read withdraws the count", func(t *testing.T) {
		f, _ := settleAllServingMembers(t, replicas)
		f.connections.m["member-a"] = failingAllMember(f, "member-a", 1, transient)
		_, got := reconcileResult(t, f)
		candidate := candidateOf(t, got, "member-a")
		assert.False(t, candidate.ObservationKnown)
		assert.Zero(t, candidate.ReadyReplicas)
		assert.NotNil(t, candidate.ObservationFailingSince, "the failure is dated even without a grace")
	})
}

func TestAllUnreadableHomeKeepsReadyUntilTheGraceEnds(t *testing.T) {
	for _, informerFailure := range []bool{false, true} {
		t.Run(fmt.Sprintf("informer=%t", informerFailure), func(t *testing.T) {

			const replicas = 3
			f, settled := settleAllServingMembers(t, replicas)
			clock := clocktesting.NewFakePassiveClock(graceTestNow)
			f.reconciler.ObservationClock = clock
			f.reconciler.ObservationGrace = 10 * time.Second
			f.reconciler.Requeue = time.Minute
			before := candidateOf(t, settled, "member-a")
			require.True(t, before.ObservationKnown)
			require.Equal(t, int32(replicas), before.ReadyReplicas)
			assertCandidateObservationMetrics(t, settled, "member-a", replicas, 1, 0)

			if informerFailure {
				f.connections.m["member-a"] = workloadcluster.NewNeverCachingClient(interceptor.NewClient(f.workers["member-a"], interceptor.Funcs{
					Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*v1beta1.InferenceService); ok {
							return workloadcluster.ErrMemberCacheUnavailable
						}
						return cl.Get(ctx, key, obj, opts...)
					},
				}))
			} else {
				f.connections.m["member-a"] = unreadableAllMember(f, "member-a")
			}
			res, got := reconcileResult(t, f)
			candidate := candidateOf(t, got, "member-a")
			assert.False(t, candidate.ObservationKnown)
			assert.Equal(t, int32(replicas), candidate.ReadyReplicas, "one failed read keeps the routed count")
			require.NotNil(t, candidate.ObservationFailingSince)
			assert.True(t, candidate.ObservationFailingSince.Time.Equal(graceTestNow))
			assert.Equal(t, 10*time.Second, res.RequeueAfter, "the controller wakes when the grace ends")
			assertCandidateObservationMetrics(t, got, "member-a", replicas, 0, float64(graceTestNow.Unix()))
			wantReason := "ObservationUnknown"
			if informerFailure {
				wantReason = "AwaitingHomeInputs"
			}
			if condition := got.Status.GetCondition(v1beta1.PlacementConverged); condition == nil || condition.Reason != wantReason {
				t.Fatalf("an unreadable member must still hold the allocation: %+v", condition)
			}

			// The grace is measured from the recorded first failure, not restarted.
			clock.SetTime(graceTestNow.Add(4 * time.Second))
			res, got = reconcileResult(t, f)
			candidate = candidateOf(t, got, "member-a")
			assert.Equal(t, int32(replicas), candidate.ReadyReplicas)
			assert.True(t, candidate.ObservationFailingSince.Time.Equal(graceTestNow))
			assert.Equal(t, 6*time.Second, res.RequeueAfter)
			assertCandidateObservationMetrics(t, got, "member-a", replicas, 0, float64(graceTestNow.Unix()))

			clock.SetTime(graceTestNow.Add(10 * time.Second))
			res, got = reconcileResult(t, f)
			candidate = candidateOf(t, got, "member-a")
			assert.Zero(t, candidate.ReadyReplicas, "a member still unreadable after the grace stops receiving traffic")
			assert.True(t, candidate.ObservationFailingSince.Time.Equal(graceTestNow))
			assert.Equal(t, time.Minute, res.RequeueAfter, "no grace is left to wake for")
			assertCandidateObservationMetrics(t, got, "member-a", 0, 0, float64(graceTestNow.Unix()))

			f.connections.m["member-a"] = workloadcluster.NewNeverCachingClient(f.workers["member-a"])
			_, got = reconcileResult(t, f)
			candidate = candidateOf(t, got, "member-a")
			assert.True(t, candidate.ObservationKnown)
			assert.Equal(t, int32(replicas), candidate.ReadyReplicas)
			assert.Nil(t, candidate.ObservationFailingSince, "a successful read clears the failure time")
			assertCandidateObservationMetrics(t, got, "member-a", replicas, 1, 0)

		})
	}
}

func TestReconcile_SplitUnreadableHomeKeepsReadyWithinTheGrace(t *testing.T) {
	s := testScheme(t)
	good := workerWithReplicas(t, s, "svc.a.example", 1, 1)
	bad := irGetFailClient{WithWatch: workerWithReplicas(t, s, "svc.z.example", 1, 1)}
	clusters := fakeClusters{m: map[string]workloadcluster.SelectivelyCachingClient{
		"a": workloadcluster.NewNeverCachingClient(good),
		"z": workloadcluster.NewNeverCachingClient(bad),
	}}
	src := srcISVCSplit("gpu=gb300", 2)
	src.Status.Placement = &v1beta1.PlacementStatus{
		Phase: v1beta1.PlacementPhasePlaced,
		Candidates: []v1beta1.CandidatePlacement{
			{Cluster: "z", Phase: v1beta1.CandidatePhaseAdmitted, ObservationKnown: true, AdmittedReplicas: 1, ReadyReplicas: 1},
		},
	}
	r, cp := newPlacer(s, clusters, src,
		readyWC("a", map[string]string{"gpu": "gb300"}), readyWC("z", map[string]string{"gpu": "gb300"}))
	r.ObservationGrace = time.Minute
	r.ObservationClock = clocktesting.NewFakePassiveClock(graceTestNow)

	_, err := r.Reconcile(context.Background(), req())
	require.NoError(t, err)
	p := cpPlacement(t, cp)
	require.NotNil(t, p)
	zc, ok := candidatesByCluster(p.Candidates)["z"]
	require.True(t, ok)
	assert.False(t, zc.ObservationKnown)
	assert.Equal(t, int32(1), zc.ReadyReplicas, "an unreadable member keeps its routed count within the grace")
	require.NotNil(t, zc.ObservationFailingSince)
	assert.True(t, zc.ObservationFailingSince.Time.Equal(graceTestNow))
}

// connectingClusters hides one member until placement asks whether its
// connection is still pending, as a registry does while its first connect
// after a controller start is in flight. With pending set, that first connect
// lands before the re-read.
type connectingClusters struct {
	fakeClusters
	cluster string
	pending bool

	mu        sync.Mutex
	connected bool
	asked     int
	lookups   int
}

func (c *connectingClusters) hidden(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if name != c.cluster {
		return false
	}
	c.lookups++
	return !c.connected
}

func (c *connectingClusters) ClientFor(name string) (workloadcluster.SelectivelyCachingClient, bool) {
	if c.hidden(name) {
		return nil, false
	}
	return c.fakeClusters.ClientFor(name)
}

func (c *connectingClusters) ClientForUID(name string, uid types.UID) (workloadcluster.SelectivelyCachingClient, bool) {
	if c.hidden(name) {
		return nil, false
	}
	return c.fakeClusters.ClientForUID(name, uid)
}

func (c *connectingClusters) ConnectionPending(name string, _ types.UID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if name != c.cluster || c.connected {
		return false
	}
	c.asked++
	if c.pending {
		c.connected = true
	}
	return c.pending
}

var pendingConnectionCases = []struct {
	name        string
	pending     bool
	retry       MemberReadRetry
	wantReread  bool
	wantPending bool
}{
	{name: "a first connection still being made is read again", pending: true, retry: MemberReadRetry{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}, wantReread: true},
	{name: "a connection the registry decided against is not read again", retry: MemberReadRetry{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}},
	{name: "without retries a pending connection fails the read", pending: true, wantPending: true},
}

// connectingObservation builds a placer whose member-a is hidden behind
// connectingClusters, over a member that serves one ready replica.
func connectingObservation(t *testing.T, pending bool, retry MemberReadRetry) (*Reconciler, *connectingClusters, plannedObservationFixture) {
	t.Helper()
	f := observationFixture(t)
	objects := append([]client.Object{}, f.extraObjects...)
	objects = append(objects, f.member, f.resources.ir)
	for i := range f.resources.pods {
		objects = append(objects, &f.resources.pods[i])
	}
	scheme := testScheme(t)
	worker := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(f.intercept).Build()
	clusters := &connectingClusters{fakeClusters: fakeClusters{m: map[string]workloadcluster.SelectivelyCachingClient{"member-a": workloadcluster.NewNeverCachingClient(worker)}}, cluster: "member-a", pending: pending}
	r, _ := newPlacer(scheme, clusters, f.source, plannedTestRegistration())
	r.InstanceStatusDecoder = irstatus.NewDecoder(f.decodeBound)
	r.MemberReadRetry = retry
	return r, clusters, f
}

func TestPlannedReadRetriesAPendingFirstConnection(t *testing.T) {
	for _, tt := range pendingConnectionCases {
		t.Run(tt.name, func(t *testing.T) {
			r, clusters, f := connectingObservation(t, tt.pending, tt.retry)
			got, err := r.observePlannedMember(t.Context(), f.source, f.source.Status.Placement.Candidates[0], f.components)
			assert.Equal(t, 1, clusters.asked, "the first read found no connection and asked whether it was pending")
			if tt.wantReread {
				require.NoError(t, err)
				assert.Equal(t, 2, clusters.lookups)
				assert.True(t, got.Candidate.ObservationKnown)
				return
			}
			require.Error(t, err)
			assert.Equal(t, 1, clusters.lookups, "the member is read once")
			assert.Equal(t, tt.wantPending, errors.Is(err, errMemberConnectionPending), "error: %v", err)
			assert.False(t, got.Candidate.ObservationKnown)
		})
	}
}

func TestStandingReadRetriesAPendingFirstConnection(t *testing.T) {
	for _, tt := range pendingConnectionCases {
		t.Run(tt.name, func(t *testing.T) {
			r, clusters, f := connectingObservation(t, tt.pending, tt.retry)
			// Without an allocation the read finds its connection by cluster name.
			previous := *f.source.Status.Placement.Candidates[0].DeepCopy()
			previous.Allocation = nil
			got := r.observeMember(t.Context(), f.source, previous, true)
			assert.Equal(t, 1, clusters.asked, "the first read found no connection and asked whether it was pending")
			if tt.wantReread {
				assert.Equal(t, 2, clusters.lookups)
				assert.NotEqual(t, memberUnknown, got.state, "error: %v", got.err)
				return
			}
			assert.Equal(t, 1, clusters.lookups, "the member is read once")
			assert.Equal(t, memberUnknown, got.state)
			assert.Equal(t, tt.wantPending, errors.Is(got.err, errMemberConnectionPending), "error: %v", got.err)
		})
	}
}

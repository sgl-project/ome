package effective

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	knapis "knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

func TestResolvePinRemainsActiveWhenLiveSourceIsMissingOrDisabled(t *testing.T) {
	autoSync := false
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("pin"))
	tests := []struct {
		name             string
		liveErr          error
		wantAvailability LiveRuntimeAvailability
	}{
		{
			name: "not found",
			liveErr: &runtimeselector.RuntimeNotFoundError{
				RuntimeName: "runtime", Namespace: "workloads",
			},
			wantAvailability: LiveRuntimeNotFound,
		},
		{
			name:             "disabled",
			liveErr:          &runtimeselector.RuntimeDisabledError{RuntimeName: "runtime", IsCluster: true},
			wantAvailability: LiveRuntimeDisabled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver, err := newRuntimePinResolver(
				func(string) revisionNamespace {
					return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
						return revision.DeepCopy(), nil
					}}
				},
				liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
					return nil, test.liveErr
				}),
				"ome", testPinLimits,
			)
			require.NoError(t, err)
			isvc := pinISVC("runtime", &autoSync, "")
			isvc.Status.PinnedRevisionName = revision.Name

			state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})

			require.NoError(t, err)
			assert.Equal(t, test.wantAvailability, state.LiveAvailability())
			assert.Equal(t, RuntimePinStateResolved, state.PinState)
			active, err := state.RequireActive()
			require.NoError(t, err)
			assert.Equal(t, revision.Name, active.RevisionName)
		})
	}
}

func TestResolveHardLiveFailureCollectsExactPinButDoesNotLabelItActive(t *testing.T) {
	autoSync := false
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("pin"))
	liveCause := errors.New("credential-bearing upstream failure")
	var getCalls int
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				getCalls++
				return revision.DeepCopy(), nil
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return nil, liveCause
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", &autoSync, revision.Name)

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})

	require.NoError(t, err)
	assert.Equal(t, 1, getCalls)
	assert.Equal(t, LiveRuntimeUnavailable, state.LiveAvailability())
	assert.Equal(t, RuntimePinStateUnavailable, state.PinState)
	_, err = state.RequireActive()
	assert.EqualError(t, err, "active runtime configuration is unavailable")
	issues := state.SourceIssues()
	require.Len(t, issues, 1)
	assert.Equal(t, "live runtime evidence is unavailable", issues[0].Error())
	assert.ErrorIs(t, issues[0], liveCause)
}

func TestResolveRevisionFailureStates(t *testing.T) {
	autoSync := false
	disabled := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", &v1beta1.ServingRuntimeSpec{Disabled: ptrBool(true)})
	malformed := disabled.DeepCopy()
	malformed.Name = "malformed"
	malformed.Data.Raw = []byte(`{`)
	missingErr := apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "controllerrevisions"}, "missing")
	tests := []struct {
		name     string
		revision string
		get      func() (*appsv1.ControllerRevision, error)
		want     RuntimePinState
	}{
		{name: "missing", revision: "missing", get: func() (*appsv1.ControllerRevision, error) { return nil, missingErr }, want: RuntimePinStateRevisionMissing},
		{name: "malformed", revision: malformed.Name, get: func() (*appsv1.ControllerRevision, error) { return malformed.DeepCopy(), nil }, want: RuntimePinStateRevisionInvalid},
		{name: "disabled", revision: disabled.Name, get: func() (*appsv1.ControllerRevision, error) { return disabled.DeepCopy(), nil }, want: RuntimePinStateRevisionDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver, err := newRuntimePinResolver(
				func(string) revisionNamespace {
					return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
						return test.get()
					}}
				},
				liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
					return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
				}),
				"ome", testPinLimits,
			)
			require.NoError(t, err)
			isvc := pinISVC("runtime", &autoSync, test.revision)

			state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})
			require.NoError(t, err)
			assert.Equal(t, test.want, state.PinState)
			_, err = state.RequireActive()
			assert.Error(t, err)
		})
	}
}

func TestResolveBoundsExactGetAndPreservesTimeoutCause(t *testing.T) {
	limits := testPinLimits
	limits.RequestTimeout = 5 * time.Millisecond
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(ctx context.Context, _ string, _ metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", limits,
	)
	require.NoError(t, err)
	isvc := pinISVC("runtime", nil, "")
	isvc.Status.PinnedRevisionName = "retained"

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})

	require.NoError(t, err)
	active, err := state.RequireActive()
	require.NoError(t, err)
	assert.Equal(t, ConfigurationOriginLiveRuntime, active.Origin)
	issues := state.SourceIssues()
	require.Len(t, issues, 1)
	assert.Equal(t, "runtime revision evidence read failed", issues[0].Error())
	assert.ErrorIs(t, issues[0], context.DeadlineExceeded)
}

func TestResolveParentCancellationIsTopLevel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace { return revisionNamespaceStub{} },
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)

	_, err = resolver.Resolve(ctx, pinISVC("runtime", nil, ""), RuntimeResolveOptions{})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestResolveChecksParentCancellationAfterSuccessfulExactGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	autoSync := false
	revision := revisionFixture(t, runtimeselector.KindClusterServingRuntime, "", "runtime", runtimeSpecFixture("pin"))
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace {
			return revisionNamespaceStub{get: func(context.Context, string, metav1.GetOptions) (*appsv1.ControllerRevision, error) {
				cancel()
				return revision.DeepCopy(), nil
			}}
		},
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return livePinFixture("runtime", runtimeselector.KindClusterServingRuntime, "", false), nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)

	_, err = resolver.Resolve(ctx, pinISVC("runtime", &autoSync, revision.Name), RuntimeResolveOptions{})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestResolveInitializesSafeDefaultsAndMarksLiveActiveEqual(t *testing.T) {
	resolver, err := newRuntimePinResolver(
		func(string) revisionNamespace { return revisionNamespaceStub{} },
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			live := livePinFixture("selected", runtimeselector.KindClusterServingRuntime, "", false)
			live.Runtime.SelectionSource = RuntimeSelected
			return live, nil
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	isvc := pinISVC("selected", nil, "")

	state, err := resolver.Resolve(context.Background(), isvc, RuntimeResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, RuntimeSelected, state.SelectionSource)
	assert.Equal(t, LiveRuntimeAvailable, state.LiveAvailability())
	assert.Equal(t, RuntimeHashRelationEqual, state.LiveToActive)

	hardResolver, err := newRuntimePinResolver(
		func(string) revisionNamespace { return revisionNamespaceStub{} },
		liveRuntimeResolverFunc(func(context.Context, *v1beta1.InferenceService) (*LiveConfiguration, error) {
			return nil, errors.New("unreadable")
		}),
		"ome", testPinLimits,
	)
	require.NoError(t, err)
	empty := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "chat", Namespace: "workloads"}}
	state, err = hardResolver.Resolve(context.Background(), empty, RuntimeResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, RuntimeSelected, state.SelectionSource)
	assert.Equal(t, RuntimeHashRelationUnknown, state.LiveToActive)
}

func TestDerivedRuntimeStates(t *testing.T) {
	assert.Equal(t, StatusFreshnessUnknown, deriveStatusFreshness(3, 0))
	assert.Equal(t, StatusFreshnessCurrent, deriveStatusFreshness(3, 3))
	assert.Equal(t, StatusFreshnessStale, deriveStatusFreshness(3, 2))
	assert.Equal(t, StatusFreshnessInconsistent, deriveStatusFreshness(3, 4))

	assert.Equal(t, SyncTokenStateAbsent, deriveSyncTokenState("", ""))
	assert.Equal(t, SyncTokenStateStatusOnly, deriveSyncTokenState("", "old"))
	assert.Equal(t, SyncTokenStateAcknowledged, deriveSyncTokenState("same", "same"))
	assert.Equal(t, SyncTokenStatePending, deriveSyncTokenState("new", "old"))

	state, reason := deriveRuntimeDrift(knapis.Conditions(nil))
	assert.Equal(t, RuntimeDriftStateNotReported, state)
	assert.Empty(t, reason)
	state, reason = deriveRuntimeDrift([]knapis.Condition{{
		Type: knapis.ConditionType(constants.RuntimeDriftedConditionType), Status: corev1.ConditionTrue, Reason: "RevisionMismatch",
	}})
	assert.Equal(t, RuntimeDriftStateReportedTrue, state)
	assert.Equal(t, RuntimeDriftReasonRevisionMismatch, reason)
	state, reason = deriveRuntimeDrift([]knapis.Condition{{
		Type: knapis.ConditionType(constants.RuntimeDriftedConditionType), Status: corev1.ConditionUnknown, Reason: "secret-reason",
	}})
	assert.Equal(t, RuntimeDriftStateReportedUnknown, state)
	assert.Equal(t, RuntimeDriftReasonOther, reason)
	state, _ = deriveRuntimeDrift([]knapis.Condition{{
		Type: knapis.ConditionType(constants.RuntimeDriftedConditionType), Status: corev1.ConditionStatus("bogus"),
	}})
	assert.Equal(t, RuntimeDriftStateMalformed, state)

	assert.Equal(t, RuntimeHashRelationAmbiguous, compareRuntimeHashes("aaaa", "deadbeef", "bbbb", "deadbeef"))
}

func ptrBool(value bool) *bool { return &value }

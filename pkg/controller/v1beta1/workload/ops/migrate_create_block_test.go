package ops

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func blockMigrationPodCreates(t *testing.T, f *migFixture, blocked *bool, allowedWhileBlocked int, blockErr error) *int {
	t.Helper()
	base, ok := f.c.(client.WithWatch)
	if !ok {
		t.Fatalf("fixture client %T does not implement client.WithWatch", f.c)
	}
	attempts := 0
	f.c = interceptor.NewClient(base, interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			pod, isPod := obj.(*corev1.Pod)
			if !isPod {
				return cl.Create(ctx, obj, opts...)
			}
			attempts++
			if !*blocked || allowedWhileBlocked > 0 {
				if *blocked {
					allowedWhileBlocked--
				}
				return cl.Create(ctx, obj, opts...)
			}
			if blockErr != nil {
				return blockErr
			}
			return apierrors.NewForbidden(corev1.Resource("pods"), pod.Name, errors.New("admission rejected test pod"))
		},
	})
	return &attempts
}

func TestMigrate_SurgePodCreateBlockUsesMigrationDeadline(t *testing.T) {
	clk := clocktesting.NewFakeClock(time.Unix(1_700_000_000, 0))
	f := newSinglePodMigFixture(t)
	f.clk = clk
	const uuid = "mig-create-block-deadline"
	f.records = []workload.MigrationRecord{
		mkMigRecordWithDeadline(uuid, 0, "node-a", clk.Now().Add(time.Minute)),
	}
	blocked := true
	attempts := blockMigrationPodCreates(t, f, &blocked, 0, nil)

	done, accepted, err := f.passResult(t, uuid)
	if err != nil || done || !accepted {
		t.Fatalf("blocked create pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	if *attempts != 1 {
		t.Fatalf("pod create attempts = %d, want 1", *attempts)
	}
	if !workload.DefaultExpectations.Satisfied(f.isvc.Namespace, f.isvc.Name, f.component, 1) {
		t.Fatal("rejected pod create left an outstanding expectation")
	}
	if rec := f.record(t, uuid); rec.Phase != workload.MigrationPhaseSurgePending {
		t.Fatalf("record phase = %s, want SurgePending", rec.Phase)
	}

	clk.Step(2 * time.Minute)
	expired, err := ExpireMigrations(context.Background(), f.deps(), f.input(t), f.plan)
	if err != nil {
		t.Fatalf("ExpireMigrations: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired records = %d, want 1", expired)
	}
	if rec := f.record(t, uuid); rec.Phase != workload.MigrationPhaseFailed {
		t.Fatalf("record phase after deadline = %s, want Failed", rec.Phase)
	}
	if source := findInstanceStatusOnIRForFixture(t, f, 0); source == nil || workload.InstancePhase(source.Phase) != workload.InstancePhaseReady || source.Operation != nil {
		t.Fatalf("source not restored after expiry: %+v", source)
	}
}

func TestMigrate_SurgePodCreateBlockRetries(t *testing.T) {
	f := newSinglePodMigFixture(t)
	const uuid = "mig-create-block-retry"
	f.records = []workload.MigrationRecord{mkMigRecord(uuid, 0, "node-a")}
	blocked := true
	blockMigrationPodCreates(t, f, &blocked, 0, nil)

	done, accepted, err := f.passResult(t, uuid)
	if err != nil || done || !accepted {
		t.Fatalf("blocked create pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	blocked = false
	done, accepted, err = f.passResult(t, uuid)
	if err != nil || done || !accepted {
		t.Fatalf("retry create pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	pods, err := query.LiveListPodsForInstance(context.Background(), f.c, f.isvc.Namespace, f.isvc.Name, f.component, 1)
	if err != nil {
		t.Fatalf("list surge pods: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("surge pod count = %d, want 1", len(pods))
	}
}

func TestMigrate_GangPartialCreateBlockRetriesMissingMembers(t *testing.T) {
	f := newGangMigFixture(t)
	const uuid = "mig-gang-create-block"
	f.records = []workload.MigrationRecord{mkMigRecord(uuid, 0, "node-a")}
	blocked := true
	attempts := blockMigrationPodCreates(t, f, &blocked, 1, nil)

	if done, accepted, err := f.passResult(t, uuid); err != nil || done || !accepted {
		t.Fatalf("gang stamp pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	if done, accepted, err := f.passResult(t, uuid); err != nil || done || !accepted {
		t.Fatalf("partial gang create pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	if *attempts != 2 {
		t.Fatalf("pod create attempts = %d, want 2", *attempts)
	}
	pods, err := query.LiveListPodsForInstance(context.Background(), f.c, f.isvc.Namespace, f.isvc.Name, f.component, 1)
	if err != nil {
		t.Fatalf("list partial surge gang: %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("partial surge gang size = %d, want 1", len(pods))
	}

	blocked = false
	if done, accepted, err := f.passResult(t, uuid); err != nil || done || !accepted {
		t.Fatalf("gang retry pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	pods, err = query.LiveListPodsForInstance(context.Background(), f.c, f.isvc.Namespace, f.isvc.Name, f.component, 1)
	if err != nil {
		t.Fatalf("list retried surge gang: %v", err)
	}
	if len(pods) != 3 {
		t.Fatalf("retried surge gang size = %d, want 3", len(pods))
	}
}

func TestMigrate_GangCreateBlockBeforeFirstMember(t *testing.T) {
	f := newGangMigFixture(t)
	const uuid = "mig-gang-create-block-first"
	f.records = []workload.MigrationRecord{mkMigRecord(uuid, 0, "node-a")}
	blocked := true
	attempts := blockMigrationPodCreates(t, f, &blocked, 0, nil)

	if done, accepted, err := f.passResult(t, uuid); err != nil || done || !accepted {
		t.Fatalf("gang stamp pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	if done, accepted, err := f.passResult(t, uuid); err != nil || done || !accepted {
		t.Fatalf("blocked gang create pass: done=%v accepted=%v err=%v", done, accepted, err)
	}
	if *attempts != 1 {
		t.Fatalf("pod create attempts = %d, want 1", *attempts)
	}
	pods, err := query.LiveListPodsForInstance(context.Background(), f.c, f.isvc.Namespace, f.isvc.Name, f.component, 1)
	if err != nil {
		t.Fatalf("list blocked surge gang: %v", err)
	}
	if len(pods) != 0 {
		t.Fatalf("blocked surge gang size = %d, want 0", len(pods))
	}
}

func TestMigrate_SurgePodCreateBlockRequiresBoundedRetry(t *testing.T) {
	tests := []struct {
		name     string
		deadline bool
		blockErr error
	}{
		{name: "zero migration deadline", blockErr: apierrors.NewForbidden(corev1.Resource("pods"), "surge", errors.New("admission rejected test pod"))},
		{name: "context cancellation", deadline: true, blockErr: context.Canceled},
		{name: "context deadline", deadline: true, blockErr: context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSinglePodMigFixture(t)
			const uuid = "mig-create-block-unbounded"
			rec := mkMigRecord(uuid, 0, "node-a")
			if !tt.deadline {
				rec.Deadline = metav1.Time{}
			}
			f.records = []workload.MigrationRecord{rec}
			blocked := true
			blockMigrationPodCreates(t, f, &blocked, 0, tt.blockErr)

			done, accepted, err := f.passResult(t, uuid)
			if err == nil || done || !accepted {
				t.Fatalf("create failure: done=%v accepted=%v err=%v", done, accepted, err)
			}
			if tt.blockErr == context.Canceled && !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if tt.blockErr == context.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want context.DeadlineExceeded", err)
			}
		})
	}
}

package inferenceservice

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/components"
)

// requeueAsk is a component reconciler that only asks to be looked at again.
type requeueAsk struct{ result ctrl.Result }

func (a requeueAsk) Reconcile(context.Context, *v1beta1.InferenceService) (ctrl.Result, error) {
	return a.result, nil
}

// TestRunComponentReconcilersComesBackAtTheSoonestAsk asserts the rule the
// pass folds the components' requeue requests by: of two intervals the
// shorter wins, in either order, and an interval of zero asks for nothing.
func TestRunComponentReconcilersComesBackAtTheSoonestAsk(t *testing.T) {
	long, short := 10*time.Minute, 15*time.Second
	tests := []struct {
		name string
		asks []ctrl.Result
		want ctrl.Result
	}{
		{name: "the shorter interval wins", asks: []ctrl.Result{{RequeueAfter: long}, {RequeueAfter: short}}, want: ctrl.Result{RequeueAfter: short}},
		{name: "in either order", asks: []ctrl.Result{{RequeueAfter: short}, {RequeueAfter: long}}, want: ctrl.Result{RequeueAfter: short}},
		{name: "zero asks for nothing", asks: []ctrl.Result{{}, {RequeueAfter: short}, {}}, want: ctrl.Result{RequeueAfter: short}},
		{name: "nothing asked stays nothing", asks: []ctrl.Result{{}, {}}, want: ctrl.Result{}},
		{name: "a bare requeue stays a flag", asks: []ctrl.Result{{Requeue: true}, {}}, want: ctrl.Result{Requeue: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			pass := &servicePass{
				reconciler: &InferenceServiceReconciler{Recorder: record.NewFakeRecorder(8)},
				isvc:       &v1beta1.InferenceService{},
				log:        logr.Discard(),
			}
			for _, ask := range test.asks {
				pass.reconcilers = append(pass.reconcilers, components.Component(requeueAsk{result: ask}))
			}
			next, err := pass.runComponentReconcilers(context.Background())
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(next).To(gomega.BeTrue())
			g.Expect(pass.pendingRequeue).To(gomega.Equal(test.want))
		})
	}
}

// TestSoonestRequeue asserts the fold rule itself, which the run layer's, the
// components' and the canary groups' requests all go through.
func TestSoonestRequeue(t *testing.T) {
	long, short := 10*time.Minute, 15*time.Second
	tests := []struct {
		name           string
		pending, asked ctrl.Result
		want           ctrl.Result
	}{
		{name: "the shorter interval wins", pending: ctrl.Result{RequeueAfter: long}, asked: ctrl.Result{RequeueAfter: short}, want: ctrl.Result{RequeueAfter: short}},
		{name: "in either order", pending: ctrl.Result{RequeueAfter: short}, asked: ctrl.Result{RequeueAfter: long}, want: ctrl.Result{RequeueAfter: short}},
		{name: "zero pending asks for nothing", pending: ctrl.Result{}, asked: ctrl.Result{RequeueAfter: short}, want: ctrl.Result{RequeueAfter: short}},
		{name: "zero asked asks for nothing", pending: ctrl.Result{RequeueAfter: short}, asked: ctrl.Result{}, want: ctrl.Result{RequeueAfter: short}},
		{name: "nothing asked stays nothing", pending: ctrl.Result{}, asked: ctrl.Result{}, want: ctrl.Result{}},
		{name: "a bare requeue stays a flag", pending: ctrl.Result{}, asked: ctrl.Result{Requeue: true}, want: ctrl.Result{Requeue: true}},
		{name: "a bare requeue beside a timed ask keeps both", pending: ctrl.Result{Requeue: true}, asked: ctrl.Result{RequeueAfter: short}, want: ctrl.Result{Requeue: true, RequeueAfter: short}},
		{name: "a timed ask beside a bare requeue keeps both", pending: ctrl.Result{RequeueAfter: short}, asked: ctrl.Result{Requeue: true}, want: ctrl.Result{Requeue: true, RequeueAfter: short}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			g.Expect(SoonestRequeue(test.pending, test.asked)).To(gomega.Equal(test.want))
		})
	}
}

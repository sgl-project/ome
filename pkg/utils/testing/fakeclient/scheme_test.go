package fakeclient_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	schedulingv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func knows(s *runtime.Scheme, obj runtime.Object) bool {
	_, _, err := s.ObjectKinds(obj)
	return err == nil
}

func TestScheme_FreshScheme_KnowsBuiltinsAndOME(t *testing.T) {
	s := fakeclient.Scheme(t)
	for _, obj := range []runtime.Object{&corev1.Pod{}, &appsv1.Deployment{}, &v1beta1.InferenceService{}, &v1beta1.InferenceReplica{}} {
		if !knows(s, obj) {
			t.Errorf("%T is not registered", obj)
		}
	}
	if knows(s, &schedulingv1alpha1.PodGroup{}) {
		t.Error("PodGroup is registered without being asked for")
	}
}

func TestScheme_ExtraGroup_IsRegistered(t *testing.T) {
	s := fakeclient.Scheme(t, schedulingv1alpha1.AddToScheme)
	if !knows(s, &schedulingv1alpha1.PodGroup{}) {
		t.Error("PodGroup is not registered after its group was handed in")
	}
}

func TestScheme_TwoCalls_DoNotShareRegistrations(t *testing.T) {
	_ = fakeclient.Scheme(t, schedulingv1alpha1.AddToScheme)
	if knows(fakeclient.Scheme(t), &schedulingv1alpha1.PodGroup{}) {
		t.Error("a later scheme knows a group only an earlier one registered")
	}
}

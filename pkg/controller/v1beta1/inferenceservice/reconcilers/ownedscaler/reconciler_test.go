package ownedscaler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

type hpa = autoscalingv2.HorizontalPodAutoscaler

// testKind is the HPA with a creatable rule the cases can switch off.
func testKind(creatable bool) Kind[*hpa] {
	return Kind[*hpa]{
		Name:      "HorizontalPodAutoscaler",
		Class:     constants.AutoscalerClassHPA,
		Log:       logf.Log.WithName("test"),
		New:       func() *hpa { return &hpa{} },
		Creatable: func(*hpa) bool { return creatable },
		SpecEqual: func(desired, existing *hpa) bool { return equality.Semantic.DeepEqual(desired.Spec, existing.Spec) },
		CopySpec:  func(from, to *hpa) { to.Spec = from.Spec },
	}
}

func object(maxReplicas int32, class string) *hpa {
	obj := &hpa{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-engine",
			Namespace: "test-ns",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "ome.io/v1beta1", Kind: "InferenceService", Name: "demo", UID: "isvc-uid", Controller: ptr.To(true),
			}},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{MaxReplicas: maxReplicas},
	}
	if class != "" {
		obj.Annotations = map[string]string{constants.AutoscalerClass: class}
	}
	return obj
}

func TestReconcileDecidesByTheLiveObjectAndTheDesiredClass(t *testing.T) {
	terminating := object(3, "")
	terminating.Finalizers = []string{"example.com/finalizer"}
	terminating.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}

	tests := []struct {
		name      string
		live      *hpa
		desired   *hpa
		creatable bool
		wantLive  *int32 // MaxReplicas of the object after the pass; nil when none exists
		wantWrite bool
		wantErr   string
	}{
		{name: "creates a missing object when no class is named", desired: object(5, ""), creatable: true, wantLive: ptr.To(int32(5)), wantWrite: true},
		{name: "creates a missing object when the class is the kind's", desired: object(5, "hpa"), creatable: true, wantLive: ptr.To(int32(5)), wantWrite: true},
		{name: "leaves a missing object missing when the class names another kind", desired: object(5, "keda"), creatable: true},
		{name: "leaves a missing object missing when the kind rules it out", desired: object(5, ""), creatable: false},
		{name: "writes nothing when the live object matches", live: object(5, ""), desired: object(5, ""), creatable: true, wantLive: ptr.To(int32(5))},
		{name: "updates a live object that differs", live: object(3, ""), desired: object(5, ""), creatable: true, wantLive: ptr.To(int32(5)), wantWrite: true},
		{name: "deletes the live object when the class is external", live: object(3, ""), desired: object(5, "external"), creatable: true, wantWrite: true},
		{name: "refuses a live object that is being deleted", live: terminating, desired: object(5, ""), creatable: true, wantLive: ptr.To(int32(3)), wantErr: "is terminating"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []fakeclient.Option
			if tt.live != nil {
				opts = append(opts, fakeclient.WithObjects(tt.live))
			}
			cl := fakeclient.New(t, opts...)
			var liveVersion string
			if tt.live != nil {
				before := &hpa{}
				require.NoError(t, cl.Get(context.Background(), client.ObjectKeyFromObject(tt.live), before))
				liveVersion = before.ResourceVersion
			}

			err := New(cl, testKind(tt.creatable), tt.desired).Reconcile(context.Background())
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			got := &hpa{}
			err = cl.Get(context.Background(), client.ObjectKeyFromObject(tt.desired), got)
			if tt.wantLive == nil {
				assert.True(t, apierrors.IsNotFound(err), "expected no object, got err=%v", err)
				assert.Equal(t, tt.wantWrite, tt.live != nil, "a delete is the only write that leaves no object")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, *tt.wantLive, got.Spec.MaxReplicas)
			assert.Equal(t, tt.wantWrite, got.ResourceVersion != liveVersion, "resourceVersion before %q, after %q", liveVersion, got.ResourceVersion)
		})
	}
}

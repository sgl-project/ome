package autoscaler

import (
	"context"
	"testing"

	kedav1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

func rawDispatchISVC() *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "test-ns",
			UID:       types.UID("isvc-uid-12345"),
		},
	}
}

func rawDispatchComponentMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      "demo-engine",
		Namespace: "test-ns",
	}
}

func rawDispatchOwner() metav1.OwnerReference {
	return *metav1.NewControllerRef(rawDispatchISVC(), v1beta1.SchemeGroupVersion.WithKind("InferenceService"))
}

func rawDispatchComponentExt() *v1beta1.ComponentExtensionSpec {
	return &v1beta1.ComponentExtensionSpec{
		MinReplicas: ptr.To(2),
		MaxReplicas: 7,
	}
}

func rawDispatchHPAAutoscaler() *v1beta1.ComponentAutoscaler {
	return &v1beta1.ComponentAutoscaler{
		Class: v1beta1.AutoscalerHPA,
		HPA: &v1beta1.HPAAutoscaler{
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: ptr.To(int32(65)),
						},
					},
				},
			},
		},
	}
}

func rawDispatchKEDAAutoscaler() *v1beta1.ComponentAutoscaler {
	return &v1beta1.ComponentAutoscaler{
		Class: v1beta1.AutoscalerKEDA,
		Keda: &v1beta1.KedaAutoscaler{
			Triggers: []kedav1.ScaleTriggers{
				{
					Type: "cron",
					Metadata: map[string]string{
						"timezone":        "UTC",
						"start":           "0 9 * * *",
						"end":             "0 17 * * *",
						"desiredReplicas": "3",
					},
				},
			},
		},
	}
}

// TestDispatchForRawComponent_BranchesAndTransitions covers every typed class
// plus the stale-resource cleanup required for class transitions.
func TestDispatchForRawComponent_BranchesAndTransitions(t *testing.T) {
	componentMeta := rawDispatchComponentMeta()
	tests := []struct {
		name       string
		autoscaler *v1beta1.ComponentAutoscaler
		preHPA     bool
		preSO      bool
		wantHPA    bool
		wantSO     bool
	}{
		{
			name:       "typed HPA creates Deployment-targeted HPA",
			autoscaler: rawDispatchHPAAutoscaler(),
			wantHPA:    true,
		},
		{
			name:       "typed KEDA creates Deployment-targeted ScaledObject",
			autoscaler: rawDispatchKEDAAutoscaler(),
			wantSO:     true,
		},
		{
			name:       "HPA to KEDA deletes stale HPA",
			autoscaler: rawDispatchKEDAAutoscaler(),
			preHPA:     true,
			wantSO:     true,
		},
		{
			name:       "KEDA to HPA deletes stale ScaledObject",
			autoscaler: rawDispatchHPAAutoscaler(),
			preSO:      true,
			wantHPA:    true,
		},
		{
			name:       "HPA to None deletes stale HPA",
			autoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerNone},
			preHPA:     true,
		},
		{
			name:       "KEDA to None deletes stale ScaledObject",
			autoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerNone},
			preSO:      true,
		},
		{
			name:       "HPA to External deletes stale HPA",
			autoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerExternal},
			preHPA:     true,
		},
		{
			name:       "KEDA to External deletes stale ScaledObject",
			autoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerExternal},
			preSO:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			scheme := dispatchScheme(t)
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.preHPA {
				builder = builder.WithObjects(existingHPA(componentMeta.Namespace, componentMeta.Name, rawDispatchOwner()))
			}
			if tt.preSO {
				builder = builder.WithObjects(existingScaledObject(componentMeta.Namespace, componentMeta.Name, rawDispatchOwner()))
			}
			cl := builder.Build()

			err := DispatchForRawComponent(context.Background(), RawDispatchInput{
				Client:             cl,
				Scheme:             scheme,
				ISVC:               rawDispatchISVC(),
				ComponentMeta:      componentMeta,
				ResolvedAutoscaler: tt.autoscaler,
				ComponentExt:       rawDispatchComponentExt(),
			})
			require.NoError(t, err)

			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			hpaErr := cl.Get(context.Background(), types.NamespacedName{
				Namespace: componentMeta.Namespace,
				Name:      componentMeta.Name,
			}, hpa)
			if tt.wantHPA {
				require.NoError(t, hpaErr)
				assert.Equal(t, autoscalingv2.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       componentMeta.Name,
				}, hpa.Spec.ScaleTargetRef)
				require.NotNil(t, hpa.Spec.MinReplicas)
				assert.Equal(t, int32(2), *hpa.Spec.MinReplicas)
				assert.Equal(t, int32(7), hpa.Spec.MaxReplicas)
				assert.Equal(t, tt.autoscaler.HPA.Metrics, hpa.Spec.Metrics)
				assertRawAutoscalerOwner(t, hpa.OwnerReferences)
			} else {
				assert.True(t, apierrors.IsNotFound(hpaErr), "expected HPA to be absent, got %v", hpaErr)
			}

			so := &kedav1.ScaledObject{}
			soErr := cl.Get(context.Background(), types.NamespacedName{
				Namespace: componentMeta.Namespace,
				Name:      utils.GetScaledObjectName(componentMeta.Name),
			}, so)
			if tt.wantSO {
				require.NoError(t, soErr)
				require.NotNil(t, so.Spec.ScaleTargetRef)
				assert.Equal(t, appsv1.SchemeGroupVersion.String(), so.Spec.ScaleTargetRef.APIVersion)
				assert.Equal(t, "Deployment", so.Spec.ScaleTargetRef.Kind)
				assert.Equal(t, componentMeta.Name, so.Spec.ScaleTargetRef.Name)
				require.NotNil(t, so.Spec.MinReplicaCount)
				assert.Equal(t, int32(2), *so.Spec.MinReplicaCount)
				require.NotNil(t, so.Spec.MaxReplicaCount)
				assert.Equal(t, int32(7), *so.Spec.MaxReplicaCount)
				assert.Equal(t, tt.autoscaler.Keda.Triggers, so.Spec.Triggers)
				assertRawAutoscalerOwner(t, so.OwnerReferences)
			} else {
				assert.True(t, apierrors.IsNotFound(soErr), "expected ScaledObject to be absent, got %v", soErr)
			}
		})
	}
}

func TestDispatchForRawComponent_PropagatesDeepOwnedMetadata(t *testing.T) {
	const kedaControlAnnotation = "scaledobject.keda.sh/transfer-hpa-ownership"

	tests := []struct {
		name                string
		autoscaler          *v1beta1.ComponentAutoscaler
		wantAutoscalerClass string
	}{
		{
			name:                "HPA",
			autoscaler:          rawDispatchHPAAutoscaler(),
			wantAutoscalerClass: string(constants.AutoscalerClassHPA),
		},
		{
			name:                "typed KEDA overrides legacy HPA annotation",
			autoscaler:          rawDispatchKEDAAutoscaler(),
			wantAutoscalerClass: string(constants.AutoscalerClassKEDA),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			componentMeta := rawDispatchComponentMeta()
			componentMeta.Labels = map[string]string{
				constants.InferenceServicePodLabelKey: "demo",
				constants.OMEComponentLabel:           string(v1beta1.EngineComponent),
				"example.com/custom-label":            "label-value",
			}
			componentMeta.Annotations = map[string]string{
				constants.AutoscalerClass: string(constants.AutoscalerClassHPA),
				kedaControlAnnotation:     "true",
				"example.com/custom-note": "annotation-value",
			}

			var createdLabels, createdAnnotations map[string]string
			scheme := dispatchScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					createdLabels = cloneStringMap(obj.GetLabels())
					createdAnnotations = cloneStringMap(obj.GetAnnotations())

					labels := obj.GetLabels()
					if labels == nil {
						labels = map[string]string{}
					}
					labels["example.com/interceptor-label"] = "mutated"
					obj.SetLabels(labels)

					annotations := obj.GetAnnotations()
					if annotations == nil {
						annotations = map[string]string{}
					}
					annotations["example.com/interceptor-note"] = "mutated"
					obj.SetAnnotations(annotations)
					return c.Create(ctx, obj, opts...)
				},
			}).Build()

			require.NoError(t, DispatchForRawComponent(context.Background(), RawDispatchInput{
				Client:             cl,
				Scheme:             scheme,
				ISVC:               rawDispatchISVC(),
				ComponentMeta:      componentMeta,
				ResolvedAutoscaler: tt.autoscaler,
				ComponentExt:       rawDispatchComponentExt(),
			}))

			assert.Equal(t, componentMeta.Labels, createdLabels)
			expectedAnnotations := cloneStringMap(componentMeta.Annotations)
			expectedAnnotations[constants.AutoscalerClass] = tt.wantAutoscalerClass
			for key, value := range expectedAnnotations {
				assert.Equal(t, value, createdAnnotations[key])
			}
			assert.Len(t, createdAnnotations, len(expectedAnnotations)+1)
			assert.NotEmpty(t, createdAnnotations[constants.AutoscalerPropagatedMetadataKeys])
			assert.NotContains(t, componentMeta.Labels, "example.com/interceptor-label")
			assert.NotContains(t, componentMeta.Annotations, "example.com/interceptor-note")
		})
	}
}

func TestDispatchForIRComponent_PropagatesStableMetadata(t *testing.T) {
	const kedaControlAnnotation = "autoscaling.keda.sh/paused"
	labels := map[string]string{
		constants.InferenceServicePodLabelKey: "demo",
		constants.OMEComponentLabel:           string(v1beta1.EngineComponent),
		"example.com/custom-label":            "label-value",
	}
	annotations := map[string]string{
		kedaControlAnnotation:     "true",
		"example.com/custom-note": "annotation-value",
	}
	ir := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "demo-engine",
			Namespace:   "test-ns",
			UID:         types.UID("ir-uid-12345"),
			Labels:      labels,
			Annotations: map[string]string{"example.com/object-note": "ignore"},
		},
		Spec: v1beta1.InferenceReplicaSpec{
			Runners: []v1beta1.Runner{
				{
					Name: v1beta1.RunnerNameDefault,
					Size: 1,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Annotations: annotations},
					},
				},
			},
		},
	}

	var createdLabels, createdAnnotations map[string]string
	scheme := dispatchScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			createdLabels = cloneStringMap(obj.GetLabels())
			createdAnnotations = cloneStringMap(obj.GetAnnotations())
			got := obj.GetLabels()
			if got == nil {
				got = map[string]string{}
			}
			got["example.com/interceptor-label"] = "mutated"
			obj.SetLabels(got)
			return c.Create(ctx, obj, opts...)
		},
	}).Build()

	require.NoError(t, DispatchForIRComponent(context.Background(), IRDispatchInput{
		Client:             cl,
		Scheme:             scheme,
		IR:                 ir,
		ResolvedAutoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA},
		ComponentExt:       &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(1), MaxReplicas: 3},
	}))

	assert.Equal(t, labels, createdLabels)
	for key, value := range annotations {
		assert.Equal(t, value, createdAnnotations[key])
	}
	assert.Len(t, createdAnnotations, len(annotations)+1)
	assert.NotEmpty(t, createdAnnotations[constants.AutoscalerPropagatedMetadataKeys])
	assert.NotContains(t, createdAnnotations, "example.com/object-note")
	assert.NotContains(t, ir.Labels, "example.com/interceptor-label")
	assert.Equal(t, annotations, ir.Spec.Runners[0].Template.Annotations)
}

func TestDispatchForIRComponent_RawHandoffRetainsRunnerAnnotations(t *testing.T) {
	const (
		ns                    = "test-ns"
		isvcName              = "demo"
		name                  = "demo-engine"
		kedaControlAnnotation = "autoscaling.keda.sh/paused"
	)
	labels := dispatchManagementLabels(isvcName, v1beta1.EngineComponent)
	annotations := map[string]string{
		kedaControlAnnotation:     "true",
		"example.com/custom-note": "annotation-value",
	}

	tests := []struct {
		name       string
		autoscaler *v1beta1.ComponentAutoscaler
		resource   string
	}{
		{name: "HPA", autoscaler: rawDispatchHPAAutoscaler(), resource: "HPA"},
		{name: "KEDA", autoscaler: rawDispatchKEDAAutoscaler(), resource: "ScaledObject"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := dispatchScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()
			require.NoError(t, DispatchForRawComponent(context.Background(), RawDispatchInput{
				Client: cl,
				Scheme: scheme,
				ISVC:   rawDispatchISVC(),
				ComponentMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   ns,
					Labels:      cloneStringMap(labels),
					Annotations: cloneStringMap(annotations),
				},
				ResolvedAutoscaler: tt.autoscaler,
				ComponentExt:       rawDispatchComponentExt(),
			}))

			ir := dispatchModeBridgeIR(ns, isvcName, name)
			ir.Annotations = map[string]string{"example.com/object-note": "ignore"}
			ir.Spec.Runners = []v1beta1.Runner{
				{
					Name: v1beta1.RunnerNameDefault,
					Size: 1,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Annotations: cloneStringMap(annotations)},
					},
				},
			}
			require.NoError(t, cl.Create(context.Background(), ir))
			require.NoError(t, DispatchForIRComponent(context.Background(), IRDispatchInput{
				Client:             cl,
				Scheme:             scheme,
				IR:                 ir,
				ResolvedAutoscaler: tt.autoscaler,
				ComponentExt:       rawDispatchComponentExt(),
			}))

			var live client.Object
			key := types.NamespacedName{Namespace: ns, Name: name}
			if tt.resource == "HPA" {
				live = &autoscalingv2.HorizontalPodAutoscaler{}
			} else {
				live = &kedav1.ScaledObject{}
				key.Name = utils.GetScaledObjectName(name)
			}
			require.NoError(t, cl.Get(context.Background(), key, live))
			for key, value := range annotations {
				assert.Equal(t, value, live.GetAnnotations()[key])
			}
			assert.NotContains(t, live.GetAnnotations(), "example.com/object-note")
			assert.Equal(t, dispatchOwner(name), *metav1.GetControllerOf(live))
		})
	}
}

func TestIRRunnerAnnotations(t *testing.T) {
	tests := []struct {
		name string
		ir   *v1beta1.InferenceReplica
	}{
		{name: "nil IR"},
		{name: "no runners", ir: &v1beta1.InferenceReplica{}},
		{
			name: "runner without annotations",
			ir: &v1beta1.InferenceReplica{
				Spec: v1beta1.InferenceReplicaSpec{Runners: []v1beta1.Runner{{Name: v1beta1.RunnerNameDefault, Size: 1}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Nil(t, irRunnerAnnotations(tt.ir))
		})
	}

	source := map[string]string{"autoscaling.keda.sh/paused": "true"}
	ir := &v1beta1.InferenceReplica{
		Spec: v1beta1.InferenceReplicaSpec{
			Runners: []v1beta1.Runner{
				{
					Name: v1beta1.RunnerNameDefault,
					Size: 1,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Annotations: source},
					},
				},
			},
		},
	}
	got := irRunnerAnnotations(ir)
	assert.Equal(t, source, got)
	got["example.com/mutated"] = "true"
	assert.NotContains(t, source, "example.com/mutated")
}

func TestDispatchForIRComponent_TerminatingIRPreservesRawScalers(t *testing.T) {
	const (
		ns       = "test-ns"
		isvcName = "demo"
		name     = "demo-engine"
	)
	labels := dispatchManagementLabels(isvcName, v1beta1.EngineComponent)
	now := metav1.Now()
	ir := dispatchModeBridgeIR(ns, isvcName, name)
	ir.DeletionTimestamp = &now
	ir.Finalizers = []string{"example.com/finalizer"}

	tests := []struct {
		name       string
		autoscaler *v1beta1.ComponentAutoscaler
	}{
		{name: "HPA", autoscaler: rawDispatchHPAAutoscaler()},
		{name: "KEDA", autoscaler: rawDispatchKEDAAutoscaler()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hpa := existingHPA(ns, name, rawDispatchOwner())
			hpa.Labels = cloneStringMap(labels)
			so := existingScaledObject(ns, name, rawDispatchOwner())
			so.Labels = cloneStringMap(labels)
			scheme := dispatchScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ir.DeepCopy(), hpa, so).Build()

			err := DispatchForIRComponent(context.Background(), IRDispatchInput{
				Client:             cl,
				Scheme:             scheme,
				IR:                 ir.DeepCopy(),
				ResolvedAutoscaler: tt.autoscaler,
				ComponentExt:       &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(1), MaxReplicas: 5},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "terminating")

			gotHPA := &autoscalingv2.HorizontalPodAutoscaler{}
			require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, gotHPA))
			assert.Equal(t, rawDispatchOwner(), *metav1.GetControllerOf(gotHPA))
			gotSO := &kedav1.ScaledObject{}
			require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: utils.GetScaledObjectName(name)}, gotSO))
			assert.Equal(t, rawDispatchOwner(), *metav1.GetControllerOf(gotSO))
		})
	}
}

func TestDispatchForRawComponent_TakesOwnershipFromTerminatingIR(t *testing.T) {
	const (
		ns       = "test-ns"
		isvcName = "demo"
		name     = "demo-engine"
	)
	labels := dispatchManagementLabels(isvcName, v1beta1.EngineComponent)
	now := metav1.Now()
	ir := dispatchModeBridgeIR(ns, isvcName, name)
	ir.DeletionTimestamp = &now
	ir.Finalizers = []string{"example.com/finalizer"}

	tests := []struct {
		name       string
		autoscaler *v1beta1.ComponentAutoscaler
		resource   string
	}{
		{name: "HPA", autoscaler: rawDispatchHPAAutoscaler(), resource: "HPA"},
		{name: "KEDA", autoscaler: rawDispatchKEDAAutoscaler(), resource: "ScaledObject"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := dispatchScheme(t)
			builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ir.DeepCopy())
			if tt.resource == "HPA" {
				hpa := existingHPA(ns, name, dispatchOwner(name))
				hpa.Labels = cloneStringMap(labels)
				builder = builder.WithObjects(hpa)
			} else {
				so := existingScaledObject(ns, name, dispatchOwner(name))
				so.Labels = cloneStringMap(labels)
				builder = builder.WithObjects(so)
			}
			cl := builder.Build()

			require.NoError(t, DispatchForRawComponent(context.Background(), RawDispatchInput{
				Client: cl,
				Scheme: scheme,
				ISVC:   rawDispatchISVC(),
				ComponentMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
					Labels:    cloneStringMap(labels),
				},
				ResolvedAutoscaler: tt.autoscaler,
				ComponentExt:       rawDispatchComponentExt(),
			}))

			if tt.resource == "HPA" {
				got := &autoscalingv2.HorizontalPodAutoscaler{}
				require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, got))
				assert.Equal(t, rawDispatchOwner(), *metav1.GetControllerOf(got))
			} else {
				got := &kedav1.ScaledObject{}
				require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: utils.GetScaledObjectName(name)}, got))
				assert.Equal(t, rawDispatchOwner(), *metav1.GetControllerOf(got))
			}
		})
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func TestDispatchForRawComponent_ReplicaBounds(t *testing.T) {
	for _, tt := range []struct {
		name    string
		maximum int
		wantMax int32
	}{
		{name: "explicit maximum", maximum: 6, wantMax: 6},
		{name: "omitted maximum", wantMax: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := dispatchScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()
			componentMeta := rawDispatchComponentMeta()

			require.NoError(t, DispatchForRawComponent(context.Background(), RawDispatchInput{
				Client:             cl,
				Scheme:             scheme,
				ISVC:               rawDispatchISVC(),
				ComponentMeta:      componentMeta,
				ResolvedAutoscaler: rawDispatchKEDAAutoscaler(),
				ComponentExt: &v1beta1.ComponentExtensionSpec{
					MinReplicas: ptr.To(0),
					MaxReplicas: tt.maximum,
				},
			}))

			got := &kedav1.ScaledObject{}
			require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Namespace: componentMeta.Namespace, Name: utils.GetScaledObjectName(componentMeta.Name)}, got))
			require.NotNil(t, got.Spec.MinReplicaCount)
			require.NotNil(t, got.Spec.MaxReplicaCount)
			assert.Equal(t, int32(0), *got.Spec.MinReplicaCount)
			assert.Equal(t, tt.wantMax, *got.Spec.MaxReplicaCount)
		})
	}
}

func TestDispatchForRawComponent_DefaultsOmittedReplicaBounds(t *testing.T) {
	tests := []struct {
		name      string
		component *v1beta1.ComponentExtensionSpec
		wantMin   int32
		wantMax   int32
	}{
		{
			name:      "both bounds omitted",
			component: &v1beta1.ComponentExtensionSpec{},
			wantMin:   1,
			wantMax:   1,
		},
		{
			name:      "minimum omitted",
			component: &v1beta1.ComponentExtensionSpec{MaxReplicas: 4},
			wantMin:   1,
			wantMax:   4,
		},
		{
			name:      "maximum omitted",
			component: &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(2)},
			wantMin:   2,
			wantMax:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := dispatchScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()
			componentMeta := rawDispatchComponentMeta()

			require.NoError(t, DispatchForRawComponent(context.Background(), RawDispatchInput{
				Client:             cl,
				Scheme:             scheme,
				ISVC:               rawDispatchISVC(),
				ComponentMeta:      componentMeta,
				ResolvedAutoscaler: rawDispatchHPAAutoscaler(),
				ComponentExt:       tt.component,
			}))

			got := &autoscalingv2.HorizontalPodAutoscaler{}
			require.NoError(t, cl.Get(context.Background(), types.NamespacedName{
				Namespace: componentMeta.Namespace,
				Name:      componentMeta.Name,
			}, got))
			require.NotNil(t, got.Spec.MinReplicas)
			assert.Equal(t, tt.wantMin, *got.Spec.MinReplicas)
			assert.Equal(t, tt.wantMax, got.Spec.MaxReplicas)
		})
	}
}

func TestDispatchForRawComponent_InvalidReplicaBounds(t *testing.T) {
	tests := []struct {
		name       string
		autoscaler *v1beta1.ComponentAutoscaler
		component  *v1beta1.ComponentExtensionSpec
		wantError  string
	}{
		{
			name:       "nil component extension",
			autoscaler: rawDispatchHPAAutoscaler(),
			wantError:  "replica bounds are required",
		},
		{
			name:       "negative minimum",
			autoscaler: rawDispatchHPAAutoscaler(),
			component:  &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(-1), MaxReplicas: 3},
			wantError:  "minReplicas must be non-negative",
		},
		{
			name:       "negative maximum",
			autoscaler: rawDispatchHPAAutoscaler(),
			component:  &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(1), MaxReplicas: -1},
			wantError:  "maxReplicas must be non-negative",
		},
		{
			name:       "HPA zero minimum",
			autoscaler: rawDispatchHPAAutoscaler(),
			component:  &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(0), MaxReplicas: 3},
			wantError:  "minReplicas=0 requires typed KEDA",
		},
		{
			name:       "None zero minimum",
			autoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerNone},
			component:  &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(0), MaxReplicas: 3},
			wantError:  "minReplicas=0 requires typed KEDA",
		},
		{
			name:       "KEDA without typed configuration zero minimum",
			autoscaler: &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerKEDA},
			component:  &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(0), MaxReplicas: 3},
			wantError:  "minReplicas=0 requires typed KEDA",
		},
		{
			name:       "minimum exceeds maximum",
			autoscaler: rawDispatchHPAAutoscaler(),
			component:  &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(5), MaxReplicas: 3},
			wantError:  "minReplicas must not exceed maxReplicas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := dispatchScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()
			componentMeta := rawDispatchComponentMeta()

			err := DispatchForRawComponent(context.Background(), RawDispatchInput{
				Client:             cl,
				Scheme:             scheme,
				ISVC:               rawDispatchISVC(),
				ComponentMeta:      componentMeta,
				ResolvedAutoscaler: tt.autoscaler,
				ComponentExt:       tt.component,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)

			hpaErr := cl.Get(context.Background(), types.NamespacedName{Namespace: componentMeta.Namespace, Name: componentMeta.Name}, &autoscalingv2.HorizontalPodAutoscaler{})
			assert.True(t, apierrors.IsNotFound(hpaErr), "invalid bounds must not emit an HPA")
			soErr := cl.Get(context.Background(), types.NamespacedName{Namespace: componentMeta.Namespace, Name: utils.GetScaledObjectName(componentMeta.Name)}, &kedav1.ScaledObject{})
			assert.True(t, apierrors.IsNotFound(soErr), "invalid bounds must not emit a ScaledObject")
		})
	}
}

func assertRawAutoscalerOwner(t *testing.T, refs []metav1.OwnerReference) {
	t.Helper()
	require.Len(t, refs, 1)
	owner := refs[0]
	assert.Equal(t, v1beta1.SchemeGroupVersion.String(), owner.APIVersion)
	assert.Equal(t, "InferenceService", owner.Kind)
	assert.Equal(t, "demo", owner.Name)
	assert.Equal(t, types.UID("isvc-uid-12345"), owner.UID)
	require.NotNil(t, owner.Controller)
	assert.True(t, *owner.Controller)
}

// TestDispatchForRawComponent_Guards rejects incomplete owner input before
// shared dispatch can stamp an orphaned autoscaler.
func TestDispatchForRawComponent_Guards(t *testing.T) {
	scheme := dispatchScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	valid := RawDispatchInput{
		Client:             cl,
		Scheme:             scheme,
		ISVC:               rawDispatchISVC(),
		ComponentMeta:      rawDispatchComponentMeta(),
		ResolvedAutoscaler: rawDispatchHPAAutoscaler(),
		ComponentExt:       rawDispatchComponentExt(),
	}

	tests := []struct {
		name      string
		mutate    func(*RawDispatchInput)
		wantError string
	}{
		{
			name: "nil ISVC",
			mutate: func(input *RawDispatchInput) {
				input.ISVC = nil
			},
			wantError: "nil ISVC",
		},
		{
			name: "empty ISVC UID",
			mutate: func(input *RawDispatchInput) {
				input.ISVC = rawDispatchISVC()
				input.ISVC.UID = ""
			},
			wantError: "empty UID",
		},
		{
			name: "empty ISVC name",
			mutate: func(input *RawDispatchInput) {
				input.ISVC = rawDispatchISVC()
				input.ISVC.Name = ""
			},
			wantError: "empty ISVC name",
		},
		{
			name: "empty ISVC namespace",
			mutate: func(input *RawDispatchInput) {
				input.ISVC = rawDispatchISVC()
				input.ISVC.Namespace = ""
			},
			wantError: "empty ISVC namespace",
		},
		{
			name: "empty component name",
			mutate: func(input *RawDispatchInput) {
				input.ComponentMeta.Name = ""
			},
			wantError: "empty component name",
		},
		{
			name: "empty component namespace",
			mutate: func(input *RawDispatchInput) {
				input.ComponentMeta.Namespace = ""
			},
			wantError: "empty component namespace",
		},
		{
			name: "namespace mismatch",
			mutate: func(input *RawDispatchInput) {
				input.ComponentMeta.Namespace = "other-ns"
			},
			wantError: "namespace mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.mutate(&input)
			err := DispatchForRawComponent(context.Background(), input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

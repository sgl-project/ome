package components

import (
	"context"
	"errors"
	"testing"

	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/irprojector"
	isvcutils "sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/utils"
)

// An InferenceService annotation must reach the pods a Component renders, and a
// Component-level annotation must win over the service-level one. In OMENative
// mode the pod template lives on the InferenceReplica, so that is where the
// merged metadata has to land — an operator inspecting the IR sees exactly what
// the pods will carry.
func TestOMENativeProjectionCarriesMergedPodMetadata(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
	c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pd-demo",
			Namespace: "serving",
			UID:       types.UID("isvc-uid"),
			Annotations: map[string]string{
				"example.com/owner":      "team-inference",
				"example.com/tier":       "service-level",
				"example.com/slice-hint": "2x2x2",
			},
		},
	}
	engineSpec := &v1beta1.EngineSpec{
		ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
			MinReplicas: ptr.To(2),
			// Component-level annotation overrides the service-level value.
			Annotations: map[string]string{"example.com/tier": "engine-level"},
		},
		Worker:      &v1beta1.WorkerSpec{Size: ptr.To(3)},
		TopologyKey: ptr.To("cloud.example.com/slice-domain"),
	}

	engine := &Engine{
		BaseComponentFields: BaseComponentFields{
			Client:         c,
			Scheme:         scheme,
			Log:            log.Log,
			DeploymentMode: constants.OMENative,
		},
		engineSpec: engineSpec,
	}

	// The Component renders its pod metadata, then projects it onto the IR.
	objectMeta := metav1.ObjectMeta{
		Name:      "pd-demo-engine",
		Namespace: isvc.Namespace,
		Annotations: map[string]string{
			"example.com/owner":      "team-inference",
			"example.com/tier":       "engine-level",
			"example.com/slice-hint": "2x2x2",
		},
		Labels: map[string]string{constants.OMEComponentLabel: string(v1beta1.EngineComponent)},
	}
	podSpec := &v1.PodSpec{Containers: []v1.Container{{Name: "ome-container", Image: "example.com/vllm:demo"}}}
	workerPodSpec := &v1.PodSpec{Containers: []v1.Container{{Name: "worker", Image: "example.com/vllm:demo"}}}

	_, err := engine.reconcileDeployment(isvc, objectMeta, podSpec, 3, workerPodSpec)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	ir := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(), types.NamespacedName{
		Namespace: isvc.Namespace, Name: "pd-demo-engine",
	}, ir)).NotTo(gomega.HaveOccurred())

	g.Expect(ir.Spec.Component).To(gomega.Equal(v1beta1.EngineComponent))
	g.Expect(ir.Spec.Runners).NotTo(gomega.BeEmpty())
	g.Expect(ir.Spec.TopologyKey).To(gomega.Equal(ptr.To("cloud.example.com/slice-domain")),
		"the gang co-location key must reach the IR so worker->leader affinity can be derived")

	for _, runner := range ir.Spec.Runners {
		ann := runner.Template.ObjectMeta.Annotations
		g.Expect(ann).To(gomega.HaveKeyWithValue("example.com/owner", "team-inference"),
			"service-level annotation must reach runner %q", runner.Name)
		g.Expect(ann).To(gomega.HaveKeyWithValue("example.com/slice-hint", "2x2x2"),
			"slice hint must reach runner %q", runner.Name)
		g.Expect(ann).To(gomega.HaveKeyWithValue("example.com/tier", "engine-level"),
			"component-level annotation must win on runner %q", runner.Name)
	}
}

// The queue an InferenceService names as annotations must reach the pods as the
// LABELS Kueue admits on — an annotation-to-label rewrite, not a copy. If it
// stops happening the pods are created ungated and bypass quota entirely, which
// is the opposite of what queue admission is for.
func TestKueueQueueAnnotationsBecomePodLabels(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	meta := &metav1.ObjectMeta{
		Annotations: map[string]string{
			constants.DedicatedAICluster:   "accelerator-pool",
			constants.KueueEnabledLabelKey: "true",
		},
		Labels: map[string]string{},
	}
	isvcutils.SetPodLabelsFromAnnotations(meta)

	g.Expect(meta.Labels).To(gomega.HaveKeyWithValue(constants.KueueQueueLabelKey, "accelerator-pool"),
		"the queue annotation must become Kueue's queue label")
	g.Expect(meta.Labels).To(gomega.HaveKey(constants.KueueWorkloadPriorityClassLabelKey),
		"kueue-enabled must also carry a workload priority class")

	// Without kueue-enabled the same annotation is read as a Volcano queue, so
	// dropping it silently changes which system admits the workload.
	volcano := &metav1.ObjectMeta{
		Annotations: map[string]string{constants.DedicatedAICluster: "accelerator-pool"},
		Labels:      map[string]string{},
	}
	isvcutils.SetPodLabelsFromAnnotations(volcano)
	g.Expect(volcano.Labels).To(gomega.HaveKeyWithValue(constants.VolcanoQueueName, "accelerator-pool"))
	g.Expect(volcano.Labels).NotTo(gomega.HaveKey(constants.KueueQueueLabelKey))
}

// projectFor drives a Component's OMENative projection with the given spec
// overrides, so the same assertion can be made against engine and decoder. A fix
// applied to one Component and not the other must fail here.
func projectFor(component v1beta1.ComponentType, c client.Client, scheme *runtime.Scheme,
	isvc *v1beta1.InferenceService, ext v1beta1.ComponentExtensionSpec) (ctrl.Result, error) {
	base := BaseComponentFields{
		Client: c, Scheme: scheme, Log: log.Log, DeploymentMode: constants.OMENative,
	}
	podSpec := &v1.PodSpec{Containers: []v1.Container{{Name: "ome-container", Image: "example.com/x:1"}}}
	meta := metav1.ObjectMeta{
		Name:      irprojector.InferenceReplicaName(isvc.Name, component),
		Namespace: isvc.Namespace,
	}
	if component == v1beta1.DecoderComponent {
		d := &Decoder{BaseComponentFields: base, decoderSpec: &v1beta1.DecoderSpec{ComponentExtensionSpec: ext}}
		return d.reconcileDeployment(isvc, meta, podSpec, 0, nil)
	}
	e := &Engine{BaseComponentFields: base, engineSpec: &v1beta1.EngineSpec{ComponentExtensionSpec: ext}}
	return e.reconcileDeployment(isvc, meta, podSpec, 0, nil)
}

// The Component's autoscaler must reach the InferenceReplica. ir.Spec.Autoscaler
// is whole-block replace, so passing nil erases it — and the projector then reads
// the Component as unmanaged and rewrites ir.Spec.Replicas from minReplicas on
// every pass, fighting whatever scaled it.
func TestOMENativeProjectionCarriesComponentAutoscaler(t *testing.T) {
	for _, component := range []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent} {
		t.Run(string(component), func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

			isvc := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{
				Name: "auto-demo", Namespace: "serving", UID: types.UID("isvc-uid"),
			}}
			_, err := projectFor(component, c, scheme, isvc, v1beta1.ComponentExtensionSpec{
				MinReplicas: ptr.To(2),
				Autoscaler:  &v1beta1.ComponentAutoscaler{Class: v1beta1.AutoscalerHPA},
			})
			g.Expect(err).NotTo(gomega.HaveOccurred())

			ir := &v1beta1.InferenceReplica{}
			g.Expect(c.Get(context.Background(), types.NamespacedName{
				Namespace: isvc.Namespace,
				Name:      irprojector.InferenceReplicaName(isvc.Name, component),
			}, ir)).NotTo(gomega.HaveOccurred())
			g.Expect(ir.Spec.Autoscaler).NotTo(gomega.BeNil(), "autoscaler must not be erased")
			g.Expect(ir.Spec.Autoscaler.Class).To(gomega.Equal(v1beta1.AutoscalerHPA))
		})
	}
}

// A racing create surfaces as Conflict, which the projector documents as a benign
// requeue. Returned as an error it becomes an error hot-loop whenever the cache
// lags behind a create this controller just made.
func TestOMENativeProjectionConflictRequeuesInsteadOfErroring(t *testing.T) {
	for _, component := range []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent} {
		t.Run(string(component), func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())

			c := fakeclient.NewClientBuilder().WithScheme(scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						return apierrors.NewConflict(
							schema.GroupResource{Group: v1beta1.SchemeGroupVersion.Group, Resource: "inferencereplicas"},
							obj.GetName(), errors.New("racing create"))
					},
				}).Build()

			isvc := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{
				Name: "conflict-demo", Namespace: "serving", UID: types.UID("isvc-uid"),
			}}
			res, err := projectFor(component, c, scheme, isvc, v1beta1.ComponentExtensionSpec{
				MinReplicas: ptr.To(1),
			})
			g.Expect(err).NotTo(gomega.HaveOccurred(), "a Conflict must not surface as a reconcile error")
			g.Expect(res.Requeue).To(gomega.BeTrue(), "a Conflict must requeue")
		})
	}
}

// The projection is only useful if the metadata it carries is the metadata the
// Component actually renders, so this drives the real merge — reconcileObjectMeta,
// which is what Reconcile calls — and feeds its output to the projection, for both
// Components. Hand-building already-merged metadata would assert nothing about the
// merge itself.
func TestOMENativeProjectionUsesRenderedComponentMetadata(t *testing.T) {
	newISVC := func(name string) *v1beta1.InferenceService {
		return &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "serving", UID: types.UID("isvc-uid"),
			Annotations: map[string]string{
				"example.com/owner": "team-inference",
				"example.com/tier":  "service-level",
			},
		}}
	}
	podSpec := func() *v1.PodSpec {
		return &v1.PodSpec{Containers: []v1.Container{{Name: "ome-container", Image: "example.com/x:1"}}}
	}

	for _, tc := range []struct {
		name      string
		component v1beta1.ComponentType
		project   func(t *testing.T, c client.Client, scheme *runtime.Scheme, isvc *v1beta1.InferenceService) error
	}{
		{
			name:      "engine",
			component: v1beta1.EngineComponent,
			project: func(t *testing.T, c client.Client, scheme *runtime.Scheme, isvc *v1beta1.InferenceService) error {
				spec := &v1beta1.EngineSpec{ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: ptr.To(1),
					Annotations: map[string]string{"example.com/tier": "engine-level"},
				}}
				e := NewEngine(c, nil, scheme, &controllerconfig.InferenceServicesConfig{},
					constants.OMENative, nil, nil, spec, nil, "", nil, nil, "").(*Engine)
				meta, err := e.reconcileObjectMeta(isvc)
				if err != nil {
					return err
				}
				_, err = e.reconcileDeployment(isvc, meta, podSpec(), 0, nil)
				return err
			},
		},
		{
			name:      "decoder",
			component: v1beta1.DecoderComponent,
			project: func(t *testing.T, c client.Client, scheme *runtime.Scheme, isvc *v1beta1.InferenceService) error {
				spec := &v1beta1.DecoderSpec{ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
					MinReplicas: ptr.To(1),
					Annotations: map[string]string{"example.com/tier": "decoder-level"},
				}}
				d := NewDecoder(c, nil, scheme, &controllerconfig.InferenceServicesConfig{},
					constants.OMENative, nil, nil, spec, nil, "", nil, nil, "").(*Decoder)
				meta, err := d.reconcileObjectMeta(isvc)
				if err != nil {
					return err
				}
				_, err = d.reconcileDeployment(isvc, meta, podSpec(), 0, nil)
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(v1beta1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			g.Expect(v1.AddToScheme(scheme)).NotTo(gomega.HaveOccurred())
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()

			isvc := newISVC("merge-demo")
			g.Expect(tc.project(t, c, scheme, isvc)).NotTo(gomega.HaveOccurred())

			ir := &v1beta1.InferenceReplica{}
			g.Expect(c.Get(context.Background(), types.NamespacedName{
				Namespace: isvc.Namespace,
				Name:      irprojector.InferenceReplicaName(isvc.Name, tc.component),
			}, ir)).NotTo(gomega.HaveOccurred())
			g.Expect(ir.Spec.Runners).NotTo(gomega.BeEmpty())

			for _, runner := range ir.Spec.Runners {
				ann := runner.Template.ObjectMeta.Annotations
				g.Expect(ann).To(gomega.HaveKeyWithValue("example.com/owner", "team-inference"),
					"service annotation must survive the real merge onto runner %q", runner.Name)
				g.Expect(ann).To(gomega.HaveKeyWithValue("example.com/tier", string(tc.component)+"-level"),
					"Component annotation must win after the real merge on runner %q", runner.Name)
			}
		})
	}
}

package components

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
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

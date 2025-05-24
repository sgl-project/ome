package core

import (
	"context"
	"testing"

	testing2 "github.com/sgl-project/sgl-ome/pkg/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

func TestClusterTrainingRuntimeNewObjects(t *testing.T) {

	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	cases := map[string]struct {
		trainJob               *omev1beta1.TrainingJob
		clusterTrainingRuntime *omev1beta1.ClusterTrainingRuntime
		baseModelSpec          *omev1beta1.BaseModelSpec
		wantObjs               []client.Object
		wantError              error
	}{
		"succeeded to build PodGroup and JobSet with NumNodes from the Runtime and container from the Trainer.": {
			clusterTrainingRuntime: testing2.MakeClusterTrainingRuntimeWrapper("test-runtime").RuntimeSpec(
				testing2.MakeTrainingRuntimeSpecWrapper(testing2.MakeClusterTrainingRuntimeWrapper("test-runtime").Spec).
					InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					NumNodes(100).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					PodGroupPolicyCoschedulingSchedulingTimeout(120).
					Obj(),
			).Obj(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Suspend(true).
				UID("uid").
				SpecLabel(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				ModelConfig(testing2.MakeTrainJobModelConfigWrapper().
					InputModel("test-input-model").
					OutputModel(&omev1beta1.StorageSpec{}).
					Obj(),
				).
				HyperParameterTuningConfig(testing2.MakeTrainJobHyperparameterTuningConfigWrapper().
					TrainJobHyperparameterTuningConfig().
					Obj(),
				).
				Obj(),
			wantObjs: []client.Object{
				testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					InitContainerModelInitializerEnv([]corev1.EnvVar{
						{
							Name:  "MODEL_NAME",
							Value: "test-input-model",
						},
					}).
					NumNodes(100).
					Labels(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					ContainerTrainer("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Suspend(true).
					Volumes("meta").
					LabelsTrainer(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					Obj(),
				testing2.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "test-job").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					MinMember(101). // 101 replicas = 100 Trainer nodes + 1 Initializer.
					MinResources(corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("101"), // Every replica has 1 CPU = 101 CPUs in total.
					}).
					SchedulingTimeout(120).
					Obj(),
			},
		},
		"missing trainingRuntime resource": {
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.ClusterTrainingRuntimeKind), "test-runtime").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				Obj(),
			wantError: errorNotFoundSpecifiedClusterTrainingRuntime,
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortSlices(func(a, b client.Object) bool {
			return a.GetObjectKind().GroupVersionKind().String() < b.GetObjectKind().GroupVersionKind().String()
		}),
		cmpopts.EquateEmpty(),
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testing2.NewClientBuilder()
			if tc.clusterTrainingRuntime != nil {
				clientBuilder.WithObjects(tc.clusterTrainingRuntime)
			}

			trainingRuntime, err := NewTrainingRuntime(ctx, clientBuilder.Build(), testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			var ok bool
			trainingRuntimeFactory, ok = trainingRuntime.(*TrainingRuntime)
			if !ok {
				t.Fatal("Failed type assertion from Runtime interface to TrainingRuntime")
			}

			clTrainingRuntime, err := NewClusterTrainingRuntime(ctx, clientBuilder.Build(), testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			vendor := "meta"
			objs, err := clTrainingRuntime.NewObjects(ctx, tc.trainJob, &vendor)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantObjs, objs, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}

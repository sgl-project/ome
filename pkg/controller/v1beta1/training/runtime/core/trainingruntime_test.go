package core

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	testing2 "github.com/sgl-project/sgl-ome/pkg/testing"
	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTrainingRuntimeNewObjects(t *testing.T) {
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}

	// TODO: Add more test cases.
	cases := map[string]struct {
		trainingRuntime *omev1beta1.TrainingRuntime
		trainJob        *omev1beta1.TrainingJob
		wantObjs        []client.Object
		wantError       error
	}{
		// Test cases for the PlainML MLPolicy.
		"succeeded to build PodGroup and JobSet with NumNodes from the TrainJob and container from the Runtime.": {
			trainingRuntime: testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").
				Label("conflictLabel", "overridden").
				Annotation("conflictAnnotation", "overridden").
				RuntimeSpec(
					testing2.MakeTrainingRuntimeSpecWrapper(testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
						InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						NumNodes(100).
						ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
						PodGroupPolicyCoschedulingSchedulingTimeout(120).
						Obj(),
				).Obj(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				Suspend(true).
				UID("uid").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingRuntimeKind), "test-runtime").
				SpecLabel("conflictLabel", "override").
				SpecAnnotation("conflictAnnotation", "override").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						NumNodes(30).
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
					NumNodes(30).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					AnnotationsTrainer("conflictAnnotation", "override").
					LabelsTrainer("conflictLabel", "override").
					Suspend(true).
					Volumes("meta").
					InitContainerModelInitializerEnv([]corev1.EnvVar{
						{
							Name:  "MODEL_NAME",
							Value: "test-input-model",
						},
					}).
					Label("conflictLabel", "override").
					Annotation("conflictAnnotation", "override").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					Obj(),
				testing2.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "test-job").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					MinMember(31). // 31 replicas = 30 Trainer nodes + 1 Initializer.
					MinResources(corev1.ResourceList{
						// Every replica has 1 CPU = 31 CPUs in total.
						// Initializer uses InitContainers which execute sequentially.
						// Thus, the MinResources is equal to the maximum from the initContainer resources.
						corev1.ResourceCPU: resource.MustParse("31"),
					}).
					SchedulingTimeout(120).
					Obj(),
			},
		},
		"succeeded to build JobSet with NumNodes from the Runtime and container from the TrainJob.": {
			trainingRuntime: testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testing2.MakeTrainingRuntimeSpecWrapper(testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					NumNodes(100).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerTrainerEnv(
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						},
					).
					Obj(),
			).Obj(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						ContainerEnv(
							[]corev1.EnvVar{
								{
									Name:  "TRAIN_JOB",
									Value: "override",
								},
								{
									Name:  "TRAIN_JOB_CUSTOM",
									Value: "test:trainjob",
								},
							},
						).
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
					NumNodes(100).
					ContainerTrainer("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					ContainerTrainerEnv(
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "override",
							},
							{
								Name:  "TRAIN_JOB_CUSTOM",
								Value: "test:trainjob",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
							{
								Name:  constants.TrainingPathPrefixEnvVarKey,
								Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
							},
							{
								Name:  constants.TrainingBaselineModelEnvVarKey,
								Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
							},
						},
					).
					InitContainerModelInitializerEnv([]corev1.EnvVar{
						{
							Name:  "MODEL_NAME",
							Value: "test-input-model",
						},
					}).
					Volumes("meta").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					Obj(),
			},
		},
		"succeeded to build JobSet with dataset and model initializer from the TrainJob.": {
			trainingRuntime: testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testing2.MakeTrainingRuntimeSpecWrapper(testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					NumNodes(100).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				DatasetConfig(
					testing2.MakeTrainJobDatasetConfigWrapper().
						StorageUri("hf://trainjob-dataset").
						StorageKey("dataset-key").
						Parameters(map[string]string{
							"param1": "value1",
						}).
						Obj(),
				).
				ModelConfig(
					testing2.MakeTrainJobModelConfigWrapper().
						InputModel("test-input-model").
						OutputModel(&omev1beta1.StorageSpec{
							StorageUri: ptr.String("hf://output-model"),
							StorageKey: ptr.String("model-key"),
							Parameters: &map[string]string{
								"param1": "value1",
							},
						}).
						Obj(),
				).
				HyperParameterTuningConfig(testing2.MakeTrainJobHyperparameterTuningConfigWrapper().
					TrainJobHyperparameterTuningConfig().
					Obj(),
				).
				Obj(),
			wantObjs: []client.Object{
				testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					NumNodes(100).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					InitContainerDatasetModelInitializer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					InitContainerDatasetInitializerEnv([]corev1.EnvVar{
						{
							Name:  "STORAGE_URI",
							Value: "hf://trainjob-dataset",
						},
						{
							Name:  "param1",
							Value: "value1",
						},
						{
							Name: "STORAGE_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "dataset-key",
									},
									Key: "key",
								},
							},
						},
					}).
					InitContainerModelInitializerEnv([]corev1.EnvVar{
						{
							Name:  "STORAGE_URI",
							Value: "hf://output-model",
						},
						{
							Name:  "MODEL_NAME",
							Value: "test-input-model",
						},
						{
							Name:  "param1",
							Value: "value1",
						},
						{
							Name: "STORAGE_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "model-key",
									},
									Key: "key",
								},
							},
						},
					}).
					Volumes("meta").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					Obj(),
			},
		},
		// Test cases for the Torch MLPolicy.
		"succeeded to build JobSet with Torch values from the TrainJob": {
			trainingRuntime: testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testing2.MakeTrainingRuntimeSpecWrapper(testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					TorchPolicy(100, "auto").
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					Obj(),
			).Obj(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						NumNodes(30).
						NumProcPerNode("3").
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
					NumNodes(30).
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					ContainerTrainerEnv(
						[]corev1.EnvVar{
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "30",
							},
							{
								Name:  constants.TorchEnvNumProcPerNode,
								Value: "3",
							},
							{
								Name: constants.TorchEnvNodeRank,
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: constants.JobCompletionIndexFieldPath,
									},
								},
							},
							{
								Name:  constants.TorchEnvMasterAddr,
								Value: fmt.Sprintf("test-job-%s-0-0.test-job", constants.JobTrainerNode),
							},
							{
								Name:  constants.TorchEnvMasterPort,
								Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
							},
							{
								Name:  constants.TrainingPathPrefixEnvVarKey,
								Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
							},
							{
								Name:  constants.TrainingBaselineModelEnvVarKey,
								Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
							},
						},
					).
					InitContainerModelInitializerEnv([]corev1.EnvVar{
						{
							Name:  "MODEL_NAME",
							Value: "test-input-model",
						},
					}).
					Volumes("meta").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					Obj(),
			},
		},
		"succeeded to build JobSet with Torch values from the Runtime and envs.": {
			trainingRuntime: testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").RuntimeSpec(
				testing2.MakeTrainingRuntimeSpecWrapper(testing2.MakeTrainingRuntimeWrapper(metav1.NamespaceDefault, "test-runtime").Spec).
					TorchPolicy(100, "auto").
					ContainerTrainer("test:runtime", []string{"runtime"}, []string{"runtime"}, resRequests).
					ContainerTrainerEnv(
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "original",
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
						},
					).
					Obj(),
			).Obj(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingRuntimeKind), "test-runtime").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						ContainerEnv(
							[]corev1.EnvVar{
								{
									Name:  "TRAIN_JOB",
									Value: "override",
								},
								{
									Name:  "TRAIN_JOB_CUSTOM",
									Value: "test:trainjob",
								},
							},
						).
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
					NumNodes(100).
					ContainerTrainer("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					ContainerTrainerPorts([]corev1.ContainerPort{{ContainerPort: constants.ContainerTrainerPort}}).
					ContainerTrainerEnv(
						[]corev1.EnvVar{
							{
								Name:  "TRAIN_JOB",
								Value: "override",
							},
							{
								Name:  "TRAIN_JOB_CUSTOM",
								Value: "test:trainjob",
							},
							{
								Name:  constants.TorchEnvNumNodes,
								Value: "100",
							},
							{
								Name:  constants.TorchEnvNumProcPerNode,
								Value: "auto",
							},
							{
								Name: constants.TorchEnvNodeRank,
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: constants.JobCompletionIndexFieldPath,
									},
								},
							},
							{
								Name:  constants.TorchEnvMasterAddr,
								Value: fmt.Sprintf("test-job-%s-0-0.test-job", constants.JobTrainerNode),
							},
							{
								Name:  constants.TorchEnvMasterPort,
								Value: fmt.Sprintf("%d", constants.ContainerTrainerPort),
							},
							{
								Name:  "RUNTIME",
								Value: "test:runtime",
							},
							{
								Name:  constants.TrainingPathPrefixEnvVarKey,
								Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
							},
							{
								Name:  constants.TrainingBaselineModelEnvVarKey,
								Value: filepath.Join(constants.CohereStorePathPrefix, "t-job"),
							},
						},
					).
					InitContainerModelInitializerEnv([]corev1.EnvVar{
						{
							Name:  "MODEL_NAME",
							Value: "test-input-model",
						},
					}).
					Volumes("meta").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingJobKind), "test-job", "uid").
					Obj(),
			},
		},
		// Failed test cases.
		"missing trainingRuntime resource": {
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job-3").
				UID("uid").
				RuntimeRef(omev1beta1.SchemeGroupVersion.WithKind(omev1beta1.TrainingRuntimeKind), "test-runtime-3").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						Obj(),
				).
				Obj(),
			wantError: errorNotFoundSpecifiedTrainingRuntime,
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
			if tc.trainingRuntime != nil {
				clientBuilder.WithObjects(tc.trainingRuntime)
			}

			trainingRuntime, err := NewTrainingRuntime(ctx, clientBuilder.Build(), testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			vendor := "meta"
			objs, err := trainingRuntime.NewObjects(ctx, tc.trainJob, &vendor)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantObjs, objs, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}

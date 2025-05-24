package kueueworkload

import (
	"context"
	"testing"

	"fmt"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/dac/utils"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	TestImage                             = "test-reservation-image:latest"
	TestCreationFailedTimeThresholdSecond = 30
)

func TestDeploymentReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	// Create ConfigMap for tests
	rawReservationJob := []byte(`{
		"image": "%s",
		"creationFailedTimeThresholdSecond": %d,
		"schedulerName": "%s"
	}`)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DedicatedAIClusterConfigMapName,
			Namespace: constants.OMENamespace,
		},
		Data: map[string]string{
			"reservationJob": fmt.Sprintf(string(rawReservationJob), TestImage, TestCreationFailedTimeThresholdSecond, constants.CustomSchedulerName),
		},
	}

	tests := []struct {
		name               string
		namespace          string
		resources          *corev1.ResourceRequirements
		affinity           *corev1.Affinity
		count              int
		existingDeployment *appsv1.Deployment
		wantErr            bool
		expectedDeployment *appsv1.Deployment
		expectedUpdate     bool
	}{
		{
			name:      "successfully create new Deployment",
			namespace: "test-ns",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("60Gi"),
					corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("1"),
				},
			},
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "beta.kubernetes.io/instance-type",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"BM.GPU.H100.8"},
									},
								},
							},
						},
					},
				},
			},
			count:              2,
			wantErr:            false,
			expectedDeployment: createTestDeployment("test-ns", 2, "BM.GPU.H100.8", "1"),
			expectedUpdate:     false,
		},
		{
			name:      "update existing Deployment",
			namespace: "existing-ns",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("30"),
					corev1.ResourceMemory: resource.MustParse("160Gi"),
					corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("2"),
				},
			},
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "beta.kubernetes.io/instance-type",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"BM.GPU.A100-v2.8"},
									},
								},
							},
						},
					},
				},
			},
			count: 2,
			existingDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.DACMainTaskName,
					Namespace: "existing-ns",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: utils.GetInt32Pointer(1),
				},
			},
			expectedDeployment: createTestDeployment("existing-ns", 2, "BM.GPU.A100-v2.8", "2"),
			expectedUpdate:     true,
		},
		{
			name:      "deployment already exists with same spec",
			namespace: "same-ns",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10"),
					corev1.ResourceMemory: resource.MustParse("200Gi"),
					corev1.ResourceName(constants.NvidiaGPUResourceType): resource.MustParse("4"),
				},
			},
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "beta.kubernetes.io/instance-type",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"BM.GPU.H100.8"},
									},
								},
							},
						},
					},
				},
			},
			count:              1,
			existingDeployment: createTestDeployment("same-ns", 1, "BM.GPU.H100.8", "4"),
			expectedDeployment: createTestDeployment("same-ns", 1, "BM.GPU.H100.8", "4"),
			expectedUpdate:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clients
			fakeClient := fakectrl.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			fakeClientset := fake.NewSimpleClientset(configMap)

			// If testing existing deployment case, create it first
			if tt.existingDeployment != nil {
				err := fakeClient.Create(context.TODO(), tt.existingDeployment)
				assert.NoError(t, err)
			}

			// Create reconciler
			reconciler, err := NewDeploymentReconciler(
				fakeClient,
				fakeClientset,
				scheme,
				tt.namespace,
				tt.resources,
				tt.affinity,
				tt.count,
			)
			assert.NoError(t, err)

			// Perform reconciliation
			result, err := reconciler.Reconcile()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Verify Deployment was created/updated correctly
			createdDeployment := &appsv1.Deployment{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{
				Name:      constants.DACMainTaskName,
				Namespace: tt.namespace,
			}, createdDeployment)
			assert.NoError(t, err)

			if tt.expectedDeployment != nil {
				// Verify specific fields
				assert.Equal(t, tt.expectedDeployment.Name, createdDeployment.Name)
				assert.Equal(t, tt.expectedDeployment.Namespace, createdDeployment.Namespace)
				assert.Equal(t, tt.expectedDeployment.Spec.Replicas, createdDeployment.Spec.Replicas)
				assert.Equal(t, tt.expectedDeployment.Spec.Template.Spec.Containers[0].Resources,
					createdDeployment.Spec.Template.Spec.Containers[0].Resources)
				assert.Equal(t, constants.CustomSchedulerName, createdDeployment.Spec.Template.Spec.SchedulerName)

				// Verify labels
				assert.Equal(t, tt.namespace, createdDeployment.Labels[constants.KueueQueueLabelKey])
				assert.Equal(t, constants.DedicatedAiClusterReservationWorkloadPriorityClass,
					createdDeployment.Labels[constants.KueueWorkloadPriorityClassLabelKey])
			}

			if tt.expectedUpdate {
				// Verify the deployment was updated
				assert.NotEqual(t, tt.existingDeployment.Spec, createdDeployment.Spec)
				// Verify last update time annotation was added
				_, hasAnnotation := createdDeployment.Annotations[constants.DACLastUpdateTimeAnnotationKey]
				assert.True(t, hasAnnotation)
			}
		})
	}
}

func createTestDeployment(namespace string, replicas int, affinityGPUShape string, gpuRequest string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DACMainTaskName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.KueueQueueLabelKey:                 namespace,
				constants.KueueWorkloadPriorityClassLabelKey: constants.DedicatedAiClusterReservationWorkloadPriorityClass,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": constants.DACMainTaskName,
				},
			},
			Replicas: utils.GetInt32Pointer(replicas),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": constants.DACMainTaskName,
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "beta.kubernetes.io/instance-type",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{affinityGPUShape},
											},
										},
									},
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &constants.DACReservationJobTerminationGracePeriodSeconds,
					SchedulerName:                 constants.CustomSchedulerName,
					Containers: []corev1.Container{
						{
							Name:  constants.DACMainTaskName,
							Image: TestImage,
							Command: []string{
								"/bin/bash",
							},
							Args: []string{
								"-c",
								"/bin/sleep infinity",
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									constants.NvidiaGPUResourceType: resource.MustParse(gpuRequest),
								},
								Limits: corev1.ResourceList{
									constants.NvidiaGPUResourceType: resource.MustParse(gpuRequest),
								},
							},
						},
					},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: utils.GetPointerOfIntOrString(0),
					MaxSurge:       utils.GetPointerOfIntOrString(1),
				},
			},
		},
	}
}

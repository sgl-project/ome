package runtime

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/sgl-project/sgl-ome/pkg/constants"
)

func TestNewInfo(t *testing.T) {

	cases := map[string]struct {
		infoOpts []InfoOption
		wantInfo *Info
	}{
		"all arguments are specified": {
			infoOpts: []InfoOption{
				WithLabels(map[string]string{
					"labelKey": "labelValue",
				}),
				WithVolumes([]corev1.Volume{
					{
						Name: "test-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}),
				WithAffinity(&corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{},
				}),
				WithAnnotations(map[string]string{
					"annotationKey": "annotationValue",
				}),
				WithPodSpecReplicas(constants.JobInitializer, 1, corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("5"),
							},
						},
						RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
					}},
					Containers: []corev1.Container{{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("10"),
							},
						},
					}},
				}),
				WithPodSpecReplicas(constants.JobTrainerNode, 10, corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							},
						},
						RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
					}},
					Containers: []corev1.Container{{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("25"),
							},
						},
					}},
				}),
			},
			wantInfo: &Info{
				Labels: map[string]string{
					"labelKey": "labelValue",
				},
				Annotations: map[string]string{
					"annotationKey": "annotationValue",
				},
				Trainer: Trainer{
					Volumes: []corev1.Volume{
						{
							Name: "test-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{},
					},
				},
				Scheduler: &Scheduler{
					TotalRequests: map[string]TotalResourceRequest{
						constants.JobInitializer: {
							Replicas: 1,
							PodRequests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("15"),
							},
						},
						constants.JobTrainerNode: {
							Replicas: 10,
							PodRequests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("40"),
							},
						},
					},
				},
			},
		},
		"all arguments are not specified": {
			wantInfo: &Info{Scheduler: &Scheduler{TotalRequests: map[string]TotalResourceRequest{}}},
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
		cmpopts.EquateEmpty(),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			info := NewInfo(tc.infoOpts...)
			if diff := cmp.Diff(tc.wantInfo, info, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected runtime.Info (-want,+got):\n%s", diff)
			}
		})
	}
}

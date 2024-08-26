package raycluster

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sort"
	"strconv"
	"strings"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis/serving/v1beta1"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/controller/v1beta1/inferenceservice/utils"
	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	rayutils "github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("RayClusterReconciler")

type RayReconciler struct {
	client        client.Client
	scheme        *runtime.Scheme
	componentExt  *v1beta1.ComponentExtensionSpec
	podSpec       *corev1.PodSpec
	RayClusters   []*ray.RayCluster
	componentMeta *metav1.ObjectMeta
}

func NewRayReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec) *RayReconciler {
	return &RayReconciler{
		client:        client,
		scheme:        scheme,
		componentMeta: &componentMeta,
		componentExt:  componentExt,
		podSpec:       podSpec,
	}
}

func (r *RayReconciler) Reconcile() ([]*ray.RayCluster, error) {
	existingRayClusters, err := r.listExistingRayClusters()
	if err != nil {
		return nil, err
	}

	r.sortRayClustersByIndex(existingRayClusters)

	for i := 0; i < int(*r.componentExt.MinReplicas); i++ {
		if err := r.reconcileRayCluster(i, existingRayClusters); err != nil {
			return nil, err
		}
	}

	if err := r.deleteExtraRayClusters(existingRayClusters); err != nil {
		return nil, err
	}

	return r.RayClusters, nil
}

func (r *RayReconciler) listExistingRayClusters() (*ray.RayClusterList, error) {
	existingRayClusters := &ray.RayClusterList{}
	labelSelector := client.MatchingLabels(r.componentMeta.Labels)
	if err := r.client.List(context.TODO(), existingRayClusters, client.InNamespace(r.componentMeta.Namespace), labelSelector); err != nil {
		return nil, err
	}
	return existingRayClusters, nil
}

func (r *RayReconciler) sortRayClustersByIndex(existingRayClusters *ray.RayClusterList) {
	sort.SliceStable(existingRayClusters.Items, func(i, j int) bool {
		iIndex, _ := extractClusterIndex(existingRayClusters.Items[i].Name)
		jIndex, _ := extractClusterIndex(existingRayClusters.Items[j].Name)
		return iIndex < jIndex
	})
}

func (r *RayReconciler) reconcileRayCluster(index int, existingRayClusters *ray.RayClusterList) error {
	desired := createRayCluster(r.componentMeta, r.podSpec, index)
	existing := &ray.RayCluster{}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		log.Info("Reconciling Ray cluster", "namespace", desired.Namespace, "name", desired.Name)
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing); err != nil {
			if apierr.IsNotFound(err) {
				log.Info("Creating Ray cluster", "namespace", desired.Namespace, "name", desired.Name)
				return r.client.Create(context.TODO(), desired)
			}
			return err
		}

		desired.ResourceVersion = existing.ResourceVersion
		if err := reconcileRayCluster(desired, existing); err != nil {
			return err
		}
		return r.client.Update(context.TODO(), existing)
	})
	if err != nil {
		return err
	}
	r.RayClusters = append(r.RayClusters, desired)
	return nil
}

func (r *RayReconciler) deleteExtraRayClusters(existingRayClusters *ray.RayClusterList) error {
	for _, existingCluster := range existingRayClusters.Items {
		clusterIndex, err := extractClusterIndex(existingCluster.Name)
		if err != nil {
			log.Error(err, "Failed to extract index from cluster name", "name", existingCluster.Name)
			continue
		}
		if clusterIndex >= int(*r.componentExt.MinReplicas) {
			log.Info("Deleting extra Ray cluster", "namespace", existingCluster.Namespace, "name", existingCluster.Name)
			if err := r.client.Delete(context.TODO(), &existingCluster); err != nil {
				log.Error(err, "Failed to delete Ray cluster", "name", existingCluster.Name)
			}
		}
	}
	return nil
}

func createRayCluster(meta *metav1.ObjectMeta, spec *corev1.PodSpec, index int) *ray.RayCluster {
	clusterName := fmt.Sprintf("%s-%d", meta.Name, index)
	annotations := meta.GetAnnotations()

	utils.SetPodLabelsFromAnnotations(meta)
	workerReplicas := int32(constants.DefaultMinReplicas)

	setLifecycleHooks(spec)
	workerPodSpec := deepCopyWorkerPodSpec(spec)

	return &ray.RayCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        clusterName,
			Namespace:   meta.Namespace,
			Labels:      meta.Labels,
			Annotations: annotations,
		},
		Spec: ray.RayClusterSpec{
			HeadGroupSpec: ray.HeadGroupSpec{
				HeadService: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        rayutils.CheckName(clusterName),
						Namespace:   meta.Namespace,
						Labels:      meta.Labels,
						Annotations: annotations,
					},
				},
				RayStartParams: map[string]string{
					"dashboard-host":      "0.0.0.0",
					"metrics-export-port": "8000",
				},
				ServiceType: corev1.ServiceTypeClusterIP,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      meta.Labels,
						Name:        clusterName,
						Annotations: annotations,
					},
					Spec: *spec,
				},
			},
			WorkerGroupSpecs: []ray.WorkerGroupSpec{
				{
					GroupName:      "wg",
					RayStartParams: map[string]string{},
					Replicas:       &workerReplicas,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      meta.Labels,
							Name:        clusterName,
							Annotations: annotations,
						},
						Spec: *workerPodSpec,
					},
				},
			},
		},
	}
}

func setLifecycleHooks(spec *corev1.PodSpec) {
	for i := range spec.Containers {
		if spec.Containers[i].Lifecycle == nil {
			spec.Containers[i].Lifecycle = &corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"/bin/bash", "-lc", "ray stop"},
					},
				},
			}
		}
	}
}

func deepCopyWorkerPodSpec(spec *corev1.PodSpec) *corev1.PodSpec {
	workerPodSpec := spec.DeepCopy()
	for i := range workerPodSpec.Containers {
		workerPodSpec.Containers[i].Command = []string{"/bin/bash", "-lc", "--"}
		workerPodSpec.Containers[i].Args = []string{"ulimit -n 65536; echo worker; $KUBERAY_GEN_RAY_START_CMD"}
	}
	return workerPodSpec
}

func extractClusterIndex(name string) (int, error) {
	parts := strings.Split(name, "-")
	indexStr := parts[len(parts)-1]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return -1, fmt.Errorf("failed to parse index from name: %s", name)
	}
	return index, nil
}

func reconcileRayCluster(desired *ray.RayCluster, existing *ray.RayCluster) error {
	if semanticEquals(desired, existing) {
		return nil
	}
	existing.Spec = desired.Spec
	existing.ObjectMeta.Labels = desired.ObjectMeta.Labels
	existing.ObjectMeta.Annotations = desired.ObjectMeta.Annotations
	return nil
}

func semanticEquals(desiredCluster, cluster *ray.RayCluster) bool {
	return equality.Semantic.DeepEqual(desiredCluster.Spec, cluster.Spec) &&
		equality.Semantic.DeepEqual(desiredCluster.ObjectMeta.Labels, cluster.ObjectMeta.Labels) &&
		equality.Semantic.DeepEqual(desiredCluster.ObjectMeta.Annotations, cluster.ObjectMeta.Annotations)
}

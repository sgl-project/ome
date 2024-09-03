package raycluster

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sort"
	"strconv"
	"strings"
	"time"

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
	client               client.Client
	scheme               *runtime.Scheme
	componentExt         *v1beta1.ComponentExtensionSpec
	podSpec              *corev1.PodSpec
	RayClusters          []*ray.RayCluster
	componentMeta        *metav1.ObjectMeta
	unavailableThreshold time.Duration
}

func NewRayReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec,
	podSpec *corev1.PodSpec,
	unavailableThreshold time.Duration) *RayReconciler {

	rayClusters := make([]*ray.RayCluster, 0, int(*componentExt.MinReplicas))
	for i := 0; i < int(*componentExt.MinReplicas); i++ {
		rayCluster := createRayCluster(&componentMeta, podSpec, i)
		rayClusters = append(rayClusters, rayCluster)
	}

	return &RayReconciler{
		client:               client,
		scheme:               scheme,
		componentMeta:        &componentMeta,
		componentExt:         componentExt,
		RayClusters:          rayClusters,
		podSpec:              podSpec,
		unavailableThreshold: unavailableThreshold,
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
	desired := r.RayClusters[index]
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

		// Check the health of the mnp deployment
		mnpName := fmt.Sprintf("%s-mnp", desired.Name)
		if shouldRecreate, err := r.isMNPDeploymentUnavailable(existing, mnpName, r.unavailableThreshold); err != nil {
			return err
		} else if shouldRecreate {
			log.Info("MNP deployment is unavailable, recreating Ray cluster", "namespace", desired.Namespace, "name", desired.Name)
			if err := r.client.Delete(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to delete Ray cluster", "name", existing.Name)
				return err
			}

			// Reset the unavailable-since annotation in the desired object before creation
			if desired.Annotations == nil {
				desired.Annotations = make(map[string]string)
			}
			delete(desired.Annotations, constants.RayClusterUnavailableSince)

			// Create the RayCluster
			if err := r.client.Create(context.TODO(), desired); err != nil {
				return err
			}
		}

		// Continue with standard reconciliation
		desired.ResourceVersion = existing.ResourceVersion

		// Preserve existing annotations
		if existing.Annotations != nil {
			if desired.Annotations == nil {
				desired.Annotations = make(map[string]string)
			}
			for k, v := range existing.Annotations {
				if _, exists := desired.Annotations[k]; !exists {
					desired.Annotations[k] = v
				}
			}
		}

		if err := reconcileRayCluster(desired, existing); err != nil {
			return err
		}
		return r.client.Update(context.TODO(), desired)
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *RayReconciler) isMNPDeploymentUnavailable(rayCluster *ray.RayCluster, mnpName string, threshold time.Duration) (bool, error) {
	deployment := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: mnpName, Namespace: rayCluster.Namespace}, deployment)
	if err != nil {
		if apierr.IsNotFound(err) {
			log.Error(err, "MNP deployment not found", "name", mnpName)
			return true, nil
		}
		log.Error(err, "Failed to get MNP deployment", "name", mnpName)
		return false, err
	}

	if rayCluster.Status.AvailableWorkerReplicas > 0 && rayCluster.Status.State == ray.Ready {
		if deployment.Status.UnavailableReplicas > 0 {
			// Fetch or set the 'unavailable-since' annotation
			if rayCluster.Annotations == nil {
				rayCluster.Annotations = make(map[string]string)
			}

			unavailableSince, exists := rayCluster.Annotations[constants.RayClusterUnavailableSince]
			if !exists {
				// If the annotation doesn't exist, set it to the current time
				unavailableSince = time.Now().Format(time.RFC3339)
				rayCluster.Annotations[constants.RayClusterUnavailableSince] = unavailableSince
				// Update the RayCluster with the new annotation
				if err := r.client.Update(context.TODO(), rayCluster); err != nil {
					log.Error(err, "Failed to update RayCluster with unavailable-since annotation", "name", rayCluster.Name)
					return false, err
				}
				log.Info("MNP deployment became unavailable", "name", mnpName, "unavailable_since", unavailableSince)
			} else {
				// Parse the timestamp from the annotation
				unavailableSinceTime, err := time.Parse(time.RFC3339, unavailableSince)
				if err != nil {
					log.Error(err, "Failed to parse unavailable-since annotation", "name", rayCluster.Name, "unavailable_since", unavailableSince)
					return false, err
				}

				// Calculate how long the deployment has been unavailable
				unavailableDuration := time.Since(unavailableSinceTime)
				log.Info("MNP deployment is unavailable", "name", mnpName, "duration", unavailableDuration)

				// Check if the duration exceeds the threshold
				if unavailableDuration > threshold {
					log.Info("MNP deployment has been unavailable for too long", "name", mnpName, "duration", unavailableDuration, "threshold", threshold)
					return true, nil
				}
			}
		} else {
			// Clear the 'unavailable-since' annotation if the deployment is available
			if _, exists := rayCluster.Annotations[constants.RayClusterUnavailableSince]; exists {
				delete(rayCluster.Annotations, constants.RayClusterUnavailableSince)
				if err := r.client.Update(context.TODO(), rayCluster); err != nil {
					log.Error(err, "Failed to clear unavailable-since annotation", "name", rayCluster.Name)
					return false, err
				}
				log.Info("MNP deployment is now available, clearing unavailable-since annotation", "name", mnpName)
			}
		}
	} else {
		log.Info("RayCluster is not yet ready or does not have available worker replicas, skipping annotation update", "name", rayCluster.Name)
	}

	return false, nil
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

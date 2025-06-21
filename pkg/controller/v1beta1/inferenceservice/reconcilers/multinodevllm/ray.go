package multinodevllm

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ray "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	rayutils "github.com/ray-project/kuberay/ray-operator/controllers/ray/utils"
	"github.com/sgl-project/ome/pkg/constants"
	"github.com/sgl-project/ome/pkg/controller/v1beta1/inferenceservice/utils"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
)

var log = logf.Log.WithName("MultiNodeVLLMReconciler")

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

func (r *RayReconciler) Reconcile() ([]*ray.RayCluster, ctrl.Result, error) {
	// List existing Ray clusters
	existingRayClusters, err := r.listExistingRayClusters()
	if err != nil {
		return nil, ctrl.Result{}, err
	}

	// Sort Ray clusters by index for deterministic processing
	r.sortRayClustersByIndex(existingRayClusters)

	// Reconcile each Ray cluster based on MinReplicas
	for i := 0; i < int(*r.componentExt.MinReplicas); i++ {
		result, err := r.reconcileRayCluster(i, existingRayClusters)
		if err != nil {
			return nil, result, err // Return the result and requeue as needed based on the reconcileRayCluster logic
		}
	}

	// Delete any extra Ray clusters beyond MinReplicas
	if err := r.deleteExtraRayClusters(existingRayClusters); err != nil {
		return nil, ctrl.Result{}, err
	}

	return r.RayClusters, ctrl.Result{}, nil
}

func (r *RayReconciler) listExistingRayClusters() (*ray.RayClusterList, error) {
	existingRayClusters := &ray.RayClusterList{}
	labelSelector := client.MatchingLabels(r.componentMeta.Labels)
	err := r.client.List(context.TODO(), existingRayClusters, client.InNamespace(r.componentMeta.Namespace), labelSelector)
	return existingRayClusters, err
}

func (r *RayReconciler) sortRayClustersByIndex(existingRayClusters *ray.RayClusterList) {
	sort.SliceStable(existingRayClusters.Items, func(i, j int) bool {
		iIndex, _ := extractClusterIndex(existingRayClusters.Items[i].Name)
		jIndex, _ := extractClusterIndex(existingRayClusters.Items[j].Name)
		return iIndex < jIndex
	})
}

func (r *RayReconciler) reconcileRayCluster(index int, existingRayClusters *ray.RayClusterList) (ctrl.Result, error) {
	desired := r.RayClusters[index]
	existing := &ray.RayCluster{}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		log.Info("Reconciling Ray cluster", "namespace", desired.Namespace, "name", desired.Name)
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing); err != nil {
			if apierr.IsNotFound(err) {
				log.Info("Creating Ray cluster", "namespace", desired.Namespace, "name", desired.Name)
				resetUnavailableSinceAnnotation(desired)
				return r.client.Create(context.TODO(), desired)
			}
			return err
		}

		mnpName := rayutils.CheckName(fmt.Sprintf("%s-mnp", desired.Name))
		if shouldRecreate, err := r.isMNPDeploymentUnavailable(existing, mnpName); err != nil {
			return err
		} else if shouldRecreate {
			log.Info("Recreating Ray cluster due to unavailable MNP deployment", "namespace", desired.Namespace, "name", desired.Name)

			// Ensure the annotation is reset on the existing cluster before deletion
			resetUnavailableSinceAnnotation(existing)
			if err := r.client.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to clear annotation on existing Ray cluster", "namespace", existing.Namespace, "name", existing.Name)
				return err
			}

			if err := r.client.Delete(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to delete Ray cluster", "namespace", existing.Namespace, "name", existing.Name)
				return err
			}

			resetUnavailableSinceAnnotation(desired)

			if err := r.client.Create(context.TODO(), desired); err != nil {
				return err
			}

			log.Info("Ray cluster recreated successfully", "namespace", desired.Namespace, "name", desired.Name)
		}

		desired.ResourceVersion = existing.ResourceVersion
		preserveAnnotations(desired, existing)

		if err := reconcileRayCluster(desired, existing); err != nil {
			return err
		}
		return r.client.Update(context.TODO(), desired)
	})

	// If MNP deployment is still not ready, requeue the reconciliation
	if err == nil {
		mnpName := rayutils.CheckName(fmt.Sprintf("%s-mnp", desired.Name))
		if stillUnavailable, err := r.isMNPDeploymentUnavailable(existing, mnpName); err != nil {
			log.Error(err, "Failed to check MNP deployment status", "namespace", existing.Namespace, "name", mnpName)
			return ctrl.Result{}, err
		} else if stillUnavailable {
			log.Info("MNP deployment still not ready, requeuing after threshold", "namespace", desired.Namespace, "name", desired.Name)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	return ctrl.Result{}, err
}

func (r *RayReconciler) isMNPDeploymentUnavailable(rayCluster *ray.RayCluster, mnpName string) (bool, error) {
	deployment := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: mnpName, Namespace: rayCluster.Namespace}, deployment)
	if err != nil {
		if apierr.IsNotFound(err) {
			log.Info("MNP deployment not found", "namespace", rayCluster.Namespace, "name", mnpName)
			return true, nil
		}
		log.Error(err, "Failed to get MNP deployment", "namespace", rayCluster.Namespace, "name", mnpName)
		return false, err
	}

	if rayCluster.Status.AvailableWorkerReplicas > 0 && isRayClusterReady(rayCluster) {
		if deployment.Status.UnavailableReplicas > 0 {
			return r.handleUnavailableMNP(rayCluster, mnpName)
		}
		return r.clearUnavailableSinceAnnotation(rayCluster, mnpName)
	}
	log.Info("RayCluster is not ready or has no available worker replicas, skipping annotation update", "namespace", rayCluster.Namespace, "name", rayCluster.Name)
	return false, nil
}

func (r *RayReconciler) handleUnavailableMNP(rayCluster *ray.RayCluster, mnpName string) (bool, error) {
	unavailableSince, exists := rayCluster.Annotations[constants.RayClusterUnavailableSince]
	if !exists {
		unavailableSince = time.Now().Format(time.RFC3339)
		rayCluster.Annotations[constants.RayClusterUnavailableSince] = unavailableSince
		if err := r.client.Update(context.TODO(), rayCluster); err != nil {
			log.Error(err, "Failed to update RayCluster with unavailable-since annotation", "namespace", rayCluster.Namespace, "name", rayCluster.Name)
			return false, err
		}
		log.Info("MNP deployment became unavailable", "namespace", rayCluster.Namespace, "name", mnpName, "unavailable_since", unavailableSince)
	} else {
		unavailableSinceTime, err := time.Parse(time.RFC3339, unavailableSince)
		if err != nil {
			log.Error(err, "Failed to parse unavailable-since annotation", "namespace", rayCluster.Namespace, "name", rayCluster.Name, "unavailable_since", unavailableSince)
			return false, err
		}

		unavailableDuration := time.Since(unavailableSinceTime)
		log.Info("MNP deployment is still unavailable", "namespace", rayCluster.Namespace, "name", mnpName, "duration", unavailableDuration)

		if unavailableDuration > r.unavailableThreshold {
			log.Info("MNP deployment has been unavailable for too long", "namespace", rayCluster.Namespace, "name", mnpName, "duration", unavailableDuration, "threshold", r.unavailableThreshold)
			return true, nil
		}
	}
	return false, nil
}

func (r *RayReconciler) clearUnavailableSinceAnnotation(rayCluster *ray.RayCluster, mnpName string) (bool, error) {
	if _, exists := rayCluster.Annotations[constants.RayClusterUnavailableSince]; exists {
		delete(rayCluster.Annotations, constants.RayClusterUnavailableSince)
		if err := r.client.Update(context.TODO(), rayCluster); err != nil {
			log.Error(err, "Failed to clear unavailable-since annotation", "namespace", rayCluster.Namespace, "name", rayCluster.Name)
			return false, err
		}
		log.Info("MNP deployment is now available, clearing unavailable-since annotation", "namespace", rayCluster.Namespace, "name", mnpName)
	}
	return false, nil
}

func (r *RayReconciler) deleteExtraRayClusters(existingRayClusters *ray.RayClusterList) error {
	for _, existingCluster := range existingRayClusters.Items {
		clusterIndex, err := extractClusterIndex(existingCluster.Name)
		if err != nil {
			log.Error(err, "Failed to extract index from cluster name", "namespace", existingCluster.Namespace, "name", existingCluster.Name)
			continue
		}
		if clusterIndex >= int(*r.componentExt.MinReplicas) {
			log.Info("Deleting extra Ray cluster", "namespace", existingCluster.Namespace, "name", existingCluster.Name)
			if err := r.client.Delete(context.TODO(), &existingCluster); err != nil {
				log.Error(err, "Failed to delete Ray cluster", "namespace", existingCluster.Namespace, "name", existingCluster.Name)
			}
		}
	}
	return nil
}

func createRayCluster(meta *metav1.ObjectMeta, spec *corev1.PodSpec, index int) *ray.RayCluster {
	clusterName := fmt.Sprintf("%s-%d", meta.Name, index)

	utils.SetPodLabelsFromAnnotations(meta)
	workerReplicas := int32(constants.DefaultMinReplicas)

	setLifecycleHooks(spec)
	workerPodSpec := deepCopyWorkerPodSpec(spec)

	return &ray.RayCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        clusterName,
			Namespace:   meta.Namespace,
			Labels:      meta.Labels,
			Annotations: meta.GetAnnotations(),
		},
		Spec: ray.RayClusterSpec{
			HeadGroupSpec: ray.HeadGroupSpec{
				HeadService: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        constants.DefaultRayHeadServiceName(meta.Name, index),
						Namespace:   meta.Namespace,
						Labels:      meta.Labels,
						Annotations: meta.GetAnnotations(),
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
						Annotations: meta.GetAnnotations(),
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
							Annotations: meta.GetAnnotations(),
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
	return strconv.Atoi(indexStr)
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

func resetUnavailableSinceAnnotation(rayCluster *ray.RayCluster) {
	if rayCluster.Annotations != nil {
		delete(rayCluster.Annotations, constants.RayClusterUnavailableSince)
	}
}

func preserveAnnotations(desired, existing *ray.RayCluster) {
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
}

// isRayClusterReady checks if the RayCluster is in Ready state by examining its conditions
func isRayClusterReady(rayCluster *ray.RayCluster) bool {
	for _, condition := range rayCluster.Status.Conditions {
		if string(condition.Type) == string(ray.Ready) && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

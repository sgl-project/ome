package ocicache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	ocirediscluster "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ociredis"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"github.com/go-logr/logr"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/redis"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=ome.io,resources=ocicaches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=ocicaches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=ocicaches/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch;watch

var (
	newRedisClient = func(cfg *ocirediscluster.Config) (*ocirediscluster.OciRedisClient, error) {
		return ocirediscluster.NewOciRedisClient(cfg)
	}
)

const redisClusterFinalizerName = "rediscluster/finalizer"

type CacheReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Log      logr.Logger
}

func (r *CacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("RedisCluster", req.NamespacedName)
	ociRedisCluster := &v1beta1.OciCache{}
	if err := r.Get(ctx, req.NamespacedName, ociRedisCluster); err != nil {
		// IgnoreNotFound tells controller-runtime not to requeue for deleted CRs
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("reconcile OciRedisCluster", "namespace", req.Namespace, "name", req.Name)

	// create ociRedisClient with a proper logger adapter
	ociRedisConfig, err := ocirediscluster.NewConfig(
		ocirediscluster.WithAnotherLog(logging.ForLogr(r.Log)),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create ociredisConfig config: %w", err)
	}

	ociRedisConfig.AuthType = Ptr(principals.AuthenticationType(ociRedisCluster.Spec.ClusterSpec.AuthType))
	redisClient, err := newRedisClient(ociRedisConfig)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create redis client: %w", err)
	}

	if !ociRedisCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, ociRedisCluster, redisClient)
	}

	if addRedisFinalizerIfNeeded(ociRedisCluster) {
		if err := r.Update(ctx, ociRedisCluster); err != nil {
			log.Error(err, "Failed to add finalizer", "name", ociRedisCluster.Name)
			r.Recorder.Event(ociRedisCluster, corev1.EventTypeWarning, "FinalizerAddFailed", "Failed to add finalizer")
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// requeue to continue normal flow
		return ctrl.Result{Requeue: true}, nil
	}
	// If we don't have a Redis cluster yet, create it
	if ociRedisCluster.Status.CacheClusterId == "" {
		log.Info("Creating new Redis cluster", "dbName", ociRedisCluster.Name)
		newID, cerr := r.createRedisCluster(ctx, ociRedisCluster, log, redisClient, string(ociRedisCluster.UID))

		if cerr != nil {
			log.Error(cerr, "Failed to create redis cluster", "name", ociRedisCluster.Name)
			return ctrl.Result{}, cerr
		}
		log.Info("Created redis cluster", "cluterId", newID, "clusterName", ociRedisCluster.Name)

		// Update status with the new ID
		ociRedisCluster.Status.CacheClusterId = newID
		ociRedisCluster.Status.LifecycleState = "CREATING"
		if uerr := r.Status().Update(ctx, ociRedisCluster); uerr != nil {
			log.Error(uerr, "Failed to update status", "name", ociRedisCluster.Name)
			r.Recorder.Event(ociRedisCluster, corev1.EventTypeWarning, "UpdateStatusFailed", "Failed to update status")
			return ctrl.Result{}, uerr
		}
		// Requeue to poll state transition
		return ctrl.Result{Requeue: true}, nil
	}

	// Redis cluster exists — poll its lifecycle and fetch connection details
	log.Info("Checking available Redis Cluster", "clusterId", ociRedisCluster.Status.CacheClusterId)

	getResponse, gerr := redisClient.GetRedisCluster(ctx, redis.GetRedisClusterRequest{
		RedisClusterId: common.String(ociRedisCluster.Status.CacheClusterId),
	})
	if gerr != nil {
		if apierr.IsNotFound(gerr) {
			log.Info("Redis cluster not found by ID; will recreate", "redisClusterId", ociRedisCluster.Status.CacheClusterId)
			ociRedisCluster.Status.CacheClusterId = ""
			if uerr := r.Status().Update(ctx, ociRedisCluster); uerr != nil {
				return ctrl.Result{}, uerr
			}
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(gerr, "Failed to getResponse redis cluster", "name", ociRedisCluster.Name)
		r.Recorder.Event(ociRedisCluster, corev1.EventTypeWarning, "GetRedisClusterFailed", "Failed to getResponse redis cluster")
		return ctrl.Result{}, fmt.Errorf("getResponse redis cluster: %w", gerr)
	}

	state := getResponse.LifecycleState
	log.Info("RedisCluster lifecycle", "redisClusterId", ociRedisCluster.Status.CacheClusterId, "state", state)

	switch state {
	case redis.RedisClusterLifecycleStateActive:
		ociRedisCluster.Status.LifecycleState = "READY"
		if uerr := r.Status().Update(ctx, ociRedisCluster); uerr != nil {
			return ctrl.Result{}, uerr
		}
		log.Info("OciRedisCluster is ready")
		return ctrl.Result{}, nil

	case redis.RedisClusterLifecycleStateCreating:
		ociRedisCluster.Status.LifecycleState = "CREATING"
		// Keep polling
		if uerr := r.Status().Update(ctx, ociRedisCluster); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	case redis.RedisClusterLifecycleStateUpdating:
		ociRedisCluster.Status.LifecycleState = "UPDATING"
		// Keep polling
		if uerr := r.Status().Update(ctx, ociRedisCluster); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	default:
		ociRedisCluster.Status.LifecycleState = "FAILED"
		_ = r.Status().Update(ctx, ociRedisCluster)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

func (r *CacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1beta1.OciCache{}).Complete(r)
}

func (r *CacheReconciler) createRedisCluster(
	ctx context.Context,
	OciRedisCluster *v1beta1.OciCache,
	log logr.Logger,
	redisClient *ocirediscluster.OciRedisClient,
	crUID string) (string, error) {
	compartmentID := OciRedisCluster.Spec.ClusterSpec.CompartmentId
	if compartmentID == "" {
		return "", fmt.Errorf("spec.compartmentId is required")
	}
	subnetId := OciRedisCluster.Spec.ClusterSpec.SubnetId
	if subnetId == "" {
		return "", fmt.Errorf("TargetSubnetOCID must be set")
	}

	displayName := OciRedisCluster.Name

	tags := map[string]string{
		"created-by": "redis-cluster-controller",
		"cr-uid":     crUID,
	}

	// parse CRD NodeMemoryInGBs string to float due to using floats in CRDs is discouraged
	v, err := strconv.ParseFloat(OciRedisCluster.Spec.ClusterSpec.NodeMemoryInGBs, 32)
	if err != nil {
		return "", fmt.Errorf("invalid spec.nodeMemoryInGBs: %w", err)
	}
	nodeMemoryInGBs := float32(v)

	details := redis.CreateRedisClusterDetails{
		DisplayName:     common.String(displayName),
		CompartmentId:   common.String(compartmentID),
		SubnetId:        common.String(subnetId),
		NodeCount:       common.Int(OciRedisCluster.Spec.ClusterSpec.NodeCount),
		SoftwareVersion: redis.RedisClusterSoftwareVersionEnum(OciRedisCluster.Spec.ClusterSpec.SoftwareVersion),
		NodeMemoryInGBs: common.Float32(nodeMemoryInGBs),
		FreeformTags:    tags,
	}
	token := hashRedisCreateDetails(details)
	req := redis.CreateRedisClusterRequest{
		CreateRedisClusterDetails: details,
		OpcRetryToken:             common.String(token),
	}
	log.Info("Creating Redis cluster in Progress with request", "request", req, "clusterName", OciRedisCluster.Name)
	resp, err := redisClient.CreateRedisCluster(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create redis cluster: %w", err)
	}
	return *resp.Id, nil
}

// hashRedisCreateDetails builds an idempotency token
func hashRedisCreateDetails(d redis.CreateRedisClusterDetails) string {
	h := sha256.New()
	if d.DisplayName != nil {
		h.Write([]byte("dn:" + *d.DisplayName))
	}
	if d.CompartmentId != nil {
		h.Write([]byte("cid:" + *d.CompartmentId))
	}
	if d.SubnetId != nil {
		h.Write([]byte("subnet:" + *d.SubnetId))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func addRedisFinalizerIfNeeded(cr *v1beta1.OciCache) bool {
	for _, f := range cr.Finalizers {
		if f == redisClusterFinalizerName {
			return false
		}
	}
	cr.Finalizers = append(cr.Finalizers, redisClusterFinalizerName)
	return true
}

func (r *CacheReconciler) reconcileDelete(
	ctx context.Context,
	log logr.Logger,
	cluster *v1beta1.OciCache,
	redisClient *ocirediscluster.OciRedisClient,
) (ctrl.Result, error) {
	// We only act if our finalizer is present
	hasFinalizer := false
	for _, f := range cluster.Finalizers {
		if f == redisClusterFinalizerName {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		// nothing to do
		return ctrl.Result{}, nil
	}

	// delete OCI Redis Cluster if exists
	if id := cluster.Status.CacheClusterId; id != "" {
		log.Info("Deleting OCI Redis Cluster", "redisClusterId", id)

		_, err := redisClient.DeleteRedisCluster(ctx, redis.DeleteRedisClusterRequest{
			RedisClusterId: common.String(id),
		})
		if err != nil {
			// transient or still deleting → requeue
			log.Error(err, "delete redis cluster failed; requeue", "redisClusterId", id)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
	}

	// remove finalizer so Kubernetes can remove the CR
	removeRedisFinalizer(cluster)
	if err := r.Update(ctx, cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	log.Info("Finalizer completed, CR can be removed")
	return ctrl.Result{}, nil
}

func removeRedisFinalizer(cr *v1beta1.OciCache) {
	fs := make([]string, 0, len(cr.Finalizers))
	for _, f := range cr.Finalizers {
		if f != redisClusterFinalizerName {
			fs = append(fs, f)
		}
	}
	cr.Finalizers = fs
}
func Ptr[T any](v T) *T { return &v }

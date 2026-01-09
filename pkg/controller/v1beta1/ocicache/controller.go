package ocicache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	PasswordUtil "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/ocipostgres/utils"
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

const (
	redisSecretKeyUsername    = "username"
	redisSecretKeyPassword    = "password"
	redisSecretKeyACL         = "acl"
	redisClusterFinalizerName = "rediscluster/finalizer"
)

type CacheReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Log      logr.Logger
}

type appRedisCreds struct {
	User       string
	Password   string
	ACL        string
	SecretName string
}

func (r *CacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("RedisCluster", req.NamespacedName)
	ociCache := &v1beta1.OciCache{}
	if err := r.Get(ctx, req.NamespacedName, ociCache); err != nil {
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

	ociRedisConfig.AuthType = Ptr(principals.AuthenticationType(ociCache.Spec.ClusterSpec.AuthType))
	redisClient, err := newRedisClient(ociRedisConfig)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create redis client: %w", err)
	}

	if !ociCache.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, ociCache, redisClient)
	}

	if addRedisFinalizerIfNeeded(ociCache) {
		if err := r.Update(ctx, ociCache); err != nil {
			log.Error(err, "Failed to add finalizer", "name", ociCache.Name)
			r.Recorder.Event(ociCache, corev1.EventTypeWarning, "FinalizerAddFailed", "Failed to add finalizer")
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// requeue to continue normal flow
		return ctrl.Result{Requeue: true}, nil
	}
	// If we don't have a Redis cluster yet, create it
	if ociCache.Status.CacheClusterId == "" {
		log.Info("Creating new Redis cluster", "dbName", ociCache.Name)
		newID, cerr := r.createRedisCluster(ctx, ociCache, log, redisClient, string(ociCache.UID))

		if cerr != nil {
			log.Error(cerr, "Failed to create redis cluster", "name", ociCache.Name)
			return ctrl.Result{}, cerr
		}
		log.Info("Created redis cluster", "cluterId", newID, "clusterName", ociCache.Name)

		// Update status with the new ID
		ociCache.Status.CacheClusterId = newID
		ociCache.Status.LifecycleState = "CREATING"
		if uerr := r.Status().Update(ctx, ociCache); uerr != nil {
			log.Error(uerr, "Failed to update status", "name", ociCache.Name)
			r.Recorder.Event(ociCache, corev1.EventTypeWarning, "UpdateStatusFailed", "Failed to update status")
			return ctrl.Result{}, uerr
		}
		// Requeue to poll state transition
		return ctrl.Result{Requeue: true}, nil
	}

	// Redis cluster exists — poll its lifecycle and fetch connection details
	log.Info("Checking available Redis Cluster", "clusterId", ociCache.Status.CacheClusterId)

	getResponse, gerr := redisClient.GetRedisCluster(ctx, redis.GetRedisClusterRequest{
		RedisClusterId: common.String(ociCache.Status.CacheClusterId),
	})
	if gerr != nil {
		if apierr.IsNotFound(gerr) {
			log.Info("Redis cluster not found by ID; will recreate", "redisClusterId", ociCache.Status.CacheClusterId)
			ociCache.Status.CacheClusterId = ""
			if uerr := r.Status().Update(ctx, ociCache); uerr != nil {
				return ctrl.Result{}, uerr
			}
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(gerr, "Failed to getResponse redis cluster", "name", ociCache.Name)
		r.Recorder.Event(ociCache, corev1.EventTypeWarning, "GetRedisClusterFailed", "Failed to getResponse redis cluster")
		return ctrl.Result{}, fmt.Errorf("getResponse redis cluster: %w", gerr)
	}

	state := getResponse.LifecycleState
	log.Info("RedisCluster lifecycle", "redisClusterId", ociCache.Status.CacheClusterId, "state", state)

	switch state {
	case redis.RedisClusterLifecycleStateActive:
		ociCache.Status.LifecycleState = "UPDATING"
		setCondition(
			&ociCache.Status,
			"Ready",
			metav1.ConditionFalse,
			"CacheUsersReconciling",
			"Redis cluster is active, but cache users are still reconciling",
			ociCache.Generation,
		)
		ociCache.Status.PrimaryFqdn = *getResponse.PrimaryFqdn
		ociCache.Status.PrimaryEndpointIpAddress = *getResponse.PrimaryEndpointIpAddress
		if uerr := r.Status().Update(ctx, ociCache); uerr != nil {
			return ctrl.Result{}, uerr
		}
		log.Info("OciRedisCluster is ready, reconciling cache users")

		if err := r.reconcileCacheUsers(ctx, log, ociCache, redisClient); err != nil {
			log.Error(err, "failed to reconcile cache users")
			// Keep polling; users may not be ready yet
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		ociCache.Status.LifecycleState = "READY"
		setCondition(
			&ociCache.Status,
			"Ready",
			metav1.ConditionTrue,
			"AllCacheUsersReady",
			"Redis cluster and all required cache users are ready",
			ociCache.Generation,
		)
		if uerr := r.Status().Update(ctx, ociCache); uerr != nil {
			return ctrl.Result{}, uerr
		}
		log.Info("Redis cache users reconciled")
		return ctrl.Result{}, nil

	case redis.RedisClusterLifecycleStateCreating:
		ociCache.Status.LifecycleState = "CREATING"
		// Keep polling
		setCondition(
			&ociCache.Status,
			"Ready",
			metav1.ConditionFalse,
			"RedisClusterCreating",
			"Redis cluster is being created",
			ociCache.Generation,
		)
		if uerr := r.Status().Update(ctx, ociCache); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	case redis.RedisClusterLifecycleStateUpdating:
		ociCache.Status.LifecycleState = "UPDATING"
		// Keep polling
		setCondition(&ociCache.Status, "Ready", metav1.ConditionFalse,
			"RedisClusterUpdating", "Redis cluster is in updating stage", ociCache.Generation)
		if uerr := r.Status().Update(ctx, ociCache); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	default:
		ociCache.Status.LifecycleState = "FAILED"
		setCondition(
			&ociCache.Status,
			"Ready",
			metav1.ConditionFalse,
			"RedisClusterNotReady",
			"RedisClusterNotReady",
			ociCache.Generation,
		)
		_ = r.Status().Update(ctx, ociCache)
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

	updated := false
	newUsers := make([]v1beta1.CacheUserStatus, 0, len(cluster.Status.CacheUsers))

	// Clean up cache users (detach, delete user, delete secret)
	for _, u := range cluster.Status.CacheUsers {
		user := u

		// detach if still marked attached
		if user.IsUserAttached && user.CacheUserId != "" && cluster.Status.CacheClusterId != "" {
			log.Info("Detaching cache user from Redis cluster during cluster delete",
				"clusterId", cluster.Status.CacheClusterId,
				"userId", user.CacheUserId,
				"user", user.CacheUserName)

			if err := detachRedisUserFromCluster(ctx, redisClient, cluster.Status.CacheClusterId, user.CacheUserId, log); err != nil {
				log.Error(err, "detach cache user from cluster failed; requeue",
					"clusterId", cluster.Status.CacheClusterId,
					"userId", user.CacheUserId,
					"user", user.CacheUserName)
				// keep user in status for next retry
				newUsers = append(newUsers, user)
				if updated {
					cluster.Status.CacheUsers = newUsers
					_ = r.Status().Update(ctx, cluster)
				}
				return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
			}
			// detach succeeded
			user.IsUserAttached = false
			updated = true
		}

		// delete OCI cache user
		if user.CacheUserId != "" {
			cacheUserId := user.CacheUserId
			log.Info("Deleting OCI cache user during cluster delete",
				"userId", cacheUserId,
				"user", user.CacheUserName)

			_, err := redisClient.DeleteOciCacheUser(ctx, cacheUserId)
			if err != nil {
				if IsUserNotFoundError(err) {
					log.Info("Cache user already deleted; continuing finalizer",
						"userId", cacheUserId,
						"user", user.CacheUserName)
					// OK – treat as success, clear the ID
					user.CacheUserId = ""
					updated = true
				} else if IsUserStillAttachedError(err) {
					log.Info("Cache user still attached to cluster according to OCI; requeue",
						"userId", cacheUserId,
						"user", user.CacheUserName)
					// keep user, retry later
					newUsers = append(newUsers, user)
					if updated {
						cluster.Status.CacheUsers = newUsers
						_ = r.Status().Update(ctx, cluster)
					}
					return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
				} else {
					log.Error(err, "delete cache user failed; requeue",
						"userId", cacheUserId,
						"user", user.CacheUserName)
					newUsers = append(newUsers, user)
					if updated {
						cluster.Status.CacheUsers = newUsers
						_ = r.Status().Update(ctx, cluster)
					}
					return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
				}
			} else {
				// delete succeeded
				log.Info("Deleted Redis User")
				user.CacheUserId = ""
				updated = true
			}
		}

		// delete per-user Secret (ignore NotFound)
		if user.UserSecretNamespace != "" && user.UserSecretName != "" {
			sec := &corev1.Secret{}
			sec.Namespace = user.UserSecretNamespace
			sec.Name = user.UserSecretName

			log.Info("Deleting redis user credentials secret during cluster delete",
				"namespace", user.UserSecretNamespace,
				"name", user.UserSecretName,
				"user", user.CacheUserName)

			if err := r.Delete(ctx, sec); err != nil && !apierr.IsNotFound(err) {
				log.Error(err, "delete redis user secret failed; requeue",
					"ns", user.UserSecretNamespace, "name", user.UserSecretName)
				newUsers = append(newUsers, user)
				if updated {
					cluster.Status.CacheUsers = newUsers
					_ = r.Status().Update(ctx, cluster)
				}
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
			// secret deleted or already gone
			user.UserSecretName = ""
			user.UserSecretNamespace = ""
			updated = true
		}

		// if there's anything left to clean, keep the user; else drop it
		if user.CacheUserId != "" || user.IsUserAttached || user.UserSecretName != "" || user.UserSecretNamespace != "" {
			newUsers = append(newUsers, user)
		}
	}

	if updated {
		cluster.Status.CacheUsers = newUsers
		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("update cacheUsers status: %w", err)
		}
	}

	// If we still have users to clean up, keep requeueing
	if len(cluster.Status.CacheUsers) > 0 {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
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

func (r *CacheReconciler) reconcileCacheUsers(
	ctx context.Context,
	log logr.Logger,
	ociCache *v1beta1.OciCache,
	redisClient *ocirediscluster.OciRedisClient,
) error {
	ns := ociCache.Namespace

	// desired set from spec.cacheUsers
	desired := make(map[string]struct{}, len(ociCache.Spec.CacheUserSpec.CacheUsers))
	for _, name := range ociCache.Spec.CacheUserSpec.CacheUsers {
		if name == "" {
			continue
		}
		desired[name] = struct{}{}
	}

	// existing map from status.cacheUsers
	existing := make(map[string]*v1beta1.CacheUserStatus, len(ociCache.Status.CacheUsers))
	for i := range ociCache.Status.CacheUsers {
		u := &ociCache.Status.CacheUsers[i]
		if u.CacheUserName != "" {
			existing[u.CacheUserName] = u
		}
	}

	updated := false
	notReady := false

	// Ensure each desired cache user exists/attached
	for userName := range desired {
		username := fmt.Sprintf("redis_%s_user", userName)
		secretName := fmt.Sprintf("%s-redis-credentials", userName)
		acl := fmt.Sprintf("~%s:* +@read +@write +@connection", userName)

		// per-user Secret
		appCreds, err := ensureRedisAppSecret(ctx, r.Client, ns, secretName, username, acl)
		if err != nil {
			log.Error(err, "ensure redis app secret failed", "cacheUser", userName)
			notReady = true
			continue
		}
		log.Info("created app secret per user", "userName", userName)

		// ensure CacheUserStatus entry
		status, ok := existing[userName]
		if !ok {
			ociCache.Status.CacheUsers = append(ociCache.Status.CacheUsers, v1beta1.CacheUserStatus{
				CacheUserName:       userName,
				UserSecretName:      appCreds.SecretName,
				UserSecretNamespace: ns,
			})
			status = &ociCache.Status.CacheUsers[len(ociCache.Status.CacheUsers)-1]
			existing[userName] = status
			updated = true
		} else {
			// update secret fields if empty
			if status.UserSecretName == "" {
				status.UserSecretName = appCreds.SecretName
				status.UserSecretNamespace = ns
				updated = true
			}
		}

		// Create OCI cache user if missing
		if status.CacheUserId == "" {
			id, err := createRedisUser(
				ctx,
				redisClient,
				ociCache.Spec.ClusterSpec.CompartmentId,
				appCreds.User,
				appCreds.Password,
				appCreds.ACL,
				log,
			)
			if err != nil {
				log.Error(err, "create redis user failed", "cacheUser", userName)
				notReady = true
				continue
			}
			status.CacheUserId = id
			log.Info("Successfully created cache user", "cacheUserId", id, "userName", userName)
			updated = true
		}

		// Check lifecycle
		active, lifecycleState, err := checkRedisUserActive(ctx, redisClient, status.CacheUserId, log)
		if err != nil {
			log.Error(err, "check redis user active failed", "cacheUser", userName)
			notReady = true
			continue
		}
		if !active {
			log.Info("Cache user not ACTIVE yet",
				"cacheUser", userName,
				"userId", status.CacheUserId,
				"state", lifecycleState)
			notReady = true
			continue
		}
		log.Info("Cache user is active", "cacheUserId", status.CacheUserId, "userName", userName)

		// Attach to ociCache if needed
		if !status.IsUserAttached {
			if err := attachRedisUserToCluster(ctx, redisClient, ociCache.Status.CacheClusterId, status.CacheUserId, log); err != nil {
				log.Error(err, "attach cache user to ociCache failed",
					"cacheUser", userName,
					"userId", status.CacheUserId)
				notReady = true
				continue
			}
			log.Info("Cache user is attached to ociCache", "cacheUserId", status.CacheUserId, "userName", userName, "clusterId", ociCache.Status.CacheClusterId)
			status.IsUserAttached = true
			updated = true
		}
	}

	// Deprovision users removed from spec
	if len(existing) > 0 {
		newList := make([]v1beta1.CacheUserStatus, 0, len(ociCache.Status.CacheUsers))

		for i := range ociCache.Status.CacheUsers {
			cu := &ociCache.Status.CacheUsers[i]

			// still desired → keep as is
			if _, stillDesired := desired[cu.CacheUserName]; stillDesired {
				newList = append(newList, *cu)
				continue
			}

			log.Info("Deprovisioning cache user removed from spec",
				"cacheUser", cu.CacheUserName,
				"userId", cu.CacheUserId)

			// detach if marked attached
			if cu.IsUserAttached && cu.CacheUserId != "" && ociCache.Status.CacheClusterId != "" {
				if err := detachRedisUserFromCluster(
					ctx, redisClient, ociCache.Status.CacheClusterId, cu.CacheUserId, log,
				); err != nil {
					var svcErr common.ServiceError
					if IsUserNotFoundError(err) {
						log.Info("User already detached or not found on ociCache; treating as success",
							"clusterId", ociCache.Status.CacheClusterId,
							"userId", cu.CacheUserId,
							"ociCode", svcErr.GetCode(),
							"ociMessage", svcErr.GetMessage())
					} else {
						log.Error(err, "detach cache user from ociCache failed",
							"cacheUser", cu.CacheUserName,
							"userId", cu.CacheUserId)
						newList = append(newList, *cu)
						notReady = true
						continue
					}
				}
				// detach success (including "not active on ociCache" case)
				log.Info("Detach user success")
				cu.IsUserAttached = false
				updated = true
			}

			// delete OCI cache user
			if cu.CacheUserId != "" {
				cacheUserId := cu.CacheUserId
				log.Info("Deleting OCI cache user as part of deprovision",
					"cacheUser", cu.CacheUserName,
					"userId", cacheUserId,
					"clusterId", ociCache.Status.CacheClusterId)
				_, err := redisClient.DeleteOciCacheUser(ctx, cacheUserId)
				if err != nil {
					if IsUserNotFoundError(err) {
						log.Info("Cache user already deleted; continuing deprovision",
							"cacheUser", cu.CacheUserName,
							"userId", cacheUserId)
						cu.CacheUserId = ""
						updated = true
					} else if IsUserStillAttachedError(err) {
						log.Info("Cache user still attached to ociCache according to OCI; will retry",
							"cacheUser", cu.CacheUserName,
							"userId", cacheUserId)
						newList = append(newList, *cu)
						notReady = true
						continue
					} else {
						log.Error(err, "delete cache user failed",
							"cacheUser", cu.CacheUserName,
							"userId", cacheUserId)
						newList = append(newList, *cu)
						notReady = true
						continue
					}
				} else {
					log.Info("Successfully deleted OCI cache user",
						"cacheUser", cu.CacheUserName,
						"userId", cacheUserId)
					// delete succeeded
					cu.CacheUserId = ""
					updated = true
				}
			}

			// delete per-user secret (ignore NotFound)
			if cu.UserSecretNamespace != "" && cu.UserSecretName != "" {
				sec := &corev1.Secret{}
				sec.Namespace = cu.UserSecretNamespace
				sec.Name = cu.UserSecretName

				if err := r.Delete(ctx, sec); err != nil && !apierr.IsNotFound(err) {
					log.Error(err, "delete redis user secret failed",
						"ns", cu.UserSecretNamespace,
						"name", cu.UserSecretName)
					newList = append(newList, *cu)
					notReady = true
					continue
				}
				cu.UserSecretName = ""
				cu.UserSecretNamespace = ""
				updated = true
			}

			// if anything left to clean, keep it; else drop this user
			if cu.CacheUserId != "" || cu.IsUserAttached || cu.UserSecretName != "" || cu.UserSecretNamespace != "" {
				newList = append(newList, *cu)
			}
		}

		ociCache.Status.CacheUsers = newList
	}

	// Persist status if changed
	if updated {
		if err := r.Status().Update(ctx, ociCache); err != nil {
			return err
		}
	}

	if notReady {
		// signal caller to requeue
		return fmt.Errorf("one or more cache users not ready or pending deprovision")
	}
	return nil
}

// ensureRedisAppSecret creates or reuses a Secret that stores username/password/ACL.
// - If Secret exists: reuse username/password/acl.
// - If missing: generate a random password, hash it, and create the Secret.
func ensureRedisAppSecret(
	ctx context.Context,
	c client.Client,
	ns, secretName string,
	username string,
	acl string,
) (*appRedisCreds, error) {
	existing := &corev1.Secret{}
	if err := c.Get(ctx, ktypes.NamespacedName{Namespace: ns, Name: secretName}, existing); err == nil {
		return &appRedisCreds{
			User:       string(existing.Data[redisSecretKeyUsername]),
			Password:   string(existing.Data[redisSecretKeyPassword]),
			ACL:        string(existing.Data[redisSecretKeyACL]),
			SecretName: existing.Name,
		}, nil
	} else if !apierr.IsNotFound(err) {
		return nil, fmt.Errorf("read redis app secret: %w", err)
	}

	// New secret: generate random password
	pass := PasswordUtil.GenerateRandomPassword(20)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ns,
			Labels:    map[string]string{"app": "redis-user"},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			redisSecretKeyUsername: username,
			redisSecretKeyPassword: pass,
			redisSecretKeyACL:      acl,
		},
	}
	if err := c.Create(ctx, sec); err != nil {
		return nil, fmt.Errorf("create redis app secret: %w", err)
	}

	return &appRedisCreds{
		User:       username,
		Password:   pass,
		ACL:        acl,
		SecretName: secretName,
	}, nil
}

func createRedisUser(
	ctx context.Context,
	client *ocirediscluster.OciRedisClient,
	compartmentID string,
	username string,
	password string,
	acl string,
	log logr.Logger,
) (string, error) {
	if compartmentID == "" {
		return "", fmt.Errorf("compartmentId is required to create OCI cache user")
	}

	log.Info("Creating OCI cache user",
		"username", username,
		"compartmentId", compartmentID,
	)
	hashedPass := PasswordUtil.HashPasswordSHA256Hex(password)

	createResp, err := client.CreateUserWithPassword(ctx, compartmentID, username, "cacheUser", acl, []string{hashedPass})
	if err != nil {
		return "", fmt.Errorf("CreateUserWithPassword failed: %w", err)
	}
	if createResp.OciCacheUser.Id == nil {
		return "", fmt.Errorf("CreateUserWithPassword: returned user has nil Id")
	}
	userOCID := *createResp.OciCacheUser.Id
	log.Info("Created/updated OCI Cache user", "userId", userOCID, "username", username)
	return userOCID, nil
}

func checkRedisUserActive(
	ctx context.Context,
	client *ocirediscluster.OciRedisClient,
	userOCID string,
	log logr.Logger,
) (bool, string, error) {
	resp, err := client.GetOciCacheUser(ctx, userOCID)
	if err != nil {
		return false, "", fmt.Errorf("GetUser failed: %w", err)
	}

	state := string(resp.OciCacheUser.LifecycleState)
	log.Info("Checked OCI cache user lifecycle state",
		"userId", userOCID,
		"state", state,
	)

	switch state {
	case "ACTIVE":
		return true, state, nil
	case "FAILED", "NEEDS_ATTENTION", "DELETED":
		// treat as terminal / error
		return false, state, fmt.Errorf("cache user %s in terminal state: %s", userOCID, state)
	default:
		// CREATING, UPDATING, etc. – not ready yet, but not an error
		return false, state, nil
	}
}

func attachRedisUserToCluster(
	ctx context.Context,
	client *ocirediscluster.OciRedisClient,
	redisClusterID string,
	userOCID string,
	log logr.Logger,
) error {
	if redisClusterID == "" {
		return fmt.Errorf("redisClusterId is required to attach user")
	}

	log.Info("Attaching OCI Cache user to Redis cluster",
		"clusterId", redisClusterID,
		"userId", userOCID,
	)

	_, err := client.AttachUserToCluster(ctx, redisClusterID, []string{userOCID})
	if err != nil {
		return fmt.Errorf("AttachUserToCluster failed: %w", err)
	}

	log.Info("Attached OCI Cache user to Redis cluster",
		"clusterId", redisClusterID,
		"userId", userOCID,
	)
	return nil
}

func detachRedisUserFromCluster(
	ctx context.Context,
	client *ocirediscluster.OciRedisClient,
	redisClusterID string,
	userOCID string,
	log logr.Logger,
) error {
	if redisClusterID == "" {
		return fmt.Errorf("redisClusterId is empty; cannot detach user %s", userOCID)
	}
	log.Info("Detaching OCI Cache user from Redis cluster",
		"clusterId", redisClusterID,
		"userId", userOCID,
	)

	_, err := client.DetachCacheUserFromCluster(ctx, []string{userOCID}, redisClusterID)
	if err != nil {
		return fmt.Errorf("DetachCacheUserFromCluster failed: %w", err)
	}
	return nil
}

// IsUserNotFoundError returns true if the OCI error indicates the cache user doesn't exist.
func IsUserNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var svcErr common.ServiceError
	if errors.As(err, &svcErr) {
		if svcErr.GetHTTPStatusCode() == 404 {
			return true
		}
		// Some OCI services use NotAuthorizedOrNotFound for 404-equivalent
		if strings.EqualFold(svcErr.GetCode(), "NotAuthorizedOrNotFound") {
			return true
		}
	}
	// Fallback string check if needed
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist")
}

// IsUserStillAttachedError returns true if DeleteUser failed because it's still attached.
func IsUserStillAttachedError(err error) bool {
	if err == nil {
		return false
	}
	var svcErr common.ServiceError
	if errors.As(err, &svcErr) {
		code := strings.ToLower(svcErr.GetCode())
		if strings.Contains(code, "stillattached") || strings.Contains(code, "attachedtoactivecluster") {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "attached to an active cluster") ||
		strings.Contains(msg, "still attached")
}

func setCondition(
	status *v1beta1.OciCacheStatus,
	condType string,
	condStatus metav1.ConditionStatus,
	reason, message string,
	gen int64,
) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: gen,
	})
}

package ocipostgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/identity"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"

	PostgreSQLUtil "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/ocipostgres/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	logging "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	ocipostgresdbsystem "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ocidbsystem"
	ociidentity "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ociidentity"
	"github.com/go-logr/logr"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/psql"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type adminCreds struct {
	Username   string
	Password   string
	SecretName string
}

const (
	tagCreatedBy           = "ome:created-by"
	tagCRUID               = "ome:cr-uid"
	adminSecretUserKey     = "username"
	adminSecretPasswordKey = "password"
	defaultAdminSecretName = "pg-admin"
	finalizerName          = "ocipostgrescluster/finalizer"
)

var (
	newPGClient = func(cfg *ocipostgresdbsystem.Config) (*ocipostgresdbsystem.OciPostgresClient, error) {
		return ocipostgresdbsystem.NewOciPostgreSQLClient(cfg)
	}
	newIdentityClient = func(cfg *ociidentity.Config) (*ociidentity.OciIdentityClient, error) {
		return ociidentity.NewIdentityClient(cfg)
	}
)

// +kubebuilder:rbac:groups=ome.io,resources=ocipostgres,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=ocipostgres/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=ocipostgres/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch;watch

type PostgresReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Log      logr.Logger
}

func (r *PostgresReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PostgresCluster", req.NamespacedName)

	ociPostgres := &v1beta1.OciPostgres{}
	if err := r.Get(ctx, req.NamespacedName, ociPostgres); err != nil {
		// IgnoreNotFound tells controller-runtime not to requeue for deleted CRs
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("reconcile OciPostgresCluster", "namespace", req.Namespace, "name", req.Name)

	// create ocipgClient with a proper logger adapter
	ocipostgresDbConfig, err := ocipostgresdbsystem.NewConfig(
		ocipostgresdbsystem.WithAnotherLog(logging.ForLogr(r.Log)),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create ocipostgresDbConfig config: %w", err)
	}

	ocipostgresDbConfig.AuthType = Ptr(principals.AuthenticationType(ociPostgres.Spec.ClusterSpec.AuthType))
	pgClient, err := newPGClient(ocipostgresDbConfig)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create postgres client: %w", err)
	}

	// handle deletion via finalizer
	if !ociPostgres.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, ociPostgres, pgClient)
	}

	if addFinalizerIfNeeded(ociPostgres) {
		if err := r.Update(ctx, ociPostgres); err != nil {
			log.Error(err, "Failed to add finalizer", "name", ociPostgres.Name)
			r.Recorder.Event(ociPostgres, corev1.EventTypeWarning, "FinalizerAddFailed", "Failed to add finalizer")
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// requeue to continue normal flow
		return ctrl.Result{Requeue: true}, nil
	}
	admin, err := r.getOrCreateAdminCred(ctx, req.Namespace, defaultAdminSecretName)
	if err != nil {
		log.Error(err, "Failed to get or create admin credentials", "namespace", req.Namespace)
		r.Recorder.Event(ociPostgres, corev1.EventTypeWarning, "AdminSecretError", err.Error())
		return ctrl.Result{}, fmt.Errorf("get or create admin credentials: %w", err)
	}

	// If we don't have a DB System yet, create it
	if strings.TrimSpace(ociPostgres.Status.DbSystemId) == "" {
		log.Info("Creating new DB system", "dbName", ociPostgres.Name)
		newID, cerr := r.createDbSystem(ctx, ociPostgres, admin, log, pgClient, string(ociPostgres.UID))
		if cerr != nil {
			log.Error(cerr, "Failed to create dbsystem", "name", ociPostgres.Name)
			return ctrl.Result{}, cerr
		}
		log.Info("Created PostgreSQL DB System", "dbSystemId", newID, "dbName", ociPostgres.Name)

		// Update status with the new ID
		ociPostgres.Status.DbSystemId = newID
		ociPostgres.Status.LifecycleState = "CREATING"
		if uerr := r.Status().Update(ctx, ociPostgres); uerr != nil {
			log.Error(uerr, "Failed to update status", "name", ociPostgres.Name)
			r.Recorder.Event(ociPostgres, corev1.EventTypeWarning, "UpdateStatusFailed", "Failed to update status")
			return ctrl.Result{}, uerr
		}
		// Requeue to poll state transition
		return ctrl.Result{Requeue: true}, nil
	}

	// DB System exists — poll its lifecycle and fetch connection details
	log.Info("Checking available DB Cluster", "clusterId", ociPostgres.Status.DbSystemId)

	get, gerr := pgClient.GetDbSystem(ctx, psql.GetDbSystemRequest{
		DbSystemId: common.String(ociPostgres.Status.DbSystemId),
	})
	if gerr != nil {
		if apierr.IsNotFound(gerr) {
			log.Info("DB system not found by ID; will recreate", "dbSystemId", ociPostgres.Status.DbSystemId)
			ociPostgres.Status.DbSystemId = ""
			if uerr := r.Status().Update(ctx, ociPostgres); uerr != nil {
				return ctrl.Result{}, uerr
			}
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(gerr, "Failed to get db system", "name", ociPostgres.Name)
		r.Recorder.Event(ociPostgres, corev1.EventTypeWarning, "GetDbSystemFailed", "Failed to get db system")
		return ctrl.Result{}, fmt.Errorf("get db system: %w", gerr)
	}

	state := get.DbSystem.LifecycleState
	log.Info("PostgreSQL lifecycle", "dbSystemId", ociPostgres.Status.DbSystemId, "state", state)

	switch state {
	case psql.DbSystemLifecycleStateActive:
		log.Info("DB ociPostgresCluster is ready")
		deepCopyOfOciPostgres := ociPostgres.DeepCopy()
		ociPostgres.Status.AdminSecretNamespace = req.Namespace
		ociPostgres.Status.AdminSecretName = defaultAdminSecretName
		allReady, _, err := r.reconcileLogicalDatabases(ctx, log, ociPostgres, pgClient, admin)
		if err != nil {
			log.Error(err, "failed to reconcile logical databases")
			ociPostgres.Status.LifecycleState = v1beta1.LifecycleStateFailed
			setCondition(
				&ociPostgres.Status,
				"Ready",
				metav1.ConditionFalse,
				"LogicalDatabasesReconciling",
				"LogicalDatabasesReconcileFailed",
				ociPostgres.Generation,
			)
			if perr := r.Status().Patch(ctx, ociPostgres, client.MergeFrom(deepCopyOfOciPostgres)); perr != nil {
				return ctrl.Result{}, perr
			}
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		if allReady {
			ociPostgres.Status.LifecycleState = v1beta1.LifecycleStateReady
			log.Info("DB logical databases reconciled")
			setCondition(
				&ociPostgres.Status,
				"Ready",
				metav1.ConditionTrue,
				"AllLogicalDatabasesReady",
				"Postgres DB system and all required logical databases are ready",
				ociPostgres.Generation,
			)
		} else {
			ociPostgres.Status.LifecycleState = v1beta1.LifecycleStateUpdating
			setCondition(
				&ociPostgres.Status,
				"Ready",
				metav1.ConditionFalse,
				"AllLogicalDatabasesUpdating",
				"Postgres DB system and all required logical databases are in updating stage",
				ociPostgres.Generation,
			)
		}
		if perr := r.Status().Patch(ctx, ociPostgres, client.MergeFrom(deepCopyOfOciPostgres)); perr != nil {
			return ctrl.Result{}, perr
		}
		if allReady {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil

	case psql.DbSystemLifecycleStateCreating:
		ociPostgres.Status.LifecycleState = "CREATING"
		setCondition(
			&ociPostgres.Status,
			"Ready",
			metav1.ConditionFalse,
			"PostgresClusterCreating",
			"Postgres DB system is in creating stage",
			ociPostgres.Generation,
		)
		// Keep polling
		if uerr := r.Status().Update(ctx, ociPostgres); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	case psql.DbSystemLifecycleStateUpdating:
		ociPostgres.Status.LifecycleState = "UPDATING"
		setCondition(
			&ociPostgres.Status,
			"Ready",
			metav1.ConditionFalse,
			"PostgresClusterUpdating",
			"Postgres DB system is in updating stage",
			ociPostgres.Generation,
		)
		// Keep polling
		if uerr := r.Status().Update(ctx, ociPostgres); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	default:
		ociPostgres.Status.LifecycleState = "FAILED"
		setCondition(
			&ociPostgres.Status,
			"Ready",
			metav1.ConditionFalse,
			"PostgresClusterFailed",
			"Postgres DB system creation faled",
			ociPostgres.Generation,
		)
		_ = r.Status().Update(ctx, ociPostgres)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

func (r *PostgresReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1beta1.OciPostgres{}).Complete(r)
}

// createDbSystem creates an OCI PostgreSQL DB System and returns its OCID.
func (r *PostgresReconciler) createDbSystem(
	ctx context.Context,
	PostgresDBCluster *v1beta1.OciPostgres,
	creds *adminCreds,
	log logr.Logger,
	pgClient *ocipostgresdbsystem.OciPostgresClient,
	crUID string,
) (string, error) {
	compartmentID := PostgresDBCluster.Spec.ClusterSpec.CompartmentId
	if compartmentID == "" {
		return "", fmt.Errorf("spec.compartmentId is required")
	}
	subnetId := PostgresDBCluster.Spec.ClusterSpec.NetworkDetails.SubnetId
	if subnetId == "" {
		return "", fmt.Errorf("TargetSubnetOCID must be set")
	}

	displayName := PostgresDBCluster.Name
	tags := map[string]string{
		tagCreatedBy: "postgresql-cluster-controller",
		tagCRUID:     crUID,
	}

	idCfg, err := ociidentity.NewConfig(ociidentity.WithAnotherLog(logging.ForLogr(r.Log)))
	if err != nil {
		return "", fmt.Errorf("build identity config: %w", err)
	}
	idCfg.AuthType = Ptr(principals.AuthenticationType(PostgresDBCluster.Spec.ClusterSpec.AuthType))

	idc, err := newIdentityClient(idCfg)
	if err != nil {
		return "", fmt.Errorf("build identity client: %w", err)
	}

	adResp, err := idc.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return "", fmt.Errorf("list availability domains: %w", err)
	}
	if len(adResp.Items) == 0 || adResp.Items[0].Name == nil {
		return "", fmt.Errorf("no availability domains returned")
	}
	// ---- Decide storage durability & AD per requirement
	var storageDetails psql.OciOptimizedStorageDetails
	if len(adResp.Items) == 1 && adResp.Items[0].Name != nil {
		// Exactly one AD -> regionally durable false, set AD name
		storageDetails = psql.OciOptimizedStorageDetails{
			IsRegionallyDurable: common.Bool(false),
			AvailabilityDomain:  adResp.Items[0].Name,
		}
	} else {
		// Otherwise -> regionally durable true, no AD
		storageDetails = psql.OciOptimizedStorageDetails{
			IsRegionallyDurable: common.Bool(true),
		}
	}

	details := psql.CreateDbSystemDetails{
		DisplayName:   common.String(displayName),
		CompartmentId: common.String(compartmentID),
		DbVersion:     common.String(PostgresDBCluster.Spec.ClusterSpec.DbVersion),
		Shape:         common.String(PostgresDBCluster.Spec.ClusterSpec.Shape),
		NetworkDetails: &psql.NetworkDetails{
			SubnetId: common.String(subnetId),
			NsgIds:   PostgresDBCluster.Spec.ClusterSpec.NetworkDetails.NsgIds,
		},
		Credentials: &psql.Credentials{
			Username: common.String(creds.Username),
			PasswordDetails: psql.PlainTextPasswordDetails{
				Password: common.String(creds.Password),
			},
		},
		StorageDetails:          storageDetails,
		InstanceCount:           common.Int(PostgresDBCluster.Spec.ClusterSpec.InstanceCount),
		InstanceOcpuCount:       common.Int(*PostgresDBCluster.Spec.ClusterSpec.InstanceOcpuCount),
		InstanceMemorySizeInGBs: common.Int(*PostgresDBCluster.Spec.ClusterSpec.InstanceMemorySizeInGbs),
		Source:                  psql.NoneSourceDetails{},
		FreeformTags:            tags,
	}

	token := hashCreateDetails(details)
	req := psql.CreateDbSystemRequest{
		CreateDbSystemDetails: details,
		OpcRetryToken:         common.String(token),
	}
	log.Info("Creating DB in Progress with request", "request", req, "dbName", PostgresDBCluster.Name)
	resp, err := pgClient.CreateDbSystem(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create db system: %w", err)
	}
	return *resp.DbSystem.Id, nil
}

func (r *PostgresReconciler) getOrCreateAdminCred(ctx context.Context, ns string, name string) (*adminCreds, error) {
	current := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, current)
	if err != nil {
		if apierr.IsNotFound(err) {
			username := "admin"
			password := PostgreSQLUtil.GenerateRandomPassword(15)
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
					Labels: map[string]string{
						"cluster": "postgresql-cluster",
					},
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					adminSecretUserKey:     username,
					adminSecretPasswordKey: password,
				},
			}
			err := r.Client.Create(ctx, secret)
			if err != nil {
				return nil, fmt.Errorf("create admin secret: %w", err)
			}
			return &adminCreds{Username: username, Password: password, SecretName: name}, nil
		} else {
			return nil, fmt.Errorf("get admin secret: %w", err)
		}
	}
	// Found — return existing values
	return &adminCreds{
		Username:   string(current.Data[adminSecretUserKey]),
		Password:   string(current.Data[adminSecretPasswordKey]),
		SecretName: current.Name,
	}, nil
}

func hashCreateDetails(d psql.CreateDbSystemDetails) string {
	h := sha256.New()
	h.Write([]byte("dn:" + *d.DisplayName))
	h.Write([]byte("cid:" + *d.CompartmentId))
	if d.NetworkDetails != nil {
		h.Write([]byte("subnet:" + *d.NetworkDetails.SubnetId))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func Ptr[T any](v T) *T { return &v }

func addFinalizerIfNeeded(cr *v1beta1.OciPostgres) bool {
	for _, f := range cr.Finalizers {
		if f == finalizerName {
			return false
		}
	}
	cr.Finalizers = append(cr.Finalizers, finalizerName)
	return true
}

func (r *PostgresReconciler) reconcileDelete(
	ctx context.Context,
	log logr.Logger,
	cluster *v1beta1.OciPostgres,
	pgClient *ocipostgresdbsystem.OciPostgresClient,
) (ctrl.Result, error) {
	// We only act if our finalizer is present
	hasFinalizer := false
	for _, f := range cluster.Finalizers {
		if f == finalizerName {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		// nothing to do
		return ctrl.Result{}, nil
	}

	// delete OCI DB System if exists
	if id := strings.TrimSpace(cluster.Status.DbSystemId); id != "" {
		log.Info("Deleting OCI PostgreSQL DB System", "dbSystemId", id)
		if _, err := pgClient.DeleteDbSystem(ctx, psql.DeleteDbSystemRequest{
			DbSystemId: &id,
		}); err != nil {
			// transient or still deleting → requeue
			log.Error(err, "delete db system failed; requeue")
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
	}

	// delete all per-DB app secrets
	for _, inst := range cluster.Status.DbInstances {
		ns := inst.AppUserSecretNamespace
		name := inst.AppUserSecretName
		if ns == "" || name == "" {
			continue
		}
		sec := &corev1.Secret{}
		sec.Namespace = ns
		sec.Name = name

		log.Info("Deleting app DB credentials secret during cluster delete",
			"namespace", ns, "name", name, "databaseName", inst.DatabaseName)

		if err := r.Delete(ctx, sec); err != nil && !apierr.IsNotFound(err) {
			log.Error(err, "delete app secret failed; requeue",
				"ns", ns, "name", name, "databaseName", inst.DatabaseName)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// delete admin secret (ignore NotFound)
	if ns := cluster.Namespace; ns != "" {
		sec := &corev1.Secret{}
		sec.Namespace = ns
		sec.Name = defaultAdminSecretName
		if err := r.Delete(ctx, sec); err != nil && !apierr.IsNotFound(err) {
			log.Error(err, "delete admin secret failed; requeue", "ns", ns, "name", defaultAdminSecretName)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// remove finalizer so Kubernetes can remove the CR
	removeFinalizer(cluster)
	if err := r.Update(ctx, cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	log.Info("Finalizer completed, CR can be removed")
	return ctrl.Result{}, nil
}

func removeFinalizer(cr *v1beta1.OciPostgres) {
	fs := make([]string, 0, len(cr.Finalizers))
	for _, f := range cr.Finalizers {
		if f != finalizerName {
			fs = append(fs, f)
		}
	}
	cr.Finalizers = fs
}

func (r *PostgresReconciler) reconcileLogicalDatabases(
	ctx context.Context,
	log logr.Logger,
	cluster *v1beta1.OciPostgres,
	pgClient *ocipostgresdbsystem.OciPostgresClient,
	admin *adminCreds,
) (bool, bool, error) {
	// optimistic default: everything is ready unless we see otherwise
	allReady := true
	updated := false

	var aggErr error

	// helper: aggregate errors for desired DB failures
	addDesiredErr := func(dbName, step string, err error) {
		if err == nil {
			return
		}
		aggErr = errors.Join(aggErr, fmt.Errorf("logical db %q: %s: %w", dbName, step, err))
	}

	// Get connection details for this DbSystem
	host, port, ca, fqdn, err := PostgreSQLUtil.GetPrimaryEndpoint(ctx, cluster.Status.DbSystemId, pgClient)
	if err != nil || host == "" || port == 0 {
		r.Recorder.Event(cluster, corev1.EventTypeWarning, "EndpointLookupFailed", "Failed to get primary endpoint")
		return false, false, fmt.Errorf("primary endpoint not ready for dbSystem %s: %w", cluster.Status.DbSystemId, err)
	}

	// Build desired set from spec.logicalDatabases
	desired := make(map[string]struct{}, len(cluster.Spec.DbInstanceSpec.LogicalDatabases))
	for _, dbName := range cluster.Spec.DbInstanceSpec.LogicalDatabases {
		if dbName == "" {
			continue
		}
		desired[dbName] = struct{}{}
	}

	// Build existing map from status.dbInstances
	existing := make(map[string]*v1beta1.DbInstanceStatus, len(cluster.Status.DbInstances))
	for i := range cluster.Status.DbInstances {
		db := &cluster.Status.DbInstances[i]
		if db.DatabaseName != "" {
			existing[db.DatabaseName] = db
		}
	}
	for dbName := range desired {
		// Already READY -> nothing to do
		if db, ok := existing[dbName]; ok && db.LifecycleState == v1beta1.LifecycleStateReady {
			continue
		}
		// if any desired db isn't READY yet, the cluster isn't "fully ready"
		allReady = false
		log.Info("Reconciling logical database",
			"dbSystemId", cluster.Status.DbSystemId,
			"databaseName", dbName)

		// Decide app secret name & namespace
		appSecretName := fmt.Sprintf("%s-db-credentials", dbName)
		appSecretNS := cluster.Namespace

		// Ensure per-app secret
		app, err := PostgreSQLUtil.EnsureAppUserAndSecret(ctx, r.Client, appSecretNS, appSecretName, dbName)
		if err != nil {
			log.Error(err, "ensure app secret failed", "databaseName", dbName)
			addDesiredErr(dbName, "ensure app secret", err)
			// Update status for this DB as FAILED
			if db, ok := existing[dbName]; ok {
				db.LifecycleState = v1beta1.LifecycleStateFailed
				db.AppUserSecretName = appSecretName
				db.AppUserSecretNamespace = appSecretNS
			} else {
				cluster.Status.DbInstances = append(cluster.Status.DbInstances, v1beta1.DbInstanceStatus{
					DatabaseName:           dbName,
					AppUserSecretName:      appSecretName,
					AppUserSecretNamespace: appSecretNS,
					LifecycleState:         v1beta1.LifecycleStateFailed,
				})
			}
			updated = true
			continue
		}
		log.Info("Reconciling logical database", "created secret per app", dbName)

		// Create DB if missing
		if err := PostgreSQLUtil.CreateDBIfMissing(ctx,
			host, port,
			admin.Username, admin.Password,
			ca, fqdn,
			dbName,
			log,
		); err != nil {
			log.Error(err, "create DB failed", "databaseName", dbName)
			addDesiredErr(dbName, "create DB", err)
			if db, ok := existing[dbName]; ok {
				db.LifecycleState = v1beta1.LifecycleStateFailed
			} else {
				cluster.Status.DbInstances = append(cluster.Status.DbInstances, v1beta1.DbInstanceStatus{
					DatabaseName:           dbName,
					AppUserSecretName:      app.SecretName,
					AppUserSecretNamespace: appSecretNS,
					LifecycleState:         v1beta1.LifecycleStateFailed,
				})
			}
			updated = true
			continue
		}
		log.Info("Reconciling logical database", "created logical db per app", dbName)

		// Ensure role & grants on that DB
		appUser := fmt.Sprintf("app_%s_owner", dbName)
		if err := PostgreSQLUtil.EnsureRoleAndGrants(
			ctx,
			host, port,
			admin.Username, admin.Password,
			ca, fqdn,
			dbName,
			appUser, app.Password,
			log,
		); err != nil {
			log.Error(err, "ensure role/grants failed", "databaseName", dbName)
			addDesiredErr(dbName, "ensure role/grants", err)
			if db, ok := existing[dbName]; ok {
				db.LifecycleState = v1beta1.LifecycleStateFailed
				db.AppUserSecretName = app.SecretName
				db.AppUserSecretNamespace = appSecretNS
			} else {
				cluster.Status.DbInstances = append(cluster.Status.DbInstances, v1beta1.DbInstanceStatus{
					DatabaseName:           dbName,
					AppUserSecretName:      app.SecretName,
					AppUserSecretNamespace: appSecretNS,
					LifecycleState:         v1beta1.LifecycleStateFailed,
				})
			}
			updated = true
			continue
		}
		log.Info("Reconciling logical database", "ensured role and grants per app", dbName)

		// Success → mark DbInstanceStatus READY
		if db, ok := existing[dbName]; ok {
			db.LifecycleState = v1beta1.LifecycleStateReady
			db.AppUserSecretName = app.SecretName
			db.AppUserSecretNamespace = appSecretNS
		} else {
			cluster.Status.DbInstances = append(cluster.Status.DbInstances, v1beta1.DbInstanceStatus{
				DatabaseName:           dbName,
				AppUserSecretName:      app.SecretName,
				AppUserSecretNamespace: appSecretNS,
				LifecycleState:         v1beta1.LifecycleStateReady,
			})
		}
		updated = true
		log.Info("Reconciling logical database success")
	}

	// Deprovision DBs that were removed from Spec
	if len(existing) > 0 {
		newList := make([]v1beta1.DbInstanceStatus, 0, len(cluster.Status.DbInstances))

		for i := range cluster.Status.DbInstances {
			db := cluster.Status.DbInstances[i]
			if _, stillDesired := desired[db.DatabaseName]; stillDesired {
				// keep this one
				newList = append(newList, db)
				continue
			}

			// Not desired anymore -> deprovision
			allReady = false // cluster isn't fully "done" while cleanup is pending
			log.Info("Deprovisioning logical database removed from spec",
				"dbSystemId", cluster.Status.DbSystemId,
				"databaseName", db.DatabaseName)

			appUser := fmt.Sprintf("app_%s_owner", db.DatabaseName)

			if err := PostgreSQLUtil.DropRoleAndDatabase(
				ctx,
				host, port,
				admin.Username, admin.Password,
				ca, fqdn,
				db.DatabaseName, appUser,
				log,
			); err != nil {
				log.Error(err, "drop role/database failed (will retry)", "databaseName", db.DatabaseName)
				// keep it in the list so we retry next reconcile
				newList = append(newList, db)
				continue
			}
			log.Info("Deprovisioning logical database", "dropped role and database", db)

			// delete app secret (ignore NotFound)
			if db.AppUserSecretNamespace != "" && db.AppUserSecretName != "" {
				sec := &corev1.Secret{}
				sec.Namespace = db.AppUserSecretNamespace
				sec.Name = db.AppUserSecretName
				if err := r.Delete(ctx, sec); err != nil && !apierr.IsNotFound(err) {
					log.Error(err, "delete app secret failed; keeping dbInstance for retry",
						"ns", db.AppUserSecretNamespace, "name", db.AppUserSecretName)
					newList = append(newList, db)
					continue
				}
			}
			log.Info("Deprovisioning logical database", "delete secret per db", db)

			// Successfully deprovisioned
			updated = true
		}
		cluster.Status.DbInstances = newList
	}

	// re-evaluate: if any desired db isn't READY in status, treat not ready
	existing = make(map[string]*v1beta1.DbInstanceStatus, len(cluster.Status.DbInstances))
	for i := range cluster.Status.DbInstances {
		db := &cluster.Status.DbInstances[i]
		if db.DatabaseName != "" {
			existing[db.DatabaseName] = db
		}
	}

	for dbName := range desired {
		if st, ok := existing[dbName]; !ok || st.LifecycleState != v1beta1.LifecycleStateReady {
			allReady = false
			break
		}
	}
	return allReady, updated, aggErr
}

func setCondition(
	status *v1beta1.OciPostgresStatus,
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

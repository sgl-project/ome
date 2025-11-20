package ocipostgrescluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/identity"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"

	PostgreSQLUtil "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/ocipostgrescluster/utils"

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
	adminSecretUserKey     = "username" // [ADDED]
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

// +kubebuilder:rbac:groups=ome.io,resources=dbclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=dbclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=dbclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch;watch

type DBClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Log      logr.Logger
}

func (r *DBClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PostgresCluster", req.NamespacedName)

	ociPostgresCluster := &v1beta1.OciPostgresCluster{}
	if err := r.Get(ctx, req.NamespacedName, ociPostgresCluster); err != nil {
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

	ocipostgresDbConfig.AuthType = Ptr(principals.AuthenticationType(ociPostgresCluster.Spec.AuthType))
	pgClient, err := newPGClient(ocipostgresDbConfig)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create postgres client: %w", err)
	}

	// handle deletion via finalizer
	if !ociPostgresCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, ociPostgresCluster, pgClient)
	}

	if addFinalizerIfNeeded(ociPostgresCluster) {
		if err := r.Update(ctx, ociPostgresCluster); err != nil {
			log.Error(err, "Failed to add finalizer", "name", ociPostgresCluster.Name)
			r.Recorder.Event(ociPostgresCluster, corev1.EventTypeWarning, "FinalizerAddFailed", "Failed to add finalizer")
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// requeue to continue normal flow
		return ctrl.Result{Requeue: true}, nil
	}

	// If we don't have a DB System yet, create it
	if strings.TrimSpace(ociPostgresCluster.Status.DbSystemId) == "" {
		creds, err := r.getOrCreateAdminCred(ctx, req.Namespace, defaultAdminSecretName)
		if err != nil {
			log.Error(err, "Failed to get or create admin credentials", "namespace", req.Namespace)
			r.Recorder.Event(ociPostgresCluster, corev1.EventTypeWarning, "AdminSecretError", err.Error())
			return ctrl.Result{}, fmt.Errorf("get or create admin credentials: %w", err)
		}

		log.Info("Creating new DB system", "dbName", ociPostgresCluster.Name)
		newID, cerr := r.createDbSystem(ctx, ociPostgresCluster, creds, log, pgClient, string(ociPostgresCluster.UID))
		if cerr != nil {
			log.Error(cerr, "Failed to create dbsystem", "name", ociPostgresCluster.Name)
			return ctrl.Result{}, cerr
		}
		log.Info("Created PostgreSQL DB System", "dbSystemId", newID, "dbName", ociPostgresCluster.Name)

		// Update status with the new ID
		ociPostgresCluster.Status.DbSystemId = newID
		ociPostgresCluster.Status.LifecycleState = "CREATING"
		if uerr := r.Status().Update(ctx, ociPostgresCluster); uerr != nil {
			log.Error(uerr, "Failed to update status", "name", ociPostgresCluster.Name)
			r.Recorder.Event(ociPostgresCluster, corev1.EventTypeWarning, "UpdateStatusFailed", "Failed to update status")
			return ctrl.Result{}, uerr
		}
		// Requeue to poll state transition
		return ctrl.Result{Requeue: true}, nil
	}

	// DB System exists — poll its lifecycle and fetch connection details
	log.Info("Checking available DB Cluster", "clusterId", ociPostgresCluster.Status.DbSystemId)

	get, gerr := pgClient.GetDbSystem(ctx, psql.GetDbSystemRequest{
		DbSystemId: common.String(ociPostgresCluster.Status.DbSystemId),
	})
	if gerr != nil {
		if apierr.IsNotFound(gerr) {
			log.Info("DB system not found by ID; will recreate", "dbSystemId", ociPostgresCluster.Status.DbSystemId)
			ociPostgresCluster.Status.DbSystemId = ""
			if uerr := r.Status().Update(ctx, ociPostgresCluster); uerr != nil {
				return ctrl.Result{}, uerr
			}
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(gerr, "Failed to get db system", "name", ociPostgresCluster.Name)
		r.Recorder.Event(ociPostgresCluster, corev1.EventTypeWarning, "GetDbSystemFailed", "Failed to get db system")
		return ctrl.Result{}, fmt.Errorf("get db system: %w", gerr)
	}

	state := get.DbSystem.LifecycleState
	log.Info("PostgreSQL lifecycle", "dbSystemId", ociPostgresCluster.Status.DbSystemId, "state", state)

	switch state {
	case psql.DbSystemLifecycleStateActive:
		ociPostgresCluster.Status.AdminSecretNamespace = req.Namespace
		ociPostgresCluster.Status.AdminSecretName = defaultAdminSecretName
		ociPostgresCluster.Status.LifecycleState = "READY"

		if uerr := r.Status().Update(ctx, ociPostgresCluster); uerr != nil {
			return ctrl.Result{}, uerr
		}
		log.Info("DB ociPostgresCluster is ready")
		return ctrl.Result{}, nil

	case psql.DbSystemLifecycleStateCreating:
		ociPostgresCluster.Status.LifecycleState = "CREATING"
		// Keep polling
		if uerr := r.Status().Update(ctx, ociPostgresCluster); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	case psql.DbSystemLifecycleStateUpdating:
		ociPostgresCluster.Status.LifecycleState = "UPDATING"
		// Keep polling
		if uerr := r.Status().Update(ctx, ociPostgresCluster); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	default:
		ociPostgresCluster.Status.LifecycleState = "FAILED"
		_ = r.Status().Update(ctx, ociPostgresCluster)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
}

func (r *DBClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1beta1.OciPostgresCluster{}).Complete(r)
}

// createDbSystem creates an OCI PostgreSQL DB System and returns its OCID.
func (r *DBClusterReconciler) createDbSystem(
	ctx context.Context,
	PostgresDBCluster *v1beta1.OciPostgresCluster,
	creds *adminCreds,
	log logr.Logger,
	pgClient *ocipostgresdbsystem.OciPostgresClient,
	crUID string,
) (string, error) {
	compartmentID := PostgresDBCluster.Spec.CompartmentId
	if compartmentID == "" {
		return "", fmt.Errorf("spec.compartmentId is required")
	}
	subnetId := PostgresDBCluster.Spec.NetworkDetails.SubnetId
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
	idCfg.AuthType = Ptr(principals.AuthenticationType(PostgresDBCluster.Spec.AuthType))

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
		DbVersion:     common.String(PostgresDBCluster.Spec.DbVersion),
		Shape:         common.String(PostgresDBCluster.Spec.Shape),
		NetworkDetails: &psql.NetworkDetails{
			SubnetId: common.String(subnetId),
		},
		Credentials: &psql.Credentials{
			Username: common.String(creds.Username),
			PasswordDetails: psql.PlainTextPasswordDetails{
				Password: common.String(creds.Password),
			},
		},
		StorageDetails:          storageDetails,
		InstanceCount:           common.Int(PostgresDBCluster.Spec.InstanceCount),
		InstanceOcpuCount:       common.Int(*PostgresDBCluster.Spec.InstanceOcpuCount),
		InstanceMemorySizeInGBs: common.Int(*PostgresDBCluster.Spec.InstanceMemorySizeInGbs),
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

func (r *DBClusterReconciler) getOrCreateAdminCred(ctx context.Context, ns string, name string) (*adminCreds, error) {
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

func addFinalizerIfNeeded(cr *v1beta1.OciPostgresCluster) bool {
	for _, f := range cr.Finalizers {
		if f == finalizerName {
			return false
		}
	}
	cr.Finalizers = append(cr.Finalizers, finalizerName)
	return true
}

func (r *DBClusterReconciler) reconcileDelete(
	ctx context.Context,
	log logr.Logger,
	cluster *v1beta1.OciPostgresCluster,
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

	// delete admin secret (ignore NotFound)
	if ns := cluster.Name; ns != "" {
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

func removeFinalizer(cr *v1beta1.OciPostgresCluster) {
	fs := make([]string, 0, len(cr.Finalizers))
	for _, f := range cr.Finalizers {
		if f != finalizerName {
			fs = append(fs, f)
		}
	}
	cr.Finalizers = fs
}

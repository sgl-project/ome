package ocipostgresdbinstance

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	PostgreSQLUtil "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/ocipostgres/utils"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	ocipostgresdbsystem "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ocidbsystem"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/psql"

	"k8s.io/client-go/tools/record"

	v1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type cluserAdminCreds struct {
	Admin     string
	AdminPass string
}

type appCreds struct {
	User       string
	Password   string
	SecretName string
}

const (
	secretKeyUsername     = "username"
	secretKeyPassword     = "password"
	finalizerName         = "postgresdbinstance/finalizer"
	reasonEndpointMissing = "EndpointNotReady"
	reasonAdminSecret     = "AdminSecretError"
	reasonAppSecret       = "AppSecretError"
	reasonCreateDB        = "CreateDatabaseError"
	reasonGrants          = "GrantError"
	reasonSuccess         = "Ready"
	eventTypeWarning      = corev1.EventTypeWarning
)

var (
	newPGClient = func(cfg *ocipostgresdbsystem.Config) (*ocipostgresdbsystem.OciPostgresClient, error) {
		return ocipostgresdbsystem.NewOciPostgreSQLClient(cfg)
	}
)

// +kubebuilder:rbac:groups=ome.io,resources=ocipostgresdbinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ome.io,resources=ocipostgresdbinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ome.io,resources=ocipostgresdbinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch;watch

type PostgresDBInstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Log      logr.Logger
}

func (r *PostgresDBInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PostgresDBInstance", req.NamespacedName)

	dbInstance := &v1beta1.OciPostgresDBInstance{}
	if err := r.Get(ctx, req.NamespacedName, dbInstance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("reconcile OciPostgres DB instance", "namespace", req.Namespace, "name", req.Name)

	// create ocipgClient with a proper logger adapter
	ocipostgresDbConfig, err := ocipostgresdbsystem.NewConfig(
		ocipostgresdbsystem.WithAnotherLog(logging.ForLogr(r.Log)),
	)
	if err != nil {
		r.recordWarn(dbInstance, "ConfigError", fmt.Sprintf("failed to create OCI PG config: %v", err))
		r.setCondition(ctx, dbInstance, metav1.ConditionFalse, "ConfigError", err.Error(), true)
		return ctrl.Result{}, fmt.Errorf("failed to create ocipostgresDbConfig config: %w", err)
	}
	ocipostgresDbConfig.AuthType = Ptr(principals.AuthenticationType(dbInstance.Spec.AuthType))
	pgClient, err := newPGClient(ocipostgresDbConfig)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create postgres client: %w", err)
	}

	// handle deletion via finalizer
	if !dbInstance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, dbInstance, pgClient)
	}

	if addFinalizerIfNeeded(dbInstance) {
		if err := r.Update(ctx, dbInstance); err != nil {
			log.Error(err, "Failed to add finalizer", "name", dbInstance.Name)
			r.recordWarn(dbInstance, "AddFinalizerError", fmt.Sprintf("failed to add finalizer: %v", err))
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Resolve endpoint from cluster ID
	clusterID := dbInstance.Spec.DBClusterId
	host, port, ca, fqdn, err := getPrimaryEndpoint(ctx, clusterID, pgClient)
	if err != nil || host == "" || port == 0 {
		if err != nil {
			r.recordWarn(dbInstance, "Fetch Endpoint error", fmt.Sprintf("failed to fetch endpoint: %v", err))
		}
		dbInstance.Status.IsReady = false
		r.setCondition(ctx, dbInstance, metav1.ConditionFalse, reasonEndpointMissing, "Primary endpoint not ready", true)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	adm, err := getClusterAdminCreds(ctx, r.Client, dbInstance.Spec.AdminSecretNamespace, dbInstance.Spec.AdminSecretName)
	if err != nil {
		r.recordWarn(dbInstance, reasonAdminSecret,
			fmt.Sprintf("failed to read admin secret %s/%s: %v", dbInstance.Spec.AdminSecretNamespace, dbInstance.Spec.AdminSecretName, err))
		dbInstance.Status.IsReady = false
		r.setCondition(ctx, dbInstance, metav1.ConditionFalse, reasonAdminSecret, "Admin secret missing or invalid", true)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// db role names, user is app_dbInstance_owner, secret name fix pattern, password random generated
	dbName := dbInstance.Name
	appUser := fmt.Sprintf("app_%s_owner", dbInstance.Name)
	appSecretName := fmt.Sprintf("%s-db-credentials", dbInstance.Name)

	// Ensure per-app Secret
	app, err := ensureAppUserAndSecret(ctx, r.Client, req.Namespace, appSecretName, dbName)
	if err != nil {
		r.recordWarn(dbInstance, reasonAppSecret, fmt.Sprintf("ensure app secret failed: %v", err))
		dbInstance.Status.IsReady = false
		r.setCondition(ctx, dbInstance, metav1.ConditionFalse, reasonAppSecret, "Failed to ensure app secret", true)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Connect & create DB
	log.Info("creating db instance", "user", adm.Admin, "host", host, "fqdn", fqdn, "port", port)
	if err := createDBIfMissing(ctx, host, port, adm.Admin, adm.AdminPass, ca, fqdn, dbName, log); err != nil {
		r.recordWarn(dbInstance, reasonCreateDB, fmt.Sprintf("create DB failed: %v", err), "db", dbName)
		dbInstance.Status.IsReady = false
		r.setCondition(ctx, dbInstance, metav1.ConditionFalse, reasonCreateDB, "Failed to create database", true)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	log.Info("creating db instance succeed", "db-name", dbName)

	// Ensure role/grants
	if err := ensureRoleAndGrants(ctx, host, port, adm.Admin, adm.AdminPass, ca, fqdn, dbName, appUser, app.Password, log); err != nil {
		r.recordWarn(dbInstance, reasonGrants, fmt.Sprintf("ensure role/grants failed: %v", err))
		dbInstance.Status.IsReady = false
		r.setCondition(ctx, dbInstance, metav1.ConditionFalse, reasonGrants, "Failed to grant role/privileges", true)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	log.Info("Role Granted Succeed")

	// Status update
	dbInstance.Status.Endpoint = host
	dbInstance.Status.IsReady = true
	dbInstance.Status.LifecycleState = "READY"
	dbInstance.Status.AppUserSecretName = app.SecretName
	dbInstance.Status.DatabaseName = dbName
	r.setCondition(ctx, dbInstance, metav1.ConditionTrue, reasonSuccess, "DBInstance ready", true)
	if err := r.Status().Update(ctx, dbInstance); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

func (r *PostgresDBInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1beta1.OciPostgresDBInstance{}).Complete(r)
}

func getPrimaryEndpoint(ctx context.Context, clusterID string, pgClient *ocipostgresdbsystem.OciPostgresClient) (string, int, string, string, error) {
	cd, err := pgClient.GetConnectionDetails(ctx, psql.GetConnectionDetailsRequest{
		DbSystemId: common.String(clusterID),
	})
	if err == nil && cd.ConnectionDetails.PrimaryDbEndpoint != nil {
		ep := cd.ConnectionDetails.PrimaryDbEndpoint
		fqdn := *ep.Fqdn
		host := *ep.IpAddress
		var port int
		if ep.Port != nil {
			port = *ep.Port
		}
		ca := *cd.ConnectionDetails.CaCertificate
		return host, port, ca, fqdn, nil
	}
	return "", 0, "", "", fmt.Errorf("no endpoints found for dbSystem %s", clusterID)
}

func getClusterAdminCreds(ctx context.Context, c client.Client, ns, name string) (*cluserAdminCreds, error) {
	s := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, s); err != nil {
		return nil, fmt.Errorf("get admin secret %s/%s: %w", ns, name, err)
	}
	u := string(s.Data[secretKeyUsername])
	p := string(s.Data[secretKeyPassword])
	if u == "" || p == "" {
		return nil, fmt.Errorf("admin secret %s/%s missing %q or %q", ns, name, secretKeyUsername, secretKeyPassword)
	}
	return &cluserAdminCreds{Admin: u, AdminPass: p}, nil
}

func createDBIfMissing(
	ctx context.Context, host string, port int,
	adminUser, adminPass, caPEM, fqdn, newDB string, log logr.Logger,
) error {
	// connect using admin credential and postgres default generated db instance
	cfg, err := tlsPgConfig(host, port, adminUser, adminPass, "postgres", caPEM, fqdn)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*cfg)
	log.Info("Open DB succeed")
	defer func() {
		if cerr := db.Close(); cerr != nil {
			log.Error(cerr, "error closing DB")
		}
	}()

	log.Info("established DB connector",
		"host(dialIP)", host,
		"port", port,
		"serverName", fqdn,
		"tlsSet", cfg.TLSConfig != nil, "db", newDB)

	pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		log.Info("PING failed")
		return err
	}
	log.Info("PING Succeed")

	// Create new db instance
	q := fmt.Sprintf(`CREATE DATABASE "%s"`, newDB)
	if _, err := db.ExecContext(ctx, q); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P04" { // duplicate_database
			return nil
		}
		return err
	}
	return nil
}

func tlsPgConfig(host string, port int, user, pass, dbName, caPEM, fqdn string) (*pgx.ConnConfig, error) {
	// local testing
	// host = "127.0.0.1"
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=verify-full",
		host, port, user, pass, dbName)
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, fmt.Errorf("invalid CA certificate PEM")
	}
	cfg.TLSConfig = &tls.Config{
		RootCAs:    pool,
		ServerName: fqdn, // verify-full with FQDN
		MinVersion: tls.VersionTLS12,
	}
	return cfg, nil
}

// App Secret creation & reuse
func ensureAppUserAndSecret(
	ctx context.Context,
	c client.Client,
	appNS, secretName,
	dbName string,
) (*appCreds, error) {
	existing := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: appNS, Name: secretName}, existing); err == nil {
		return &appCreds{
			User:       string(existing.Data[secretKeyUsername]),
			Password:   string(existing.Data[secretKeyPassword]),
			SecretName: existing.Name,
		}, nil
	} else if !apierr.IsNotFound(err) {
		return nil, fmt.Errorf("read app secret: %w", err)
	}

	user := fmt.Sprintf("app_%s_owner", dbName)
	pass := PostgreSQLUtil.GenerateRandomPassword(20)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: appNS,
			Labels:    map[string]string{"app": "postgres-instance"},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			secretKeyUsername: user,
			secretKeyPassword: pass,
		},
	}
	if err := c.Create(ctx, sec); err != nil {
		return nil, fmt.Errorf("create app secret: %w", err)
	}
	return &appCreds{User: user, Password: pass, SecretName: secretName}, nil
}

func ensureRoleAndGrants(
	ctx context.Context,
	host string, port int,
	adminUser, adminPass, caPEM, fqdn string,
	dbName string,
	appUser, appPass string,
	log logr.Logger,
) error {
	// connect to the target logical DB
	cfg, err := tlsPgConfig(host, port, adminUser, adminPass, dbName, caPEM, fqdn)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*cfg)
	defer db.Close()

	// Ensure LOGIN role exists
	var exists bool
	if err := db.QueryRowContext(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = $1)`,
		appUser,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check role exists: %w", err)
	}
	if !exists {
		_, err = db.ExecContext(ctx,
			fmt.Sprintf(
				`CREATE ROLE %s LOGIN PASSWORD %s NOSUPERUSER NOCREATEDB NOCREATEROLE`,
				quoteIdent(appUser), quoteLiteral(appPass),
			),
		)
		if err != nil {
			return fmt.Errorf("create role: %w", err)
		}
	} else {
		_, err = db.ExecContext(ctx,
			fmt.Sprintf(
				`ALTER ROLE %s WITH PASSWORD %s`,
				quoteIdent(appUser), quoteLiteral(appPass),
			),
		)
		if err != nil {
			return fmt.Errorf("alter role password: %w", err)
		}
	}

	// Allow the user to connect to this database
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`GRANT CONNECT ON DATABASE %s TO %s`, quoteIdent(dbName), quoteIdent(appUser)),
	); err != nil {
		return fmt.Errorf("grant CONNECT: %w", err)
	}

	// Grant R/W capability in the default schema (`public`)
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`GRANT USAGE, CREATE ON SCHEMA public TO %s`, quoteIdent(appUser)),
	); err != nil {
		return fmt.Errorf("grant USAGE/CREATE on schema public: %w", err)
	}

	// Grant R/W on all existing tables/sequences in public
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s`, quoteIdent(appUser)),
	); err != nil {
		return fmt.Errorf("grant on existing tables: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO %s`, quoteIdent(appUser)),
	); err != nil {
		return fmt.Errorf("grant on existing sequences: %w", err)
	}

	log.Info("app user R/W grants applied",
		"db", dbName, "appUser", appUser)
	return nil
}

func addFinalizerIfNeeded(cr *v1beta1.OciPostgresDBInstance) bool {
	for _, f := range cr.Finalizers {
		if f == finalizerName {
			return false
		}
	}
	cr.Finalizers = append(cr.Finalizers, finalizerName)
	return true
}

func removeFinalizer(cr *v1beta1.OciPostgresDBInstance) {
	out := make([]string, 0, len(cr.Finalizers))
	for _, f := range cr.Finalizers {
		if f != finalizerName {
			out = append(out, f)
		}
	}
	cr.Finalizers = out
}

func (r *PostgresDBInstanceReconciler) reconcileDelete(
	ctx context.Context,
	log logr.Logger,
	inst *v1beta1.OciPostgresDBInstance,
	pgClient *ocipostgresdbsystem.OciPostgresClient,
) (ctrl.Result, error) {
	// only act if our finalizer is present
	hasFinalizer := false
	for _, f := range inst.Finalizers {
		if f == finalizerName {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		return ctrl.Result{}, nil
	}
	// delete the db instance
	clusterID := strings.TrimSpace(inst.Spec.DBClusterId)
	if clusterID != "" {
		if host, port, ca, fqdn, err := getPrimaryEndpoint(ctx, clusterID, pgClient); err == nil {
			if adm, aerr := getClusterAdminCreds(ctx, r.Client, inst.Spec.AdminSecretNamespace, inst.Spec.AdminSecretName); aerr == nil {
				dbName := inst.Name
				appUser := fmt.Sprintf("app_%s_owner", inst.Name)
				_ = dropRoleAndDatabase(ctx, host, port, adm.Admin, adm.AdminPass, ca, fqdn, dbName, appUser, log)
			}
		}
	}

	// delete the app secret in <inst.Name> namespace
	appSecretName := fmt.Sprintf("%s-db-credentials", inst.Name)
	if err := r.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      appSecretName,
		Namespace: inst.Name,
	}}); err != nil && !apierr.IsNotFound(err) {
		log.Error(err, "delete app secret failed; requeue", "ns", inst.Name, "name", appSecretName)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// remove finalizer to allow deletion to finish
	removeFinalizer(inst)
	if err := r.Update(ctx, inst); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	log.Info("Finalizer completed, CR can be removed")
	return ctrl.Result{}, nil
}
func dropRoleAndDatabase(
	ctx context.Context,
	host string, port int,
	adminUser, adminPass, caPEM, fqdn string,
	dbName, appUser string,
	log logr.Logger,
) error {
	// Drop DB role and database can't do on target specific db instance, instead should connect to postgres with admin user
	cfg, err := tlsPgConfig(host, port, adminUser, adminPass, "postgres", caPEM, fqdn)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*cfg)
	defer db.Close()

	//force-closes all active connections to a specific database before drop it
	_, _ = db.ExecContext(ctx, fmt.Sprintf(
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s`,
		quoteLiteral(dbName),
	))

	// drop db instance
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, quoteIdent(dbName))); err != nil {
		log.Error(err, "drop database failed (ignored)", "db", dbName)
	}
	// drop role
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP ROLE IF EXISTS %s`, quoteIdent(appUser))); err != nil {
		log.Error(err, "drop role failed (ignored)", "role", appUser)
	}
	return nil
}

func (r *PostgresDBInstanceReconciler) setCondition(
	ctx context.Context,
	obj *v1beta1.OciPostgresDBInstance,
	status metav1.ConditionStatus,
	reason, msg string,
	update bool,
) {
	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  status,
		Reason:  reason,
		Message: msg,
	})
	if update {
		if err := r.Status().Update(ctx, obj); err != nil {
			r.Log.WithValues("name", obj.Name, "ns", obj.Namespace, "reason", reason).
				Error(err, "failed to update status condition")
		}
	}
}

func (r *PostgresDBInstanceReconciler) recordWarn(
	obj client.Object, reason, msg string, keysAndValues ...any,
) {
	err := errors.New(msg)
	r.Log.WithValues("reason", reason).WithValues(keysAndValues...).Error(err, msg)
	if r.Recorder != nil {
		r.Recorder.Event(obj, eventTypeWarning, reason, msg)
	}
}

func Ptr[T any](v T) *T { return &v }

// quoteIdent safely quotes a PostgreSQL **identifier** (e.g., role, schema, table, database names).
//   - Identifiers are wrapped in double quotes "...".
//   - Any embedded double quotes are escaped by doubling them per SQL standard: " -> "".
//   - Use this when you MUST construct dynamic SQL for identifiers (which cannot be parameterized).
//     Example: fmt.Sprintf(`CREATE ROLE %s`, quoteIdent(roleName))
//
// NOTE: Quoted identifiers are case-sensitive in PostgreSQL. "AppUser" != appuser.
func quoteIdent(id string) string {
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

// quoteLiteral safely quotes a PostgreSQL **string literal** (data value).
//   - Literals are wrapped in single quotes '...'.
//   - Any embedded single quotes are escaped by doubling them: ' -> ”.
//   - Prefer bind parameters ($1, $2, …) whenever possible; only use this when
//     parameters are not supported (e.g., parts of DDL or PASSWORD in CREATE ROLE).
//     Example: fmt.Sprintf(`ALTER ROLE %s WITH PASSWORD %s`, quoteIdent(user), quoteLiteral(pass))
func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

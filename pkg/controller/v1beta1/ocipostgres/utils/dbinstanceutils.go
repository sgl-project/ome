package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"

	ocipostgresdbsystem "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ocidbsystem"
	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/psql"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type appCreds struct {
	User       string
	Password   string
	SecretName string
}

const (
	secretKeyUsername = "username"
	secretKeyPassword = "password"
)

func GetPrimaryEndpoint(ctx context.Context, clusterID string, pgClient *ocipostgresdbsystem.OciPostgresClient) (string, int, string, string, error) {
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

func CreateDBIfMissing(
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
func EnsureAppUserAndSecret(
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
	pass := GenerateRandomPassword(20)

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

func EnsureRoleAndGrants(
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

	log.Info("app user R/W grants applied",
		"db", dbName, "appUser", appUser)
	return nil
}

func DropRoleAndDatabase(
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

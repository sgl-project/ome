// Package workloadcluster implements the WorkloadCluster registry
// controller: it resolves each cluster's kubeconfig, connects, and reports a
// Ready (reachable) condition. Capacity is never read here.
package workloadcluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ExecCredentialPolicy controls whether a kubeconfig may use an exec credential
// plugin to authenticate to a workload cluster, and which commands are allowed.
// The zero value denies exec (the safe default).
type ExecCredentialPolicy struct {
	// Allowed turns exec credential plugins on. Default false.
	Allowed bool
	// AllowedCommands is the set of permitted plugin command basenames
	// (e.g. "aws", "gke-gcloud-auth-plugin", "kubelogin"). Empty => none allowed.
	AllowedCommands []string
}

func (p ExecCredentialPolicy) commandAllowed(cmd string) bool {
	base := filepath.Base(cmd)
	for _, a := range p.AllowedCommands {
		if a == base || a == cmd {
			return true
		}
	}
	return false
}

// RESTConfigFromKubeConfig parses raw kubeconfig bytes into a *rest.Config and
// rejects credential shapes that are unsafe to run from the control plane — a
// stored kubeconfig is a cross-cluster blast radius.
// It validates twice, mirroring Kueue's validateKubeconfig: once over the raw
// kubeconfig (every cluster/user, before flattening) and once over the
// flattened rest.Config.
func RESTConfigFromKubeConfig(raw []byte, exec ExecCredentialPolicy) (*rest.Config, error) {
	if err := validateKubeConfigBytes(raw, exec); err != nil {
		return nil, err
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	if err := validateRESTConfig(cfg, exec); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validateKubeConfigBytes inspects the raw kubeconfig at the API-config level
// (across ALL clusters and users, not just the current context) and rejects:
// per-user on-disk token files, per-cluster insecure TLS, CA file paths
// (require inline certificate-authority-data), and a missing server URL.
func validateKubeConfigBytes(raw []byte, exec ExecCredentialPolicy) error {
	cfg, err := clientcmd.Load(raw)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}
	for name, authInfo := range cfg.AuthInfos {
		if authInfo.Exec != nil && !exec.Allowed {
			return fmt.Errorf("kubeconfig user %q uses an exec credential plugin, which is not allowed (enable --allow-exec-credentials)", name)
		}
		if authInfo.TokenFile != "" {
			return fmt.Errorf("kubeconfig user %q references an on-disk token file, which is not allowed", name)
		}
	}
	for name, cluster := range cfg.Clusters {
		if cluster.InsecureSkipTLSVerify {
			return fmt.Errorf("kubeconfig disables TLS verification for cluster %q, which is not allowed", name)
		}
		if cluster.CertificateAuthority != "" {
			return fmt.Errorf("kubeconfig cluster %q uses a certificate-authority file path; use certificate-authority-data instead", name)
		}
		if cluster.Server == "" {
			return fmt.Errorf("kubeconfig cluster %q has no server URL", name)
		}
	}
	return nil
}

// validateRESTConfig rejects unsafe settings on the flattened rest.Config:
// exec/auth-provider plugins, on-disk token files, basic auth, disabled TLS, a
// CA file path, or an untrusted server endpoint. BearerToken (service-account
// token) and inline CA data are allowed.
func validateRESTConfig(cfg *rest.Config, exec ExecCredentialPolicy) error {
	switch {
	case cfg.ExecProvider != nil:
		if !exec.Allowed {
			return errors.New("kubeconfig uses an exec credential plugin, which is not allowed (enable --allow-exec-credentials)")
		}
		if !exec.commandAllowed(cfg.ExecProvider.Command) {
			return fmt.Errorf("exec credential plugin command %q is not in the allowed list", cfg.ExecProvider.Command)
		}
	case cfg.AuthProvider != nil:
		return errors.New("kubeconfig uses an auth-provider plugin, which is not allowed")
	case cfg.BearerTokenFile != "":
		return errors.New("kubeconfig references an on-disk token file, which is not allowed")
	case cfg.Username != "" || cfg.Password != "":
		return errors.New("kubeconfig uses basic auth, which is not allowed")
	case cfg.TLSClientConfig.Insecure:
		return errors.New("kubeconfig disables TLS verification, which is not allowed")
	case cfg.TLSClientConfig.CAFile != "":
		return errors.New("kubeconfig uses a CA file path; use inline CA data instead")
	case !isValidHostname(cfg.Host):
		return errors.New("kubeconfig server endpoint is not a trusted host")
	}
	return nil
}

// isValidHostname reports whether server is an https endpoint whose host is a
// valid IP or DNS name — guards against malformed/untrusted endpoints. Mirrors
// Kueue's isValidHostname (url.Parse -> SplitHostPort -> ParseIP / DNS1123).
func isValidHostname(server string) bool {
	u, err := url.Parse(strings.ToLower(server))
	if err != nil {
		return false
	}
	// A workload-cluster API endpoint carries a cross-cluster bearer credential;
	// it MUST be reached over TLS. Reject plaintext (http) and scheme-less
	// endpoints so a credential is never sent in the clear.
	if u.Scheme != "https" {
		return false
	}
	host := u.Host
	if h, _, err := net.SplitHostPort(u.Host); err == nil {
		host = h
	}
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	return len(validation.IsDNS1123Subdomain(host)) == 0
}

// probeViaServerVersion is the default reachability probe: build a discovery
// client and fetch the remote /version. Cheap and needs no extra RBAC. It is
// assigned to Reconciler.Probe by SetupWithManager and overridden in tests.
func probeViaServerVersion(_ context.Context, raw []byte, exec ExecCredentialPolicy) error {
	cfg, err := RESTConfigFromKubeConfig(raw, exec)
	if err != nil {
		return err
	}
	cfg.Timeout = 10 * time.Second
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("build discovery client: %w", err)
	}
	if _, err := dc.ServerVersion(); err != nil {
		return fmt.Errorf("connect to cluster: %w", err)
	}
	return nil
}

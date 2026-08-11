package workloadcluster

import (
	"testing"

	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestValidateRESTConfig(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *rest.Config
		exec    ExecCredentialPolicy
		wantErr bool
	}{
		{"clean", &rest.Config{Host: "https://h", BearerToken: "t", TLSClientConfig: rest.TLSClientConfig{CAData: []byte("ca")}}, ExecCredentialPolicy{}, false},
		{"clean ip host", &rest.Config{Host: "https://10.0.0.1:6443", BearerToken: "t"}, ExecCredentialPolicy{}, false},
		{"exec provider denied", &rest.Config{ExecProvider: &clientcmdapi.ExecConfig{Command: "aws"}}, ExecCredentialPolicy{}, true},
		{"bearer token file", &rest.Config{BearerTokenFile: "/var/run/token"}, ExecCredentialPolicy{}, true},
		{"insecure TLS", &rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}}, ExecCredentialPolicy{}, true},
		{"basic auth", &rest.Config{Host: "https://h", Username: "u", Password: "p"}, ExecCredentialPolicy{}, true},
		{"ca file path", &rest.Config{Host: "https://h", TLSClientConfig: rest.TLSClientConfig{CAFile: "/etc/ca.crt"}}, ExecCredentialPolicy{}, true},
		{"untrusted host", &rest.Config{Host: "https://bad_host"}, ExecCredentialPolicy{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRESTConfig(tc.cfg, tc.exec)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateRESTConfig() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestValidateKubeConfigBytes pins the raw-kubeconfig-level checks (run across
// ALL clusters/users, before flattening): per-user token files, per-cluster
// insecure TLS, and CA file paths are rejected; a server URL is required.
func TestValidateKubeConfigBytes(t *testing.T) {
	const clean = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://example.com
    certificate-authority-data: ZHVtbXk=
contexts:
- name: ctx
  context: {cluster: c, user: u}
current-context: ctx
users:
- name: u
  user:
    token: abc
`
	const tokenFile = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster: {server: https://example.com, certificate-authority-data: ZHVtbXk=}
users:
- name: u
  user:
    tokenFile: /var/run/secrets/token
`
	const insecure = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://example.com
    insecure-skip-tls-verify: true
`
	const caPath = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://example.com
    certificate-authority: /etc/ca.crt
`
	const noServer = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    certificate-authority-data: ZHVtbXk=
`
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"clean", clean, false},
		{"per-user token file", tokenFile, true},
		{"per-cluster insecure", insecure, true},
		{"ca file path", caPath, true},
		{"missing server", noServer, true},
		{"malformed", "this is not a kubeconfig", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateKubeConfigBytes([]byte(tc.raw), ExecCredentialPolicy{})
			if (err != nil) != tc.wantErr {
				t.Errorf("validateKubeConfigBytes() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestIsValidHostname_RequiresHTTPS pins that a workload-cluster endpoint must
// be reached over TLS: a plaintext (http) or scheme-less server is rejected so
// the cross-cluster bearer credential is never sent in the clear.
func TestIsValidHostname_RequiresHTTPS(t *testing.T) {
	cases := []struct {
		name   string
		server string
		want   bool
	}{
		{"https dns", "https://api.example.com:6443", true},
		{"https ip", "https://10.0.0.1:6443", true},
		{"http rejected", "http://api.example.com:6443", false},
		{"http ip rejected", "http://10.0.0.1:6443", false},
		{"scheme-less rejected", "api.example.com:6443", false},
		{"empty rejected", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidHostname(tc.server); got != tc.want {
				t.Errorf("isValidHostname(%q) = %v, want %v", tc.server, got, tc.want)
			}
		})
	}
}

func TestRESTConfigFromKubeConfig_BadBytes(t *testing.T) {
	if _, err := RESTConfigFromKubeConfig([]byte("not-a-kubeconfig"), ExecCredentialPolicy{}); err == nil {
		t.Errorf("expected error for malformed kubeconfig, got nil")
	}
}

func TestRESTConfig_ExecPolicy(t *testing.T) {
	const execKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://example.com
    certificate-authority-data: ZHVtbXk=
contexts:
- name: ctx
  context: {cluster: c, user: u}
current-context: ctx
users:
- name: u
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args: ["eks","get-token","--cluster-name","c"]
`
	cases := []struct {
		name    string
		exec    ExecCredentialPolicy
		wantErr bool
	}{
		{"deny exec (zero policy)", ExecCredentialPolicy{}, true},
		{"allow aws command", ExecCredentialPolicy{Allowed: true, AllowedCommands: []string{"aws"}}, false},
		{"command not in allowlist", ExecCredentialPolicy{Allowed: true, AllowedCommands: []string{"kubelogin"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RESTConfigFromKubeConfig([]byte(execKubeconfig), tc.exec)
			if (err != nil) != tc.wantErr {
				t.Errorf("RESTConfigFromKubeConfig() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

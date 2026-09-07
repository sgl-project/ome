package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

const (
	alfredServiceAccount = "ome-alfred"
	alfredNamespace      = "ome"
)

func TestRenderedAlfredWriteBoundary(t *testing.T) {
	objects := renderAlfredManifests(t)

	t.Run("pre-created leader Lease is spec-less", func(t *testing.T) {
		lease := findManifest(t, objects, "Lease", leaderElectionID)
		if lease.GetNamespace() != alfredNamespace {
			t.Fatalf("leader Lease namespace = %q, want %q", lease.GetNamespace(), alfredNamespace)
		}
		if _, hasSpec := lease.Object["spec"]; hasSpec {
			t.Fatalf("leader Lease must be pre-created without spec: %+v", lease.Object["spec"])
		}
	})

	role := manifestAs[rbacv1.Role](t, findManifest(t, objects, "Role", alfredServiceAccount))
	clusterRole := manifestAs[rbacv1.ClusterRole](t, findManifest(t, objects, "ClusterRole", alfredServiceAccount))
	roleBinding := manifestAs[rbacv1.RoleBinding](t, findManifest(t, objects, "RoleBinding", alfredServiceAccount))
	clusterRoleBinding := manifestAs[rbacv1.ClusterRoleBinding](t, findManifest(t, objects, "ClusterRoleBinding", alfredServiceAccount))

	t.Run("bindings target only ome-alfred", func(t *testing.T) {
		if role.Namespace != alfredNamespace || roleBinding.Namespace != alfredNamespace {
			t.Fatalf("Role/RoleBinding namespaces = %q/%q, want %q",
				role.Namespace, roleBinding.Namespace, alfredNamespace)
		}
		assertBinding(t, roleBinding.RoleRef, roleBinding.Subjects, "Role")
		assertBinding(t, clusterRoleBinding.RoleRef, clusterRoleBinding.Subjects, "ClusterRole")

		var applicable []string
		for _, object := range objects {
			switch object.GetKind() {
			case "RoleBinding":
				binding := manifestAs[rbacv1.RoleBinding](t, object)
				if subjectsCoverAlfred(binding.Subjects, binding.Namespace) {
					applicable = append(applicable, "RoleBinding|"+binding.Namespace+"|"+binding.Name+"|"+
						binding.RoleRef.Kind+"|"+binding.RoleRef.Name)
				}
			case "ClusterRoleBinding":
				binding := manifestAs[rbacv1.ClusterRoleBinding](t, object)
				if subjectsCoverAlfred(binding.Subjects, "") {
					applicable = append(applicable, "ClusterRoleBinding|<cluster>|"+binding.Name+"|"+
						binding.RoleRef.Kind+"|"+binding.RoleRef.Name)
				}
			}
		}
		want := []string{
			"ClusterRoleBinding|<cluster>|ome-alfred|ClusterRole|ome-alfred",
			"RoleBinding|ome|ome-alfred|Role|ome-alfred",
		}
		sort.Strings(applicable)
		if !reflect.DeepEqual(applicable, want) {
			t.Fatalf("rendered bindings covering ome/ome-alfred = %v, want exactly %v", applicable, want)
		}
	})
	if clusterRole.AggregationRule != nil {
		t.Fatalf("ome-alfred ClusterRole must not acquire dynamic aggregated rules: %+v", clusterRole.AggregationRule)
	}

	// Alfred receives both bindings, so authorization is the union. Auditing
	// either object alone could miss a broad grant on the other one.
	effectiveRules := append(append([]rbacv1.PolicyRule(nil), role.Rules...), clusterRole.Rules...)
	t.Run("effective writes exactly match the current boundary", func(t *testing.T) {
		got := append(
			effectiveWriteTuples("Role", role.Rules),
			effectiveWriteTuples("ClusterRole", clusterRole.Rules)...,
		)
		want := []string{
			"ClusterRole|core|events|create|<all>",
			"ClusterRole|core|events|patch|<all>",
			"Role|coordination.k8s.io|leases|update|alfred.ome.io",
			"Role|core|configmaps|update|alfred-recommendations",
		}
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("effective write tuples =\n  %s\nwant exactly =\n  %s",
				strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
	})
	t.Run("effective Lease verbs are name scoped", func(t *testing.T) {
		verbs := []string{"get", "update", "create", "list", "watch", "patch", "delete", "deletecollection"}
		tests := []struct {
			name    string
			lease   string
			allowed map[string]bool
		}{
			{name: "leader", lease: leaderElectionID, allowed: map[string]bool{"get": true, "update": true}},
			{name: "unrelated", lease: "unrelated-lease", allowed: map[string]bool{}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				for _, verb := range verbs {
					got := allowsLease(effectiveRules, verb, test.lease)
					if got != test.allowed[verb] {
						t.Errorf("effective permission %s leases/%s = %t, want %t",
							verb, test.lease, got, test.allowed[verb])
					}
				}
			})
		}
	})

	t.Run("InferenceService writes wait for the Dispatcher", func(t *testing.T) {
		for _, resource := range []string{"inferenceservices", "inferenceservices/status"} {
			for _, verb := range []string{"get", "list", "watch"} {
				if !allowsResource(effectiveRules, "ome.io", resource, verb) {
					t.Errorf("effective permission %s ome.io/%s = false, want read access", verb, resource)
				}
			}
			for _, verb := range []string{"create", "update", "patch", "delete", "deletecollection"} {
				if allowsResource(effectiveRules, "ome.io", resource, verb) {
					t.Errorf("effective permission %s ome.io/%s = true before Dispatcher and admission guard", verb, resource)
				}
			}
		}
	})

	t.Run("Nodes and Pods remain read-only", func(t *testing.T) {
		for _, resource := range []string{"nodes", "pods"} {
			for _, verb := range []string{"get", "list", "watch"} {
				if !allowsResource(effectiveRules, "", resource, verb) {
					t.Errorf("effective permission %s core/%s = false, want read access", verb, resource)
				}
			}
		}
		for _, resource := range []string{"nodes", "nodes/status", "pods", "pods/status", "pods/eviction"} {
			for _, verb := range []string{"create", "update", "patch", "delete", "deletecollection"} {
				if allowsResource(effectiveRules, "", resource, verb) {
					t.Errorf("effective permission %s core/%s = true, want read-only access", verb, resource)
				}
			}
		}
	})
}

func TestEffectiveWriteTuplesRetainsWildcardAndNonResourceGrants(t *testing.T) {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		},
		{
			NonResourceURLs: []string{"/debug/*"},
			Verbs:           []string{"post"},
		},
	}
	want := []string{
		"Role|*|*|*|<all>",
		"Role|non-resource|/debug/*|post|<not-applicable>",
	}
	if got := effectiveWriteTuples("Role", rules); !reflect.DeepEqual(got, want) {
		t.Fatalf("effectiveWriteTuples() = %v, want %v", got, want)
	}
}

func TestSubjectsCoverAlfred(t *testing.T) {
	tests := []struct {
		name             string
		subject          rbacv1.Subject
		bindingNamespace string
		want             bool
	}{
		{name: "exact service account", subject: rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: "ome", Name: "ome-alfred"}, want: true},
		{name: "service account defaults to binding namespace", subject: rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "ome-alfred"}, bindingNamespace: "ome", want: true},
		{name: "canonical user", subject: rbacv1.Subject{Kind: rbacv1.UserKind, Name: "system:serviceaccount:ome:ome-alfred"}, want: true},
		{name: "authenticated group", subject: rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:authenticated"}, want: true},
		{name: "all service accounts", subject: rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts"}, want: true},
		{name: "namespace service accounts", subject: rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:ome"}, want: true},
		{name: "other service account", subject: rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: "other", Name: "ome-alfred"}},
		{name: "unrelated group", subject: rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "developers"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := subjectsCoverAlfred([]rbacv1.Subject{test.subject}, test.bindingNamespace); got != test.want {
				t.Fatalf("subjectsCoverAlfred(%+v, %q) = %t, want %t",
					test.subject, test.bindingNamespace, got, test.want)
			}
		})
	}
}

func TestAllowsResourceRecognizesWildcardGrants(t *testing.T) {
	tests := []struct {
		name            string
		groups          []string
		resources       []string
		verbs           []string
		checkedResource string
		want            bool
	}{
		{name: "global wildcard", groups: []string{"*"}, resources: []string{"*"}, verbs: []string{"*"}, checkedResource: "inferenceservices", want: true},
		{name: "wildcard API group", groups: []string{"*"}, resources: []string{"inferenceservices"}, verbs: []string{"patch"}, checkedResource: "inferenceservices", want: true},
		{name: "wildcard resource", groups: []string{"ome.io"}, resources: []string{"*"}, verbs: []string{"patch"}, checkedResource: "inferenceservices", want: true},
		{name: "wildcard resource covers subresource", groups: []string{"ome.io"}, resources: []string{"*"}, verbs: []string{"patch"}, checkedResource: "inferenceservices/status", want: true},
		{name: "wildcard verb", groups: []string{"ome.io"}, resources: []string{"inferenceservices"}, verbs: []string{"*"}, checkedResource: "inferenceservices", want: true},
		{name: "all subresources", groups: []string{"ome.io"}, resources: []string{"inferenceservices/*"}, verbs: []string{"patch"}, checkedResource: "inferenceservices/status", want: true},
		{name: "named subresource wildcard", groups: []string{"ome.io"}, resources: []string{"*/status"}, verbs: []string{"patch"}, checkedResource: "inferenceservices/status", want: true},
		{name: "two-part wildcard", groups: []string{"ome.io"}, resources: []string{"*/*"}, verbs: []string{"patch"}, checkedResource: "inferenceservices/status", want: true},
		{name: "other resource", groups: []string{"ome.io"}, resources: []string{"pods"}, verbs: []string{"patch"}, checkedResource: "inferenceservices", want: false},
		{name: "other subresource", groups: []string{"ome.io"}, resources: []string{"*/scale"}, verbs: []string{"patch"}, checkedResource: "inferenceservices/status", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules := []rbacv1.PolicyRule{{
				APIGroups: test.groups,
				Resources: test.resources,
				Verbs:     test.verbs,
			}}
			if got := allowsResource(rules, "ome.io", test.checkedResource, "patch"); got != test.want {
				t.Fatalf("allowsResource(%v, %q) = %t, want %t", test.resources, test.checkedResource, got, test.want)
			}
		})
	}
}

func renderAlfredManifests(t *testing.T) []*unstructured.Unstructured {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve manifests test path")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "alfred")
	kustomizationRaw, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read config/alfred kustomization: %v", err)
	}
	var kustomization struct {
		APIVersion string   `json:"apiVersion"`
		Kind       string   `json:"kind"`
		Namespace  string   `json:"namespace"`
		Resources  []string `json:"resources"`
	}
	if err := yaml.UnmarshalStrict(kustomizationRaw, &kustomization); err != nil {
		t.Fatalf("decode config/alfred kustomization: %v", err)
	}
	if len(kustomization.Resources) == 0 {
		t.Fatal("config/alfred kustomization has no resources")
	}

	var objects []*unstructured.Unstructured
	for _, resource := range kustomization.Resources {
		raw, err := os.ReadFile(filepath.Join(dir, resource))
		if err != nil {
			t.Fatalf("read rendered resource %q: %v", resource, err)
		}
		decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 4096)
		for {
			var object unstructured.Unstructured
			if err := decoder.Decode(&object); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("decode rendered resource %q: %v", resource, err)
			}
			if len(object.Object) == 0 {
				continue
			}
			if object.GetNamespace() == "" && object.GetKind() != "ClusterRole" && object.GetKind() != "ClusterRoleBinding" {
				object.SetNamespace(kustomization.Namespace)
			}
			objects = append(objects, &object)
		}
	}
	return objects
}

func findManifest(t *testing.T, objects []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	t.Helper()
	var found *unstructured.Unstructured
	for _, object := range objects {
		if object.GetKind() != kind || object.GetName() != name {
			continue
		}
		if found != nil {
			t.Fatalf("rendered duplicate %s/%s", kind, name)
		}
		found = object
	}
	if found == nil {
		t.Fatalf("rendered %s/%s not found", kind, name)
	}
	return found
}

func manifestAs[T any](t *testing.T, object *unstructured.Unstructured) *T {
	t.Helper()
	raw, err := json.Marshal(object.Object)
	if err != nil {
		t.Fatal(err)
	}
	var typed T
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("decode %s/%s: %v", object.GetKind(), object.GetName(), err)
	}
	return &typed
}

func assertBinding(t *testing.T, ref rbacv1.RoleRef, subjects []rbacv1.Subject, wantKind string) {
	t.Helper()
	if ref.APIGroup != rbacv1.GroupName || ref.Kind != wantKind || ref.Name != alfredServiceAccount {
		t.Fatalf("roleRef = %+v, want %s/%s", ref, wantKind, alfredServiceAccount)
	}
	want := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: alfredServiceAccount, Namespace: alfredNamespace}
	if len(subjects) != 1 || subjects[0] != want {
		t.Fatalf("subjects = %+v, want only %+v", subjects, want)
	}
}

func subjectsCoverAlfred(subjects []rbacv1.Subject, bindingNamespace string) bool {
	for _, subject := range subjects {
		switch subject.Kind {
		case rbacv1.ServiceAccountKind:
			namespace := subject.Namespace
			if namespace == "" {
				namespace = bindingNamespace
			}
			if namespace == alfredNamespace && subject.Name == alfredServiceAccount {
				return true
			}
		case rbacv1.UserKind:
			if subject.Name == "system:serviceaccount:"+alfredNamespace+":"+alfredServiceAccount {
				return true
			}
		case rbacv1.GroupKind:
			if subject.Name == "system:authenticated" || subject.Name == "system:serviceaccounts" ||
				subject.Name == "system:serviceaccounts:"+alfredNamespace {
				return true
			}
		}
	}
	return false
}

func allowsLease(rules []rbacv1.PolicyRule, verb, name string) bool {
	for _, rule := range rules {
		if !containsRBAC(rule.APIGroups, "coordination.k8s.io") ||
			!containsRBAC(rule.Resources, "leases") ||
			!containsRBAC(rule.Verbs, verb) {
			continue
		}
		// Empty resourceNames grants every name. Treat "*" conservatively as
		// broad too, so a manifest cannot evade this safety audit.
		if len(rule.ResourceNames) == 0 || containsRBAC(rule.ResourceNames, name) {
			return true
		}
	}
	return false
}

func containsRBAC(values []string, want string) bool {
	for _, value := range values {
		if value == want || value == "*" {
			return true
		}
	}
	return false
}

// effectiveWriteTuples expands every effective verb other than the three
// read-only Kubernetes verbs. Keeping wildcards and uncommon verbs in the
// result makes the rendered-manifest allowlist fail closed if a broad grant is
// introduced on either binding.
func effectiveWriteTuples(scope string, rules []rbacv1.PolicyRule) []string {
	var tuples []string
	for _, rule := range rules {
		resourceNames := "<all>"
		if len(rule.ResourceNames) > 0 {
			names := append([]string(nil), rule.ResourceNames...)
			sort.Strings(names)
			resourceNames = strings.Join(names, ",")
		}

		for _, verb := range rule.Verbs {
			if verb == "get" || verb == "list" || verb == "watch" {
				continue
			}
			for _, group := range rule.APIGroups {
				groupName := group
				if groupName == "" {
					groupName = "core"
				}
				for _, resource := range rule.Resources {
					tuples = append(tuples,
						scope+"|"+groupName+"|"+resource+"|"+verb+"|"+resourceNames)
				}
			}
			for _, nonResourceURL := range rule.NonResourceURLs {
				tuples = append(tuples,
					scope+"|non-resource|"+nonResourceURL+"|"+verb+"|<not-applicable>")
			}
		}
	}
	sort.Strings(tuples)
	return tuples
}

func allowsResource(rules []rbacv1.PolicyRule, group, resource, verb string) bool {
	for _, rule := range rules {
		if !containsRBAC(rule.APIGroups, group) || !containsRBAC(rule.Verbs, verb) {
			continue
		}
		for _, candidate := range rule.Resources {
			if matchesRBACResource(candidate, resource) {
				return true
			}
		}
	}
	return false
}

func matchesRBACResource(pattern, resource string) bool {
	if pattern == "*" {
		return true
	}
	patternParts, resourceParts := strings.Split(pattern, "/"), strings.Split(resource, "/")
	if len(patternParts) != len(resourceParts) {
		return false
	}
	for i := range patternParts {
		if patternParts[i] != "*" && patternParts[i] != resourceParts[i] {
			return false
		}
	}
	return true
}

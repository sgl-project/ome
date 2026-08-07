package v1beta1

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestLifecycleSpec_JSONKeyIsNestedUnderLifecycle(t *testing.T) {
	// Confirm the OMENative restart policy serializes under
	// `lifecycle.restartPolicy`, not at the top level where it would collide
	// with the inlined corev1.PodSpec.RestartPolicy. encoding/json would
	// silently shadow one of the two on a future flatten/refactor with no
	// error — this is the guard.
	policy := InstanceRestartPolicyRecreateInstance
	spec := EngineSpec{
		ComponentExtensionSpec: ComponentExtensionSpec{
			Lifecycle: &LifecycleSpec{RestartPolicy: &policy},
		},
	}
	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(encoded)
	if !strings.Contains(got, `"lifecycle":{"restartPolicy":"RecreateInstanceOnPodRestart"}`) {
		t.Fatalf("expected nested lifecycle.restartPolicy, got: %s", got)
	}
}

// TestRollingUpdate_MaxUnavailable_AcceptsIntAndPercent exercises the
// IntOrString-typed MaxUnavailable field over the three shapes the
// CRD validator permits: nil (default), integer count, and percent
// string. Mirrors the corresponding MaxSurge case below — both fields
// share the same encoder so the parity check guards against a future
// refactor (e.g. a narrowing back to int32 or a lost omitempty) that
// would drift the CRD-stored shape.
func TestRollingUpdate_MaxUnavailable_AcceptsIntAndPercent(t *testing.T) {
	intVal := intstr.FromInt(2)
	pctVal := intstr.FromString("25%")
	cases := []struct {
		name string
		in   *intstr.IntOrString
		want string // empty when MaxUnavailable is nil (field omitted)
	}{
		{"nil-omitted", nil, ""},
		{"integer-count", &intVal, `"maxUnavailable":2`},
		{"percent-string", &pctVal, `"maxUnavailable":"25%"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ru := RollingUpdate{MaxUnavailable: tc.in}
			b, err := json.Marshal(ru)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got := string(b)
			if tc.want == "" {
				if strings.Contains(got, "maxUnavailable") {
					t.Fatalf("expected maxUnavailable omitted, got %s", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("expected %q in %s", tc.want, got)
			}
			var decoded RollingUpdate
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if diff := cmp.Diff(ru, decoded); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRollingUpdate_MaxSurge_AcceptsIntAndPercent mirrors the
// MaxUnavailable matrix above. The integer and percent forms give
// callers the same flexibility as appsv1.Deployment.Strategy.
// RollingUpdate.MaxSurge.
func TestRollingUpdate_MaxSurge_AcceptsIntAndPercent(t *testing.T) {
	intVal := intstr.FromInt(1)
	pctVal := intstr.FromString("50%")
	cases := []struct {
		name string
		in   *intstr.IntOrString
		want string
	}{
		{"nil-omitted", nil, ""},
		{"integer-count", &intVal, `"maxSurge":1`},
		{"percent-string", &pctVal, `"maxSurge":"50%"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ru := RollingUpdate{MaxSurge: tc.in}
			b, err := json.Marshal(ru)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got := string(b)
			if tc.want == "" {
				if strings.Contains(got, "maxSurge") {
					t.Fatalf("expected maxSurge omitted, got %s", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("expected %q in %s", tc.want, got)
			}
			var decoded RollingUpdate
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if diff := cmp.Diff(ru, decoded); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

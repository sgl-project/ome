package rolloutpolicy

import (
	"errors"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func testCanary() *v1beta1.GroupCanary {
	return &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{
		{Capacity: intstr.FromString("10%"), Traffic: 10},
		{Capacity: intstr.FromString("100%"), Traffic: 100},
	}}
}

func TestPortableDigestStableAndPrefixed(t *testing.T) {
	spec := &v1beta1.RolloutPolicySpec{Canary: testCanary()}
	a, err := PortableDigest(spec)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PortableDigest(spec.DeepCopy())
	if err != nil {
		t.Fatal(err)
	}
	if a != b || !strings.HasPrefix(a, "rp1:") {
		t.Fatalf("digests %q vs %q", a, b)
	}
}

// The migration no-op proof: an inline body identical to a policy body must
// digest equal, so shadowedPolicyRef.wouldPinDigest == observed inline digest
// certifies a zero-behavior-change cutover.
func TestProgressionDigestMatchesPolicyDigest(t *testing.T) {
	inline, err := ProgressionDigest(&v1beta1.RolloutGroup{Canary: testCanary()})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := PortableDigest(&v1beta1.RolloutPolicySpec{Canary: testCanary()})
	if err != nil {
		t.Fatal(err)
	}
	if inline == "" || inline != policy {
		t.Fatalf("inline %q vs policy %q", inline, policy)
	}
}

func TestComposeGroupInlineWins(t *testing.T) {
	g := &v1beta1.RolloutGroup{
		Components: []v1beta1.ComponentType{v1beta1.EngineComponent},
		Canary:     testCanary(),
		PolicyRef:  &v1beta1.RolloutPolicyRef{Name: "p", Progression: v1beta1.RolloutProgressionBlueGreen},
	}
	out, err := ComposeGroup(g, &v1beta1.RolloutPolicySpec{BlueGreen: &v1beta1.GroupBlueGreen{}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Canary == nil || out.BlueGreen != nil || out.PolicyRef != nil {
		t.Fatalf("inline must win and the pin must drop the ref: %+v", out)
	}
}

func TestComposeGroupResolvesRef(t *testing.T) {
	g := &v1beta1.RolloutGroup{
		Components: []v1beta1.ComponentType{v1beta1.EngineComponent},
		PolicyRef:  &v1beta1.RolloutPolicyRef{Name: "p", Progression: v1beta1.RolloutProgressionCanary},
	}
	out, err := ComposeGroup(g, &v1beta1.RolloutPolicySpec{Canary: testCanary()})
	if err != nil {
		t.Fatal(err)
	}
	if out.Canary == nil || len(out.Canary.Steps) != 2 || out.PolicyRef != nil {
		t.Fatalf("composed = %+v", out)
	}
}

func TestComposeGroupMismatchIsSentinel(t *testing.T) {
	g := &v1beta1.RolloutGroup{
		Components: []v1beta1.ComponentType{v1beta1.EngineComponent},
		PolicyRef:  &v1beta1.RolloutPolicyRef{Name: "p", Progression: v1beta1.RolloutProgressionCanary},
	}
	_, err := ComposeGroup(g, &v1beta1.RolloutPolicySpec{BlueGreen: &v1beta1.GroupBlueGreen{}})
	if !errors.Is(err, ErrProgressionMismatch) {
		t.Fatalf("want ErrProgressionMismatch, got %v", err)
	}
}

func TestComposeGroupDefaultsBareGroupToBlueGreen(t *testing.T) {
	g := &v1beta1.RolloutGroup{Components: []v1beta1.ComponentType{v1beta1.EngineComponent}}
	out, err := ComposeGroup(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.BlueGreen == nil {
		t.Fatalf("bare group must pin the explicit blueGreen default: %+v", out)
	}
}

func TestCombinedDigestOrderSensitive(t *testing.T) {
	ab := CombinedDigest([]string{"rp1:a", "rp1:b"})
	ba := CombinedDigest([]string{"rp1:b", "rp1:a"})
	if ab == ba {
		t.Fatal("group order is plan content; the combined digest must reflect it")
	}
	if ab != CombinedDigest([]string{"rp1:a", "rp1:b"}) {
		t.Fatal("combined digest must be deterministic")
	}
}

package revision

import (
	"bytes"
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// TestHashPairingProtocol_EmptyIsByteIdenticalToLegacy proves the
// upgrade-neutrality contract: an empty pairing protocol produces the exact
// hash AND canonical bytes of the pairing-unaware entry points, for both the
// single-pod and leader+worker shapes. If this fails, every protocol-free
// workload re-rolls on operator upgrade.
func TestHashPairingProtocol_EmptyIsByteIdenticalToLegacy(t *testing.T) {
	cases := []struct {
		name   string
		legacy func() (string, []byte, error)
		paired func() (string, []byte, error)
	}{
		{
			name: "single-pod",
			legacy: func() (string, []byte, error) {
				return Hash(stablePodSpec(), stableTemplateMeta(), nil, "uid-a")
			},
			paired: func() (string, []byte, error) {
				return HashWithWorkerTopologyAndPairing(stablePodSpec(), nil, stableTemplateMeta(), "", "", nil, "uid-a")
			},
		},
		{
			name: "leader plus worker with topology",
			legacy: func() (string, []byte, error) {
				return HashWithWorkerAndTopology(stablePodSpec(), stableWorkerSpec(), nil, "topology.kubernetes.io/zone", nil, "uid-a")
			},
			paired: func() (string, []byte, error) {
				return HashWithWorkerTopologyAndPairing(stablePodSpec(), stableWorkerSpec(), nil, "topology.kubernetes.io/zone", "", nil, "uid-a")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lh, lraw, err := tc.legacy()
			if err != nil {
				t.Fatalf("legacy hash: %v", err)
			}
			ph, praw, err := tc.paired()
			if err != nil {
				t.Fatalf("pairing-aware hash: %v", err)
			}
			if lh != ph || !bytes.Equal(lraw, praw) {
				t.Errorf("empty protocol changed the canonical payload:\nlegacy %s %s\npaired %s %s", lh, lraw, ph, praw)
			}
		})
	}
}

// TestHashPairingProtocol_MintsNewRevision proves a protocol changes the hash
// on BOTH shapes — including single-pod, which the topology key deliberately
// does not affect: a PD engine is often single-pod and must still re-roll on
// a protocol change.
func TestHashPairingProtocol_MintsNewRevision(t *testing.T) {
	base, _, err := Hash(stablePodSpec(), nil, nil, "uid-a")
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	single, _, err := HashWithWorkerTopologyAndPairing(stablePodSpec(), nil, nil, "", "nixl-v2", nil, "uid-a")
	if err != nil {
		t.Fatalf("single with protocol: %v", err)
	}
	if single == base {
		t.Errorf("single-pod protocol did not mint a new revision (both %s)", base)
	}
	other, _, err := HashWithWorkerTopologyAndPairing(stablePodSpec(), nil, nil, "", "mooncake-1.4", nil, "uid-a")
	if err != nil {
		t.Fatalf("other protocol: %v", err)
	}
	if other == single {
		t.Errorf("distinct protocols hash identically (%s)", single)
	}
	multiBase, _, err := HashWithWorkerAndTopology(stablePodSpec(), stableWorkerSpec(), nil, "topology.kubernetes.io/zone", nil, "uid-a")
	if err != nil {
		t.Fatalf("multi baseline: %v", err)
	}
	multi, _, err := HashWithWorkerTopologyAndPairing(stablePodSpec(), stableWorkerSpec(), nil, "topology.kubernetes.io/zone", "nixl-v2", nil, "uid-a")
	if err != nil {
		t.Fatalf("multi with protocol: %v", err)
	}
	if multi == multiBase {
		t.Errorf("leader+worker protocol did not mint a new revision (both %s)", multiBase)
	}
}

// TestPairingPayload_GoldenCanonicalShape pins where pairingProtocol sits in
// the canonical bytes so recorded revisions keep matching live payloads.
func TestPairingPayload_GoldenCanonicalShape(t *testing.T) {
	template := basicPodSpecForRevision()
	_, raw, err := HashWithWorkerTopologyAndPairing(template, nil, nil, "", "nixl-v2", nil, "")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	const want = `{"podSpec":{"containers":[{"name":"main","image":"llama:v1","resources":{}}]},"podMeta":null,"pairingProtocol":"nixl-v2"}`
	if string(raw) != want {
		t.Errorf("canonical pairing payload drifted:\n got: %s\nwant: %s", raw, want)
	}
	if p, perr := PayloadFromControllerRevision(&appsv1.ControllerRevision{Data: runtime.RawExtension{Raw: raw}}); perr != nil || p == nil || p.PairingProtocol == nil || *p.PairingProtocol != "nixl-v2" {
		t.Errorf("payload does not round-trip pairingProtocol: %+v err=%v", p, perr)
	}
}

// TestEnsureControllerRevision_StampsPairingAnnotation proves the created CR
// carries the ome.io/pairing-protocol annotation exactly when the hashed
// payload declares a protocol.
func TestEnsureControllerRevision_StampsPairingAnnotation(t *testing.T) {
	isvc := minimalISVC("llama-70b", "prod", 1)
	c := newFakeClientWithApps(t, isvc)
	key := testKey(isvc, workload.ComponentEngine)

	hash, raw, err := HashWithWorkerTopologyAndPairing(basicPodSpecForRevision(), nil, nil, "", "nixl-v2", nil, isvc.UID)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	cr, collision, err := EnsureControllerRevisionFromHash(context.Background(), c, c, isvc, testISVCGVK, key, hash, raw)
	if err != nil || collision {
		t.Fatalf("ensure: err=%v collision=%v", err, collision)
	}
	got := &appsv1.ControllerRevision{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(cr), got); err != nil {
		t.Fatalf("get CR: %v", err)
	}
	if got.Annotations[query.LabelPairingProtocol] != "nixl-v2" {
		t.Errorf("pairing annotation not stamped at create: %+v", got.Annotations)
	}

	// Protocol-free payloads must NOT grow annotations (byte-stable CR shape).
	hash2, raw2, err := Hash(basicPodSpecForRevision(), nil, nil, isvc.UID)
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	cr2, _, err := EnsureControllerRevisionFromHash(context.Background(), c, c, isvc, testISVCGVK, key, hash2, raw2)
	if err != nil {
		t.Fatalf("ensure2: %v", err)
	}
	got2 := &appsv1.ControllerRevision{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(cr2), got2); err != nil {
		t.Fatalf("get CR2: %v", err)
	}
	if _, has := got2.Annotations[query.LabelPairingProtocol]; has {
		t.Errorf("protocol-free CR unexpectedly annotated: %+v", got2.Annotations)
	}
}

// TestPairingProtocolFromRevision covers the read ladder: annotation first,
// payload fallback for annotation-less CRs, "" for pre-pairing and nil CRs.
func TestPairingProtocolFromRevision(t *testing.T) {
	withAnnotation := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{query.LabelPairingProtocol: "nixl-v2"},
		},
	}
	if got := PairingProtocolFromRevision(withAnnotation); got != "nixl-v2" {
		t.Errorf("annotation read: got %q", got)
	}

	_, raw, err := HashWithWorkerTopologyAndPairing(basicPodSpecForRevision(), nil, nil, "", "mooncake-1.4", nil, "")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	payloadOnly := &appsv1.ControllerRevision{Data: runtime.RawExtension{Raw: raw}}
	if got := PairingProtocolFromRevision(payloadOnly); got != "mooncake-1.4" {
		t.Errorf("payload fallback: got %q", got)
	}

	// The annotation wins over the payload when both exist (they only diverge
	// on hand-edited objects; the annotation is the documented fast path).
	both := payloadOnly.DeepCopy()
	both.Annotations = map[string]string{query.LabelPairingProtocol: "nixl-v2"}
	if got := PairingProtocolFromRevision(both); got != "nixl-v2" {
		t.Errorf("annotation precedence: got %q", got)
	}

	_, legacyRaw, err := Hash(basicPodSpecForRevision(), nil, nil, "")
	if err != nil {
		t.Fatalf("legacy hash: %v", err)
	}
	legacy := &appsv1.ControllerRevision{Data: runtime.RawExtension{Raw: legacyRaw}}
	if got := PairingProtocolFromRevision(legacy); got != "" {
		t.Errorf("pre-pairing CR: got %q want empty", got)
	}
	if got := PairingProtocolFromRevision(nil); got != "" {
		t.Errorf("nil CR: got %q want empty", got)
	}
	corrupt := &appsv1.ControllerRevision{Data: runtime.RawExtension{Raw: []byte("{not json")}}
	if got := PairingProtocolFromRevision(corrupt); got != "" {
		t.Errorf("corrupt payload: got %q want empty", got)
	}
}

// TestPairingProtocol_CollisionSaltStillDistinguishes ensures the protocol
// participates in the payload (Data.Raw), not just the hash: two revisions
// differing only by protocol must carry different canonical bytes, or the
// collision detector would treat them as the same revision.
func TestPairingProtocol_CollisionSaltStillDistinguishes(t *testing.T) {
	_, rawA, err := HashWithWorkerTopologyAndPairing(basicPodSpecForRevision(), nil, nil, "", "a", ptr.To(int32(0)), "")
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	_, rawB, err := HashWithWorkerTopologyAndPairing(basicPodSpecForRevision(), nil, nil, "", "b", ptr.To(int32(0)), "")
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if bytes.Equal(rawA, rawB) {
		t.Error("payloads with distinct protocols are byte-identical; collision detection cannot separate them")
	}
}

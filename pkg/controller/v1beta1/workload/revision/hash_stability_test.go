package revision

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// stablePodSpec is a fixed leader/single-pod template exercising the
// field classes that feed the canonical payload (image, env, ports,
// resources). Any edit to this fixture invalidates the golden values
// below by design.
func stablePodSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "ome-container",
			Image: "registry.example.com/serving:v1",
			Env: []corev1.EnvVar{
				{Name: "MODEL_PATH", Value: "/models/base"},
			},
			Ports: []corev1.ContainerPort{
				{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("2"),
					"memory": resource.MustParse("4Gi"),
				},
			},
		}},
	}
}

// stableWorkerSpec is a fixed worker template distinct from the leader.
func stableWorkerSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  "worker",
			Image: "registry.example.com/serving:v1",
		}},
	}
}

// stableTemplateMeta is a fixed user-intent label/annotation set.
func stableTemplateMeta() *metav1.ObjectMeta {
	return &metav1.ObjectMeta{
		Labels:      map[string]string{"app.kubernetes.io/name": "engine"},
		Annotations: map[string]string{"example.com/tier": "gold"},
	}
}

// TestRevisionHash_GoldenStability pins the exact hash emitted for
// fixed payloads. The hash IS the ControllerRevision name suffix and
// the pod revision-hash label: if the canonical payload shape, the
// JSON serialization, the FNV framing, or the collision/owner-UID
// folding ever changes, every existing workload computes a new
// revision name on the next reconcile and rolls — a silent
// fleet-wide restart on operator upgrade. A failure here means the
// change is NOT backward-compatible and needs an explicit migration
// story, not a golden-value update.
func TestRevisionHash_GoldenStability(t *testing.T) {
	cases := []struct {
		name string
		hash func() (string, []byte, error)
		want string
	}{
		{
			name: "single-pod no meta",
			hash: func() (string, []byte, error) {
				return Hash(stablePodSpec(), nil, nil, "")
			},
			want: "c6697510",
		},
		{
			name: "single-pod with template meta",
			hash: func() (string, []byte, error) {
				return Hash(stablePodSpec(), stableTemplateMeta(), nil, "")
			},
			want: "82d844dd",
		},
		{
			name: "leader plus worker",
			hash: func() (string, []byte, error) {
				return HashWithWorker(stablePodSpec(), stableWorkerSpec(), nil, nil, "")
			},
			want: "d7a8a72f",
		},
		{
			name: "leader plus worker with topology key",
			hash: func() (string, []byte, error) {
				return HashWithWorkerAndTopology(stablePodSpec(), stableWorkerSpec(), nil, "topology.kubernetes.io/zone", nil, "")
			},
			want: "a1c83d4d",
		},
		{
			name: "collision count folded",
			hash: func() (string, []byte, error) {
				return Hash(stablePodSpec(), nil, ptr.To(int32(1)), "")
			},
			want: "8df83ecd",
		},
		{
			name: "owner UID folded",
			hash: func() (string, []byte, error) {
				return Hash(stablePodSpec(), nil, nil, "11111111-2222-3333-4444-555555555555")
			},
			want: "10b5fd04",
		},
	}
	seen := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := tc.hash()
			if err != nil {
				t.Fatalf("hash: %v", err)
			}
			if got != tc.want {
				t.Errorf("hash changed for a fixed payload: got %q want %q — this rolls every live workload on upgrade", got, tc.want)
			}
			if prev, dup := seen[got]; dup {
				t.Errorf("hash %q collides across distinct payloads (%q and %q)", got, prev, tc.name)
			}
			seen[got] = tc.name
		})
	}
}

// TestRevisionPayload_GoldenCanonicalShape pins the exact canonical
// bytes stored in ControllerRevision.Data.Raw for the minimal
// single-pod payload: podSpec first, podMeta explicitly null (never
// omitted), and no workerPodSpec/topologyKey keys when unset. The
// recorded bytes are compared against live payloads to decide whether
// a revision matches, so any drift in this shape breaks matching for
// every previously-recorded revision.
func TestRevisionPayload_GoldenCanonicalShape(t *testing.T) {
	template := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "ome-container", Image: "registry.example.com/serving:v1"}},
	}
	_, raw, err := Hash(template, nil, nil, "")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	const want = `{"podSpec":{"containers":[{"name":"ome-container","image":"registry.example.com/serving:v1","resources":{}}]},"podMeta":null}`
	if string(raw) != want {
		t.Errorf("canonical payload shape drifted:\n got: %s\nwant: %s", raw, want)
	}
}

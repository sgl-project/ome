package pvc

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	"sigs.k8s.io/ome/pkg/utils/storage"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func TestMetadataJobName_SameModelDifferentURI_PrefixPlusDistinctHash(t *testing.T) {
	tests := []struct {
		name        string
		modelName   string
		uri         string
		wantPrefix  string
		wantSuffLen int
	}{
		{
			name:        "short model name",
			modelName:   "llama-7b",
			uri:         "pvc://my-pvc/models/llama",
			wantPrefix:  "llama-7b-metadata-",
			wantSuffLen: 8,
		},
		{
			name:        "different uri yields different name",
			modelName:   "llama-7b",
			uri:         "pvc://my-pvc/models/other",
			wantPrefix:  "llama-7b-metadata-",
			wantSuffLen: 8,
		},
	}

	seen := map[string]string{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := metadataJobName(tc.modelName, tc.uri)
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("name %q missing prefix %q", got, tc.wantPrefix)
			}
			if len(got)-len(tc.wantPrefix) != tc.wantSuffLen {
				t.Fatalf("hash suffix length: got %d, want %d", len(got)-len(tc.wantPrefix), tc.wantSuffLen)
			}
			if len(got) > 63 {
				t.Fatalf("name %q exceeds 63 chars", got)
			}
			if prev, ok := seen[got]; ok {
				t.Fatalf("name collision %q for inputs %q and %q", got, prev, tc.uri)
			}
			seen[got] = tc.uri
		})
	}
}

func TestMetadataJobName_TooLongModelTrimmed(t *testing.T) {
	long := strings.Repeat("a", 80)
	got := metadataJobName(long, "pvc://my-pvc/p")
	if len(got) > 63 {
		t.Fatalf("name %q exceeds 63 chars", got)
	}
}

func TestMetadataJobName_EmptyAfterTrimGuard(t *testing.T) {
	// Pathological input: a model name long enough that the trim leaves
	// nothing must NOT yield a name starting with '-'. Use an extreme
	// length to force the trimmed-to-empty branch even with the 8-char hash.
	got := metadataJobName(strings.Repeat("a", 200), "pvc://my-pvc/p")
	if got == "" || got[0] == '-' {
		t.Fatalf("name %q must start with an alphanumeric (DNS-1123)", got)
	}
	if len(got) > 63 {
		t.Fatalf("name %q exceeds 63 chars", got)
	}
}

func TestBuildMetadataJob_BaseModel(t *testing.T) {
	scheme := fakeclient.Scheme(t)
	bm := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-7b", Namespace: "models", UID: "uid-1"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://my-pvc/models/llama"),
		}},
	}
	components := &storage.PVCStorageComponents{PVCName: "my-pvc", SubPath: "models/llama"}
	cfg := MetadataJobConfig{
		Image:          "ome-agent:latest",
		ServiceAccount: "ome-model-metadata",
		CPURequest:     "100m",
		MemoryRequest:  "128Mi",
	}

	job, err := buildMetadataJob(bm, false, components, "models", cfg, scheme)
	if err != nil {
		t.Fatalf("buildMetadataJob: %v", err)
	}

	pod := job.Spec.Template.Spec
	if len(job.OwnerReferences) != 1 || len(pod.Volumes) != 1 || len(pod.Containers) != 1 {
		t.Fatalf("job has %d owner refs, %d volumes and %d containers, want 1 of each",
			len(job.OwnerReferences), len(pod.Volumes), len(pod.Containers))
	}
	claim := pod.Volumes[0].PersistentVolumeClaim
	container := pod.Containers[0]
	if claim == nil || len(container.VolumeMounts) != 1 {
		t.Fatalf("volume source = %+v with %d mounts, want one PVC volume with one mount",
			pod.Volumes[0].VolumeSource, len(container.VolumeMounts))
	}
	mount := container.VolumeMounts[0]
	podNamespace := func() string {
		for _, env := range container.Env {
			if env.Name == "POD_NAMESPACE" {
				return env.Value
			}
		}
		return ""
	}

	for _, check := range []struct {
		field string
		got   any
		want  any
	}{
		{"job namespace", job.Namespace, "models"},
		{"scope label", job.Labels[constants.PVCMetadataScopeLabel], metadataJobScopeNamespaced},
		{"model-name label", job.Labels[constants.PVCMetadataModelNameLabel], "llama-7b"},
		{"owner kind", job.OwnerReferences[0].Kind, "BaseModel"},
		{"backoff limit", *job.Spec.BackoffLimit, int32(2)},
		{"ttl", *job.Spec.TTLSecondsAfterFinished, int32(3600)},
		{"restart policy", pod.RestartPolicy, corev1.RestartPolicyNever},
		{"service account", pod.ServiceAccountName, cfg.ServiceAccount},
		{"claim name", claim.ClaimName, "my-pvc"},
		{"claim read-only", claim.ReadOnly, true},
		{"image", container.Image, cfg.Image},
		{"mount path", mount.MountPath, metadataJobModelMountPath},
		{"sub path", mount.SubPath, "models/llama"},
		{"mount read-only", mount.ReadOnly, true},
		{"cpu request", container.Resources.Requests.Cpu().String(), "100m"},
		{"memory request", container.Resources.Requests.Memory().String(), "128Mi"},
		// POD_NAMESPACE carries the controller's own namespace into the Job pod,
		// so the agent writes its status ConfigMap where the controller reads it.
		{"POD_NAMESPACE env", podNamespace(), constants.OMENamespace},
	} {
		if check.got != check.want {
			t.Errorf("%s = %#v, want %#v", check.field, check.got, check.want)
		}
	}

	args := strings.Join(container.Args, " ")
	for _, want := range []string{
		"model-metadata",
		// The agent refuses to start without a config file path.
		"--config " + metadataJobAgentConfigPath,
		"--model-path " + metadataJobModelMountPath,
		"--basemodel-name llama-7b",
		"--basemodel-namespace models",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
	if strings.Contains(args, "--cluster-scoped") {
		t.Errorf("namespaced BaseModel must not pass --cluster-scoped")
	}
}

func TestBuildMetadataJob_ClusterBaseModel(t *testing.T) {
	scheme := fakeclient.Scheme(t)
	cbm := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-llama", UID: "uid-2"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://shared:my-pvc/models/llama"),
		}},
	}
	components := &storage.PVCStorageComponents{Namespace: "shared", PVCName: "my-pvc", SubPath: "models/llama"}

	job, err := buildMetadataJob(cbm, true, components, "shared", MetadataJobConfig{
		Image:          "ome-agent:latest",
		ServiceAccount: "ome-model-metadata",
	}, scheme)
	if err != nil {
		t.Fatalf("buildMetadataJob: %v", err)
	}

	if job.Namespace != "shared" {
		t.Errorf("job namespace = %q, want shared", job.Namespace)
	}
	if got := job.Labels[constants.PVCMetadataScopeLabel]; got != metadataJobScopeCluster {
		t.Errorf("scope label = %q, want %q", got, metadataJobScopeCluster)
	}

	if got := len(job.OwnerReferences); got != 1 {
		t.Fatalf("expected 1 owner ref, got %d", got)
	}
	or := job.OwnerReferences[0]
	if or.Kind != "ClusterBaseModel" {
		t.Errorf("owner kind = %q, want ClusterBaseModel", or.Kind)
	}

	args := strings.Join(job.Spec.Template.Spec.Containers[0].Args, " ")
	if !strings.Contains(args, "--cluster-scoped") {
		t.Errorf("cluster-scoped BaseModel must pass --cluster-scoped, got %q", args)
	}
	if strings.Contains(args, "--basemodel-namespace") {
		t.Errorf("cluster-scoped BaseModel must not pass --basemodel-namespace, got %q", args)
	}
}

func TestBuildMetadataJob_SchedulingHints_PassedThrough(t *testing.T) {
	scheme := fakeclient.Scheme(t)
	bm := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-7b", Namespace: "models"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://my-pvc/models/llama"),
		}},
	}
	cfg := MetadataJobConfig{
		Image:             "ome-agent:test",
		ServiceAccount:    "ome-model-metadata",
		NodeSelector:      map[string]string{"disktype": "ssd"},
		PriorityClassName: "system-cluster-critical",
		Tolerations: []corev1.Toleration{{
			Key:      "nvidia.com/gpu",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}},
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "topology.kubernetes.io/zone",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"us-east-1a"},
						}},
					}},
				},
			},
		},
	}

	job, err := buildMetadataJob(bm, false, &storage.PVCStorageComponents{PVCName: "my-pvc", SubPath: "models/llama"}, "models", cfg, scheme)
	if err != nil {
		t.Fatalf("buildMetadataJob: %v", err)
	}
	pod := job.Spec.Template.Spec
	if pod.NodeSelector["disktype"] != "ssd" {
		t.Errorf("NodeSelector not passed through: %v", pod.NodeSelector)
	}
	if pod.PriorityClassName != "system-cluster-critical" {
		t.Errorf("PriorityClassName = %q, want system-cluster-critical", pod.PriorityClassName)
	}
	if len(pod.Tolerations) != 1 || pod.Tolerations[0].Key != "nvidia.com/gpu" {
		t.Errorf("Tolerations not passed through: %+v", pod.Tolerations)
	}
	if pod.Affinity == nil || pod.Affinity.NodeAffinity == nil {
		t.Errorf("Affinity not passed through: %+v", pod.Affinity)
	}
}

// TestOmeAgentConfig_RoundTrip_ChartJSONIntoPodSpec asserts that the
// JSON shape the Helm chart's `omeAgent: |-` block emits survives the
// full chain: ConfigMap data → controllerconfig.OmeAgentConfig (json
// Unmarshal) → omeAgentConfigToMetadataJobConfig → buildMetadataJob →
// pod spec. If a contributor renames a JSON tag on OmeAgentConfig, drops
// a field from the converter, or removes a key from the chart template,
// the assertions below fire — without this test those mismatches would
// silently render the corresponding override unusable.
func TestOmeAgentConfig_RoundTrip_ChartJSONIntoPodSpec(t *testing.T) {
	scheme := fakeclient.Scheme(t)

	// Mirror the keys+shapes the Helm template renders. Keep this in
	// sync with charts/ome-resources/templates/ome-controller/configmap.yaml
	// (the omeAgent block) and config/configmap/inferenceservice.yaml.
	chartJSON := `{
        "image": "ome-agent:from-chart",
        "serviceAccount": "ome-model-metadata",
        "memoryRequest": "256Mi",
        "memoryLimit": "512Mi",
        "cpuRequest": "100m",
        "cpuLimit": "500m",
        "backoffLimit": 7,
        "ttlSecondsAfterFinished": 1234,
        "nodeSelector": {"disktype": "ssd"},
        "tolerations": [{"key": "models-only", "operator": "Exists", "effect": "NoSchedule"}],
        "affinity": {
          "nodeAffinity": {
            "requiredDuringSchedulingIgnoredDuringExecution": {
              "nodeSelectorTerms": [{
                "matchExpressions": [{
                  "key": "topology.kubernetes.io/zone",
                  "operator": "In",
                  "values": ["us-east-1a"]
                }]
              }]
            }
          }
        },
        "priorityClassName": "system-cluster-critical"
    }`

	cfg := &controllerconfig.OmeAgentConfig{}
	if err := json.Unmarshal([]byte(chartJSON), cfg); err != nil {
		t.Fatalf("unmarshal chart-shaped JSON: %v", err)
	}

	jobCfg := omeAgentConfigToMetadataJobConfig(cfg)
	if jobCfg.Image != "ome-agent:from-chart" {
		t.Errorf("Image lost in conversion: %q", jobCfg.Image)
	}
	if jobCfg.BackoffLimit != 7 {
		t.Errorf("BackoffLimit lost in conversion: %d", jobCfg.BackoffLimit)
	}
	if jobCfg.TTLSecondsAfterFinished != 1234 {
		t.Errorf("TTLSecondsAfterFinished lost in conversion: %d", jobCfg.TTLSecondsAfterFinished)
	}

	bm := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-7b", Namespace: "models"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://my-pvc/models/llama"),
		}},
	}
	job, err := buildMetadataJob(bm, false, &storage.PVCStorageComponents{PVCName: "my-pvc", SubPath: "models/llama"}, "models", jobCfg, scheme)
	if err != nil {
		t.Fatalf("buildMetadataJob: %v", err)
	}

	pod := job.Spec.Template.Spec
	if pod.NodeSelector["disktype"] != "ssd" {
		t.Errorf("nodeSelector did not round-trip from chart JSON to pod spec: %+v", pod.NodeSelector)
	}
	if pod.PriorityClassName != "system-cluster-critical" {
		t.Errorf("priorityClassName did not round-trip: %q", pod.PriorityClassName)
	}
	if len(pod.Tolerations) != 1 || pod.Tolerations[0].Key != "models-only" {
		t.Errorf("tolerations did not round-trip: %+v", pod.Tolerations)
	}
	if pod.Affinity == nil || pod.Affinity.NodeAffinity == nil {
		t.Fatalf("affinity did not round-trip: %+v", pod.Affinity)
	}
	terms := pod.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if terms == nil || len(terms.NodeSelectorTerms) != 1 {
		t.Fatalf("affinity NodeSelectorTerms missing: %+v", terms)
	}
	exprs := terms.NodeSelectorTerms[0].MatchExpressions
	if len(exprs) != 1 || exprs[0].Key != "topology.kubernetes.io/zone" {
		t.Errorf("affinity MatchExpressions lost: %+v", exprs)
	}

	if *job.Spec.BackoffLimit != 7 {
		t.Errorf("BackoffLimit not propagated to Job: %d", *job.Spec.BackoffLimit)
	}
	if *job.Spec.TTLSecondsAfterFinished != 1234 {
		t.Errorf("TTLSecondsAfterFinished not propagated to Job: %d", *job.Spec.TTLSecondsAfterFinished)
	}
}

func TestBuildMetadataJob_PodSecurityContext_Restricted(t *testing.T) {
	scheme := fakeclient.Scheme(t)
	bm := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-7b", Namespace: "models"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://my-pvc/models/llama"),
		}},
	}
	components := &storage.PVCStorageComponents{PVCName: "my-pvc", SubPath: "models/llama"}

	job, err := buildMetadataJob(bm, false, components, "models",
		MetadataJobConfig{Image: "ome-agent:test"}, scheme)
	if err != nil {
		t.Fatalf("buildMetadataJob: %v", err)
	}

	pod := job.Spec.Template.Spec
	psc := pod.SecurityContext
	if psc == nil {
		t.Fatalf("expected pod SecurityContext to be set")
	}
	if psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Errorf("RunAsNonRoot must be true")
	}
	if psc.RunAsUser == nil || *psc.RunAsUser != metadataJobNonRootUID {
		t.Errorf("RunAsUser = %v, want %d", psc.RunAsUser, metadataJobNonRootUID)
	}
	if psc.SeccompProfile == nil || psc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("seccompProfile must be RuntimeDefault, got %+v", psc.SeccompProfile)
	}

	c := pod.Containers[0]
	csc := c.SecurityContext
	if csc == nil {
		t.Fatalf("expected container SecurityContext to be set")
	}
	if csc.AllowPrivilegeEscalation == nil || *csc.AllowPrivilegeEscalation {
		t.Errorf("AllowPrivilegeEscalation must be false")
	}
	if csc.ReadOnlyRootFilesystem == nil || !*csc.ReadOnlyRootFilesystem {
		t.Errorf("ReadOnlyRootFilesystem must be true")
	}
	if csc.Capabilities == nil || len(csc.Capabilities.Drop) == 0 || csc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("Capabilities.Drop must be [ALL], got %+v", csc.Capabilities)
	}
}

func TestBuildMetadataJob_RejectsMissingImage(t *testing.T) {
	scheme := fakeclient.Scheme(t)
	bm := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "m", Namespace: "models"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://my-pvc/p"),
		}},
	}
	if _, err := buildMetadataJob(bm, false, &storage.PVCStorageComponents{PVCName: "my-pvc", SubPath: "p"}, "models", MetadataJobConfig{}, scheme); err == nil {
		t.Fatalf("expected error when image is empty")
	}
}

package utils

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/sgl-project/ome/pkg/constants"
)

func TestFilterUtil(t *testing.T) {
	scenarios := map[string]struct {
		input     map[string]string
		predicate func(string) bool
		expected  map[string]string
	}{
		"TruthyFilter": {
			input:     map[string]string{"key1": "val1", "key2": "val2"},
			predicate: func(key string) bool { return true },
			expected:  map[string]string{"key1": "val1", "key2": "val2"},
		},
		"FalsyFilter": {
			input:     map[string]string{"key1": "val1", "key2": "val2"},
			predicate: func(key string) bool { return false },
			expected:  map[string]string{},
		},
	}
	for name, scenario := range scenarios {
		result := Filter(scenario.input, scenario.predicate)

		if diff := cmp.Diff(scenario.expected, result); diff != "" {
			t.Errorf("Test %q unexpected result (-want +got): %v", name, diff)
		}
	}
}

func TestUnionUtil(t *testing.T) {
	scenarios := map[string]struct {
		input1   map[string]string
		input2   map[string]string
		expected map[string]string
	}{
		"UnionTwoMaps": {
			input1: map[string]string{"ome.io/service": "mnist",
				"label1": "value1"},
			input2: map[string]string{"service.knative.dev/service": "mnist",
				"label2": "value2"},
			expected: map[string]string{"ome.io/service": "mnist",
				"label1": "value1", "service.knative.dev/service": "mnist", "label2": "value2"},
		},
		"UnionTwoMapsOverwritten": {
			input1: map[string]string{"ome.io/service": "mnist",
				"label1": "value1", "label3": "value1"},
			input2: map[string]string{"service.knative.dev/service": "mnist",
				"label2": "value2", "label3": "value3"},
			expected: map[string]string{"ome.io/service": "mnist",
				"label1": "value1", "service.knative.dev/service": "mnist", "label2": "value2", "label3": "value3"},
		},
		"UnionWithEmptyMap": {
			input1: map[string]string{},
			input2: map[string]string{"service.knative.dev/service": "mnist",
				"label2": "value2"},
			expected: map[string]string{"service.knative.dev/service": "mnist", "label2": "value2"},
		},
		"UnionWithNilMap": {
			input1: nil,
			input2: map[string]string{"service.knative.dev/service": "mnist",
				"label2": "value2"},
			expected: map[string]string{"service.knative.dev/service": "mnist", "label2": "value2"},
		},
		"UnionNilMaps": {
			input1:   nil,
			input2:   nil,
			expected: map[string]string{},
		},
	}
	for name, scenario := range scenarios {
		result := Union(scenario.input1, scenario.input2)

		if diff := cmp.Diff(scenario.expected, result); diff != "" {
			t.Errorf("Test %q unexpected result (-want +got): %v", name, diff)
		}
	}
}

func TestContainsUtil(t *testing.T) {
	scenarios := map[string]struct {
		input1   []string
		input2   string
		expected bool
	}{
		"SliceContainsString": {
			input1:   []string{"hey", "hello"},
			input2:   "hey",
			expected: true,
		},
		"SliceDoesNotContainString": {
			input1:   []string{"hey", "hello"},
			input2:   "he",
			expected: false,
		},
		"SliceIsEmpty": {
			input1:   []string{},
			input2:   "hey",
			expected: false,
		},
	}
	for name, scenario := range scenarios {
		result := Includes(scenario.input1, scenario.input2)
		if diff := cmp.Diff(scenario.expected, result); diff != "" {
			t.Errorf("Test %q unexpected result (-want +got): %v", name, diff)
		}
	}
}

func TestAppendVolumeIfNotExists(t *testing.T) {

	scenarios := map[string]struct {
		volumes         []v1.Volume
		volume          v1.Volume
		expectedVolumes []v1.Volume
	}{
		"DuplicateVolume": {
			volumes: []v1.Volume{
				{
					Name: "oci",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-oci-sa",
						},
					},
				},
				{
					Name: "blue",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-gcp-sa",
						},
					},
				},
			},
			volume: v1.Volume{
				Name: "oci",
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: "user-oci-sa",
					},
				},
			},
			expectedVolumes: []v1.Volume{
				{
					Name: "oci",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-oci-sa",
						},
					},
				},
				{
					Name: "blue",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-gcp-sa",
						},
					},
				},
			},
		},
		"NotDuplicateVolume": {
			volumes: []v1.Volume{
				{
					Name: "azure",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-azure-sa",
						},
					},
				},
				{
					Name: "blue",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-gcp-sa",
						},
					},
				},
			},
			volume: v1.Volume{
				Name: "green",
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: "user-gcp-sa",
					},
				},
			},
			expectedVolumes: []v1.Volume{
				{
					Name: "azure",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-azure-sa",
						},
					},
				},
				{
					Name: "blue",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-gcp-sa",
						},
					},
				},
				{
					Name: "green",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "user-gcp-sa",
						},
					},
				},
			},
		},
	}

	for name, scenario := range scenarios {
		volumes := AppendVolumeIfNotExists(scenario.volumes, scenario.volume)

		if diff := cmp.Diff(scenario.expectedVolumes, volumes); diff != "" {
			t.Errorf("Test %q unexpected volume (-want +got): %v", name, diff)
		}
	}
}

func TestMergeEnvs(t *testing.T) {

	scenarios := map[string]struct {
		baseEnvs     []v1.EnvVar
		overrideEnvs []v1.EnvVar
		expectedEnvs []v1.EnvVar
	}{
		"EmptyOverrides": {
			baseEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
			},
			overrideEnvs: []v1.EnvVar{},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
			},
		},
		"EmptyBase": {
			baseEnvs: []v1.EnvVar{},
			overrideEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
			},
		},
		"NoOverlap": {
			baseEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
			},
			overrideEnvs: []v1.EnvVar{
				{
					Name:  "name2",
					Value: "value2",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
				{
					Name:  "name2",
					Value: "value2",
				},
			},
		},
		"SingleOverlap": {
			baseEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
			},
			overrideEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value2",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value2",
				},
			},
		},
		"MultiOverlap": {
			baseEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value1",
				},
				{
					Name:  "name2",
					Value: "value2",
				},
				{
					Name:  "name3",
					Value: "value3",
				},
			},
			overrideEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value3",
				},
				{
					Name:  "name3",
					Value: "value1",
				},
				{
					Name:  "name4",
					Value: "value4",
				},
			},
			expectedEnvs: []v1.EnvVar{
				{
					Name:  "name1",
					Value: "value3",
				},
				{
					Name:  "name2",
					Value: "value2",
				},
				{
					Name:  "name3",
					Value: "value1",
				},
				{
					Name:  "name4",
					Value: "value4",
				},
			},
		},
	}

	for name, scenario := range scenarios {
		envs := MergeEnvs(scenario.baseEnvs, scenario.overrideEnvs)

		if diff := cmp.Diff(scenario.expectedEnvs, envs); diff != "" {
			t.Errorf("Test %q unexpected envs (-want +got): %v", name, diff)
		}
	}
}

func TestIsGpuEnabled(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	scenarios := map[string]struct {
		resource v1.ResourceRequirements
		expected bool
	}{
		"GpuEnabled": {
			resource: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					"cpu": resource.Quantity{
						Format: "100",
					},
					constants.NvidiaGPUResourceType: resource.MustParse("1"),
				},
				Requests: v1.ResourceList{
					"cpu": resource.Quantity{
						Format: "90",
					},
					constants.NvidiaGPUResourceType: resource.MustParse("1"),
				},
			},
			expected: true,
		},
		"GPUDisabled": {
			resource: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					"cpu": resource.Quantity{
						Format: "100",
					},
				},
				Requests: v1.ResourceList{
					"cpu": resource.Quantity{
						Format: "90",
					},
				},
			},
			expected: false,
		},
	}
	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			res := IsGPUEnabled(scenario.resource)
			g.Expect(res).To(gomega.Equal(scenario.expected))
		})
	}
}

func TestFirstNonNilError(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	scenarios := map[string]struct {
		errors  []error
		matcher types.GomegaMatcher
	}{
		"NoNonNilError": {
			errors: []error{
				nil,
				nil,
			},
			matcher: gomega.BeNil(),
		},
		"ContainsError": {
			errors: []error{
				nil,
				errors.New("First non nil error"),
				errors.New("Second non nil error"),
			},
			matcher: gomega.Equal(errors.New("First non nil error")),
		},
	}
	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			err := FirstNonNilError(scenario.errors)
			g.Expect(err).Should(scenario.matcher)
		})
	}
}

func TestRemoveString(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	testStrings := []string{
		"Model Tensorflow",
		"SKLearn Model",
		"Model",
		"ModelPytorch",
	}
	expected := []string{
		"Model Tensorflow",
		"SKLearn Model",
		"ModelPytorch",
	}
	res := RemoveString(testStrings, "Model")
	g.Expect(res).Should(gomega.Equal(expected))
}

func TestIsPrefixSupported(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	prefixes := []string{
		"S3://",
		"GCS://",
		"HTTP://",
		"HTTPS://",
	}
	scenarios := map[string]struct {
		input    string
		expected bool
	}{
		"SupportedPrefix": {
			input:    "GCS://test/model",
			expected: true,
		},
		"UnSupportedPreifx": {
			input:    "PVC://test/model",
			expected: false,
		},
	}
	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			res := IsPrefixSupported(scenario.input, prefixes)
			g.Expect(res).Should(gomega.Equal(scenario.expected))
		})
	}
}

func TestFilterPodOnlyAnnotations(t *testing.T) {
	scenarios := map[string]struct {
		annotations map[string]string
		expected    map[string]string
	}{
		"NilAnnotations": {
			annotations: nil,
			expected:    nil,
		},
		"EmptyAnnotations": {
			annotations: map[string]string{},
			expected:    map[string]string{},
		},
		"FilterGrafanaAnnotations": {
			annotations: map[string]string{
				"k8s.grafana.com/scrape": "true",
				"k8s.grafana.com/port":   "8080",
				"ome.io/base-model-name": "test-model",
			},
			expected: map[string]string{
				"ome.io/base-model-name": "test-model",
			},
		},
		"FilterLokiAnnotations": {
			annotations: map[string]string{
				"loki.grafana.com/scrape":     "true",
				"loki.grafana.com/log-format": "json",
				"meta.helm.sh/release-name":   "test",
			},
			expected: map[string]string{
				"meta.helm.sh/release-name": "test",
			},
		},
		"FilterPrometheusAnnotations": {
			annotations: map[string]string{
				"prometheus.io/scrape":       "true",
				"prometheus.io/port":         "8080",
				"prometheus.io/path":         "/metrics",
				"meta.helm.sh/release-name":  "test",
			},
			expected: map[string]string{
				"meta.helm.sh/release-name": "test",
			},
		},
		"FilterNetworkingGKEAnnotations": {
			annotations: map[string]string{
				"networking.gke.io/default-interface": "eth0",
				"networking.gke.io/interfaces":        "[{\"interfaceName\":\"eth0\"}]",
				"meta.helm.sh/release-name":           "test",
			},
			expected: map[string]string{
				"meta.helm.sh/release-name": "test",
			},
		},
		"FilterRDMAAnnotations": {
			annotations: map[string]string{
				"rdma.ome.io/auto-inject":    "true",
				"rdma.ome.io/profile":        "default",
				"ome.io/base-model-name":     "test-model",
			},
			expected: map[string]string{
				"ome.io/base-model-name": "test-model",
			},
		},
		"FilterInjectionAnnotations": {
			annotations: map[string]string{
				constants.ModelInitInjectionKey:        "true",
				constants.FineTunedAdapterInjectionKey: "weight-name",
				constants.ServingSidecarInjectionKey:   "true",
				"ome.io/base-model-name":               "test-model",
				"ome.io/serving-runtime":               "test-runtime",
			},
			expected: map[string]string{
				"ome.io/base-model-name":  "test-model",
				"ome.io/serving-runtime":  "test-runtime",
			},
		},
		"PreserveNonPodOnlyAnnotations": {
			annotations: map[string]string{
				"ome.io/deploymentMode":      "RawDeployment",
				"ome.io/service-type":        "ClusterIP",
				"custom.annotation/key":      "value",
				"meta.helm.sh/release-name":  "test",
			},
			expected: map[string]string{
				"ome.io/deploymentMode":      "RawDeployment",
				"ome.io/service-type":        "ClusterIP",
				"custom.annotation/key":      "value",
				"meta.helm.sh/release-name":  "test",
			},
		},
		"MixedAnnotations": {
			annotations: map[string]string{
				"k8s.grafana.com/scrape":               "true",
				"networking.gke.io/interfaces":         "[...]",
				constants.ModelInitInjectionKey:        "true",
				"ome.io/base-model-name":               "test-model",
				"ome.io/service-type":                  "ClusterIP",
				"meta.helm.sh/release-name":            "test",
			},
			expected: map[string]string{
				"ome.io/base-model-name":    "test-model",
				"ome.io/service-type":       "ClusterIP",
				"meta.helm.sh/release-name": "test",
			},
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			result := FilterPodOnlyAnnotations(scenario.annotations)
			if diff := cmp.Diff(scenario.expected, result); diff != "" {
				t.Errorf("Test %q unexpected result (-want +got): %v", name, diff)
			}
		})
	}
}

// Helper to assert a path is a symlink with expected relative target and resolves to the absolute target.
func assertSymlink(t *testing.T, linkPath, expectedRelTarget, absoluteTarget string) {
	t.Helper()

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat on %s failed: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to be a symlink", linkPath)
	}

	gotTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}
	if gotTarget != expectedRelTarget {
		t.Fatalf("unexpected symlink target: got %q want %q", gotTarget, expectedRelTarget)
	}

	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}
	expectedResolved, err := filepath.EvalSymlinks(absoluteTarget)
	if err != nil {
		t.Fatalf("EvalSymlinks of expected target failed: %v", err)
	}
	if filepath.Clean(resolved) != filepath.Clean(expectedResolved) {
		t.Fatalf("resolved path mismatch: got %q want %q", resolved, expectedResolved)
	}
}

func TestCreateSymbolicLink_HappyPath(t *testing.T) {
	tmp := t.TempDir()

	parentPath := filepath.Join(tmp, "parentDir")
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}

	childPath := filepath.Join(tmp, "child", "link")

	if err := CreateSymbolicLink(childPath, parentPath); err != nil {
		t.Fatalf("CreateSymbolicLink returned error: %v", err)
	}

	expectedRel, err := filepath.Rel(filepath.Dir(childPath), parentPath)
	if err != nil {
		t.Fatalf("failed to compute expected relative path: %v", err)
	}

	assertSymlink(t, childPath, expectedRel, parentPath)
}

func TestCreateSymbolicLink_IdempotentNoOp(t *testing.T) {
	tmp := t.TempDir()

	parentPath := filepath.Join(tmp, "parent")
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}

	childPath := filepath.Join(tmp, "child", "link")

	// First creation
	if err := CreateSymbolicLink(childPath, parentPath); err != nil {
		t.Fatalf("CreateSymbolicLink returned error: %v", err)
	}

	target1, err := os.Readlink(childPath)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}

	// Second creation should be a no-op
	if err := CreateSymbolicLink(childPath, parentPath); err != nil {
		t.Fatalf("CreateSymbolicLink returned error on idempotent call: %v", err)
	}

	target2, err := os.Readlink(childPath)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}

	if target1 != target2 {
		t.Fatalf("idempotent call should keep same symlink target, got %q want %q", target2, target1)
	}

	resolved, err := filepath.EvalSymlinks(childPath)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}
	expectedResolved, err := filepath.EvalSymlinks(parentPath)
	if err != nil {
		t.Fatalf("EvalSymlinks of expected target failed: %v", err)
	}
	if filepath.Clean(resolved) != filepath.Clean(expectedResolved) {
		t.Fatalf("resolved mismatch: got %q want %q", resolved, expectedResolved)
	}
}

func TestCreateSymbolicLink_ReplacesExistingSymlinkWithDifferentTarget(t *testing.T) {
	tmp := t.TempDir()

	parentPath1 := filepath.Join(tmp, "parent1")
	parentPath2 := filepath.Join(tmp, "parent2")
	if err := os.MkdirAll(parentPath1, 0o755); err != nil {
		t.Fatalf("failed to create parent1: %v", err)
	}
	if err := os.MkdirAll(parentPath2, 0o755); err != nil {
		t.Fatalf("failed to create parent2: %v", err)
	}

	childPath := filepath.Join(tmp, "child", "link")

	// Create symlink to parent1
	if err := CreateSymbolicLink(childPath, parentPath1); err != nil {
		t.Fatalf("CreateSymbolicLink returned error: %v", err)
	}
	oldTarget, err := os.Readlink(childPath)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}

	// Retarget to parent2
	if err := CreateSymbolicLink(childPath, parentPath2); err != nil {
		t.Fatalf("CreateSymbolicLink returned error on retarget: %v", err)
	}

	newTarget, err := os.Readlink(childPath)
	if err != nil {
		t.Fatalf("readlink failed: %v", err)
	}

	if oldTarget == newTarget {
		t.Fatalf("symlink target should have changed, still %q", newTarget)
	}

	expectedRel2, err := filepath.Rel(filepath.Dir(childPath), parentPath2)
	if err != nil {
		t.Fatalf("failed to compute expected relative path: %v", err)
	}
	assertSymlink(t, childPath, expectedRel2, parentPath2)
}

func TestCreateSymbolicLink_ErrorWhenNonSymlinkAlreadyExists(t *testing.T) {
	tmp := t.TempDir()

	parentPath := filepath.Join(tmp, "parent")
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	childPath := filepath.Join(tmp, "child", "link")

	// Create a regular file at childPath
	if err := os.MkdirAll(filepath.Dir(childPath), 0o755); err != nil {
		t.Fatalf("failed to create child dir: %v", err)
	}
	if err := os.WriteFile(childPath, []byte("regular file"), 0o644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

	if err := CreateSymbolicLink(childPath, parentPath); err == nil {
		t.Fatalf("expected error when non-symlink already exists at childPath")
	}

	info, err := os.Lstat(childPath)
	if err != nil {
		t.Fatalf("lstat failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("existing file should not be converted to symlink")
	}
}

func TestCreateSymbolicLink_CreatesParentDirsForChildPath(t *testing.T) {
	tmp := t.TempDir()

	parentPath := filepath.Join(tmp, "targetDir")
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}

	// Intentionally nested childPath parent that doesn't exist
	childPath := filepath.Join(tmp, "a", "very", "deep", "dir", "link")

	// Precondition: parent of childPath should not exist
	if _, err := os.Stat(filepath.Dir(childPath)); !os.IsNotExist(err) {
		t.Fatalf("expected parent dir to not exist before call")
	}

	if err := CreateSymbolicLink(childPath, parentPath); err != nil {
		t.Fatalf("CreateSymbolicLink returned error: %v", err)
	}

	// Parent dir should now exist and link should be correct
	if _, err := os.Stat(filepath.Dir(childPath)); err != nil {
		t.Fatalf("expected parent dir to exist after call, err: %v", err)
	}

	expectedRel, err := filepath.Rel(filepath.Dir(childPath), parentPath)
	if err != nil {
		t.Fatalf("failed to compute expected relative path: %v", err)
	}
	assertSymlink(t, childPath, expectedRel, parentPath)
}

func TestContainsString(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	scenarios := map[string]struct {
		values          []interface{}
		target          string
		isCaseSensitive bool
		expected        bool
	}{
		"Sensitive_Match": {
			values:          []interface{}{"hey", "Hello"},
			target:          "Hello",
			isCaseSensitive: true,
			expected:        true,
		},
		"Sensitive_NoMatch_DifferentCase": {
			values:          []interface{}{"hey", "Hello"},
			target:          "hello",
			isCaseSensitive: true,
			expected:        false,
		},
		"Insensitive_Match_DifferentCase": {
			values:          []interface{}{"hey", "Hello"},
			target:          "hello",
			isCaseSensitive: false,
			expected:        true,
		},
		"NotFoundInAllStrings": {
			values:          []interface{}{"hey", "hello"},
			target:          "he",
			isCaseSensitive: true,
			expected:        false,
		},
		"FoundWithMixedTypes": {
			values:          []interface{}{"a", 123, "b", 4.56},
			target:          "b",
			isCaseSensitive: true,
			expected:        true,
		},
		"NotFoundWithMixedTypesOnlyNonStringMatch": {
			values:          []interface{}{123, 456},
			target:          "123",
			isCaseSensitive: false,
			expected:        false,
		},
		"EmptySlice": {
			values:          []interface{}{},
			target:          "a",
			isCaseSensitive: false,
			expected:        false,
		},
		"NilSlice": {
			values:          nil,
			target:          "a",
			isCaseSensitive: true,
			expected:        false,
		},
		"ContainsNilElements": {
			values:          []interface{}{"a", nil, "c"},
			target:          "a",
			isCaseSensitive: true,
			expected:        true,
		},
		"Duplicates": {
			values:          []interface{}{"x", "x", "y"},
			target:          "x",
			isCaseSensitive: true,
			expected:        true,
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			got := ContainsString(scenario.values, scenario.target, scenario.isCaseSensitive)
			g.Expect(got).To(gomega.Equal(scenario.expected))
		})
	}
}

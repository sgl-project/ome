package runtime

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestGeneratedInferenceServiceCRDDefaultsFileObject(t *testing.T) {
	generatedCRD := generatedInferenceServiceCRDPath()
	o := &renderOptions{
		inferenceServiceCRDPath: generatedCRD,
		readFile:                readRenderFile,
		IOStreams:               genericiooptions.IOStreams{ErrOut: io.Discard},
	}
	structural, err := o.loadInferenceServiceStructuralSchema()
	require.NoError(t, err)

	objects, err := o.decodeFileObjects("service.yaml", []byte(`apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: service
spec:
  model:
    name: llama
    overlays:
    - name: adapter
  runtime:
    name: runtime
`), "team-a", structural)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	isvc := objects[0].object.(*v1beta1.InferenceService)

	require.NotNil(t, isvc.Spec.Model)
	require.NotNil(t, isvc.Spec.Model.Kind)
	assert.Equal(t, "ClusterBaseModel", *isvc.Spec.Model.Kind)
	require.NotNil(t, isvc.Spec.Model.APIGroup)
	assert.Equal(t, "ome.io", *isvc.Spec.Model.APIGroup)
	require.Len(t, isvc.Spec.Model.Overlays, 1)
	require.NotNil(t, isvc.Spec.Model.Overlays[0].Kind)
	assert.Equal(t, "ClusterBaseModel", *isvc.Spec.Model.Overlays[0].Kind)
	require.NotNil(t, isvc.Spec.Model.Overlays[0].APIGroup)
	assert.Equal(t, "ome.io", *isvc.Spec.Model.Overlays[0].APIGroup)

	require.NotNil(t, isvc.Spec.Runtime)
	require.NotNil(t, isvc.Spec.Runtime.Kind)
	assert.Equal(t, "ClusterServingRuntime", *isvc.Spec.Runtime.Kind)
	require.NotNil(t, isvc.Spec.Runtime.APIGroup)
	assert.Equal(t, "ome.io", *isvc.Spec.Runtime.APIGroup)
	require.NotNil(t, isvc.Spec.Runtime.AutoSync)
	assert.True(t, *isvc.Spec.Runtime.AutoSync)
}

func generatedInferenceServiceCRDPath() string {
	return filepath.Join("..", "..", "..", "..", "config", "crd", "full", "ome.io_inferenceservices.yaml")
}

func TestInferenceServiceCRDDefaultsAreSchemaDrivenAndPreserveExplicitValues(t *testing.T) {
	changedDefaults := strings.Replace(renderFileInferenceServiceCRD,
		"default: ClusterServingRuntime", "default: ServingRuntime", 1)
	changedDefaults = strings.Replace(changedDefaults, "default: true", "default: false", 1)

	t.Run("schema defaults", func(t *testing.T) {
		isvc := decodeFileInferenceService(t, changedDefaults, `apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: service
spec:
  runtime:
    name: runtime
`)

		require.NotNil(t, isvc.Spec.Runtime.Kind)
		assert.Equal(t, "ServingRuntime", *isvc.Spec.Runtime.Kind)
		require.NotNil(t, isvc.Spec.Runtime.AutoSync)
		assert.False(t, *isvc.Spec.Runtime.AutoSync)
	})

	t.Run("explicit values", func(t *testing.T) {
		isvc := decodeFileInferenceService(t, renderFileInferenceServiceCRD, `apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: service
spec:
  runtime:
    name: runtime
    kind: ServingRuntime
    apiGroup: private.example
    autoSync: false
`)

		require.NotNil(t, isvc.Spec.Runtime.Kind)
		assert.Equal(t, "ServingRuntime", *isvc.Spec.Runtime.Kind)
		require.NotNil(t, isvc.Spec.Runtime.APIGroup)
		assert.Equal(t, "private.example", *isvc.Spec.Runtime.APIGroup)
		require.NotNil(t, isvc.Spec.Runtime.AutoSync)
		assert.False(t, *isvc.Spec.Runtime.AutoSync, "an explicit false must not be replaced by the true schema default")
	})
}

func TestInferenceServiceCRDFailures(t *testing.T) {
	invalidDefault := strings.Replace(renderFileInferenceServiceCRD,
		"default: true", `default: "not-a-boolean"`, 1)
	invalidStructuralInvariant := strings.Replace(renderFileInferenceServiceCRD,
		"          spec:\n            type: object\n            properties:",
		"          spec:\n            type: object\n            allOf:\n            - properties:\n                undeclared:\n                  type: string\n            properties:", 1)
	nonStructural := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: inferenceservices.ome.io
spec:
  group: ome.io
  names:
    kind: InferenceService
    plural: inferenceservices
  scope: Namespaced
  versions:
  - name: v1beta1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: array
        items:
        - type: string
        - type: integer
`
	withoutSchema := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: inferenceservices.ome.io
spec:
  group: ome.io
  names:
    kind: InferenceService
    plural: inferenceservices
  scope: Namespaced
  versions:
  - name: v1beta1
    served: true
    storage: true
`

	for _, test := range []struct {
		name     string
		manifest string
		contains string
	}{
		{name: "malformed YAML", manifest: "spec: [", contains: "decode manifest"},
		{name: "duplicate key", manifest: strings.Replace(renderFileInferenceServiceCRD, "  group: ome.io", "  group: ome.io\n  group: other.io", 1), contains: `key "group" already set in map`},
		{name: "wrong object kind", manifest: strings.Replace(renderFileInferenceServiceCRD, "kind: CustomResourceDefinition", "kind: CustomResourceDefinitionList", 1), contains: "want apiextensions.k8s.io/v1 CustomResourceDefinition"},
		{name: "wrong resource identity", manifest: strings.Replace(renderFileInferenceServiceCRD, "name: inferenceservices.ome.io", "name: models.ome.io", 1), contains: "want namespaced CRD inferenceservices.ome.io"},
		{name: "missing v1beta1", manifest: strings.Replace(renderFileInferenceServiceCRD, "name: v1beta1", "name: v1alpha1", 1), contains: "version v1beta1 is not defined"},
		{name: "unserved v1beta1", manifest: strings.Replace(renderFileInferenceServiceCRD, "served: true", "served: false", 1), contains: "version v1beta1 is not served"},
		{name: "missing schema", manifest: withoutSchema, contains: "version v1beta1 has no openAPIV3Schema"},
		{name: "non-structural schema", manifest: nonStructural, contains: "schema is not structural"},
		{name: "invalid structural invariant", manifest: invalidStructuralInvariant, contains: "schema is not structural"},
		{name: "invalid default", manifest: invalidDefault, contains: "version v1beta1 defaults are invalid"},
		{name: "multiple objects", manifest: renderFileInferenceServiceCRD + "---\n" + renderFileInferenceServiceCRD, contains: "manifest must contain exactly one object"},
	} {
		t.Run(test.name, func(t *testing.T) {
			o := &renderOptions{
				inferenceServiceCRDPath: "isvc-crd.yaml",
				readFile: func(path string) ([]byte, error) {
					assert.Equal(t, "isvc-crd.yaml", path)
					return []byte(test.manifest), nil
				},
			}
			_, err := o.loadInferenceServiceStructuralSchema()
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.contains)
		})
	}

	t.Run("unreadable", func(t *testing.T) {
		o := &renderOptions{inferenceServiceCRDPath: "absent.yaml", readFile: readRenderFile}
		_, err := o.loadInferenceServiceStructuralSchema()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read --inference-service-crd")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func decodeFileInferenceService(t *testing.T, crd, service string) *v1beta1.InferenceService {
	t.Helper()
	o := &renderOptions{
		inferenceServiceCRDPath: "isvc-crd.yaml",
		readFile: func(string) ([]byte, error) {
			return []byte(crd), nil
		},
		IOStreams: genericiooptions.IOStreams{ErrOut: io.Discard},
	}
	structural, err := o.loadInferenceServiceStructuralSchema()
	require.NoError(t, err)
	objects, err := o.decodeFileObjects("service.yaml", []byte(service), "team-a", structural)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	isvc, ok := objects[0].object.(*v1beta1.InferenceService)
	require.True(t, ok)
	return isvc
}

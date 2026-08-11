package endpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestConfig_IsEnabled(t *testing.T) {
	assert.False(t, Config{}.IsEnabled(), "no gateway -> disabled")
	assert.False(t, Config{GlobalGateway: "  "}.IsEnabled(), "whitespace gateway -> disabled")
	assert.True(t, Config{GlobalGateway: "ns/gw"}.IsEnabled())
}

func TestConfig_GlobalHostFor(t *testing.T) {
	isvc := func(ann map[string]string) *v1beta1.InferenceService {
		return &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "prod", Annotations: ann}}
	}

	t.Run("annotation overrides template", func(t *testing.T) {
		cfg := Config{GlobalHostTemplate: "{{.Name}}.{{.Namespace}}.global.example"}
		h, err := cfg.GlobalHostFor(isvc(map[string]string{GlobalHostAnnotation: "pinned.example"}))
		require.NoError(t, err)
		assert.Equal(t, "pinned.example", h)
	})

	t.Run("template render", func(t *testing.T) {
		cfg := Config{GlobalHostTemplate: "{{.Name}}.{{.Namespace}}.global.example"}
		h, err := cfg.GlobalHostFor(isvc(nil))
		require.NoError(t, err)
		assert.Equal(t, "svc.prod.global.example", h)
	})

	t.Run("no template and no annotation -> empty (not publishable, no magic default)", func(t *testing.T) {
		h, err := Config{}.GlobalHostFor(isvc(nil))
		require.NoError(t, err)
		assert.Equal(t, "", h)
	})

	t.Run("empty template with annotation still publishes", func(t *testing.T) {
		h, err := Config{}.GlobalHostFor(isvc(map[string]string{GlobalHostAnnotation: "pinned.example"}))
		require.NoError(t, err)
		assert.Equal(t, "pinned.example", h)
	})

	t.Run("bad template -> error", func(t *testing.T) {
		_, err := Config{GlobalHostTemplate: "{{.Nope"}.GlobalHostFor(isvc(nil))
		assert.Error(t, err)
	})
}

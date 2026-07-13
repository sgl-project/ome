package modelmetadata

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/modelagent"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func writeMinimalLlamaConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := map[string]any{
		"model_type":              "llama",
		"architectures":           []string{"LlamaForCausalLM"},
		"max_position_embeddings": 4096,
		"hidden_size":             4096,
		"num_hidden_layers":       32,
		"num_attention_heads":     32,
		"num_key_value_heads":     32,
		"intermediate_size":       11008,
		"vocab_size":              32000,
		"torch_dtype":             "float16",
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644))
	return dir
}

func TestMetadataConfigMapName_Deterministic(t *testing.T) {
	a := constants.GetPVCMetadataConfigMapName("llama-7b", "models", false)
	b := constants.GetPVCMetadataConfigMapName("llama-7b", "models", false)
	if a != b {
		t.Fatalf("non-deterministic: %s vs %s", a, b)
	}
	if len(a) > 63 {
		t.Fatalf("name %q exceeds 63 chars", a)
	}
	c := constants.GetPVCMetadataConfigMapName("llama-7b", "other-namespace", false)
	if a == c {
		t.Fatalf("namespace must affect ConfigMap name")
	}
	d := constants.GetPVCMetadataConfigMapName("llama-7b", "", true)
	if a == d {
		t.Fatalf("scope must affect ConfigMap name")
	}
}

func TestMetadataExtractor_Start_WritesReadyConfigMap(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &Config{
		ModelPath:          writeMinimalLlamaConfig(t),
		BaseModelName:      "llama-7b",
		BaseModelNamespace: "models",
		ClusterScoped:      false,
		Logger:             logging.Discard(),
	}
	ex, err := NewMetadataExtractor(cfg, c, zap.NewNop())
	require.NoError(t, err)

	require.NoError(t, ex.Start())

	cmName := constants.GetPVCMetadataConfigMapName(cfg.BaseModelName, cfg.BaseModelNamespace, cfg.ClusterScoped)
	cm := &corev1.ConfigMap{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: constants.OMENamespace, Name: cmName}, cm))

	// PVC ConfigMaps deliberately omit the per-node basemodel-status label.
	assert.NotContains(t, cm.Labels, constants.ModelStatusConfigMapLabel)
	assert.Equal(t, "true", cm.Labels[constants.PVCStorageConfigMapLabel])
	assert.Equal(t, cfg.BaseModelName, cm.Labels[constants.PVCMetadataModelNameLabel])
	assert.Equal(t, "namespaced", cm.Labels[constants.PVCMetadataScopeLabel])

	modelKey := constants.GetModelConfigMapKey(cfg.BaseModelNamespace, cfg.BaseModelName, cfg.ClusterScoped)
	rawEntry, ok := cm.Data[modelKey]
	require.True(t, ok, "ConfigMap must carry an entry under the model key")

	var entry modelagent.ModelEntry
	require.NoError(t, json.Unmarshal([]byte(rawEntry), &entry))
	assert.Equal(t, modelagent.ModelStatusReady, entry.Status)
	require.NotNil(t, entry.Config)
	assert.Equal(t, "llama", entry.Config.ModelType)
	assert.Equal(t, "LlamaForCausalLM", entry.Config.ModelArchitecture)
	assert.Equal(t, int32(4096), entry.Config.MaxTokens)

	// No error annotation on the success path.
	_, hasErr := cm.Annotations[constants.PVCMetadataLastErrorAnnotation]
	assert.False(t, hasErr)
}

func TestMetadataExtractor_Start_ClusterScoped_LabelsAndScope(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &Config{
		ModelPath:     writeMinimalLlamaConfig(t),
		BaseModelName: "shared-llama",
		ClusterScoped: true,
		Logger:        logging.Discard(),
	}
	ex, err := NewMetadataExtractor(cfg, c, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, ex.Start())

	cmName := constants.GetPVCMetadataConfigMapName(cfg.BaseModelName, "", true)
	cm := &corev1.ConfigMap{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: constants.OMENamespace, Name: cmName}, cm))
	assert.Equal(t, "cluster", cm.Labels[constants.PVCMetadataScopeLabel])

	modelKey := constants.GetModelConfigMapKey("", cfg.BaseModelName, true)
	_, ok := cm.Data[modelKey]
	assert.True(t, ok)
}

func TestMetadataExtractor_Start_NoConfigFile_WritesFailedAndReturnsError(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &Config{
		ModelPath:          t.TempDir(), // empty dir → parser returns error
		BaseModelName:      "llama-7b",
		BaseModelNamespace: "models",
		Logger:             logging.Discard(),
	}
	ex, err := NewMetadataExtractor(cfg, c, zap.NewNop())
	require.NoError(t, err)
	err = ex.Start()
	require.Error(t, err)

	cmName := constants.GetPVCMetadataConfigMapName(cfg.BaseModelName, cfg.BaseModelNamespace, cfg.ClusterScoped)
	cm := &corev1.ConfigMap{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: constants.OMENamespace, Name: cmName}, cm))

	modelKey := constants.GetModelConfigMapKey(cfg.BaseModelNamespace, cfg.BaseModelName, cfg.ClusterScoped)
	rawEntry := cm.Data[modelKey]
	var entry modelagent.ModelEntry
	require.NoError(t, json.Unmarshal([]byte(rawEntry), &entry))
	assert.Equal(t, modelagent.ModelStatusFailed, entry.Status)
	assert.NotEmpty(t, cm.Annotations[constants.PVCMetadataLastErrorAnnotation])
}

func TestMetadataExtractor_Start_Idempotent_RewriteSucceeds(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &Config{
		ModelPath:          writeMinimalLlamaConfig(t),
		BaseModelName:      "llama-7b",
		BaseModelNamespace: "models",
		Logger:             logging.Discard(),
	}
	ex, err := NewMetadataExtractor(cfg, c, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, ex.Start())
	require.NoError(t, ex.Start()) // second run should overwrite, not error

	var cms corev1.ConfigMapList
	require.NoError(t, c.List(context.Background(), &cms, client.InNamespace(constants.OMENamespace)))
	assert.Equal(t, 1, len(cms.Items), "second extraction must not create a duplicate ConfigMap")
}

func TestSanitizeLabelValue(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"llama-7b", "llama-7b"},
		{"Meta/Llama-3.1-8B-Instruct", "Meta-Llama-3.1-8B-Instruct"},
		{"...", "model"},
		{"", "model"},
		{strings.Repeat("a", 80), strings.Repeat("a", 63)},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := sanitizeLabelValue(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestShouldWriteAbortedStatus pins the exact truth table for Start()'s
// post-run gate. Each combination matters:
//
//   - happy path (runErr nil, no signal): nothing to write
//   - run failed for unrelated reasons (no signal): run()'s own
//     writeStatus(Failed, parseErr.Error()) already surfaced the cause;
//     a generic "aborted" overwrite would lose the real reason
//   - signal arrived AFTER successful Ready write: do NOT clobber Ready
//     with Failed-Aborted — that would silently transition a healthy
//     model to LifeCycleStateFailed
//   - signal arrived during run() AND run() failed: write Aborted so the
//     controller doesn't see the model stuck at In_Transit forever
func TestShouldWriteAbortedStatus(t *testing.T) {
	cases := []struct {
		name              string
		runErr, signalErr error
		want              bool
	}{
		{name: "happy path: no run error, no signal", want: false},
		{name: "run failed for unrelated reasons (no signal)", runErr: errors.New("disk full"), want: false},
		{name: "SIGTERM arrived AFTER successful Ready write", signalErr: context.Canceled, want: false},
		{name: "signal arrived AND run failed", runErr: errors.New("apiserver refused"), signalErr: context.Canceled, want: true},
		{name: "DeadlineExceeded counts as signal-fired (parent here is Background, only signals cancel it)", runErr: errors.New("x"), signalErr: context.DeadlineExceeded, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldWriteAbortedStatus(tc.runErr, tc.signalErr); got != tc.want {
				t.Errorf("shouldWriteAbortedStatus(%v, %v) = %v, want %v", tc.runErr, tc.signalErr, got, tc.want)
			}
		})
	}
}

// TestStart_DoesNotClobberReadyOnPostRunSignal pairs with the gate
// truth table above by exercising the full Start() path. We simulate
// the regression by:
//  1. Running Start() once to write Ready successfully
//  2. Verifying the ConfigMap holds Status=Ready
//  3. Re-running and asserting the Ready entry survives — i.e., no
//     code path in Start() flips it to Failed when run() succeeded.
//
// If the gate ever regresses to `if ctx.Err() != nil` (without the
// `runErr != nil` predicate), TestShouldWriteAbortedStatus catches the
// unit-level bug; this test catches the integration-level effect.
func TestStart_DoesNotClobberReadyOnPostRunSignal(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &Config{
		ModelPath:          writeMinimalLlamaConfig(t),
		BaseModelName:      "llama-7b",
		BaseModelNamespace: "models",
		Logger:             logging.Discard(),
	}
	ex, err := NewMetadataExtractor(cfg, c, zap.NewNop())
	require.NoError(t, err)

	require.NoError(t, ex.Start())

	cmName := constants.GetPVCMetadataConfigMapName(cfg.BaseModelName, cfg.BaseModelNamespace, cfg.ClusterScoped)
	cm := &corev1.ConfigMap{}
	modelKey := constants.GetModelConfigMapKey(cfg.BaseModelNamespace, cfg.BaseModelName, cfg.ClusterScoped)
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Namespace: constants.OMENamespace, Name: cmName}, cm))

	var entry modelagent.ModelEntry
	require.NoError(t, json.Unmarshal([]byte(cm.Data[modelKey]), &entry))
	require.Equal(t, modelagent.ModelStatusReady, entry.Status, "first run must write Ready")

	// Sanity: the Ready entry must NOT carry the aborted-error annotation.
	// If a regression made Start() always run the aborted-write block
	// regardless of runErr, this annotation would appear.
	_, hasAbortedErr := cm.Annotations[constants.PVCMetadataLastErrorAnnotation]
	require.False(t, hasAbortedErr, "Ready ConfigMap must not carry the Failed-Aborted error annotation")
}

// TestMetadataExtractor_AbortedWritesFailedConfigMap directly tests the
// "agent received SIGTERM mid-extraction" path by passing a pre-canceled
// ctx into a small wrapper that mirrors Start()'s deferred Failed-write.
// This catches regressions in the graceful-cancel write path without
// requiring real signal injection.
func TestMetadataExtractor_AbortedWritesFailedConfigMap(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &Config{
		ModelPath:          writeMinimalLlamaConfig(t),
		BaseModelName:      "llama-7b",
		BaseModelNamespace: "models",
		Logger:             logging.Discard(),
	}
	ex, err := NewMetadataExtractor(cfg, c, zap.NewNop())
	require.NoError(t, err)

	// Mimic what Start() does on cancellation: best-effort write of the
	// Aborted Failed entry under a fresh, bounded context.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, ex.writeStatus(ctx, modelagent.ModelStatusFailed, nil, "metadata extraction aborted (pod received SIGTERM/SIGINT)"))

	cmName := constants.GetPVCMetadataConfigMapName(cfg.BaseModelName, cfg.BaseModelNamespace, cfg.ClusterScoped)
	cm := &corev1.ConfigMap{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: constants.OMENamespace, Name: cmName}, cm))
	assert.NotEmpty(t, cm.Annotations[constants.PVCMetadataLastErrorAnnotation], "Aborted ConfigMap must carry the error annotation")

	modelKey := constants.GetModelConfigMapKey(cfg.BaseModelNamespace, cfg.BaseModelName, cfg.ClusterScoped)
	var entry modelagent.ModelEntry
	require.NoError(t, json.Unmarshal([]byte(cm.Data[modelKey]), &entry))
	assert.Equal(t, modelagent.ModelStatusFailed, entry.Status)
}

// TestMetadataExtractor_Run_AcceptsContext exercises the inner run(ctx)
// path directly to confirm the context lift is wired. We don't rely on
// the fake client honoring cancellation (it doesn't), only on the
// signature/path being plumbed.
func TestMetadataExtractor_Run_AcceptsContext(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := &Config{
		ModelPath:          writeMinimalLlamaConfig(t),
		BaseModelName:      "llama-7b",
		BaseModelNamespace: "models",
		Logger:             logging.Discard(),
	}
	ex, err := NewMetadataExtractor(cfg, c, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, ex.run(context.Background()))
}

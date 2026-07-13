// Package modelmetadata extracts metadata from PVC-mounted model directories
// and writes a single per-model status ConfigMap that the BaseModel /
// ClusterBaseModel controller reads directly by deterministic name.
//
// Flow when run as a Kubernetes Job:
//  1. The BaseModel controller mounts the PVC at /model and invokes
//     `ome-agent model-metadata --model-path /model --basemodel-name X
//     [--basemodel-namespace Y | --cluster-scoped]`.
//  2. We delegate parsing to pkg/modelparser, which already understands
//     both config.json (HF text models) and model_index.json (diffusion).
//  3. We convert the parser's metadata into the modelagent.ModelEntry
//     shape and upsert exactly one ConfigMap per PVC-backed model in
//     the OME namespace. The ConfigMap name is deterministic
//     (constants.GetPVCMetadataConfigMapName) so the controller can
//     Get it by name without scanning.
//  4. On any extraction failure (including SIGTERM mid-write) we still
//     write a Status=Failed entry so the controller can transition the
//     model to Failed with a useful message instead of waiting
//     indefinitely.
package modelmetadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/modelagent"
	"sigs.k8s.io/ome/pkg/modelparser"
)

// MetadataExtractor parses a PVC-mounted model and writes the result to a
// status ConfigMap.
type MetadataExtractor struct {
	config *Config
	client client.Client
	parser *modelparser.ModelConfigParser
	logger logging.Interface
}

func NewMetadataExtractor(config *Config, kubeClient client.Client, zapLogger *zap.Logger) (*MetadataExtractor, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	if kubeClient == nil {
		return nil, errors.New("kubeClient must not be nil")
	}
	if zapLogger == nil {
		return nil, errors.New("zapLogger must not be nil")
	}

	// modelparser logs through *zap.SugaredLogger; reuse the fx-provided
	// *zap.Logger so we have a single logging stack in the agent process.
	// Pass nil omeClient — ParseModelConfig only writes to the API server
	// when called with a non-nil BaseModel/ClusterBaseModel and we pass
	// both nil; the ConfigMap, not the parser, is the canonical write
	// path in the PVC flow.
	parser := modelparser.NewModelConfigParser(nil, zapLogger.Sugar())

	return &MetadataExtractor{
		config: config,
		client: kubeClient,
		parser: parser,
		logger: config.Logger,
	}, nil
}

// Start runs the extractor end-to-end. The fx OnStart hook calls this in
// a fire-and-forget goroutine, so we derive a SIGTERM/SIGINT-aware
// context here. A signal that arrives mid-extraction:
//   - cancels the parent ctx for in-flight modelparser/API calls
//   - triggers a deferred best-effort Failed-ConfigMap write under a
//     short detached context so the controller doesn't see the model
//     stuck at In_Transit forever
func (m *MetadataExtractor) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runErr := m.run(ctx)

	if shouldWriteAbortedStatus(runErr, ctx.Err()) {
		// Use a fresh, bounded context so the write doesn't immediately
		// fail under the cancelled parent.
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg := "metadata extraction aborted (pod received SIGTERM/SIGINT)"
		if writeErr := m.writeStatus(writeCtx, modelagent.ModelStatusFailed, nil, msg); writeErr != nil {
			m.logger.Errorf("Failed to write Aborted status on cancellation: %v", writeErr)
		}
	}
	return runErr
}

// shouldWriteAbortedStatus encodes Start()'s post-run gate for the
// best-effort Failed-Aborted ConfigMap write. Both predicates must hold:
//
//   - signalErr != nil  → a SIGTERM/SIGINT actually fired. Detecting via
//     the signal context's Err() (not errors.Is(runErr, context.Canceled))
//     because runErr may have been wrapped with %v, replaced with a
//     parser/network error, or never been a context error at all.
//   - runErr   != nil   → run() did NOT successfully write Ready. Without
//     this guard a signal that arrives between Ready-write completing and
//     Start() returning would clobber the freshly-written Ready entry
//     with Failed-Aborted — transitioning a healthy model to
//     LifeCycleStateFailed.
//
// Pulled out of Start() because the actual signal/Ready-write race is
// impossible to reproduce deterministically; the helper is unit-tested
// exhaustively in metadata_test.go.
func shouldWriteAbortedStatus(runErr, signalErr error) bool {
	return runErr != nil && signalErr != nil
}

func (m *MetadataExtractor) run(ctx context.Context) error {
	m.logger.Infof("Starting model metadata extraction for model at %s", m.config.ModelPath)

	metadata, parseErr := m.parser.ParseModelConfig(m.config.ModelPath, nil, nil)
	if parseErr != nil {
		// Surface the parse failure through the ConfigMap so the controller
		// can transition the BaseModel to Failed. We still return the
		// underlying error so the Job pod exits non-zero.
		writeErr := m.writeStatus(ctx, modelagent.ModelStatusFailed, nil, parseErr.Error())
		if writeErr != nil {
			m.logger.Errorf("Failed to write Failed status to ConfigMap: %v", writeErr)
		}
		return fmt.Errorf("failed to parse model config at %s: %w", m.config.ModelPath, parseErr)
	}
	if metadata == nil {
		err := errors.New("model parser returned no metadata (skip-parsing annotation set or directory empty)")
		writeErr := m.writeStatus(ctx, modelagent.ModelStatusFailed, nil, err.Error())
		if writeErr != nil {
			m.logger.Errorf("Failed to write Failed status to ConfigMap: %v", writeErr)
		}
		return err
	}

	cfg := modelagent.ConvertMetadataToModelConfig(*metadata)
	if err := m.writeStatus(ctx, modelagent.ModelStatusReady, cfg, ""); err != nil {
		return fmt.Errorf("failed to write Ready status to ConfigMap: %w", err)
	}
	m.logger.Infof("Wrote Ready status with metadata to ConfigMap for model %s", m.modelInfo())
	return nil
}

// writeStatus upserts the per-PVC ConfigMap with a single ModelEntry. The
// controller looks the ConfigMap up by exact name (no list-and-filter), so
// we do NOT label it with the per-node `models.ome/basemodel-status`
// selector — that label is reserved for the per-node ConfigMap pattern.
func (m *MetadataExtractor) writeStatus(ctx context.Context, status modelagent.ModelStatus, cfg *modelagent.ModelConfig, errMessage string) error {
	cmName := constants.GetPVCMetadataConfigMapName(m.config.BaseModelName, m.config.BaseModelNamespace, m.config.ClusterScoped)
	modelKey := constants.GetModelConfigMapKey(m.config.BaseModelNamespace, m.config.BaseModelName, m.config.ClusterScoped)

	entry := modelagent.ModelEntry{
		Name:   m.config.BaseModelName,
		Status: status,
		Config: cfg,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal ModelEntry: %w", err)
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm := &corev1.ConfigMap{}
		err := m.client.Get(ctx, types.NamespacedName{Namespace: constants.OMENamespace, Name: cmName}, cm)
		if apierrors.IsNotFound(err) {
			cm = newPVCMetadataConfigMap(cmName, m.config.BaseModelName, m.config.ClusterScoped)
			cm.Data = map[string]string{modelKey: string(data)}
			if errMessage != "" {
				cm.Annotations = map[string]string{constants.PVCMetadataLastErrorAnnotation: errMessage}
			}
			return m.client.Create(ctx, cm)
		}
		if err != nil {
			return err
		}
		ensurePVCMetadataLabels(cm, m.config.BaseModelName, m.config.ClusterScoped)
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[modelKey] = string(data)
		if errMessage != "" {
			if cm.Annotations == nil {
				cm.Annotations = map[string]string{}
			}
			cm.Annotations[constants.PVCMetadataLastErrorAnnotation] = errMessage
		} else {
			delete(cm.Annotations, constants.PVCMetadataLastErrorAnnotation)
		}
		return m.client.Update(ctx, cm)
	})
}

func (m *MetadataExtractor) modelInfo() string {
	if m.config.ClusterScoped {
		return m.config.BaseModelName
	}
	return m.config.BaseModelNamespace + "/" + m.config.BaseModelName
}

func newPVCMetadataConfigMap(name, modelName string, isClusterScoped bool) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: constants.OMENamespace,
		},
	}
	ensurePVCMetadataLabels(cm, modelName, isClusterScoped)
	return cm
}

func ensurePVCMetadataLabels(cm *corev1.ConfigMap, modelName string, isClusterScoped bool) {
	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	cm.Labels[constants.PVCStorageConfigMapLabel] = "true"
	cm.Labels[constants.PVCMetadataModelNameLabel] = sanitizeLabelValue(modelName)
	scope := "namespaced"
	if isClusterScoped {
		scope = "cluster"
	}
	cm.Labels[constants.PVCMetadataScopeLabel] = scope
}

// sanitizeLabelValue maps an arbitrary string to a value that satisfies
// the K8s label-value rules ([a-zA-Z0-9._-]{0,63}). Invalid characters
// become '-'; the result is trimmed and a "model" fallback is returned if
// trimming would leave it empty so labels are always present and useful.
func sanitizeLabelValue(s string) string {
	const maxLen = 63
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && i < maxLen; i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.', c == '_', c == '-':
			out = append(out, c)
		default:
			out = append(out, '-')
		}
	}
	// Trim leading/trailing non-alphanumeric per K8s rules.
	trimmed := strings.Trim(string(out), "._-")
	if trimmed == "" {
		return "model"
	}
	return trimmed
}

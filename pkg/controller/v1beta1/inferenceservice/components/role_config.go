package components

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

// RoleConfig captures the resolved per-component configuration that an
// All-in-One workload strategy (such as RBG) needs to assemble a single
// orchestrator resource. It is the data structure exchanged between
// strategies and downstream reconcilers and is filled in by extractors
// that share the same pod-spec building logic used during single component
// reconciliation.
type RoleConfig struct {
	// ComponentType identifies which InferenceService role this config
	// represents (Engine / Decoder / Router).
	ComponentType v1beta1.ComponentType

	// DeploymentMode is the per-component deployment mode resolved by the
	// controller. Strategies use it to decide how to project the role into
	// the underlying workload (e.g. Deployment vs LeaderWorkerSet).
	DeploymentMode constants.DeploymentModeType

	// PodSpec is the main pod spec. For RawDeployment it is the full pod
	// spec; for MultiNode it doubles as the leader pod spec.
	PodSpec *corev1.PodSpec

	// LeaderPodSpec is set for MultiNode deployments and equals PodSpec.
	// Kept as a separate field so callers can be explicit at the call site.
	LeaderPodSpec *corev1.PodSpec

	// WorkerPodSpec is the worker pod spec for MultiNode deployments.
	// nil for RawDeployment / when no worker spec is defined.
	WorkerPodSpec *corev1.PodSpec

	// WorkerSize is the number of worker pods per replica for MultiNode
	// deployments. 0 for RawDeployment.
	WorkerSize int

	// ComponentExtensionSpec carries replica, scaling and disruption
	// configuration shared with the single-component reconcilers.
	ComponentExtensionSpec *v1beta1.ComponentExtensionSpec

	// ObjectMeta is the per-role object metadata. Name defaults to
	// "<isvc>-<component>" and labels/annotations include the InferenceService
	// pod label so downstream HPAs and PDBs match.
	ObjectMeta metav1.ObjectMeta
}

// RoleConfigExtractor is implemented by component types that can be used by
// All-in-One workload strategies. Implementations are expected to be cheap
// and side-effect free with respect to the cluster (other than the model /
// fine-tuned weight lookups already performed during single-component
// reconciliation).
type RoleConfigExtractor interface {
	// ExtractRoleConfig builds the RoleConfig for this component. It must
	// resolve all of: object metadata, primary pod spec, optional worker
	// pod spec and worker size.
	ExtractRoleConfig(isvc *v1beta1.InferenceService) (*RoleConfig, error)
}

// ComponentStatusUpdater is implemented by component types that can
// propagate their status (readiness conditions, model status) into the
// InferenceService status without performing a full reconciliation.
type ComponentStatusUpdater interface {
	UpdateStatus(isvc *v1beta1.InferenceService) error
}

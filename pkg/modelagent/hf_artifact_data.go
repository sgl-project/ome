package modelagent

// HfArtifactStatus represents the lifecycle of a shared Hugging Face artifact
// on one node.
type HfArtifactStatus string

const (
	HfArtifactStatusReady    HfArtifactStatus = "Ready"
	HfArtifactStatusUpdating HfArtifactStatus = "Updating"
	HfArtifactStatusFailed   HfArtifactStatus = "Failed"
)

// HfArtifactIdentity identifies an immutable Hugging Face artifact.
type HfArtifactIdentity struct {
	ModelID   string `json:"modelId"`
	CommitSHA string `json:"commitSha"`
}

// HfArtifactEntry records one shared node-local artifact. It is stored under
// the artifact.huggingface.* key namespace in the node ConfigMap.
type HfArtifactEntry struct {
	Key       string             `json:"key"`
	Status    HfArtifactStatus   `json:"status"`
	Identity  HfArtifactIdentity `json:"identity"`
	LocalPath string             `json:"localPath"`
	// Children maps a model ConfigMap key to its model-local symlink path.
	Children map[string]string `json:"children,omitempty"`
	LockID   string            `json:"lockId,omitempty"`
	// LastCompletedLockID keeps terminal status updates safe to retry. MarkReady
	// and MarkFailed clear LockID after the parent reaches a terminal state; if
	// Kubernetes accepts that update but the response is lost, the same caller
	// can retry and receive a no-op success instead of a stale-owner error.
	LastCompletedLockID string `json:"lastCompletedLockId,omitempty"`
}

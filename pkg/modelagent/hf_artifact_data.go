package modelagent

// HfArtifactStatus represents the lifecycle of a shared Hugging Face artifact.
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

// HfArtifactEntry records a shared artifact's identity, state, and references.
// It uses the artifact.huggingface.* key namespace in the ConfigMap.
type HfArtifactEntry struct {
	Key       string             `json:"key"`
	Status    HfArtifactStatus   `json:"status"`
	Identity  HfArtifactIdentity `json:"identity"`
	LocalPath string             `json:"localPath"`
	// Children maps a model ConfigMap key to its child symlink path.
	Children map[string]string `json:"children,omitempty"`
	// Updating without a LockID and with a LastCompletedLockID is interrupted
	// work waiting for retry or cleanup.
	LockID string `json:"lockId,omitempty"`
	// ChildStatusesBeforeRepair survives failed repairs so a later completion
	// restores only the statuses that repair temporarily replaced with Failed.
	ChildStatusesBeforeRepair map[string]ModelStatus `json:"childStatusesBeforeRepair,omitempty"`
	// LastCompletedLockID makes lock release safe to retry after a lost response.
	// Ready, Failed, and interrupted Updating clear LockID and remember its owner;
	// a new acquisition clears this receipt and assigns a fresh LockID.
	LastCompletedLockID string `json:"lastCompletedLockId,omitempty"`
}

func (entry HfArtifactEntry) interrupted() bool {
	return entry.Status == HfArtifactStatusUpdating && entry.LockID == "" && entry.LastCompletedLockID != ""
}

// blocksNewOperation distinguishes owned work from a released, interrupted
// attempt. An Updating record without either ownership or a release receipt
// also stays blocked; missing ownership must not authorize file changes.
func (entry HfArtifactEntry) blocksNewOperation() bool {
	return entry.Status == HfArtifactStatusUpdating && !entry.interrupted()
}

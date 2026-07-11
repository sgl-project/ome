package modelagent

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func shouldUseHuggingFaceOriginObjectStorageReuse(task *GopherTask, baseModelSpec v1beta1.BaseModelSpec) bool {
	return shouldUseSamePathObjectStorageReuse(task) &&
		baseModelSpec.Storage != nil &&
		baseModelSpec.Storage.DownloadPolicy != nil &&
		*baseModelSpec.Storage.DownloadPolicy == v1beta1.ReuseIfExists
}

func shouldRepairHuggingFaceOriginObjectStorageParent(task *GopherTask, baseModelSpec v1beta1.BaseModelSpec) bool {
	return task != nil &&
		task.TaskType == DownloadOverride &&
		baseModelSpec.Storage != nil &&
		baseModelSpec.Storage.DownloadPolicy != nil &&
		*baseModelSpec.Storage.DownloadPolicy == v1beta1.ReuseIfExists
}

func (i ArtifactIdentity) isValid() bool {
	return strings.EqualFold(i.OriginType, ArtifactOriginTypeHuggingFace) &&
		strings.TrimSpace(i.HFModelID) != "" &&
		isValidHuggingFaceCommitSHA(i.HFCommitSHA)
}

func (i ArtifactIdentity) toOrigin() *ArtifactOrigin {
	if !i.isValid() {
		return nil
	}
	return &ArtifactOrigin{
		Type:        ArtifactOriginTypeHuggingFace,
		HFModelID:   i.HFModelID,
		HFCommitSHA: strings.ToLower(i.HFCommitSHA),
	}
}

func huggingFaceArtifactParentIdentityAndPath(parentKey string, entry ModelEntry) (ArtifactIdentity, string, bool) {
	if entry.Config == nil {
		return ArtifactIdentity{}, "", false
	}
	origin := entry.Config.Artifact.Origin
	if origin == nil {
		return ArtifactIdentity{}, "", false
	}
	identity := ArtifactIdentity{
		OriginType:  origin.Type,
		HFModelID:   origin.HFModelID,
		HFCommitSHA: strings.ToLower(origin.HFCommitSHA),
	}
	parentPath := entry.Config.Artifact.ParentPath[parentKey]
	if !identity.isValid() || strings.TrimSpace(parentPath) == "" {
		return ArtifactIdentity{}, "", false
	}
	return identity, parentPath, true
}

func isValidHuggingFaceCommitSHA(sha string) bool {
	if len(sha) != 40 {
		return false
	}
	for _, c := range sha {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func huggingFaceArtifactIdentityFromTask(task *GopherTask) (ArtifactIdentity, bool) {
	if task == nil {
		return ArtifactIdentity{}, false
	}
	if task.BaseModel != nil {
		return huggingFaceArtifactIdentityFromAnnotations(task.BaseModel.Annotations)
	}
	if task.ClusterBaseModel != nil {
		return huggingFaceArtifactIdentityFromAnnotations(task.ClusterBaseModel.Annotations)
	}
	return ArtifactIdentity{}, false
}

func huggingFaceArtifactIdentityFromAnnotations(annotations map[string]string) (ArtifactIdentity, bool) {
	if annotations == nil {
		return ArtifactIdentity{}, false
	}
	modelID := strings.TrimSpace(annotations[HuggingFaceModelIDAnnotationKey])
	sha := strings.TrimSpace(annotations[HuggingFaceSHAAnnotationKey])
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHuggingFace,
		HFModelID:   modelID,
		HFCommitSHA: strings.ToLower(sha),
	}
	if !identity.isValid() {
		return ArtifactIdentity{}, false
	}
	return identity, true
}

func huggingFaceArtifactConfigMapKey(identity ArtifactIdentity) string {
	return huggingFaceArtifactConfigMapKeyPrefix + sanitizeConfigMapKeyComponent(identity.HFModelID) + "." + shortConfigMapKeyHash(identity.HFModelID) + "." + strings.ToLower(identity.HFCommitSHA)
}

func isHuggingFaceArtifactConfigMapKey(key string) bool {
	return strings.HasPrefix(key, huggingFaceArtifactConfigMapKeyPrefix)
}

// shortConfigMapKeyHash keeps the synthetic parent key readable while avoiding
// collisions from lossy path sanitization of Hugging Face model IDs.
func shortConfigMapKeyHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])[:12]
}

func sanitizeConfigMapKeyComponent(value string) string {
	var b strings.Builder
	for _, c := range strings.TrimSpace(value) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			b.WriteRune(c)
			continue
		}
		b.WriteByte('.')
	}
	sanitized := strings.Trim(b.String(), ".")
	if sanitized == "" {
		return "unknown"
	}
	return sanitized
}

func canonicalHuggingFaceArtifactPath(destPath string, identity ArtifactIdentity) string {
	return filepath.Join(filepath.Dir(destPath), constants.ModelArtifactsDirectory, filepath.FromSlash(strings.Trim(strings.TrimSpace(identity.HFModelID), "/")), strings.ToLower(identity.HFCommitSHA))
}

// BaseModel downloads can have model-local child paths under a local store, so
// place shared HF parents in that same store. ClusterBaseModel keeps the
// existing model-root layout for backward compatibility.
func canonicalHuggingFaceArtifactPathForTask(task *GopherTask, modelRootDir string, destPath string, identity ArtifactIdentity) string {
	if task != nil && task.BaseModel != nil {
		return canonicalHuggingFaceArtifactPath(destPath, identity)
	}
	return filepath.Join(modelRootDir, filepath.FromSlash(strings.Trim(strings.TrimSpace(identity.HFModelID), "/")), strings.ToLower(identity.HFCommitSHA))
}

func huggingFaceArtifactReadyMarkerPath(parentPath string) string {
	return filepath.Join(parentPath, huggingFaceArtifactReadyMarkerFile)
}

func writeHuggingFaceArtifactReadyMarker(parentPath string) error {
	if err := os.MkdirAll(parentPath, 0755); err != nil {
		return err
	}
	return os.WriteFile(huggingFaceArtifactReadyMarkerPath(parentPath), []byte("ready\n"), 0644)
}

func removeHuggingFaceArtifactReadyMarker(parentPath string) error {
	err := os.Remove(huggingFaceArtifactReadyMarkerPath(parentPath))
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func hasHuggingFaceArtifactReadyMarker(parentPath string) bool {
	if strings.TrimSpace(parentPath) == "" {
		return false
	}
	info, err := os.Stat(huggingFaceArtifactReadyMarkerPath(parentPath))
	return err == nil && !info.IsDir()
}

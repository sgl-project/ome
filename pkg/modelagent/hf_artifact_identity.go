package modelagent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"sigs.k8s.io/ome/pkg/constants"
)

func newHfArtifactIdentity(modelID, commitSHA string) (ArtifactIdentity, error) {
	identity := ArtifactIdentity{
		OriginType:  ArtifactOriginTypeHf,
		HFModelID:   strings.TrimSpace(modelID),
		HFCommitSHA: strings.ToLower(strings.TrimSpace(commitSHA)),
	}
	if !identity.isValid() {
		return ArtifactIdentity{}, fmt.Errorf("invalid Hugging Face artifact identity %q@%q", modelID, commitSHA)
	}
	return identity, nil
}

func (i ArtifactIdentity) isValid() bool {
	return strings.EqualFold(i.OriginType, ArtifactOriginTypeHf) &&
		isValidHfModelID(i.HFModelID) &&
		isValidHfCommitSHA(i.HFCommitSHA)
}

func (i ArtifactIdentity) toOrigin() *ArtifactOrigin {
	if !i.isValid() {
		return nil
	}
	return &ArtifactOrigin{
		Type:        ArtifactOriginTypeHf,
		HFModelID:   i.HFModelID,
		HFCommitSHA: strings.ToLower(i.HFCommitSHA),
	}
}

func isValidHfCommitSHA(sha string) bool {
	if len(sha) != 40 {
		return false
	}
	for _, character := range sha {
		if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
			return false
		}
	}
	return true
}

func isValidHfModelID(modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" || len(modelID) > 96 || strings.HasPrefix(modelID, "/") || strings.HasSuffix(modelID, "/") {
		return false
	}
	if strings.Contains(modelID, "\\") || strings.Contains(modelID, "//") || strings.Contains(modelID, "..") || strings.Contains(modelID, "--") || strings.HasSuffix(modelID, ".git") {
		return false
	}
	parts := strings.Split(modelID, "/")
	if len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !isValidHfModelIDSegment(part) {
			return false
		}
	}
	return true
}

func isValidHfModelIDSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." || len(segment) > 96 {
		return false
	}
	if strings.HasPrefix(segment, ".") || strings.HasPrefix(segment, "-") || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, "-") {
		return false
	}
	for _, character := range segment {
		valid := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.'
		if !valid {
			return false
		}
	}
	return true
}

func hfArtifactIdentityFromTask(task *GopherTask) (ArtifactIdentity, bool) {
	if task == nil {
		return ArtifactIdentity{}, false
	}
	if task.BaseModel != nil {
		return hfArtifactIdentityFromAnnotations(task.BaseModel.Annotations)
	}
	if task.ClusterBaseModel != nil {
		return hfArtifactIdentityFromAnnotations(task.ClusterBaseModel.Annotations)
	}
	return ArtifactIdentity{}, false
}

func hfArtifactIdentityFromAnnotations(annotations map[string]string) (ArtifactIdentity, bool) {
	identity, err := newHfArtifactIdentity(
		annotations[HfModelIDAnnotationKey],
		annotations[HfSHAAnnotationKey],
	)
	return identity, err == nil
}

func hfArtifactConfigMapKey(identity ArtifactIdentity) string {
	return constants.HfArtifactConfigMapKeyPrefix +
		sanitizeConfigMapKeyComponent(identity.HFModelID) + "." +
		shortConfigMapKeyHash(identity.HFModelID) + "." +
		strings.ToLower(identity.HFCommitSHA)
}

func isHfArtifactConfigMapKey(key string) bool {
	return strings.HasPrefix(key, constants.HfArtifactConfigMapKeyPrefix)
}

func shortConfigMapKeyHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])[:12]
}

func sanitizeConfigMapKeyComponent(value string) string {
	var result strings.Builder
	for _, character := range strings.TrimSpace(value) {
		valid := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.'
		if valid {
			result.WriteRune(character)
		} else {
			result.WriteByte('.')
		}
	}
	sanitized := strings.Trim(result.String(), ".")
	if sanitized == "" {
		return "unknown"
	}
	return sanitized
}

func canonicalHfArtifactPath(childPath string, identity ArtifactIdentity) string {
	return filepath.Join(
		filepath.Dir(childPath),
		constants.ModelArtifactsDirectory,
		filepath.FromSlash(identity.HFModelID),
		strings.ToLower(identity.HFCommitSHA),
	)
}

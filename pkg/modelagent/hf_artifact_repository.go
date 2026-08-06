package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/constants"
)

type ParentOutcome string

const (
	ParentAcquired ParentOutcome = "Acquired"
	ParentReady    ParentOutcome = "Ready"
	ParentBusy     ParentOutcome = "Busy"
)

type ArtifactParent struct {
	Key              string
	Path             string
	Identity         ArtifactIdentity
	Status           ModelStatus
	Children         []string
	ReservationToken string

	originPresent bool
	recordedSHA   string
}

type ParentReservation struct {
	Outcome ParentOutcome
	Parent  ArtifactParent
}

type ChildRelease struct {
	Parent       ArtifactParent
	Found        bool
	DeleteParent bool
}

type HfArtifactRepository struct {
	configMaps *ConfigMapReconciler
}

func newHfArtifactRepository(configMaps *ConfigMapReconciler) *HfArtifactRepository {
	return &HfArtifactRepository{configMaps: configMaps}
}

func (r *HfArtifactRepository) Get(ctx context.Context, identity ArtifactIdentity) (ArtifactParent, bool, error) {
	if !identity.isValid() {
		return ArtifactParent{}, false, fmt.Errorf("invalid Hugging Face artifact identity")
	}
	key := hfArtifactConfigMapKey(identity)
	exists, raw, err := r.configMaps.getDataEntryBasedOnModelKey(ctx, key)
	if err != nil || !exists {
		return ArtifactParent{}, false, err
	}
	parent, err := decodeArtifactParent(key, raw)
	if err != nil {
		return ArtifactParent{}, false, err
	}
	if !parent.originPresent || !sameArtifactIdentity(parent.Identity, identity) {
		return ArtifactParent{}, false, fmt.Errorf("synthetic parent %s has mismatched origin metadata", key)
	}
	if err := validateDesiredArtifactParent(parent); err != nil {
		return ArtifactParent{}, false, err
	}
	return parent, true, nil
}

func (r *HfArtifactRepository) ListByChildPath(ctx context.Context, childPath string) ([]ArtifactParent, error) {
	configMap, err := r.configMaps.getConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	parents := make([]ArtifactParent, 0)
	for key, raw := range configMap.Data {
		if !isHfArtifactConfigMapKey(key) {
			continue
		}
		parent, err := decodeArtifactParent(key, raw)
		if err != nil {
			return nil, err
		}
		if err := validateDesiredArtifactParent(parent); err != nil {
			return nil, err
		}
		if containsPath(parent.Children, childPath) {
			parents = append(parents, parent)
		}
	}
	return parents, nil
}

func (r *HfArtifactRepository) Reserve(ctx context.Context, desired ArtifactParent) (ParentReservation, error) {
	return r.claim(ctx, desired, false)
}

func (r *HfArtifactRepository) AcquireRepair(ctx context.Context, desired ArtifactParent) (ParentReservation, error) {
	return r.claim(ctx, desired, true)
}

// TakeOverUpdating establishes a fresh owner for startup recovery. It is only
// safe before workers start, after any owner from the previous process has
// stopped. This also upgrades Updating entries written before reservation
// tokens were introduced.
func (r *HfArtifactRepository) TakeOverUpdating(ctx context.Context, desired ArtifactParent) (ArtifactParent, error) {
	if err := validateDesiredArtifactParent(desired); err != nil {
		return ArtifactParent{}, err
	}
	reservationToken := uuid.NewString()
	var result ArtifactParent
	err := r.configMaps.mutateConfigMapWithRetry(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
		current, err := existingArtifactParent(configMap.Data, desired)
		if err != nil {
			return false, err
		}
		if current.Status != ModelStatusUpdating {
			return false, fmt.Errorf("synthetic parent %s is not Updating", current.Key)
		}
		current.ReservationToken = reservationToken
		result = current
		return writeArtifactParent(configMap.Data, current)
	})
	return result, err
}

func (r *HfArtifactRepository) claim(ctx context.Context, desired ArtifactParent, repair bool) (ParentReservation, error) {
	if err := validateDesiredArtifactParent(desired); err != nil {
		return ParentReservation{}, err
	}

	reservationToken := uuid.NewString()
	var result ParentReservation
	err := r.configMaps.mutateConfigMapWithRetry(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
		result = ParentReservation{}
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}

		raw, exists := configMap.Data[desired.Key]
		if !exists {
			parent := desired.withStatus(ModelStatusUpdating)
			parent.ReservationToken = reservationToken
			result = ParentReservation{Outcome: ParentAcquired, Parent: parent}
			return writeArtifactParent(configMap.Data, parent)
		}

		current, decodeErr := decodeArtifactParent(desired.Key, raw)
		if decodeErr != nil {
			return false, decodeErr
		}
		if err := validateRecordedProvenance(current, desired.Identity); err != nil {
			return false, err
		}
		if current.originPresent && validateDesiredArtifactParent(current) == nil {
			desired.Path = current.Path
		}

		if current.Status == ModelStatusUpdating {
			if current.Path != "" && current.Path != desired.Path {
				return false, fmt.Errorf("synthetic parent %s is Updating and uses noncanonical path %s", current.Key, current.Path)
			}
			current = applyDesiredArtifactParentMetadata(current, desired)
			result = ParentReservation{Outcome: ParentBusy, Parent: current}
			return writeArtifactParent(configMap.Data, current)
		}

		metadataComplete := current.originPresent && current.Path == desired.Path
		current = applyDesiredArtifactParentMetadata(current, desired)
		if current.Status == ModelStatusReady && metadataComplete && !repair {
			result = ParentReservation{Outcome: ParentReady, Parent: current}
			return false, nil
		}

		current.Status = ModelStatusUpdating
		current.ReservationToken = reservationToken
		result = ParentReservation{Outcome: ParentAcquired, Parent: current}
		return writeArtifactParent(configMap.Data, current)
	})
	return result, err
}

func (r *HfArtifactRepository) MarkReady(ctx context.Context, desired ArtifactParent) error {
	return r.setStatus(ctx, desired, ModelStatusReady)
}

func (r *HfArtifactRepository) MarkFailed(ctx context.Context, desired ArtifactParent) error {
	return r.setStatus(ctx, desired, ModelStatusFailed)
}

func (r *HfArtifactRepository) setStatus(ctx context.Context, desired ArtifactParent, status ModelStatus) error {
	if err := validateDesiredArtifactParent(desired); err != nil {
		return err
	}
	return r.configMaps.mutateConfigMapWithRetry(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
		current, err := existingArtifactParent(configMap.Data, desired)
		if err != nil {
			return false, err
		}
		if current.Status == status {
			return false, nil
		}
		if current.Status != ModelStatusUpdating {
			return false, fmt.Errorf("synthetic parent %s cannot transition from %s to %s", current.Key, current.Status, status)
		}
		if desired.ReservationToken == "" || current.ReservationToken != desired.ReservationToken {
			return false, fmt.Errorf("synthetic parent %s reservation token does not match current owner", current.Key)
		}
		current.Status = status
		current.ReservationToken = ""
		return writeArtifactParent(configMap.Data, current)
	})
}

func (r *HfArtifactRepository) AddChild(ctx context.Context, desired ArtifactParent, childPath string) error {
	if err := validateDesiredArtifactParent(desired); err != nil {
		return err
	}
	if strings.TrimSpace(childPath) == "" {
		return fmt.Errorf("child path is empty")
	}
	return r.configMaps.mutateConfigMapWithRetry(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
		current, err := existingArtifactParent(configMap.Data, desired)
		if err != nil {
			return false, err
		}
		if current.Status != ModelStatusReady {
			return false, fmt.Errorf("Hugging Face artifact parent %s is %s", current.Key, current.Status)
		}
		if containsPath(current.Children, childPath) {
			return false, nil
		}
		current.Children = append(current.Children, childPath)
		return writeArtifactParent(configMap.Data, current)
	})
}

// AddChildWithArtifact records the parent reference and child provenance in one ConfigMap update.
func (r *HfArtifactRepository) AddChildWithArtifact(
	ctx context.Context,
	desired ArtifactParent,
	child ArtifactChild,
	artifact Artifact,
) error {
	if err := validateDesiredArtifactParent(desired); err != nil {
		return err
	}
	if strings.TrimSpace(child.Key) == "" || strings.TrimSpace(child.Name) == "" || strings.TrimSpace(child.Path) == "" {
		return fmt.Errorf("shared artifact child metadata is incomplete")
	}
	err := r.configMaps.mutateModelEntryWithRetry(ctx, child.Key, child.UID, false, func(data map[string]string) (bool, error) {
		current, err := existingArtifactParent(data, desired)
		if err != nil {
			return false, err
		}
		if current.Status != ModelStatusReady {
			return false, fmt.Errorf("Hugging Face artifact parent %s is %s", current.Key, current.Status)
		}
		if !containsPath(current.Children, child.Path) {
			current.Children = append(current.Children, child.Path)
		}
		if _, err := writeArtifactParent(data, current); err != nil {
			return false, err
		}

		entry := ModelEntry{Name: child.Name, Status: ModelStatusUpdating}
		if raw, exists := data[child.Key]; exists {
			if err := json.Unmarshal([]byte(raw), &entry); err != nil {
				return false, fmt.Errorf("decode child model entry %s: %w", child.Key, err)
			}
		}
		if entry.Config == nil {
			entry.Config = &ModelConfig{}
		}
		entry.Config.Artifact = cloneArtifact(artifact)
		encoded, err := json.Marshal(entry)
		if err != nil {
			return false, err
		}
		data[child.Key] = string(encoded)
		return true, nil
	})
	if err != nil {
		return err
	}
	if r.configMaps.isModelMutationBlocked(child.Key, child.UID) {
		return fmt.Errorf("shared artifact child %s is no longer active", child.Key)
	}
	r.configMaps.cacheModelArtifact(child.Key, child.Name, child.UID, artifact)
	return nil
}

// RemoveChild removes a child reference and claims an empty parent by moving it
// to Updating. While claimed, AddChild and Reserve cannot race with filesystem
// cleanup. DeleteIfUnreferenced performs the final guarded ConfigMap removal.
func (r *HfArtifactRepository) RemoveChild(ctx context.Context, desired ArtifactParent, childPath string) (ChildRelease, error) {
	if err := validateDesiredArtifactParent(desired); err != nil {
		return ChildRelease{}, err
	}
	var result ChildRelease
	err := r.configMaps.mutateConfigMapWithRetry(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
		result = ChildRelease{}
		if configMap.Data == nil {
			return false, nil
		}
		if _, exists := configMap.Data[desired.Key]; !exists {
			return false, nil
		}

		current, err := existingArtifactParent(configMap.Data, desired)
		if err != nil {
			return false, err
		}
		if current.Status == ModelStatusUpdating {
			return false, fmt.Errorf("Hugging Face artifact parent %s is Updating", current.Key)
		}
		current.Children = removePath(current.Children, childPath)
		result = ChildRelease{Parent: current, Found: true}
		if len(current.Children) == 0 {
			current.Status = ModelStatusUpdating
			current.ReservationToken = uuid.NewString()
			result.Parent = current
			result.DeleteParent = true
		}
		return writeArtifactParent(configMap.Data, current)
	})
	return result, err
}

func (r *HfArtifactRepository) DeleteIfUnreferenced(ctx context.Context, desired ArtifactParent) (bool, error) {
	if err := validateDesiredArtifactParent(desired); err != nil {
		return false, err
	}
	deleted := false
	err := r.configMaps.mutateConfigMapWithRetry(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
		deleted = false
		if configMap.Data == nil {
			return false, nil
		}
		if _, exists := configMap.Data[desired.Key]; !exists {
			return false, nil
		}
		current, err := existingArtifactParent(configMap.Data, desired)
		if err != nil {
			return false, err
		}
		if len(current.Children) != 0 || current.Status != ModelStatusUpdating {
			return false, nil
		}
		if desired.ReservationToken == "" || current.ReservationToken != desired.ReservationToken {
			return false, fmt.Errorf("synthetic parent %s reservation token does not match current owner", current.Key)
		}
		delete(configMap.Data, desired.Key)
		deleted = true
		return true, nil
	})
	return deleted, err
}

func existingArtifactParent(data map[string]string, desired ArtifactParent) (ArtifactParent, error) {
	if data == nil {
		return ArtifactParent{}, fmt.Errorf("synthetic parent %s does not exist", desired.Key)
	}
	raw, exists := data[desired.Key]
	if !exists {
		return ArtifactParent{}, fmt.Errorf("synthetic parent %s does not exist", desired.Key)
	}
	current, err := decodeArtifactParent(desired.Key, raw)
	if err != nil {
		return ArtifactParent{}, err
	}
	if err := validateRecordedProvenance(current, desired.Identity); err != nil {
		return ArtifactParent{}, err
	}
	if !current.originPresent || current.Path != desired.Path {
		return ArtifactParent{}, fmt.Errorf("synthetic parent %s has incomplete canonical metadata", desired.Key)
	}
	return current, nil
}

func validateDesiredArtifactParent(parent ArtifactParent) error {
	if !parent.Identity.isValid() {
		return fmt.Errorf("invalid Hugging Face artifact identity")
	}
	expectedKey := hfArtifactConfigMapKey(parent.Identity)
	if parent.Key != expectedKey {
		return fmt.Errorf("synthetic parent key %s does not match identity; expected %s", parent.Key, expectedKey)
	}
	if strings.TrimSpace(parent.Path) == "" {
		return fmt.Errorf("synthetic parent path is empty")
	}
	expectedSuffix := filepath.Join(
		constants.ModelArtifactsDirectory,
		filepath.FromSlash(parent.Identity.HFModelID),
		parent.Identity.HFCommitSHA,
	)
	cleanPath := filepath.Clean(parent.Path)
	if cleanPath != expectedSuffix && !strings.HasSuffix(cleanPath, string(filepath.Separator)+expectedSuffix) {
		return fmt.Errorf("synthetic parent path %s is not canonical for identity %s@%s", parent.Path, parent.Identity.HFModelID, parent.Identity.HFCommitSHA)
	}
	return nil
}

func validateRecordedProvenance(parent ArtifactParent, desired ArtifactIdentity) error {
	if parent.originPresent && !sameArtifactIdentity(parent.Identity, desired) {
		return fmt.Errorf("synthetic parent %s has invalid provenance: mismatched origin", parent.Key)
	}
	if parent.recordedSHA != "" && !strings.EqualFold(parent.recordedSHA, desired.HFCommitSHA) {
		return fmt.Errorf("synthetic parent %s has invalid provenance: sha conflicts with origin", parent.Key)
	}
	return nil
}

func applyDesiredArtifactParentMetadata(current, desired ArtifactParent) ArtifactParent {
	current.Key = desired.Key
	current.Path = desired.Path
	current.Identity = desired.Identity
	current.originPresent = true
	current.recordedSHA = desired.Identity.HFCommitSHA
	if current.Children == nil {
		current.Children = []string{}
	}
	return current
}

func decodeArtifactParent(key, raw string) (ArtifactParent, error) {
	var entry ModelEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return ArtifactParent{}, fmt.Errorf("cannot decode synthetic parent %s: %w", key, err)
	}
	parent := ArtifactParent{Key: key, Status: entry.Status}
	if entry.Config == nil {
		return parent, nil
	}
	artifact := entry.Config.Artifact
	parent.Path = artifact.ParentPath[key]
	parent.Children = append([]string(nil), artifact.ChildrenPaths...)
	parent.recordedSHA = artifact.Sha
	parent.ReservationToken = artifact.ReservationToken
	if artifact.Origin == nil {
		return parent, nil
	}
	if !strings.EqualFold(artifact.Origin.Type, ArtifactOriginTypeHf) {
		return ArtifactParent{}, fmt.Errorf("synthetic parent %s has invalid provenance: unsupported origin type %q", key, artifact.Origin.Type)
	}
	identity, err := newHfArtifactIdentity(artifact.Origin.HFModelID, artifact.Origin.HFCommitSHA)
	if err != nil {
		return ArtifactParent{}, fmt.Errorf("synthetic parent %s has invalid provenance: %w", key, err)
	}
	parent.Identity = identity
	parent.originPresent = true
	if artifact.Sha != "" && !strings.EqualFold(artifact.Sha, identity.HFCommitSHA) {
		return ArtifactParent{}, fmt.Errorf("synthetic parent %s has invalid provenance: sha conflicts with origin", key)
	}
	return parent, nil
}

func writeArtifactParent(data map[string]string, parent ArtifactParent) (bool, error) {
	entry := modelEntryFromArtifactParent(parent)
	encoded, err := json.Marshal(entry)
	if err != nil {
		return false, err
	}
	next := string(encoded)
	if data[parent.Key] == next {
		return false, nil
	}
	data[parent.Key] = next
	return true, nil
}

func modelEntryFromArtifactParent(parent ArtifactParent) ModelEntry {
	return ModelEntry{
		Name:   parent.Key,
		Status: parent.Status,
		Config: &ModelConfig{Artifact: Artifact{
			Sha:              parent.Identity.HFCommitSHA,
			ReservationToken: parent.ReservationToken,
			Origin:           parent.Identity.toOrigin(),
			ParentPath:       map[string]string{parent.Key: parent.Path},
			ChildrenPaths:    append([]string(nil), parent.Children...),
		}},
	}
}

func (p ArtifactParent) withStatus(status ModelStatus) ArtifactParent {
	p.Status = status
	p.originPresent = p.Identity.isValid()
	p.recordedSHA = p.Identity.HFCommitSHA
	if p.Children == nil {
		p.Children = []string{}
	}
	return p
}

func sameArtifactIdentity(left, right ArtifactIdentity) bool {
	return strings.EqualFold(left.OriginType, right.OriginType) &&
		left.HFModelID == right.HFModelID &&
		strings.EqualFold(left.HFCommitSHA, right.HFCommitSHA)
}

func containsPath(values []string, target string) bool {
	for _, value := range values {
		if filepath.Clean(value) == filepath.Clean(target) {
			return true
		}
	}
	return false
}

func removePath(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if filepath.Clean(value) != filepath.Clean(target) {
			result = append(result, value)
		}
	}
	return result
}

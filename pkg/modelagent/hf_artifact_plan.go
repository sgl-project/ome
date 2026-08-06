package modelagent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

type ArtifactOperation string

const (
	ArtifactOperationEnsure  ArtifactOperation = "Ensure"
	ArtifactOperationRepair  ArtifactOperation = "Repair"
	ArtifactOperationRelease ArtifactOperation = "Release"
)

type ArtifactPlan struct {
	Operation  ArtifactOperation
	Parent     ArtifactParent
	Child      ArtifactChild
	ChildPath  string
	SearchRoot string
}

type ArtifactChild struct {
	Key  string
	Name string
	UID  types.UID
	Path string
}

func planHfOriginOCIArtifact(task *GopherTask, spec v1beta1.BaseModelSpec, modelRootDir, childPath string) (ArtifactPlan, bool) {
	if task == nil || spec.Storage == nil || spec.Storage.DownloadPolicy == nil ||
		*spec.Storage.DownloadPolicy != v1beta1.ReuseIfExists {
		return ArtifactPlan{}, false
	}
	if spec.Storage.StorageUri == nil {
		return ArtifactPlan{}, false
	}
	storageType, err := storage.GetStorageType(*spec.Storage.StorageUri)
	if err != nil || storageType != storage.StorageTypeOCI {
		return ArtifactPlan{}, false
	}
	identity, ok := hfArtifactIdentityFromTask(task)
	if !ok {
		return ArtifactPlan{}, false
	}
	if !objectStoragePrefixMatchesArtifactIdentity(*spec.Storage.StorageUri, identity) {
		return ArtifactPlan{}, false
	}
	operation := ArtifactOperationEnsure
	if task.TaskType == DownloadOverride {
		operation = ArtifactOperationRepair
	} else if task.TaskType != Download {
		return ArtifactPlan{}, false
	}
	return ArtifactPlan{
		Operation:  operation,
		Parent:     artifactParentForTask(task, modelRootDir, childPath, identity),
		Child:      artifactChildForTask(task, childPath),
		ChildPath:  childPath,
		SearchRoot: artifactSearchRoot(modelRootDir, childPath),
	}, true
}

func objectStoragePrefixMatchesArtifactIdentity(storageURI string, identity ArtifactIdentity) bool {
	objectURI, err := storage.NewObjectURI(storageURI)
	if err != nil {
		return false
	}
	prefix := strings.Trim(objectURI.Prefix, "/")
	lastSeparator := strings.LastIndex(prefix, "/")
	if lastSeparator < 0 || !strings.EqualFold(prefix[lastSeparator+1:], identity.HFCommitSHA) {
		return false
	}
	modelPrefix := prefix[:lastSeparator]
	return modelPrefix == identity.HFModelID || strings.HasSuffix(modelPrefix, "/"+identity.HFModelID)
}

func artifactChildForTask(task *GopherTask, childPath string) ArtifactChild {
	child := ArtifactChild{Path: childPath}
	if task == nil {
		return child
	}
	if task.BaseModel != nil {
		child.Key = constants.GetModelConfigMapKey(task.BaseModel.Namespace, task.BaseModel.Name, false)
		child.Name = task.BaseModel.Name
		child.UID = task.BaseModel.UID
	} else if task.ClusterBaseModel != nil {
		child.Key = constants.GetModelConfigMapKey("", task.ClusterBaseModel.Name, true)
		child.Name = task.ClusterBaseModel.Name
		child.UID = task.ClusterBaseModel.UID
	}
	return child
}

func artifactSearchRoot(modelRootDir, childPath string) string {
	if root := strings.TrimSpace(modelRootDir); root != "" {
		return root
	}
	return filepath.Dir(childPath)
}

func artifactParentForTask(task *GopherTask, modelRootDir, childPath string, identity ArtifactIdentity) ArtifactParent {
	if task != nil && task.ClusterBaseModel != nil {
		root := strings.TrimSpace(modelRootDir)
		if root == "" {
			root = filepath.Dir(childPath)
		}
		return ArtifactParent{
			Key:      hfArtifactConfigMapKey(identity),
			Path:     filepath.Join(root, constants.ModelArtifactsDirectory, filepath.FromSlash(identity.HFModelID), identity.HFCommitSHA),
			Identity: identity,
		}
	}
	return artifactParentForChild(identity, childPath)
}

func artifactParentForChild(identity ArtifactIdentity, childPath string) ArtifactParent {
	return ArtifactParent{
		Key:      hfArtifactConfigMapKey(identity),
		Path:     canonicalHfArtifactPath(childPath, identity),
		Identity: identity,
	}
}

// loadHfArtifactReleasePlan uses the persisted child entry instead of
// current CR annotations. That keeps shared artifacts deletable after the
// control-plane feature is disabled or during a mixed-version rollout.
func loadHfArtifactReleasePlan(
	ctx context.Context,
	configMaps *ConfigMapReconciler,
	modelKey string,
	childPath string,
	searchRoot string,
) (ArtifactPlan, bool, error) {
	if configMaps == nil || strings.TrimSpace(modelKey) == "" {
		return ArtifactPlan{}, false, nil
	}
	exists, raw, err := configMaps.getDataEntryBasedOnModelKey(ctx, modelKey)
	if err != nil || !exists {
		return ArtifactPlan{}, false, err
	}
	var entry ModelEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return ArtifactPlan{}, false, fmt.Errorf("cannot decode model artifact metadata for %s: %w", modelKey, err)
	}
	if entry.Config == nil || entry.Config.Artifact.Origin == nil {
		return ArtifactPlan{}, false, nil
	}
	origin := entry.Config.Artifact.Origin
	identity, err := newHfArtifactIdentity(origin.HFModelID, origin.HFCommitSHA)
	if err != nil || !strings.EqualFold(origin.Type, ArtifactOriginTypeHf) {
		return ArtifactPlan{}, false, fmt.Errorf("model %s has invalid Hugging Face artifact provenance", modelKey)
	}
	key := hfArtifactConfigMapKey(identity)
	recordedPath, exists := entry.Config.Artifact.ParentPath[key]
	parent := ArtifactParent{Key: key, Path: recordedPath, Identity: identity}
	if !exists || validateDesiredArtifactParent(parent) != nil {
		return ArtifactPlan{}, false, fmt.Errorf("model %s has noncanonical Hugging Face artifact parent metadata", modelKey)
	}
	return ArtifactPlan{
		Operation:  ArtifactOperationRelease,
		Parent:     parent,
		Child:      ArtifactChild{Key: modelKey, Path: childPath},
		ChildPath:  childPath,
		SearchRoot: artifactSearchRoot(searchRoot, childPath),
	}, true, nil
}

package modelagent

import (
	"context"
	"strings"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

type hfRevisionResolver func(context.Context, string, string, string, string) (string, error)

func resolveDirectHfIdentity(
	ctx context.Context,
	modelID string,
	revision string,
	token string,
	endpoint string,
	resolve hfRevisionResolver,
) (ArtifactIdentity, bool) {
	revision = strings.TrimSpace(revision)
	if isValidHfCommitSHA(revision) {
		identity, err := newHfArtifactIdentity(modelID, revision)
		return identity, err == nil
	}
	sha, err := resolve(ctx, modelID, revision, token, endpoint)
	if err != nil {
		return ArtifactIdentity{}, false
	}
	identity, err := newHfArtifactIdentity(modelID, sha)
	return identity, err == nil
}

func planDirectHfArtifact(
	task *GopherTask,
	spec v1beta1.BaseModelSpec,
	modelRootDir string,
	childPath string,
	identity ArtifactIdentity,
) (ArtifactPlan, bool) {
	if task == nil || !identity.isValid() || spec.Storage == nil || spec.Storage.StorageUri == nil ||
		spec.Storage.DownloadPolicy == nil || *spec.Storage.DownloadPolicy != v1beta1.ReuseIfExists {
		return ArtifactPlan{}, false
	}
	storageType, err := storage.GetStorageType(*spec.Storage.StorageUri)
	if err != nil || storageType != storage.StorageTypeHuggingFace {
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

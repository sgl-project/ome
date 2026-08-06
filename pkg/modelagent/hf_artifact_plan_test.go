package modelagent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestPlanHfOriginOCIArtifactRequiresCompleteGate(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "model-1")
	model := newHfOriginOCIModel("model-1", childPath)

	plan, ok := planHfOriginOCIArtifact(&GopherTask{TaskType: Download, BaseModel: model}, model.Spec, filepath.Dir(childPath), childPath)
	require.True(t, ok)
	assert.Equal(t, ArtifactOperationEnsure, plan.Operation)
	assert.Equal(t, childPath, plan.ChildPath)
	assert.Equal(t, filepath.Dir(childPath), plan.SearchRoot)
	assert.Equal(t, canonicalHfArtifactPath(childPath, plan.Parent.Identity), plan.Parent.Path)

	withoutPolicy := model.DeepCopy()
	withoutPolicy.Spec.Storage.DownloadPolicy = nil
	_, ok = planHfOriginOCIArtifact(&GopherTask{TaskType: Download, BaseModel: withoutPolicy}, withoutPolicy.Spec, filepath.Dir(childPath), childPath)
	assert.False(t, ok)

	withoutSHA := model.DeepCopy()
	delete(withoutSHA.Annotations, HfSHAAnnotationKey)
	_, ok = planHfOriginOCIArtifact(&GopherTask{TaskType: Download, BaseModel: withoutSHA}, withoutSHA.Spec, filepath.Dir(childPath), childPath)
	assert.False(t, ok)
}

func TestPlanHfOriginOCIArtifactRepairsOnDownloadOverride(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "model-1")
	model := newHfOriginOCIModel("model-1", childPath)

	plan, ok := planHfOriginOCIArtifact(&GopherTask{TaskType: DownloadOverride, BaseModel: model}, model.Spec, filepath.Dir(childPath), childPath)

	require.True(t, ok)
	assert.Equal(t, ArtifactOperationRepair, plan.Operation)
}

func TestPlanHfOriginOCIArtifactRejectsMismatchedObjectStoragePrefix(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "model-1")
	tests := map[string]string{
		"model id":             "oci://n/ns/b/models/o/customer-imported-basemodels/Other/Model/" + testHFCommitSHA,
		"commit":               "oci://n/ns/b/models/o/customer-imported-basemodels/Qwen/Qwen3-8B/" + strings.Repeat("a", 40),
		"noncanonical segment": "oci://n/ns/b/models/o/customer-imported-basemodels/Qwen/Qwen3-8B/ignored/../" + testHFCommitSHA,
	}
	for name, storageURI := range tests {
		t.Run(name, func(t *testing.T) {
			model := newHfOriginOCIModel("model-1", childPath)
			model.Spec.Storage.StorageUri = &storageURI

			_, ok := planHfOriginOCIArtifact(
				&GopherTask{TaskType: Download, BaseModel: model}, model.Spec, filepath.Dir(childPath), childPath,
			)

			assert.False(t, ok)
		})
	}
}

func TestArtifactParentForTaskUsesModelRootForClusterBaseModel(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "openai", "model-1")
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	task := &GopherTask{ClusterBaseModel: &v1beta1.ClusterBaseModel{}}

	parent := artifactParentForTask(task, root, childPath, identity)

	assert.Equal(t, filepath.Join(root, "_artifacts", "Qwen", "Qwen3-8B", testHFCommitSHA), parent.Path)
}

func TestLoadHfArtifactReleasePlanUsesRecordedChildMetadata(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	identity, ok := hfArtifactIdentityFromTask(&GopherTask{BaseModel: model})
	require.True(t, ok)
	parent := artifactParentForChild(identity, childPath)
	modelKey := "default.basemodel.model-1"
	entry := ModelEntry{
		Name:   model.Name,
		Status: ModelStatusReady,
		Config: &ModelConfig{Artifact: Artifact{
			Sha:           identity.HFCommitSHA,
			Origin:        identity.toOrigin(),
			ParentPath:    map[string]string{parent.Key: parent.Path},
			ChildrenPaths: []string{},
		}},
	}
	raw, err := json.Marshal(entry)
	require.NoError(t, err)
	repository, _ := newTestHfArtifactRepository(t, map[string]string{modelKey: string(raw)})

	plan, found, err := loadHfArtifactReleasePlan(context.Background(), repository.configMaps, modelKey, childPath, filepath.Dir(childPath))

	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ArtifactOperationRelease, plan.Operation)
	assert.Equal(t, parent.Key, plan.Parent.Key)
	assert.Equal(t, parent.Path, plan.Parent.Path)
}

func newHfOriginOCIModel(name, childPath string) *v1beta1.BaseModel {
	policy := v1beta1.ReuseIfExists
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Annotations: map[string]string{
				HfModelIDAnnotationKey: "Qwen/Qwen3-8B",
				HfSHAAnnotationKey:     testHFCommitSHA,
			},
		},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri:     stringPtr("oci://n/ns/b/models/o/Qwen/Qwen3-8B/" + testHFCommitSHA),
			Path:           &childPath,
			DownloadPolicy: &policy,
		}},
	}
}

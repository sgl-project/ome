package modelagent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestHfArtifactRepository(t *testing.T, data map[string]string) (*HfArtifactRepository, *fake.Clientset) {
	t.Helper()
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1", Namespace: "ome"},
		Data:       data,
	}
	client := fake.NewSimpleClientset(configMap)
	reconciler := NewConfigMapReconciler(configMap.Name, configMap.Namespace, client, zaptest.NewLogger(t).Sugar())
	return newHfArtifactRepository(reconciler), client
}

func testArtifactParent(t *testing.T) ArtifactParent {
	t.Helper()
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	return ArtifactParent{
		Key:      hfArtifactConfigMapKey(identity),
		Path:     "/mnt/data/models/customer-model-store/_artifacts/Qwen/Qwen3-8B/" + testHFCommitSHA,
		Identity: identity,
	}
}

func TestHfArtifactRepositoryReserveLifecycle(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)

	first, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	assert.Equal(t, ParentAcquired, first.Outcome)
	assert.Equal(t, ModelStatusUpdating, first.Parent.Status)
	assert.NotEmpty(t, first.Parent.ReservationToken)

	second, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	assert.Equal(t, ParentBusy, second.Outcome)

	require.NoError(t, repository.MarkReady(context.Background(), first.Parent))
	require.NoError(t, repository.AddChild(context.Background(), parent, "/mnt/data/models/customer-model-store/model-1"))
	require.NoError(t, repository.AddChild(context.Background(), parent, "/mnt/data/models/customer-model-store/model-1"))

	ready, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	assert.Equal(t, ParentReady, ready.Outcome)
	assert.Equal(t, []string{"/mnt/data/models/customer-model-store/model-1"}, ready.Parent.Children)
}

func TestHfArtifactRepositoryAcquiresFailedParentForRepair(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)

	reservation, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	require.NoError(t, repository.MarkFailed(context.Background(), reservation.Parent))

	result, err := repository.AcquireRepair(context.Background(), parent)
	require.NoError(t, err)
	assert.Equal(t, ParentAcquired, result.Outcome)
	assert.Equal(t, ModelStatusUpdating, result.Parent.Status)
}

func TestHfArtifactRepositoryFencesStaleReservationOwner(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)

	first, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	require.NoError(t, repository.MarkFailed(context.Background(), first.Parent))
	second, err := repository.AcquireRepair(context.Background(), parent)
	require.NoError(t, err)
	require.NotEqual(t, first.Parent.ReservationToken, second.Parent.ReservationToken)

	err = repository.MarkReady(context.Background(), first.Parent)
	assert.ErrorContains(t, err, "reservation token")
	require.NoError(t, repository.MarkReady(context.Background(), second.Parent))
}

func TestHfArtifactRepositoryPreservesUpdatingOwnershipWhenRepairingMetadata(t *testing.T) {
	parent := testArtifactParent(t)
	invalidEntry, err := json.Marshal(ModelEntry{
		Name:   parent.Key,
		Status: ModelStatusUpdating,
		Config: &ModelConfig{Artifact: Artifact{
			ParentPath:    map[string]string{parent.Key: parent.Path},
			ChildrenPaths: []string{"/models/existing-child"},
		}},
	})
	require.NoError(t, err)
	repository, _ := newTestHfArtifactRepository(t, map[string]string{parent.Key: string(invalidEntry)})

	result, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	assert.Equal(t, ParentBusy, result.Outcome)
	assert.Equal(t, []string{"/models/existing-child"}, result.Parent.Children)
	assert.Equal(t, parent.Identity, result.Parent.Identity)
}

func TestHfArtifactRepositoryRejectsConflictingProvenance(t *testing.T) {
	parent := testArtifactParent(t)
	tests := []struct {
		name   string
		origin *ArtifactOrigin
		sha    string
	}{
		{
			name: "wrong origin type",
			origin: &ArtifactOrigin{
				Type:        "oci",
				HFModelID:   parent.Identity.HFModelID,
				HFCommitSHA: parent.Identity.HFCommitSHA,
			},
			sha: parent.Identity.HFCommitSHA,
		},
		{
			name: "malformed origin",
			origin: &ArtifactOrigin{
				Type:        ArtifactOriginTypeHf,
				HFModelID:   "../invalid",
				HFCommitSHA: parent.Identity.HFCommitSHA,
			},
			sha: parent.Identity.HFCommitSHA,
		},
		{
			name: "sha conflicts with origin",
			origin: &ArtifactOrigin{
				Type:        ArtifactOriginTypeHf,
				HFModelID:   parent.Identity.HFModelID,
				HFCommitSHA: parent.Identity.HFCommitSHA,
			},
			sha: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := json.Marshal(ModelEntry{
				Name:   parent.Key,
				Status: ModelStatusReady,
				Config: &ModelConfig{Artifact: Artifact{
					Sha:           tt.sha,
					Origin:        tt.origin,
					ParentPath:    map[string]string{parent.Key: parent.Path},
					ChildrenPaths: []string{},
				}},
			})
			require.NoError(t, err)
			repository, _ := newTestHfArtifactRepository(t, map[string]string{parent.Key: string(entry)})

			_, err = repository.Reserve(context.Background(), parent)
			assert.ErrorContains(t, err, "invalid provenance")
		})
	}
}

func TestHfArtifactRepositoryRebuildsReadyParentAtWrongPath(t *testing.T) {
	parent := testArtifactParent(t)
	wrongPath := parent
	wrongPath.Path = "/models/old-model-owned-parent"
	encoded, err := json.Marshal(modelEntryFromArtifactParent(wrongPath.withStatus(ModelStatusReady)))
	require.NoError(t, err)
	repository, _ := newTestHfArtifactRepository(t, map[string]string{parent.Key: string(encoded)})

	result, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	assert.Equal(t, ParentAcquired, result.Outcome)
	assert.Equal(t, parent.Path, result.Parent.Path)
	assert.Equal(t, ModelStatusUpdating, result.Parent.Status)
}

func TestHfArtifactRepositoryDoesNotMoveUpdatingParent(t *testing.T) {
	parent := testArtifactParent(t)
	updating := parent
	updating.Path = "/models/active-download"
	encoded, err := json.Marshal(modelEntryFromArtifactParent(updating.withStatus(ModelStatusUpdating)))
	require.NoError(t, err)
	repository, client := newTestHfArtifactRepository(t, map[string]string{parent.Key: string(encoded)})

	_, err = repository.Reserve(context.Background(), parent)
	assert.ErrorContains(t, err, "Updating and uses noncanonical path")

	configMap, err := client.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	stored, err := decodeArtifactParent(parent.Key, configMap.Data[parent.Key])
	require.NoError(t, err)
	assert.Equal(t, updating.Path, stored.Path)
}

func TestHfArtifactRepositoryRejectsMismatchedIdentity(t *testing.T) {
	parent := testArtifactParent(t)
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	_, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)

	otherIdentity, err := newHfArtifactIdentity("Qwen/Other", testHFCommitSHA)
	require.NoError(t, err)
	parent.Identity = otherIdentity

	_, err = repository.Reserve(context.Background(), parent)
	assert.ErrorContains(t, err, "does not match identity")
}

func TestHfArtifactRepositoryRejectsInvalidDesiredParent(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)
	parent.Key = "artifact.huggingface.wrong"

	_, err := repository.Reserve(context.Background(), parent)
	assert.ErrorContains(t, err, "does not match identity")

	parent = testArtifactParent(t)
	parent.Path = ""
	_, err = repository.Reserve(context.Background(), parent)
	assert.ErrorContains(t, err, "path is empty")

	parent = testArtifactParent(t)
	parent.Path = "/models/not_artifacts/Qwen/Qwen3-8B/" + testHFCommitSHA
	_, err = repository.Reserve(context.Background(), parent)
	assert.ErrorContains(t, err, "is not canonical")

	configMap, err := client.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, configMap.Data)
}

func TestHfArtifactRepositoryRemovesChildAndParent(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)
	reservation, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	require.NoError(t, repository.MarkReady(context.Background(), reservation.Parent))
	require.NoError(t, repository.AddChild(context.Background(), parent, "/models/child"))

	release, err := repository.RemoveChild(context.Background(), parent, "/models/child")
	require.NoError(t, err)
	assert.True(t, release.Found)
	assert.True(t, release.DeleteParent)
	assert.Empty(t, release.Parent.Children)
	assert.Equal(t, ModelStatusUpdating, release.Parent.Status)

	err = repository.AddChild(context.Background(), parent, "/models/concurrent-child")
	assert.ErrorContains(t, err, "Updating")

	deleted, err := repository.DeleteIfUnreferenced(context.Background(), parent)
	assert.ErrorContains(t, err, "reservation token")
	assert.False(t, deleted)

	deleted, err = repository.DeleteIfUnreferenced(context.Background(), release.Parent)
	require.NoError(t, err)
	assert.True(t, deleted)

	configMap, err := client.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	_, exists := configMap.Data[parent.Key]
	assert.False(t, exists)
}

func TestHfArtifactRepositoryKeepsParentWithRemainingChild(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)
	reservation, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)
	require.NoError(t, repository.MarkReady(context.Background(), reservation.Parent))
	require.NoError(t, repository.AddChild(context.Background(), parent, "/models/child-1"))
	require.NoError(t, repository.AddChild(context.Background(), parent, "/models/child-2"))

	release, err := repository.RemoveChild(context.Background(), parent, "/models/child-1")
	require.NoError(t, err)
	assert.False(t, release.DeleteParent)
	assert.Equal(t, ModelStatusReady, release.Parent.Status)
	assert.Equal(t, []string{"/models/child-2"}, release.Parent.Children)

	deleted, err := repository.DeleteIfUnreferenced(context.Background(), parent)
	require.NoError(t, err)
	assert.False(t, deleted)
}

func TestHfArtifactRepositoryMissingParentMutationsDoNotCreateEntry(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)

	assert.Error(t, repository.MarkReady(context.Background(), parent))
	assert.Error(t, repository.MarkFailed(context.Background(), parent))
	assert.Error(t, repository.AddChild(context.Background(), parent, "/models/child"))
	release, err := repository.RemoveChild(context.Background(), parent, "/models/child")
	require.NoError(t, err)
	assert.False(t, release.Found)

	configMap, err := client.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, configMap.Data)
}

func TestHfArtifactRepositoryDoesNotStealUpdatingParentForDeletion(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	parent := testArtifactParent(t)
	reservation, err := repository.Reserve(context.Background(), parent)
	require.NoError(t, err)

	_, err = repository.RemoveChild(context.Background(), parent, "/models/child")
	assert.ErrorContains(t, err, "is Updating")

	current, found, err := repository.Get(context.Background(), parent.Identity)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, reservation.Parent.ReservationToken, current.ReservationToken)
}

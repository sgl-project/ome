package modelagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/xet"
)

func TestResolveDirectHfIdentityUsesCommitWithoutAPI(t *testing.T) {
	called := false
	resolve := func(context.Context, string, string, string, string) (string, error) {
		called = true
		return "", nil
	}

	identity, ok := resolveDirectHfIdentity(
		context.Background(), "Qwen/Qwen3-8B", testHFCommitSHA, "", "https://hf.example", resolve,
	)

	require.True(t, ok)
	assert.Equal(t, testHFCommitSHA, identity.HFCommitSHA)
	assert.False(t, called)
}

func TestResolveDirectHfIdentityResolvesMovingRevision(t *testing.T) {
	resolve := func(_ context.Context, modelID, revision, token, endpoint string) (string, error) {
		assert.Equal(t, "Qwen/Qwen3-8B", modelID)
		assert.Equal(t, "feature/branch", revision)
		assert.Equal(t, "hf-token", token)
		assert.Equal(t, "https://hf.example", endpoint)
		return testHFCommitSHA, nil
	}

	identity, ok := resolveDirectHfIdentity(
		context.Background(), "Qwen/Qwen3-8B", "feature/branch", "hf-token", "https://hf.example", resolve,
	)

	require.True(t, ok)
	assert.Equal(t, testHFCommitSHA, identity.HFCommitSHA)
}

func TestDownloadHfSnapshotPinsResolvedSHA(t *testing.T) {
	original := snapshotDownloadWithProgress
	defer func() { snapshotDownloadWithProgress = original }()
	var observedRevision string
	var observedToken string
	snapshotDownloadWithProgress = func(
		_ context.Context,
		config *xet.DownloadConfig,
		_ xet.ProgressHandler,
		_ time.Duration,
	) (string, error) {
		observedRevision = config.Revision
		observedToken = config.Token
		return config.LocalDir, nil
	}
	gopher := &Gopher{xetConfig: &xet.Config{Token: "configured-token"}, logger: zap.NewNop().Sugar()}
	task := &GopherTask{BaseModel: &v1beta1.BaseModel{}}

	err := gopher.downloadHfSnapshot(
		context.Background(), task, "Qwen/Qwen3-8B", testHFCommitSHA, "", t.TempDir(),
		"model", "BaseModel", "default", "model",
	)

	require.NoError(t, err)
	assert.Equal(t, testHFCommitSHA, observedRevision)
	assert.Equal(t, "configured-token", observedToken)
}

func TestEffectiveHfTokenFallsBackToConfiguredToken(t *testing.T) {
	config := &xet.Config{Token: "configured-token"}

	assert.Equal(t, "model-token", effectiveHfToken(" model-token ", config))
	assert.Equal(t, "configured-token", effectiveHfToken("", config))
	assert.Empty(t, effectiveHfToken("", nil))
}

func TestHfDownloadRevisionUsesResolvedSHA(t *testing.T) {
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)

	assert.Equal(t, testHFCommitSHA, hfDownloadRevision("main", identity, true))
	assert.Equal(t, "main", hfDownloadRevision("main", ArtifactIdentity{}, false))
}

func TestPlanDirectHfArtifactUsesSharedLifecycle(t *testing.T) {
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.Spec.Storage.StorageUri = stringPtr("hf://Qwen/Qwen3-8B@main")
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)

	plan, ok := planDirectHfArtifact(
		&GopherTask{TaskType: Download, BaseModel: model}, model.Spec, filepath.Dir(childPath), childPath, identity,
	)

	require.True(t, ok)
	assert.Equal(t, ArtifactOperationEnsure, plan.Operation)
	assert.Equal(t, identity, plan.Parent.Identity)
	assert.Equal(t, canonicalHfArtifactPath(childPath, identity), plan.Parent.Path)
	assert.Equal(t, childPath, plan.Child.Path)
	assert.NotEmpty(t, plan.Child.Key)

	always := v1beta1.AlwaysDownload
	model.Spec.Storage.DownloadPolicy = &always
	_, ok = planDirectHfArtifact(
		&GopherTask{TaskType: Download, BaseModel: model}, model.Spec, filepath.Dir(childPath), childPath, identity,
	)
	assert.False(t, ok)
}

func TestDirectHfReplacementKeepsOldParentUntilNewParentIsReady(t *testing.T) {
	root := filepath.Join(t.TempDir(), "models")
	childPath := filepath.Join(root, "model-1")
	model := newHfOriginOCIModel("model-1", childPath)
	model.UID = "uid-model-1"
	model.Spec.Storage.StorageUri = stringPtr("hf://Qwen/Qwen3-8B@main")
	task := &GopherTask{TaskType: Download, BaseModel: model}
	gopher := newGopherWithConfigMap(makeConfigMap("node-1", map[string]string{}))
	gopher.modelRootDir = root

	oldIdentity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", strings.Repeat("a", 40))
	require.NoError(t, err)
	oldPlan, ok := planDirectHfArtifact(task, model.Spec, root, childPath, oldIdentity)
	require.True(t, ok)
	manager := gopher.getHfArtifactManager()
	result, err := manager.Ensure(context.Background(), oldPlan, true, writeTestArtifact)
	require.NoError(t, err)
	require.Equal(t, ArtifactLifecycleCompleted, result.Outcome)

	newIdentity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	newPlan, ok := planDirectHfArtifact(task, model.Spec, root, childPath, newIdentity)
	require.True(t, ok)
	_, err = manager.Ensure(context.Background(), newPlan, true, func(string) error {
		return errors.New("replacement download failed")
	})
	require.Error(t, err)
	assertSymlinkTarget(t, childPath, oldPlan.Parent.Path)
	_, found, err := manager.repository.Get(context.Background(), oldIdentity)
	require.NoError(t, err)
	assert.True(t, found)

	result, err = manager.Ensure(context.Background(), newPlan, true, writeTestArtifact)
	require.NoError(t, err)
	require.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	assertSymlinkTarget(t, childPath, newPlan.Parent.Path)
	_, found, err = manager.repository.Get(context.Background(), oldIdentity)
	require.NoError(t, err)
	assert.True(t, found)

	outcome, err := gopher.releaseSupersededHfParents(context.Background(), task, childPath, newIdentity)

	require.NoError(t, err)
	assert.Equal(t, modelTaskCompleted, outcome)
	_, found, err = manager.repository.Get(context.Background(), oldIdentity)
	require.NoError(t, err)
	assert.False(t, found)
	_, err = os.Lstat(oldPlan.Parent.Path)
	assert.True(t, os.IsNotExist(err))
	assertSymlinkTarget(t, childPath, newPlan.Parent.Path)
}

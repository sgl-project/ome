package modelagent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
)

func TestHfArtifactManagerDownloadsCanonicalParentAndReusesIt(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childOne := filepath.Join(t.TempDir(), "models", "model-1")
	planOne := testArtifactPlan(t, ArtifactOperationEnsure, childOne)
	var downloads atomic.Int32
	download := func(path string) error {
		downloads.Add(1)
		return writeTestArtifact(path)
	}

	first, err := manager.Ensure(context.Background(), planOne, true, download)
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, first.Outcome)
	require.NotNil(t, first.Artifact)
	assert.Equal(t, planOne.Parent.Path, first.Artifact.ParentPath[planOne.Parent.Key])
	assert.Equal(t, int32(1), downloads.Load())
	assertSymlinkTarget(t, childOne, planOne.Parent.Path)
	assert.True(t, hasHfArtifactReadyMarker(planOne.Parent.Path))

	childTwo := filepath.Join(filepath.Dir(childOne), "model-2")
	planTwo := testArtifactPlan(t, ArtifactOperationEnsure, childTwo)
	second, err := manager.Ensure(context.Background(), planTwo, true, download)
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, second.Outcome)
	assert.Equal(t, int32(1), downloads.Load())
	assertSymlinkTarget(t, childTwo, planOne.Parent.Path)

	parent, found, err := repository.Get(context.Background(), planOne.Parent.Identity)
	require.NoError(t, err)
	require.True(t, found)
	assert.ElementsMatch(t, []string{childOne, childTwo}, parent.Children)
}

func TestHfArtifactManagerUsesRegisteredPathForExistingIdentity(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	firstChild := filepath.Join(t.TempDir(), "first-root", "model-1")
	firstPlan := testArtifactPlan(t, ArtifactOperationEnsure, firstChild)
	_, err := manager.Ensure(context.Background(), firstPlan, true, writeTestArtifact)
	require.NoError(t, err)

	secondChild := filepath.Join(t.TempDir(), "second-root", "model-2")
	secondPlan := testArtifactPlan(t, ArtifactOperationEnsure, secondChild)
	require.NotEqual(t, firstPlan.Parent.Path, secondPlan.Parent.Path)
	result, err := manager.Ensure(context.Background(), secondPlan, true, func(string) error {
		t.Fatal("an existing identity must reuse its registered parent path")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	require.NotNil(t, result.Artifact)
	assert.Equal(t, firstPlan.Parent.Path, result.Artifact.ParentPath[firstPlan.Parent.Key])
	assertSymlinkTarget(t, secondChild, firstPlan.Parent.Path)
}

func TestHfArtifactManagerRecordsParentReferenceAndChildArtifactAtomically(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "models", "model-1"))
	var incompleteUpdate atomic.Bool
	client.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		updated := action.(ktesting.UpdateAction).GetObject().(*corev1.ConfigMap)
		rawParent, exists := updated.Data[plan.Parent.Key]
		if !exists {
			return false, nil, nil
		}
		parent, err := decodeArtifactParent(plan.Parent.Key, rawParent)
		if err != nil || len(parent.Children) == 0 {
			return false, nil, nil
		}
		var child ModelEntry
		if rawChild, exists := updated.Data[plan.Child.Key]; !exists || json.Unmarshal([]byte(rawChild), &child) != nil ||
			child.Config == nil || child.Config.Artifact.Sha != testHFCommitSHA {
			incompleteUpdate.Store(true)
		}
		return false, nil, nil
	})

	result, err := manager.Ensure(context.Background(), plan, true, writeTestArtifact)

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	assert.False(t, incompleteUpdate.Load())
}

func TestHfArtifactManagerRemovesUnreferencedParentWhenChildRegistrationIsFenced(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "models", "model-1"))
	repository.configMaps.fenceModelUIDAndEvictCache(plan.Child.Key, plan.Child.UID)

	result, err := manager.Ensure(context.Background(), plan, true, writeTestArtifact)

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, result.Outcome)
	_, found, getErr := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, getErr)
	assert.False(t, found)
	_, statErr := os.Stat(plan.Parent.Path)
	assert.True(t, os.IsNotExist(statErr))
}

func TestHfArtifactManagerDefersWhileParentIsUpdating(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "model-1"))
	_, err := repository.Reserve(context.Background(), plan.Parent)
	require.NoError(t, err)

	result, err := manager.Ensure(context.Background(), plan, true, func(string) error {
		t.Fatal("busy parent must not start another download")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, result.Outcome)
	assert.Equal(t, plan.Parent.Key, result.WaitKey)
}

func TestHfArtifactManagerDoesNotFinalizeRepairFromPreviousReadyMarker(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	first := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "models", "model-1"))
	_, err := manager.Ensure(context.Background(), first, true, writeTestArtifact)
	require.NoError(t, err)
	repairReservation, err := repository.AcquireRepair(context.Background(), first.Parent)
	require.NoError(t, err)
	require.Equal(t, ParentAcquired, repairReservation.Outcome)
	second := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(filepath.Dir(first.ChildPath), "model-2"))

	result, err := manager.Ensure(context.Background(), second, true, func(string) error {
		t.Fatal("a sibling must not download or reuse a parent during an active repair")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, result.Outcome)
	parent, found, getErr := repository.Get(context.Background(), first.Parent.Identity)
	require.NoError(t, getErr)
	require.True(t, found)
	assert.Equal(t, ModelStatusUpdating, parent.Status)
}

func TestHfArtifactManagerDefersTransientRepositoryFailure(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	client.PrependReactor("get", "configmaps", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("temporary API outage")
	})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "model-1"))

	result, err := manager.Ensure(context.Background(), plan, true, func(string) error {
		t.Fatal("repository failure must not start a download")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, result.Outcome)
	assert.Equal(t, plan.Parent.Key, result.WaitKey)
	assert.ErrorContains(t, result.Cause, "temporary API outage")
}

func TestHfArtifactManagerRebuildsReadyParentAtOldPath(t *testing.T) {
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "models", "model-1"))
	oldParent := plan.Parent
	oldParent.Path = filepath.Join(t.TempDir(), "old-model-owned-parent")
	oldParent.Status = ModelStatusReady
	encoded, err := json.Marshal(modelEntryFromArtifactParent(oldParent))
	require.NoError(t, err)
	repository, _ := newTestHfArtifactRepository(t, map[string]string{plan.Parent.Key: string(encoded)})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	var downloads atomic.Int32

	result, err := manager.Ensure(context.Background(), plan, true, func(path string) error {
		downloads.Add(1)
		assert.Equal(t, plan.Parent.Path, path)
		return writeTestArtifact(path)
	})

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	assert.Equal(t, int32(1), downloads.Load())
}

func TestHfArtifactManagerRepairsSharedParent(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childOne := filepath.Join(t.TempDir(), "models", "model-1")
	ensure := testArtifactPlan(t, ArtifactOperationEnsure, childOne)
	_, err := manager.Ensure(context.Background(), ensure, true, writeTestArtifact)
	require.NoError(t, err)

	childTwo := filepath.Join(filepath.Dir(childOne), "model-2")
	_, err = manager.Ensure(context.Background(), testArtifactPlan(t, ArtifactOperationEnsure, childTwo), true, writeTestArtifact)
	require.NoError(t, err)

	repair := testArtifactPlan(t, ArtifactOperationRepair, childOne)
	var repairs atomic.Int32
	result, err := manager.Repair(context.Background(), repair, func(path string) error {
		repairs.Add(1)
		return os.WriteFile(filepath.Join(path, "config.json"), []byte(`{"repaired":true}`), 0o644)
	})

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	assert.Equal(t, int32(1), repairs.Load())
	parent, found, err := repository.Get(context.Background(), repair.Parent.Identity)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ModelStatusReady, parent.Status)
	assert.ElementsMatch(t, []string{childOne, childTwo}, parent.Children)
}

func TestHfArtifactManagerMarksReservationFailedAfterDownloadContextCancellation(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "model-1"))
	ctx, cancel := context.WithCancel(context.Background())

	_, err := manager.Ensure(ctx, plan, true, func(string) error {
		cancel()
		return context.Canceled
	})

	assert.ErrorIs(t, err, context.Canceled)
	parent, found, getErr := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, getErr)
	require.True(t, found)
	assert.Equal(t, ModelStatusFailed, parent.Status)
}

func TestHfArtifactManagerRetriesFailedCancellationCleanup(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "model-1"))
	var failedTransition atomic.Bool
	client.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		updated := action.(ktesting.UpdateAction).GetObject().(*corev1.ConfigMap)
		raw, exists := updated.Data[plan.Parent.Key]
		if !exists {
			return false, nil, nil
		}
		parent, err := decodeArtifactParent(plan.Parent.Key, raw)
		if err == nil && parent.Status == ModelStatusFailed && failedTransition.CompareAndSwap(false, true) {
			return true, nil, errors.New("temporary cleanup failure")
		}
		return false, nil, nil
	})
	ctx, cancel := context.WithCancel(context.Background())

	first, err := manager.Ensure(ctx, plan, true, func(string) error {
		cancel()
		return context.Canceled
	})
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, first.Outcome)
	assert.ErrorContains(t, first.Cause, "temporary cleanup failure")

	second, err := manager.Ensure(context.Background(), plan, true, writeTestArtifact)
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, second.Outcome)
	parent, found, getErr := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, getErr)
	require.True(t, found)
	assert.Equal(t, ModelStatusReady, parent.Status)
}

func TestHfArtifactManagerFinalizesRepairAfterTransientMarkReadyFailure(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	ensure := testArtifactPlan(t, ArtifactOperationEnsure, childPath)
	_, err := manager.Ensure(context.Background(), ensure, true, writeTestArtifact)
	require.NoError(t, err)
	var failedReady atomic.Bool
	client.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		updated := action.(ktesting.UpdateAction).GetObject().(*corev1.ConfigMap)
		raw, exists := updated.Data[ensure.Parent.Key]
		if !exists {
			return false, nil, nil
		}
		parent, decodeErr := decodeArtifactParent(ensure.Parent.Key, raw)
		if decodeErr == nil && parent.Status == ModelStatusReady && failedReady.CompareAndSwap(false, true) {
			return true, nil, errors.New("temporary ready update failure")
		}
		return false, nil, nil
	})
	repair := ensure
	repair.Operation = ArtifactOperationRepair
	var downloads atomic.Int32
	download := func(path string) error {
		downloads.Add(1)
		return writeTestArtifact(path)
	}

	first, err := manager.Repair(context.Background(), repair, download)
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, first.Outcome)
	second, err := manager.Repair(context.Background(), repair, download)
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, second.Outcome)
	assert.Equal(t, int32(1), downloads.Load(), "retry should finalize the completed repair without downloading again")
}

func TestHfArtifactManagerReleasesLastChildAndParent(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	plan := testArtifactPlan(t, ArtifactOperationEnsure, childPath)
	_, err := manager.Ensure(context.Background(), plan, true, writeTestArtifact)
	require.NoError(t, err)

	release := plan
	release.Operation = ArtifactOperationRelease
	result, err := manager.Release(context.Background(), release)

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	_, err = os.Lstat(childPath)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(plan.Parent.Path)
	assert.True(t, os.IsNotExist(err))
	configMap, err := client.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, configMap.Data, plan.Parent.Key)
}

func TestHfArtifactManagerMarksParentFailedWhenFinalDeleteUpdateFails(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	plan := testArtifactPlan(t, ArtifactOperationEnsure, childPath)
	_, err := manager.Ensure(context.Background(), plan, true, writeTestArtifact)
	require.NoError(t, err)
	var failedDelete atomic.Bool
	client.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		updated := action.(ktesting.UpdateAction).GetObject().(*corev1.ConfigMap)
		if _, exists := updated.Data[plan.Parent.Key]; !exists && failedDelete.CompareAndSwap(false, true) {
			return true, nil, errors.New("temporary final update failure")
		}
		return false, nil, nil
	})
	release := plan
	release.Operation = ArtifactOperationRelease

	_, err = manager.Release(context.Background(), release)

	assert.ErrorContains(t, err, "temporary final update failure")
	parent, found, getErr := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, getErr)
	require.True(t, found)
	assert.Equal(t, ModelStatusFailed, parent.Status)
	assert.Empty(t, parent.Children)
	_, statErr := os.Stat(parent.Path)
	assert.True(t, os.IsNotExist(statErr))
}

func TestHfArtifactManagerRetriesFailedReadyRollbackDuringRelease(t *testing.T) {
	repository, client := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	plan := testArtifactPlan(t, ArtifactOperationEnsure, childPath)
	_, err := manager.Ensure(context.Background(), plan, true, writeTestArtifact)
	require.NoError(t, err)
	externalChild := filepath.Join(filepath.Dir(childPath), "external-child")
	require.NoError(t, os.Symlink(plan.Parent.Path, externalChild))
	var failedReady atomic.Bool
	client.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		updated := action.(ktesting.UpdateAction).GetObject().(*corev1.ConfigMap)
		raw, exists := updated.Data[plan.Parent.Key]
		if !exists {
			return false, nil, nil
		}
		parent, decodeErr := decodeArtifactParent(plan.Parent.Key, raw)
		if decodeErr == nil && parent.Status == ModelStatusReady && failedReady.CompareAndSwap(false, true) {
			return true, nil, errors.New("temporary rollback failure")
		}
		return false, nil, nil
	})
	release := plan
	release.Operation = ArtifactOperationRelease

	first, err := manager.Release(context.Background(), release)
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, first.Outcome)
	second, err := manager.Release(context.Background(), release)
	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, second.Outcome)
	parent, found, getErr := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, getErr)
	require.True(t, found)
	assert.Equal(t, ModelStatusReady, parent.Status)
}

func TestHfArtifactManagerDoesNotClearNewerPendingTransition(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	parent := testArtifactParent(t)
	older := pendingArtifactTransition{parent: parent, status: ModelStatusFailed}
	manager.pendingTransitions[parent.Key] = map[string]pendingArtifactTransition{parent.ReservationToken: older}
	newerParent := parent
	newerParent.ReservationToken = "newer-reservation"
	newer := pendingArtifactTransition{parent: newerParent, status: ModelStatusReady}
	manager.pendingTransitions[parent.Key][newerParent.ReservationToken] = newer

	cleared := manager.clearPendingTransition(parent.Key, older)

	assert.True(t, cleared)
	assert.Equal(t, newer, manager.pendingTransitions[parent.Key][newerParent.ReservationToken])
}

func TestHfArtifactManagerDefersReleaseWhileParentIsUpdating(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	plan := testArtifactPlan(t, ArtifactOperationRelease, childPath)
	require.NoError(t, writeTestArtifact(plan.Parent.Path))
	require.NoError(t, os.MkdirAll(filepath.Dir(childPath), 0o755))
	require.NoError(t, os.Symlink(plan.Parent.Path, childPath))
	_, err := repository.Reserve(context.Background(), plan.Parent)
	require.NoError(t, err)

	result, err := manager.Release(context.Background(), plan)

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleDeferred, result.Outcome)
	_, err = os.Lstat(childPath)
	assert.NoError(t, err, "release must preserve the child while its parent is being repaired")
}

func TestHfArtifactManagerRetriesStartupRecoveryForDeleteOnlyWorkload(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	childPath := filepath.Join(t.TempDir(), "models", "model-1")
	plan := testArtifactPlan(t, ArtifactOperationRelease, childPath)
	_, err := repository.Reserve(context.Background(), plan.Parent)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(childPath), 0o755))
	require.NoError(t, os.Symlink(plan.Parent.Path, childPath))
	manager.startupRecoveryPending = true

	result, err := manager.Release(context.Background(), plan)

	require.NoError(t, err)
	assert.Equal(t, ArtifactLifecycleCompleted, result.Outcome)
	_, found, getErr := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, getErr)
	assert.False(t, found)
}

func TestHfArtifactManagerRecoversUpdatingParentsAtStartup(t *testing.T) {
	root := t.TempDir()
	readyPlan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(root, "models", "model-1"))
	readyParent := readyPlan.Parent
	failedIdentity, err := newHfArtifactIdentity("Qwen/Qwen3-4B", testHFCommitSHA)
	require.NoError(t, err)
	failedParent := artifactParentForChild(failedIdentity, filepath.Join(root, "models", "model-2"))

	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	readyReservation, err := repository.Reserve(context.Background(), readyParent)
	require.NoError(t, err)
	failedReservation, err := repository.Reserve(context.Background(), failedParent)
	require.NoError(t, err)
	require.NoError(t, writeTestArtifact(readyParent.Path))
	require.NoError(t, writeHfArtifactReadyMarker(readyParent.Path, readyReservation.Parent.ReservationToken))
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())

	recovered := manager.InitializeAtStartup(context.Background())

	assert.True(t, recovered)
	ready, found, err := repository.Get(context.Background(), readyParent.Identity)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ModelStatusReady, ready.Status)
	assert.Empty(t, ready.ReservationToken)
	failed, found, err := repository.Get(context.Background(), failedParent.Identity)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ModelStatusFailed, failed.Status)
	assert.Empty(t, failed.ReservationToken)
	assert.NotEmpty(t, readyReservation.Parent.ReservationToken)
	assert.NotEmpty(t, failedReservation.Parent.ReservationToken)
}

func TestHfArtifactManagerRejectsStaleReadyMarkerDuringStartupRecovery(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "models", "model-1"))
	_, err := manager.Ensure(context.Background(), plan, true, writeTestArtifact)
	require.NoError(t, err)
	reservation, err := repository.AcquireRepair(context.Background(), plan.Parent)
	require.NoError(t, err)
	require.Equal(t, ParentAcquired, reservation.Outcome)
	restarted := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())

	recovered := restarted.InitializeAtStartup(context.Background())

	assert.True(t, recovered)
	parent, found, getErr := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, getErr)
	require.True(t, found)
	assert.Equal(t, ModelStatusFailed, parent.Status)
}

func TestHfArtifactManagerRecoversLegacyUpdatingParentWithoutReservationToken(t *testing.T) {
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "models", "model-1"))
	legacyUpdating := plan.Parent
	legacyUpdating.Status = ModelStatusUpdating
	encoded, err := json.Marshal(modelEntryFromArtifactParent(legacyUpdating))
	require.NoError(t, err)
	repository, _ := newTestHfArtifactRepository(t, map[string]string{plan.Parent.Key: string(encoded)})
	require.NoError(t, writeTestArtifact(plan.Parent.Path))
	require.NoError(t, writeHfArtifactReadyMarker(plan.Parent.Path))
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())

	recovered := manager.InitializeAtStartup(context.Background())

	assert.True(t, recovered)
	parent, found, err := repository.Get(context.Background(), plan.Parent.Identity)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ModelStatusReady, parent.Status)
}

func TestHfArtifactManagerDeletesInvalidUpdatingParentAtStartup(t *testing.T) {
	parent := testArtifactParent(t)
	invalid, err := json.Marshal(ModelEntry{
		Name:   parent.Key,
		Status: ModelStatusUpdating,
		Config: &ModelConfig{Artifact: Artifact{
			ParentPath: map[string]string{parent.Key: parent.Path},
		}},
	})
	require.NoError(t, err)
	repository, client := newTestHfArtifactRepository(t, map[string]string{parent.Key: string(invalid)})
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())

	recovered := manager.InitializeAtStartup(context.Background())

	assert.True(t, recovered)
	configMap, err := client.CoreV1().ConfigMaps("ome").Get(context.Background(), "node-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, configMap.Data, parent.Key)
}

func TestHfArtifactManagerValidatesReadyParentOnceAfterRestart(t *testing.T) {
	repository, _ := newTestHfArtifactRepository(t, map[string]string{})
	plan := testArtifactPlan(t, ArtifactOperationEnsure, filepath.Join(t.TempDir(), "models", "model-1"))
	reservation, err := repository.Reserve(context.Background(), plan.Parent)
	require.NoError(t, err)
	require.NoError(t, writeTestArtifact(plan.Parent.Path))
	require.NoError(t, writeHfArtifactReadyMarker(plan.Parent.Path))
	require.NoError(t, repository.MarkReady(context.Background(), reservation.Parent))
	manager := newHfArtifactManager(repository, newOSArtifactFileSystem(), zaptest.NewLogger(t).Sugar())
	require.True(t, manager.InitializeAtStartup(context.Background()))
	var validations atomic.Int32
	download := func(path string) error {
		validations.Add(1)
		return writeTestArtifact(path)
	}

	_, err = manager.Ensure(context.Background(), plan, true, download)
	require.NoError(t, err)
	second := plan
	second.ChildPath = filepath.Join(filepath.Dir(plan.ChildPath), "model-2")
	_, err = manager.Ensure(context.Background(), second, true, download)
	require.NoError(t, err)

	assert.Equal(t, int32(1), validations.Load())
}

func testArtifactPlan(t *testing.T, operation ArtifactOperation, childPath string) ArtifactPlan {
	t.Helper()
	identity, err := newHfArtifactIdentity("Qwen/Qwen3-8B", testHFCommitSHA)
	require.NoError(t, err)
	return ArtifactPlan{
		Operation: operation,
		Parent:    artifactParentForChild(identity, childPath),
		Child: ArtifactChild{
			Key:  "default.basemodel." + filepath.Base(childPath),
			Name: filepath.Base(childPath),
			UID:  "uid-" + types.UID(filepath.Base(childPath)),
			Path: childPath,
		},
		ChildPath:  childPath,
		SearchRoot: filepath.Dir(childPath),
	}
}

func writeTestArtifact(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(path, "config.json"), []byte(`{}`), 0o644)
}

func assertSymlinkTarget(t *testing.T, childPath, parentPath string) {
	t.Helper()
	child, err := filepath.EvalSymlinks(childPath)
	require.NoError(t, err)
	parent, err := filepath.EvalSymlinks(parentPath)
	require.NoError(t, err)
	assert.Equal(t, parent, child)
}

package modelagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	modelslister "sigs.k8s.io/ome/pkg/client/listers/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/ociobjectstore"
)

func newLocalRestoreFixture(t *testing.T) (*Gopher, []*GopherTask, hfArtifactTaskInput, cache.Indexer) {
	t.Helper()
	g, original, input := newTestHfArtifactGopher(t)
	h := g.sharedHfArtifactHandler()
	require.NoError(t, runTestHfArtifactDownload(h, input))
	copyInput := input
	copyInput.ChildModelKey = "dac.basemodel.model-1"
	copyInput.ChildModelUID = "copied-uid"
	seedTestChildModelEntry(t, h.repository, copyInput)
	require.NoError(t, runTestHfArtifactDownload(h, copyInput))
	keeper := input
	keeper.ChildModelKey = "default.basemodel.keeper"
	keeper.ChildModelUID = "keeper-uid"
	keeper.ChildModelPath = filepath.Join(input.ModelStoreRoot, "keeper")
	seedTestChildModelEntry(t, h.repository, keeper)
	require.NoError(t, runTestHfArtifactDownload(h, keeper))
	copyTask := &GopherTask{TaskType: Download, BaseModel: original.BaseModel.DeepCopy()}
	copyTask.BaseModel.Namespace = "dac"
	copyTask.BaseModel.UID = copyInput.ChildModelUID
	tasks := []*GopherTask{original, copyTask}
	index := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	g.baseModelLister = modelslister.NewBaseModelLister(index)
	for _, task := range tasks {
		task.BaseModel.Generation = 2
		policy := v1beta1.AlwaysDownload
		task.BaseModel.Spec.Storage.DownloadPolicy = &policy
		task.BaseModel.Annotations[localArtifactRestoreAnnotation] = localArtifactRestoreToken(g.configMapReconciler.nodeName, task.BaseModel)
		require.NoError(t, index.Add(task.BaseModel.DeepCopy()))
	}
	// Model the CM's completed rename: the original bytes are back at the
	// unchanged storage.path, while existing Pods can still mount the parent.
	require.NoError(t, os.Remove(input.ChildModelPath))
	require.NoError(t, writeTestHfArtifactFiles(input.ChildModelPath))
	client := g.configMapReconciler.kubeClient.(*k8sfake.Clientset)
	_, err := client.CoreV1().Nodes().Create(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: g.configMapReconciler.nodeName, Labels: map[string]string{"kubernetes.io/hostname": g.configMapReconciler.nodeName}}}, metav1.CreateOptions{})
	require.NoError(t, err)
	g.nodeLabelReconciler = NewNodeLabelReconciler(g.configMapReconciler.nodeName, client, 1, g.logger)
	return g, tasks, input, index
}

func TestLocalArtifactRestoreRejectsUnsafeInputs(t *testing.T) {
	for _, scenario := range []string{"symlink", "missing", "invalid bytes", "wrong node", "wrong UID", "stale generation", "peer still reuse", "peer unmarked", "peer source differs", "peer deleting", "delete task", "cluster consumer", "keeper absent", "pending download", "no model lister"} {
		t.Run(scenario, func(t *testing.T) {
			g, tasks, input, index := newLocalRestoreFixture(t)
			task, peer := tasks[0], tasks[1].BaseModel.DeepCopy()
			// Exercise fail-closed behavior after the bounded group wait expires.
			g.samePathWaitTimeout = time.Millisecond
			task.SamePathWaitStartedAt = time.Now().Add(-time.Second)
			switch scenario {
			case "symlink":
				require.NoError(t, os.Rename(input.ChildModelPath, input.ChildModelPath+".backup"))
				require.NoError(t, os.Symlink(input.Parent.LocalPath, input.ChildModelPath))
			case "missing":
				require.NoError(t, os.Rename(input.ChildModelPath, input.ChildModelPath+".backup"))
			case "invalid bytes":
				require.NoError(t, os.WriteFile(filepath.Join(input.ChildModelPath, "config.json"), []byte("wrong"), 0o600))
			case "wrong node":
				task.BaseModel.Annotations[localArtifactRestoreAnnotation] = "other-node/uid/2"
			case "wrong UID":
				task.BaseModel.UID = "replacement"
			case "stale generation":
				task.BaseModel.Generation++
			case "peer still reuse":
				policy := v1beta1.ReuseIfExists
				peer.Spec.Storage.DownloadPolicy = &policy
			case "peer unmarked":
				delete(peer.Annotations, localArtifactRestoreAnnotation)
			case "peer source differs":
				peer.Spec.Storage.StorageUri = stringPtr("oci://n/ns/b/models/o/other")
			case "peer deleting":
				now := metav1.Now()
				peer.DeletionTimestamp = &now
			case "delete task":
				task.TaskType = Delete
			case "no model lister":
				g.baseModelLister = nil
			case "cluster consumer":
				clusterIndex := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				require.NoError(t, clusterIndex.Add(&v1beta1.ClusterBaseModel{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Spec: task.BaseModel.Spec}))
				g.clusterBaseModelLister = modelslister.NewClusterBaseModelLister(clusterIndex)
			case "keeper absent", "pending download":
				require.NoError(t, g.configMapReconciler.mutateConfigMapWithRetry(context.Background(), func(cm *corev1.ConfigMap) (bool, error) {
					if scenario == "keeper absent" {
						parent, err := decodeHfArtifactEntry(input.Parent.Key, cm.Data[input.Parent.Key])
						require.NoError(t, err)
						delete(parent.Children, "default.basemodel.keeper")
						return writeHfArtifactEntry(cm.Data, parent)
					}
					child, err := existingModelEntry(cm.Data, input.ChildModelKey)
					require.NoError(t, err)
					child.HfArtifactPendingDownload = &HfArtifactPendingDownload{}
					return writeModelEntry(cm.Data, input.ChildModelKey, child)
				}))
			}
			require.NoError(t, index.Update(peer))
			before, err := g.configMapReconciler.getConfigMap(context.Background())
			require.NoError(t, err)
			handled, err := g.processLocalArtifactRestore(context.Background(), task, true, validateLocalRestoreFixture)
			require.True(t, handled)
			require.Error(t, err)
			after, err := g.configMapReconciler.getConfigMap(context.Background())
			require.NoError(t, err)
			require.Equal(t, before.Data, after.Data, "rejection must not detach or publish Ready")
			require.DirExists(t, input.Parent.LocalPath)
		})
	}
}

func TestLocalArtifactRestoreWaitsForPeerOptIn(t *testing.T) {
	g, tasks, input, index := newLocalRestoreFixture(t)
	g.samePathWaitDelay = time.Millisecond
	peer := tasks[1].BaseModel.DeepCopy()
	delete(peer.Annotations, localArtifactRestoreAnnotation)
	require.NoError(t, index.Update(peer))
	handled, err := g.processLocalArtifactRestore(context.Background(), tasks[0], true, validateLocalRestoreFixture)
	require.True(t, handled)
	require.NoError(t, err)
	_, found, err := g.sharedHfArtifactHandler().repository.GetParentForChild(context.Background(), input.ChildModelKey)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, index.Update(tasks[1].BaseModel.DeepCopy()))
	select {
	case retry := <-g.gopherChan:
		handled, err = g.processLocalArtifactRestore(context.Background(), retry, true, validateLocalRestoreFixture)
		require.True(t, handled)
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("restore did not retry after the peer became ready")
	}
}

func TestLocalArtifactRestoreSerializesCopies(t *testing.T) {
	g, tasks, input, _ := newLocalRestoreFixture(t)
	g.samePathWaitDelay = time.Millisecond
	validating, resume := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := g.processLocalArtifactRestore(context.Background(), tasks[0], true, func(path string) (bool, error) {
			close(validating)
			<-resume
			return validateLocalRestoreFixture(path)
		})
		done <- err
	}()
	<-validating
	// Direct writers and a sibling's restore must both wait for verification,
	// reference removal, and status publication to finish.
	lock, acquired, err := tryHfArtifactChildFileLock(input)
	if lock != nil {
		defer lock.Close()
	}
	require.NoError(t, err)
	require.False(t, acquired)
	handled, err := g.processLocalArtifactRestore(context.Background(), tasks[1], true, validateLocalRestoreFixture)
	require.True(t, handled)
	require.NoError(t, err)
	close(resume)
	require.NoError(t, <-done)
	select {
	case retry := <-g.gopherChan:
		require.Same(t, tasks[1], retry)
		handled, err = g.processLocalArtifactRestore(context.Background(), retry, true, validateLocalRestoreFixture)
		require.True(t, handled)
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("sibling restore was not requeued")
	}
}

func TestLocalArtifactRestoreDirectoryReplacementAndCancellation(t *testing.T) {
	for _, scenario := range []string{"replace directory", "cancel", "validation error"} {
		t.Run(scenario, func(t *testing.T) {
			g, tasks, input, _ := newLocalRestoreFixture(t)
			before, err := g.configMapReconciler.getConfigMap(context.Background())
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			handled, err := g.processLocalArtifactRestore(ctx, tasks[0], true, func(path string) (bool, error) {
				switch scenario {
				case "replace directory":
					require.NoError(t, os.Rename(path, path+".saved"))
					require.NoError(t, writeTestHfArtifactFiles(path))
				case "cancel":
					cancel()
				case "validation error":
					return false, errors.New("inspection unavailable")
				}
				return true, nil
			})
			require.True(t, handled)
			require.Error(t, err)
			after, err := g.configMapReconciler.getConfigMap(context.Background())
			require.NoError(t, err)
			require.Equal(t, before.Data, after.Data)
			require.DirExists(t, input.Parent.LocalPath)
		})
	}
}

func TestLocalArtifactRestoreWorkerDoesNotDownloadOnValidationFailure(t *testing.T) {
	g, tasks, input, index := newLocalRestoreFixture(t)
	for _, task := range tasks {
		parameters := map[string]string{"auth": "unsupported-test-auth"}
		task.BaseModel.Spec.Storage.Parameters = &parameters
		require.NoError(t, index.Update(task.BaseModel.DeepCopy()))
	}
	err := g.processTaskWithSourceAdapters(tasks[0], true,
		func(context.Context, *GopherTask, v1beta1.BaseModelSpec, bool) (bool, error) {
			t.Fatal("restore must not download from HF")
			return false, nil
		}, func(context.Context, *ociobjectstore.ObjectURI, string, *GopherTask) error {
			t.Fatal("restore must not download from OCI")
			return nil
		})
	require.ErrorContains(t, err, "local restore validation failed")
	_, found, err := g.sharedHfArtifactHandler().repository.GetParentForChild(context.Background(), input.ChildModelKey)
	require.NoError(t, err)
	require.True(t, found)
	require.DirExists(t, input.ChildModelPath)
}

func TestLocalArtifactRestoreNormalSamePathGuardUnchanged(t *testing.T) {
	g, tasks, input, index := newLocalRestoreFixture(t)
	for _, task := range tasks {
		delete(task.BaseModel.Annotations, localArtifactRestoreAnnotation)
		require.NoError(t, index.Update(task.BaseModel.DeepCopy()))
	}
	handled, err := g.processLocalArtifactRestore(context.Background(), tasks[0], true, func(string) (bool, error) {
		t.Fatal("unselected model must not enter recovery")
		return false, nil
	})
	require.False(t, handled)
	require.NoError(t, err)
	_, _, err = g.processHfOCIArtifact(context.Background(), tasks[0], tasks[0].BaseModel.Spec, true)
	require.ErrorContains(t, err, "still used by another model")
	_, found, err := g.sharedHfArtifactHandler().repository.GetParentForChild(context.Background(), input.ChildModelKey)
	require.NoError(t, err)
	require.True(t, found)
}

func TestLocalArtifactRestoreConfigMapFailures(t *testing.T) {
	for _, scenario := range []string{"conflict", "update failure", "committed response lost", "peer changes on conflict"} {
		t.Run(scenario, func(t *testing.T) {
			g, tasks, input, index := newLocalRestoreFixture(t)
			client := g.configMapReconciler.kubeClient.(*k8sfake.Clientset)
			first := true
			client.PrependReactor("update", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
				if !first {
					return false, nil, nil
				}
				first = false
				switch scenario {
				case "conflict", "peer changes on conflict":
					if scenario == "peer changes on conflict" {
						peer := tasks[1].BaseModel.DeepCopy()
						peer.Generation++
						require.NoError(t, index.Update(peer))
					}
					return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, "node", errors.New("concurrent update"))
				case "committed response lost":
					obj := action.(k8stesting.UpdateAction).GetObject()
					require.NoError(t, client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("configmaps"), obj, action.GetNamespace()))
				}
				return true, nil, errors.New("response unavailable")
			})
			handled, err := g.processLocalArtifactRestore(context.Background(), tasks[0], true, validateLocalRestoreFixture)
			require.True(t, handled)
			if scenario == "conflict" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			_, found, err := g.sharedHfArtifactHandler().repository.GetParentForChild(context.Background(), input.ChildModelKey)
			require.NoError(t, err)
			require.Equal(t, scenario == "update failure" || scenario == "peer changes on conflict", found)
			if scenario != "peer changes on conflict" {
				handled, err = g.processLocalArtifactRestore(context.Background(), tasks[0], true, validateLocalRestoreFixture)
				require.True(t, handled)
				require.NoError(t, err)
			}
			require.DirExists(t, input.ChildModelPath)
			require.DirExists(t, input.Parent.LocalPath)
		})
	}
}

func TestLocalArtifactRestoreOldQueuedTaskCannotBypassOptIn(t *testing.T) {
	g, tasks, input, _ := newLocalRestoreFixture(t)
	old := *tasks[0]
	old.BaseModel = tasks[0].BaseModel.DeepCopy()
	delete(old.BaseModel.Annotations, localArtifactRestoreAnnotation)
	err := g.processTask(&old)
	require.ErrorContains(t, err, "local restore")
	_, found, err := g.sharedHfArtifactHandler().repository.GetParentForChild(context.Background(), input.ChildModelKey)
	require.NoError(t, err)
	require.True(t, found)
}

func TestLocalArtifactRestoreRechecksAfterValidation(t *testing.T) {
	g, tasks, input, index := newLocalRestoreFixture(t)
	before, err := g.configMapReconciler.getConfigMap(context.Background())
	require.NoError(t, err)
	handled, err := g.processLocalArtifactRestore(context.Background(), tasks[0], true, func(path string) (bool, error) {
		peer := tasks[1].BaseModel.DeepCopy()
		peer.Generation++
		require.NoError(t, index.Update(peer))
		return validateLocalRestoreFixture(path)
	})
	require.True(t, handled)
	require.Error(t, err)
	after, err := g.configMapReconciler.getConfigMap(context.Background())
	require.NoError(t, err)
	require.Equal(t, before.Data, after.Data)
	require.DirExists(t, input.ChildModelPath)
}

func TestLocalArtifactRestoreResumesAfterReadyFailure(t *testing.T) {
	g, tasks, input, _ := newLocalRestoreFixture(t)
	client := g.configMapReconciler.kubeClient.(*k8sfake.Clientset)
	fail := true
	client.PrependReactor("patch", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		if fail {
			return true, nil, errors.New("node status unavailable")
		}
		return false, nil, nil
	})
	handled, err := g.processLocalArtifactRestore(context.Background(), tasks[0], true, validateLocalRestoreFixture)
	require.True(t, handled)
	require.ErrorContains(t, err, "node status unavailable")
	_, found, err := g.sharedHfArtifactHandler().repository.GetParentForChild(context.Background(), input.ChildModelKey)
	require.NoError(t, err)
	require.False(t, found)
	fail = false
	// Recreate the agent and its in-memory cache from persisted state.
	restarted := &Gopher{configMapReconciler: NewConfigMapReconciler(g.configMapReconciler.nodeName, g.configMapReconciler.namespace, client, g.logger),
		modelRootDir: g.modelRootDir, logger: g.logger, baseModelLister: g.baseModelLister,
		clusterBaseModelLister: g.clusterBaseModelLister, nodeLabelReconciler: g.nodeLabelReconciler}
	handled, err = restarted.processLocalArtifactRestore(context.Background(), tasks[0], true, validateLocalRestoreFixture)
	require.True(t, handled)
	require.NoError(t, err)
	require.DirExists(t, input.ChildModelPath)
	require.DirExists(t, input.Parent.LocalPath)
}

func TestLocalArtifactRestoreWorkerBlocksDelete(t *testing.T) {
	g, tasks, input, _ := newLocalRestoreFixture(t)
	tasks[0].TaskType = Delete
	before, err := g.configMapReconciler.getConfigMap(context.Background())
	require.NoError(t, err)
	err = g.processTask(tasks[0])
	require.ErrorContains(t, err, "local restore")
	after, err := g.configMapReconciler.getConfigMap(context.Background())
	require.NoError(t, err)
	require.Equal(t, before.Data, after.Data)
	require.DirExists(t, input.ChildModelPath)
}

func validateLocalRestoreFixture(path string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(path, "config.json"))
	return string(data) == "{}", err
}

func TestLocalArtifactRestoreBothCopyOrders(t *testing.T) {
	for _, copyFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "original first", true: "copy first"}[copyFirst], func(t *testing.T) {
			g, tasks, input, _ := newLocalRestoreFixture(t)
			if copyFirst {
				tasks[0], tasks[1] = tasks[1], tasks[0]
			}
			before, err := os.Stat(input.ChildModelPath)
			require.NoError(t, err)
			for _, task := range tasks {
				handled, err := g.processLocalArtifactRestore(context.Background(), task, true, validateLocalRestoreFixture)
				require.True(t, handled)
				require.NoError(t, err)
			}
			parent, found, err := g.sharedHfArtifactHandler().repository.Get(context.Background(), input.Parent.Identity)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, map[string]string{"default.basemodel.keeper": filepath.Join(input.ModelStoreRoot, "keeper")}, parent.Children)
			require.Equal(t, HfArtifactStatusReady, parent.Status)
			for _, task := range tasks {
				// Repeating after references are gone must still validate and adopt,
				// never fall through to an ordinary download.
				handled, err := g.processLocalArtifactRestore(context.Background(), task, true, validateLocalRestoreFixture)
				require.True(t, handled)
				require.NoError(t, err)
				cm, err := g.configMapReconciler.getConfigMap(context.Background())
				require.NoError(t, err)
				entry, err := existingModelEntry(cm.Data, getModelID(task.BaseModel, nil))
				require.NoError(t, err)
				require.Empty(t, entry.HfArtifactKey)
				require.Equal(t, ModelStatusReady, entry.Status)
			}
			after, err := os.Stat(input.ChildModelPath)
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after))
			require.DirExists(t, input.Parent.LocalPath)
		})
	}
}

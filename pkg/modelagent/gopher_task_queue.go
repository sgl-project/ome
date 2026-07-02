package modelagent

import (
	"sync"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

type gopherTaskQueue struct {
	mutex  sync.Mutex
	cond   *sync.Cond
	high   []*GopherTask
	normal []*GopherTask
	closed bool
}

func newGopherTaskQueue() *gopherTaskQueue {
	queue := &gopherTaskQueue{}
	queue.cond = sync.NewCond(&queue.mutex)
	return queue
}

func (q *gopherTaskQueue) enqueue(task *GopherTask) {
	if task == nil {
		return
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.closed {
		return
	}
	if task.TaskType == Delete {
		// Delete preempts pending work for the same model and should run before
		// reuse-wait tasks, so it is the only non-FIFO insertion.
		q.high = removeSupersededTasks(q.high, task)
		q.normal = removeSupersededTasks(q.normal, task)
		q.high = append([]*GopherTask{task}, q.high...)
	} else if shouldUseHighPriorityQueue(task) {
		q.high = append(q.high, task)
	} else {
		q.normal = append(q.normal, task)
	}
	q.cond.Broadcast()
}

func (q *gopherTaskQueue) popNormal() (*GopherTask, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	for len(q.normal) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.normal) > 0 {
		task := q.normal[0]
		q.normal = q.normal[1:]
		return task, true
	}
	return nil, false
}

func (q *gopherTaskQueue) popHighPriority() (*GopherTask, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	for len(q.high) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.high) > 0 {
		task := q.high[0]
		q.high = q.high[1:]
		return task, true
	}
	return nil, false
}

func (q *gopherTaskQueue) close() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

func (q *gopherTaskQueue) len() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.high) + len(q.normal)
}

func shouldUseHighPriorityQueue(task *GopherTask) bool {
	return task.TaskType == Delete || isObjectStorageDownloadTask(task) || (!task.NormalPriorityOnly && !task.SamePathWaitStartedAt.IsZero())
}

func isObjectStorageDownloadTask(task *GopherTask) bool {
	if task == nil || task.TaskType != Download || task.NormalPriorityOnly {
		return false
	}
	var storageSpec *v1beta1.StorageSpec
	if task.BaseModel != nil {
		storageSpec = task.BaseModel.Spec.Storage
	} else if task.ClusterBaseModel != nil {
		storageSpec = task.ClusterBaseModel.Spec.Storage
	}
	if storageSpec == nil || storageSpec.StorageUri == nil {
		return false
	}
	storageType, err := storage.GetStorageType(*storageSpec.StorageUri)
	return err == nil && storageType == storage.StorageTypeOCI
}

func removeSupersededTasks(tasks []*GopherTask, deleteTask *GopherTask) []*GopherTask {
	modelUID := getModelUID(deleteTask)
	if modelUID == "" {
		return tasks
	}
	kept := tasks[:0]
	for _, task := range tasks {
		if task.TaskType != Delete && getModelUID(task) == modelUID {
			continue
		}
		kept = append(kept, task)
	}
	return kept
}

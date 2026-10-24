// Package input builds Alfred's predictive scheduler inputs from public API
// objects. It neither renders workloads nor performs live migration actions.
package input

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Snapshot is a lossless, content-addressed observation. Treat it as immutable;
// Validate detects accidental mutation before request construction. The lists
// are not an atomic API-server transaction: freshness is measured from the
// first read, and a prediction never reserves capacity or authorizes migration.
type Snapshot struct {
	ID                   string                     `json:"id"`
	StartedAt            time.Time                  `json:"startedAt"`
	CompletedAt          time.Time                  `json:"completedAt"`
	Objects              []runtime.RawExtension     `json:"objects"`
	InferenceServices    []v1beta1.InferenceService `json:"inferenceServices"`
	InferenceReplicas    []v1beta1.InferenceReplica `json:"inferenceReplicas"`
	ListResourceVersions map[string]string          `json:"listResourceVersions"`
}

// Capture lists the worker-supported scheduling objects plus the public OME
// owners used to identify a migration candidate. reader MUST be lossless (for
// example manager.GetAPIReader()); Alfred's transformed Pod cache is not a
// valid source. Supply a context deadline to bound API reads. A missing CRD,
// permission error, partial list, or other read error returns no snapshot.
// Pending and terminating Pods are retained so the model cannot silently omit
// transient demand. The worker models supported same-profile standalone pending
// competitors; other pending demand is rejected, never dropped. Terminal Pods
// do not occupy scheduler capacity.
func Capture(ctx context.Context, reader client.Reader, now func() time.Time) (*Snapshot, error) {
	if reader == nil || now == nil {
		return nil, fmt.Errorf("capture requires a reader and clock")
	}
	group := schema.GroupVersion{Group: "scheduling.x-k8s.io", Version: "v1alpha1"}
	groups := &unstructured.UnstructuredList{}
	groups.SetGroupVersionKind(group.WithKind("PodGroupList"))
	return capture(ctx, reader, now, []captureList{
		{list: &corev1.NodeList{}, kind: corev1.SchemeGroupVersion.WithKind("Node")},
		{list: &corev1.PodList{}, kind: corev1.SchemeGroupVersion.WithKind("Pod")},
		{list: &corev1.NamespaceList{}, kind: corev1.SchemeGroupVersion.WithKind("Namespace")},
		{list: &corev1.ServiceList{}, kind: corev1.SchemeGroupVersion.WithKind("Service")},
		{list: &corev1.ReplicationControllerList{}, kind: corev1.SchemeGroupVersion.WithKind("ReplicationController")},
		{list: &appsv1.ReplicaSetList{}, kind: appsv1.SchemeGroupVersion.WithKind("ReplicaSet")},
		{list: &appsv1.StatefulSetList{}, kind: appsv1.SchemeGroupVersion.WithKind("StatefulSet")},
		{list: groups, kind: group.WithKind("PodGroup")},
		{list: &v1beta1.InferenceServiceList{}, kind: v1beta1.SchemeGroupVersion.WithKind("InferenceService")},
		{list: &v1beta1.InferenceReplicaList{}, kind: v1beta1.SchemeGroupVersion.WithKind("InferenceReplica")},
	}, nil)
}

// CaptureSource is the bounded, lossless capture for a move onto a new TPU
// slice, which runs no scheduler simulation. It reads every InferenceService
// and InferenceReplica (in-flight migrations and budgets), the source
// Instance's Pods, the PodGroups they name, and the source Node: what
// ExecutionSourcePods and the dispatch checks read. Listing every Pod would not
// fit a large cluster, and anything wider than the source would make two
// captures differ on unrelated churn. Pods are selected by their identity
// labels, so unlike Capture it cannot reject a Pod that the source
// InferenceReplica owns but that carries other identity labels; the
// InferenceReplica's Pod counts must still match. It is not a scheduling input;
// BuildExecutionRequest needs Capture.
func CaptureSource(ctx context.Context, reader client.Reader, source Source, now func() time.Time) (*Snapshot, error) {
	if reader == nil || now == nil {
		return nil, fmt.Errorf("capture requires a reader and clock")
	}
	if err := validateSourceKey(source); err != nil {
		return nil, err
	}
	started := now()
	node := &corev1.Node{}
	if err := reader.Get(ctx, client.ObjectKey{Name: source.FromNode}, node); err != nil {
		return nil, fmt.Errorf("capture source Node: %w", err)
	}
	pods := &corev1.PodList{}
	if err := reader.List(ctx, pods, client.InNamespace(source.Namespace), client.MatchingLabels{
		labelInferenceService: source.InferenceService,
		labelComponent:        string(source.Component),
		labelInstanceIndex:    fmt.Sprintf("%d", source.Instance),
	}); err != nil {
		return nil, fmt.Errorf("capture source Pods: %w", err)
	}
	objects := []capturedObject{{object: node, kind: corev1.SchemeGroupVersion.WithKind("Node")}}
	groupKind := schema.GroupVersionKind{Group: "scheduling.x-k8s.io", Version: "v1alpha1", Kind: "PodGroup"}
	groups := map[string]bool{}
	for i := range pods.Items {
		if name := pods.Items[i].Labels[labelPodGroup]; name != "" && !groups[name] {
			groups[name] = true
			group := &unstructured.Unstructured{}
			group.SetGroupVersionKind(groupKind)
			if err := reader.Get(ctx, client.ObjectKey{Namespace: source.Namespace, Name: name}, group); err != nil {
				return nil, fmt.Errorf("capture source PodGroup %q: %w", name, err)
			}
			objects = append(objects, capturedObject{object: group, kind: groupKind})
		}
	}
	return capture(ctx, reader, startedAt(started, now), []captureList{
		{list: pods, kind: corev1.SchemeGroupVersion.WithKind("Pod"), listed: true},
		{list: &v1beta1.InferenceServiceList{}, kind: v1beta1.SchemeGroupVersion.WithKind("InferenceService")},
		{list: &v1beta1.InferenceReplicaList{}, kind: v1beta1.SchemeGroupVersion.WithKind("InferenceReplica")},
	}, objects)
}

// startedAt returns a clock whose first reading is start, so a capture's
// StartedAt precedes the reads made before capture runs.
func startedAt(start time.Time, now func() time.Time) func() time.Time {
	first := true
	return func() time.Time {
		if first {
			first = false
			return start
		}
		return now()
	}
}

// captureList is one list a capture reads, with its kind and list options.
// listed marks a list the caller already read.
type captureList struct {
	list   client.ObjectList
	kind   schema.GroupVersionKind
	opts   []client.ListOption
	listed bool
}

// capturedObject is one object a capture read with Get instead of a list.
type capturedObject struct {
	object runtime.Object
	kind   schema.GroupVersionKind
}

// capture reads lists, then adds objects, into one Snapshot with a stable
// order and content identity.
func capture(ctx context.Context, reader client.Reader, now func() time.Time, lists []captureList, objects []capturedObject) (*Snapshot, error) {
	s := &Snapshot{StartedAt: now(), ListResourceVersions: make(map[string]string)}
	seen := make(map[string]bool)
	add := func(item runtime.Object, kind schema.GroupVersionKind) error {
		object := item.DeepCopyObject()
		object.GetObjectKind().SetGroupVersionKind(kind)
		meta, err := apiMeta.Accessor(object)
		if err != nil {
			return fmt.Errorf("capture %s metadata: %w", kind.Kind, err)
		}
		key := kind.String() + "/" + meta.GetNamespace() + "/" + meta.GetName()
		if meta.GetName() == "" || seen[key] {
			return fmt.Errorf("capture has empty or duplicate object identity %q", key)
		}
		seen[key] = true
		switch typed := object.(type) {
		case *v1beta1.InferenceService:
			s.InferenceServices = append(s.InferenceServices, *typed)
			return nil
		case *v1beta1.InferenceReplica:
			s.InferenceReplicas = append(s.InferenceReplicas, *typed)
			return nil
		case *corev1.Pod:
			if typed.Status.Phase == corev1.PodSucceeded || typed.Status.Phase == corev1.PodFailed {
				return nil
			}
		}
		raw, err := json.Marshal(object)
		if err != nil {
			return fmt.Errorf("capture %s JSON: %w", key, err)
		}
		s.Objects = append(s.Objects, runtime.RawExtension{Raw: raw})
		return nil
	}
	for _, entry := range lists {
		if !entry.listed {
			if err := reader.List(ctx, entry.list, entry.opts...); err != nil {
				return nil, fmt.Errorf("capture %s: %w", entry.kind.Kind, err)
			}
		}
		if entry.list.GetContinue() != "" {
			return nil, fmt.Errorf("capture %s returned a partial list", entry.kind.Kind)
		}
		s.ListResourceVersions[entry.kind.String()] = entry.list.GetResourceVersion()
		items, err := apiMeta.ExtractList(entry.list)
		if err != nil {
			return nil, fmt.Errorf("capture %s objects: %w", entry.kind.Kind, err)
		}
		for _, item := range items {
			if err := add(item, entry.kind); err != nil {
				return nil, err
			}
		}
	}
	for _, entry := range objects {
		meta, err := apiMeta.Accessor(entry.object)
		if err != nil {
			return nil, fmt.Errorf("capture %s metadata: %w", entry.kind.Kind, err)
		}
		s.ListResourceVersions[entry.kind.String()] = meta.GetResourceVersion()
		if err := add(entry.object, entry.kind); err != nil {
			return nil, err
		}
	}
	// JSON is deterministic for the typed objects and string-keyed maps above.
	// Sorting the serialized objects also makes input-list ordering irrelevant.
	sort.Slice(s.Objects, func(i, j int) bool { return string(s.Objects[i].Raw) < string(s.Objects[j].Raw) })
	sort.Slice(s.InferenceServices, func(i, j int) bool {
		a, b := s.InferenceServices[i], s.InferenceServices[j]
		return a.Namespace+"/"+a.Name < b.Namespace+"/"+b.Name
	})
	sort.Slice(s.InferenceReplicas, func(i, j int) bool {
		a, b := s.InferenceReplicas[i], s.InferenceReplicas[j]
		return a.Namespace+"/"+a.Name < b.Namespace+"/"+b.Name
	})
	s.CompletedAt = now()
	if s.StartedAt.IsZero() || s.CompletedAt.Before(s.StartedAt) {
		return nil, fmt.Errorf("capture clock is zero or moved backwards")
	}
	var err error
	s.ID, err = s.contentID()
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Validate checks observation freshness and content identity. It does not
// claim cross-resource atomicity or verify that live objects remain unchanged.
func (s *Snapshot) Validate(now time.Time, maxAge time.Duration) error {
	if s == nil || maxAge <= 0 || now.IsZero() {
		return fmt.Errorf("snapshot, current time and positive maximum age are required")
	}
	if s.StartedAt.IsZero() || s.CompletedAt.IsZero() || s.CompletedAt.Before(s.StartedAt) || s.CompletedAt.After(now) {
		return fmt.Errorf("snapshot has invalid or future capture times")
	}
	if now.Sub(s.StartedAt) > maxAge {
		return fmt.Errorf("snapshot is stale")
	}
	id, err := s.contentID()
	if err != nil {
		return err
	}
	if s.ID == "" || s.ID != id {
		return fmt.Errorf("snapshot content identity changed")
	}
	return nil
}

func (s *Snapshot) contentID() (string, error) {
	copy := *s
	copy.ID = ""
	encoded, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("snapshot digest: %w", err)
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

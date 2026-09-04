package workload

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

// ObservationEpoch identifies where an observation is consumed.
type ObservationEpoch uint8

const (
	ObservationEpochUnknown ObservationEpoch = iota
	ObservationEpochDecision
	ObservationEpochPublication
)

// PodObservationSource identifies the Kubernetes read path.
type PodObservationSource uint8

const (
	PodObservationSourceUnknown PodObservationSource = iota
	PodObservationSourceCache
	PodObservationSourceAPIReader
)

// PodObservationScope identifies which matching Pods a read contains.
type PodObservationScope uint8

const (
	PodObservationScopeUnknown PodObservationScope = iota
	PodObservationScopeSelector
	PodObservationScopeOwnerUID
)

// PodObservation is a per-pass, read-only view of Component Pods.
type PodObservation struct {
	source     PodObservationSource
	scope      PodObservationScope
	byInstance map[int32][]*corev1.Pod
}

func newPodObservation(source PodObservationSource, scope PodObservationScope, pods []*corev1.Pod, byInstance map[int32][]*corev1.Pod) PodObservation {
	if byInstance == nil {
		byInstance = query.BucketPodsByInstanceIdx(pods)
	}
	return PodObservation{source: source, scope: scope, byInstance: byInstance}
}

// NewCachedSelectorPodObservation records a selector-scoped cache read.
func NewCachedSelectorPodObservation(pods []*corev1.Pod, byInstance map[int32][]*corev1.Pod) PodObservation {
	return newPodObservation(PodObservationSourceCache, PodObservationScopeSelector, pods, byInstance)
}

// NewAPIReaderSelectorPodObservation records a selector-scoped API read.
func NewAPIReaderSelectorPodObservation(pods []*corev1.Pod, byInstance map[int32][]*corev1.Pod) PodObservation {
	return newPodObservation(PodObservationSourceAPIReader, PodObservationScopeSelector, pods, byInstance)
}

func (o PodObservation) Source() PodObservationSource { return o.source }

func (o PodObservation) Scope() PodObservationScope { return o.scope }

// PodsForInstance returns the read-only bucket for index.
func (o PodObservation) PodsForInstance(index int32) []*corev1.Pod {
	return o.byInstance[index]
}

// ComponentObservation joins one persisted row set with one Pod observation.
type ComponentObservation struct {
	epoch                ObservationEpoch
	persisted            []InstanceStatus
	pods                 PodObservation
	availableByPod       map[string]struct{}
	availabilityWindow   AvailabilityWindow
	availabilityObserved bool
	consumed             bool
}

var errDecisionObservationProvenance = errors.New("decision observation requires a supported Pod source and scope")

// NewDecisionObservation borrows persisted as read-only reconcile state.
func NewDecisionObservation(persisted []InstanceStatus, pods PodObservation) (ComponentObservation, error) {
	if !pods.validForDecision() {
		return ComponentObservation{}, fmt.Errorf("%w: source=%d scope=%d", errDecisionObservationProvenance, pods.source, pods.scope)
	}
	return ComponentObservation{
		epoch:     ObservationEpochDecision,
		persisted: persisted,
		pods:      pods,
	}, nil
}

var errPublicationObservationProvenance = errors.New("publication observation requires a selector-scoped cache read")

// NewOwnedPublicationObservation takes ownership of persisted. The caller must
// not access that slice after construction. availableByPod is the
// EndpointSlice rotation set and window the Component's minReadySeconds rule;
// together they define the Available counters this observation publishes.
func NewOwnedPublicationObservation(persisted []InstanceStatus, pods PodObservation, availableByPod map[string]struct{}, window AvailabilityWindow) (*ComponentObservation, error) {
	if pods.source != PodObservationSourceCache || pods.scope != PodObservationScopeSelector {
		return nil, fmt.Errorf("%w: source=%d scope=%d", errPublicationObservationProvenance, pods.source, pods.scope)
	}
	return &ComponentObservation{
		epoch:                ObservationEpochPublication,
		persisted:            persisted,
		pods:                 pods,
		availableByPod:       availableByPod,
		availabilityWindow:   window,
		availabilityObserved: true,
	}, nil
}

func (o ComponentObservation) Epoch() ObservationEpoch { return o.epoch }

func (o ComponentObservation) PodSource() PodObservationSource { return o.pods.Source() }

func (o ComponentObservation) PodScope() PodObservationScope { return o.pods.Scope() }

// PersistedStatuses returns an isolated copy in epoch row order.
func (o *ComponentObservation) PersistedStatuses() []InstanceStatus {
	return cloneInstanceStatusSlice(o.persisted)
}

// CurrentCounters returns index-keyed Pod facts and whether availability was
// observed at this epoch.
func (o *ComponentObservation) CurrentCounters(index int32) (InstanceCounters, bool) {
	return CountersForInstance(o.pods.PodsForInstance(index), o.availableByPod, o.availabilityWindow), o.availabilityObserved
}

var errPublicationObservationRequired = errors.New("publication materialization requires a publication observation")
var errPublicationObservationConsumed = errors.New("publication observation has already been consumed")

// InlineV1Statuses overlays publication facts on copies of persisted rows.
func (o *ComponentObservation) InlineV1Statuses() ([]InstanceStatus, error) {
	if err := o.validatePublication(); err != nil {
		return nil, err
	}
	out := cloneInstanceStatusSlice(o.persisted)
	o.overlayInlineV1(out, nil)
	return out, nil
}

// TakeInlineV1Statuses consumes the owned rows and overlays publication facts
// without allocating another full status slice.
func (o *ComponentObservation) TakeInlineV1Statuses() ([]InstanceStatus, error) {
	return o.takeInlineV1Statuses(nil)
}

func (o *ComponentObservation) takeInlineV1Statuses(observe func(int32, string, InstanceCounters)) ([]InstanceStatus, error) {
	if err := o.validatePublication(); err != nil {
		return nil, err
	}
	out := o.persisted
	o.persisted = nil
	o.consumed = true
	o.overlayInlineV1(out, observe)
	return out, nil
}

func (o *ComponentObservation) validatePublication() error {
	if o == nil || o.epoch != ObservationEpochPublication || !o.availabilityObserved ||
		o.pods.source != PodObservationSourceCache || o.pods.scope != PodObservationScopeSelector {
		return errPublicationObservationRequired
	}
	if o.consumed {
		return errPublicationObservationConsumed
	}
	return nil
}

func (o *ComponentObservation) overlayInlineV1(out []InstanceStatus, observe func(int32, string, InstanceCounters)) {
	for i := range out {
		current, _ := o.CurrentCounters(out[i].Index)
		if observe != nil {
			observe(out[i].Index, out[i].RunningRevision, current)
		}
		out[i].PodCount = current.PodCount
		out[i].ReadyPodCount = current.ReadyPodCount
		out[i].ServingPodCount = current.ServingPodCount
		out[i].AvailablePodCount = current.AvailablePodCount
		out[i].ScheduledPodCount = current.ScheduledPodCount
		out[i].Admitted = current.Admitted
		out[i].NodesOccupied = current.NodesOccupied
	}
}

func (o PodObservation) validForDecision() bool {
	switch o.source {
	case PodObservationSourceCache:
		return o.scope == PodObservationScopeSelector
	case PodObservationSourceAPIReader:
		return o.scope == PodObservationScopeUnknown ||
			o.scope == PodObservationScopeSelector ||
			o.scope == PodObservationScopeOwnerUID
	default:
		return false
	}
}

func cloneInstanceStatusSlice(in []InstanceStatus) []InstanceStatus {
	if in == nil {
		return nil
	}
	out := make([]InstanceStatus, len(in))
	for i := range in {
		out[i] = cloneInstanceStatus(in[i])
	}
	return out
}

func cloneInstanceStatus(in InstanceStatus) InstanceStatus {
	out := in
	if in.NodesOccupied != nil {
		out.NodesOccupied = append([]string(nil), in.NodesOccupied...)
	}
	if in.Conditions != nil {
		out.Conditions = append([]metav1.Condition(nil), in.Conditions...)
	}
	if in.Operation != nil {
		operation := *in.Operation
		if in.Operation.SurgeIndex != nil {
			surgeIndex := *in.Operation.SurgeIndex
			operation.SurgeIndex = &surgeIndex
		}
		if in.Operation.HintTargetNodes != nil {
			operation.HintTargetNodes = append([]string(nil), in.Operation.HintTargetNodes...)
		}
		out.Operation = &operation
	}
	if in.LastFailure != nil {
		lastFailure := *in.LastFailure
		if in.LastFailure.ExitCode != nil {
			exitCode := *in.LastFailure.ExitCode
			lastFailure.ExitCode = &exitCode
		}
		out.LastFailure = &lastFailure
	}
	return out
}

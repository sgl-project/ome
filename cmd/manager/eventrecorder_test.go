package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clocktesting "k8s.io/utils/clock/testing"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

// deliveredEvents stands in for the API server: it counts, per involved
// object, every event the broadcaster lets through its spam filter, and
// closes the channel a test registered for an object's first delivery.
type deliveredEvents struct {
	mu       sync.Mutex
	counts   map[string]int
	arrivals map[string]chan struct{}
}

func (d *deliveredEvents) accept(event *corev1.Event) (*corev1.Event, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	name := event.InvolvedObject.Name
	d.counts[name]++
	if arrived, ok := d.arrivals[name]; ok {
		close(arrived)
		delete(d.arrivals, name)
	}
	return event, nil
}

func (d *deliveredEvents) Create(event *corev1.Event) (*corev1.Event, error) { return d.accept(event) }
func (d *deliveredEvents) Update(event *corev1.Event) (*corev1.Event, error) { return d.accept(event) }
func (d *deliveredEvents) Patch(event *corev1.Event, _ []byte) (*corev1.Event, error) {
	return d.accept(event)
}

func (d *deliveredEvents) count(name string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.counts[name]
}

// arrival returns a channel the sink closes when the first event about the
// named object is delivered. Register it before emitting that event.
func (d *deliveredEvents) arrival(name string) <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	arrived := make(chan struct{})
	d.arrivals[name] = arrived
	return arrived
}

// eventHarness drives a broadcaster built the way the manager builds its
// own, with a counting sink in place of the API server.
type eventHarness struct {
	recorder record.EventRecorder
	sink     *deliveredEvents
	sequence int
}

func startEventHarness(t *testing.T, settings controllerconfig.EventRecorderSettings) *eventHarness {
	t.Helper()
	return attachEventHarness(t, newEventBroadcaster(settings))
}

// startEventHarnessOnClock builds the broadcaster from the same correlator
// options newEventBroadcaster uses, with the spam filter's token bucket
// reading the given clock instead of the wall clock: a test steps the
// refill instead of waiting for it.
func startEventHarnessOnClock(t *testing.T, settings controllerconfig.EventRecorderSettings, clock *clocktesting.FakeClock) *eventHarness {
	t.Helper()
	options := settings.CorrelatorOptions()
	options.Clock = clock
	return attachEventHarness(t, record.NewBroadcasterWithCorrelatorOptions(options))
}

func attachEventHarness(t *testing.T, broadcaster record.EventBroadcaster) *eventHarness {
	t.Helper()
	t.Cleanup(broadcaster.Shutdown)
	sink := &deliveredEvents{counts: map[string]int{}, arrivals: map[string]chan struct{}{}}
	broadcaster.StartRecordingToSink(sink)
	return &eventHarness{
		recorder: broadcaster.NewRecorder(runtime.NewScheme(), corev1.EventSource{Component: "v1beta1Controllers"}),
		sink:     sink,
	}
}

// emitStateChanges records count distinct Normal events about one object,
// the shape of the per-Instance events a rollout stamps on its component.
func (h *eventHarness) emitStateChanges(object string, count int) {
	subject := &corev1.ObjectReference{Kind: "InferenceReplica", Namespace: "team-a", Name: object, UID: types.UID("uid-" + object)}
	for i := 0; i < count; i++ {
		h.sequence++
		h.recorder.Eventf(subject, corev1.EventTypeNormal, fmt.Sprintf("StateChange%d", h.sequence), "state change %d", h.sequence)
	}
}

// settle emits one event about a fresh object and waits for the sink to
// receive it. The broadcaster delivers in emission order, so once that
// event arrives every earlier event has been delivered or dropped.
func (h *eventHarness) settle(t *testing.T) {
	t.Helper()
	h.sequence++
	closing := fmt.Sprintf("closing-%d", h.sequence)
	arrived := h.sink.arrival(closing)
	h.emitStateChanges(closing, 1)
	select {
	case <-arrived:
	case <-time.After(10 * time.Second):
		t.Fatalf("the event about %s did not reach the sink within 10s", closing)
	}
}

// TestNewEventBroadcaster_Unconfigured_KeepsClientGoBudget is the loss the
// configuration exists to fix: with nothing configured, one object receives
// client-go's default of 25 events and every later one is dropped silently.
func TestNewEventBroadcaster_Unconfigured_KeepsClientGoBudget(t *testing.T) {
	harness := startEventHarness(t, controllerconfig.EventRecorderSettings{})
	harness.emitStateChanges("fleet-engine", 40)
	harness.settle(t)
	assert.Equal(t, 25, harness.sink.count("fleet-engine"))
}

// TestNewEventBroadcaster_ConfiguredBurst_IsTheBound: a configured burst of 64
// lets one object emit 64 events back to back without a drop, and the 65th
// is the first one dropped — the bound is the configured one.
func TestNewEventBroadcaster_ConfiguredBurst_IsTheBound(t *testing.T) {
	settings := controllerconfig.EventRecorderSettings{BurstSize: 64}

	withinBurst := startEventHarness(t, settings)
	withinBurst.emitStateChanges("fleet-engine", 64)
	withinBurst.settle(t)
	assert.Equal(t, 64, withinBurst.sink.count("fleet-engine"))

	pastBurst := startEventHarness(t, settings)
	pastBurst.emitStateChanges("fleet-engine", 65)
	pastBurst.settle(t)
	assert.Equal(t, 64, pastBurst.sink.count("fleet-engine"))
}

// TestNewEventBroadcaster_ConfiguredRefill_AdmitsEventsPastTheBurst: with a configured
// refill, an object that spent its burst keeps emitting without drops as
// long as it stays under the refill rate. The bucket reads a fake clock;
// each settle lets the filter see the events emitted so far, and each step
// of five refill intervals regains room for the next event with margin.
func TestNewEventBroadcaster_ConfiguredRefill_AdmitsEventsPastTheBurst(t *testing.T) {
	const refill = 20 * time.Millisecond
	clock := clocktesting.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	harness := startEventHarnessOnClock(t, controllerconfig.EventRecorderSettings{
		BurstSize:      8,
		RefillInterval: refill,
	}, clock)
	harness.emitStateChanges("fleet-engine", 8)
	for i := 0; i < 3; i++ {
		harness.settle(t)
		clock.Step(5 * refill)
		harness.emitStateChanges("fleet-engine", 1)
	}
	harness.settle(t)
	assert.Equal(t, 11, harness.sink.count("fleet-engine"))
}

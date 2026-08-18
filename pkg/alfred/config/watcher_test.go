package config

import (
	"strings"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

type countingObserver struct {
	success, failure int
}

func (c *countingObserver) ObserveConfigReload(outcome string) {
	if outcome == OutcomeSuccess {
		c.success++
	} else {
		c.failure++
	}
}

func configMap(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ome", Name: "alfred-config"},
		Data:       data,
	}
}

func newTestWatcher() (*Watcher, *countingObserver, *record.FakeRecorder) {
	observer := &countingObserver{}
	recorder := record.NewFakeRecorder(10)
	w := &Watcher{
		Namespace: "ome",
		Name:      "alfred-config",
		Key:       "config.yaml",
		Store:     NewStore(),
		Log:       logr.Discard(),
		Recorder:  recorder,
		Observer:  observer,
	}
	return w, observer, recorder
}

func TestWatcherAppliesValidConfig(t *testing.T) {
	w, observer, _ := newTestWatcher()

	w.Apply(configMap(map[string]string{"config.yaml": "schemaVersion: 1\nmode: execute"}))
	if w.Store.Get().Mode != ModeExecute {
		t.Fatal("config not applied")
	}
	if observer.success != 1 || observer.failure != 0 {
		t.Fatalf("observations: %+v", observer)
	}

	// Resync with an identical document must not inflate the metric.
	w.Apply(configMap(map[string]string{"config.yaml": "schemaVersion: 1\nmode: execute"}))
	if observer.success != 1 {
		t.Fatalf("resync dedup failed: %+v", observer)
	}
}

func TestWatcherKeepsLastKnownGoodOnBadEdit(t *testing.T) {
	w, observer, recorder := newTestWatcher()

	w.Apply(configMap(map[string]string{"config.yaml": "schemaVersion: 1\nmode: execute"}))
	w.Apply(configMap(map[string]string{"config.yaml": "schemaVersion: 42"}))

	if w.Store.Get().Mode != ModeExecute {
		t.Fatal("broken edit must keep last-known-good")
	}
	if observer.failure != 1 {
		t.Fatalf("failure not observed: %+v", observer)
	}
	select {
	case event := <-recorder.Events:
		if want := "PolicyReloadFailed"; !strings.Contains(event, want) {
			t.Fatalf("event %q does not mention %q", event, want)
		}
	default:
		t.Fatal("no PolicyReloadFailed event emitted")
	}

	// The same broken document again is deduped — one loud report, not a
	// report per resync.
	w.Apply(configMap(map[string]string{"config.yaml": "schemaVersion: 42"}))
	if observer.failure != 1 {
		t.Fatalf("failure dedup failed: %+v", observer)
	}
}

func TestWatcherMissingKey(t *testing.T) {
	w, observer, recorder := newTestWatcher()
	w.Apply(configMap(map[string]string{"other.yaml": ""}))
	if observer.failure != 1 {
		t.Fatalf("missing key should observe failure: %+v", observer)
	}
	if w.Store.Get().Mode != ModeRecommendOnly {
		t.Fatal("defaults must survive a missing key")
	}

	// The same missing-key state on an informer resync is deduped: one
	// loud report, not one per resync.
	w.Apply(configMap(map[string]string{"other.yaml": ""}))
	if observer.failure != 1 {
		t.Fatalf("missing-key resync dedup failed: %+v", observer)
	}
	if got := len(recorder.Events); got != 1 {
		t.Fatalf("want exactly one PolicyReloadFailed event, recorder holds %d", got)
	}

	// The key appearing afterwards reloads normally.
	w.Apply(configMap(map[string]string{"config.yaml": "schemaVersion: 1\nmode: execute"}))
	if observer.success != 1 || w.Store.Get().Mode != ModeExecute {
		t.Fatalf("recovery after missing key failed: %+v", observer)
	}

	// And a fresh disappearance is reported again (state, not one-shot).
	w.Apply(configMap(map[string]string{"other.yaml": ""}))
	if observer.failure != 2 {
		t.Fatalf("re-disappearance should report once more: %+v", observer)
	}
}

func TestWatcherIgnoresOtherConfigMaps(t *testing.T) {
	w, observer, _ := newTestWatcher()
	other := configMap(map[string]string{"config.yaml": "schemaVersion: 1\nmode: execute"})
	other.Name = "not-alfred"
	w.observe(other)
	if observer.success != 0 || w.Store.Get().Mode != ModeRecommendOnly {
		t.Fatal("watcher must ignore unrelated ConfigMaps")
	}
}

func TestWatcherNeedsNoLeadership(t *testing.T) {
	w, _, _ := newTestWatcher()
	if w.NeedLeaderElection() {
		t.Fatal("config watcher must run on every replica")
	}
}

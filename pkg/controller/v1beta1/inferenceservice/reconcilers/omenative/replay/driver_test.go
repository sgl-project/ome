package replay

import (
	"context"
	"strings"
	"testing"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

const plainWalk = `scenario: plain
cells: ["run/example-cell"]
initial:
  service:
    components:
      engine: {replicas: 2, image: registry.example.com/runtime:v1}
    groups:
      - components: [engine]
        blueGreen: {}
  config: {requeue: 10s}
timeline:
  - tick: 1
    note: settled
  - tick: 2
    events:
      - spec.revision[new]: {component: engine, image: registry.example.com/runtime:v2}
  - tick: 3
    events:
      - ctrl.resync
`

// A run opens on the pass after a template edit, once the projection has
// caught up, and the trace carries the note, the applied event and the
// pinned run.
func TestRunOpensAfterTheProjectionCatchesUp(t *testing.T) {
	s, err := Parse([]byte(plainWalk), Vocabulary{})
	if err != nil {
		t.Fatal(err)
	}
	trace, err := Run(context.Background(), s, Options{})
	if err != nil {
		t.Fatalf("run: %v\n%s", err, trace)
	}
	for _, want := range []string{
		"tick=1 note settled",
		"tick=2 apply spec.revision[new] component=engine image=registry.example.com/runtime:v2 generation=2",
		"tick=2 run outcome=none requeueAfter=10s",
		"tick=3 run outcome=opened stateChanged=true",
		"tick=3 flush which=boundary result=written",
	} {
		if !strings.Contains(trace, want) {
			t.Errorf("trace lacks %q\n%s", want, trace)
		}
	}
}

// A stale read before any pass has nothing to serve, and a stale-read
// variant the cluster does not bear out fails the run.
func TestStaleReadClaimsAreChecked(t *testing.T) {
	early := strings.Replace(plainWalk, "  - tick: 1\n    note: settled\n", "  - tick: 1\n    events:\n      - ctrl.staleRead[missesOpen]\n", 1)
	s, err := Parse([]byte(early), Vocabulary{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), s, Options{}); err == nil || !strings.Contains(err.Error(), "no cache snapshot") {
		t.Fatalf("stale read with nothing to serve: %v", err)
	}
	unmet := strings.Replace(plainWalk, "  - tick: 3\n    events:\n      - ctrl.resync\n", "  - tick: 3\n    events:\n      - ctrl.staleRead[missesClose]\n", 1)
	s, err = Parse([]byte(unmet), Vocabulary{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), s, Options{}); err == nil || !strings.Contains(err.Error(), "does not miss") {
		t.Fatalf("unmet stale claim: %v", err)
	}
}

// An observer sees every tick's end state: the service, the replicas, the
// pods and the run layer's outcome, with the trace unchanged by it.
func TestObserverSeesEveryTick(t *testing.T) {
	s, err := Parse([]byte(plainWalk), Vocabulary{})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Run(context.Background(), s, Options{})
	if err != nil {
		t.Fatalf("run: %v\n%s", err, plain)
	}
	var views []TickView
	observed, err := Run(context.Background(), s, Options{Observer: func(view TickView) { views = append(views, view) }})
	if err != nil {
		t.Fatalf("run with the observer on: %v\n%s", err, observed)
	}
	if observed != plain {
		t.Fatalf("the observer changed the trace\n--- plain ---\n%s--- observed ---\n%s", plain, observed)
	}
	if len(views) != 3 {
		t.Fatalf("%d views, want one per tick", len(views))
	}
	opened := views[2]
	switch {
	case opened.Tick != 3 || !opened.Run.Ran || !opened.Run.Opened || opened.Run.Adopted:
		t.Fatalf("tick 3 run view = %+v, want an opened run", opened.Run)
	case len(opened.Run.Targets) != 1 || opened.Run.Targets[0].Revision == opened.Run.Targets[0].StableRevision:
		t.Fatalf("tick 3 targets = %+v, want one diverged target", opened.Run.Targets)
	case opened.Service == nil || opened.Service.Status.Rollout == nil || opened.Service.Status.Rollout.ActiveRun == nil:
		t.Fatalf("tick 3 service carries no active run: %+v", opened.Service)
	case len(opened.Pods[v1beta1.EngineComponent]) != 2:
		t.Fatalf("tick 3 lists %d engine pods, want 2", len(opened.Pods[v1beta1.EngineComponent]))
	}
	ir := opened.Replicas[v1beta1.EngineComponent]
	if ir == nil || ir.Status.CurrentRevision == ir.Status.UpdateRevision {
		t.Fatalf("tick 3 replica pair is not diverged: %+v", ir)
	}
	if views[0].Run.Ran && views[0].Run.Opened {
		t.Fatalf("tick 1 opened a run on a settled service: %+v", views[0].Run)
	}
	if got := CheckInvariants(opened); len(got) != 0 {
		t.Fatalf("the plain walk makes findings: %+v", got)
	}
}

// A status that trails its spec by a write publishes nothing on its tick:
// the last written status stands, pair and generation included, and the
// next tick publishes the spec's revision under the new generation. A lag
// the stand-in does not know fails the run.
func TestStaleStatusKeepsTheLastPublication(t *testing.T) {
	kept := strings.Replace(plainWalk, "      - spec.revision[new]: {component: engine, image: registry.example.com/runtime:v2}\n",
		"      - spec.revision[new]: {component: engine, image: registry.example.com/runtime:v2}\n      - ir.stale: {component: engine, lag: status}\n", 1)
	s, err := Parse([]byte(kept), Vocabulary{})
	if err != nil {
		t.Fatal(err)
	}
	trace, err := Run(context.Background(), s, Options{})
	if err != nil {
		t.Fatalf("run: %v\n%s", err, trace)
	}
	if strings.Contains(trace, "tick=2 publish component=engine") {
		t.Errorf("the engine must publish nothing on the tick its status trails the spec\n%s", trace)
	}
	for _, want := range []string{
		"tick=2 apply ir.stale component=engine lag=status",
		"tick=3 publish component=engine current=svc-a-engine-f332566b update=svc-a-engine-46e64ada observedGeneration=2",
	} {
		if !strings.Contains(trace, want) {
			t.Errorf("trace lacks %q\n%s", want, trace)
		}
	}
	unknown := strings.Replace(kept, "lag: status", "lag: rows", 1)
	s, err = Parse([]byte(unknown), Vocabulary{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), s, Options{}); err == nil || !strings.Contains(err.Error(), "lag must be generation or status") {
		t.Fatalf("unknown lag: %v", err)
	}
}

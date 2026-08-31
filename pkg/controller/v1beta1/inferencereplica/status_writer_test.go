package inferencereplica

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

var transientInstanceObservationFields = map[string]struct{}{
	"ReadyPodCount":     {},
	"ScheduledPodCount": {},
	"NodesOccupied":     {},
}

func TestClearPodDerivedInstanceObservations(t *testing.T) {
	original := populatedInstanceStatus()
	assertEveryRetainedInstanceFieldPopulated(t, original)

	ir := &v1beta1.InferenceReplica{
		Status: v1beta1.InferenceReplicaStatus{
			InstanceStatuses: []v1beta1.OMENativeInstanceStatus{original},
		},
	}
	want := original.DeepCopy()
	want.ReadyPodCount = 0
	want.ScheduledPodCount = 0
	want.NodesOccupied = nil

	clearPodDerivedInstanceObservations(ir)
	if got := ir.Status.InstanceStatuses[0]; !reflect.DeepEqual(got, *want) {
		t.Fatalf("cleared status differs outside the three transient fields:\n got: %#v\nwant: %#v", got, *want)
	}

	once := ir.DeepCopy()
	clearPodDerivedInstanceObservations(ir)
	if !reflect.DeepEqual(ir, once) {
		t.Fatalf("second clear changed status:\n got: %#v\nwant: %#v", ir.Status, once.Status)
	}
}

func TestUpdateInferenceReplicaStatusPersistsCompactedStatus(t *testing.T) {
	ctx := context.Background()
	ir := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{Name: "model-engine", Namespace: "default"},
		Status: v1beta1.InferenceReplicaStatus{
			ObservedGeneration: 3,
			Replicas:           1,
			InstanceStatuses:   []v1beta1.OMENativeInstanceStatus{populatedInstanceStatus()},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(ir).
		WithStatusSubresource(&v1beta1.InferenceReplica{}).
		Build()

	live := &v1beta1.InferenceReplica{}
	key := client.ObjectKeyFromObject(ir)
	if err := c.Get(ctx, key, live); err != nil {
		t.Fatalf("get InferenceReplica: %v", err)
	}
	want := live.DeepCopy()
	want.Status.InstanceStatuses[0].ReadyPodCount = 0
	want.Status.InstanceStatuses[0].ScheduledPodCount = 0
	want.Status.InstanceStatuses[0].NodesOccupied = nil

	if err := updateInferenceReplicaStatus(ctx, c, live); err != nil {
		t.Fatalf("update status: %v", err)
	}
	stored := &v1beta1.InferenceReplica{}
	if err := c.Get(ctx, key, stored); err != nil {
		t.Fatalf("get persisted InferenceReplica: %v", err)
	}
	if !reflect.DeepEqual(stored.Status, want.Status) {
		t.Fatalf("persisted status differs outside the three transient fields:\n got: %#v\nwant: %#v", stored.Status, want.Status)
	}
}

func TestInferenceReplicaStatusUpdatesUseWriterBoundary(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	dir := filepath.Dir(thisFile)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	const writer = "updateInferenceReplicaStatus"
	var writerCalls, statusAccesses, statusSubresources, rawUpdates []string
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				site := fmt.Sprintf("%s:%d (%s)", entry.Name(), fset.Position(call.Pos()).Line, fn.Name.Name)
				if ident, isIdent := call.Fun.(*ast.Ident); isIdent && ident.Name == writer {
					writerCalls = append(writerCalls, site)
				}
				if selectorCallNamed(call, "Status") {
					statusAccesses = append(statusAccesses, site)
				}
				if statusSubresourceCall(call) {
					statusSubresources = append(statusSubresources, site)
				}
				if rawStatusUpdate(call) {
					rawUpdates = append(rawUpdates, site)
				}
				return true
			})
		}
	}

	if len(writerCalls) != 12 {
		t.Fatalf("production status writes through %s = %d, want 12: %v", writer, len(writerCalls), writerCalls)
	}
	if len(statusAccesses) != 1 || !strings.Contains(statusAccesses[0], "status_writer.go") || !strings.Contains(statusAccesses[0], "("+writer+")") {
		t.Fatalf("raw Status() access must exist only in %s: %v", writer, statusAccesses)
	}
	if len(rawUpdates) != 1 || rawUpdates[0] != statusAccesses[0] {
		t.Fatalf("raw Status().Update must exist only in %s: %v", writer, rawUpdates)
	}
	if len(statusSubresources) != 0 {
		t.Fatalf("SubResource(\"status\") bypasses %s: %v", writer, statusSubresources)
	}
}

func selectorCallNamed(call *ast.CallExpr, name string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == name
}

func rawStatusUpdate(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Update" {
		return false
	}
	statusCall, ok := selector.X.(*ast.CallExpr)
	return ok && selectorCallNamed(statusCall, "Status")
}

func statusSubresourceCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "SubResource" || len(call.Args) != 1 {
		return false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	return ok && literal.Kind == token.STRING && literal.Value == `"status"`
}

func assertEveryRetainedInstanceFieldPopulated(t *testing.T, status v1beta1.OMENativeInstanceStatus) {
	t.Helper()
	typeOfStatus := reflect.TypeOf(status)
	valueOfStatus := reflect.ValueOf(status)
	for i := 0; i < typeOfStatus.NumField(); i++ {
		field := typeOfStatus.Field(i)
		if _, transient := transientInstanceObservationFields[field.Name]; transient {
			continue
		}
		if valueOfStatus.Field(i).IsZero() {
			t.Fatalf("retained fixture field %s must be populated", field.Name)
		}
	}
}

func populatedInstanceStatus() v1beta1.OMENativeInstanceStatus {
	now := metav1.NewTime(time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC))
	surgeIndex := int32(9)
	exitCode := int32(137)
	return v1beta1.OMENativeInstanceStatus{
		Index:             7,
		Incarnation:       2,
		Phase:             v1beta1.OMENativeInstanceUpdating,
		RunningRevision:   "revision-a",
		TargetRevision:    "revision-b",
		PodCount:          8,
		ReadyPodCount:     7,
		ServingPodCount:   6,
		AvailablePodCount: 5,
		ScheduledPodCount: 8,
		Admitted:          true,
		NodesOccupied:     []string{"node-a", "node-b"},
		Conditions:        []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, ObservedGeneration: 3, LastTransitionTime: now, Reason: "PodsReady", Message: "all pods are ready"}},
		ActiveOrdinal:     1,
		Operation:         &v1beta1.InstanceOperation{ID: "operation-a", Type: v1beta1.InstanceOperationMigrate, Step: "WaitReady", StartedAt: now, LastProgressAt: now, Deadline: now, RetryCount: 1, TargetRevision: "revision-b", Reason: "placement", SurgeIndex: &surgeIndex, FromNode: "node-a", HintTargetNodes: []string{"node-b"}, RequestUUID: "request-a"},
		LastFailure:       &v1beta1.InstanceTermination{PodName: "model-engine-7", ContainerName: "server", Reason: "OOMKilled", ExitCode: &exitCode, Message: "container terminated", Time: now},
	}
}

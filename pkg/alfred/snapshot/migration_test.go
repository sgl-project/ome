package snapshot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/audit"
)

func migrationTestWorkload() *Workload {
	return &Workload{Components: map[v1beta1.ComponentType]*Component{
		v1beta1.EngineComponent: {
			Type: v1beta1.EngineComponent,
			Instances: []*Instance{
				{Index: 0},
				{Index: 2},
			},
		},
	}}
}

func TestParsePendingMigrationAcceptsCanonicalSnakeCase(t *testing.T) {
	now := time.Date(2026, 8, 31, 9, 30, 0, 0, time.UTC)
	raw, err := json.Marshal(audit.MigrationRequest{
		SchemaVersion:   audit.SchemaV1,
		Component:       string(v1beta1.EngineComponent),
		Instance:        2,
		FromNode:        "gpu-a",
		HintTargetNodes: []string{"gpu-b", "gpu-c"},
		Reason:          "fragmentation",
		RequestedAt:     now.Format(time.RFC3339),
		RequestedBy:     "alfred-controller",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	got, err := parsePendingMigration("request-1", string(raw), migrationTestWorkload())
	if err != nil {
		t.Fatalf("parsePendingMigration: %v", err)
	}
	if got.UUID != "request-1" || got.Component != v1beta1.EngineComponent || got.FromNode != "gpu-a" {
		t.Fatalf("identity mismatch: %+v", got)
	}
	if got.RequestedBy != "alfred-controller" {
		t.Fatalf("RequestedBy = %q, want alfred-controller", got.RequestedBy)
	}
	if !got.RequestedAt.Equal(now) {
		t.Fatalf("RequestedAt = %v, want %v", got.RequestedAt, now)
	}
}

func TestParsePendingMigrationRejectsCamelCase(t *testing.T) {
	raw := `{"schemaVersion":"v1","component":"engine","instance":2,"fromNode":"gpu-a","requestedAt":"2026-08-31T09:30:00Z","requestedBy":"alfred-controller"}`

	_, err := parsePendingMigration("request-1", raw, migrationTestWorkload())
	if err == nil {
		t.Fatal("parsePendingMigration accepted camel-case request")
	}
	if !strings.Contains(err.Error(), "from_node") {
		t.Fatalf("error = %q, want missing from_node", err)
	}
}

func TestParsePendingMigrationValidatesCurrentInstance(t *testing.T) {
	valid := audit.MigrationRequest{
		SchemaVersion: audit.SchemaV1,
		Component:     string(v1beta1.EngineComponent),
		Instance:      2,
		FromNode:      "gpu-a",
		RequestedAt:   "2026-08-31T09:30:00Z",
	}
	tests := []struct {
		name        string
		uuid        string
		mutate      func(*audit.MigrationRequest)
		wantErrPart string
	}{
		{name: "empty UUID", uuid: "", mutate: func(*audit.MigrationRequest) {}, wantErrPart: "UUID"},
		{name: "missing source", uuid: "request-1", mutate: func(r *audit.MigrationRequest) { r.FromNode = "" }, wantErrPart: "from_node"},
		{name: "negative instance", uuid: "request-1", mutate: func(r *audit.MigrationRequest) { r.Instance = -1 }, wantErrPart: "instance"},
		{name: "unknown component", uuid: "request-1", mutate: func(r *audit.MigrationRequest) { r.Component = "sidecar" }, wantErrPart: "component"},
		{name: "missing current instance", uuid: "request-1", mutate: func(r *audit.MigrationRequest) { r.Instance = 1 }, wantErrPart: "current instance"},
		{name: "invalid requested_at", uuid: "request-1", mutate: func(r *audit.MigrationRequest) { r.RequestedAt = "yesterday" }, wantErrPart: "requested_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			tt.mutate(&req)
			raw, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			_, err = parsePendingMigration(tt.uuid, string(raw), migrationTestWorkload())
			if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErrPart)
			}
		})
	}
}

func TestParsePendingMigrationPreservesMissingRequestedAt(t *testing.T) {
	raw, err := json.Marshal(audit.MigrationRequest{
		SchemaVersion: audit.SchemaV1,
		Component:     string(v1beta1.EngineComponent),
		Instance:      2,
		FromNode:      "gpu-a",
		RequestedBy:   "foreign-controller",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	got, err := parsePendingMigration("legacy-1", string(raw), migrationTestWorkload())
	if err != nil {
		t.Fatalf("parsePendingMigration: %v", err)
	}
	if !got.RequestedAt.IsZero() {
		t.Fatalf("RequestedAt = %v, want zero", got.RequestedAt)
	}
}

func TestApplyMigrationStatePreservesRequesterFromPendingAnnotation(t *testing.T) {
	raw, err := json.Marshal(audit.MigrationRequest{
		SchemaVersion: audit.SchemaV1,
		Component:     string(v1beta1.EngineComponent),
		Instance:      0,
		FromNode:      "gpu-a",
		RequestedAt:   "2026-08-31T09:30:00Z",
		RequestedBy:   "alfred-controller",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	workload := migrationTestWorkload()
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
			migrationAnnotationKey("request-1"): string(raw),
		}},
		Status: v1beta1.InferenceServiceStatus{MigrationHistory: []v1beta1.MigrationHistoryEntry{{
			ID:          "request-1",
			Component:   v1beta1.EngineComponent,
			Mode:        v1beta1.MigrationModeSurge,
			Phase:       v1beta1.MigrationPhaseSurgePending,
			RequestedAt: metav1.NewTime(time.Date(2026, 8, 31, 9, 30, 0, 0, time.UTC)),
		}}},
	}

	applyMigrationState(workload, isvc)

	if len(workload.ActiveMigrations) != 1 {
		t.Fatalf("ActiveMigrations = %+v, want one migration", workload.ActiveMigrations)
	}
	if got := workload.ActiveMigrations[0].RequestedBy; got != "alfred-controller" {
		t.Fatalf("RequestedBy = %q, want alfred-controller", got)
	}
}

func TestMarshalAlfredMigrationRequestRoundTripsThroughAudit(t *testing.T) {
	now := time.Date(2026, 8, 31, 9, 30, 0, 0, time.UTC)
	req := audit.MigrationRequest{
		SchemaVersion:   audit.SchemaV1,
		Component:       string(v1beta1.EngineComponent),
		Instance:        2,
		FromNode:        "gpu-a",
		HintTargetNodes: []string{"gpu-b", "gpu-c"},
		Reason:          "fragmentation",
		RequestedAt:     now.UTC().Format(time.RFC3339),
		RequestedBy:     "alfred-controller",
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("inspect request keys: %v", err)
	}
	for _, key := range []string{"from_node", "hint_target_nodes", "requested_at", "requested_by"} {
		if _, ok := keys[key]; !ok {
			t.Errorf("canonical key %q missing from %s", key, raw)
		}
	}
	for _, key := range []string{"fromNode", "hintTargetNodes", "requestedAt", "requestedBy"} {
		if _, ok := keys[key]; ok {
			t.Errorf("camel-case key %q present in %s", key, raw)
		}
	}
	parsed, err := audit.ParseMigrationRequest(string(raw))
	if err != nil {
		t.Fatalf("audit.ParseMigrationRequest: %v", err)
	}
	if parsed.FromNode != req.FromNode || parsed.RequestedAt != req.RequestedAt || parsed.RequestedBy != req.RequestedBy {
		t.Fatalf("round-trip mismatch: got %+v want %+v", parsed, req)
	}
	if got := migrationAnnotationKey("request-1"); got != audit.MigrationRequestAnnotationPrefix+"request-1" {
		t.Fatalf("migrationAnnotationKey = %q", got)
	}
}

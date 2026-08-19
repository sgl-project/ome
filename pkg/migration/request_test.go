package migration

import (
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/constants"
)

var testTime = metav1.NewTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))

func validRequest() Request {
	return Request{
		SchemaVersion:   SchemaVersionV1,
		Component:       "engine",
		Instance:        0,
		Reason:          "fragmentation",
		FromNode:        "node1",
		HintTargetNodes: []string{"node3", "node7"},
		RequestedAt:     testTime,
		RequestedBy:     "alfred-controller",
	}
}

func TestAnnotationKeyRoundtrip(t *testing.T) {
	key := AnnotationKey("abc-123")
	if key != constants.MigrationRequestAnnotationPrefix+"abc-123" {
		t.Fatalf("unexpected key %q", key)
	}
	if !IsAnnotationKey(key) {
		t.Fatalf("IsAnnotationKey(%q) = false", key)
	}
	uuid, ok := UUIDFromAnnotationKey(key)
	if !ok || uuid != "abc-123" {
		t.Fatalf("UUIDFromAnnotationKey(%q) = %q, %v", key, uuid, ok)
	}
}

func TestIsAnnotationKeyRejectsNonRequests(t *testing.T) {
	for _, key := range []string{
		"ome.io/deploymentMode",
		"ome.io/migration-restart",
		constants.MigrationRequestAnnotationPrefix, // prefix with no UUID
		"alfred.ome.io/movable",
	} {
		if IsAnnotationKey(key) {
			t.Errorf("IsAnnotationKey(%q) = true, want false", key)
		}
	}
}

func TestMarshalParseRoundtrip(t *testing.T) {
	req := validRequest()
	payload, err := req.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	valid, malformed := ExtractRequests(map[string]string{AnnotationKey("u1"): payload})
	if len(malformed) != 0 {
		t.Fatalf("unexpected malformed: %+v", malformed)
	}
	if len(valid) != 1 || valid[0].UUID != "u1" {
		t.Fatalf("unexpected valid: %+v", valid)
	}
	got := valid[0].Request
	if got.Component != "engine" || got.FromNode != "node1" || len(got.HintTargetNodes) != 2 || !got.RequestedAt.Equal(&testTime) {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Request)
		wantErr bool
		wantIs  error
	}{
		{name: "valid", mutate: func(r *Request) {}},
		{name: "unsupported schema", mutate: func(r *Request) { r.SchemaVersion = "v2" }, wantErr: true, wantIs: ErrUnsupportedSchemaVersion},
		{name: "unknown component", mutate: func(r *Request) { r.Component = "sidecar" }, wantErr: true},
		{name: "missing fromNode", mutate: func(r *Request) { r.FromNode = "" }, wantErr: true},
		{name: "negative instance", mutate: func(r *Request) { r.Instance = -1 }, wantErr: true},
		{name: "missing requestedAt", mutate: func(r *Request) { r.RequestedAt = metav1.Time{} }, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validRequest()
			tc.mutate(&req)
			err := req.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("want errors.Is(%v), got %v", tc.wantIs, err)
			}
		})
	}
}

func TestExtractRequestsSplitsAndSorts(t *testing.T) {
	older := validRequest()
	older.RequestedAt = metav1.NewTime(testTime.Add(-time.Hour))
	olderPayload, _ := older.Marshal()

	newer := validRequest()
	newerPayload, _ := newer.Marshal()

	badSchema := validRequest()
	badSchema.SchemaVersion = "v9"
	badSchemaPayload, _ := badSchema.Marshal()

	annotations := map[string]string{
		AnnotationKey("newer"):  newerPayload,
		AnnotationKey("older"):  olderPayload,
		AnnotationKey("json"):   "{not-json",
		AnnotationKey("schema"): badSchemaPayload,
		"ome.io/deploymentMode": "RawDeployment", // ignored
	}
	valid, malformed := ExtractRequests(annotations)

	if len(valid) != 2 || valid[0].UUID != "older" || valid[1].UUID != "newer" {
		t.Fatalf("want [older newer], got %+v", valid)
	}
	if len(malformed) != 2 {
		t.Fatalf("want 2 malformed, got %+v", malformed)
	}
	if malformed[0].UUID != "json" || malformed[1].UUID != "schema" {
		t.Fatalf("malformed order/content mismatch: %+v", malformed)
	}
	if !errors.Is(malformed[1].Err, ErrUnsupportedSchemaVersion) {
		t.Fatalf("schema malformed should wrap ErrUnsupportedSchemaVersion, got %v", malformed[1].Err)
	}
}

func TestExtractRequestsTiesBreakOnUUID(t *testing.T) {
	a, b := validRequest(), validRequest()
	payloadA, _ := a.Marshal()
	payloadB, _ := b.Marshal()
	valid, _ := ExtractRequests(map[string]string{
		AnnotationKey("bbb"): payloadB,
		AnnotationKey("aaa"): payloadA,
	})
	if len(valid) != 2 || valid[0].UUID != "aaa" || valid[1].UUID != "bbb" {
		t.Fatalf("want UUID tie-break [aaa bbb], got %+v", valid)
	}
}

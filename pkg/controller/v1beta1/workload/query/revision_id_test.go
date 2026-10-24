package query_test

import (
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

func TestRevisionID_SameAndConstructors(t *testing.T) {
	full := query.RevisionFromName("isvc-engine-aaaaaaaa")     // name+hash
	sameName := query.RevisionFromName("isvc-engine-aaaaaaaa") // identical
	diffHash := query.RevisionFromName("isvc-engine-bbbbbbbb")
	hashOnly := query.RevisionFromHash("aaaaaaaa")               // e.g. from a pod
	crossComp := query.RevisionFromName("isvc-decoder-aaaaaaaa") // same hash, other component
	zero := query.RevisionID{}

	cases := []struct {
		name string
		a, b query.RevisionID
		want bool
	}{
		{"full vs full same name", full, sameName, true},
		{"full vs full diff hash", full, diffHash, false},
		{"full vs hash-only same hash", full, hashOnly, true},
		{"hash-only vs hash-only same", hashOnly, query.RevisionFromHash("aaaaaaaa"), true},
		{"cross-component same hash, both named", full, crossComp, false},
		{"zero vs anything", zero, full, false},
		{"anything vs zero", full, zero, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Same(tc.b); got != tc.want {
				t.Fatalf("Same: got %v want %v", got, tc.want)
			}
		})
	}

	if full.Name() != "isvc-engine-aaaaaaaa" || full.Hash() != "aaaaaaaa" {
		t.Fatalf("RevisionFromName accessors: name=%q hash=%q", full.Name(), full.Hash())
	}
	if !zero.IsZero() || full.IsZero() {
		t.Fatalf("IsZero wrong: zero=%v full=%v", zero.IsZero(), full.IsZero())
	}
	cr := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: "isvc-engine-aaaaaaaa"}}
	if !query.RevisionOf(cr).Same(full) {
		t.Fatalf("RevisionOf(cr) must equal RevisionFromName")
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{query.LabelRevisionHash: "aaaaaaaa"}}}
	if !query.RevisionFromPod(pod).Same(full) {
		t.Fatalf("RevisionFromPod must match by hash")
	}
	if !query.RevisionOf(nil).IsZero() || !query.RevisionFromPod(nil).IsZero() {
		t.Fatalf("nil inputs must yield zero RevisionID")
	}
}

func TestRequireNamedRevision_RequiresAControllerRevisionReference(t *testing.T) {
	for _, tc := range []struct {
		name     string
		revision query.RevisionID
		known    bool
	}{
		{name: "named version", revision: query.RevisionFromName("example-engine-0123abcd"), known: true},
		{name: "pod hash alone", revision: query.RevisionFromHash("0123abcd")},
		{name: "zero identity"},
		{name: "name has no hash", revision: query.RevisionFromName("example-engine-")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := query.RequireNamedRevision(tc.revision)
			if (err == nil) != tc.known || (!tc.known && !errors.Is(err, query.ErrRevisionUnknown)) {
				t.Fatalf("error = %v, want known identity=%t", err, tc.known)
			}
		})
	}
}

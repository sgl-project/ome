package runtimerevision

import "testing"

func TestName_ClusterScope(t *testing.T) {
	got := Name(KindClusterServingRuntime, "", "srt-llama-pd", "abc12345")
	want := "cr-srt-llama-pd-abc12345"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestName_NamespacedScope(t *testing.T) {
	got := Name(KindServingRuntime, "team-a", "srt-foo", "abc12345")
	want := "r-team-a-srt-foo-abc12345"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestName_NoCollisionAcrossNamespaces(t *testing.T) {
	// Same SR name in two namespaces hashes to (different content
	// most likely, but even with identical content) must produce
	// different revision names so they don't clash in the OME ns.
	a := Name(KindServingRuntime, "team-a", "srt-foo", "abc12345")
	b := Name(KindServingRuntime, "team-b", "srt-foo", "abc12345")
	if a == b {
		t.Fatalf("name collision across namespaces: both %q", a)
	}
}

func TestName_UnknownKindIsDistinct(t *testing.T) {
	got := Name(SourceKind("Bogus"), "team-a", "srt-foo", "abc12345")
	if got == Name(KindClusterServingRuntime, "team-a", "srt-foo", "abc12345") {
		t.Fatal("unknown kind silently aliased to cluster name shape")
	}
}

func TestMatchesRuntime_Cluster(t *testing.T) {
	cases := []struct {
		name string
		rev  string
		want bool
	}{
		{"happy", "cr-srt-llama-pd-abc12345", true},
		{"wrong runtime", "cr-srt-other-abc12345", false},
		{"wrong scope prefix", "r-default-srt-llama-pd-abc12345", false},
		{"missing hash", "cr-srt-llama-pd-", false},
		{"short hash", "cr-srt-llama-pd-abc", false},
		{"long hash", "cr-srt-llama-pd-abc1234567890", false},
		{"uppercase hex rejected", "cr-srt-llama-pd-ABC12345", false},
		{"non-hex chars rejected", "cr-srt-llama-pd-zzzzzzzz", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchesRuntime(tc.rev, KindClusterServingRuntime, "", "srt-llama-pd")
			if got != tc.want {
				t.Errorf("MatchesRuntime(%q) = %v, want %v", tc.rev, got, tc.want)
			}
		})
	}
}

func TestMatchesRuntime_Namespaced(t *testing.T) {
	cases := []struct {
		name string
		rev  string
		want bool
	}{
		{"happy", "r-team-a-srt-foo-abc12345", true},
		{"wrong namespace", "r-team-b-srt-foo-abc12345", false},
		{"wrong runtime", "r-team-a-srt-other-abc12345", false},
		{"cluster prefix rejected", "cr-srt-foo-abc12345", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchesRuntime(tc.rev, KindServingRuntime, "team-a", "srt-foo")
			if got != tc.want {
				t.Errorf("MatchesRuntime(%q) = %v, want %v", tc.rev, got, tc.want)
			}
		})
	}
}

func TestMatchesRuntime_UnknownKind(t *testing.T) {
	if MatchesRuntime("cr-srt-llama-pd-abc12345", SourceKind("Bogus"), "", "srt-llama-pd") {
		t.Fatal("MatchesRuntime should reject unknown kind")
	}
}

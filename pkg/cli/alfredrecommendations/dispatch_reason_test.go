package alfredrecommendations

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestEveryDispatchReasonProjects checks the projector accepts every dispatch
// reason Alfred's dispatcher can write. A reason missing from the whitelist
// would drop its whole row from the report.
func TestEveryDispatchReasonProjects(t *testing.T) {
	files, err := filepath.Glob("../../alfred/engine/dispatch*.go")
	if err != nil {
		t.Fatal(err)
	}
	written := regexp.MustCompile(`(?:fmt\.Errorf\(|return empty, |withholdDecisions\([a-z]+, |DispatchReason = |e\.Reason = )"([A-Z][A-Za-z]+)"`)
	reasons := map[string]bool{}
	for _, file := range files {
		if matched, _ := filepath.Match("*_test.go", filepath.Base(file)); matched {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range written.FindAllSubmatch(source, -1) {
			reasons[string(m[1])] = true
		}
	}
	if len(reasons) < 20 {
		t.Fatalf("found only %d dispatch reasons; the scan no longer matches the dispatcher: %v", len(reasons), reasons)
	}
	for reason := range reasons {
		if !validDispatchReason(reason) {
			t.Errorf("dispatch reason %q is not projected", reason)
		}
	}
}

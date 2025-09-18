package xet

import (
	"testing"
)

func TestProgressPhaseString(t *testing.T) {
	cases := map[ProgressPhase]string{
		ProgressPhaseScanning:    "scanning",
		ProgressPhaseDownloading: "downloading",
		ProgressPhaseFinalizing:  "finalizing",
		ProgressPhase(42):        "unknown",
	}

	for phase, expected := range cases {
		if phase.String() != expected {
			t.Fatalf("expected %q for phase %v, got %q", expected, phase, phase.String())
		}
	}
}

func TestProgressHelpersNilClient(t *testing.T) {
	var c *Client
	if err := c.EnableConsoleProgress("test", 0); err == nil {
		t.Fatal("expected error when enabling progress on nil client")
	}

	if err := c.DisableProgress(); err == nil {
		t.Fatal("expected error when disabling progress on nil client")
	}

	empty := &Client{}
	if err := empty.EnableConsoleProgress("test", 0); err == nil {
		t.Fatal("expected error when enabling progress on uninitialized client")
	}

	if err := empty.DisableProgress(); err == nil {
		t.Fatal("expected error when disabling progress on uninitialized client")
	}
}

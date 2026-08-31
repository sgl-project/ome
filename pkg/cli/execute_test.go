package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli/exitcode"
)

func TestExecuteCommandMapsErrorsAndPrintsOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "success", want: exitcode.Success},
		{name: "ordinary", err: errors.New("ordinary failure"), want: exitcode.GeneralError},
		{name: "assertion", err: &exitcode.UnmetAssertionError{Err: errors.New("not ready")}, want: exitcode.AssertionUnmet},
		{name: "conflict", err: &exitcode.PreconditionError{Err: errors.New("stale target")}, want: exitcode.MutationConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(*cobra.Command, []string) error {
					return test.err
				},
			}
			cmd.SetErr(&stderr)

			if got := ExecuteCommand(cmd, &stderr); got != test.want {
				t.Fatalf("ExecuteCommand() = %d, want %d", got, test.want)
			}
			if test.err == nil {
				if stderr.Len() != 0 {
					t.Fatalf("stderr = %q, want empty", stderr.String())
				}
				return
			}
			want := "error: " + test.err.Error() + "\n"
			if stderr.String() != want {
				t.Fatalf("stderr = %q, want %q", stderr.String(), want)
			}
		})
	}
}

func TestRunUsesRootAndProvidedStreams(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	streams := genericiooptions.IOStreams{
		In:     strings.NewReader(""),
		Out:    &stdout,
		ErrOut: &stderr,
	}
	if got := Run([]string{"--help"}, streams); got != exitcode.Success {
		t.Fatalf("Run(--help) = %d, want %d; stderr=%q", got, exitcode.Success, stderr.String())
	}
	if !strings.Contains(stdout.String(), "kubectl ome <command>") || stderr.Len() != 0 {
		t.Fatalf("Run(--help) stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunReturnsGeneralErrorForBadInvocation(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	streams := genericiooptions.IOStreams{
		In:     strings.NewReader(""),
		Out:    &bytes.Buffer{},
		ErrOut: &stderr,
	}
	if got := Run([]string{"not-a-command"}, streams); got != exitcode.GeneralError {
		t.Fatalf("Run(bad command) = %d, want %d", got, exitcode.GeneralError)
	}
	if !strings.Contains(stderr.String(), "error: unknown command") {
		t.Fatalf("stderr = %q, want unknown-command diagnostic", stderr.String())
	}
}

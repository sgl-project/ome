// Package printers renders CLI output. Human-readable tables adapt to terminal
// width and preserve the historical tabwriter layout when redirected;
// -o json|yaml passes API objects through unmodified.
package printers

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"sigs.k8s.io/yaml"
)

type Table struct {
	Headers []string
	Rows    [][]string
}

func (t Table) Write(w io.Writer) error {
	return writeTable(w, t.Headers, t.Rows, false)
}

// WriteSanitized writes the table while escaping control characters even when
// output is redirected. Interactive terminal output is always sanitized.
func (t Table) WriteSanitized(w io.Writer) error {
	return writeTable(w, t.Headers, t.Rows, true)
}

// PrintObj writes obj to w in the requested format ("json" or "yaml").
func PrintObj(obj runtime.Object, format string, w io.Writer) error {
	var data []byte
	var err error
	switch format {
	case "json":
		data, err = json.MarshalIndent(obj, "", "  ")
		if err == nil {
			data = append(data, '\n')
		}
	case "yaml":
		data, err = yaml.Marshal(obj)
	default:
		return fmt.Errorf("unsupported output format %q (supported: json, yaml)", format)
	}
	if err != nil {
		return err
	}
	written, err := w.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

// Age renders a creation timestamp the way kubectl does (41d, 10m, ...).
func Age(t metav1.Time) string {
	if t.IsZero() {
		return "-"
	}
	return duration.HumanDuration(time.Since(t.Time))
}

// OrDash substitutes "-" for empty values so table cells never collapse.
func OrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

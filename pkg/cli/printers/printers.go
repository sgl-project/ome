// Package printers renders CLI output. Human-readable tables use tabwriter;
// -o json|yaml passes API objects through unmodified.
package printers

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
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
	tw := tabwriter.NewWriter(w, 0, 8, 3, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(t.Headers, "\t")); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// PrintObj writes obj to w in the requested format ("json" or "yaml").
func PrintObj(obj runtime.Object, format string, w io.Writer) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(obj)
	case "yaml":
		b, err := yaml.Marshal(obj)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	default:
		return fmt.Errorf("unsupported output format %q (supported: json, yaml)", format)
	}
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

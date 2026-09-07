package report

import (
	"io"

	"sigs.k8s.io/ome/pkg/cli/printers"
)

// Table is the deterministic human-readable view of a typed report.
type Table struct {
	Headers []string
	Rows    [][]string
}

// Write renders the table and returns all underlying writer errors.
func (t Table) Write(w io.Writer) error {
	return (printers.Table{Headers: t.Headers, Rows: t.Rows}).WriteSanitized(w)
}

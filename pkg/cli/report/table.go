package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode"
)

// Table is the deterministic human-readable view of a typed report.
type Table struct {
	Headers []string
	Rows    [][]string
}

// Write renders the table and returns all underlying writer errors.
func (t Table) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 8, 3, ' ', 0)
	if _, err := fmt.Fprintln(tw, joinCells(t.Headers)); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if _, err := fmt.Fprintln(tw, joinCells(row)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func joinCells(cells []string) string {
	normalized := make([]string, len(cells))
	for i, cell := range cells {
		normalized[i] = sanitizeCell(cell)
	}
	return strings.Join(normalized, "\t")
}

func sanitizeCell(value string) string {
	var result strings.Builder
	for _, char := range value {
		switch char {
		case '\t':
			result.WriteString(`\t`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		default:
			if unicode.IsControl(char) {
				fmt.Fprintf(&result, `\u%04x`, char)
				continue
			}
			result.WriteRune(char)
		}
	}
	return result.String()
}

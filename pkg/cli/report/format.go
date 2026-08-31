// Package report defines deterministic, versioned diagnostic output helpers.
// Raw Kubernetes API object output remains owned by pkg/cli/printers.
package report

import "fmt"

// Format selects the representation used for a diagnostic report.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// ParseFormat validates a diagnostic output format. An empty value selects
// the human-readable table format used by diagnostic commands by default.
func ParseFormat(value string) (Format, error) {
	switch Format(value) {
	case "", FormatTable:
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatYAML:
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (supported: table, json, yaml)", value)
	}
}

package report

import (
	"encoding/json"
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
)

// Document is implemented by typed reports that can return a deterministic
// copy and derive a human table from that same typed value.
type Document[T any] interface {
	Canonical() T
	Table() Table
}

// Write renders a typed report in table, JSON, or YAML form. Serialization
// and destination writer errors are returned to the command layer.
func Write[T Document[T]](w io.Writer, format Format, document T) error {
	parsedFormat, err := ParseFormat(string(format))
	if err != nil {
		return err
	}
	format = parsedFormat

	canonical := document.Canonical()
	switch format {
	case FormatTable:
		if err := canonical.Table().Write(w); err != nil {
			return fmt.Errorf("write report table: %w", err)
		}
		return nil
	case FormatJSON:
		data, err := json.MarshalIndent(canonical, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal report json: %w", err)
		}
		data = append(data, '\n')
		if err := writeBytes(w, data); err != nil {
			return fmt.Errorf("write report json: %w", err)
		}
		return nil
	case FormatYAML:
		data, err := yaml.Marshal(canonical)
		if err != nil {
			return fmt.Errorf("marshal report yaml: %w", err)
		}
		if err := writeBytes(w, data); err != nil {
			return fmt.Errorf("write report yaml: %w", err)
		}
		return nil
	default:
		panic("unreachable output format")
	}
}

func writeBytes(w io.Writer, data []byte) error {
	written, err := w.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

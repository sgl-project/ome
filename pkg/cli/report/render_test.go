package report_test

import (
	"bytes"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/ome/pkg/cli/report"
	"sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
)

func TestWriteTableUsesCanonicalTypedContent(t *testing.T) {
	document := reportEnvelope([]string{"z", "a"})
	var output bytes.Buffer

	require.NoError(t, report.Write(&output, report.FormatTable, document))
	assert.Equal(t, "NAME\na\nz\n", output.String())
	assert.Equal(t, []string{"z", "a"}, document.Content.Rows)
}

func TestWriteMachineFormatsIgnoreTerminalWidth(t *testing.T) {
	document := reportEnvelope([]string{"a value that is deliberately wider than a tiny terminal"})

	for _, format := range []report.Format{report.FormatJSON, report.FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			var regular bytes.Buffer
			narrow := &narrowTerminalBuffer{width: 12}

			require.NoError(t, report.Write(&regular, format, document))
			require.NoError(t, report.Write(narrow, format, document))
			assert.Equal(t, regular.String(), narrow.String())
		})
	}
}

func TestWriteEmptyFormatUsesDefaultTable(t *testing.T) {
	var output bytes.Buffer

	require.NotPanics(t, func() {
		require.NoError(t, report.Write(&output, report.Format(""), reportEnvelope([]string{"a"})))
	})
	assert.Equal(t, "NAME\na\n", output.String())
}

func TestWriteJSONIsDeterministicAndKeepsEmptySlices(t *testing.T) {
	document := reportEnvelope([]string{})
	document.Sources = []v1alpha1.SourceReference{
		{Kind: "Pod", Namespace: "prod", Name: "z", Evidence: v1alpha1.EvidenceObserved},
		{Kind: "Pod", Namespace: "prod", Name: "a", Evidence: v1alpha1.EvidenceObserved},
	}
	document.Warnings = nil
	var first bytes.Buffer
	var second bytes.Buffer

	require.NoError(t, report.Write(&first, report.FormatJSON, document))
	require.NoError(t, report.Write(&second, report.FormatJSON, document))

	assert.Equal(t, first.String(), second.String())
	assert.Contains(t, first.String(), `"apiVersion": "cli.ome.io/v1alpha1"`)
	assert.Contains(t, first.String(), `"sources": [`)
	assert.Contains(t, first.String(), `"rows": []`)
	assert.Contains(t, first.String(), `"warnings": []`)
	assert.Less(t, strings.Index(first.String(), `"name": "a"`), strings.Index(first.String(), `"name": "z"`))
	assert.Equal(t, "z", document.Sources[0].Name)
}

func TestWriteYAMLIsDeterministicAndKeepsEmptySlices(t *testing.T) {
	document := reportEnvelope(nil)
	document.Sources = nil
	document.Warnings = nil
	var first bytes.Buffer
	var second bytes.Buffer

	require.NoError(t, report.Write(&first, report.FormatYAML, document))
	require.NoError(t, report.Write(&second, report.FormatYAML, document))

	assert.Equal(t, first.String(), second.String())
	assert.Contains(t, first.String(), "apiVersion: cli.ome.io/v1alpha1\n")
	assert.Contains(t, first.String(), "sources: []\n")
	assert.Contains(t, first.String(), "rows: []\n")
	assert.Contains(t, first.String(), "warnings: []\n")
}

func TestWriteRejectsUnsupportedFormat(t *testing.T) {
	err := report.Write(&bytes.Buffer{}, report.Format("wide"), reportEnvelope(nil))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "table, json, yaml")
}

func TestWriteReturnsSerializationErrors(t *testing.T) {
	for _, format := range []report.Format{report.FormatJSON, report.FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			err := report.Write(&bytes.Buffer{}, format, unsupportedDocument{Value: make(chan int)})

			require.Error(t, err)
			assert.Contains(t, err.Error(), string(format))
		})
	}
}

func TestWriteReturnsOutputErrors(t *testing.T) {
	wantErr := errors.New("destination closed")
	for _, format := range []report.Format{report.FormatTable, report.FormatJSON, report.FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			err := report.Write(errorWriter{err: wantErr}, format, reportEnvelope(nil))

			require.Error(t, err)
			assert.ErrorIs(t, err, wantErr)
		})
	}
}

func TestWriteReturnsShortWriteErrors(t *testing.T) {
	for _, format := range []report.Format{report.FormatJSON, report.FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			err := report.Write(shortWriter{}, format, reportEnvelope(nil))

			require.Error(t, err)
			assert.ErrorIs(t, err, io.ErrShortWrite)
		})
	}
}

type rowsContent struct {
	Rows []string `json:"rows"`
}

func (c rowsContent) Canonical() rowsContent {
	result := c
	result.Rows = append([]string{}, c.Rows...)
	sort.Strings(result.Rows)
	return result
}

func (c rowsContent) Table() report.Table {
	table := report.Table{Headers: []string{"NAME"}, Rows: make([][]string, 0, len(c.Rows))}
	for _, row := range c.Rows {
		table.Rows = append(table.Rows, []string{row})
	}
	return table
}

func reportEnvelope(rows []string) v1alpha1.Envelope[rowsContent] {
	return v1alpha1.Envelope[rowsContent]{
		Kind:        "InstanceReport",
		Metadata:    v1alpha1.Metadata{Namespace: "prod", Name: "chat"},
		CollectedAt: time.Date(2026, time.August, 31, 18, 0, 0, 0, time.UTC),
		Content:     rowsContent{Rows: rows},
	}
}

type unsupportedDocument struct {
	Value chan int `json:"value"`
}

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) {
	return len(value) - 1, nil
}

func (d unsupportedDocument) Canonical() unsupportedDocument {
	return d
}

type narrowTerminalBuffer struct {
	bytes.Buffer
	width int
}

func (w *narrowTerminalBuffer) TerminalWidth() (int, bool) {
	return w.width, true
}

func (unsupportedDocument) Table() report.Table {
	return report.Table{}
}

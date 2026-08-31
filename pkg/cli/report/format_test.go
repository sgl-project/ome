package report_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/ome/pkg/cli/report"
)

func TestParseFormatAcceptsSupportedValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  report.Format
	}{
		{name: "default", value: "", want: report.FormatTable},
		{name: "table", value: "table", want: report.FormatTable},
		{name: "json", value: "json", want: report.FormatJSON},
		{name: "yaml", value: "yaml", want: report.FormatYAML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := report.ParseFormat(tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFormatRejectsUnsupportedValues(t *testing.T) {
	_, err := report.ParseFormat("wide")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table, json, yaml")
}

func TestTableWriteProducesStableColumns(t *testing.T) {
	var output bytes.Buffer
	table := report.Table{
		Headers: []string{"NAME", "STATE"},
		Rows: [][]string{
			{"short", "Ready"},
			{"a-longer-name", "Unavailable"},
		},
	}

	require.NoError(t, table.Write(&output))
	assert.Equal(t, "NAME            STATE\nshort           Ready\n"+
		"a-longer-name   Unavailable\n", output.String())
}

func TestTableWriteReturnsOutputErrors(t *testing.T) {
	wantErr := errors.New("write failed")
	err := (report.Table{Headers: []string{"NAME"}}).Write(errorWriter{err: wantErr})

	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestTableWriteEscapesStructureControlCharactersWithoutMutation(t *testing.T) {
	var output bytes.Buffer
	table := report.Table{
		Headers: []string{"NA\tME"},
		Rows:    [][]string{{"chat\nprod\r\x01"}},
	}

	require.NoError(t, table.Write(&output))
	assert.Equal(t, "NA\\tME\nchat\\nprod\\r\\u0001\n", output.String())
	assert.Equal(t, "NA\tME", table.Headers[0])
	assert.Equal(t, "chat\nprod\r\x01", table.Rows[0][0])
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

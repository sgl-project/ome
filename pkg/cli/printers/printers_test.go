package printers

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTableAlignsColumns(t *testing.T) {
	var buf bytes.Buffer
	table := Table{
		Headers: []string{"NAME", "READY"},
		Rows:    [][]string{{"llama-70b", "True"}, {"m", "False"}},
	}
	require.NoError(t, table.Write(&buf))
	assert.Equal(t, "NAME        READY\nllama-70b   True\nm           False\n", buf.String())
}

func TestPrintObjJSONRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p"}}
	require.NoError(t, PrintObj(pod, "json", &buf))
	assert.Contains(t, buf.String(), `"name": "p"`)
}

func TestPrintObjRejectsUnknownFormat(t *testing.T) {
	assert.Error(t, PrintObj(&corev1.Pod{}, "toml", &bytes.Buffer{}))
}

func TestAge(t *testing.T) {
	assert.Equal(t, "10m", Age(metav1.NewTime(time.Now().Add(-10*time.Minute))))
}

func TestOrDash(t *testing.T) {
	assert.Equal(t, "-", OrDash(""))
	assert.Equal(t, "x", OrDash("x"))
}

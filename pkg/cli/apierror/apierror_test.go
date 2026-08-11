package apierror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var gr = schema.GroupResource{Group: "ome.io", Resource: "inferenceservices"}

func TestFriendlyMissingCRD(t *testing.T) {
	raw := kerrors.NewGenericServerResponse(404, "get", gr, "", "", 0, true)
	err := Friendly(raw)
	assert.Contains(t, err.Error(), "OME does not appear to be installed")
	assert.True(t, errors.Is(err, raw), "must wrap, not replace")
}

func TestFriendlyObjectNotFoundPassesThrough(t *testing.T) {
	raw := kerrors.NewNotFound(gr, "missing")
	assert.Equal(t, error(raw), Friendly(raw))
}

func TestFriendlyNilAndOther(t *testing.T) {
	assert.NoError(t, Friendly(nil))
	other := errors.New("boom")
	assert.Equal(t, other, Friendly(other))
}

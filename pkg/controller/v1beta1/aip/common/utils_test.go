package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

// TestUtils_GenerateId tests GenerateId
func TestUtils_GenerateId(t *testing.T) {
	uid := types.UID("e89674fe-af27-4fdd-91ed-34087115d191")
	projectId := GenerateId("proj_", uid)
	assert.Equal(t, "proj_ZTg5Njc0ZmUtYWYyNy00ZmRkLTkxZWQtMzQwODcxMTVkMTkx", projectId)
}

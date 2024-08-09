package ginlog

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetOrCreateRequestID(t *testing.T) {
	t.Run("if no request ID is present, then one should be created", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err, "should not error creating request")

		c := &gin.Context{Request: r}

		_, ok := c.Get(RequestIDKey)
		assert.False(t, ok, "should not have request ID")

		id := GetOrCreateRequestID(c)
		assert.NotEmpty(t, id, "request ID should not be empty")
	})

	t.Run("if request ID is present in header, then it should be used", func(t *testing.T) {
		r, err := http.NewRequest("GET", "/", nil)
		assert.NoError(t, err, "should not error creating request")

		r.Header.Add(RequestIDHeader, "test")
		c := &gin.Context{Request: r}

		id := GetOrCreateRequestID(c)
		assert.Equal(t, "test", id)
	})

	t.Run("if request ID is on context, then it should be used", func(t *testing.T) {
		c := &gin.Context{}
		c.Set(RequestIDKey, "test")

		id := GetOrCreateRequestID(c)
		assert.Equal(t, "test", id)
	})
}

package ginlog

import (
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
)

// GetOrCreateRequestID returns the request ID of the supplied Gin context. If a
// request ID doesn't exist, then it creates and sets one.
func GetOrCreateRequestID(ctx *gin.Context) string {
	var requestID string
	if id, ok := ctx.Get(RequestIDKey); ok {
		requestID = id.(string)
	} else {
		if requestID = ctx.GetHeader(RequestIDHeader); requestID == "" {
			requestID = uuid.NewV4().String()
		}
		ctx.Set(RequestIDKey, requestID)
	}

	return requestID
}

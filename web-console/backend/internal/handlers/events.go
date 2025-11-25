package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
)

// defaultAllowedOrigins is used when CORS_ALLOWED_ORIGINS is not set
var defaultAllowedOrigins = []string{"http://localhost:3000", "http://localhost:3001"}

// getAllowedOrigins returns the list of allowed CORS origins
func getAllowedOrigins() []string {
	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		return strings.Split(origins, ",")
	}
	return defaultAllowedOrigins
}

// isOriginAllowed checks if the given origin is in the allowed list
func isOriginAllowed(origin string) bool {
	for _, allowed := range getAllowedOrigins() {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

// EventsHandler handles SSE connections for real-time updates
type EventsHandler struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(k8sClient *k8s.Client, logger *zap.Logger) *EventsHandler {
	return &EventsHandler{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// Stream handles SSE connections for real-time Kubernetes resource updates
func (h *EventsHandler) Stream(c *gin.Context) {
	// Validate and set CORS origin header
	origin := c.GetHeader("Origin")
	if origin != "" && isOriginAllowed(origin) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// Subscribe to events
	eventChan := h.k8sClient.Broadcaster.Subscribe()
	defer h.k8sClient.Broadcaster.Unsubscribe(eventChan)

	clientIP := c.ClientIP()
	h.logger.Info("SSE client connected", zap.String("client_ip", clientIP))

	// Send initial connection confirmation
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"message\": \"Connected to event stream\"}\n\n")
	c.Writer.Flush()

	// Create a ticker for periodic keep-alive messages
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Listen for events and send to client
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Channel closed, client disconnected
				h.logger.Info("SSE event channel closed", zap.String("client_ip", clientIP))
				return
			}

			// Marshal event to JSON
			data, err := json.Marshal(event)
			if err != nil {
				h.logger.Error("Failed to marshal event", zap.Error(err))
				continue
			}

			// Send event to client
			// Format: event: <event-type>\ndata: <json-data>\n\n
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, data)
			c.Writer.Flush()

		case <-ticker.C:
			// Send keep-alive ping
			fmt.Fprintf(c.Writer, "event: ping\ndata: {\"timestamp\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
			c.Writer.Flush()

		case <-c.Request.Context().Done():
			// Client disconnected
			h.logger.Info("SSE client disconnected", zap.String("client_ip", clientIP))
			return
		}
	}
}

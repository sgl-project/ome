package examples

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var projectRateLimitKeyLog = logrus.WithFields(logrus.Fields{
	"component": "project-rate-limit-example",
})

// formatProjectRateLimitList returns a clean string representation of a project rate limit list
func formatProjectRateLimitList(al *openaisdk.ProjectRateLimitListResponse) string {
	var projectRateLimits []map[string]interface{}
	for _, a := range al.Data {
		projectRateLimits = append(projectRateLimits, map[string]interface{}{
			"id":                           a.ID,
			"model":                        a.Model,
			"max_requests_per_1_minute":    a.MaxRequestsPerOneMinute,
			"max_tokens_per_1_minute":      a.MaxTokensPerOneMinute,
			"batch_1_day_max_input_tokens": a.BatchOneDayMaxInputTokens,
		})
	}

	fields := map[string]interface{}{
		"count":               len(al.Data),
		"project_rate_limits": projectRateLimits,
		"has_more":            al.HasMore,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

// formatProjectRateLimit returns a clean string representation of a project rate limit
func formatProjectRateLimit(a *openaisdk.ProjectRateLimit) string {
	fields := map[string]interface{}{
		"id":                           a.ID,
		"model":                        a.Model,
		"max_requests_per_1_minute":    a.MaxRequestsPerOneMinute,
		"max_tokens_per_1_minute":      a.MaxTokensPerOneMinute,
		"batch_1_day_max_input_tokens": a.BatchOneDayMaxInputTokens,
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

func ProjectRateLimitExample() {
	client := openaisdk.NewClient(option.WithAPIKey("api-admin-key"))
	ctx := context.Background()

	// List project rate limits
	projectRateLimits, err := client.ProjectRateLimits.List(ctx, "project-id")
	if err != nil {
		projectRateLimitKeyLog.WithError(err).Error("Failed to list project rate limits")
		return
	}
	projectRateLimitKeyLog.Info(formatProjectRateLimitList(projectRateLimits))

	// Update project rate limit
	requestBody := `{"max_requests_per_1_minute": 50}` // JSON string
	projectRateLimit, err := client.ProjectRateLimits.Update(ctx, "project-id", "rl-id", option.WithRequestBody("application/json", strings.NewReader(requestBody)))
	if err != nil {
		projectRateLimitKeyLog.WithError(err).Error("Failed to update project rate limit")
		return
	}
	projectRateLimitKeyLog.Info(formatProjectRateLimit(projectRateLimit))
}

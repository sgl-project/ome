package openaisdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/apijson"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
)

// ProjectRateLimitService contains methods for interacting with project rate limits
type ProjectRateLimitService struct {
	Options []option.RequestOption
}

func NewProjectRateLimitService(opts ...option.RequestOption) (r *ProjectRateLimitService) {
	r = &ProjectRateLimitService{}
	r.Options = opts
	return
}

func (r *ProjectRateLimitService) List(ctx context.Context, projectID string, opts ...option.RequestOption) (res *ProjectRateLimitListResponse, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/rate_limits", projectID)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

func (r *ProjectRateLimitService) Update(ctx context.Context, projectID string, rateLimitID string, opts ...option.RequestOption) (res *ProjectRateLimit, err error) {
	opts = append(r.Options[:], opts...)
	if projectID == "" {
		err = errors.New("missing required projectID parameter")
		return
	}
	if rateLimitID == "" {
		err = errors.New("missing required rateLimitID parameter")
		return
	}
	path := fmt.Sprintf("organization/projects/%s/rate_limits/%s", projectID, rateLimitID)
	err = option.ExecuteNewRequest(ctx, http.MethodPost, path, nil, &res, opts...)
	return
}

// ProjectRateLimit represents a project rate limit
type ProjectRateLimit struct {
	// The object type, which is always "project.rate_limit"
	Object string `json:"object,required"`
	// The identifier of the project rate limit
	ID string `json:"id,required"`
	// The model this rate limit applies to
	Model string `json:"model,required"`
	// The maximum requests per minute.
	MaxRequestsPerOneMinute int `json:"max_requests_per_1_minute,required"`
	// The maximum tokens per minute.
	MaxTokensPerOneMinute int `json:"max_tokens_per_1_minute,required"`
	// The maximum images per minute. Only present for relevant models.
	MaxImagesPerOneMinute int `json:"max_images_per_1_minute,required"`
	// The maximum audio megabytes per minute. Only present for relevant models.
	MaxAudioMegabytesPerOneMinute int `json:"max_audio_megabytes_per_1_minute,required"`
	// The maximum requests per day. Only present for relevant models.
	MaxRequestsPerDay int `json:"max_requests_per_1_day,required"`
	// The maximum batch input tokens per day. Only present for relevant models.
	BatchOneDayMaxInputTokens int                  `json:"batch_1_day_max_input_tokens,required"`
	JSON                      projectRateLimitJSON `json:"-"`
}

type projectRateLimitJSON struct {
	Object                        apijson.Field
	ID                            apijson.Field
	Model                         apijson.Field
	MaxRequestsPerOneMinute       apijson.Field
	MaxTokensPerOneMinute         apijson.Field
	MaxImagesPerOneMinute         apijson.Field
	MaxAudioMegabytesPerOneMinute apijson.Field
	MaxRequestsPerDay             apijson.Field
	BatchOneDayMaxInputTokens     apijson.Field
	raw                           string
	ExtraFields                   map[string]apijson.Field
}

func (r *ProjectRateLimit) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ProjectRateLimitListResponse struct {
	Object  string                   `json:"object,required"`
	Data    []ProjectRateLimit       `json:"data,required"`
	FirstID string                   `json:"first_id,required"`
	LastID  string                   `json:"last_id,required"`
	HasMore bool                     `json:"has_more,required"`
	Json    projectRateLimitListJSON `json:"-"`
}

type projectRateLimitListJSON struct {
	Object      apijson.Field
	Data        apijson.Field
	FirstID     apijson.Field
	LastID      apijson.Field
	HasMore     apijson.Field
	raw         string
	ExtraFields map[string]apijson.Field
}

func (r *ProjectRateLimitListResponse) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

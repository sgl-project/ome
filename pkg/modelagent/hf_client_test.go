package modelagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveHfRevisionUsesRevisionAndToken(t *testing.T) {
	const token = "hf_test_token"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/api/models/Qwen/Qwen3-8B/revision/feature%2Fbranch", request.URL.EscapedPath())
		assert.Equal(t, "Bearer "+token, request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"sha":"` + testHFCommitSHA + `"}`))
	}))
	defer server.Close()

	sha, err := resolveHfRevisionWithEndpoint(
		context.Background(), "Qwen/Qwen3-8B", "feature/branch", token, server.URL+"/",
	)

	require.NoError(t, err)
	assert.Equal(t, testHFCommitSHA, sha)
}

func TestResolveHfRevisionRejectsInvalidSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"sha":"not-a-commit"}`))
	}))
	defer server.Close()

	_, err := resolveHfRevisionWithEndpoint(
		context.Background(), "Qwen/Qwen3-8B", "main", "", server.URL,
	)

	assert.ErrorContains(t, err, "valid 40-character commit SHA")
}

func TestResolveHfRevisionRejectsUnsafeModelID(t *testing.T) {
	_, err := resolveHfRevisionWithEndpoint(
		context.Background(), "../unsafe/model", "main", "", "https://huggingface.invalid",
	)

	assert.ErrorContains(t, err, "invalid Hugging Face model ID")
}

func TestFetchAttributeFromHfModelMetaData(t *testing.T) {
	tests := []struct {
		name          string
		modelId       string
		attribute     string
		statusCode    int
		expectedValue interface{}
		wantErr       bool
		errMessageStr string
	}{
		{
			name:          "successfully fetch the attribute",
			modelId:       "deepseek-ai/DeepSeek-V3",
			attribute:     "sha",
			statusCode:    200,
			wantErr:       false,
			expectedValue: "e815299b0bcbac849fa540c768ef21845365c9eb",
		},
		{
			name:          "model metadata returned but cannot fetch attribute value",
			modelId:       "deepseek-ai/DeepSeek-V3",
			attribute:     "random",
			statusCode:    200,
			wantErr:       true,
			expectedValue: "",
			errMessageStr: "attribute random not found in JSON of the response",
		},
		{
			name:          "fail to fetch model metadata",
			modelId:       "deepseek-ai/DeepSeek-V3-unknown",
			attribute:     "sha",
			statusCode:    404,
			wantErr:       true,
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createMockHfMetaDataServer(tt.statusCode, tt.attribute, tt.modelId)
			defer server.Close()

			ctx := context.Background()

			value, err := fetchAttributeFromHfModelMetaDataWithEndpoint(ctx, tt.modelId, tt.attribute, server.URL)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMessageStr != "" {
					assert.Contains(t, err.Error(), tt.errMessageStr)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}

func createMockHfMetaDataServer(statusCode int, attribute string, modelId string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "GET" {
			if statusCode == 200 {
				if modelId == "deepseek-ai/DeepSeek-V3" {
					if attribute == "sha" {
						data := map[string]interface{}{
							attribute: "e815299b0bcbac849fa540c768ef21845365c9eb",
						}
						bytes, _ := json.Marshal(data)
						writer.Write(bytes)
					} else {
						// Return valid JSON without the requested attribute
						data := map[string]interface{}{
							"sha": "e815299b0bcbac849fa540c768ef21845365c9eb",
						}
						bytes, _ := json.Marshal(data)
						writer.Write(bytes)
					}
				}
			} else {
				writer.WriteHeader(statusCode)
				writer.Write([]byte("Repository not found"))
			}
		}
	}))

}

func TestHFModelMetaDataUrl_SuccessCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple valid model",
			input:    "bigscience/bloom",
			expected: "https://huggingface.co/api/models/bigscience/bloom",
		},
		{
			name:     "org scoped with hyphens and numbers",
			input:    "openai/clip-vit-base-patch32",
			expected: "https://huggingface.co/api/models/openai/clip-vit-base-patch32",
		},
		{
			name:     "trims surrounding whitespace",
			input:    "   user/model-name   ",
			expected: "https://huggingface.co/api/models/user/model-name",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := hfModelMetaDataUrl(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("hfModelMetaDataUrl(%q) = %q, want %q", tc.input, got, tc.expected)
			}

			if _, err := url.ParseRequestURI(got); err != nil {
				t.Fatalf("resulting URL is not a valid URI: %v (url=%q)", err, got)
			}
		})
	}
}

func TestHFModelMetaDataUrl_ErrorCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		input         string
		wantErrSubstr string
	}{
		{
			name:          "empty input",
			input:         "",
			wantErrSubstr: "no model name has been specified",
		},
		{
			name:          "missing namespace (no slash)",
			input:         "bert-base-uncased",
			wantErrSubstr: "invalid model name",
		},
		{
			name:          "leading slash",
			input:         "/org/model",
			wantErrSubstr: "invalid model name",
		},
		{
			name:          "trailing slash",
			input:         "org/model/",
			wantErrSubstr: "invalid model name",
		},
		{
			name:          "double slash",
			input:         "org//model",
			wantErrSubstr: "invalid model name",
		},
		{
			name:          "whitespace only (becomes empty after trim -> invalid format)",
			input:         "   ",
			wantErrSubstr: "invalid model name",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := hfModelMetaDataUrl(tc.input)
			if err == nil {
				t.Fatalf("expected error for input %q, got nil (url=%q)", tc.input, got)
			}
			if got != "" {
				t.Fatalf("expected empty URL on error, got %q", got)
			}
			if !strings.Contains(err.Error(), tc.wantErrSubstr) {
				t.Fatalf("error %q does not contain expected substring %q", err.Error(), tc.wantErrSubstr)
			}
		})
	}
}

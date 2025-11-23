package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultAPIURL = "https://huggingface.co/api"
	defaultTimeout = 30 * time.Second
)

// Client represents a HuggingFace API client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new HuggingFace API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL: defaultAPIURL,
	}
}

// ModelSearchResult represents a model search result from HuggingFace
type ModelSearchResult struct {
	ID            string      `json:"id"`
	ModelID       string      `json:"modelId"`
	Author        string      `json:"author"`
	SHA           string      `json:"sha"`
	LastModified  string      `json:"lastModified"`
	Private       bool        `json:"private"`
	Gated         interface{} `json:"gated"` // Can be false (bool) or "auto"/"manual" (string)
	Disabled      bool        `json:"disabled"`
	Downloads     int         `json:"downloads"`
	Likes         int         `json:"likes"`
	Tags          []string    `json:"tags"`
	Pipeline      string      `json:"pipeline_tag,omitempty"`
	Library       string      `json:"library_name,omitempty"`
}

// ModelInfo represents detailed model information
type ModelInfo struct {
	ID            string                 `json:"id"`
	ModelID       string                 `json:"modelId"`
	Author        string                 `json:"author"`
	SHA           string                 `json:"sha"`
	LastModified  string                 `json:"lastModified"`
	Private       bool                   `json:"private"`
	Gated         interface{}            `json:"gated"` // Can be false (bool) or "auto"/"manual" (string)
	Disabled      bool                   `json:"disabled"`
	Downloads     int                    `json:"downloads"`
	Likes         int                    `json:"likes"`
	Tags          []string               `json:"tags"`
	Pipeline      string                 `json:"pipeline_tag,omitempty"`
	Library       string                 `json:"library_name,omitempty"`
	Siblings      []FileSibling          `json:"siblings,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
	CardData      map[string]interface{} `json:"cardData,omitempty"`
}

// FileSibling represents a file in the model repository
type FileSibling struct {
	Filename string `json:"rfilename"`
	Size     int64  `json:"size,omitempty"`
}

// ModelConfig represents the parsed config.json from a model
type ModelConfig struct {
	Architectures     []string               `json:"architectures,omitempty"`
	ModelType         string                 `json:"model_type,omitempty"`
	TaskSpecific      map[string]interface{} `json:"task_specific_params,omitempty"`
	MaxPositionEmbed  int                    `json:"max_position_embeddings,omitempty"`
	VocabSize         int                    `json:"vocab_size,omitempty"`
	HiddenSize        int                    `json:"hidden_size,omitempty"`
	NumLayers         int                    `json:"num_hidden_layers,omitempty"`
	NumAttentionHeads int                    `json:"num_attention_heads,omitempty"`
	TorchDType        string                 `json:"torch_dtype,omitempty"`
	Quantization      map[string]interface{} `json:"quantization_config,omitempty"`
}

// SearchModelsParams represents search parameters for model search
type SearchModelsParams struct {
	Query    string   // Search query
	Author   string   // Filter by author
	Filter   string   // Filter by tag/library
	Sort     string   // Sort field (e.g., "downloads", "likes", "lastModified")
	Direction string  // Sort direction ("asc" or "desc")
	Limit    int      // Max number of results
	Tags     []string // Filter by tags
}

// SearchModels searches for models on HuggingFace
func (c *Client) SearchModels(ctx context.Context, params SearchModelsParams) ([]ModelSearchResult, error) {
	endpoint := fmt.Sprintf("%s/models", c.baseURL)

	// Build query parameters
	queryParams := url.Values{}
	if params.Query != "" {
		queryParams.Add("search", params.Query)
	}
	if params.Author != "" {
		queryParams.Add("author", params.Author)
	}
	if params.Filter != "" {
		queryParams.Add("filter", params.Filter)
	}
	if params.Sort != "" {
		queryParams.Add("sort", params.Sort)
	}
	// HuggingFace API expects direction as "-1" (desc) or "1" (asc)
	if params.Direction == "desc" {
		queryParams.Add("direction", "-1")
	} else if params.Direction == "asc" {
		queryParams.Add("direction", "1")
	}
	if params.Limit > 0 {
		queryParams.Add("limit", fmt.Sprintf("%d", params.Limit))
	} else {
		queryParams.Add("limit", "20") // Default limit
	}
	for _, tag := range params.Tags {
		queryParams.Add("tags", tag)
	}

	if len(queryParams) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var results []ModelSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return results, nil
}

// GetModelInfo retrieves detailed information about a specific model
func (c *Client) GetModelInfo(ctx context.Context, modelID string) (*ModelInfo, error) {
	endpoint := fmt.Sprintf("%s/models/%s", c.baseURL, url.PathEscape(modelID))

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var info ModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}

// GetModelConfig retrieves the config.json from a model repository
func (c *Client) GetModelConfig(ctx context.Context, modelID string) (*ModelConfig, error) {
	// HuggingFace raw file URL pattern
	endpoint := fmt.Sprintf("https://huggingface.co/%s/raw/main/config.json", url.PathEscape(modelID))

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("config.json not found or request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var config ModelConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config.json: %w", err)
	}

	return &config, nil
}

// DetectModelFormat detects the model format from file siblings
func DetectModelFormat(siblings []FileSibling) string {
	formatPriority := []struct {
		name      string
		extension string
	}{
		{"safetensors", ".safetensors"},
		{"pytorch", ".bin"},
		{"pytorch", ".pt"},
		{"onnx", ".onnx"},
		{"tensorflow", ".pb"},
		{"tensorflow", ".h5"},
	}

	for _, format := range formatPriority {
		for _, sibling := range siblings {
			if len(sibling.Filename) > len(format.extension) &&
				sibling.Filename[len(sibling.Filename)-len(format.extension):] == format.extension {
				return format.name
			}
		}
	}

	return "unknown"
}

// EstimateModelSize estimates total model size from file siblings
func EstimateModelSize(siblings []FileSibling) int64 {
	var total int64
	for _, sibling := range siblings {
		total += sibling.Size
	}
	return total
}

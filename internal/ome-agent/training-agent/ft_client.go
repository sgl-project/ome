package training_agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/constants"
)

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Client struct {
	BaseURL string
	client  *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		client:  &http.Client{},
	}
}

type FineTuneDetails interface{}

type FTClient interface {
	PostFineTune(payload []byte) (*http.Response, error)

	GetStatus() (*http.Response, error)

	GetTrainingMetrics() (*http.Response, error)

	PostTerminate() (*http.Response, error)
}

func NewFTClient(config *Config) (FTClient, error) {
	client := NewClient(constants.TrainingEndpoint)

	switch config.Runtime {
	case constants.CohereCommand1TrainingSidecar, constants.CohereCommandRTrainingSidecar:
		return newCohereFTClient(config, client), nil
	case constants.PeftTrainingSidecar:
		return newPeftFTClient(config, client), nil
	default:
		return nil, fmt.Errorf("unknown runtime %s", config.Runtime)
	}
}

func NewFineTuneDetails(config *Config) (FineTuneDetails, error) {
	switch config.Runtime {
	case constants.CohereCommand1TrainingSidecar, constants.CohereCommandRTrainingSidecar:
		// set LoraConfig to nil when it is not LoRA strategy
		if config.CohereFineTuneDetails.Strategy != constants.LoraTrainingStrategy {
			config.CohereFineTuneDetails.LoraConfig = nil
		}
		return config.CohereFineTuneDetails, nil
	case constants.PeftTrainingSidecar:
		return config.PeftFineTuneDetails, nil
	default:
		return nil, fmt.Errorf("unknown runtime %s", config.Runtime)
	}
}

func ConvertFTDetailsToJSON(payload FineTuneDetails) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

type DefaultFTClient struct {
	Client
}

func (d *DefaultFTClient) PostFineTune(payload []byte) (*http.Response, error) {
	resp, err := d.client.Post(d.BaseURL+"/finetune", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d *DefaultFTClient) GetStatus() (*http.Response, error) {
	resp, err := d.client.Get(d.BaseURL + "/status")
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DefaultFTClient) GetTrainingMetrics() (*http.Response, error) {
	resp, err := c.client.Get(c.BaseURL + "/metrics")
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d *DefaultFTClient) PostTerminate() (*http.Response, error) {
	resp, err := d.client.Get(d.BaseURL + "/terminate")
	if err != nil {
		return nil, err
	}
	return resp, nil
}

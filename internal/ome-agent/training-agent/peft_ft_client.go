package training_agent

import "net/http"

var (
	_ FTClient = &PeftFTClient{}

	_ FineTuneDetails = &PeftFineTuneDetails{}
)

type PeftFineTuneDetails struct {
	ModelName                      string  `mapstructure:"model_name" json:"model_name"`
	TrainingDataFileName           string  `mapstructure:"train_dataset_file" json:"train_dataset_file"`
	NumTrainEpochs                 int     `mapstructure:"num_train_epochs" json:"num_train_epochs"`
	LearningRate                   float64 `mapstructure:"learning_rate" json:"learning_rate"`
	TrainBatchSize                 int     `mapstructure:"train_batch_size" json:"train_batch_size"`
	EarlyStoppingPatience          int     `mapstructure:"early_stopping_patience" json:"early_stopping_patience"`
	EarlyStoppingThreshold         float64 `mapstructure:"early_stopping_threshold" json:"early_stopping_threshold"`
	LogModelMetricsIntervalInSteps int     `mapstructure:"log_model_metrics_interval_in_steps" json:"log_model_metrics_interval_in_steps"`
	PeftType                       string  `mapstructure:"peft_type" json:"peft_type"`
	LoraR                          int     `mapstructure:"lora_r" json:"lora_r"`
	LoraAlpha                      int     `mapstructure:"lora_alpha" json:"lora_alpha"`
	LoraDropout                    float64 `mapstructure:"lora_dropout" json:"lora_dropout"`
}

type PeftFTClient struct {
	DefaultFTClient
	PeftFineTuneDetails *PeftFineTuneDetails
}

func newPeftFTClient(config *Config, client *Client) *PeftFTClient {
	peftFTClient := &PeftFTClient{}
	peftFTClient.Client = *client
	peftFTClient.PeftFineTuneDetails = config.PeftFineTuneDetails
	return peftFTClient
}

func (d *PeftFTClient) PostTerminate() (*http.Response, error) {
	resp, err := d.client.Post(d.BaseURL+"/terminate", "application/json", nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

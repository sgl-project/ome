package training_agent

import "github.com/sgl-project/sgl-ome/pkg/constants"

type LoraConfig struct {
	LoraR     int `mapstructure:"rank" json:"rank,omitempty"`
	LoraAlpha int `mapstructure:"alpha" json:"alpha,omitempty"`
}

type CohereFineTuneDetails struct {
	Name                     string                     `mapstructure:"name" json:"name"`
	Size                     string                     `mapstructure:"size" json:"size"`
	Strategy                 constants.TrainingStrategy `mapstructure:"strategy" json:"strategy"`
	ServingStrategy          constants.ServingStrategy  `mapstructure:"serving_strategy" json:"serving_strategy,omitempty"`
	TrainEpochs              int                        `mapstructure:"train_epochs" json:"train_epochs"`
	LearningRate             float64                    `mapstructure:"learning_rate" json:"learning_rate"`
	TrainBatchSize           int                        `mapstructure:"train_batch_size" json:"train_batch_size"`
	EarlyStoppingPatience    int                        `mapstructure:"early_stopping_patience" json:"early_stopping_patience"`
	EarlyStoppingThreshold   float64                    `mapstructure:"early_stopping_threshold" json:"early_stopping_threshold"`
	BaseModel                string                     `mapstructure:"base_model" json:"base_model,omitempty"`
	LogTrainStatusEverySteps int                        `mapstructure:"log_train_status_every_steps" json:"log_train_status_every_steps,omitempty"`
	NLastLayers              int                        `mapstructure:"n_last_layers" json:"n_last_layers,omitempty"`
	TensorParallelSize       int                        `mapstructure:"tensor_parallel_size" json:"tensor_parallel_size,omitempty"`
	LoraConfig               *LoraConfig                `mapstructure:"lora_config" json:"lora_config,omitempty"`
}

var (
	_ FTClient = &CohereFTClient{}

	_ FineTuneDetails = &CohereFineTuneDetails{}
)

type CohereFTClient struct {
	DefaultFTClient
	CohereFineTuneDetails *CohereFineTuneDetails
}

func newCohereFTClient(config *Config, client *Client) *CohereFTClient {
	cohereFTClient := &CohereFTClient{}
	cohereFTClient.Client = *client
	cohereFTClient.CohereFineTuneDetails = config.CohereFineTuneDetails
	return cohereFTClient
}

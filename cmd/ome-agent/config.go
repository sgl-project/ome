package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sgl-project/sgl-ome/pkg/configutils"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func configProvider(cli *cobra.Command, module AgentModule) fx.Option {
	return fx.Provide(func() (*viper.Viper, error) {
		v := viper.GetViper()

		v.SetDefault("OME_AGENT", constants.AgentAppName)
		v.SetEnvPrefix(constants.AgentAppName)
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		// Set up specific viper configuration for training agent
		if module.Name() == "training-agent" {
			err := v.BindEnv("runtime", "RUNTIME")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("auth_type", "AUTH_TYPE")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("compartment_id", "COMPARTMENT_ID")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("input_object_store.obo_token", "INPUT_OBJECT_STORE_OBO_TOKEN")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("input_object_store.enable_obo_token", "INPUT_OBJECT_STORE_ENABLE_OBO_TOKEN")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_name", "TRAINING_NAME")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("model_directory", "MODEL_DIRECTORY")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("model.object_name", "MODEL_OBJECT_NAME")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("model.namespace", "MODEL_NAMESPACE")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_metrics.bucket_name", "TRAINING_METRICS_BUCKET_NAME")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_metrics.namespace", "TRAINING_METRICS_NAMESPACE")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_metrics.object_name", "TRAINING_METRICS_OBJECT_NAME")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_data_directory", "TRAINING_DATA_DIRECTORY")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_data.bucket_name", "TRAINING_DATA_BUCKET_NAME")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_data.namespace", "TRAINING_DATA_NAMESPACE")
			if err != nil {
				return nil, err
			}
			err = v.BindEnv("training_data.object_name", "TRAINING_DATA_OBJECT_NAME")
			if err != nil {
				return nil, err
			}

			// Default the sidecar runtime to cohere if not specified
			runtime := v.GetString("runtime")
			if runtime == "" {
				runtime = "cohere"
			}

			err = setRuntimeSpecificConfig(runtime)
			if err != nil {
				return nil, err
			}
		}

		if err := v.BindPFlag("debug", cli.Flags().Lookup("debug")); err != nil {
			panic(err)
		}
		if configFilePath == "" {
			return nil, errors.New("no config file provided")
		}

		if err := configutils.ResolveAndMergeFile(v, configFilePath); err != nil {
			return nil, fmt.Errorf("cannot read config file: %w", err)
		}

		// Fix the issue where viper.UnmarshalKey only uses read config, neglects environment variables
		for _, key := range v.AllKeys() {
			v.Set(key, v.Get(key))
		}
		return v, nil
	})
}

func setRuntimeSpecificConfig(runtime string) error {
	switch runtime {
	case string(Cohere), string(CohereCommandR):
		fmt.Println("Setting cohere FT specific config...")
		return cohereFTConfigProvider(runtime)
	case string(Peft):
		fmt.Println("Setting peft FT specific config ...")
		return peftFTConfigProvider()
	default:
		panic(fmt.Errorf("unknown runtime %s specified for training agent", runtime))
	}
}

func cohereFTConfigProvider(runtime string) error {
	v := viper.GetViper()
	// Cohere FT Hyper-Parameters
	// train_name is the OCID suffix of the model
	err := v.BindEnv("cohere_ft.name", "COHERE_FT_NAME")
	if err != nil {
		return err
	}
	err = v.BindEnv("cohere_ft.size", "COHERE_FT_SIZE")
	if err != nil {
		return err
	}
	err = v.BindEnv("cohere_ft.strategy", "COHERE_FT_STRATEGY")
	if err != nil {
		return err
	}
	err = v.BindEnv("cohere_ft.train_epochs", "COHERE_FT_TRAIN_EPOCHS")
	if err != nil {
		return err
	}
	err = v.BindEnv("cohere_ft.learning_rate", "COHERE_FT_LEARNING_RATE")
	if err != nil {
		return err
	}
	err = v.BindEnv("cohere_ft.train_batch_size", "COHERE_FT_TRAIN_BATCH_SIZE")
	if err != nil {
		return err
	}
	err = v.BindEnv("cohere_ft.early_stopping_patience", "COHERE_FT_EARLY_STOPPING_PATIENCE")
	if err != nil {
		return err
	}
	err = v.BindEnv("cohere_ft.early_stopping_threshold", "COHERE_FT_EARLY_STOPPING_THRESHOLD")
	if err != nil {
		return err
	}

	if runtime == string(Cohere) {
		err = v.BindEnv("cohere_ft.log_train_status_every_steps", "COHERE_FT_LOG_TRAIN_STATUS_EVERY_STEPS")
		if err != nil {
			return err
		}

		if v.Get("cohere_ft.strategy") == "vanilla" {
			err = v.BindEnv("cohere_ft.n_last_layers", "COHERE_FT_N_LAST_LAYERS")
			if err != nil {
				return err
			}
		}
	}
	if runtime == string(CohereCommandR) {
		err = v.BindEnv("cohere_ft.base_model", "COHERE_FT_BASE_MODEL")
		if err != nil {
			return err
		}

		err = v.BindEnv("cohere_ft.serving_strategy", "COHERE_FT_SERVING_STRATEGY")
		if err != nil {
			return err
		}

		err = v.BindEnv("cohere_ft.tensor_parallel_size", "COHERE_FT_TENSOR_PARALLEL_SIZE")
		if err != nil {
			return err
		}

		if v.Get("cohere_ft.strategy") == "tfew" || v.Get("cohere_ft.strategy") == "lora" && v.Get("cohere_ft.tensor_parallel_size") == "1" {
			err = v.BindEnv("zipped_fine_tuned_weight_directory", "ZIPPED_MERGED_MODEL_PATH")
			if err != nil {
				return err
			}
		}

		if v.Get("cohere_ft.strategy") == "lora" {
			err = v.BindEnv("cohere_ft.lora_config.rank", "COHERE_FT_LORA_CONFIG_RANK")
			if err != nil {
				return err
			}
			err = v.BindEnv("cohere_ft.lora_config.alpha", "COHERE_FT_LORA_CONFIG_ALPHA")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func peftFTConfigProvider() error {
	v := viper.GetViper()
	// Peft FT Hyper-Parameters
	err := v.BindEnv("peft_ft.model_name", "PEFT_FT_MODEL_NAME")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.train_dataset_file", "PEFT_FT_TRAIN_DATASET_FILE")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.log_model_metrics_interval_in_steps", "PEFT_FT_LOG_MODEL_METRICS_INTERNAL_IN_STEPS")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.peft_type", "PEFT_FT_PEFT_TYPE")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.lora_r", "PEFT_FT_LORA_R")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.lora_alpha", "PEFT_FT_LORA_ALPHA")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.lora_dropout", "PEFT_FT_LORA_DROPOUT")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.num_train_epochs", "PEFT_FT_NUM_TRAIN_EPOCHS")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.learning_rate", "PEFT_FT_LEARNING_RATE")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.train_batch_size", "PEFT_FT_TRAIN_BATCH_SIZE")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.early_stopping_patience", "PEFT_FT_EARLY_STOPPING_PATIENCE")
	if err != nil {
		return err
	}
	err = v.BindEnv("peft_ft.early_stopping_threshold", "PEFT_FT_EARLY_STOPPING_THRESHOLD")
	if err != nil {
		return err
	}

	return nil
}

package main

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/configutils"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"strings"
)

const (
	KmsConfigViperKeyName             = "kms_config"
	SecretRetrievalConfigViperKeyName = "secret_retrieval_config"
)

func configProvider(cli *cobra.Command) fx.Option {
	return fx.Provide(func() (*viper.Viper, error) {
		v := viper.GetViper()

		v.SetDefault("app_name", appName)
		v.SetDefault(keymanagement.KmsConfigViperKeyNameKey, KmsConfigViperKeyName)
		v.SetDefault(secretretrieval.SecretRetrievalConfigViperKeyNameKey, SecretRetrievalConfigViperKeyName)

		v.SetEnvPrefix(appName)
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		v.BindEnv("model_name", "MODEL_NAME")
		v.BindEnv("auth_type", "AUTH_TYPE")
		v.BindEnv("model_store_directory", "MODEL_STORE_PATH")
		v.BindEnv("local_store_directory", "LOCAL_STORE_PATH")
		v.BindEnv("disable_model_decryption", "DISABLE_MODEL_DECRYPTION")
		v.BindEnv("node_shape_mapping_path", "NODE_SHAPE_MAPPING_PATH")
		v.BindEnv("is_tensorrt_model", "IS_TENSORRT_MODEL")
		
		v.BindEnv("tensorrt_config.tensorrt_llm_version", "TENSORRT_LLM_VERSION")
		// we will not attach TENSORRT_NODE_SHAPE_SHORT to the container yet, since we will get it from IMDS,
		// but just in case in future we'll need it
		v.BindEnv("tensorrt_config.node_shape_short", "TENSORRT_NODE_SHAPE_SHORT")
		v.BindEnv("tensorrt_config.num_of_gpu", "TENSORRT_NUM_OF_GPU")

		v.BindEnv("key_config.key_name", "KEY_NAME")
		v.BindEnv("key_config.compartment_id", "COMPARTMENT_ID")
		v.BindEnv("key_config.enable_key_defined_tag", "ENABLE_KEY_DEFINED_TAG")
		v.BindEnv("key_config.defined_tag_value", "KEY_DEFINED_TAG_VALUE")
		v.BindEnv("secret_config.secret_name", "SECRET_NAME")
		v.BindEnv("secret_config.vault_id", "VAULT_ID")

		v.BindEnv(fmt.Sprintf("%s.%s", KmsConfigViperKeyName, keymanagement.VaultPrefixViperKeyName), "VAULT_PREFIX")
		v.BindEnv(fmt.Sprintf("%s.%s", KmsConfigViperKeyName, keymanagement.KmsCryptoEndpointViperKeyName), "KMS_CRYPTO_ENDPOINT")
		v.BindEnv(fmt.Sprintf("%s.%s", KmsConfigViperKeyName, keymanagement.KmsManagementEndpointViperKeyName), "KMS_MANAGEMENT_ENDPOINT")

		if err := v.BindPFlag("debug", cli.Flags().Lookup("debug")); err != nil {
			panic(err)
		}

		if configFilePath == "" {
			return nil, errors.New("no config file provided")
		}

		if err := configutils.ResolveAndMergeFile(v, configFilePath); err != nil {
			return nil, fmt.Errorf("cannot read config file: %w", err)
		}

		return v, nil
	})
}

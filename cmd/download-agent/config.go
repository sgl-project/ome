package main

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/configutils"
	secretinvault "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_in_vault"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"strings"
)

const (
	TargetSecretInVaultConfigViperKeyName = "target_secret_in_vault_config"
)

func cohereConfigProvider(cli *cobra.Command) fx.Option {
	return fx.Provide(func() (*viper.Viper, error) {
		v := viper.GetViper()

		v.SetDefault("app_name", appName)
		v.SetDefault("vendor", "cohere")
		v.SetDefault("temp_model_store_path", "/opt/model/store/")
		v.SetDefault(secretinvault.SecretInVaultConfigViperKeyNameKey, TargetSecretInVaultConfigViperKeyName)

		v.SetEnvPrefix(appName)
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		v.BindEnv("model_name", "MODEL_NAME")
		v.BindEnv("source_model_encrypted", "SOURCE_MODEL_ENCRYPTED")
		v.BindEnv("auth_type", "AUTH_TYPE")
		v.BindEnv("mode", "MODE")
		v.BindEnv("model_path_config_json", "MODEL_PATH_CONFIG_JSON")

		v.BindEnv("source_object_store_uri.bucket_name", "SOURCE_OBJECT_BUCKET_NAME")
		v.BindEnv("source_object_store_uri.prefix", "SOURCE_OBJECT_PREFIX")
		v.BindEnv("source_object_store_config.compartment_id", "SOURCE_OBJECT_COMPARTMENT_ID")
		v.BindEnv("source_key_config.key_name", "SOURCE_KEY_NAME")
		v.BindEnv("source_key_config.compartment_id", "SOURCE_KEY_SECRET_COMPARTMENT_ID")
		v.BindEnv("source_secret_config.secret_id", "SOURCE_SECRET_ID")
		v.BindEnv("source_secret_config.secret_name", "SOURCE_SECRET_NAME")
		v.BindEnv("source_secret_config.vault_id", "SOURCE_VAULT_ID")
		v.BindEnv("source_kms_config.vault_id", "SOURCE_VAULT_ID")
		v.BindEnv("source_kms_config.vault_prefix", "SOURCE_VAULT_PREFIX")
		v.BindEnv("source_kms_config.kms_crypto_endpoint", "SOURCE_KMS_CRYPTO_ENDPOINT")
		v.BindEnv("source_kms_config.kms_management_endpoint", "SOURCE_KMS_MANAGEMENT_ENDPOINT")

		/* Region override for source object store and source secret store.
		 * No need to set up for target object storage and target secret store for partner download agent
		 * since all partner download agent jobs will just run in the same region as the target object storage and
		 * target secret store where we can just get current `REGION` for target client
		 */
		v.BindEnv("source_object_store_config.region_override", "SOURCE_REGION_OVERRIDE")
		v.BindEnv("source_secret_retrieval_config.region_override", "SOURCE_REGION_OVERRIDE")

		v.BindEnv("target_object_store_uri.bucket_name", "TARGET_OBJECT_BUCKET_NAME")
		v.BindEnv("target_object_store_uri.prefix", "TARGET_OBJECT_PREFIX")
		v.BindEnv("target_object_store_config.compartment_id", "TARGET_OBJECT_COMPARTMENT_ID")
		v.BindEnv("target_key_config.key_name", "TARGET_KEY_NAME")
		v.BindEnv("target_key_config.compartment_id", "TARGET_KEY_SECRET_COMPARTMENT_ID")
		v.BindEnv("target_key_config.enable_key_defined_tag", "TARGET_ENABLE_KEY_DEFINED_TAG")
		v.BindEnv("target_key_config.defined_tag_value", "TARGET_KEY_DEFINED_TAG_VALUE")
		v.BindEnv("target_secret_config.compartment_id", "TARGET_KEY_SECRET_COMPARTMENT_ID")
		v.BindEnv("target_secret_config.secret_name", "TARGET_SECRET_NAME")
		v.BindEnv("target_secret_config.vault_id", "TARGET_VAULT_ID")
		v.BindEnv("target_kms_config.vault_id", "TARGET_VAULT_ID")
		v.BindEnv("target_kms_config.vault_prefix", "TARGET_VAULT_PREFIX")
		v.BindEnv("target_kms_config.kms_crypto_endpoint", "TARGET_KMS_CRYPTO_ENDPOINT")
		v.BindEnv("target_kms_config.kms_management_endpoint", "TARGET_KMS_MANAGEMENT_ENDPOINT")

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

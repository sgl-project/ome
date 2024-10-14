package injection

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type PartnerDownloadAgentConfig struct {
	AuthType                    *principals.AuthenticationType         `mapstructure:"auth_type" validate:"required"`
	SourceModelEncrypted        bool                                   `mapstructure:"source_model_encrypted"`
	SourceCasperConfig          *casper.Config                         `mapstructure:"source_object_store_config" validate:"required"`
	TargetCasperConfig          *casper.Config                         `mapstructure:"target_object_store_config" validate:"required"`
	SourceKMSConfig             *keymanagement.KmsConfig               `mapstructure:"source_kms_config" validate:"required_if=SourceModelEncrypted true"`
	TargetKMSConfig             *keymanagement.KmsConfig               `mapstructure:"target_kms_config" validate:"required"`
	SourceSecretRetrievalConfig *secretretrieval.SecretRetrievalConfig `mapstructure:"source_secret_retrieval_config" validate:"required_if=SourceModelEncrypted true"`
	TargetSecretRetrievalConfig *secretretrieval.SecretRetrievalConfig `mapstructure:"target_secret_retrieval_config" validate:"required"`
}

func PartnerDownloadAgentConfigProvider() fx.Option {
	return fx.Provide(func(v *viper.Viper) *PartnerDownloadAgentConfig {
		config := &PartnerDownloadAgentConfig{}
		if err := v.Unmarshal(config); err != nil {
			panic(fmt.Errorf("error occurred when unmarshalling partner download agent config: %+v", err))
		}

		if err := config.Validate(); err != nil {
			panic(fmt.Errorf("invalid partner download agent config: %+v", err))
		}
		return config
	})
}

func CasperDataStoreListProvider() fx.Option {
	return fx.Provide(
		provideSourceCasperConfig,
		provideTargetCasperConfig,
		casper.ProvideListOfCasperDataStoreWithAppParams,
	)
}

func KmsModuleProvider() fx.Option {
	return fx.Provide(
		provideSourceKMSConfig,
		provideTargetKMSConfig,
		keymanagement.ProvideListOfKmsCryptoWithAppParams,
		keymanagement.ProvideListOfKmsManagementWithAppParams,
	)
}

func SecretRetrievalProvider() fx.Option {
	return fx.Provide(
		provideSourceSecretRetrievalConfig,
		provideTargetSecretRetrievalConfig,
		secretretrieval.ProvideListOfSecretRetrievalWithAppParams,
	)
}

func provideSourceCasperConfig(logger logging.Interface, config *PartnerDownloadAgentConfig) CasperConfigWrapper {
	sourceCasperConfig := config.SourceCasperConfig
	sourceCasperConfig.AnotherLogger = logger
	return CasperConfigWrapper{
		CasperConfig: sourceCasperConfig,
	}
}

func provideTargetCasperConfig(logger logging.Interface, config *PartnerDownloadAgentConfig) CasperConfigWrapper {
	targetCasperConfig := config.TargetCasperConfig
	targetCasperConfig.AnotherLogger = logger
	return CasperConfigWrapper{
		CasperConfig: targetCasperConfig,
	}
}

func provideSourceKMSConfig(logger logging.Interface, config *PartnerDownloadAgentConfig) KMSConfigWrapper {
	var sourceKMSConfig *keymanagement.KmsConfig = nil
	if config.SourceModelEncrypted {
		sourceKMSConfig = config.SourceKMSConfig
		sourceKMSConfig.AnotherLogger = logger
	}
	return KMSConfigWrapper{
		KMSConfig: sourceKMSConfig,
	}
}

func provideTargetKMSConfig(logger logging.Interface, config *PartnerDownloadAgentConfig) KMSConfigWrapper {
	targetKMSConfig := config.TargetKMSConfig
	targetKMSConfig.AnotherLogger = logger
	return KMSConfigWrapper{
		KMSConfig: targetKMSConfig,
	}
}

func provideSourceSecretRetrievalConfig(logger logging.Interface, config *PartnerDownloadAgentConfig) SecretRetrievalConfigWrapper {
	var sourceSecretRetrievalConfig *secretretrieval.SecretRetrievalConfig = nil
	if config.SourceModelEncrypted {
		sourceSecretRetrievalConfig = config.SourceSecretRetrievalConfig
		sourceSecretRetrievalConfig.AnotherLogger = logger
	}
	return SecretRetrievalConfigWrapper{
		SecretRetrievalConfig: sourceSecretRetrievalConfig,
	}
}

func provideTargetSecretRetrievalConfig(logger logging.Interface, config *PartnerDownloadAgentConfig) SecretRetrievalConfigWrapper {
	targetSecretRetrievalConfig := config.TargetSecretRetrievalConfig
	targetSecretRetrievalConfig.AnotherLogger = logger
	return SecretRetrievalConfigWrapper{
		SecretRetrievalConfig: targetSecretRetrievalConfig,
	}
}

func (c *PartnerDownloadAgentConfig) Validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}

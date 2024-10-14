package injection

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type GenericDownloadAgentConfig struct {
	AuthType           *principals.AuthenticationType `mapstructure:"auth_type" validate:"required"`
	SourceCasperConfig *casper.Config                 `mapstructure:"source_object_store_config" validate:"required"`
	TargetCasperConfig *casper.Config                 `mapstructure:"target_object_store_config" validate:"required"`
}

func GenericDownloadAgentConfigProvider() fx.Option {
	return fx.Provide(func(v *viper.Viper) *GenericDownloadAgentConfig {
		config := &GenericDownloadAgentConfig{}
		if err := v.Unmarshal(config); err != nil {
			panic(fmt.Errorf("error occurred when unmarshalling generic download agent config: %+v", err))
		}

		if err := config.Validate(); err != nil {
			panic(fmt.Errorf("invalid generic download agent config: %+v", err))
		}
		return config
	})
}

func CasperDataStoreListProviderForGenericDownloadAgent() fx.Option {
	return fx.Provide(
		provideSourceCasperConfigForGenericDownloadAgent,
		provideTargetCasperConfigForGenericDownloadAgent,
		casper.ProvideListOfCasperDataStoreWithAppParams,
	)
}

func provideSourceCasperConfigForGenericDownloadAgent(logger logging.Interface, config *GenericDownloadAgentConfig) CasperConfigWrapper {
	sourceCasperConfig := config.SourceCasperConfig
	sourceCasperConfig.AnotherLogger = logger
	return CasperConfigWrapper{
		CasperConfig: sourceCasperConfig,
	}
}

func provideTargetCasperConfigForGenericDownloadAgent(logger logging.Interface, config *GenericDownloadAgentConfig) CasperConfigWrapper {
	targetCasperConfig := config.TargetCasperConfig
	targetCasperConfig.AnotherLogger = logger
	return CasperConfigWrapper{
		CasperConfig: targetCasperConfig,
	}
}

func (c *GenericDownloadAgentConfig) Validate() error {
	validate := validator.New()
	// Validate by using go-playground validator
	if err := validate.Struct(c); err != nil {
		return err
	}
	return nil
}

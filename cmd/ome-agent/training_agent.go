package main

import (
	"context"
	"fmt"

	training_agent "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/training-agent"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var cmdTrainingAgent = &cobra.Command{
	Use:   "training-agent",
	Short: "Run OME Training Agent",
	Long:  "OME Training Agent is dedicated for training lifecycle management, training performance metrics store",
	Run:   runTrainingAgent,
}

func runTrainingAgent(cmd *cobra.Command, args []string) {
	app := fx.New(trainingAgentOpts(cmd))
	app.Run()
	err := app.Stop(context.Background())
	if err != nil {
		return
	}
}

func trainingAgentOpts(cli *cobra.Command) fx.Option {
	return fx.Options(
		// Set up all config variables to viper
		configProvider(cli),

		// Inject dependency modules
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		AuthTypeProvider(),
		CasperDataStoreListProvider(),

		// Inject main application module
		training_agent.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, a *training_agent.TrainingAgent, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						go func() {
							a.Start()
							if err := sh.Shutdown(); err != nil {
								l.Error("Failed to shutdown Training Agent", zap.Error(err))
							}
						}()
						return nil
					},
					OnStop: func(ctx context.Context) error {
						return nil
					},
				})
		}),
	)
}

func init() {
	cmdTrainingAgent.Flags().StringVarP(&configFilePath, "config", "c", "", "path to config file")
	cmdTrainingAgent.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

/*CasperConfigWrapper provides CasperConfig to the fx app defined in casper module (from casper pkg).
 * The initialized configuration in this struct will be added to the "casperConfigs" group, further allowing multiple
 * CasperConfig to be injected and managed collectively.
 * More info regarding fx Value Groups can be found: https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
 */
type CasperConfigWrapper struct {
	fx.Out

	CasperConfig *casper.Config `group:"casperConfigs"`
}

func AuthTypeProvider() fx.Option {
	return fx.Provide(func(v *viper.Viper) principals.AuthenticationType {
		var authType principals.AuthenticationType
		if err := v.UnmarshalKey("auth_type", &authType); err != nil {
			panic(fmt.Errorf("error occurred when unmarshalling key auth_type: %+v", err))
		}
		return authType
	})
}

func CasperDataStoreListProvider() fx.Option {
	return fx.Provide(
		provideInputCasperConfig,
		provideOutputCasperConfig,
		casper.ProvideListOfCasperDataStoreWithAppParams,
	)
}

func provideInputCasperConfig(logger logging.Interface, v *viper.Viper, authType principals.AuthenticationType) CasperConfigWrapper {
	inputCasperConfig := &casper.Config{}
	if err := v.UnmarshalKey("input_object_store", inputCasperConfig); err != nil {
		panic(fmt.Errorf("error occurred when unmarshalling key input_object_store: %+v", err))
	}
	inputCasperConfig.AnotherLogger = logger
	inputCasperConfig.Name = training_agent.InputCasperConfigName
	inputCasperConfig.AuthType = &authType
	return CasperConfigWrapper{
		CasperConfig: inputCasperConfig,
	}
}

func provideOutputCasperConfig(logger logging.Interface, authType principals.AuthenticationType) CasperConfigWrapper {
	outputCasperConfig := &casper.Config{
		AnotherLogger: logger,
		Name:          training_agent.OutputCasperConfigName,
		AuthType:      &authType,
	}
	return CasperConfigWrapper{
		CasperConfig: outputCasperConfig,
	}
}

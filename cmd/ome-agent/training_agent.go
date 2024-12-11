package main

import (
	training_agent "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/training-agent"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"context"
	"github.com/spf13/cobra"
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
		casper.CasperDataStoreModule,

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

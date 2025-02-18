package main

import (
	"context"
	"os"

	serving_sidecar "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/serving-sidecar"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var cmdServingSidecar = &cobra.Command{
	Use:   "serving-sidecar",
	Short: "Run OME serving sidecar",
	Long:  "OME Serving sidecar is for assiting some of the ome serving containers",
	Run:   runServingSidecar,
}

func runServingSidecar(cmd *cobra.Command, args []string) {
	app := fx.New(servingSidecarOpts(cmd))
	app.Run()
	err := app.Stop(context.Background())
	if err != nil {
		return
	}
}

func servingSidecarOpts(cli *cobra.Command) fx.Option {
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
		serving_sidecar.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, a *serving_sidecar.ServingSidecar, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						go func() {
							if err := a.Start(); err != nil {
								l.Error("Serving Sidecar encountered an error during Start", zap.Error(err))
								os.Exit(1)
							}
							if err := sh.Shutdown(); err != nil {
								l.Error("Failed to shutdown Serving Sidecar", zap.Error(err))
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
	cmdEnigma.Flags().StringVarP(&configFilePath, "config", "c", "", "path to config file")
	cmdEnigma.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

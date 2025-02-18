package main

import (
	"context"
	"os"

	merged_finetuned_adapter "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/merged-finetuned-adapter"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var cmdMergedFinetunedAdapter = &cobra.Command{
	Use:   "merged-finetuned-adapter",
	Short: "Run OME merged finetuned adapter",
	Long:  "OME merged finetuned adapter is for downloading the merged finetuned weights and prepared for the serving container to consume",
	Run:   runMergedFinetunedAdapter,
}

func runMergedFinetunedAdapter(cmd *cobra.Command, args []string) {
	app := fx.New(mergedFinetunedAdapterOpts(cmd))
	app.Run()
	err := app.Stop(context.Background())
	if err != nil {
		return
	}
}

func mergedFinetunedAdapterOpts(cli *cobra.Command) fx.Option {
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
		merged_finetuned_adapter.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, a *merged_finetuned_adapter.MergedFinetunedAdapter, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						go func() {
							if err := a.Start(); err != nil {
								l.Error("Merged Finetuned Adapter encountered an error during Start", zap.Error(err))
								os.Exit(1)
							}
							if err := sh.Shutdown(); err != nil {
								l.Error("Failed to shutdown Merged Finetuned Adapter", zap.Error(err))
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

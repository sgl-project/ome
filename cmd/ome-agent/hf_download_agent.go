package main

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/hf_download"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"context"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var configFilePath string
var debug bool

var cmdHFDownload = &cobra.Command{
	Use:   "hf-download",
	Short: "Run OME HuggingFace Download Agent",
	Long:  "OME Agent HuggingFace Download Agent is dedicated for downloading any model from HF.",
	Run:   runHFDownload,
}

func runHFDownload(cmd *cobra.Command, args []string) {
	app := fx.New(hfDownloadOpts(cmd))
	app.Run()
	err := app.Stop(context.Background())
	if err != nil {
		return
	}
}

func hfDownloadOpts(cli *cobra.Command) fx.Option {
	return fx.Options(
		// Set up all hf_download agent config variables to viper
		configProvider(cli),

		// Inject dependency modules
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),

		// Inject main application module
		hf_download.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, a *hf_download.HFDownloadAgent, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						go func() {
							err := a.Start()
							if err != nil {
								return
							}
							err = sh.Shutdown()
							if err != nil {
								return
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
	cmdHFDownload.Flags().StringVarP(&configFilePath, "config", "c", "", "path to config file")
	cmdHFDownload.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

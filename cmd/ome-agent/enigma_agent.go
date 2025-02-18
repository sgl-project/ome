package main

import (
	"context"
	"os"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/enigma"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmscrypto"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmsmgm"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmsvault"
	ocisecret "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/secret"
	ocivault "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/vault"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var cmdEnigma = &cobra.Command{
	Use:   "enigma",
	Short: "Run OME Enigma Agent",
	Long:  "OME Agent Enigma is dedicated for model encryption and decryption.",
	Run:   runEnigma,
}

func runEnigma(cmd *cobra.Command, args []string) {
	app := fx.New(enigmaOpts(cmd))
	app.Run()
	err := app.Stop(context.Background())
	if err != nil {
		return
	}
}

func enigmaOpts(cli *cobra.Command) fx.Option {
	return fx.Options(
		// Set up all config variables to viper
		configProvider(cli),

		// Inject dependency modules
		kmsvault.Module,
		kmscrypto.Module,
		kmsmgm.Module,
		ocisecret.Module,
		ocivault.Module,
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),

		// Inject main application module
		enigma.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, a *enigma.Enigma, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						go func() {
							if err := a.Start(); err != nil {
								l.Error("Enigma Agent encountered an error during Start", zap.Error(err))
								os.Exit(1)
							}
							if err := sh.Shutdown(); err != nil {
								l.Error("Failed to shutdown Enigma Agent", zap.Error(err))
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

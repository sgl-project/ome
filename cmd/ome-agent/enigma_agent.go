package main

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/enigma"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	keymanagement "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/key_management"
	secretretrieval "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_retrieval"
	"context"
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
		keymanagement.KmsCryptoModule,
		keymanagement.KmsManagementModule,
		secretretrieval.SecretRetrievalModule,
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
							a.Start()
							err := sh.Shutdown()
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
	cmdEnigma.Flags().StringVarP(&configFilePath, "config", "c", "", "path to config file")
	cmdEnigma.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

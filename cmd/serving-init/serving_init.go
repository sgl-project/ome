package main

import (
	servinginit "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/serving-init"
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

var configFilePath string

var debug bool

var cmdServe = &cobra.Command{
	Use:   "serving-init",
	Short: "Run Serving Init",
	Long:  "Run Serving Init to serve model for inference",
	Run:   serving,
}

func serving(c *cobra.Command, args []string) {
	app := fx.New(opts(c))
	app.Run()
	app.Stop(context.Background())
}

func opts(cli *cobra.Command) fx.Option {
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
		servinginit.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, a *servinginit.ServingInit, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						go func() {
							a.Start()
							sh.Shutdown()
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
	cmdServe.Flags().StringVarP(&configFilePath, "config", "c", "", "path to config file")

	cmdServe.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

package main

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/cmd/download-agent/injection"
	genericdownloadagent "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/download-agent/generic"
	hfdownloadagent "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/download-agent/hf"
	partnerdownloadagent "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/download-agent/partner"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	secretinvault "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/secrets/secret_in_vault"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var configFilePath string

var vendor string

var debug bool

var cmdServe = &cobra.Command{
	Use:   "download-agent",
	Short: "Run download agent",
	Long:  "Run download agent",
	Run:   download,
}

func download(c *cobra.Command, args []string) {
	var app *fx.App
	switch vendor {
	case "generic":
		fmt.Println("Running generic download agent...")
		app = fx.New(genericOpts(c))
	case "hf":
		fmt.Println("Running HF download agent...")
		app = fx.New(hfOpts(c))
	default:
		fmt.Println("Running default Cohere download agent...")
		app = fx.New(cohereOpts(c))
	}
	app.Run()
	app.Stop(context.Background())
}

func cohereOpts(cli *cobra.Command) fx.Option {
	return fx.Options(
		// Set up all partner/cohere download agent config variables to viper
		cohereConfigProvider(cli),

		// Inject dependency modules
		injection.PartnerDownloadAgentConfigProvider(),
		injection.CasperDataStoreListProvider(),
		injection.KmsModuleProvider(),
		injection.SecretRetrievalProvider(),
		secretinvault.SecretInVaultModule,
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),

		// Inject main app module
		partnerdownloadagent.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, d *partnerdownloadagent.DownloadAgent, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(ctx context.Context) error {
						go func() {
							d.Start()
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

func hfOpts(cli *cobra.Command) fx.Option {
	return fx.Options(
		// Set up all HF download agent config variables to viper
		hfConfigProvider(cli),

		// Inject dependency modules
		casper.CasperDataStoreModule,
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),

		// Inject main app module
		hfdownloadagent.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, d *hfdownloadagent.HFDownloadAgent, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(ctx context.Context) error {
						go func() {
							d.Start()
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

func genericOpts(cli *cobra.Command) fx.Option {
	return fx.Options(
		// Set up all Generic download agent config variables to viper
		genericConfigProvider(cli),

		// Inject dependency modules
		injection.GenericDownloadAgentConfigProvider(),
		injection.CasperDataStoreListProviderForGenericDownloadAgent(),
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),

		// Inject main app module
		genericdownloadagent.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, d *genericdownloadagent.GenericDownloadAgent, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(ctx context.Context) error {
						go func() {
							d.Start()
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
	cmdServe.Flags().StringVarP(&vendor, "vendor", "v", "", "Specify the vendor for model download agent")
	cmdServe.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

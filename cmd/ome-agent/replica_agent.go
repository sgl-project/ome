package main

import (
	"context"
	"os"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/replica"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var cmdReplica = &cobra.Command{
	Use:   "replica",
	Short: "Run OME Object Storage Replica Agent",
	Long:  "OME Agent Object Storage Replica Agent is dedicated for replicate model weight across regions and/or tenancies.",
	Run:   runOReplica,
}

func runOReplica(cmd *cobra.Command, args []string) {
	app := fx.New(replicaOpts(cmd))
	app.Run()
	err := app.Stop(context.Background())
	if err != nil {
		return
	}
}

func replicaOpts(cli *cobra.Command) fx.Option {
	return fx.Options(
		// Set up all hf_download agent config variables to viper
		configProvider(cli),

		// Inject dependency modules
		env.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		casper.CasperDataStoreModule,

		// Inject main application module
		replica.Module,

		// Start the server
		fx.Invoke(func(lc fx.Lifecycle, a *replica.ReplicaAgent, l *zap.Logger, sh fx.Shutdowner) {
			lc.Append(
				fx.Hook{
					OnStart: func(context.Context) error {
						go func() {
							if err := a.Start(); err != nil {
								l.Error("Replica Agent encountered an error during Start", zap.Error(err))
								os.Exit(1)
							}
							if err := sh.Shutdown(); err != nil {
								l.Error("Failed to shutdown ReplicaAgent", zap.Error(err))
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
	cmdReplica.Flags().StringVarP(&configFilePath, "config", "c", "", "path to config file")
	cmdReplica.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

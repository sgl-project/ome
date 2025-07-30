package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/internal/ome-agent/replica"
	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
)

// ReplicaAgent implements the AgentModule interface for object storage replica agent
type ReplicaAgent struct {
	agent *replica.ReplicaAgent
}

// Name returns the name of the agent
func (r *ReplicaAgent) Name() string {
	return "replica"
}

// ShortDescription returns a short description of the agent
func (r *ReplicaAgent) ShortDescription() string {
	return "Run OME Object Storage Replica Agent"
}

// LongDescription returns a detailed description of the agent
func (r *ReplicaAgent) LongDescription() string {
	return "OME Agent Object Storage Replica Agent is dedicated for replicate model weight across regions and/or tenancies."
}

// ConfigureCommand configures the agent command
func (r *ReplicaAgent) ConfigureCommand(cmd *cobra.Command) {
	// Set the default action for this command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, r, r.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (r *ReplicaAgent) FxModules() []fx.Option {
	return []fx.Option{
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		logging.ModuleNamed("hub_logger"),
		OCIOSDataStoreListProvider(),
		PVCFileSystemProviders(),
		hub.Module,
		replica.Module,
		fx.Populate(&r.agent),
	}
}

// Start starts the agent
func (r *ReplicaAgent) Start() error {
	return r.agent.Start()
}

// NewReplicaAgent creates a new replica agent
func NewReplicaAgent() *ReplicaAgent {
	return &ReplicaAgent{}
}

type OCIOSDataStoreConfigWrapper struct {
	fx.Out

	OCIOSDataStoreConfig *ociobjectstore.Config `group:"OCIOSDataStoreConfigs"`
}

func OCIOSDataStoreListProvider() fx.Option {
	return fx.Provide(
		provideSourceOCIOSDataSourceConfig,
		provideTargetOCIOSDataStoreConfig,
		ociobjectstore.ProvideListOfOCIOSDataStoreWithAppParams,
	)
}

func provideSourceOCIOSDataSourceConfig(
	logger logging.Interface,
	v *viper.Viper) (OCIOSDataStoreConfigWrapper, error) {
	sourceOCIEnabled := v.GetBool("source.oci.enabled")
	if !sourceOCIEnabled {
		return OCIOSDataStoreConfigWrapper{}, nil
	}

	sourceOCIOSDataStoreConfig := &ociobjectstore.Config{}
	if err := v.UnmarshalKey("source.oci", sourceOCIOSDataStoreConfig); err != nil {
		return OCIOSDataStoreConfigWrapper{}, fmt.Errorf("error occurred when unmarshalling key source: %+v", err)
	}
	sourceOCIOSDataStoreConfig.AnotherLogger = logger
	sourceOCIOSDataStoreConfig.Name = replica.SourceStorageConfigKeyName

	return OCIOSDataStoreConfigWrapper{
		OCIOSDataStoreConfig: sourceOCIOSDataStoreConfig,
	}, nil
}

func provideTargetOCIOSDataStoreConfig(logger logging.Interface, v *viper.Viper) (OCIOSDataStoreConfigWrapper, error) {
	targetOCIEnabled := v.GetBool("target.oci.enabled")
	if !targetOCIEnabled {
		return OCIOSDataStoreConfigWrapper{}, nil
	}

	targetOCIOSDataStoreConfig := &ociobjectstore.Config{}
	if err := v.UnmarshalKey("target.oci", targetOCIOSDataStoreConfig); err != nil {
		return OCIOSDataStoreConfigWrapper{}, fmt.Errorf("error occurred when unmarshalling key target: %+v", err)
	}

	targetOCIOSDataStoreConfig.AnotherLogger = logger
	targetOCIOSDataStoreConfig.Name = replica.TargetStorageConfigKeyName

	return OCIOSDataStoreConfigWrapper{
		OCIOSDataStoreConfig: targetOCIOSDataStoreConfig,
	}, nil
}

func PVCFileSystemProviders() fx.Option {
	return fx.Provide(
		fx.Annotate(
			provideSourcePVCFileSystem,
			fx.ResultTags(`name:"source_pvc_fs"`),
		),
		fx.Annotate(
			provideTargetPVCFileSystem,
			fx.ResultTags(`name:"target_pvc_fs"`),
		),
	)
}

func provideSourcePVCFileSystem(v *viper.Viper) *afero.OsFs {
	sourcePVCEnabled := v.GetBool("source.pvc.enabled")
	if !sourcePVCEnabled {
		return nil
	}
	return afero.NewOsFs().(*afero.OsFs)
}

func provideTargetPVCFileSystem(v *viper.Viper) *afero.OsFs {
	targetPVCEnabled := v.GetBool("target.pvc.enabled")
	if !targetPVCEnabled {
		return nil
	}
	return afero.NewOsFs().(*afero.OsFs)
}

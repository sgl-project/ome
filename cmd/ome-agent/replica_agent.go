package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/internal/ome-agent/replica"
	"github.com/sgl-project/ome/pkg/afero"
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
		ociobjectstore.OCIOSDataStoreListProvider,
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

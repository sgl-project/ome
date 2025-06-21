package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/internal/ome-agent/enigma"
	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/vault/kmscrypto"
	"github.com/sgl-project/ome/pkg/vault/kmsmgm"
	"github.com/sgl-project/ome/pkg/vault/kmsvault"
	ocisecret "github.com/sgl-project/ome/pkg/vault/secret"
	ocivault "github.com/sgl-project/ome/pkg/vault/vault"
)

// EnigmaAgent implements the AgentModule interface for enigma agent
type EnigmaAgent struct {
	agent *enigma.Enigma
}

// Name returns the name of the agent
func (e *EnigmaAgent) Name() string {
	return "enigma"
}

// ShortDescription returns a short description of the agent
func (e *EnigmaAgent) ShortDescription() string {
	return "Run OME Enigma Agent"
}

// LongDescription returns a detailed description of the agent
func (e *EnigmaAgent) LongDescription() string {
	return "OME Agent Enigma is dedicated for model encryption and decryption."
}

// ConfigureCommand configures the agent command
func (e *EnigmaAgent) ConfigureCommand(cmd *cobra.Command) {
	// Set the default action for this command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, e, e.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (e *EnigmaAgent) FxModules() []fx.Option {
	return []fx.Option{
		kmsvault.Module,
		kmscrypto.Module,
		kmsmgm.Module,
		ocisecret.Module,
		ocivault.Module,
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		enigma.Module,
		fx.Populate(&e.agent),
	}
}

// Start starts the agent
func (e *EnigmaAgent) Start() error {
	return e.agent.Start()
}

// NewEnigmaAgent creates a new enigma agent
func NewEnigmaAgent() *EnigmaAgent {
	return &EnigmaAgent{}
}

package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/enigma"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/env"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmscrypto"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmsmgm"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmsvault"
	ocisecret "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/secret"
	ocivault "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/vault"
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
		env.Module,
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

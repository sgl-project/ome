package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	trainingAgent "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/training-agent"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/afero"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/ociobjectstore"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
)

type Runtime string

const (
	Cohere         Runtime = "cohere"
	CohereCommandR Runtime = "cohere-commandr"
	Peft           Runtime = "peft"
)

// TrainingAgent implements the AgentModule interface for training agent
type TrainingAgent struct {
	agent *trainingAgent.TrainingAgent
}

// Name returns the name of the agent
func (t *TrainingAgent) Name() string {
	return "training-agent"
}

// ShortDescription returns a short description of the agent
func (t *TrainingAgent) ShortDescription() string {
	return "Run OME Training Agent"
}

// LongDescription returns a detailed description of the agent
func (t *TrainingAgent) LongDescription() string {
	return "OME Training Agent is dedicated for training lifecycle management, training performance metrics store"
}

// ConfigureCommand configures the agent command
func (t *TrainingAgent) ConfigureCommand(cmd *cobra.Command) {
	// Set the default action for this command
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runAgentCommand(cmd, t, t.Start)
	}
}

// FxModules returns the fx modules needed by this agent
func (t *TrainingAgent) FxModules() []fx.Option {
	return []fx.Option{
		afero.Module,
		logging.Module,
		logging.ModuleNamed("another_log"),
		AuthTypeProvider(),
		CasperDataStoreListProvider(),
		trainingAgent.Module,
		fx.Populate(&t.agent),
	}
}

// Start starts the agent
func (t *TrainingAgent) Start() error {
	t.agent.Start()
	return nil
}

// NewTrainingAgent creates a new training agent
func NewTrainingAgent() *TrainingAgent {
	return &TrainingAgent{}
}

/*CasperConfigWrapper provides CasperConfig to the fx app defined in ociobjectstore module (from ociobjectstore pkg).
 * The initialized configuration in this struct will be added to the "casperConfigs" group, further allowing multiple
 * CasperConfig to be injected and managed collectively.
 * More info regarding fx Value Groups can be found: https://pkg.go.dev/go.uber.org/fx#hdr-Value_Groups
 */
type CasperConfigWrapper struct {
	fx.Out

	CasperConfig *ociobjectstore.Config `group:"casperConfigs"`
}

func AuthTypeProvider() fx.Option {
	return fx.Provide(func(v *viper.Viper) principals.AuthenticationType {
		var authType principals.AuthenticationType
		if err := v.UnmarshalKey("auth_type", &authType); err != nil {
			panic(fmt.Errorf("error occurred when unmarshalling key auth_type: %+v", err))
		}
		return authType
	})
}

func CasperDataStoreListProvider() fx.Option {
	return fx.Provide(
		provideInputCasperConfig,
		provideOutputCasperConfig,
		ociobjectstore.ProvideListOfOCIOSDataStoreWithAppParams,
	)
}

func provideInputCasperConfig(logger logging.Interface, v *viper.Viper, authType principals.AuthenticationType) CasperConfigWrapper {
	inputCasperConfig := &ociobjectstore.Config{}
	if err := v.UnmarshalKey("input_object_store", inputCasperConfig); err != nil {
		panic(fmt.Errorf("error occurred when unmarshalling key input_object_store: %+v", err))
	}
	inputCasperConfig.AnotherLogger = logger
	inputCasperConfig.Name = trainingAgent.InputCasperConfigName
	inputCasperConfig.AuthType = &authType

	return CasperConfigWrapper{
		CasperConfig: inputCasperConfig,
	}
}

func provideOutputCasperConfig(logger logging.Interface, authType principals.AuthenticationType) CasperConfigWrapper {
	outputCasperConfig := &ociobjectstore.Config{
		AnotherLogger: logger,
		Name:          trainingAgent.OutputCasperConfigName,
		AuthType:      &authType,
	}
	return CasperConfigWrapper{
		CasperConfig: outputCasperConfig,
	}
}

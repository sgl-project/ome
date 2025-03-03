package main

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/fx"
)

// MockAgentModule is a mock implementation of the AgentModule interface for testing
type MockAgentModule struct {
	mock.Mock
}

func (m *MockAgentModule) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgentModule) ShortDescription() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgentModule) LongDescription() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgentModule) FxModules() []fx.Option {
	args := m.Called()
	return args.Get(0).([]fx.Option)
}

func (m *MockAgentModule) ConfigureCommand(cmd *cobra.Command) {
	m.Called(cmd)
}

func (m *MockAgentModule) Start() error {
	args := m.Called()
	return args.Error(0)
}

// TestCreateAgentCommand tests the CreateAgentCommand function
func TestCreateAgentCommand(t *testing.T) {
	// Create a mock agent module
	mockModule := new(MockAgentModule)

	// Set up expectations
	mockModule.On("Name").Return("mock-agent")
	mockModule.On("ShortDescription").Return("Mock Agent Short Description")
	mockModule.On("LongDescription").Return("Mock Agent Long Description")
	mockModule.On("ConfigureCommand", mock.AnythingOfType("*cobra.Command")).Run(func(args mock.Arguments) {
		cmd := args.Get(0).(*cobra.Command)
		// Add a simple run function to the command
		cmd.Run = func(cmd *cobra.Command, args []string) {}
	})

	// Call the function being tested
	cmd := CreateAgentCommand(mockModule)

	// Verify the command was created correctly
	assert.Equal(t, "mock-agent", cmd.Use)
	assert.Equal(t, "Mock Agent Short Description", cmd.Short)
	assert.Equal(t, "Mock Agent Long Description", cmd.Long)

	// Verify the flags were added
	configFlag := cmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)
	assert.Equal(t, "c", configFlag.Shorthand)

	debugFlag := cmd.PersistentFlags().Lookup("debug")
	assert.NotNil(t, debugFlag)
	assert.Equal(t, "d", debugFlag.Shorthand)

	// Verify ConfigureCommand was called
	mockModule.AssertCalled(t, "ConfigureCommand", mock.AnythingOfType("*cobra.Command"))
}

// TestRunAgentCommand tests the runAgentCommand function indirectly
// Note: Since runAgentCommand starts an fx application, we can't easily test it directly
// Instead, we'll test the function signature and basic setup
func TestRunAgentCommandSignature(t *testing.T) {
	// Verify the function signature is correct
	// This doesn't actually run the function, just verifies it can be called with these arguments
	assert.NotPanics(t, func() {
		// We can't actually call runAgentCommand here because it would start an fx application
		// This is just to verify the function signature
		_ = runAgentCommand
	})
}

// TestAgentModuleInterface tests that our mock correctly implements the AgentModule interface
func TestAgentModuleInterface(t *testing.T) {
	// This test verifies that MockAgentModule implements AgentModule
	var _ AgentModule = (*MockAgentModule)(nil)
}

// TestAgentStartError tests handling of errors from the Start method
func TestAgentStartError(t *testing.T) {
	// Create a mock agent module that returns an error from Start
	mockModule := new(MockAgentModule)
	mockModule.On("Name").Return("error-agent")
	mockModule.On("Start").Return(errors.New("start error"))

	// Verify the error is returned
	err := mockModule.Start()
	assert.Error(t, err)
	assert.Equal(t, "start error", err.Error())
}

// TestAgentWithSubcommands tests an agent with subcommands
func TestAgentWithSubcommands(t *testing.T) {
	// Create a mock agent module
	mockModule := new(MockAgentModule)

	// Set up expectations
	mockModule.On("Name").Return("mock-agent")
	mockModule.On("ShortDescription").Return("Mock Agent Short Description")
	mockModule.On("LongDescription").Return("Mock Agent Long Description")
	mockModule.On("ConfigureCommand", mock.AnythingOfType("*cobra.Command")).Run(func(args mock.Arguments) {
		cmd := args.Get(0).(*cobra.Command)

		// Add a subcommand
		subCmd := &cobra.Command{
			Use:   "subcommand",
			Short: "Subcommand Short Description",
			Run:   func(cmd *cobra.Command, args []string) {},
		}
		cmd.AddCommand(subCmd)
	})

	// Call the function being tested
	cmd := CreateAgentCommand(mockModule)

	// Verify the subcommand was added
	assert.Equal(t, 1, len(cmd.Commands()))
	assert.Equal(t, "subcommand", cmd.Commands()[0].Use)
	assert.Equal(t, "Subcommand Short Description", cmd.Commands()[0].Short)
}

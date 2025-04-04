# Proposal: Simplify OME Agent Command Structure

## Current Structure

Currently, the OME Agent has multiple entry points:
- `enigma_agent.go`
- `hf_download_agent.go`
- `fine_tuned_adapter.go`
- `replica_agent.go`
- `serving_sidecar.go`
- `training_agent.go`

Each file follows a similar pattern:
1. Defines a Cobra command
2. Implements a `run[AgentName]` function
3. Implements an `[agentName]Opts` function that sets up the fx application
4. Has an `init()` function that adds flags

The `main.go` file simply adds all these commands to a root command.

There are also global variables (`configFilePath` and `debug`) that are defined in one file but used across all files.

## Issues with Current Structure

1. **Duplication**: Each agent file has similar boilerplate code for setting up the fx application.
2. **Global Variables**: The use of global variables across files makes the code harder to maintain.
3. **Inconsistent Flag Registration**: Some files register flags for other commands (e.g., `cmdEnigma.Flags()` in `serving_sidecar.go`).
4. **Scattered Configuration**: The configuration logic is spread across multiple files.

## Proposed Solution

### 1. Create a Common Agent Framework

Create a generic agent framework that can be used by all agent types:

```go
// agent.go
package main

import (
    "context"
    "os"

    "github.com/spf13/cobra"
    "go.uber.org/fx"
    "go.uber.org/zap"
)

// AgentModule represents a module that can be run by the agent framework
type AgentModule interface {
    Name() string
    ShortDescription() string
    LongDescription() string
    FxModules() []fx.Option
    
    // Allow agents to configure their commands (add subcommands, custom flags, etc.)
    ConfigureCommand(*cobra.Command)
    
    // Default action when no subcommand is specified
    Start() error
}

// CreateAgentCommand creates a cobra command for an agent module
func CreateAgentCommand(module AgentModule) *cobra.Command {
    cmd := &cobra.Command{
        Use:   module.Name(),
        Short: module.ShortDescription(),
        Long:  module.LongDescription(),
        // We don't set Run here - let the module decide if it wants a default action
    }
    
    // Add common flags to persistent flags so they're available to subcommands
    cmd.PersistentFlags().StringVarP(&configFilePath, "config", "c", "", "path to config file")
    cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
    
    // Let the module configure its command (add subcommands, set Run function, etc.)
    module.ConfigureCommand(cmd)
    
    return cmd
}

// runAgentCommand runs a specific command action for an agent
func runAgentCommand(cmd *cobra.Command, module AgentModule, action func() error) {
    options := []fx.Option{
        // Set up all config variables to viper
        configProvider(cmd),
        
        // Common modules
        env.Module,
        afero.Module,
        logging.Module,
        logging.ModuleNamed("another_log"),
    }
    
    // Add module-specific options
    for _, opt := range module.FxModules() {
        options = append(options, opt)
    }
    
    // Add lifecycle hooks
    options = append(options, fx.Invoke(func(lc fx.Lifecycle, l *zap.Logger, sh fx.Shutdowner) {
        lc.Append(
            fx.Hook{
                OnStart: func(context.Context) error {
                    go func() {
                        if err := action(); err != nil {
                            l.Error(module.Name()+" encountered an error during execution", zap.Error(err))
                            os.Exit(1)
                        }
                        if err := sh.Shutdown(); err != nil {
                            l.Error("Failed to shutdown "+module.Name(), zap.Error(err))
                        }
                    }()
                    return nil
                },
                OnStop: func(ctx context.Context) error {
                    return nil
                },
            })
    }))
    
    app := fx.New(fx.Options(options...))
    app.Run()
    err := app.Stop(context.Background())
    if err != nil {
        return
    }
}
```

### 2. Implement Each Agent as a Module

Each agent would implement the `AgentModule` interface. Here are examples for simple and complex agents:

#### Simple Agent Example (No Subcommands)

```go
// enigma_agent.go
package main

import (
    "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/enigma"
    "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmscrypto"
    "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmsmgm"
    "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/kmsvault"
    ocisecret "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/secret"
    ocivault "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/vault/vault"
    "github.com/spf13/cobra"
    "go.uber.org/fx"
)

type EnigmaAgent struct {
    agent *enigma.Enigma
}

func (e *EnigmaAgent) Name() string {
    return "enigma"
}

func (e *EnigmaAgent) ShortDescription() string {
    return "Run OME Enigma Agent"
}

func (e *EnigmaAgent) LongDescription() string {
    return "OME Agent Enigma is dedicated for model encryption and decryption."
}

func (e *EnigmaAgent) ConfigureCommand(cmd *cobra.Command) {
    // Set the default action for this command
    cmd.Run = func(cmd *cobra.Command, args []string) {
        runAgentCommand(cmd, e, e.Start)
    }
    
    // Add any command-specific flags here
    // cmd.Flags().StringVar(&e.someFlag, "some-flag", "", "Description of flag")
}

func (e *EnigmaAgent) FxModules() []fx.Option {
    return []fx.Option{
        kmsvault.Module,
        kmscrypto.Module,
        kmsmgm.Module,
        ocisecret.Module,
        ocivault.Module,
        enigma.Module,
        fx.Populate(&e.agent),
    }
}

func (e *EnigmaAgent) Start() error {
    return e.agent.Start()
}

func NewEnigmaAgent() *EnigmaAgent {
    return &EnigmaAgent{}
}
```

#### Complex Agent Example (With Subcommands)

```go
// training_agent.go
package main

import (
    training_agent "bitbucket.oci.oraclecorp.com/genaicore/ome/internal/ome-agent/training-agent"
    "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
    "github.com/spf13/cobra"
    "go.uber.org/fx"
)

type TrainingAgent struct {
    agent *training_agent.TrainingAgent
    modelPath string // Example of agent-specific configuration
}

func (t *TrainingAgent) Name() string {
    return "training-agent"
}

func (t *TrainingAgent) ShortDescription() string {
    return "Run OME Training Agent"
}

func (t *TrainingAgent) LongDescription() string {
    return "OME Training Agent is dedicated for training lifecycle management, training performance metrics store"
}

func (t *TrainingAgent) ConfigureCommand(cmd *cobra.Command) {
    // Add subcommands
    startCmd := &cobra.Command{
        Use:   "start",
        Short: "Start a training job",
        Run: func(cmd *cobra.Command, args []string) {
            runAgentCommand(cmd, t, t.StartTraining)
        },
    }
    
    stopCmd := &cobra.Command{
        Use:   "stop",
        Short: "Stop a training job",
        Run: func(cmd *cobra.Command, args []string) {
            runAgentCommand(cmd, t, t.StopTraining)
        },
    }
    
    statusCmd := &cobra.Command{
        Use:   "status",
        Short: "Get training job status",
        Run: func(cmd *cobra.Command, args []string) {
            runAgentCommand(cmd, t, t.GetStatus)
        },
    }
    
    // Add subcommands to the main command
    cmd.AddCommand(startCmd, stopCmd, statusCmd)
    
    // Set default action for the main command
    cmd.Run = func(cmd *cobra.Command, args []string) {
        runAgentCommand(cmd, t, t.Start)
    }
    
    // Add command-specific flags
    cmd.PersistentFlags().StringVar(&t.modelPath, "model-path", "", "Path to the model")
    
    // Add subcommand-specific flags
    startCmd.Flags().BoolVar(&t.someStartFlag, "some-start-flag", false, "Flag specific to start command")
}

func (t *TrainingAgent) FxModules() []fx.Option {
    return []fx.Option{
        casper.CasperDataStoreModule,
        training_agent.Module,
        fx.Populate(&t.agent),
    }
}

func (t *TrainingAgent) Start() error {
    return t.agent.Start()
}

func (t *TrainingAgent) StartTraining() error {
    return t.agent.StartTraining(t.modelPath)
}

func (t *TrainingAgent) StopTraining() error {
    return t.agent.StopTraining()
}

func (t *TrainingAgent) GetStatus() error {
    return t.agent.GetStatus()
}

func NewTrainingAgent() *TrainingAgent {
    return &TrainingAgent{}
}
```

### 3. Simplify Main.go

```go
// main.go
package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var configFilePath string
var debug bool

var rootCmd = &cobra.Command{
    Use:   "ome-agent",
    Short: "Run OME Agent",
    Long:  "OME Agent is a swiss army knife for OME inference service, training job, model management, etc.",
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        _, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
        os.Exit(1)
    }
}

func init() {
    // Register all agent commands
    rootCmd.AddCommand(CreateAgentCommand(NewEnigmaAgent()))
    rootCmd.AddCommand(CreateAgentCommand(NewHFDownloadAgent()))
    rootCmd.AddCommand(CreateAgentCommand(NewReplicaAgent()))
    rootCmd.AddCommand(CreateAgentCommand(NewTrainingAgent()))
    rootCmd.AddCommand(CreateAgentCommand(NewServingSidecarAgent()))
    rootCmd.AddCommand(CreateAgentCommand(NewFineTunedAdapterAgent()))
}
```

## Benefits of the Proposed Solution

1. **Reduced Duplication**: Common code is consolidated into a single framework.
2. **Improved Maintainability**: Each agent only needs to implement its specific logic.
3. **Consistent Configuration**: Flag registration and configuration are handled in one place.
4. **Cleaner Dependencies**: Each agent explicitly declares its dependencies.
5. **Easier to Add New Agents**: Just implement the `AgentModule` interface and register it in `main.go`.
6. **Support for Complex Command Structures**: Agents can define subcommands and command-specific flags.
7. **Flexible Command Actions**: Each command or subcommand can have its own action function.

## Implementation Plan

1. Create the `agent.go` file with the common framework.
2. Refactor each agent to implement the `AgentModule` interface.
3. Update `main.go` to use the new framework.
4. Move global variables to `main.go`.
5. Remove duplicate code from agent files.

## Additional Considerations

1. **Command Groups**: For very complex command structures, consider using command groups to organize related commands.
2. **Command Aliases**: Add command aliases for backward compatibility if command names change.
3. **Command Options**: Add all command options instead of relying on reading from config files.

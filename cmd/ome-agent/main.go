package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

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
	rootCmd.AddCommand(cmdEnigma)
	rootCmd.AddCommand(cmdHFDownload)
	rootCmd.AddCommand(cmdReplica)
	rootCmd.AddCommand(cmdTrainingAgent)
	rootCmd.AddCommand(cmdServingSidecar)
}

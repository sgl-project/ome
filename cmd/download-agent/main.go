package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

const appName = "download-agent"

var rootCmd = &cobra.Command{
	Use:   "download-agent",
	Short: "A download agent for model",
	Long:  "A download agent for initial model importing or model replication",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(cmdServe)
}

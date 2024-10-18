package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

const appName = "serving-ft"

var rootCmd = &cobra.Command{
	Use:   "serving-ft",
	Short: "serving-ft for model FT serving",
	Long:  "serving-ft for model FT serving used to load fine-tuned checkpoints",
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

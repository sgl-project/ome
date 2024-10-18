package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

const appName = "serving-init"

var rootCmd = &cobra.Command{
	Use:   "serving-init",
	Short: "serving-init for model serving",
	Long:  "serving-init for model serving used to decrypt the model",
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

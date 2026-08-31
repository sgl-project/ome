package main

import (
	"os"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"sigs.k8s.io/ome/pkg/cli"
)

func main() {
	streams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	os.Exit(run(os.Args[1:], streams))
}

func run(args []string, streams genericiooptions.IOStreams) int {
	return cli.Run(args, streams)
}

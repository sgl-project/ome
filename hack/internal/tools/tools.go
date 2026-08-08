//go:build tools

package tools

// Keep a reference to the code generators so they are not removed by go mod tidy

import (
	_ "github.com/golangci/golangci-lint/pkg/exitcodes"
	_ "github.com/mikefarah/yq/v4/cmd"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest/env"
	_ "sigs.k8s.io/controller-tools/pkg/crd"
	_ "sigs.k8s.io/controller-tools/pkg/genall/help/pretty"
	_ "sigs.k8s.io/kind/pkg/cmd"
	_ "sigs.k8s.io/kustomize/kustomize/v4/commands/edit/listbuiltin"
)

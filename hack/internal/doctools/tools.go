//go:build tools

package doctools

// Keep a reference to the documentation and chart tooling so it is not
// removed by go mod tidy. Kept in a module separate from the codegen
// tools: helm/hugo track current k8s libraries while the code
// generators stay pinned to the repository's k8s minor, and the two
// requirement sets conflict inside a single module.

import (
	_ "github.com/gohugoio/hugo/common"
	_ "github.com/gohugoio/hugo/docshelper"
	_ "github.com/norwoodj/helm-docs/cmd/helm-docs"
	_ "helm.sh/helm/v3/pkg/cli"
	_ "helm.sh/helm/v3/pkg/lint"
	_ "sigs.k8s.io/mdtoc"
)

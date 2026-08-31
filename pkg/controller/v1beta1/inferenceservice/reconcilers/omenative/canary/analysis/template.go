package analysis

import (
	"strings"
	"text/template"
)

// TemplateContext holds the revision-scoped values an AnalysisMetric.Query may
// reference. The reconciler populates it from live rollout state: the per-revision
// Service names are the same <isvc>-<component>-rev-<hash> the executor programs
// traffic onto, so a query can target exactly the canary or the stable revision's
// pods.
type TemplateContext struct {
	Namespace      string
	ISVCName       string
	Component      string
	CanaryService  string
	StableService  string
	CanaryRevision string
	StableRevision string
}

// RenderQuery substitutes the TemplateContext into a PromQL template. A reference
// to a field that does not exist on TemplateContext is an execution error rather
// than a silent empty string, so a typo'd {{.Cnaary...}} fails loudly at
// evaluation instead of querying a malformed selector that would read as "no
// data" forever.
func RenderQuery(query string, tc TemplateContext) (string, error) {
	tmpl, err := template.New("promql").Option("missingkey=error").Parse(query)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, tc); err != nil {
		return "", err
	}
	return sb.String(), nil
}

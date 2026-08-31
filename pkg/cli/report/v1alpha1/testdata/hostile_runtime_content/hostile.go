package hostile_runtime_content

import (
	"sigs.k8s.io/ome/pkg/cli/report"
	"sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
)

type hostileContent struct {
	v1alpha1.RuntimeEffectiveContent
	ResourceVersion string `json:"resourceVersion"`
}

func (content hostileContent) Canonical() hostileContent {
	return content
}

func (hostileContent) Table() report.Table {
	return report.Table{}
}

var _ = v1alpha1.RuntimeEnvelope[hostileContent]{}

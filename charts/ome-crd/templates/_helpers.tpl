{{/*
Chart name and version, suitable for use as a Kubernetes label value.
*/}}
{{- define "ome-crd.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Standard Helm chart provenance labels.
*/}}
{{- define "ome-crd.labels" -}}
helm.sh/chart: {{ include "ome-crd.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ome
{{- end -}}

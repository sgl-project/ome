{{/*
Constructs an image ref with an optional global hub prefix. If the repository
already contains '/', hub is ignored. Mirrors the ome-resources helper.
Parameters: values, repository, tag.
*/}}
{{- define "ome-scheduler.image" -}}
{{- $hub := .values.global.hub -}}
{{- $repo := .repository -}}
{{- $tag := .tag -}}
{{- if and $hub (not (contains "/" $repo)) -}}
{{- printf "%s/%s:%s" $hub $repo $tag -}}
{{- else -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}
{{- end -}}

{{/*
Chart name and version, suitable for use as a Kubernetes label value.
*/}}
{{- define "ome-scheduler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Selector labels — minimal and stable across re-renders.
*/}}
{{- define "ome-scheduler.selectorLabels" -}}
app.kubernetes.io/name: ome-scheduler
app.kubernetes.io/component: scheduler
{{- end -}}

{{/*
Full label set stamped on chart resources.
*/}}
{{- define "ome-scheduler.labels" -}}
{{ include "ome-scheduler.selectorLabels" . }}
helm.sh/chart: {{ include "ome-scheduler.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ome
{{- end -}}

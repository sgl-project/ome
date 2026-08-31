{{/*
Constructs a full image URL with optional global hub prefix.
If repository already contains '/', hub is ignored.
Parameters:
  - values: helm values (to import "Values.global.hub", required)
  - repository: the repository name (required)
  - tag: the image tag (required)
*/}}
{{- define "ome.imageWithHub" -}}
{{- $hub := .values.global.hub }}
{{- $repo := .repository }}
{{- $tag := .tag }}
{{- if and $hub (not (contains "/" $repo)) -}}
{{- printf "%s/%s:%s" $hub $repo $tag -}}
{{- else }}
{{- printf "%s:%s" $repo $tag -}}
{{- end }}
{{- end }}

{{/*
Chart name and version, suitable for use as a Kubernetes label value.
*/}}
{{- define "ome.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common chart provenance labels. Keep these out of selectors so a chart or app
version change does not make an existing workload selector immutable.
*/}}
{{- define "ome.labels" -}}
helm.sh/chart: {{ include "ome.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ome
{{- end -}}

{{/*
Selector labels for the bundled Prometheus (Deployment + Service
selector). Kept minimal so they stay stable across re-renders.
*/}}
{{- define "ome-prometheus.selectorLabels" -}}
app.kubernetes.io/name: ome-prometheus
app.kubernetes.io/component: ome-prometheus
{{- end }}

{{/*
Full label set stamped on bundled Prometheus resources. Mirrors the
selector labels and folds in operator-supplied
.Values.prometheus.additionalLabels.
*/}}
{{- define "ome-prometheus.labels" -}}
{{ include "ome-prometheus.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ome
{{- with .Values.prometheus.additionalLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Instance-type -> short-name map as a compact JSON string.
Single source of truth (.Values.modelAgent.instanceTypeMap) rendered into both
the model-agent ConfigMap (instance-type-map key) and the enigma init
container's INSTANCE_TYPE_MAP env var (ome-controller/configmap.yaml).
*/}}
{{- define "ome.instanceTypeMap" -}}
{{- .Values.modelAgent.instanceTypeMap | toJson -}}
{{- end }}

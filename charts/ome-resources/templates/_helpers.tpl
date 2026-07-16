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
Instance-type -> short-name map as a compact JSON string.
Single source of truth (.Values.modelAgent.instanceTypeMap) rendered into both
the model-agent ConfigMap (instance-type-map key) and the enigma init
container's INSTANCE_TYPE_MAP env var (ome-controller/configmap.yaml).
*/}}
{{- define "ome.instanceTypeMap" -}}
{{- .Values.modelAgent.instanceTypeMap | toJson -}}
{{- end }}

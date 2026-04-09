{{/*
  genaiModelLabels: emits genai-model-deprecated-date, genai-model-on-demand-retired-date, and genai-model-dedicated-retired-date labels.
  Usage: {{ include "proxy-models-importer.genaiModelLabels" (dict "root" $ "model" "gpt-4-1") | nindent 4 }}
*/}}

{{- define "proxy-models-importer.genaiModelLabels" -}}
genai-model-deprecated-date: "{{ index .root.Values .model "timeDeprecated" }}"
genai-model-on-demand-retired-date: "{{ index .root.Values .model "timeOnDemandRetired" }}"
genai-model-dedicated-retired-date: "{{ index .root.Values .model "timeDedicatedRetired" }}"
{{- end -}}

{{/*
  genaiModelAnnotations: emits experimental, internal, and lifecycle-phase annotations.
  Usage: {{ include "proxy-models-importer.genaiModelAnnotations" (dict "root" $ "model" "gpt-4-1") | nindent 4 }}
*/}}

{{- define "proxy-models-importer.genaiModelAnnotations" -}}
models.ome.io/experimental: "false"
models.ome.io/internal: "false"
models.ome.io/lifecycle-phase: "{{ index .root.Values .model "lifecyclePhase" }}"
{{- end -}}

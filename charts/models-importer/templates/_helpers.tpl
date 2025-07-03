{{/* 
  genaiModelLabels: emits genai-model-deprecated-date, genai-model-on-demand-retired-date, and genai-model-dedicated-retired-date labels.
  Usage: {{ include "models-importer.genaiModelLabels" (dict "root" $ "model" "llama-3-3-70b-instruct-fp8-dynamic") | nindent 4 }}
*/}}

{{- define "models-importer.genaiModelLabels" -}}
genai-model-deprecated-date: "{{ index .root.Values .model "timeDeprecated" }}"
genai-model-on-demand-retired-date: "{{ index .root.Values .model "timeOnDemandRetired" }}"
genai-model-dedicated-retired-date: "{{ index .root.Values .model "timeDedicatedRetired" }}"
{{- end -}}

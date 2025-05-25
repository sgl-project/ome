#!/usr/bin/env bash
set -euo pipefail

# Constants
readonly SCRIPT_NAME=$(basename "$0")
readonly SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# Logging
log() {
  local level="$1"
  local msg="$2"
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $msg" >&2
}

# Usage
usage() {
  cat <<EOF
Usage: $SCRIPT_NAME [OPTIONS] <input-file> <output-file>
Convert Kustomize YAML to Helm chart template

Arguments:
  input-file    Path to input Kustomize YAML
  output-file   Path to output Helm template

Options:
  -p, --prefix PREFIX  Helm values prefix (default: auto-detected)
  -c, --use-common-image     Use common image path (.Values.{runtime}.commonImage)
  -r, --run-time       Runtime (vllm, sglang)
  -h, --help           Show this help message
EOF
  exit 1
}

# Main processing function
process_yaml() {
  local input_file="$1"
  local output_file="$2"
  local helm_prefix="${3:-}"
  local use_common_image="${4:-false}"
  local run_time="${5:-}"
  
  [[ -z "$helm_prefix" ]] && {
    log "ERROR" "Failed to extract metadata.name from input"
    exit 1
  }

  [[ -z "$run_time" ]] && {
    log "ERROR" "Failed to obtain runtime from input"
    exit 1
  }
  
  log "INFO" "Using Helm values prefix: .Values.$helm_prefix"

  log "INFO" "Image template: $([ "$use_common_image" = true ] && echo "common" || echo "prefix-based")"
  awk -v hp="$helm_prefix" -v use_common="$use_common_image" -v runtime="$run_time" '
    # Image replacement logic
    /image: / {
      if (use_common == "true") {
        sub(/:.*/, ": {{ .Values." runtime ".commonImage.repository }}:{{ .Values." runtime ".commonImage.tag }}")
      } else {
        sub(/:.*/, ": {{ .Values." hp ".image.repository }}:{{ .Values." hp ".image.tag }}")
      }
    }

    # Common template replacements
    /annotations:/ { handle_conditional("Annotations") }
    /labels:/ { handle_conditional("Labels") }
    /tolerations:/ { handle_conditional("Tolerations") }
    
    /affinity:/ {
      print "  {{ with .Values." hp " }}"
      print "  affinity: {{ .affinity | toYaml | nindent 6 }}"
      print "  {{ end }}"
      while (getline && /^    /) { continue }
    }

    # Port and model replacements
    /containerPort: 8080/ { sub(/8080/, "{{ .Values." runtime ".port }}") }
    /--port=8080/ { sub(/8080/, "{{ .Values." runtime ".port }}") }
    /--served-model-name=/ { sub(/vllm-model/, "{{ .Values." runtime ".serveModelName }}") }

    # Preserve other lines
    { print }

    # Helper function for conditional blocks
    function handle_conditional(type) {
      lc_type = tolower(type)
      print "  {{- if or .Values.common" type " .Values." hp "." lc_type " }}"
      print "  " lc_type ":"
      print "    {{- if .Values.common" type " }}"
      print "    {{ toYaml .Values.common" type " | nindent 6 }}"
      print "    {{- end }}"
      print "    {{- if .Values." hp "." lc_type " }}"
      print "    {{ toYaml .Values." hp "." lc_type " | nindent 6 }}"
      print "    {{- end }}"
      print "  {{- end }}"
      while (getline && /^    /) { continue }
    }
  ' "$input_file" > "$output_file"
  
  log "INFO" "Generated Helm template: $output_file"
}

# Main function
main() {
  local helm_prefix=""
  local use_common_image=false
  local run_time=""
  
  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -p|--prefix)
        helm_prefix="$2"
        shift 2
        ;;
      -c|--use-common-image)
        use_common_image=true
        shift
        ;;
      -r|--run-time)
        run_time="$2"
        shift 2
        ;;
      -h|--help)
        usage
        ;;
      *)
        break
        ;;
    esac
  done

  if [[ $# -ne 2 ]]; then
    log "ERROR" "Invalid number of arguments"
    usage
  fi

  local input_file="$1"
  local output_file="$2"
  
  # Validate input file
  if [[ ! -f "$input_file" || ! -r "$input_file" ]]; then
    log "ERROR" "Input file not found or not readable: $input_file"
    exit 1
  fi
  
  # Process the YAML
  process_yaml "$input_file" "$output_file" "$helm_prefix" "$use_common_image" "$run_time"
  
  log "INFO" "Conversion completed successfully"
}

main "$@"
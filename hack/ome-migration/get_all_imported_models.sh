#!/bin/bash

#
# ==========================================================================================
# Script to Extract and Analyze Existing Imported Models
# ------------------------------------------------------------------------------------------
# This script interacts with a Kubernetes cluster and OCI Object Storage to:
#   - Prompt the user for stage and region, building a region/stage-specific folder
#   - List and find all imported models in the cluster vendored as DUMMY_VENDOR
#   - Download only the config.json files for 'Ready' models from Object Storage
#   - (Optionally) identify and log any alternative or subdirectory config.jsons
#   - Extract key metadata from each config.json (name, architectures, version)
#   - Call OCI Generative AI to analyze and classify models based on config
#   - Output all results and logs to a region/stage-specific folder
#
# Output:
#   - Model configs and extracted CSV summary are stored under a structured
#     directory: <stage>_<region>/configs, <stage>_<region>/extracted_models.csv
#   - All script terminal output is also written to <stage>_<region>/script.log
#
# Requirements:
#   - bash, kubectl, oci CLI, jq, access to OCI tenancy with correct auth/profile
# ==========================================================================================

# === Globals ===
# Auth
PROFILE="BoatOC1"
AUTH="security_token"

# Object storage
BUCKET="customer-model-store"
PREFIX="customer-imported-basemodels"

# calling chat
ENDPOINT="https://inference.generativeai.us-chicago-1.oci.oraclecloud.com"
COMPARTMENTID="ocid1.tenancy.oc1..aaaaaaaaumuuscymm6yb3wsbaicfx3mjhesghplvrvamvbypyehh5pgaasna"
MODEL="openai.gpt-4o"

# local
TARGET_BASE="$(pwd)"

# === Logging utility ===
log()     { echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $*" >&2 ; }
log_err() { echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $*" >&2 ; }

# Exit on error with a message
trap 'log_err "Script failed at line $LINENO. Last command: $BASH_COMMAND"' ERR

validate_dependencies() {
  for cmd in kubectl jq oci; do
    if ! command -v "$cmd" &>/dev/null; then
      log_err "Missing required command: $cmd"
      exit 2
    fi
  done
}

# Prompts for stage (dev/ppe/prod) and region, validates both, and sets STAGE, NAMESPACE, REGION.
select_stage_and_region() {
    # Stage mapping
    echo "Select stage:"
    echo "  dev"
    echo "  ppe"
    echo "  prod"
    read -rp "Enter stage (dev/ppe/prod): " stage

    case "$stage" in
        dev)  namespace="idqj093njucb" ;;
        ppe)  namespace="id4o0mkpq0wg" ;;
        prod) namespace="idlsnvn0f2is" ;;
        *)    echo "Unknown stage: $stage"; exit 1 ;;
    esac

    # Region list
    regions=(
        ap-osaka-1
        me-dubai-1
        sa-saopaulo-1
        us-phoenix-1
        uk-london-1
        us-ashburn-1
        eu-frankfurt-1
        us-chicago-1
    )

    echo "Select region from the following list:"
    i=1
    for region in "${regions[@]}"; do
        echo "  $i) $region"
        ((i++))
    done

    read -rp "Enter region number (1-${#regions[@]}): " num
    if ! [[ "$num" =~ ^[1-8]$ ]]; then
        echo "Invalid region number!"
        exit 1
    fi

    # Export/assign
    STAGE="$stage"
    NAMESPACE="$namespace"
    REGION="${regions[num-1]}"
}

# === Functions ===
get_ready_model_names() {
    # List all imported models, focus on vendor and ready state
    log "Querying cluster for all DUMMY_VENDOR models..."
    all_log=$(kubectl get models.ome.oracle.com \
        --output=custom-columns=NAME:.metadata.name,VENDOR:.spec.vendor,STATE:.status.state \
        --no-headers \
        | awk '$2=="DUMMY_VENDOR" {print $1 "\t" $3}' \
        | sort -k2)
    echo "$all_log" > "$TARGET_DIR/ready_models.log"
    ready_model_names=$(echo "$all_log" | awk -F'\t' '$2=="Ready"{print $1}')
    log "Found $(echo "$ready_model_names" | grep -c .) Ready model(s)."
    echo "$ready_model_names"
}

download_config_jsons() {
    ready_models="$1"  # list of model names (newline-separated)
    log "Listing all model objects in OCI bucket..."
    OCI_JSON=$(oci os object list \
        --namespace "$NAMESPACE" \
        --bucket-name "$BUCKET" \
        --region "$REGION" \
        --profile "$PROFILE" \
        --auth $AUTH \
        --prefix "$PREFIX/" \
        --all) || { log_err "Failed to list objects from OCI bucket"; exit 2; }

    if ! echo "$OCI_JSON" | jq -e '.data | length > 0' >/dev/null; then
        log_err "No objects found in bucket $BUCKET (namespace $NAMESPACE, prefix $PREFIX)!"
        exit 1
    fi

    OBJECTS=$(echo "$OCI_JSON" | jq -r '.data[].name')

    MODEL_DIRS=$(echo "$OBJECTS" | awk -F/ 'NF>=4{print $2"/"$3}' | sort -u)
    MODEL_COUNT=$(echo "$MODEL_DIRS" | wc -l)
    log "Processing $MODEL_COUNT model directories..."

    mkdir -p "$CONFIG_DIR"

    # Make a named pattern for ready models only
    while read -r MODEL_NAME
    do
        [ -z "$MODEL_NAME" ] && continue  # skip blanks
        # Find corresponding MODEL_PATH
        # Assuming MODEL_NAME is "$2/$3" from bucket (to match original code)
        MODEL_PATH=$(echo "$MODEL_DIRS" | grep "/$MODEL_NAME$" || true)
        if [ -z "$MODEL_PATH" ]; then
            log "WARNING: Model $MODEL_NAME not found in object storage listing, skipping."
            continue
        fi

        FILES=$(grep "^$PREFIX/$MODEL_PATH/" <<< "$OBJECTS")
        HAS_CONFIG_JSON=$(echo "$FILES" | awk -F/ '$NF == "config.json" {found=1} END{if(found) print "yes"; else print "no"}')
        if [ "$HAS_CONFIG_JSON" = "yes" ]; then
            log "✅ $MODEL_PATH has config.json"
            SAFE_FILE_NAME="$(echo "$MODEL_PATH" | tr '/' '_')_config.json"
            CONFIG_OBJECT="$PREFIX/$MODEL_PATH/config.json"
            log "Downloading $CONFIG_OBJECT to $CONFIG_DIR/$SAFE_FILE_NAME"
            oci os object get \
                --namespace "$NAMESPACE" \
                --bucket-name "$BUCKET" \
                --region "$REGION" \
                --profile "$PROFILE" \
                --auth security_token \
                --name "$CONFIG_OBJECT" \
                --file "$CONFIG_DIR/$SAFE_FILE_NAME"

            # Find all other config.json files in subdirs
            OTHER_CONFIGS=$(echo "$FILES" | grep -E "^$PREFIX/$MODEL_PATH/.+/.*/config\.json$|^$PREFIX/$MODEL_PATH/[^/]+/config\.json$" | grep -vxF "$DIRECT_CONFIG" || true)
            if [ -n "$OTHER_CONFIGS" ]; then
                log "🔎 $MODEL_PATH has config.json in subdirs:"
                echo "$OTHER_CONFIGS"
            fi
        else
            ALT_CONFIGS=$(echo "$FILES" | awk -F/ '$NF ~ /^config\./ && $NF != "config.json" {print $NF}')
            if [ -n "$ALT_CONFIGS" ]; then
                log "❌ $MODEL_PATH missing config.json, but has: $(echo "$ALT_CONFIGS" | tr '\n' ' ')"
            else
                log "❌ $MODEL_PATH does not have any config.json or config.* file"
            fi
        fi
    done <<< "$ready_models"
}

extract_and_analyze_configs() {
    log "Extracting info and generating CSV ($OUTPUT_CSV)..."
    echo "compartment_id,model_id,_name_or_path,architectures,transformers_version,ai_answer" > "$OUTPUT_CSV"

    for file in "$CONFIG_DIR"/*.json; do
        filename=$(basename "$file")
        compartment_id=$(echo "$filename" | awk -F_ '{print $1}')
        model_id=$(echo "$filename" | awk -F_ '{print $2}')

        # Get _name_or_path
        name_or_path=$(jq -r '."__name_or_path" // ."_name_or_path" // empty' "$file")
        if [ -z "$name_or_path" ]; then
            name_or_path=$(grep -m1 -E '"(__name_or_path|_name_or_path)"' "$file" | awk -F: '{gsub(/[ ",]/,"",$2); print $2}')
        fi
        [ -z "$name_or_path" -o "$name_or_path" == "null" ] && name_or_path=""

        # Architectures
        architectures=$(jq -r '.architectures | join("|")' "$file" 2>/dev/null)
        if [ -z "$architectures" ] || [ "$architectures" == "null" ]; then
            architectures=$(grep -m1 '"architectures"' "$file" | awk -F: '{print $2}' | grep -o '\[[^]]*\]' | tr -d '[]" ' | tr ',' '|')
        fi
        [ -z "$architectures" -o "$architectures" == "null" ] && architectures=""

        # Transformers version
        transformers_version=$(jq -r '.transformers_version // empty' "$file")
        if [ -z "$transformers_version" ]; then
            transformers_version=$(grep '"transformers_version"' "$file" | awk -F: '{gsub(/[ ",]/,"",$2); print $2}')
        fi
        [ -z "$transformers_version" -o "$transformers_version" == "null" ] && transformers_version=""

        # OCI GenAI: classify the model config.json ☁️
        question="Given this model config.json, what model is it? Please give a brief answer and only output the most potential model name."
        json_text=$(cat "$file")
        PROMPT="${question}\n${json_text}"

        log "Requesting GenAI for [$filename]..."

        chat_answer=$(oci --profile "$PROFILE" \
            --auth "$AUTH" \
            --endpoint "$ENDPOINT" \
            generative-ai-inference \
            chat-result \
            chat-on-demand-serving-mode \
            --compartment-id "$COMPARTMENTID" \
            --serving-mode-model-id "$MODEL" \
            --chat-request file://<(jq -n \
                --arg text "$PROMPT" \
                '{messages: [ { role: "USER", content: [ { type: "TEXT", text: $text } ] } ], apiFormat: "GENERIC"}'
            )
        )
        # Extract the GenAI answer (plain model name)
        extracted_text=$(echo "$chat_answer" | jq -r '.data["chat-response"].choices[0].message.content[0].text // empty')
        ai_answer="${extracted_text//\"/\"\"}"

        echo "$compartment_id,$model_id,\"$name_or_path\",\"$architectures\",\"$transformers_version\",\"$ai_answer\"" >> "$OUTPUT_CSV"
    done

    log "CSV written to $OUTPUT_CSV"
}

main() {
    log "=========== STARTING MODEL IMPORT/METADATA EXTRACTION ==========="
    validate_dependencies
    # Example usage in main script
    select_stage_and_region
    echo "Selected stage: $STAGE"
    echo "Selected namespace: $NAMESPACE"
    echo "Selected region: $REGION"

    # Local dir
    TARGET_DIR="$TARGET_BASE/${STAGE}_${REGION}"
    CONFIG_DIR="$TARGET_DIR/configs"
    OUTPUT_CSV="$TARGET_DIR/extracted_models.csv"

    mkdir -p "$CONFIG_DIR"
    LOG_FILE="$TARGET_DIR/script.log"
    exec > >(tee -a "$LOG_FILE") 2>&1

    log "Workspace: $TARGET_DIR"
    log "Config dir: $CONFIG_DIR"
    log "CSV will be written to: $OUTPUT_CSV"

    ready_models=$(get_ready_model_names)
    if ! echo "$ready_models" | grep -q '[^[:space:]]'; then
        log_err "No Ready models found, exiting."
        exit 1
    fi
    download_config_jsons "$ready_models"
    extract_and_analyze_configs
    log "=========== ALL DONE ==========="
}

main "$@"
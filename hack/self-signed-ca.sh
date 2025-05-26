#!/bin/bash

# OME Webhook Certificate Generator
# Generates self-signed CA certificates for OME webhook services
# Production-ready script with proper error handling and validation

set -o errexit
set -o pipefail
set -o nounset

# Global variables with defaults
readonly SCRIPT_NAME="$(basename "$0")"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Default configuration
SERVICE_NAME="ome-webhook-server-service"
NAMESPACE="ome"
SECRET_NAME="ome-webhook-server-cert"
WEBHOOK_DEPLOYMENT="ome-controller-manager"
CERT_VALIDITY_DAYS=365
VERBOSE=false
DRY_RUN=false

# Arrays for webhook configurations
declare -a MUTATING_WEBHOOK_NAMES=()
declare -a VALIDATING_WEBHOOK_NAMES=()

# Temporary directory for certificates (will be cleaned up)
TMPDIR=""

# Logging functions
log_info() {
    echo "[INFO] $*" >&2
}

log_warn() {
    echo "[WARN] $*" >&2
}

log_error() {
    echo "[ERROR] $*" >&2
}

log_debug() {
    if [[ "${VERBOSE}" == "true" ]]; then
        echo "[DEBUG] $*" >&2
    fi
}

# Cleanup function
cleanup() {
    local exit_code=$?
    if [[ -n "${TMPDIR}" && -d "${TMPDIR}" ]]; then
        log_debug "Cleaning up temporary directory: ${TMPDIR}"
        rm -rf "${TMPDIR}"
    fi
    exit $exit_code
}

# Set up cleanup trap
trap cleanup EXIT INT TERM

# Usage function
usage() {
    cat <<EOF
${SCRIPT_NAME} - Generate self-signed CA certificates for OME webhook services

USAGE:
    ${SCRIPT_NAME} [OPTIONS]

DESCRIPTION:
    This script generates self-signed CA certificates suitable for use with OME webhook services.
    It creates the necessary certificates, stores them in Kubernetes secrets, and patches the
    webhook configurations with the CA bundle.

    See: https://kubernetes.io/docs/concepts/cluster-administration/certificates/#distributing-self-signed-ca-certificate

OPTIONS:
    --service NAME              Service name of webhook (default: ${SERVICE_NAME})
    --namespace NAME            Namespace where webhook service and secret reside (default: ${NAMESPACE})
    --secret NAME               Secret name for CA certificate and server certificate/key pair (default: ${SECRET_NAME})
    --mutatingWebhookName NAME  Name for the mutating webhook config (can be specified multiple times)
    --validatingWebhookName NAME Name for the validating webhook config (can be specified multiple times)
    --webhookDeployment NAME    Deployment name of the webhook controller (default: ${WEBHOOK_DEPLOYMENT})
    --cert-validity-days DAYS   Certificate validity period in days (default: ${CERT_VALIDITY_DAYS})
    --dry-run                   Show what would be done without making changes
    --verbose                   Enable verbose logging
    --help                      Show this help message

EXAMPLES:
    # Generate certificates with default settings
    ${SCRIPT_NAME}

    # Generate certificates for custom service and namespace
    ${SCRIPT_NAME} --service my-webhook --namespace my-namespace

    # Add custom webhook configurations
    ${SCRIPT_NAME} --mutatingWebhookName custom.webhook.io --validatingWebhookName another.webhook.io

EOF
}

# Validate required tools
check_dependencies() {
    local missing_tools=()
    
    for tool in openssl kubectl jq; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            missing_tools+=("$tool")
        fi
    done
    
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_error "Please install the missing tools and try again"
        exit 1
    fi
    
    log_debug "All required tools are available"
}

# Validate Kubernetes connectivity
check_kubernetes_connectivity() {
    log_debug "Checking Kubernetes connectivity..."
    
    if ! kubectl cluster-info >/dev/null 2>&1; then
        log_error "Cannot connect to Kubernetes cluster"
        log_error "Please ensure kubectl is configured and you have cluster access"
        exit 1
    fi
    
    log_debug "Kubernetes connectivity verified"
}

# Validate namespace exists
validate_namespace() {
    local namespace="$1"
    
    log_debug "Validating namespace: ${namespace}"
    
    if ! kubectl get namespace "${namespace}" >/dev/null 2>&1; then
        log_error "Namespace '${namespace}' does not exist"
        log_error "Please create the namespace or specify an existing one"
        exit 1
    fi
    
    log_debug "Namespace '${namespace}' exists"
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --service)
                SERVICE_NAME="$2"
                shift 2
                ;;
            --namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            --secret)
                SECRET_NAME="$2"
                shift 2
                ;;
            --mutatingWebhookName)
                MUTATING_WEBHOOK_NAMES+=("$2")
                shift 2
                ;;
            --validatingWebhookName)
                VALIDATING_WEBHOOK_NAMES+=("$2")
                shift 2
                ;;
            --webhookDeployment)
                WEBHOOK_DEPLOYMENT="$2"
                shift 2
                ;;
            --cert-validity-days)
                CERT_VALIDITY_DAYS="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Set default webhook names if none provided
set_default_webhook_names() {
    if [[ ${#VALIDATING_WEBHOOK_NAMES[@]} -eq 0 ]]; then
        VALIDATING_WEBHOOK_NAMES=(
            "inferenceservice.ome.io"
            "servingruntime.ome.io"
            "clusterservingruntime.ome.io"
        )
    fi
    
    if [[ ${#MUTATING_WEBHOOK_NAMES[@]} -eq 0 ]]; then
        MUTATING_WEBHOOK_NAMES=("inferenceservice.ome.io")
    fi
}

# Validate configuration
validate_configuration() {
    log_debug "Validating configuration..."
    
    # Validate certificate validity days
    if ! [[ "${CERT_VALIDITY_DAYS}" =~ ^[0-9]+$ ]] || [[ "${CERT_VALIDITY_DAYS}" -lt 1 ]]; then
        log_error "Certificate validity days must be a positive integer"
        exit 1
    fi
    
    # Validate service name format
    if [[ ! "${SERVICE_NAME}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
        log_error "Service name '${SERVICE_NAME}' is not a valid Kubernetes service name"
        exit 1
    fi
    
    # Validate namespace format
    if [[ ! "${NAMESPACE}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
        log_error "Namespace '${NAMESPACE}' is not a valid Kubernetes namespace name"
        exit 1
    fi
    
    log_debug "Configuration validation passed"
}

# Display current configuration
display_configuration() {
    log_info "Configuration:"
    log_info "  Service: ${SERVICE_NAME}"
    log_info "  Namespace: ${NAMESPACE}"
    log_info "  Secret: ${SECRET_NAME}"
    log_info "  Webhook Deployment: ${WEBHOOK_DEPLOYMENT}"
    log_info "  Certificate Validity: ${CERT_VALIDITY_DAYS} days"
    log_info "  Mutating Webhooks: ${MUTATING_WEBHOOK_NAMES[*]}"
    log_info "  Validating Webhooks: ${VALIDATING_WEBHOOK_NAMES[*]}"
    log_info "  Dry Run: ${DRY_RUN}"
}

# Create temporary directory
create_temp_directory() {
    TMPDIR=$(mktemp -d)
    log_debug "Created temporary directory: ${TMPDIR}"
}

# Generate CSR configuration
generate_csr_config() {
    local csr_config="${TMPDIR}/csr.conf"
    
    log_debug "Generating CSR configuration: ${csr_config}"
    
    cat > "${csr_config}" <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster
DNS.5 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF
    
    log_debug "CSR configuration generated successfully"
}

# Generate CA certificate and key
generate_ca_certificate() {
    local ca_key="${TMPDIR}/ca.key"
    local ca_crt="${TMPDIR}/ca.crt"
    local subject="/CN=${SERVICE_NAME}.${NAMESPACE}.svc"
    
    log_info "Generating CA certificate and key..."
    
    # Generate CA private key
    if ! openssl genrsa -out "${ca_key}" 2048 2>/dev/null; then
        log_error "Failed to generate CA private key"
        exit 1
    fi
    
    # Generate CA certificate
    if ! openssl req -x509 -newkey rsa:2048 -key "${ca_key}" -out "${ca_crt}" \
        -days "${CERT_VALIDITY_DAYS}" -nodes -subj "${subject}" 2>/dev/null; then
        log_error "Failed to generate CA certificate"
        exit 1
    fi
    
    log_debug "CA certificate and key generated successfully"
}

# Generate server certificate and key
generate_server_certificate() {
    local server_key="${TMPDIR}/server.key"
    local server_csr="${TMPDIR}/server.csr"
    local server_crt="${TMPDIR}/server.crt"
    local ca_key="${TMPDIR}/ca.key"
    local ca_crt="${TMPDIR}/ca.crt"
    local csr_config="${TMPDIR}/csr.conf"
    local subject="/CN=${SERVICE_NAME}.${NAMESPACE}.svc"
    
    log_info "Generating server certificate and key..."
    
    # Generate server private key
    if ! openssl genrsa -out "${server_key}" 2048 2>/dev/null; then
        log_error "Failed to generate server private key"
        exit 1
    fi
    
    # Generate certificate signing request
    if ! openssl req -new -key "${server_key}" -subj "${subject}" \
        -out "${server_csr}" -config "${csr_config}" 2>/dev/null; then
        log_error "Failed to generate certificate signing request"
        exit 1
    fi
    
    # Sign the certificate
    if ! openssl x509 -extensions v3_req -req -days "${CERT_VALIDITY_DAYS}" \
        -in "${server_csr}" -CA "${ca_crt}" -CAkey "${ca_key}" -CAcreateserial \
        -out "${server_crt}" -extfile "${csr_config}" 2>/dev/null; then
        log_error "Failed to sign server certificate"
        exit 1
    fi
    
    log_debug "Server certificate and key generated successfully"
}

# Create or update Kubernetes secret
create_kubernetes_secret() {
    local server_key="${TMPDIR}/server.key"
    local server_crt="${TMPDIR}/server.crt"
    
    log_info "Creating Kubernetes secret: ${SECRET_NAME}"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "[DRY RUN] Would create secret '${SECRET_NAME}' in namespace '${NAMESPACE}'"
        return
    fi
    
    # Create the secret
    if ! kubectl create secret generic "${SECRET_NAME}" \
        --from-file=tls.key="${server_key}" \
        --from-file=tls.crt="${server_crt}" \
        --dry-run=client -o yaml | kubectl -n "${NAMESPACE}" apply -f - >/dev/null; then
        log_error "Failed to create Kubernetes secret"
        exit 1
    fi
    
    log_debug "Kubernetes secret created successfully"
}

# Restart webhook pods
restart_webhook_pods() {
    log_info "Restarting webhook pods to reload certificates..."
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "[DRY RUN] Would restart webhook pods with deployment: ${WEBHOOK_DEPLOYMENT}"
        return
    fi
    
    # Get webhook pods
    local webhook_pods
    webhook_pods=$(kubectl get pods -n "${NAMESPACE}" -l "app=${WEBHOOK_DEPLOYMENT}" -o name 2>/dev/null || true)
    
    if [[ -z "${webhook_pods}" ]]; then
        log_warn "No webhook pods found for deployment: ${WEBHOOK_DEPLOYMENT}"
        return
    fi
    
    # Delete pods to trigger restart
    for pod in ${webhook_pods}; do
        if kubectl delete "${pod}" -n "${NAMESPACE}" >/dev/null 2>&1; then
            log_debug "Restarted pod: ${pod}"
        else
            log_warn "Failed to restart pod: ${pod}"
        fi
    done
    
    log_debug "Webhook pods restart initiated"
}

# Get CA bundle for webhook patching
get_ca_bundle() {
    local ca_crt="${TMPDIR}/ca.crt"
    
    if [[ ! -f "${ca_crt}" ]]; then
        log_error "CA certificate not found: ${ca_crt}"
        exit 1
    fi
    
    # Base64 encode the CA certificate
    openssl enc -a -A < "${ca_crt}"
}

# Build JSON patch string for webhooks
build_webhook_patch() {
    local webhook_count="$1"
    local ca_bundle="$2"
    local patch_string='['
    
    for ((i=0; i<webhook_count; i++)); do
        patch_string+="{\"op\": \"replace\", \"path\": \"/webhooks/${i}/clientConfig/caBundle\", \"value\":\"${ca_bundle}\"}"
        if [[ $i -lt $((webhook_count - 1)) ]]; then
            patch_string+=", "
        fi
    done
    
    patch_string+=']'
    echo "${patch_string//\$\{ca_bundle\}/$ca_bundle}"
}

# Patch mutating webhook configurations
patch_mutating_webhooks() {
    local ca_bundle="$1"
    
    for webhook_name in "${MUTATING_WEBHOOK_NAMES[@]}"; do
        log_info "Patching mutating webhook: ${webhook_name}"
        
        if [[ "${DRY_RUN}" == "true" ]]; then
            log_info "[DRY RUN] Would patch mutating webhook: ${webhook_name}"
            continue
        fi
        
        # Check if webhook exists
        if ! kubectl get mutatingwebhookconfiguration "${webhook_name}" >/dev/null 2>&1; then
            log_warn "Mutating webhook configuration not found: ${webhook_name}"
            continue
        fi
        
        # Get webhook count
        local webhook_count
        webhook_count=$(kubectl get mutatingwebhookconfiguration "${webhook_name}" -o json | jq -r '.webhooks | length')
        
        if [[ "${webhook_count}" == "0" || "${webhook_count}" == "null" ]]; then
            log_warn "No webhooks found in configuration: ${webhook_name}"
            continue
        fi
        
        # Build and apply patch
        local patch_string
        patch_string=$(build_webhook_patch "${webhook_count}" "${ca_bundle}")
        
        if ! kubectl patch mutatingwebhookconfiguration "${webhook_name}" \
            --type='json' -p="${patch_string}" >/dev/null; then
            log_error "Failed to patch mutating webhook: ${webhook_name}"
            exit 1
        fi
        
        log_debug "Successfully patched mutating webhook: ${webhook_name}"
    done
}

# Patch validating webhook configurations
patch_validating_webhooks() {
    local ca_bundle="$1"
    
    for webhook_name in "${VALIDATING_WEBHOOK_NAMES[@]}"; do
        log_info "Patching validating webhook: ${webhook_name}"
        
        if [[ "${DRY_RUN}" == "true" ]]; then
            log_info "[DRY RUN] Would patch validating webhook: ${webhook_name}"
            continue
        fi
        
        # Check if webhook exists
        if ! kubectl get validatingwebhookconfiguration "${webhook_name}" >/dev/null 2>&1; then
            log_warn "Validating webhook configuration not found: ${webhook_name}"
            continue
        fi
        
        # Get webhook count
        local webhook_count
        webhook_count=$(kubectl get validatingwebhookconfiguration "${webhook_name}" -o json | jq -r '.webhooks | length')
        
        if [[ "${webhook_count}" == "0" || "${webhook_count}" == "null" ]]; then
            log_warn "No webhooks found in configuration: ${webhook_name}"
            continue
        fi
        
        # Build and apply patch
        local patch_string
        patch_string=$(build_webhook_patch "${webhook_count}" "${ca_bundle}")
        
        if ! kubectl patch validatingwebhookconfiguration "${webhook_name}" \
            --type='json' -p="${patch_string}" >/dev/null; then
            log_error "Failed to patch validating webhook: ${webhook_name}"
            exit 1
        fi
        
        log_debug "Successfully patched validating webhook: ${webhook_name}"
    done
}

# Patch conversion webhook configuration
patch_conversion_webhook() {
    local ca_bundle="$1"
    local crd_name="inferenceservices.ome.io"
    
    log_info "Patching conversion webhook for CRD: ${crd_name}"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "[DRY RUN] Would patch conversion webhook for CRD: ${crd_name}"
        return
    fi
    
    # Check if CRD exists
    if ! kubectl get crd "${crd_name}" >/dev/null 2>&1; then
        log_warn "CustomResourceDefinition not found: ${crd_name}"
        return
    fi
    
    # Build patch string
    local patch_string="[{\"op\": \"replace\", \"path\": \"/spec/conversion/webhook/clientConfig/caBundle\", \"value\":\"${ca_bundle}\"}]"
    
    if ! kubectl patch crd "${crd_name}" --type='json' -p="${patch_string}" >/dev/null; then
        log_error "Failed to patch conversion webhook for CRD: ${crd_name}"
        exit 1
    fi
    
    log_debug "Successfully patched conversion webhook for CRD: ${crd_name}"
}

# Display certificate information
display_certificate_info() {
    local ca_crt="${TMPDIR}/ca.crt"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "[DRY RUN] Certificate information would be displayed here"
        return
    fi
    
    log_info "Generated CA Certificate:"
    cat "${ca_crt}"
    echo
    
    log_info "Base64 Encoded CA Bundle:"
    get_ca_bundle
    echo
}

# Main function
main() {
    log_info "Starting OME webhook certificate generation..."
    
    # Parse command line arguments
    parse_arguments "$@"
    
    # Set default webhook names if none provided
    set_default_webhook_names
    
    # Validate configuration
    validate_configuration
    
    # Display configuration
    display_configuration
    
    # Check dependencies
    check_dependencies
    
    # Check Kubernetes connectivity
    check_kubernetes_connectivity
    
    # Validate namespace
    validate_namespace "${NAMESPACE}"
    
    # Create temporary directory
    create_temp_directory
    
    # Generate certificates
    generate_csr_config
    generate_ca_certificate
    generate_server_certificate
    
    # Create Kubernetes secret
    create_kubernetes_secret
    
    # Restart webhook pods
    restart_webhook_pods
    
    # Get CA bundle for patching
    local ca_bundle
    ca_bundle=$(get_ca_bundle)
    
    # Patch webhook configurations
    patch_mutating_webhooks "${ca_bundle}"
    patch_validating_webhooks "${ca_bundle}"
    patch_conversion_webhook "${ca_bundle}"
    
    # Display certificate information
    display_certificate_info
    
    log_info "OME webhook certificate generation completed successfully!"
}

# Run main function with all arguments
main "$@"

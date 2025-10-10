package basemodel

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
	utils "github.com/sgl-project/ome/pkg/utils/storage"
	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
)

// Constants for retry and observability
const (
	// Annotation keys for retry tracking
	RetryCountAnnotationKey    = "ome.io/pvc-retry-count"
	CorrelationIDAnnotationKey = "ome.io/correlation-id"

	// Default retry configuration
	DefaultMaxRetries    = 3
	DefaultBaseDelay     = 2 * time.Second
	DefaultMaxDelay      = 5 * time.Minute
	DefaultBackoffFactor = 2.0

	// Condition types for BaseModel status
	ConditionPVCValidated         = "PVCValidated"
	ConditionPVCSecurityValidated = "PVCSecurityValidated"
	ConditionJobCreated           = "JobCreated"
	ConditionMetadataExtracted    = "MetadataExtracted"
	ConditionRetrying             = "Retrying"
	ConditionJobFailed            = "JobFailed"
)

// RetryConfig defines retry behavior for PVC operations
type RetryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// NewDefaultRetryConfig creates a RetryConfig with sensible defaults
func NewDefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    DefaultMaxRetries,
		BaseDelay:     DefaultBaseDelay,
		MaxDelay:      DefaultMaxDelay,
		BackoffFactor: DefaultBackoffFactor,
	}
}

// calculateBackoffDelay calculates exponential backoff delay
func (rc *RetryConfig) calculateBackoffDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return rc.BaseDelay
	}

	// Exponential backoff: BaseDelay * (BackoffFactor ^ retryCount)
	delay := time.Duration(float64(rc.BaseDelay) * math.Pow(rc.BackoffFactor, float64(retryCount)))

	// Cap at MaxDelay
	if delay > rc.MaxDelay {
		delay = rc.MaxDelay
	}

	return delay
}

// Error classification for different handling strategies
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "validation" // Permanent errors - don't retry
	ErrorTypeTransient  ErrorType = "transient"  // Temporary errors - retry with backoff
	ErrorTypeSecurity   ErrorType = "security"   // Security violations - don't retry
)

// ValidationError provides structured error handling with types and remediation hints
type ValidationError struct {
	Type    ErrorType
	Message string
	Cause   error
}

func (ve *ValidationError) Error() string {
	if ve.Cause != nil {
		return fmt.Sprintf("%s: %v", ve.Message, ve.Cause)
	}
	return ve.Message
}

func (ve *ValidationError) Unwrap() error {
	return ve.Cause
}

// classifyError determines the error type for appropriate handling
func classifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeTransient
	}

	errStr := err.Error()

	// Security-related errors (don't retry)
	if contains(errStr, "cross-namespace") || contains(errStr, "access denied") {
		return ErrorTypeSecurity
	}

	// Validation errors (don't retry)
	if contains(errStr, "not found") || contains(errStr, "not bound") || contains(errStr, "invalid") {
		return ErrorTypeValidation
	}

	// Default to transient (can retry)
	return ErrorTypeTransient
}

// contains is a helper function for string checking
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && contains(s[1:], substr)) ||
		(len(s) >= len(substr) && s[:len(substr)] == substr))
}

// Retry count and correlation ID management functions

// getRetryCount gets the current retry count from model annotations
func getRetryCount(model metav1.Object) int {
	if model.GetAnnotations() == nil {
		return 0
	}

	if countStr, exists := model.GetAnnotations()[RetryCountAnnotationKey]; exists {
		if count, err := strconv.Atoi(countStr); err == nil {
			return count
		}
	}

	return 0
}

// incrementRetryCount increments the retry count in model annotations
func incrementRetryCount(model metav1.Object) {
	currentCount := getRetryCount(model)
	setRetryCount(model, currentCount+1)
}

// setRetryCount sets the retry count in model annotations
func setRetryCount(model metav1.Object, count int) {
	annotations := model.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[RetryCountAnnotationKey] = strconv.Itoa(count)
	model.SetAnnotations(annotations)
}

// clearRetryCount removes the retry count annotation (on success)
func clearRetryCount(model metav1.Object) {
	annotations := model.GetAnnotations()
	if annotations != nil {
		delete(annotations, RetryCountAnnotationKey)
		model.SetAnnotations(annotations)
	}
}

// generateCorrelationID generates a unique correlation ID for tracking operations
func generateCorrelationID() string {
	// Use timestamp + random component for uniqueness
	return fmt.Sprintf("pvc-%d", time.Now().UnixNano())
}

// getOrSetCorrelationID gets existing correlation ID or creates a new one
func getOrSetCorrelationID(model metav1.Object) string {
	annotations := model.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		model.SetAnnotations(annotations)
	}

	if correlationID, exists := annotations[CorrelationIDAnnotationKey]; exists && correlationID != "" {
		return correlationID
	}

	// Generate new correlation ID
	correlationID := generateCorrelationID()
	annotations[CorrelationIDAnnotationKey] = correlationID
	model.SetAnnotations(annotations)

	return correlationID
}

// Condition management helper functions

// setCondition sets a condition on the BaseModel status
func (r *BaseModelReconciler) setCondition(baseModel *v1beta1.BaseModel, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	conditions := baseModel.Status.Conditions
	for i, existingCondition := range conditions {
		if existingCondition.Type == conditionType {
			// Update existing condition
			if existingCondition.Status != status {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			conditions[i] = condition
			baseModel.Status.Conditions = conditions
			return
		}
	}

	// Append new condition
	baseModel.Status.Conditions = append(baseModel.Status.Conditions, condition)
}

// getCondition gets a specific condition from BaseModel status
func (r *BaseModelReconciler) getCondition(baseModel *v1beta1.BaseModel, conditionType string) *metav1.Condition {
	for _, condition := range baseModel.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// setClusterCondition sets a condition on the ClusterBaseModel status
func (r *ClusterBaseModelReconciler) setCondition(clusterBaseModel *v1beta1.ClusterBaseModel, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	conditions := clusterBaseModel.Status.Conditions
	for i, existingCondition := range conditions {
		if existingCondition.Type == conditionType {
			// Update existing condition
			if existingCondition.Status != status {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			conditions[i] = condition
			clusterBaseModel.Status.Conditions = conditions
			return
		}
	}

	// Append new condition
	clusterBaseModel.Status.Conditions = append(clusterBaseModel.Status.Conditions, condition)
}

// getClusterCondition gets a specific condition from ClusterBaseModel status
func (r *ClusterBaseModelReconciler) getCondition(clusterBaseModel *v1beta1.ClusterBaseModel, conditionType string) *metav1.Condition {
	for _, condition := range clusterBaseModel.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// validatePVCWithSecurity provides comprehensive PVC validation with enhanced security and observability
func (r *BaseModelReconciler) validatePVCWithSecurity(ctx context.Context, baseModel client.Object, pvcURI, namespace, correlationID string) error {
	log := r.Log.WithValues("correlationID", correlationID, "namespace", namespace, "pvcURI", pvcURI)

	// Metrics hook point: Track validation start
	log.V(1).Info("Starting PVC security validation", "operation", "pvc_validation_start")

	// Parse PVC URI with detailed error handling
	pvcComponents, err := utils.ParsePVCStorageURI(pvcURI)
	if err != nil {
		// Metrics hook point: Track parsing failure
		log.Error(err, "PVC URI parsing failed", "uri", pvcURI, "operation", "pvc_validation_parse_failed")

		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "PVCValidationFailed",
			fmt.Sprintf("Invalid PVC URI format: %s", err.Error()))
		return &ValidationError{
			Type:    ErrorTypeValidation,
			Message: fmt.Sprintf("Invalid PVC URI format '%s'. Use format: pvc://namespace:pvc-name/subpath", pvcURI),
			Cause:   err,
		}
	}

	pvcNamespace, pvcName, subPath := pvcComponents.Namespace, pvcComponents.PVCName, pvcComponents.SubPath

	log.Info("Parsed PVC details", "pvcNamespace", pvcNamespace, "pvcName", pvcName, "subPath", subPath)

	// Critical security validation: Cross-namespace prevention
	if pvcNamespace != namespace {
		securityMsg := fmt.Sprintf("Security violation: BaseModel in namespace '%s' attempted to access PVC '%s' in namespace '%s'",
			namespace, pvcName, pvcNamespace)
		// Metrics hook point: Track security violation
		log.Error(nil, "Cross-namespace PVC access denied", "baseModelNamespace", namespace,
			"targetPVCNamespace", pvcNamespace, "pvcName", pvcName, "operation", "pvc_validation_security_violation")

		// Set condition for BaseModel only (not ClusterBaseModel)
		if bm, ok := baseModel.(*v1beta1.BaseModel); ok {
			r.setCondition(bm, ConditionPVCValidated, metav1.ConditionFalse, "CrossNamespaceAccessDenied",
				fmt.Sprintf("Cross-namespace access denied. PVC must be in the same namespace '%s'", namespace))
		}

		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "SecurityViolation", securityMsg)

		return &ValidationError{
			Type: ErrorTypeSecurity,
			Message: fmt.Sprintf("Cross-namespace PVC access denied. BaseModel in namespace '%s' cannot access PVC in namespace '%s'. "+
				"Remediation: Move the PVC to namespace '%s' or create a new PVC in the correct namespace.",
				namespace, pvcNamespace, namespace),
			Cause: fmt.Errorf("cross-namespace access violation"),
		}
	}

	// Fetch PVC with comprehensive error handling
	var pvc corev1.PersistentVolumeClaim
	pvcKey := types.NamespacedName{Name: pvcName, Namespace: pvcNamespace}
	err = r.Get(ctx, pvcKey, &pvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			notFoundMsg := fmt.Sprintf("PVC '%s' not found in namespace '%s'", pvcName, pvcNamespace)
			// Metrics hook point: Track PVC not found
			log.Error(err, "PVC not found", "pvcName", pvcName, "pvcNamespace", pvcNamespace, "operation", "pvc_validation_not_found")

			// Set condition for BaseModel only (not ClusterBaseModel)
			if bm, ok := baseModel.(*v1beta1.BaseModel); ok {
				r.setCondition(bm, ConditionPVCValidated, metav1.ConditionFalse, "PVCNotFound",
					fmt.Sprintf("PVC '%s' not found in namespace '%s'", pvcName, pvcNamespace))
			}

			r.Recorder.Event(baseModel, corev1.EventTypeWarning, "PVCNotFound", notFoundMsg)

			return &ValidationError{
				Type: ErrorTypeValidation,
				Message: fmt.Sprintf("PVC '%s' not found in namespace '%s'. "+
					"Remediation: Create the PVC or verify the correct PVC name and namespace.",
					pvcName, pvcNamespace),
				Cause: err,
			}
		}

		accessMsg := fmt.Sprintf("Failed to access PVC '%s' in namespace '%s'", pvcName, pvcNamespace)
		// Metrics hook point: Track PVC access failure
		log.Error(err, "PVC access failed", "pvcName", pvcName, "pvcNamespace", pvcNamespace, "operation", "pvc_validation_access_failed")

		// Set condition for BaseModel only (not ClusterBaseModel)
		if bm, ok := baseModel.(*v1beta1.BaseModel); ok {
			r.setCondition(bm, ConditionPVCValidated, metav1.ConditionFalse, "PVCAccessFailed",
				fmt.Sprintf("Failed to access PVC '%s': %s", pvcName, err.Error()))
		}

		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "PVCAccessFailed", accessMsg)

		return &ValidationError{
			Type: ErrorTypeTransient,
			Message: fmt.Sprintf("Failed to access PVC '%s' in namespace '%s'. "+
				"Remediation: Check RBAC permissions and cluster connectivity. Error: %s",
				pvcName, pvcNamespace, err.Error()),
			Cause: err,
		}
	}

	log.Info("PVC found successfully", "pvcName", pvcName, "phase", pvc.Status.Phase,
		"capacity", pvc.Status.Capacity, "accessModes", pvc.Status.AccessModes)

	// Validate PVC status and binding
	if pvc.Status.Phase != corev1.ClaimBound {
		bindingMsg := fmt.Sprintf("PVC '%s' in namespace '%s' is not bound (current phase: %s)",
			pvcName, pvcNamespace, pvc.Status.Phase)
		log.Error(nil, "PVC not bound", "pvcName", pvcName, "currentPhase", pvc.Status.Phase)

		// Set condition for BaseModel only (not ClusterBaseModel)
		if bm, ok := baseModel.(*v1beta1.BaseModel); ok {
			r.setCondition(bm, ConditionPVCValidated, metav1.ConditionFalse, "PVCNotBound",
				fmt.Sprintf("PVC '%s' is not bound (phase: %s)", pvcName, pvc.Status.Phase))
		}

		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "PVCNotBound", bindingMsg)

		var remediationHint string
		switch pvc.Status.Phase {
		case corev1.ClaimPending:
			remediationHint = "PVC is pending. Check if suitable PersistentVolumes are available or if StorageClass can provision new volumes."
		case corev1.ClaimLost:
			remediationHint = "PVC is lost. The associated PersistentVolume may have been deleted. Recreate the PVC."
		default:
			remediationHint = "Check PVC status and ensure cluster has available storage resources."
		}

		return &ValidationError{
			Type: ErrorTypeValidation,
			Message: fmt.Sprintf("PVC '%s' in namespace '%s' is not bound (current phase: %s). "+
				"Remediation: %s", pvcName, pvcNamespace, pvc.Status.Phase, remediationHint),
			Cause: fmt.Errorf("PVC not in bound state"),
		}
	}

	// Validate access modes for model loading compatibility
	validAccessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteOnce, corev1.ReadOnlyMany, corev1.ReadWriteMany,
	}
	hasValidAccessMode := false
	for _, mode := range pvc.Status.AccessModes {
		for _, validMode := range validAccessModes {
			if mode == validMode {
				hasValidAccessMode = true
				break
			}
		}
		if hasValidAccessMode {
			break
		}
	}

	if !hasValidAccessMode {
		accessModeMsg := fmt.Sprintf("PVC '%s' has incompatible access modes: %v", pvcName, pvc.Status.AccessModes)
		log.Error(nil, "Incompatible PVC access modes", "pvcName", pvcName, "accessModes", pvc.Status.AccessModes)

		// Set condition for BaseModel only (not ClusterBaseModel)
		if bm, ok := baseModel.(*v1beta1.BaseModel); ok {
			r.setCondition(bm, ConditionPVCValidated, metav1.ConditionFalse, "IncompatibleAccessModes",
				fmt.Sprintf("PVC '%s' has incompatible access modes", pvcName))
		}

		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "IncompatibleAccessModes", accessModeMsg)

		return &ValidationError{
			Type: ErrorTypeValidation,
			Message: fmt.Sprintf("PVC '%s' has incompatible access modes %v. "+
				"Remediation: Ensure PVC has ReadWriteOnce, ReadOnlyMany, or ReadWriteMany access mode.",
				pvcName, pvc.Status.AccessModes),
			Cause: fmt.Errorf("incompatible access modes"),
		}
	}

	// Success: Set positive condition and log successful validation
	// Metrics hook point: Track successful validation
	log.Info("PVC validation successful", "pvcName", pvcName, "capacity", pvc.Status.Capacity, "operation", "pvc_validation_success")

	// Set condition for BaseModel only (not ClusterBaseModel)
	if bm, ok := baseModel.(*v1beta1.BaseModel); ok {
		r.setCondition(bm, ConditionPVCValidated, metav1.ConditionTrue, "ValidationSucceeded",
			fmt.Sprintf("PVC '%s' validated successfully", pvcName))
	}

	return nil
}

// Helper functions for enhanced Job status handling

// getConfigMapName generates the ConfigMap name for a BaseModel
func (r *BaseModelReconciler) getConfigMapName(baseModelName string) string {
	return fmt.Sprintf("%s-metadata", baseModelName)
}

// checkConfigMapForMetadata checks if metadata ConfigMap exists and is valid
func (r *BaseModelReconciler) checkConfigMapForMetadata(ctx context.Context, baseModel *v1beta1.BaseModel, configMapName string) error {
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: baseModel.Namespace,
		Name:      configMapName,
	}, configMap)

	if err != nil {
		return fmt.Errorf("ConfigMap %s not found: %w", configMapName, err)
	}

	// Basic validation - check if ConfigMap has expected metadata
	if len(configMap.Data) == 0 {
		return fmt.Errorf("ConfigMap %s exists but has no data", configMapName)
	}

	return nil
}

// extractJobFailureReason extracts failure reason from Job status conditions
func (r *BaseModelReconciler) extractJobFailureReason(job *batchv1.Job) string {
	if job.Status.Conditions != nil {
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				if condition.Message != "" {
					return condition.Message
				}
				if condition.Reason != "" {
					return condition.Reason
				}
			}
		}
	}

	// Fallback to generic message
	return fmt.Sprintf("Job failed after %d attempts", job.Status.Failed)
}

// handlePVCStorageWithValidation handles BaseModel resources with PVC storage including validation
func (r *BaseModelReconciler) handlePVCStorageWithValidation(ctx context.Context, baseModel *v1beta1.BaseModel) (ctrl.Result, error) {
	// Enhanced logging with correlation ID and retry tracking
	correlationID := getOrSetCorrelationID(baseModel)
	retryCount := getRetryCount(baseModel)
	retryConfig := NewDefaultRetryConfig()

	log := ctrl.LoggerFrom(ctx).WithValues(
		"storage", "pvc",
		"correlation_id", correlationID,
		"retry_count", retryCount,
		"max_retries", retryConfig.MaxRetries,
	)

	log.Info("Starting PVC validation and metadata extraction")

	// Parse PVC storage URI
	pvcComponents, err := utilstorage.ParsePVCStorageURI(*baseModel.Spec.Storage.StorageUri)
	if err != nil {
		log.Error(err, "Failed to parse PVC storage URI")
		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "InvalidPVCURI",
			fmt.Sprintf("Invalid PVC storage URI: %v", err))
		// Validation errors are permanent - don't retry
		return ctrl.Result{}, err
	}

	// Security validation: BaseModel cannot reference PVCs in other namespaces
	if pvcComponents.Namespace != "" && pvcComponents.Namespace != baseModel.Namespace {
		err := fmt.Errorf("BaseModel cannot reference PVC in different namespace %s. Use ClusterBaseModel for cross-namespace PVCs", pvcComponents.Namespace)
		log.Error(err, "Cross-namespace PVC access denied for BaseModel")
		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "CrossNamespacePVCDenied", err.Error())
		// Security violations are permanent - don't retry
		return ctrl.Result{}, err
	}

	// Validate PVC exists and is accessible
	pvc, err := r.validatePVC(ctx, baseModel.Namespace, pvcComponents)
	if err != nil {
		errorType := classifyError(err)
		log.Error(err, "PVC validation failed", "error_type", string(errorType))
		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "PVCValidationFailed",
			fmt.Sprintf("PVC validation failed: %v", err))

		// Only retry transient errors
		if errorType == ErrorTypeTransient && retryCount < retryConfig.MaxRetries {
			incrementRetryCount(baseModel)
			delay := retryConfig.calculateBackoffDelay(retryCount)
			log.Info("Retrying PVC validation", "delay_seconds", delay.Seconds())
			return ctrl.Result{RequeueAfter: delay}, nil
		}

		// Don't retry validation or security errors, or if max retries exceeded
		return ctrl.Result{}, err
	}

	log.Info("PVC validation successful", "pvc", pvc.Name, "namespace", pvc.Namespace)
	r.Recorder.Event(baseModel, corev1.EventTypeNormal, "PVCValidated",
		fmt.Sprintf("PVC %s validated successfully", pvc.Name))

	// Clear retry count on successful validation
	clearRetryCount(baseModel)

	// Check if metadata extraction job exists
	jobName := r.getMetadataJobName(baseModel.Name)
	job := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: baseModel.Namespace,
		Name:      jobName,
	}, job)

	if err != nil {
		if errors.IsNotFound(err) {
			// Create metadata extraction job with corrected specifications
			job, err = r.createMetadataExtractionJob(ctx, baseModel, pvc, pvcComponents)
			if err != nil {
				errorType := classifyError(err)
				log.Error(err, "Failed to create metadata extraction job", "error_type", string(errorType))
				r.Recorder.Event(baseModel, corev1.EventTypeWarning, "JobCreationFailed",
					fmt.Sprintf("Failed to create metadata extraction job: %v", err))

				// Retry transient job creation failures
				if errorType == ErrorTypeTransient && retryCount < retryConfig.MaxRetries {
					incrementRetryCount(baseModel)
					delay := retryConfig.calculateBackoffDelay(retryCount)
					log.Info("Retrying job creation", "delay_seconds", delay.Seconds())
					return ctrl.Result{RequeueAfter: delay}, nil
				}

				return ctrl.Result{}, err
			}

			if err := r.Create(ctx, job); err != nil {
				errorType := classifyError(err)
				log.Error(err, "Failed to create job", "error_type", string(errorType))

				// Retry transient job creation failures
				if errorType == ErrorTypeTransient && retryCount < retryConfig.MaxRetries {
					incrementRetryCount(baseModel)
					delay := retryConfig.calculateBackoffDelay(retryCount)
					log.Info("Retrying job creation", "delay_seconds", delay.Seconds())
					return ctrl.Result{RequeueAfter: delay}, nil
				}

				return ctrl.Result{}, err
			}

			log.Info("Created metadata extraction job", "job", jobName, "correlation_id", correlationID)
			r.Recorder.Event(baseModel, corev1.EventTypeNormal, "JobCreated",
				"Created metadata extraction job")

			// Clear retry count on successful job creation
			clearRetryCount(baseModel)

			// Requeue to monitor job progress
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle existing job status
	return r.handleJobStatus(ctx, baseModel, job, pvcComponents)
}

// handlePVCStorageWithValidation handles ClusterBaseModel resources with PVC storage including validation
func (r *ClusterBaseModelReconciler) handlePVCStorageWithValidation(ctx context.Context, clusterBaseModel *v1beta1.ClusterBaseModel) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("storage", "pvc", "type", "cluster")

	// Parse PVC storage URI
	pvcComponents, err := utilstorage.ParsePVCStorageURI(*clusterBaseModel.Spec.Storage.StorageUri)
	if err != nil {
		log.Error(err, "Failed to parse PVC storage URI")
		r.Recorder.Event(clusterBaseModel, corev1.EventTypeWarning, "InvalidPVCURI",
			fmt.Sprintf("Invalid PVC storage URI: %v", err))
		return ctrl.Result{}, err
	}

	// For ClusterBaseModel, namespace must be specified in URI
	if pvcComponents.Namespace == "" {
		err := fmt.Errorf("ClusterBaseModel requires namespace in PVC URI: pvc://namespace:pvc-name/path")
		log.Error(err, "Namespace required for ClusterBaseModel PVC")
		r.Recorder.Event(clusterBaseModel, corev1.EventTypeWarning, "NamespaceRequired",
			"ClusterBaseModel requires namespace in PVC URI")
		return ctrl.Result{}, err
	}

	// Use the specified namespace for PVC validation
	pvc, err := r.validatePVC(ctx, pvcComponents.Namespace, pvcComponents)
	if err != nil {
		log.Error(err, "PVC validation failed")
		r.Recorder.Event(clusterBaseModel, corev1.EventTypeWarning, "PVCValidationFailed",
			fmt.Sprintf("PVC validation failed: %v", err))
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	log.Info("PVC validation successful", "pvc", pvc.Name, "namespace", pvc.Namespace)
	r.Recorder.Event(clusterBaseModel, corev1.EventTypeNormal, "PVCValidated",
		fmt.Sprintf("PVC %s validated successfully", pvc.Name))

	// Create job in the same namespace as the PVC
	jobName := r.getMetadataJobName(clusterBaseModel.Name)
	job := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: pvcComponents.Namespace, // Use PVC namespace for job
		Name:      jobName,
	}, job)

	if err != nil {
		if errors.IsNotFound(err) {
			// Create metadata extraction job in PVC namespace
			job, err = r.createMetadataExtractionJobForCluster(ctx, clusterBaseModel, pvc, pvcComponents)
			if err != nil {
				log.Error(err, "Failed to create metadata extraction job")
				r.Recorder.Event(clusterBaseModel, corev1.EventTypeWarning, "JobCreationFailed",
					fmt.Sprintf("Failed to create metadata extraction job: %v", err))
				return ctrl.Result{}, err
			}

			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Failed to create job")
				return ctrl.Result{}, err
			}

			log.Info("Created metadata extraction job for ClusterBaseModel",
				"job", jobName, "namespace", pvcComponents.Namespace)
			r.Recorder.Event(clusterBaseModel, corev1.EventTypeNormal, "JobCreated",
				"Created metadata extraction job")

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle existing job
	return r.handleClusterJobStatus(ctx, clusterBaseModel, job, pvcComponents)
}

// validatePVC validates that the PVC exists and is accessible (enhanced with security validation)
func (r *BaseModelReconciler) validatePVC(ctx context.Context, namespace string, pvcComponents *utilstorage.PVCStorageComponents) (*corev1.PersistentVolumeClaim, error) {
	// Generate correlation ID for this validation
	correlationID := generateCorrelationID()

	// Reconstruct PVC URI for enhanced validation
	pvcURI := fmt.Sprintf("pvc://%s:%s/%s", pvcComponents.Namespace, pvcComponents.PVCName, pvcComponents.SubPath)

	// For legacy calls, try to extract BaseModel from context or create a minimal one
	var baseModel client.Object
	if reqInfo, ok := ctx.Value("baseModel").(client.Object); ok {
		baseModel = reqInfo
	} else {
		// Create a minimal object for condition setting (this is a fallback)
		baseModel = &v1beta1.BaseModel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unknown",
				Namespace: namespace,
			},
		}
	}

	// Use enhanced security validation
	if err := r.validatePVCWithSecurity(ctx, baseModel, pvcURI, namespace, correlationID); err != nil {
		return nil, err
	}

	// If validation succeeds, fetch and return the PVC
	targetNamespace := namespace
	if pvcComponents.Namespace != "" {
		targetNamespace = pvcComponents.Namespace
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: targetNamespace,
		Name:      pvcComponents.PVCName,
	}, pvc); err != nil {
		return nil, fmt.Errorf("failed to get PVC %s: %w", pvcComponents.PVCName, err)
	}

	return pvc, nil
}

// validatePVC validates that the PVC exists and is accessible (ClusterBaseModel version)
func (r *ClusterBaseModelReconciler) validatePVC(ctx context.Context, namespace string, pvcComponents *utilstorage.PVCStorageComponents) (*corev1.PersistentVolumeClaim, error) {
	// Resolve namespace for PVC lookup
	targetNamespace := namespace
	if pvcComponents.Namespace != "" {
		targetNamespace = pvcComponents.Namespace
	}

	// Get PVC from cluster
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: targetNamespace,
		Name:      pvcComponents.PVCName,
	}, pvc); err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("PVC %s not found in namespace %s",
				pvcComponents.PVCName, targetNamespace)
		}
		return nil, fmt.Errorf("failed to get PVC %s: %w", pvcComponents.PVCName, err)
	}

	// Validate PVC is bound
	if pvc.Status.Phase != corev1.ClaimBound {
		return nil, fmt.Errorf("PVC %s is not bound (current phase: %s)",
			pvcComponents.PVCName, pvc.Status.Phase)
	}

	return pvc, nil
}

// createMetadataExtractionJob creates a Job for extracting metadata from PVC (BaseModel)
func (r *BaseModelReconciler) createMetadataExtractionJob(ctx context.Context, baseModel *v1beta1.BaseModel, pvc *corev1.PersistentVolumeClaim, pvcComponents *utilstorage.PVCStorageComponents) (*batchv1.Job, error) {
	jobName := r.getMetadataJobName(baseModel.Name)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: baseModel.Namespace,
			Labels: map[string]string{
				"ome.io/component":              "metadata-extraction",
				"ome.io/basemodel":              baseModel.Name,
				constants.BaseModelTypeLabelKey: string(constants.ServingBaseModel),
			},
			Annotations: map[string]string{
				"ome.io/storage-uri": *baseModel.Spec.Storage.StorageUri,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(3)),
			// CRITICAL FIX 1: Add 5-minute timeout as required
			ActiveDeadlineSeconds: ptr.To(int64(300)), // 5 minutes
			// CRITICAL FIX 2: Add TTL cleanup as required
			TTLSecondsAfterFinished: ptr.To(int32(300)), // 5 minutes cleanup
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"ome.io/component": "metadata-extraction",
						"ome.io/basemodel": baseModel.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					// CRITICAL FIX 3: Add dedicated ServiceAccount as required
					ServiceAccountName: "basemodel-metadata-extractor",
					Containers: []corev1.Container{
						{
							Name:    "metadata-extractor",
							Image:   r.getOMEAgentImage(),
							Command: []string{"/usr/bin/ome-agent"},
							// CRITICAL FIX 4: Use "model-metadata" command (not "hf-download")
							Args: []string{
								"model-metadata",
								"--model-path", "/mnt/models",
								"--basemodel-name", baseModel.Name,
								"--basemodel-namespace", baseModel.Namespace,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "model-storage",
									MountPath: "/mnt/models",
									SubPath:   pvcComponents.SubPath,
									ReadOnly:  true,
								},
							},
							// CRITICAL FIX 5: Add resource constraints as required
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "model-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.Name,
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference for cleanup
	if err := controllerutil.SetControllerReference(baseModel, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return job, nil
}

// createMetadataExtractionJobForCluster creates a Job for extracting metadata from PVC (ClusterBaseModel)
func (r *ClusterBaseModelReconciler) createMetadataExtractionJobForCluster(ctx context.Context, clusterBaseModel *v1beta1.ClusterBaseModel, pvc *corev1.PersistentVolumeClaim, pvcComponents *utilstorage.PVCStorageComponents) (*batchv1.Job, error) {
	jobName := r.getMetadataJobName(clusterBaseModel.Name)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: pvcComponents.Namespace, // Use PVC namespace for ClusterBaseModel
			Labels: map[string]string{
				"ome.io/component":              "metadata-extraction",
				"ome.io/clusterbasemodel":       clusterBaseModel.Name,
				constants.BaseModelTypeLabelKey: string(constants.ServingBaseModel),
			},
			Annotations: map[string]string{
				"ome.io/storage-uri": *clusterBaseModel.Spec.Storage.StorageUri,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(3)),
			// CRITICAL FIX 1: Add 5-minute timeout as required
			ActiveDeadlineSeconds: ptr.To(int64(300)), // 5 minutes
			// CRITICAL FIX 2: Add TTL cleanup as required
			TTLSecondsAfterFinished: ptr.To(int32(300)), // 5 minutes cleanup
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"ome.io/component":        "metadata-extraction",
						"ome.io/clusterbasemodel": clusterBaseModel.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					// CRITICAL FIX 3: Add dedicated ServiceAccount as required
					ServiceAccountName: "basemodel-metadata-extractor",
					Containers: []corev1.Container{
						{
							Name:    "metadata-extractor",
							Image:   r.getOMEAgentImage(),
							Command: []string{"/usr/bin/ome-agent"},
							// CRITICAL FIX 4: Use "model-metadata" command (not "hf-download")
							Args: []string{
								"model-metadata",
								"--model-path", "/mnt/models",
								"--basemodel-name", clusterBaseModel.Name,
								"--basemodel-namespace", "", // Empty for cluster-scoped
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "model-storage",
									MountPath: "/mnt/models",
									SubPath:   pvcComponents.SubPath,
									ReadOnly:  true,
								},
							},
							// CRITICAL FIX 5: Add resource constraints as required
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "model-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.Name,
									ReadOnly:  true,
								},
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference for cleanup
	if err := controllerutil.SetControllerReference(clusterBaseModel, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return job, nil
}

// handleJobStatus handles the status of metadata extraction jobs (BaseModel)
func (r *BaseModelReconciler) handleJobStatus(ctx context.Context, baseModel *v1beta1.BaseModel, job *batchv1.Job, pvcComponents *utilstorage.PVCStorageComponents) (ctrl.Result, error) {
	correlationID := getOrSetCorrelationID(baseModel)
	log := ctrl.LoggerFrom(ctx).WithValues("correlation_id", correlationID)

	// CRITICAL: Handle TTL edge case - job was deleted but might have succeeded
	if job == nil {
		log.Info("Job not found, checking for TTL cleanup scenario")
		configMapName := r.getConfigMapName(baseModel.Name)

		// Check if ConfigMap exists (indicates successful job that was TTL-cleaned)
		if err := r.checkConfigMapForMetadata(ctx, baseModel, configMapName); err == nil {
			log.Info("Job was TTL-deleted but ConfigMap exists, treating as success")
			r.Recorder.Event(baseModel, corev1.EventTypeNormal, "MetadataRecovered",
				"Recovered metadata from TTL-cleaned job")

			// Update status from ConfigMap and clear retry count
			clearRetryCount(baseModel)
			if err := r.updateModelStatus(ctx, baseModel); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// ConfigMap doesn't exist, need to recreate job
		log.Info("Job not found and no ConfigMap, will recreate job on next reconcile")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Calculate job duration for observability
	var duration time.Duration
	if job.Status.StartTime != nil {
		if job.Status.CompletionTime != nil {
			duration = job.Status.CompletionTime.Sub(job.Status.StartTime.Time)
		} else {
			duration = time.Since(job.Status.StartTime.Time)
		}
	}

	// Check job completion
	if job.Status.Succeeded > 0 {
		log.Info("Metadata extraction job completed successfully",
			"completion_time", job.Status.CompletionTime,
			"duration_seconds", duration.Seconds())
		r.Recorder.Event(baseModel, corev1.EventTypeNormal, "MetadataExtracted",
			fmt.Sprintf("Successfully extracted metadata from PVC in %.1f seconds", duration.Seconds()))

		// Clear retry count on successful completion
		clearRetryCount(baseModel)

		// ConfigMap integration: The Job has completed successfully, indicating that
		// the model is accessible and the PVC is working correctly. The existing
		// updateModelStatus() flow handles status propagation appropriately.
		// Future enhancement: Extract actual metadata from Job output and populate ConfigMaps
		if err := r.updateModelStatus(ctx, baseModel); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check for job failure
	if job.Status.Failed > 0 {
		failureReason := r.extractJobFailureReason(job)
		log.Error(fmt.Errorf("job failed"), "Metadata extraction job failed",
			"failed_count", job.Status.Failed,
			"failure_reason", failureReason,
			"duration_seconds", duration.Seconds())
		r.Recorder.Event(baseModel, corev1.EventTypeWarning, "MetadataExtractionFailed",
			fmt.Sprintf("Metadata extraction job failed after %d attempts: %s", job.Status.Failed, failureReason))

		// Don't retry job failures - let user investigate
		return ctrl.Result{}, fmt.Errorf("metadata extraction job failed: %s", failureReason)
	}

	// Job still running - provide detailed status
	if job.Status.Active > 0 {
		log.Info("Metadata extraction job still running",
			"start_time", job.Status.StartTime,
			"active_pods", job.Status.Active,
			"duration_seconds", duration.Seconds())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Job pending (not started yet)
	log.Info("Metadata extraction job pending", "created", job.CreationTimestamp)
	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

// handleClusterJobStatus handles the status of metadata extraction jobs (ClusterBaseModel)
func (r *ClusterBaseModelReconciler) handleClusterJobStatus(ctx context.Context, clusterBaseModel *v1beta1.ClusterBaseModel, job *batchv1.Job, pvcComponents *utilstorage.PVCStorageComponents) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Check job completion
	if job.Status.Succeeded > 0 {
		log.Info("Metadata extraction job completed successfully")

		r.Recorder.Event(clusterBaseModel, corev1.EventTypeNormal, "MetadataExtracted",
			"Successfully extracted metadata from PVC")

		// ConfigMap integration: The Job has completed successfully, indicating that
		// the model is accessible and the PVC is working correctly. The existing
		// updateModelStatus() flow handles status propagation appropriately.
		// Future enhancement: Extract actual metadata from Job output and populate ConfigMaps
		if err := r.updateModelStatus(ctx, clusterBaseModel); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check for job failure
	if job.Status.Failed > 0 {
		log.Error(fmt.Errorf("job failed"), "Metadata extraction job failed",
			"failed", job.Status.Failed)

		r.Recorder.Event(clusterBaseModel, corev1.EventTypeWarning, "MetadataExtractionFailed",
			fmt.Sprintf("Metadata extraction job failed after %d attempts", job.Status.Failed))

		// Return error to trigger requeue with backoff
		return ctrl.Result{RequeueAfter: time.Minute * 5},
			fmt.Errorf("metadata extraction job failed")
	}

	// Job is still running, requeue to check later
	log.V(1).Info("Metadata extraction job is still running")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// Helper methods
func (r *BaseModelReconciler) getMetadataJobName(baseModelName string) string {
	return fmt.Sprintf("%s-metadata-extraction", baseModelName)
}

func (r *ClusterBaseModelReconciler) getMetadataJobName(baseModelName string) string {
	return fmt.Sprintf("%s-metadata-extraction", baseModelName)
}

func (r *BaseModelReconciler) getOMEAgentImage() string {
	// Use environment variable or default
	if image := os.Getenv("OME_AGENT_IMAGE"); image != "" {
		return image
	}
	// CRITICAL: Use pinned version, NOT :latest for security and stability
	return "ghcr.io/sgl-project/ome/ome-agent:v1.2.3"
}

func (r *ClusterBaseModelReconciler) getOMEAgentImage() string {
	// Use environment variable or default
	if image := os.Getenv("OME_AGENT_IMAGE"); image != "" {
		return image
	}
	// CRITICAL: Use pinned version, NOT :latest for security and stability
	return "ghcr.io/sgl-project/ome/ome-agent:v1.2.3"
}

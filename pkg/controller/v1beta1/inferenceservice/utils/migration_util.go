package utils

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta2 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

// MigratePredictorToNewArchitecture migrates existing predictor resources to new engine/model architecture
func MigratePredictorToNewArchitecture(ctx context.Context, c client.Client, log logr.Logger, isvc *v1beta2.InferenceService) error {
	// Check if predictor is being used and migration hasn't happened yet
	if IsPredictorUsed(isvc) && isvc.Spec.Engine == nil && isvc.Spec.Model == nil {
		log.Info("Migrating predictor spec to new architecture",
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name)

		// Add deprecation warning annotation
		if isvc.ObjectMeta.Annotations == nil {
			isvc.ObjectMeta.Annotations = map[string]string{}
		}
		isvc.ObjectMeta.Annotations[constants.DeprecationWarning] = "The Predictor field is deprecated and will be removed in a future release. Please use Engine and Model fields instead."

		// Perform the migration
		if err := MigratePredictor(isvc); err != nil {
			return err
		}

		// Update the resource with migrated fields
		if err := c.Update(ctx, isvc); err != nil {
			return errors.Wrapf(err, "failed to update InferenceService after predictor migration")
		}

		// Delete the old predictor deployment if it exists
		deployment := &appsv1.Deployment{}
		deploymentName := isvc.Name // predictor deployment uses the inference service name
		err := c.Get(ctx, types.NamespacedName{
			Name:      deploymentName,
			Namespace: isvc.Namespace,
		}, deployment)

		if err == nil {
			// Deployment exists, delete it
			log.Info("Deleting old predictor deployment",
				"deployment", deploymentName,
				"namespace", isvc.Namespace)

			if err := c.Delete(ctx, deployment); err != nil {
				return errors.Wrapf(err, "failed to delete old predictor deployment")
			}
		} else if !apierrors.IsNotFound(err) {
			// Error other than not found
			return errors.Wrapf(err, "failed to check for old predictor deployment")
		}

		log.Info("Successfully migrated predictor to new architecture",
			"namespace", isvc.Namespace,
			"inferenceService", isvc.Name)
	}

	return nil
}

// IsPredictorUsed checks if the Predictor field has any meaningful configuration
func IsPredictorUsed(isvc *v1beta2.InferenceService) bool {
	predictor := &isvc.Spec.Predictor

	// Check if Model is defined in Predictor
	if predictor.Model != nil && predictor.Model.BaseModel != nil {
		return true
	}

	// Check if MinReplicas or MaxReplicas are set
	if predictor.MinReplicas != nil || predictor.MaxReplicas != 0 {
		return true
	}

	// Check other significant fields
	if predictor.ServiceAccountName != "" ||
		len(predictor.Containers) > 0 ||
		len(predictor.Volumes) > 0 ||
		len(predictor.NodeSelector) > 0 ||
		len(predictor.Tolerations) > 0 ||
		predictor.Affinity != nil ||
		predictor.Worker != nil {
		return true
	}

	return false
}

// MigratePredictor performs the actual migration from predictor to engine/model
func MigratePredictor(isvc *v1beta2.InferenceService) error {
	// Migrate Model
	if isvc.Spec.Predictor.Model != nil && isvc.Spec.Predictor.Model.BaseModel != nil {
		isvc.Spec.Model = &v1beta2.ModelRef{
			Name:             *isvc.Spec.Predictor.Model.BaseModel,
			FineTunedWeights: isvc.Spec.Predictor.Model.FineTunedWeights,
		}

		// Set default kind and API group
		kind := "ClusterBaseModel"
		apiGroup := "ome.io"
		isvc.Spec.Model.Kind = &kind
		isvc.Spec.Model.APIGroup = &apiGroup

		// Migrate Runtime reference
		if isvc.Spec.Predictor.Model.Runtime != nil {
			runtimeKind := "ClusterServingRuntime"
			runtimeAPIGroup := "ome.io"
			isvc.Spec.Runtime = &v1beta2.ServingRuntimeRef{
				Name:     *isvc.Spec.Predictor.Model.Runtime,
				Kind:     &runtimeKind,
				APIGroup: &runtimeAPIGroup,
			}
		}
	}

	// Migrate Engine
	isvc.Spec.Engine = &v1beta2.EngineSpec{
		PodSpec:                isvc.Spec.Predictor.PodSpec,
		ComponentExtensionSpec: isvc.Spec.Predictor.ComponentExtensionSpec,
	}

	// Process containers - look for ome-container or first container as Runner
	if len(isvc.Spec.Predictor.Containers) > 0 {
		// Look for ome-container first
		runnerFound := false
		var otherContainers []v1.Container

		for _, container := range isvc.Spec.Predictor.Containers {
			if !runnerFound && (container.Name == "ome-container" || strings.Contains(strings.ToLower(container.Name), "ome")) {
				// Migrate container from PredictorExtensionSpec to Runner
				runnerSpec := &v1beta2.RunnerSpec{
					Container: container,
				}

				// Merge PredictorExtensionSpec container settings if present
				if isvc.Spec.Predictor.Model != nil {
					// Merge environment variables
					if len(isvc.Spec.Predictor.Model.Env) > 0 {
						runnerSpec.Env = append(runnerSpec.Env, isvc.Spec.Predictor.Model.Env...)
					}

					// Add storage URI as environment variable if present
					if isvc.Spec.Predictor.Model.StorageUri != nil {
						runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
							Name:  "STORAGE_URI",
							Value: *isvc.Spec.Predictor.Model.StorageUri,
						})
					}

					// Add protocol version as environment variable if present
					if isvc.Spec.Predictor.Model.ProtocolVersion != nil {
						runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
							Name:  "PROTOCOL_VERSION",
							Value: string(*isvc.Spec.Predictor.Model.ProtocolVersion),
						})
					}
				}

				isvc.Spec.Engine.Runner = runnerSpec
				runnerFound = true
			} else {
				otherContainers = append(otherContainers, container)
			}
		}

		// If no ome container found, use first container as Runner
		if !runnerFound && len(isvc.Spec.Predictor.Containers) > 0 {
			runnerSpec := &v1beta2.RunnerSpec{
				Container: isvc.Spec.Predictor.Containers[0],
			}

			// Apply PredictorExtensionSpec settings
			if isvc.Spec.Predictor.Model != nil {
				if len(isvc.Spec.Predictor.Model.Env) > 0 {
					runnerSpec.Env = append(runnerSpec.Env, isvc.Spec.Predictor.Model.Env...)
				}
				if isvc.Spec.Predictor.Model.StorageUri != nil {
					runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
						Name:  "STORAGE_URI",
						Value: *isvc.Spec.Predictor.Model.StorageUri,
					})
				}
				if isvc.Spec.Predictor.Model.ProtocolVersion != nil {
					runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
						Name:  "PROTOCOL_VERSION",
						Value: string(*isvc.Spec.Predictor.Model.ProtocolVersion),
					})
				}
			}

			isvc.Spec.Engine.Runner = runnerSpec

			// Keep remaining containers
			if len(isvc.Spec.Predictor.Containers) > 1 {
				isvc.Spec.Engine.Containers = isvc.Spec.Predictor.Containers[1:]
			}
		} else {
			isvc.Spec.Engine.Containers = otherContainers
		}
	} else if isvc.Spec.Predictor.Model != nil {
		// No containers in PodSpec, but we have Model spec with container configuration
		runnerSpec := &v1beta2.RunnerSpec{
			Container: isvc.Spec.Predictor.Model.Container,
		}

		// Add storage URI as environment variable if present
		if isvc.Spec.Predictor.Model.StorageUri != nil {
			if runnerSpec.Env == nil {
				runnerSpec.Env = []v1.EnvVar{}
			}
			runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
				Name:  "STORAGE_URI",
				Value: *isvc.Spec.Predictor.Model.StorageUri,
			})
		}

		// Add protocol version as environment variable if present
		if isvc.Spec.Predictor.Model.ProtocolVersion != nil {
			if runnerSpec.Env == nil {
				runnerSpec.Env = []v1.EnvVar{}
			}
			runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
				Name:  "PROTOCOL_VERSION",
				Value: string(*isvc.Spec.Predictor.Model.ProtocolVersion),
			})
		}

		// No containers in Model spec, set runnerSpec as nil
		if runnerSpec.Container.Name == "" {
			runnerSpec = nil
		}

		isvc.Spec.Engine.Runner = runnerSpec
	}

	// Migrate Worker spec if present
	if isvc.Spec.Predictor.Worker != nil {
		isvc.Spec.Engine.Worker = isvc.Spec.Predictor.Worker
	}

	// Clear the predictor spec after migration
	isvc.Spec.Predictor = v1beta2.PredictorSpec{}

	return nil
}

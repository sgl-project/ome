package utils

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta2 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"
)

func translatePredictorModelRef(predictor *v1beta2.PredictorSpec) *v1beta2.ModelRef {
	if predictor == nil || predictor.Model == nil || predictor.Model.BaseModel == nil {
		return nil
	}

	return &v1beta2.ModelRef{
		Name:             *predictor.Model.BaseModel,
		FineTunedWeights: predictor.Model.FineTunedWeights,
	}
}

func translatePredictorRuntimeRef(predictor *v1beta2.PredictorSpec) *v1beta2.ServingRuntimeRef {
	if predictor == nil || predictor.Model == nil || predictor.Model.Runtime == nil {
		return nil
	}

	runtimeKind := "ClusterServingRuntime"
	runtimeAPIGroup := "ome.io"
	return &v1beta2.ServingRuntimeRef{
		Name:     *predictor.Model.Runtime,
		Kind:     &runtimeKind,
		APIGroup: &runtimeAPIGroup,
	}
}

func translatePredictorEngineSpec(predictor *v1beta2.PredictorSpec) *v1beta2.EngineSpec {
	if predictor == nil {
		return nil
	}

	engine := &v1beta2.EngineSpec{
		PodSpec:                predictor.PodSpec,
		ComponentExtensionSpec: predictor.ComponentExtensionSpec,
	}

	// Process containers - look for ome-container or first container as Runner.
	if len(predictor.Containers) > 0 {
		runnerFound := false
		var otherContainers []v1.Container

		for _, container := range predictor.Containers {
			if !runnerFound && (container.Name == "ome-container" || strings.Contains(strings.ToLower(container.Name), "ome")) {
				runnerSpec := &v1beta2.RunnerSpec{
					Container: container,
				}

				if predictor.Model != nil {
					if len(predictor.Model.Env) > 0 {
						runnerSpec.Env = append(runnerSpec.Env, predictor.Model.Env...)
					}

					if predictor.Model.StorageUri != nil {
						runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
							Name:  "STORAGE_URI",
							Value: *predictor.Model.StorageUri,
						})
					}

					if predictor.Model.ProtocolVersion != nil {
						runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
							Name:  "PROTOCOL_VERSION",
							Value: string(*predictor.Model.ProtocolVersion),
						})
					}
				}

				engine.Runner = runnerSpec
				runnerFound = true
			} else {
				otherContainers = append(otherContainers, container)
			}
		}

		if !runnerFound {
			runnerSpec := &v1beta2.RunnerSpec{
				Container: predictor.Containers[0],
			}

			if predictor.Model != nil {
				if len(predictor.Model.Env) > 0 {
					runnerSpec.Env = append(runnerSpec.Env, predictor.Model.Env...)
				}
				if predictor.Model.StorageUri != nil {
					runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
						Name:  "STORAGE_URI",
						Value: *predictor.Model.StorageUri,
					})
				}
				if predictor.Model.ProtocolVersion != nil {
					runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
						Name:  "PROTOCOL_VERSION",
						Value: string(*predictor.Model.ProtocolVersion),
					})
				}
			}

			engine.Runner = runnerSpec
			if len(predictor.Containers) > 1 {
				engine.Containers = predictor.Containers[1:]
			}
		} else {
			engine.Containers = otherContainers
		}
	} else if predictor.Model != nil {
		runnerSpec := &v1beta2.RunnerSpec{
			Container: predictor.Model.Container,
		}

		if predictor.Model.StorageUri != nil {
			runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
				Name:  "STORAGE_URI",
				Value: *predictor.Model.StorageUri,
			})
		}

		if predictor.Model.ProtocolVersion != nil {
			runnerSpec.Env = append(runnerSpec.Env, v1.EnvVar{
				Name:  "PROTOCOL_VERSION",
				Value: string(*predictor.Model.ProtocolVersion),
			})
		}

		if runnerSpec.Container.Name == "" {
			runnerSpec = nil
		}

		engine.Runner = runnerSpec
	}

	if predictor.Worker != nil {
		engine.Worker = predictor.Worker
	}

	return engine
}

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
		if err := MigratePredictor(ctx, c, isvc); err != nil {
			return err
		}

		// Update the resource with migrated fields
		if err := c.Update(ctx, isvc); err != nil {
			return errors.Wrapf(err, "failed to update InferenceService after predictor migration")
		}

		// Note: Old predictor deployment cleanup is handled by cleanupOldPredictorDeployment
		// in the controller after new component deployments are ready

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
func MigratePredictor(ctx context.Context, c client.Client, isvc *v1beta2.InferenceService) error {
	predictor := &isvc.Spec.Predictor

	isvc.Spec.Model = translatePredictorModelRef(predictor)
	isvc.Spec.Runtime = translatePredictorRuntimeRef(predictor)
	isvc.Spec.Engine = translatePredictorEngineSpec(predictor)

	if isvc.Spec.Model != nil {
		// Determine the model kind dynamically
		modelName := isvc.Spec.Model.Name
		kind, err := DetermineModelKind(ctx, c, modelName, isvc.Namespace)
		if err != nil {
			return err
		}

		// Set kind and API group
		apiGroup := "ome.io"
		isvc.Spec.Model.Kind = &kind
		isvc.Spec.Model.APIGroup = &apiGroup
	}

	return nil
}

func DetermineModelKind(ctx context.Context, c client.Client, modelName string, namespace string) (string, error) {
	// First, try to get ClusterBaseModel (cluster-scoped)
	clusterBaseModelGetErr := c.Get(ctx, client.ObjectKey{Name: modelName}, &v1beta2.ClusterBaseModel{})
	if clusterBaseModelGetErr == nil {
		return "ClusterBaseModel", nil
	}

	// Try BaseModel (namespace-scoped) even if ClusterBaseModel lookup had an error
	baseModelGetErr := c.Get(ctx, client.ObjectKey{Name: modelName, Namespace: namespace}, &v1beta2.BaseModel{})
	if baseModelGetErr == nil {
		return "BaseModel", nil
	}

	// Both lookups failed - determine the appropriate error to return
	if apierrors.IsNotFound(clusterBaseModelGetErr) && apierrors.IsNotFound(baseModelGetErr) {
		return "", errors.Errorf("neither ClusterBaseModel nor BaseModel found with name %s", modelName)
	} else if !apierrors.IsNotFound(clusterBaseModelGetErr) && apierrors.IsNotFound(baseModelGetErr) {
		return "", errors.Errorf("failed to get ClusterBaseModel %s: %v; BaseModel %s not found in namespace %s",
			modelName, clusterBaseModelGetErr, modelName, namespace)
	} else if apierrors.IsNotFound(clusterBaseModelGetErr) && !apierrors.IsNotFound(baseModelGetErr) {
		return "", errors.Errorf("ClusterBaseModel %s not found; failed to get BaseModel %s in namespace %s: %v",
			modelName, modelName, namespace, baseModelGetErr)
	} else {
		return "", errors.Errorf("failed to get ClusterBaseModel %s: %v; failed to get BaseModel %s in namespace %s: %v",
			modelName, clusterBaseModelGetErr, modelName, namespace, baseModelGetErr)
	}
}

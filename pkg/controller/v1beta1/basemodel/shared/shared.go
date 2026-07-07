// Package shared holds the Backend interface + cross-cutting helpers
// (RetryUpdate, ModelSpecAndStatus, UpdateSpecWithConfig). It lives apart
// from basemodel/ so the backend sub-packages can implement/consume the
// interface without importing basemodel (which would be an import cycle).
package shared

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/modelagent"
)

// Backend is one of OME's BaseModel distribution backends. The
// controller dispatches Reconcile / HandleDeletion to the first backend
// whose Matches(spec) returns true; per-node is the default fallback and
// must be registered last.
type Backend interface {
	Name() string
	Matches(spec *v1beta1.BaseModelSpec) bool
	Reconcile(ctx context.Context, args BackendArgs) (ctrl.Result, error)
	HandleDeletion(ctx context.Context, args BackendArgs) (ctrl.Result, error)
}

// BackendArgs bundles per-reconcile state. Construction-time deps come in
// through backend constructors instead, so this stays free of
// backend-specific imports.
type BackendArgs struct {
	Client          client.Client
	Log             logr.Logger
	Obj             client.Object
	Spec            *v1beta1.BaseModelSpec
	Status          *v1beta1.ModelStatusSpec
	Finalizer       string
	IsClusterScoped bool
	Kind            string
}

// ModelSpecAndStatus returns pointers to the embedded spec and status of
// either a BaseModel or ClusterBaseModel.
func ModelSpecAndStatus(obj client.Object) (*v1beta1.BaseModelSpec, *v1beta1.ModelStatusSpec, error) {
	switch model := obj.(type) {
	case *v1beta1.BaseModel:
		return &model.Spec, &model.Status, nil
	case *v1beta1.ClusterBaseModel:
		return &model.Spec, &model.Status, nil
	default:
		return nil, nil, fmt.Errorf("unsupported model type: %T", obj)
	}
}

// RetryUpdate retries updateFunc on conflict up to 3 times, with
// 100ms/200ms/400ms backoff. Each attempt receives a freshly-fetched
// copy of obj.
func RetryUpdate(ctx context.Context, kubeClient client.Client, log logr.Logger, obj client.Object, updateType string, updateFunc func(context.Context, client.Client, client.Object) error) error {
	const maxRetries = 3

	for i := 0; i < maxRetries; i++ {
		// Get the latest version
		latest := obj.DeepCopyObject().(client.Object)
		if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(obj), latest); err != nil {
			return fmt.Errorf("failed to get latest object version: %w", err)
		}

		// Execute the update function
		if err := updateFunc(ctx, kubeClient, latest); err != nil {
			if errors.IsConflict(err) && i < maxRetries-1 {
				// Exponential backoff: wait 100ms, 200ms, 400ms
				backoff := time.Millisecond * time.Duration(100<<uint(i))
				log.V(1).Info("Resource conflict during update, retrying with backoff",
					"updateType", updateType, "retry", i+1, "backoff", backoff, "object", client.ObjectKeyFromObject(obj))
				time.Sleep(backoff)
				continue
			}
			if errors.IsConflict(err) {
				return fmt.Errorf("failed to update %s after %d retries due to conflicts", updateType, maxRetries)
			}
			return fmt.Errorf("failed to update %s: %w", updateType, err)
		}
		return nil
	}
	return fmt.Errorf("failed to update %s after %d retries", updateType, maxRetries)
}

// UpdateSpecWithConfig applies fields from config onto spec when spec is
// empty for that field. Fill-only — never overwrites operator-set values.
// Returns true iff anything changed.
func UpdateSpecWithConfig(spec *v1beta1.BaseModelSpec, config *modelagent.ModelConfig) bool {
	if spec == nil || config == nil {
		return false
	}

	updated := false

	// Update ModelType if not set
	if spec.ModelType == nil && config.ModelType != "" {
		modelType := config.ModelType
		spec.ModelType = &modelType
		updated = true
	}

	// Update ModelArchitecture if not set
	if spec.ModelArchitecture == nil && config.ModelArchitecture != "" {
		architecture := config.ModelArchitecture
		spec.ModelArchitecture = &architecture
		updated = true
	}

	// Update ModelParameterSize if not set
	if spec.ModelParameterSize == nil && config.ModelParameterSize != "" {
		paramSize := config.ModelParameterSize
		spec.ModelParameterSize = &paramSize
		updated = true
	}

	// Update capabilities if not set
	if len(spec.ModelCapabilities) == 0 && len(config.ModelCapabilities) > 0 {
		spec.ModelCapabilities = make([]string, len(config.ModelCapabilities))
		copy(spec.ModelCapabilities, config.ModelCapabilities)
		updated = true
	}

	// Update framework if not set
	if spec.ModelFramework == nil && config.ModelFramework != nil {
		name := config.ModelFramework["name"]
		version := config.ModelFramework["version"]
		if name != "" {
			framework := &v1beta1.ModelFrameworkSpec{Name: name}
			if version != "" {
				framework.Version = &version
			}
			spec.ModelFramework = framework
			updated = true
		}
	}

	// Update model format if not set
	if config.ModelFormat != nil {
		name := config.ModelFormat["name"]
		version := config.ModelFormat["version"]

		if name != "" && spec.ModelFormat.Name == "" {
			spec.ModelFormat.Name = name
			updated = true
		}

		if version != "" && spec.ModelFormat.Version == nil {
			versionValue := version
			spec.ModelFormat.Version = &versionValue
			updated = true
		}
	}

	// Update MaxTokens if not set and valid
	if spec.MaxTokens == nil && config.MaxTokens > 0 {
		spec.MaxTokens = &config.MaxTokens
		updated = true
	}

	return updated
}

# Runtime Selector Package

The `runtimeselector` package provides a comprehensive, modular solution for runtime selection and validation in the OME operator. It determines the best serving runtime for machine learning models based on model characteristics, runtime capabilities, and scoring algorithms.

## Overview

The package provides:

- **Automatic runtime selection** based on model requirements
- **Runtime validation** for user-specified runtimes
- **Detailed compatibility analysis** with clear error reporting
- **Efficient caching** using controller-runtime's cache mechanism
- **Flexible scoring system** with runtime-defined weights

## Architecture

### Component Structure

```
runtimeselector/
├── types.go        # Core interfaces and data structures
├── selector.go     # Main selector implementation
├── fetcher.go      # Runtime resource fetching with caching
├── matcher.go      # Compatibility evaluation logic
├── scorer.go       # Scoring and ranking algorithms
└── errors.go       # Custom error types
```

### Key Interfaces

#### Selector
The main interface for runtime selection operations:

```go
type Selector interface {
    // Auto-select best runtime for a model
    SelectRuntime(ctx context.Context, model *v1beta1.BaseModelSpec, namespace string) (*RuntimeSelection, error)
    
    // Get all compatible runtimes sorted by priority
    GetCompatibleRuntimes(ctx context.Context, model *v1beta1.BaseModelSpec, namespace string) ([]RuntimeMatch, error)
    
    // Validate a specific runtime choice
    ValidateRuntime(ctx context.Context, runtimeName string, model *v1beta1.BaseModelSpec, namespace string) error
    
    // Get runtime spec by name
    GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error)
}
```

#### RuntimeFetcher
Abstracts runtime resource fetching:

```go
type RuntimeFetcher interface {
    // Fetch all runtimes in a namespace
    FetchRuntimes(ctx context.Context, namespace string) (*RuntimeCollection, error)
    
    // Get specific runtime by name
    GetRuntime(ctx context.Context, name string, namespace string) (*v1beta1.ServingRuntimeSpec, bool, error)
}
```

#### RuntimeMatcher
Handles compatibility checking:

```go
type RuntimeMatcher interface {
    // Check basic compatibility
    IsCompatible(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, runtimeName string) (bool, error)
    
    // Get detailed compatibility report
    GetCompatibilityDetails(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec, runtimeName string) (*CompatibilityReport, error)
}
```

#### RuntimeScorer
Calculates runtime scores:

```go
type RuntimeScorer interface {
    // Calculate score for runtime-model pair
    CalculateScore(runtime *v1beta1.ServingRuntimeSpec, model *v1beta1.BaseModelSpec) (int64, error)
    
    // Compare two runtime matches
    CompareRuntimes(a, b RuntimeMatch, model *v1beta1.BaseModelSpec) int
}
```

## Algorithms

### Runtime Selection Algorithm

1. **Fetch Runtimes**
   - Retrieve all ServingRuntimes in the namespace
   - Retrieve all ClusterServingRuntimes
   - Sort by creation timestamp (newest first) and name

2. **Filter Compatible Runtimes**
   - Skip disabled runtimes
   - Check model format compatibility
   - Verify model size is within supported range
   - Ensure auto-select is enabled
   - Calculate compatibility score

3. **Score and Sort**
   - Calculate weighted scores based on:
     - Model format match (weight × priority)
     - Model framework match (weight × priority)
   - Sort by score (highest first)
   - For equal scores, prefer:
     - Namespace-scoped over cluster-scoped
     - Closer model size range match

4. **Return Best Match**
   - Select the highest-scoring runtime
   - Include detailed match information

### Compatibility Checking

The matcher evaluates compatibility across multiple dimensions:

1. **Model Format and Framework**
   - Name must match exactly
   - Version comparison using semantic versioning
   - Special handling for unofficial versions (forces equality)

2. **Model Architecture**
   - Must match if both specify architecture

3. **Quantization**
   - Must match if both specify quantization

4. **Model Size**
   - Must be within runtime's min/max range

### Scoring Formula

```
Score = Σ(weight × priority) for each matching attribute
```

Where:
- **weight**: Importance of the attribute (defined in the runtime's ModelFormat/ModelFramework)
- **priority**: Runtime-specific multiplier for the supported model format entry

Note: A runtime can support multiple model formats, each with its own weights and priorities. The scorer evaluates all supported formats and uses the highest scoring match.

## Usage Examples

### Basic Integration

```go
// In InferenceService controller
type InferenceServiceReconciler struct {
    client.Client
    RuntimeSelector runtimeselector.Selector
    // ... other fields
}

func (r *InferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
    // Initialize runtime selector
    r.RuntimeSelector = runtimeselector.New(mgr.GetClient())
    
    // Add watches to populate cache
    ctrlBuilder.
        Watches(&v1beta1.ServingRuntime{},
            handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
                return nil // Just populate cache
            })).
        Watches(&v1beta1.ClusterServingRuntime{},
            handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
                return nil // Just populate cache
            }))
    
    return ctrlBuilder.Complete(r)
}
```

### Runtime Selection in Reconciler

```go
func (r *InferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // ... fetch InferenceService and BaseModel ...
    
    var rt *v1beta1.ServingRuntimeSpec
    var rtName string
    
    if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
        // Validate specified runtime
        rtName = isvc.Spec.Runtime.Name
        if err := r.RuntimeSelector.ValidateRuntime(ctx, rtName, baseModel, isvc.Namespace); err != nil {
            return reconcile.Result{}, fmt.Errorf("runtime validation failed: %w", err)
        }
        
        // Get runtime spec
        rtSpec, _, err := r.RuntimeSelector.GetRuntime(ctx, rtName, isvc.Namespace)
        if err != nil {
            return reconcile.Result{}, err
        }
        rt = rtSpec
    } else {
        // Auto-select runtime
        selection, err := r.RuntimeSelector.SelectRuntime(ctx, baseModel, isvc.Namespace)
        if err != nil {
            return reconcile.Result{}, fmt.Errorf("runtime selection failed: %w", err)
        }
        rt = selection.Spec
        rtName = selection.Name
    }
    
    // ... continue with runtime spec ...
}
```

### Webhook Validation

```go
type InferenceServiceValidator struct {
    Client          client.Client
    RuntimeSelector runtimeselector.Selector
}

func (v *InferenceServiceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
    isvc := obj.(*v1beta1.InferenceService)
    
    // Check if runtime can be selected/validated
    if isvc.Spec.Runtime != nil && isvc.Spec.Runtime.Name != "" {
        if err := v.RuntimeSelector.ValidateRuntime(ctx, isvc.Spec.Runtime.Name, baseModel, isvc.Namespace); err != nil {
            return nil, fmt.Errorf("invalid runtime selection: %w", err)
        }
    } else {
        // Verify auto-selection is possible
        if _, err := v.RuntimeSelector.SelectRuntime(ctx, baseModel, isvc.Namespace); err != nil {
            return nil, fmt.Errorf("no compatible runtime available: %w", err)
        }
    }
    
    return nil, nil
}
```

## Error Handling

The package provides rich error types with detailed information:

### NoRuntimeFoundError
```go
if runtimeselector.IsNoRuntimeFoundError(err) {
    e := err.(*runtimeselector.NoRuntimeFoundError)
    log.Info("No runtime found",
        "model", e.ModelName,
        "format", e.ModelFormat,
        "namespace", e.Namespace,
        "totalRuntimes", e.TotalRuntimes,
        "excludedCount", len(e.ExcludedRuntimes))
    
    // Log why each runtime was excluded
    for name, reason := range e.ExcludedRuntimes {
        log.Info("Runtime excluded", "runtime", name, "reason", reason)
    }
}
```

### RuntimeCompatibilityError
```go
if runtimeselector.IsRuntimeCompatibilityError(err) {
    e := err.(*runtimeselector.RuntimeCompatibilityError)
    log.Error(err, "Runtime incompatible",
        "runtime", e.RuntimeName,
        "model", e.ModelName,
        "format", e.ModelFormat,
        "reason", e.Reason)
}
```

## Testing

The package includes comprehensive tests covering:

- Runtime selection with multiple compatible runtimes
- Scoring algorithm verification
- Version comparison (semantic and unofficial)
- Model size range validation
- Namespace vs cluster runtime prioritization
- Error scenarios and edge cases

Run tests:
```bash
cd pkg/runtimeselector
go test -v
```

## Performance Considerations

1. **Caching**: Uses controller-runtime's cache to avoid repeated API calls
2. **Watches**: Runtime resources are watched to keep cache fresh
3. **Efficient Sorting**: Runtimes are pre-sorted by creation time and name
4. **Early Termination**: Compatibility checks fail fast on first mismatch

## Configurable Scoring

The scoring system supports runtime-defined weights for fine-tuning selection priorities:

```yaml
apiVersion: ome.io/v1beta1
kind: ServingRuntime
metadata:
  name: high-priority-pytorch-runtime
spec:
  supportedModelFormats:
  - modelFormat:
      name: pytorch
      weight: 20  # Higher weight = higher priority
    modelFramework:
      name: transformers
      weight: 15
    priority: 2     # Multiplier for all weights
    autoSelect: true
```

The final score is calculated as: `(modelFormat.weight × priority) + (modelFramework.weight × priority)`

## Future Enhancements

1. **Accelerator-Aware Selection**: Integration with OEP-0003 for GPU-aware runtime selection
   - AcceleratorClass abstraction for heterogeneous GPU environments
   - Capability-based matching for different GPU types (H100, A100, etc.)
   - Integration with Kueue ResourceFlavors

2. **Runtime Affinity/Anti-affinity**: Support for preferred/excluded runtime lists at the InferenceService level

3. **Enhanced Version Matching**: Support for more complex version constraints and ranges

4. **Runtime Capabilities API**: Expose runtime capabilities for advanced selection scenarios
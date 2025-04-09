# V1beta1ServingRuntimeRef

## Properties

| Name          | Type    | Description                                                                                                                                                                                                                         | Notes                      |
|---------------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **api_group** | **str** | APIGroup of the resource being referenced Defaults to &#x60;ome.io&#x60; Specifies the Kubernetes API group of the referenced runtime.                                                                                              | [optional]                 |
| **kind**      | **str** | Kind of the runtime being referenced Defaults to ClusterServingRuntime Specifies the Kubernetes resource kind of the referenced runtime. ClusterServingRuntime is a cluster-wide runtime, while ServingRuntime is namespace-scoped. | [optional]                 |
| **name**      | **str** | Name of the runtime being referenced Identifies the specific runtime environment to be used for model execution.                                                                                                                    | [optional] [default to ''] |

## Example

```python
from ome.models.v1beta1_serving_runtime_ref import V1beta1ServingRuntimeRef

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ServingRuntimeRef from a JSON string
v1beta1_serving_runtime_ref_instance = V1beta1ServingRuntimeRef.from_json(json)
# print the JSON string representation of the object
print(V1beta1ServingRuntimeRef.to_json())

# convert the object into a dict
v1beta1_serving_runtime_ref_dict = v1beta1_serving_runtime_ref_instance.to_dict()
# create an instance of V1beta1ServingRuntimeRef from a dict
v1beta1_serving_runtime_ref_from_dict = V1beta1ServingRuntimeRef.from_dict(v1beta1_serving_runtime_ref_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

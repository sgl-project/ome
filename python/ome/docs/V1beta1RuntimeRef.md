# V1beta1RuntimeRef

RuntimeRef represents the reference to the existing training runtime.

## Properties

| Name          | Type    | Description                                                                                                                                              | Notes                      |
|---------------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **api_group** | **str** | APIGroup of the runtime being referenced. Defaults to &#x60;ome.io&#x60;.                                                                                | [optional]                 |
| **kind**      | **str** | Kind of the runtime being referenced. Defaults to ClusterTrainingRuntime.                                                                                | [optional]                 |
| **name**      | **str** | Name of the runtime being referenced. When namespaced-scoped TrainingRuntime is used, the TrainJob must have the same namespace as the deployed runtime. | [optional] [default to ''] |

## Example

```python
from ome.models.v1beta1_runtime_ref import V1beta1RuntimeRef

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1RuntimeRef from a JSON string
v1beta1_runtime_ref_instance = V1beta1RuntimeRef.from_json(json)
# print the JSON string representation of the object
print(V1beta1RuntimeRef.to_json())

# convert the object into a dict
v1beta1_runtime_ref_dict = v1beta1_runtime_ref_instance.to_dict()
# create an instance of V1beta1RuntimeRef from a dict
v1beta1_runtime_ref_from_dict = V1beta1RuntimeRef.from_dict(v1beta1_runtime_ref_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

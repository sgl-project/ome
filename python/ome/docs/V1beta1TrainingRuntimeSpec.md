# V1beta1TrainingRuntimeSpec

TrainingRuntimeSpec defines the desired state of TrainingRuntime

## Properties

| Name                 | Type                                                  | Description                                                                                                     | Notes      |
|----------------------|-------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------|------------|
| **annotations**      | **Dict[str, str]**                                    | Annotations that will be added to the runtime spec. More info: http://kubernetes.io/docs/user-guide/annotations | [optional] |
| **compartment_id**   | **str**                                               | The compartment ID to use for the training runtime                                                              | [optional] |
| **labels**           | **Dict[str, str]**                                    | Labels that will be added to the runtime spec. More info: http://kubernetes.io/docs/user-guide/labels           | [optional] |
| **ml_policy**        | [**V1beta1MLPolicy**](V1beta1MLPolicy.md)             |                                                                                                                 | [optional] |
| **pod_group_policy** | [**V1beta1PodGroupPolicy**](V1beta1PodGroupPolicy.md) |                                                                                                                 | [optional] |
| **template**         | [**V1beta1JobSetTemplateSpec**](V1beta1JobSetTemplateSpec.md) |                                                                                                                 |

## Example

```python
from ome.models.v1beta1_training_runtime_spec import V1beta1TrainingRuntimeSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1TrainingRuntimeSpec from a JSON string
v1beta1_training_runtime_spec_instance = V1beta1TrainingRuntimeSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1TrainingRuntimeSpec.to_json())

# convert the object into a dict
v1beta1_training_runtime_spec_dict = v1beta1_training_runtime_spec_instance.to_dict()
# create an instance of V1beta1TrainingRuntimeSpec from a dict
v1beta1_training_runtime_spec_from_dict = V1beta1TrainingRuntimeSpec.from_dict(v1beta1_training_runtime_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

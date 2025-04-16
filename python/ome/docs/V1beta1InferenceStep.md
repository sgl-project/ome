# V1beta1InferenceStep

InferenceStep defines the inference target of the current step with condition, weights and data.

## Properties

| Name             | Type    | Description                                                                                                                           | Notes      |
|------------------|---------|---------------------------------------------------------------------------------------------------------------------------------------|------------|
| **condition**    | **str** | routing based on the condition                                                                                                        | [optional] |
| **data**         | **str** | request data sent to the next route with input/output from the previous step $request $response.predictions                           | [optional] |
| **dependency**   | **str** | to decide whether a step is a hard or a soft dependency in the Inference Graph                                                        | [optional] |
| **name**         | **str** | Unique name for the step within this node                                                                                             | [optional] |
| **node_name**    | **str** | The node name for routing as next step                                                                                                | [optional] |
| **service_name** | **str** | named reference for InferenceService                                                                                                  | [optional] |
| **service_url**  | **str** | InferenceService URL, mutually exclusive with ServiceName                                                                             | [optional] |
| **weight**       | **int** | the weight for split of the traffic, only used for Split Router when weight is specified all the routing targets should be sum to 100 | [optional] |

## Example

```python
from ome.models.v1beta1_inference_step import V1beta1InferenceStep

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1InferenceStep from a JSON string
v1beta1_inference_step_instance = V1beta1InferenceStep.from_json(json)
# print the JSON string representation of the object
print(V1beta1InferenceStep.to_json())

# convert the object into a dict
v1beta1_inference_step_dict = v1beta1_inference_step_instance.to_dict()
# create an instance of V1beta1InferenceStep from a dict
v1beta1_inference_step_from_dict = V1beta1InferenceStep.from_dict(v1beta1_inference_step_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

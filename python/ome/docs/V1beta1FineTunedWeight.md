# V1beta1FineTunedWeight

FineTunedWeight is the Schema for the finetunedweights API

## Properties

| Name            | Type                                                                                                        | Description                                                                                                                                                                                                                                                                                        | Notes      |
|-----------------|-------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **api_version** | **str**                                                                                                     | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources  | [optional] |
| **kind**        | **str**                                                                                                     | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds | [optional] |
| **metadata**    | [**V1ObjectMeta**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ObjectMeta.md) |                                                                                                                                                                                                                                                                                                    | [optional] |
| **spec**        | [**V1beta1FineTunedWeightSpec**](V1beta1FineTunedWeightSpec.md)                                             |                                                                                                                                                                                                                                                                                                    | [optional] |
| **status**      | [**V1beta1ModelStatusSpec**](V1beta1ModelStatusSpec.md)                                                     |                                                                                                                                                                                                                                                                                                    | [optional] |

## Example

```python
from ome.models.v1beta1_fine_tuned_weight import V1beta1FineTunedWeight

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1FineTunedWeight from a JSON string
v1beta1_fine_tuned_weight_instance = V1beta1FineTunedWeight.from_json(json)
# print the JSON string representation of the object
print(V1beta1FineTunedWeight.to_json())

# convert the object into a dict
v1beta1_fine_tuned_weight_dict = v1beta1_fine_tuned_weight_instance.to_dict()
# create an instance of V1beta1FineTunedWeight from a dict
v1beta1_fine_tuned_weight_from_dict = V1beta1FineTunedWeight.from_dict(v1beta1_fine_tuned_weight_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

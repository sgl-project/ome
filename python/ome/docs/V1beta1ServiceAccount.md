# V1beta1ServiceAccount

ServiceAccount represents a service account within a project

## Properties

| Name            | Type                                                                                                        | Description                                                                                                                                                                                                                                                                                        | Notes      |
|-----------------|-------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **api_version** | **str**                                                                                                     | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources  | [optional] |
| **kind**        | **str**                                                                                                     | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds | [optional] |
| **metadata**    | [**V1ObjectMeta**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ObjectMeta.md) |                                                                                                                                                                                                                                                                                                    | [optional] |
| **spec**        | [**V1beta1ServiceAccountSpec**](V1beta1ServiceAccountSpec.md)                                               |                                                                                                                                                                                                                                                                                                    | [optional] |
| **status**      | [**V1beta1ServiceAccountStatus**](V1beta1ServiceAccountStatus.md)                                           |                                                                                                                                                                                                                                                                                                    | [optional] |

## Example

```python
from ome.models.v1beta1_service_account import V1beta1ServiceAccount

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ServiceAccount from a JSON string
v1beta1_service_account_instance = V1beta1ServiceAccount.from_json(json)
# print the JSON string representation of the object
print(V1beta1ServiceAccount.to_json())

# convert the object into a dict
v1beta1_service_account_dict = v1beta1_service_account_instance.to_dict()
# create an instance of V1beta1ServiceAccount from a dict
v1beta1_service_account_from_dict = V1beta1ServiceAccount.from_dict(v1beta1_service_account_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1DedicatedAIClusterList

DedicatedAIClusterList contains a list of DedicatedAICluster

## Properties

| Name            | Type    | Description                                                                                                                                                                                                                                                                                       | Notes      |
|-----------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **api_version** | **str** | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources | [optional] |
| **items**       | [**List[V1beta1DedicatedAICluster]**](V1beta1DedicatedAICluster.md)                                     |                                                                                                                                                                                                                                                                                                    |
| **kind**        | **str**                                                                                                 | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds | [optional] |
| **metadata**    | [**V1ListMeta**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ListMeta.md) |                                                                                                                                                                                                                                                                                                    | [optional] |

## Example

```python
from ome.models.v1beta1_dedicated_ai_cluster_list import V1beta1DedicatedAIClusterList

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1DedicatedAIClusterList from a JSON string
v1beta1_dedicated_ai_cluster_list_instance = V1beta1DedicatedAIClusterList.from_json(json)
# print the JSON string representation of the object
print(V1beta1DedicatedAIClusterList.to_json())

# convert the object into a dict
v1beta1_dedicated_ai_cluster_list_dict = v1beta1_dedicated_ai_cluster_list_instance.to_dict()
# create an instance of V1beta1DedicatedAIClusterList from a dict
v1beta1_dedicated_ai_cluster_list_from_dict = V1beta1DedicatedAIClusterList.from_dict(v1beta1_dedicated_ai_cluster_list_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

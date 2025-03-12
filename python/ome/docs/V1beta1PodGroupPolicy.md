# V1beta1PodGroupPolicy

PodGroupPolicy represents a PodGroup configuration for gang-scheduling.

## Properties

| Name             | Type                                                                                      | Description | Notes      |
|------------------|-------------------------------------------------------------------------------------------|-------------|------------|
| **coscheduling** | [**V1beta1CoschedulingPodGroupPolicyConfig**](V1beta1CoschedulingPodGroupPolicyConfig.md) |             | [optional] |

## Example

```python
from ome.models.v1beta1_pod_group_policy import V1beta1PodGroupPolicy

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1PodGroupPolicy from a JSON string
v1beta1_pod_group_policy_instance = V1beta1PodGroupPolicy.from_json(json)
# print the JSON string representation of the object
print(V1beta1PodGroupPolicy.to_json())

# convert the object into a dict
v1beta1_pod_group_policy_dict = v1beta1_pod_group_policy_instance.to_dict()
# create an instance of V1beta1PodGroupPolicy from a dict
v1beta1_pod_group_policy_from_dict = V1beta1PodGroupPolicy.from_dict(v1beta1_pod_group_policy_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

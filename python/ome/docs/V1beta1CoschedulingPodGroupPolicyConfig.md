# V1beta1CoschedulingPodGroupPolicyConfig

CoschedulingPodGroupPolicyConfig represents configuration for co-scheduling plugin. The number of min members in the PodGroupSpec is always equal to the number of nodes.

## Properties

| Name                         | Type    | Description                                                                                                                                          | Notes      |
|------------------------------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **schedule_timeout_seconds** | **int** | Time threshold to schedule PodGroup for gang-scheduling. If the scheduling timeout is equal to 0, the default value is used. Defaults to 60 seconds. | [optional] |

## Example

```python
from ome.models.v1beta1_coscheduling_pod_group_policy_config import V1beta1CoschedulingPodGroupPolicyConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1CoschedulingPodGroupPolicyConfig from a JSON string
v1beta1_coscheduling_pod_group_policy_config_instance = V1beta1CoschedulingPodGroupPolicyConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1CoschedulingPodGroupPolicyConfig.to_json())

# convert the object into a dict
v1beta1_coscheduling_pod_group_policy_config_dict = v1beta1_coscheduling_pod_group_policy_config_instance.to_dict()
# create an instance of V1beta1CoschedulingPodGroupPolicyConfig from a dict
v1beta1_coscheduling_pod_group_policy_config_from_dict = V1beta1CoschedulingPodGroupPolicyConfig.from_dict(v1beta1_coscheduling_pod_group_policy_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

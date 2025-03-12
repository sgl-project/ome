# V1beta1DedicatedAIClusterStatus

DedicatedAIClusterStatus defines the observed state of DedicatedAICluster

## Properties

| Name                    | Type                                    | Description                                                                                                                                                | Notes      |
|-------------------------|-----------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **allocated_gpu**       | **int**                                 | The number of GPU already allocated                                                                                                                        | [optional] |
| **available_gpu**       | **int**                                 | The available number of GPU for allocation                                                                                                                 | [optional] |
| **conditions**          | [**List[V1Condition]**](V1Condition.md) | Conditions reflects the current state of the cluster.                                                                                                      | [optional] |
| **dac_lifecycle_state** | **str**                                 | DacLifecycleState indicates the current phase of the Dedicated AI Cluster (e.g., \&quot;active\&quot;, \&quot;creating\&quot;, \&quot;Failed\&quot; etc.). | [optional] |
| **lifecycle_detail**    | **str**                                 | A message describing the current state in more detail that can provide actionable information.                                                             | [optional] |

## Example

```python
from ome.models.v1beta1_dedicated_ai_cluster_status import V1beta1DedicatedAIClusterStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1DedicatedAIClusterStatus from a JSON string
v1beta1_dedicated_ai_cluster_status_instance = V1beta1DedicatedAIClusterStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1DedicatedAIClusterStatus.to_json())

# convert the object into a dict
v1beta1_dedicated_ai_cluster_status_dict = v1beta1_dedicated_ai_cluster_status_instance.to_dict()
# create an instance of V1beta1DedicatedAIClusterStatus from a dict
v1beta1_dedicated_ai_cluster_status_from_dict = V1beta1DedicatedAIClusterStatus.from_dict(v1beta1_dedicated_ai_cluster_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

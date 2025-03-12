# V1beta1DedicatedAIClusterSpec

DedicatedAIClusterSpec defines the desired state of DedicatedAICluster

## Properties

| Name                        | Type                                                                                                                            | Description                                                                                                    | Notes                     |
|-----------------------------|---------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------|---------------------------|
| **affinity**                | [**V1Affinity**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1Affinity.md)                         |                                                                                                                | [optional]                |
| **capacity_reservation_id** | **str**                                                                                                                         | CapacityReservation ID that used to create this DedicatedAICluster.                                            | [optional]                |
| **compartment_id**          | **str**                                                                                                                         | The compartment ID to use for the DAC                                                                          | [optional]                |
| **count**                   | **int**                                                                                                                         | Count is the number of resources in the DAC                                                                    | [optional] [default to 0] |
| **node_selector**           | **Dict[str, str]**                                                                                                              | NodeSelector specifies node selectors for scheduling the resources on specific nodes.                          | [optional]                |
| **priority_class_name**     | **str**                                                                                                                         | PriorityClassName is the priority class assigned to workloads in this Dedicated AI Cluster.                    | [optional]                |
| **profile**                 | **str**                                                                                                                         | DedicatedAIClusterProfileName is the name of the DedicatedAIClusterProfile to use for this DedicatedAICluster. | [optional]                |
| **resources**               | [**V1ResourceRequirements**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ResourceRequirements.md) |                                                                                                                | [optional]                |
| **tolerations**             | [**List[V1Toleration]**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1Toleration.md)               | Tolerations specifies the tolerations for scheduling the resources on tainted nodes.                           | [optional]                |

## Example

```python
from ome.models.v1beta1_dedicated_ai_cluster_spec import V1beta1DedicatedAIClusterSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1DedicatedAIClusterSpec from a JSON string
v1beta1_dedicated_ai_cluster_spec_instance = V1beta1DedicatedAIClusterSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1DedicatedAIClusterSpec.to_json())

# convert the object into a dict
v1beta1_dedicated_ai_cluster_spec_dict = v1beta1_dedicated_ai_cluster_spec_instance.to_dict()
# create an instance of V1beta1DedicatedAIClusterSpec from a dict
v1beta1_dedicated_ai_cluster_spec_from_dict = V1beta1DedicatedAIClusterSpec.from_dict(v1beta1_dedicated_ai_cluster_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

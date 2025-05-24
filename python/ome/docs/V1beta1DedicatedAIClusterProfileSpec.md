# V1beta1DedicatedAIClusterProfileSpec

DedicatedAIClusterProfileSpec defines the desired state of DedicatedAIClusterProfile

## Properties

| Name                    | Type                                                                                                                            | Description                                                                                 | Notes                     |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- | ------------------------- |
| **affinity**            | [**V1Affinity**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1Affinity.md)                         |                                                                                             |
| **count**               | **int**                                                                                                                         | Count is the number of units in the DAC                                                     | [optional] [default to 0] |
| **disabled**            | **bool**                                                                                                                        | Set to true to disable use of this profile.                                                 | [optional]                |
| **node_selector**       | **dict(str, str)**                                                                                                              | NodeSelector specifies node selectors for scheduling the resources on specific nodes.       | [optional]                |
| **priority_class_name** | **str**                                                                                                                         | PriorityClassName is the priority class assigned to workloads in this Dedicated AI Cluster. | [optional]                |
| **resources**           | [**V1ResourceRequirements**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ResourceRequirements.md) |                                                                                             |
| **tolerations**         | [**list[V1Toleration]**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1Toleration.md)               | Tolerations specifies the tolerations for scheduling the resources on tainted nodes.        | [optional]                |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

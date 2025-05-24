# V1beta1DedicatedAIClusterSpec

DedicatedAIClusterSpec defines the desired state of DedicatedAICluster

## Properties

| Name                        | Type                                                                                                                            | Description                                                                                                    | Notes                     |
|-----------------------------|---------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------|---------------------------|
| **affinity**                | [**V1Affinity**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1Affinity.md)                         |                                                                                                                | [optional]                |
| **capacity_reservation_id** | **str**                                                                                                                         | CapacityReservation ID that used to create this DedicatedAICluster.                                            | [optional]                |
| **compartment_id**          | **str**                                                                                                                         | The compartment ID to use for the DAC                                                                          | [optional]                |
| **count**                   | **int**                                                                                                                         | Count is the number of resources in the DAC                                                                    | [optional] [default to 0] |
| **node_selector**           | **dict(str, str)**                                                                                                              | NodeSelector specifies node selectors for scheduling the resources on specific nodes.                          | [optional]                |
| **priority_class_name**     | **str**                                                                                                                         | PriorityClassName is the priority class assigned to workloads in this Dedicated AI Cluster.                    | [optional]                |
| **profile**                 | **str**                                                                                                                         | DedicatedAIClusterProfileName is the name of the DedicatedAIClusterProfile to use for this DedicatedAICluster. | [optional]                |
| **resources**               | [**V1ResourceRequirements**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ResourceRequirements.md) |                                                                                                                | [optional]                |
| **tolerations**             | [**list[V1Toleration]**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1Toleration.md)               | Tolerations specifies the tolerations for scheduling the resources on tainted nodes.                           | [optional]                |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

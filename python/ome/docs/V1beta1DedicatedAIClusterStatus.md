# V1beta1DedicatedAIClusterStatus

DedicatedAIClusterStatus defines the observed state of DedicatedAICluster

## Properties

| Name                    | Type                                    | Description                                                                                                                                                | Notes      |
|-------------------------|-----------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **allocated_gpu**       | **int**                                 | The number of GPU already allocated                                                                                                                        | [optional] |
| **available_gpu**       | **int**                                 | The available number of GPU for allocation                                                                                                                 | [optional] |
| **conditions**          | [**list[V1Condition]**](V1Condition.md) | Conditions reflects the current state of the cluster.                                                                                                      | [optional] |
| **dac_lifecycle_state** | **str**                                 | DacLifecycleState indicates the current phase of the Dedicated AI Cluster (e.g., \&quot;active\&quot;, \&quot;creating\&quot;, \&quot;Failed\&quot; etc.). | [optional] |
| **lifecycle_detail**    | **str**                                 | A message describing the current state in more detail that can provide actionable information.                                                             | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1MLPolicy

MLPolicy represents configuration for the model training with ML-specific parameters.

## Properties

| Name          | Type                                                            | Description                              | Notes      |
|---------------|-----------------------------------------------------------------|------------------------------------------|------------|
| **mpi**       | [**V1beta1MPIMLPolicyConfig**](V1beta1MPIMLPolicyConfig.md)     |                                          | [optional] |
| **num_nodes** | **int**                                                         | Number of training nodes. Defaults to 1. | [optional] |
| **torch**     | [**V1beta1TorchMLPolicyConfig**](V1beta1TorchMLPolicyConfig.md) |                                          | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

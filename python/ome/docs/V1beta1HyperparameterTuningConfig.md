# V1beta1HyperparameterTuningConfig

## Properties

| Name           | Type    | Description                                                        | Notes           |
|----------------|---------|--------------------------------------------------------------------|-----------------|
| **max_trials** | **int** | MaxTrials specifies the maximum number of trials to run            | [optional]      |
| **method**     | **str** | Method specifies the search algorithm to use (grid, random, bayes) | [default to ''] |
| **metric**     | [**V1beta1MetricConfig**](V1beta1MetricConfig.md)                                         |                                                                    |
| **parameters** | [**K8sIoApimachineryPkgRuntimeRawExtension**](K8sIoApimachineryPkgRuntimeRawExtension.md) |                                                                    |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

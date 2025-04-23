# V1beta1ModelStatusSpec

ModelStatusSpec defines the observed state of Model weight

## Properties

| Name             | Type          | Description                                                      | Notes           |
|------------------|---------------|------------------------------------------------------------------|-----------------|
| **lifecycle**    | **str**       | LifeCycle is an enum of Deprecated, Experiment, Public, Internal | [optional]      |
| **nodes_failed** | **list[str]** |                                                                  | [optional]      |
| **nodes_ready**  | **list[str]** |                                                                  | [optional]      |
| **state**        | **str**       | Status of the model weight                                       | [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

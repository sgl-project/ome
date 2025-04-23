# V1beta1ModelRef

## Properties

| Name                   | Type          | Description                                                                                                                          | Notes                      |
|------------------------|---------------|--------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **api_group**          | **str**       | APIGroup of the resource being referenced Defaults to &#x60;ome.io&#x60; Specifies the Kubernetes API group of the referenced model. | [optional]                 |
| **fine_tuned_weights** | **list[str]** | Optional FineTunedWeights references References to fine-tuned weights that should be applied to the base model.                      | [optional]                 |
| **kind**               | **str**       | Kind of the model being referenced Defaults to ClusterBaseModel Specifies the Kubernetes resource kind of the referenced model.      | [optional]                 |
| **name**               | **str**       | Name of the model being referenced Identifies the specific model to be used for inference.                                           | [optional] [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

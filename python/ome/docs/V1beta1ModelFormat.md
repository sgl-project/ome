# V1beta1ModelFormat

## Properties

| Name        | Type    | Description                                                                                                                                                                       | Notes                      |
|-------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **name**    | **str** | Name of the format in which the model is stored, e.g., \&quot;ONNX\&quot;, \&quot;TensorFlow SavedModel\&quot;, \&quot;PyTorch\&quot;, \&quot;SafeTensors\&quot;                  | [optional] [default to ''] |
| **version** | **str** | Version of the model format. Used in validating that a runtime supports a predictor. It Can be \&quot;major\&quot;, \&quot;major.minor\&quot; or \&quot;major.minor.patch\&quot;. | [optional]                 |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1HuggingFaceSecretReference

HuggingFaceSecretReference defines a reference to a Kubernetes Secret containing the Hugging Face API key. This secret must reside in the same namespace as the BenchmarkJob. Cross-namespace references are not allowed for security and simplicity.

## Properties

| Name     | Type    | Description                                                                                                               | Notes                      |
|----------|---------|---------------------------------------------------------------------------------------------------------------------------|----------------------------|
| **name** | **str** | Name of the secret containing the Hugging Face API key. The secret must reside in the same namespace as the BenchmarkJob. | [optional] [default to ''] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

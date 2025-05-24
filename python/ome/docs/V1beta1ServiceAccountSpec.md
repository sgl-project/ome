# V1beta1ServiceAccountSpec

## Properties

| Name       | Type               | Description                                   | Notes      |
|------------|--------------------|-----------------------------------------------|------------|
| **config** | **dict(str, str)** | Config contains vendor-specific configuration | [optional] |
| **name**        | **str**                                               | Name is the service account name                                            |
| **permissions** | **list[str]**                                         | Permissions defines the service account permissions                         | [optional] |
| **project_ref** | [**V1beta1CrossReference**](V1beta1CrossReference.md) |                                                                             |
| **role**        | **str**                                               | Role defines the service account&#39;s role in the project, owner or member | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1OrganizationSpec

## Properties

| Name                | Type                                                    | Description                                             | Notes           |
|---------------------|---------------------------------------------------------|---------------------------------------------------------|-----------------|
| **config**          | **dict(str, str)**                                      | Config contains vendor-specific configuration           | [optional]      |
| **disabled**        | **bool**                                                | Disabled indicates whether the organization is disabled | [optional]      |
| **organization_id** | **str**                                                 | OrganizationId is the platform-specific organization ID | [default to ''] |
| **secret_ref**      | [**V1beta1SecretReference**](V1beta1SecretReference.md) |                                                         | [optional]      |
| **vendor**          | **str**                                                 | Vendor specifies the AI platform vendor (e.g., \&quot;openai\&quot;, \&quot;anthropic\&quot;) |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

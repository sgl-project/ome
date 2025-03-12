# V1beta1OCIConfig

## Properties

| Name                       | Type    | Description                                                                            | Notes           |
|----------------------------|---------|----------------------------------------------------------------------------------------|-----------------|
| **ad_number_name**         | **str** | AdNumberName for all applications                                                      | [default to ''] |
| **airport_code**           | **str** | AirportCode for all applications                                                       | [default to ''] |
| **application_stage**      | **str** | ApplicationStage for all applications                                                  | [default to ''] |
| **internal_domain_name**   | **str** | InternalDomainName for all applications                                                | [default to ''] |
| **namespace**              | **str** | Namespace for service tenancy                                                          | [default to ''] |
| **public_domain_name**     | **str** | PublicDomainName for all applications                                                  | [default to ''] |
| **realm**                  | **str** | Realm for all applications                                                             | [default to ''] |
| **region**                 | **str** | Region for all applications                                                            | [default to ''] |
| **service_compartment_id** | **str** | compartment OCID, this is defaulted to the compartment OCID in agent service configMap | [default to ''] |
| **service_tenancy_id**     | **str** | service tenancy OCID, this is defaulted to the tenancy OCID in agent service configMap | [default to ''] |
| **stage**                  | **str** | Stage for all applications                                                             | [default to ''] |

## Example

```python
from ome.models.v1beta1_oci_config import V1beta1OCIConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1OCIConfig from a JSON string
v1beta1_oci_config_instance = V1beta1OCIConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1OCIConfig.to_json())

# convert the object into a dict
v1beta1_oci_config_dict = v1beta1_oci_config_instance.to_dict()
# create an instance of V1beta1OCIConfig from a dict
v1beta1_oci_config_from_dict = V1beta1OCIConfig.from_dict(v1beta1_oci_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

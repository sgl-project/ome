# V1beta1ServiceAccountStatus

## Properties

| Name                   | Type                                          | Description                                                                     | Notes      |
|------------------------|-----------------------------------------------|---------------------------------------------------------------------------------|------------|
| **api_key**            | [**V1beta1APIKeySpec**](V1beta1APIKeySpec.md) |                                                                                 | [optional] |
| **conditions**         | [**List[V1Condition]**](V1Condition.md)       | Conditions represent the latest available observations of an object&#39;s state | [optional] |
| **creation_time**      | [**V1Time**](V1Time.md)                       |                                                                                 | [optional] |
| **service_account_id** | **str**                                       | ServiceAccountId is the platform-specific service account ID                    | [optional] |

## Example

```python
from ome.models.v1beta1_service_account_status import V1beta1ServiceAccountStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1ServiceAccountStatus from a JSON string
v1beta1_service_account_status_instance = V1beta1ServiceAccountStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1ServiceAccountStatus.to_json())

# convert the object into a dict
v1beta1_service_account_status_dict = v1beta1_service_account_status_instance.to_dict()
# create an instance of V1beta1ServiceAccountStatus from a dict
v1beta1_service_account_status_from_dict = V1beta1ServiceAccountStatus.from_dict(v1beta1_service_account_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

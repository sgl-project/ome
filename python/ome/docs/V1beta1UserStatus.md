# V1beta1UserStatus

## Properties

| Name              | Type                                    | Description                                                                     | Notes           |
|-------------------|-----------------------------------------|---------------------------------------------------------------------------------|-----------------|
| **conditions**    | [**List[V1Condition]**](V1Condition.md) | Conditions represent the latest available observations of an object&#39;s state | [optional]      |
| **creation_time** | [**V1Time**](V1Time.md)                 |                                                                                 | [optional]      |
| **user_id**       | **str**                                 | UserID is the platform-specific user ID                                         | [default to ''] |

## Example

```python
from ome.models.v1beta1_user_status import V1beta1UserStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1UserStatus from a JSON string
v1beta1_user_status_instance = V1beta1UserStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1UserStatus.to_json())

# convert the object into a dict
v1beta1_user_status_dict = v1beta1_user_status_instance.to_dict()
# create an instance of V1beta1UserStatus from a dict
v1beta1_user_status_from_dict = V1beta1UserStatus.from_dict(v1beta1_user_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

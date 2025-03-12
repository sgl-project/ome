# V1beta1RateLimitStatus

## Properties

| Name           | Type                                    | Description                                                                     | Notes      |
|----------------|-----------------------------------------|---------------------------------------------------------------------------------|------------|
| **conditions** | [**List[V1Condition]**](V1Condition.md) | Conditions represent the latest available observations of an object&#39;s state | [optional] |

## Example

```python
from ome.models.v1beta1_rate_limit_status import V1beta1RateLimitStatus

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1RateLimitStatus from a JSON string
v1beta1_rate_limit_status_instance = V1beta1RateLimitStatus.from_json(json)
# print the JSON string representation of the object
print(V1beta1RateLimitStatus.to_json())

# convert the object into a dict
v1beta1_rate_limit_status_dict = v1beta1_rate_limit_status_instance.to_dict()
# create an instance of V1beta1RateLimitStatus from a dict
v1beta1_rate_limit_status_from_dict = V1beta1RateLimitStatus.from_dict(v1beta1_rate_limit_status_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1RateLimitConfig

## Properties

| Name       | Type    | Description                                                                         | Notes           |
|------------|---------|-------------------------------------------------------------------------------------|-----------------|
| **limit**  | **int** | Limit is the maximum allowed value                                                  | [default to 0]  |
| **type**   | **str** | Type is the type of rate limit (e.g., \&quot;requests\&quot;, \&quot;tokens\&quot;) | [default to ''] |
| **window** | **str** | Window is the time window for the limit (e.g., \&quot;1m\&quot;, \&quot;1d\&quot;)  | [default to ''] |

## Example

```python
from ome.models.v1beta1_rate_limit_config import V1beta1RateLimitConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1RateLimitConfig from a JSON string
v1beta1_rate_limit_config_instance = V1beta1RateLimitConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1RateLimitConfig.to_json())

# convert the object into a dict
v1beta1_rate_limit_config_dict = v1beta1_rate_limit_config_instance.to_dict()
# create an instance of V1beta1RateLimitConfig from a dict
v1beta1_rate_limit_config_from_dict = V1beta1RateLimitConfig.from_dict(v1beta1_rate_limit_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1RateLimitSpec

## Properties

| Name       | Type               | Description                                   | Notes      |
|------------|--------------------|-----------------------------------------------|------------|
| **config** | **Dict[str, str]** | Config contains vendor-specific configuration | [optional] |
| **limits**      | [**List[V1beta1RateLimitConfig]**](V1beta1RateLimitConfig.md) | Limits defines the rate limit configurations  |
| **project_ref** | [**V1beta1CrossReference**](V1beta1CrossReference.md)         |                                               |
| **target_ref**  | [**V1beta1CrossReference**](V1beta1CrossReference.md)         |                                               |

## Example

```python
from ome.models.v1beta1_rate_limit_spec import V1beta1RateLimitSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1RateLimitSpec from a JSON string
v1beta1_rate_limit_spec_instance = V1beta1RateLimitSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1RateLimitSpec.to_json())

# convert the object into a dict
v1beta1_rate_limit_spec_dict = v1beta1_rate_limit_spec_instance.to_dict()
# create an instance of V1beta1RateLimitSpec from a dict
v1beta1_rate_limit_spec_from_dict = V1beta1RateLimitSpec.from_dict(v1beta1_rate_limit_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

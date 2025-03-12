# V1beta1MetricConfig

MetricConfig defines the metric to optimize during hyperparameter tuning

## Properties

| Name     | Type    | Description                                               | Notes           |
|----------|---------|-----------------------------------------------------------|-----------------|
| **goal** | **str** | Goal indicates whether to minimize or maximize the metric | [default to ''] |
| **name** | **str** | Name of the metric                                        | [default to ''] |

## Example

```python
from ome.models.v1beta1_metric_config import V1beta1MetricConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1MetricConfig from a JSON string
v1beta1_metric_config_instance = V1beta1MetricConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1MetricConfig.to_json())

# convert the object into a dict
v1beta1_metric_config_dict = v1beta1_metric_config_instance.to_dict()
# create an instance of V1beta1MetricConfig from a dict
v1beta1_metric_config_from_dict = V1beta1MetricConfig.from_dict(v1beta1_metric_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1HyperparameterTuningConfig

## Properties

| Name           | Type    | Description                                                        | Notes           |
|----------------|---------|--------------------------------------------------------------------|-----------------|
| **max_trials** | **int** | MaxTrials specifies the maximum number of trials to run            | [optional]      |
| **method**     | **str** | Method specifies the search algorithm to use (grid, random, bayes) | [default to ''] |
| **metric**     | [**V1beta1MetricConfig**](V1beta1MetricConfig.md)                                         |                                                                    |
| **parameters** | [**K8sIoApimachineryPkgRuntimeRawExtension**](K8sIoApimachineryPkgRuntimeRawExtension.md) |                                                                    |

## Example

```python
from ome.models.v1beta1_hyperparameter_tuning_config import V1beta1HyperparameterTuningConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1HyperparameterTuningConfig from a JSON string
v1beta1_hyperparameter_tuning_config_instance = V1beta1HyperparameterTuningConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1HyperparameterTuningConfig.to_json())

# convert the object into a dict
v1beta1_hyperparameter_tuning_config_dict = v1beta1_hyperparameter_tuning_config_instance.to_dict()
# create an instance of V1beta1HyperparameterTuningConfig from a dict
v1beta1_hyperparameter_tuning_config_from_dict = V1beta1HyperparameterTuningConfig.from_dict(v1beta1_hyperparameter_tuning_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1TrainingJobSpec

TrainingJobSpec defines the base job spec which various training job specs implement. It defines the desired state of a training job

## Properties

| Name               | Type               | Description                                                                                                   | Notes      |
|--------------------|--------------------|---------------------------------------------------------------------------------------------------------------|------------|
| **annotations**    | **Dict[str, str]** | Annotations to apply for the derivative JobSet and Jobs. They will be merged with the TrainingRuntime values. | [optional] |
| **compartment_id** | **str**            | The compartment ID to use for the training job                                                                | [optional] |
| **datasets**              | [**V1beta1StorageSpec**](V1beta1StorageSpec.md)                               |                                                                                                               |
| **hyperparameter_config** | [**V1beta1HyperparameterTuningConfig**](V1beta1HyperparameterTuningConfig.md) |                                                                                                               | [optional] |
| **labels**                | **Dict[str, str]**                                                            | Labels to apply for the derivative JobSet and Jobs. They will be merged with the TrainingRuntime values.      | [optional] |
| **model_config**          | [**V1beta1ModelConfig**](V1beta1ModelConfig.md)                               |                                                                                                               |
| **runtime_ref**           | [**V1beta1RuntimeRef**](V1beta1RuntimeRef.md)                                 |                                                                                                               |
| **suspend**               | **bool**                                                                      | Whether the controller should suspend the running TrainJob. Defaults to false.                                | [optional] |
| **trainer**               | [**V1beta1TrainerSpec**](V1beta1TrainerSpec.md)                               |                                                                                                               |

## Example

```python
from ome.models.v1beta1_training_job_spec import V1beta1TrainingJobSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1TrainingJobSpec from a JSON string
v1beta1_training_job_spec_instance = V1beta1TrainingJobSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1TrainingJobSpec.to_json())

# convert the object into a dict
v1beta1_training_job_spec_dict = v1beta1_training_job_spec_instance.to_dict()
# create an instance of V1beta1TrainingJobSpec from a dict
v1beta1_training_job_spec_from_dict = V1beta1TrainingJobSpec.from_dict(v1beta1_training_job_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

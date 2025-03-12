# V1beta1TrainerSpec

## Properties

| Name                   | Type                                                                                                                            | Description                                                                                                                                                                                                  | Notes      |
|------------------------|---------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **args**               | **List[str]**                                                                                                                   | Arguments to the entrypoint for the training container.                                                                                                                                                      | [optional] |
| **command**            | **List[str]**                                                                                                                   | Entrypoint commands for the training container.                                                                                                                                                              | [optional] |
| **env**                | [**List[V1EnvVar]**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1EnvVar.md)                       | List of environment variables to set in the training container. These values will be merged with the TrainingRuntime&#39;s trainer environments.                                                             | [optional] |
| **image**              | **str**                                                                                                                         | Docker image for the training container.                                                                                                                                                                     | [optional] |
| **num_nodes**          | **int**                                                                                                                         | Number of training nodes.                                                                                                                                                                                    | [optional] |
| **num_proc_per_node**  | **str**                                                                                                                         | Number of processes/workers/slots on every training node. For the Torch runtime: &#x60;auto&#x60;, &#x60;cpu&#x60;, &#x60;gpu&#x60;, or int value can be set. For the MPI runtime only int value can be set. | [optional] |
| **resources_per_node** | [**V1ResourceRequirements**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ResourceRequirements.md) |                                                                                                                                                                                                              | [optional] |

## Example

```python
from ome.models.v1beta1_trainer_spec import V1beta1TrainerSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1TrainerSpec from a JSON string
v1beta1_trainer_spec_instance = V1beta1TrainerSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1TrainerSpec.to_json())

# convert the object into a dict
v1beta1_trainer_spec_dict = v1beta1_trainer_spec_instance.to_dict()
# create an instance of V1beta1TrainerSpec from a dict
v1beta1_trainer_spec_from_dict = V1beta1TrainerSpec.from_dict(v1beta1_trainer_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

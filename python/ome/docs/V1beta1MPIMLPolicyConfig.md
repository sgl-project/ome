# V1beta1MPIMLPolicyConfig

MPIMLPolicyConfig represents a MPI runtime configuration.

## Properties

| Name                     | Type     | Description                                                                                             | Notes      |
|--------------------------|----------|---------------------------------------------------------------------------------------------------------|------------|
| **mpi_implementation**   | **str**  | Implementation name for the MPI to create the appropriate hostfile. Defaults to OpenMPI.                | [optional] |
| **num_proc_per_node**    | **int**  | Number of processes per node. This value is equal to the number of slots for each node in the hostfile. | [optional] |
| **run_launcher_as_node** | **bool** | Whether to run training process on the launcher Job. Defaults to false.                                 | [optional] |
| **ssh_auth_mount_path**  | **str**  | Directory where SSH keys are mounted.                                                                   | [optional] |

## Example

```python
from ome.models.v1beta1_mpiml_policy_config import V1beta1MPIMLPolicyConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1MPIMLPolicyConfig from a JSON string
v1beta1_mpiml_policy_config_instance = V1beta1MPIMLPolicyConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1MPIMLPolicyConfig.to_json())

# convert the object into a dict
v1beta1_mpiml_policy_config_dict = v1beta1_mpiml_policy_config_instance.to_dict()
# create an instance of V1beta1MPIMLPolicyConfig from a dict
v1beta1_mpiml_policy_config_from_dict = V1beta1MPIMLPolicyConfig.from_dict(v1beta1_mpiml_policy_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

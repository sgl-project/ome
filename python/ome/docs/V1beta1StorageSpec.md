# V1beta1StorageSpec

## Properties

| Name            | Type               | Description                                                                                 | Notes      |
|-----------------|--------------------|---------------------------------------------------------------------------------------------|------------|
| **key**         | **str**            | The Storage Key in the secret for this model.                                               | [optional] |
| **parameters**  | **Dict[str, str]** | Parameters to override the default storage credentials and config.                          | [optional] |
| **path**        | **str**            | The path to the model where it will be downloaded. Default is /mnt/models/vendor/model-name | [optional] |
| **schema_path** | **str**            | The path to the model schema file in the storage.                                           | [optional] |
| **storage_uri** | **str**            | The path to the model object in storage. Supported storage types: - OCI object storage (e.g., oci://n/{namespace}/b/{bucket}/o/{object_path}) - PVC storage (e.g., pvc://{pvc-name}/{sub-path}) - Vendor storage (e.g., vendor://{vendor-name}/{resource-type}/{resource-path}) |

## Example

```python
from ome.models.v1beta1_storage_spec import V1beta1StorageSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1StorageSpec from a JSON string
v1beta1_storage_spec_instance = V1beta1StorageSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1StorageSpec.to_json())

# convert the object into a dict
v1beta1_storage_spec_dict = v1beta1_storage_spec_instance.to_dict()
# create an instance of V1beta1StorageSpec from a dict
v1beta1_storage_spec_from_dict = V1beta1StorageSpec.from_dict(v1beta1_storage_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

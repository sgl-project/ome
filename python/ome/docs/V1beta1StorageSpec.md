# V1beta1StorageSpec

## Properties

| Name              | Type                                    | Description                                                                                                                                                              | Notes      |
|-------------------|-----------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **key**           | **str**                                 | The Storage Key in the secret for this model.                                                                                                                            | [optional] |
| **node_affinity** | [**V1NodeAffinity**](V1NodeAffinity.md) |                                                                                                                                                                          | [optional] |
| **node_selector** | **dict(str, str)**                      | NodeSelector is a selector which must be true for the model to fit on a node. Selector which must match a node&#39;s labels for the model to be downloaded on that node. | [optional] |
| **parameters**    | **dict(str, str)**                      | Parameters to override the default storage credentials and config.                                                                                                       | [optional] |
| **path**          | **str**                                 | The path to the model where it will be downloaded. Default is /mnt/models/vendor/model-name                                                                              | [optional] |
| **schema_path**   | **str**                                 | The path to the model schema file in the storage.                                                                                                                        | [optional] |
| **storage_uri**   | **str**                                 | The path to the model object in storage. Supported storage types: - OCI object storage (e.g., oci://n/{namespace}/b/{bucket}/o/{object_path}) - PVC storage (e.g., pvc://{pvc-name}/{sub-path}) - Vendor storage (e.g., vendor://{vendor-name}/{resource-type}/{resource-path}) |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

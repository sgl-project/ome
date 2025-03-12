# V1beta1JobSetTemplateSpec

JobSetTemplateSpec represents a template of the desired JobSet.

## Properties

| Name         | Type                                                                                                        | Description | Notes      |
|--------------|-------------------------------------------------------------------------------------------------------------|-------------|------------|
| **metadata** | [**V1ObjectMeta**](https://github.com/kubernetes-client/python/blob/master/kubernetes/docs/V1ObjectMeta.md) |             | [optional] |
| **spec**     | [**SigsK8sIoJobsetApiJobsetV1alpha2JobSetSpec**](SigsK8sIoJobsetApiJobsetV1alpha2JobSetSpec.md)             |             | [optional] |

## Example

```python
from ome.models.v1beta1_job_set_template_spec import V1beta1JobSetTemplateSpec

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1JobSetTemplateSpec from a JSON string
v1beta1_job_set_template_spec_instance = V1beta1JobSetTemplateSpec.from_json(json)
# print the JSON string representation of the object
print(V1beta1JobSetTemplateSpec.to_json())

# convert the object into a dict
v1beta1_job_set_template_spec_dict = v1beta1_job_set_template_spec_instance.to_dict()
# create an instance of V1beta1JobSetTemplateSpec from a dict
v1beta1_job_set_template_spec_from_dict = V1beta1JobSetTemplateSpec.from_dict(v1beta1_job_set_template_spec_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

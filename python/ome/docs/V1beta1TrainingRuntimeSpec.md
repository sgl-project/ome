# V1beta1TrainingRuntimeSpec

TrainingRuntimeSpec defines the desired state of TrainingRuntime

## Properties

| Name                 | Type                                                  | Description                                                                                                     | Notes      |
|----------------------|-------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------|------------|
| **annotations**      | **dict(str, str)**                                    | Annotations that will be added to the runtime spec. More info: http://kubernetes.io/docs/user-guide/annotations | [optional] |
| **compartment_id**   | **str**                                               | The compartment ID to use for the training runtime                                                              | [optional] |
| **labels**           | **dict(str, str)**                                    | Labels that will be added to the runtime spec. More info: http://kubernetes.io/docs/user-guide/labels           | [optional] |
| **ml_policy**        | [**V1beta1MLPolicy**](V1beta1MLPolicy.md)             |                                                                                                                 | [optional] |
| **pod_group_policy** | [**V1beta1PodGroupPolicy**](V1beta1PodGroupPolicy.md) |                                                                                                                 | [optional] |
| **template**         | [**V1beta1JobSetTemplateSpec**](V1beta1JobSetTemplateSpec.md) |                                                                                                                 |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

# V1beta1InferenceServiceSpec

InferenceServiceSpec is the top level type for this resource

## Properties

| Name               | Type                                            | Description                                                                                                                              | Notes      |
|--------------------|-------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **compartment_id** | **str**                                         | The compartment ID to use for the inference service Specifies the OCI compartment where the inference service resources will be created. | [optional] |
| **decoder**        | [**V1beta1DecoderSpec**](V1beta1DecoderSpec.md) |                                                                                                                                          | [optional] |
| **engine**         | [**V1beta1EngineSpec**](V1beta1EngineSpec.md)   |                                                                                                                                          | [optional] |
| **keda_config**    | [**V1beta1KedaConfig**](V1beta1KedaConfig.md)   |                                                                                                                                          | [optional] |
| **model**          | [**V1beta1ModelRef**](V1beta1ModelRef.md)       |                                                                                                                                          | [optional] |
| **predictor**      | [**V1beta1PredictorSpec**](V1beta1PredictorSpec.md)         |                                                                                                                                          |
| **router**         | [**V1beta1RouterSpec**](V1beta1RouterSpec.md)               |                                                                                                                                          | [optional] |
| **runtime**        | [**V1beta1ServingRuntimeRef**](V1beta1ServingRuntimeRef.md) |                                                                                                                                          | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

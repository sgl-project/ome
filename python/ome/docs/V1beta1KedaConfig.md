# V1beta1KedaConfig

KedaConfig stores the configuration settings for KEDA autoscaling within the InferenceService. It includes fields like the Prometheus server address, custom query, scaling threshold, and operator.

## Properties

| Name                    | Type     | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Notes      |
|-------------------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **custom_prom_query**   | **str**  | CustomPromQuery defines a custom Prometheus query that KEDA will execute to evaluate the desired metric for scaling. This query should return a single numerical value that represents the metric to be monitored. Example: avg_over_time(http_requests_total{service&#x3D;\&quot;llama\&quot;}[5m])                                                                                                                                                                                | [optional] |
| **enable_keda**         | **bool** | EnableKeda determines whether KEDA autoscaling is enabled for the InferenceService. - true: KEDA will manage the autoscaling based on the provided configuration. - false: KEDA will not be used, and autoscaling will rely on other mechanisms (e.g., HPA).                                                                                                                                                                                                                        | [optional] |
| **prom_server_address** | **str**  | PromServerAddress specifies the address of the Prometheus server that KEDA will query to retrieve metrics for autoscaling decisions. This should be a fully qualified URL, including the protocol and port number. Example: http://prometheus-operated.monitoring.svc.cluster.local:9090                                                                                                                                                                                            | [optional] |
| **scaling_operator**    | **str**  | ScalingOperator specifies the comparison operator used by KEDA to decide whether to scale the Deployment. Common operators include: - \&quot;GreaterThanOrEqual\&quot;: Scale up when the metric is &gt;&#x3D; ScalingThreshold. - \&quot;LessThanOrEqual\&quot;: Scale down when the metric is &lt;&#x3D; ScalingThreshold. This operator defines the condition under which scaling actions are triggered based on the evaluated metric. Example: \&quot;GreaterThanOrEqual\&quot; | [optional] |
| **scaling_threshold**   | **str**  | ScalingThreshold sets the numerical threshold against which the result of the Prometheus query will be compared. Depending on the ScalingOperator, this threshold determines when to scale the number of replicas up or down. Example: \&quot;10\&quot; - The Autoscaler will compare the metric value to 10.                                                                                                                                                                       | [optional] |

## Example

```python
from ome.models.v1beta1_keda_config import V1beta1KedaConfig

# TODO update the JSON string below
json = "{}"
# create an instance of V1beta1KedaConfig from a JSON string
v1beta1_keda_config_instance = V1beta1KedaConfig.from_json(json)
# print the JSON string representation of the object
print(V1beta1KedaConfig.to_json())

# convert the object into a dict
v1beta1_keda_config_dict = v1beta1_keda_config_instance.to_dict()
# create an instance of V1beta1KedaConfig from a dict
v1beta1_keda_config_from_dict = V1beta1KedaConfig.from_dict(v1beta1_keda_config_dict)
```

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

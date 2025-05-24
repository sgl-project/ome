# V1beta1LoggerSpec

LoggerSpec specifies optional payload logging available for all components Configures how request and response payloads are logged for auditing and debugging.

## Properties

| Name     | Type    | Description                                                                                                                                                                                                                                                                 | Notes      |
|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| **mode** | **str** | Specifies the scope of the loggers. &lt;br /&gt; Valid values are: &lt;br /&gt; - \&quot;all\&quot; (default): log both request and response; &lt;br /&gt; - \&quot;request\&quot;: log only request; &lt;br /&gt; - \&quot;response\&quot;: log only response &lt;br /&gt; | [optional] |
| **url**  | **str** | URL to send logging events The endpoint where log data will be sent for external processing or storage.                                                                                                                                                                     | [optional] |

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

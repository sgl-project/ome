# model-serving

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.16.0](https://img.shields.io/badge/AppVersion-1.16.0-informational?style=flat-square)

A Helm chart for Kubernetes

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| commonAnnotations."chainsaw.k8s-integration.oracle.com/compartmentId" | string | `"ocid1.compartment.oc1..aaaaaaaathgntpo75bdehisnl6wkxfc4slkd6rpheafbt5a6ekm2ri4bmeva"` |  |
| commonAnnotations."chainsaw.k8s-integration.oracle.com/inject" | string | `"enabled"` |  |
| commonAnnotations."chainsaw.k8s-integration.oracle.com/logPath" | string | `"vllm"` |  |
| commonAnnotations."chainsaw.k8s-integration.oracle.com/namespace" | string | `"vllm"` |  |
| commonAnnotations."linkerd.io/inject" | string | `"enabled"` |  |
| commonAnnotations."prometheus.io/path" | string | `"/metrics"` |  |
| commonAnnotations."prometheus.io/port" | string | `"8080"` |  |
| commonAnnotations."prometheus.io/scrape" | string | `"true"` |  |
| commonLabels.logging-forward | string | `"enabled"` |  |
| commonTolerations[0].effect | string | `"NoSchedule"` |  |
| commonTolerations[0].key | string | `"nvidia.com/gpu"` |  |
| commonTolerations[0].operator | string | `"Exists"` |  |
| e5_mistral_7b_instruct.affinity | object | `{}` |  |
| e5_mistral_7b_instruct.annotations | object | `{}` |  |
| e5_mistral_7b_instruct.enabled | bool | `false` |  |
| e5_mistral_7b_instruct.image.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| e5_mistral_7b_instruct.image.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| e5_mistral_7b_instruct.maxReplicas | int | `1` |  |
| e5_mistral_7b_instruct.minReplicas | int | `1` |  |
| e5_mistral_7b_instruct.scaleMetric | object | `{}` |  |
| e5_mistral_7b_instruct.tolerations | list | `[]` |  |
| e5_mistral_7b_instruct.topologySpreadConstraints | list | `[]` |  |
| e5_mistral_7b_instruct.volumes | list | `[]` |  |
| llama_3_1_405b_instruct.affinity | object | `{}` |  |
| llama_3_1_405b_instruct.annotations."config.linkerd.io/skip-inbound-ports" | string | `"1-8079,8081-65535"` |  |
| llama_3_1_405b_instruct.annotations."config.linkerd.io/skip-outbound-ports" | string | `"1-8079,8081-65535"` |  |
| llama_3_1_405b_instruct.enableRDMA | bool | `false` |  |
| llama_3_1_405b_instruct.enabled | bool | `false` |  |
| llama_3_1_405b_instruct.image.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| llama_3_1_405b_instruct.image.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| llama_3_1_405b_instruct.maxReplicas | int | `1` |  |
| llama_3_1_405b_instruct.minReplicas | int | `1` |  |
| llama_3_1_405b_instruct.tolerations | list | `[]` |  |
| llama_3_1_405b_instruct.topologySpreadConstraints | list | `[]` |  |
| llama_3_1_405b_instruct.volumes | list | `[]` |  |
| llama_3_1_70b_instruct.affinity | object | `{}` |  |
| llama_3_1_70b_instruct.annotations | object | `{}` |  |
| llama_3_1_70b_instruct.enabled | bool | `false` |  |
| llama_3_1_70b_instruct.image.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| llama_3_1_70b_instruct.image.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| llama_3_1_70b_instruct.maxReplicas | int | `1` |  |
| llama_3_1_70b_instruct.minReplicas | int | `1` |  |
| llama_3_1_70b_instruct.scaleMetric | object | `{}` |  |
| llama_3_1_70b_instruct.tolerations | list | `[]` |  |
| llama_3_1_70b_instruct.topologySpreadConstraints | list | `[]` |  |
| llama_3_1_70b_instruct.volumes | list | `[]` |  |
| llama_3_2_11b_vision_instruct.affinity | object | `{}` |  |
| llama_3_2_11b_vision_instruct.annotations | object | `{}` |  |
| llama_3_2_11b_vision_instruct.enabled | bool | `false` |  |
| llama_3_2_11b_vision_instruct.image.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| llama_3_2_11b_vision_instruct.image.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| llama_3_2_11b_vision_instruct.maxReplicas | int | `1` |  |
| llama_3_2_11b_vision_instruct.minReplicas | int | `1` |  |
| llama_3_2_11b_vision_instruct.scaleMetric | object | `{}` |  |
| llama_3_2_11b_vision_instruct.tolerations | list | `[]` |  |
| llama_3_2_11b_vision_instruct.topologySpreadConstraints | list | `[]` |  |
| llama_3_2_11b_vision_instruct.volumes | list | `[]` |  |
| llama_3_2_90b_vision_fp8_dynamic.affinity | object | `{}` |  |
| llama_3_2_90b_vision_fp8_dynamic.annotations | object | `{}` |  |
| llama_3_2_90b_vision_fp8_dynamic.enabled | bool | `false` |  |
| llama_3_2_90b_vision_fp8_dynamic.image.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| llama_3_2_90b_vision_fp8_dynamic.image.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| llama_3_2_90b_vision_fp8_dynamic.maxReplicas | int | `1` |  |
| llama_3_2_90b_vision_fp8_dynamic.minReplicas | int | `1` |  |
| llama_3_2_90b_vision_fp8_dynamic.scaleMetric | object | `{}` |  |
| llama_3_2_90b_vision_fp8_dynamic.tolerations | list | `[]` |  |
| llama_3_2_90b_vision_fp8_dynamic.topologySpreadConstraints | list | `[]` |  |
| llama_3_2_90b_vision_fp8_dynamic.volumes | list | `[]` |  |
| llama_3_2_90b_vision_instruct.affinity | object | `{}` |  |
| llama_3_2_90b_vision_instruct.annotations | object | `{}` |  |
| llama_3_2_90b_vision_instruct.enabled | bool | `false` |  |
| llama_3_2_90b_vision_instruct.image.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| llama_3_2_90b_vision_instruct.image.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| llama_3_2_90b_vision_instruct.maxReplicas | int | `1` |  |
| llama_3_2_90b_vision_instruct.minReplicas | int | `1` |  |
| llama_3_2_90b_vision_instruct.scaleMetric | object | `{}` |  |
| llama_3_2_90b_vision_instruct.tolerations | list | `[]` |  |
| llama_3_2_90b_vision_instruct.topologySpreadConstraints | list | `[]` |  |
| llama_3_2_90b_vision_instruct.volumes | list | `[]` |  |
| phi_3_mini_128k_instruct.affinity | object | `{}` |  |
| phi_3_mini_128k_instruct.annotations | object | `{}` |  |
| phi_3_mini_128k_instruct.enabled | bool | `false` |  |
| phi_3_mini_128k_instruct.image.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| phi_3_mini_128k_instruct.image.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| phi_3_mini_128k_instruct.maxReplicas | int | `1` |  |
| phi_3_mini_128k_instruct.minReplicas | int | `1` |  |
| phi_3_mini_128k_instruct.scaleMetric | object | `{}` |  |
| phi_3_mini_128k_instruct.tolerations | list | `[]` |  |
| phi_3_mini_128k_instruct.topologySpreadConstraints | list | `[]` |  |
| phi_3_mini_128k_instruct.volumes | list | `[]` |  |
| vllm.commonImage.repository | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"` |  |
| vllm.commonImage.tag | string | `"0.5.3.post1.7f8d612d-log"` |  |
| vllm.llama3_2_image.tag | string | `"v0.6.2.e019c49f"` |  |
| vllm.port | int | `8080` |  |
| vllm.serveModelName | string | `"vllm-model"` |  |


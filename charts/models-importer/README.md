# models-importer

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart for importing ome base models

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| cohere-command-r-082024-v1-6-tp4-128k.enabled | bool | `false` |  |
| cohere-command-r-082024-v1-6-tp4-128k.lifecyclePhase | string | `"ACTIVE"` |  |
| cohere-command-r-082024-v1-7-tp1-128k.enabled | bool | `false` |  |
| cohere-command-r-082024-v1-7-tp1-128k.lifecyclePhase | string | `"ACTIVE"` |  |
| cohere-command-r-bs128-16k-chat.enabled | bool | `false` |  |
| cohere-command-r-bs128-16k-chat.lifecyclePhase | string | `"ONDEMAND_SERVING_DISABLED"` |  |
| cohere-command-r-plus-082024-v1-6-128k.enabled | bool | `false` |  |
| cohere-command-r-plus-082024-v1-6-128k.lifecyclePhase | string | `"ACTIVE"` |  |
| cohere-command-r-plus-v1-2-chat.enabled | bool | `false` |  |
| cohere-command-r-plus-v1-2-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| cohere-command-r-v1-2-16k-chat.enabled | bool | `false` |  |
| cohere-command-r-v1-2-16k-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| cohere-embed-english-light-v2-0.enabled | bool | `false` |  |
| cohere-embed-english-light-v2-0.lifecyclePhase | string | `"BASEMODEL_LIFE_CYCLE_PHASE_DEPRECATED"` |  |
| cohere-embed-english-light-v3-0.enabled | bool | `false` |  |
| cohere-embed-english-light-v3-0.lifecyclePhase | string | `"BASEMODEL_LIFE_CYCLE_PHASE_DEPRECATED"` |  |
| cohere-embed-english-v3-0.enabled | bool | `false` |  |
| cohere-embed-english-v3-0.lifecyclePhase | string | `"BASEMODEL_LIFE_CYCLE_PHASE_DEPRECATED"` |  |
| cohere-embed-multilingual-light-v3-0.enabled | bool | `false` |  |
| cohere-embed-multilingual-light-v3-0.lifecyclePhase | string | `"BASEMODEL_LIFE_CYCLE_PHASE_DEPRECATED"` |  |
| cohere-embed-multilingual-v3-0.enabled | bool | `false` |  |
| cohere-embed-multilingual-v3-0.lifecyclePhase | string | `"BASEMODEL_LIFE_CYCLE_PHASE_DEPRECATED"` |  |
| compartmentID | string | `"ocid1.compartment.oc1..aaaaaaaazq"` |  |
| deberta-v3-base-prompt-injection-v2.enabled | bool | `false` |  |
| e5-mistral-7b-instruct.enabled | bool | `false` |  |
| e5-mistral-7b-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| gliner-large-v2-1.enabled | bool | `false` |  |
| llama-3-1-405b-instruct-fp8.enabled | bool | `false` |  |
| llama-3-1-405b-instruct-fp8.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-1-70b-instruct.enabled | bool | `false` |  |
| llama-3-1-70b-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-11b-vision-instruct.enabled | bool | `false` |  |
| llama-3-2-11b-vision-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-11b-vision.enabled | bool | `false` |  |
| llama-3-2-11b-vision.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-1b-instruct.enabled | bool | `false` |  |
| llama-3-2-1b-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-1b.enabled | bool | `false` |  |
| llama-3-2-1b.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-3b-instruct.enabled | bool | `false` |  |
| llama-3-2-3b-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-3b.enabled | bool | `false` |  |
| llama-3-2-3b.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-90b-vision-instruct-fp8-dynamic.enabled | bool | `false` |  |
| llama-3-2-90b-vision-instruct-fp8-dynamic.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-90b-vision-instruct.enabled | bool | `false` |  |
| llama-3-2-90b-vision-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-2-90b-vision.enabled | bool | `false` |  |
| llama-3-2-90b-vision.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-3-70b-instruct.enabled | bool | `false` |  |
| llama-3-3-70b-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-70b-instruct.enabled | bool | `false` |  |
| llama-3-70b-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| mistral-7b-instruct-v0-2-ft-action-v0-0-chat.enabled | bool | `false` |  |
| mistral-7b-instruct-v0-2-ft-action-v0-0-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| mistral-7b-instruct-v0-2-ft-action-v0-1-chat.enabled | bool | `false` |  |
| mistral-7b-instruct-v0-2-ft-action-v0-1-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| mistral-7b-instruct-v0-2-ft-thought-v0-0-chat.enabled | bool | `false` |  |
| mistral-7b-instruct-v0-2-ft-thought-v0-0-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| mistral-7b-instruct-v0-2-ft-thought-v0-1-chat.enabled | bool | `false` |  |
| mistral-7b-instruct-v0-2-ft-thought-v0-1-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| mixtral-8x7b-instruct-v0-1-chat.enabled | bool | `false` |  |
| mixtral-8x7b-instruct-v0-1-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| nl2rqs-7b-v3.enabled | bool | `false` |  |
| nl2rqs-7b-v3.lifecyclePhase | string | `"ACTIVE"` |  |
| nl2rqs-7b-v4.enabled | bool | `false` |  |
| nl2rqs-7b-v4.lifecyclePhase | string | `"ACTIVE"` |  |
| nl2sql-m1-base-7b.enabled | bool | `false` |  |
| nl2sql-m1-base-7b.lifecyclePhase | string | `"ACTIVE"` |  |
| osnamespace | string | `"mynamespace"` |  |
| phi-3-vision-128k-instruct-chat.enabled | bool | `false` |  |
| phi-3-vision-128k-instruct-chat.lifecyclePhase | string | `"ACTIVE"` |  |
| phi-3-vision-128k-instruct-ft-v1.enabled | bool | `false` |  |
| phi-3-vision-128k-instruct-ft-v1.lifecyclePhase | string | `"ACTIVE"` |  |


# models-importer

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart for importing ome base models

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| compartmentID | string | `"ocid1.compartment.oc1..aaaaaaaazq"` |  |
| llama-3-1-405b-instruct-fp8.enabled | bool | `false` |  |
| llama-3-1-405b-instruct-fp8.lifecyclePhase | string | `"ACTIVE"` |  |
| llama-3-1-70b-instruct.enabled | bool | `false` |  |
| llama-3-1-70b-instruct.lifecyclePhase | string | `"ACTIVE"` |  |
| osnamespace | string | `"mynamespace"` |  |
| phi-3-mini-128k-instruct.enabled | bool | `false` |  |
| phi-3-mini-128k-instruct.lifecyclePhase | string | `"ACTIVE"` |  |


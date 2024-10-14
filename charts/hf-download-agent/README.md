# hf-download-agent

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.16.0](https://img.shields.io/badge/AppVersion-1.16.0-informational?style=flat-square)

A Helm chart for Kubernetes

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| containerEnv.authType | string | `"InstancePrincipal"` |  |
| containerEnv.downloadCommit | string | `"main"` |  |
| containerEnv.hfToken | string | `"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"` |  |
| containerEnv.internalModelName | string | `"t5-small"` |  |
| containerEnv.modelName | string | `"t5-small"` |  |
| containerEnv.objectBucketName | string | `"model-store"` |  |
| containerEnv.objectCompartmentId | string | `"ocid1.compartment.oc1..aaaaaaaathgntpo75bdehisnl6wkxfc4slkd6rpheafbt5a6ekm2ri4bmeva"` |  |
| containerEnv.objectNamespace | string | `"idqj093njucb"` |  |
| containerEnv.region | string | `"ap-osaka-1"` |  |
| containerEnv.regionOverride | string | `"ap-osaka-1"` |  |
| containers.command[0] | string | `"/download-agent"` |  |
| containers.command[1] | string | `"download-agent"` |  |
| containers.command[2] | string | `"-c"` |  |
| containers.command[3] | string | `"/configs/hf-download-agent.yaml"` |  |
| containers.command[4] | string | `"-v"` |  |
| containers.command[5] | string | `"hf"` |  |
| fullnameOverride | string | `"hf-download-agent"` |  |
| image.pullPolicy | string | `"Always"` |  |
| image.repository | string | `"ap-osaka-1.ocir.io/idqj093njucb/download-agent"` |  |
| image.tag | float | `1` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podSecurityContext | object | `{}` |  |
| replicaCount | int | `1` |  |
| resources | object | `{}` |  |
| securityContext | object | `{}` |  |
| tolerations[0].effect | string | `"NoSchedule"` |  |
| tolerations[0].key | string | `"nvidia.com/gpu"` |  |
| tolerations[0].operator | string | `"Exists"` |  |


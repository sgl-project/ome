# generic-download-agent

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.0](https://img.shields.io/badge/AppVersion-0.0.0-informational?style=flat-square)

A Helm chart for model download agent

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key | string | `"nvidia.com/gpu.present"` |  |
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].operator | string | `"In"` |  |
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values[0] | string | `"true"` |  |
| containerEnv.authType | string | `"InstancePrincipal"` |  |
| containerEnv.enableSizeLimitCheck | string | `"true"` |  |
| containerEnv.modelName | string | `"model-name"` |  |
| containerEnv.numOfThreadsForReplication | string | `"8"` |  |
| containerEnv.region | string | `"us-chicago-1"` |  |
| containerEnv.sourceEnableOboToken | string | `"false"` |  |
| containerEnv.sourceObjectBucketName | string | `"model-store"` |  |
| containerEnv.sourceObjectCompartmentId | string | `"ocid1.compartment.oc1..aaaaaaaa2hdk363zzvlegei25gbspsps74cec467hgl3gocyrcsopcguh6pq"` |  |
| containerEnv.sourceObjectPrefix | string | `"model-prefix"` |  |
| containerEnv.sourceOboToken | string | `""` |  |
| containerEnv.sourceRegionOverride | string | `"ap-osaka-1"` |  |
| containerEnv.targetObjectBucketName | string | `"model-store"` |  |
| containerEnv.targetObjectCompartmentId | string | `"ocid1.compartment.oc1..aaaaaaaathgntpo75bdehisnl6wkxfc4slkd6rpheafbt5a6ekm2ri4bmeva"` |  |
| containerEnv.targetObjectPrefix | string | `"model-prefix"` |  |
| containers.command[0] | string | `"/download-agent"` |  |
| containers.command[1] | string | `"download-agent"` |  |
| containers.command[2] | string | `"-c"` |  |
| containers.command[3] | string | `"/configs/generic-download-agent.yaml"` |  |
| containers.command[4] | string | `"-v"` |  |
| containers.command[5] | string | `"generic"` |  |
| enableHooks | bool | `false` |  |
| fullnameOverride | string | `"generic-download-agent"` |  |
| image.pullPolicy | string | `"Always"` |  |
| image.repository | string | `"us-ashburn-1.ocir.io/idqj093njucb/download-agent"` |  |
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


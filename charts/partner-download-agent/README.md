# partner-download-agent

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.0](https://img.shields.io/badge/AppVersion-0.0.0-informational?style=flat-square)

A Helm chart for model download agent

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key | string | `"nvidia.com/gpu.present"` |  |
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].operator | string | `"In"` |  |
| affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values[0] | string | `"true"` |  |
| containerEnv.authType | string | `"OkeWorkloadIdentity"` |  |
| containerEnv.mode | string | `"ModelImporting"` |  |
| containerEnv.modelName | string | `"rerank-english-02"` |  |
| containerEnv.modelPathConfigJson | string | `"[]"` |  |
| containerEnv.region | string | `"us-chicago-1"` |  |
| containerEnv.sourceKeyName | string | `"rerank_english_02"` |  |
| containerEnv.sourceKeySecretCompartmentId | string | `"ocid1.compartment.oc1..aaaaaaaa2hdk363zzvlegei25gbspsps74cec467hgl3gocyrcsopcguh6pq"` |  |
| containerEnv.sourceKmsCryptoEndpoint | string | `"https://hvsmlw23aaf7a-crypto.kms.us-chicago-1.oci.oraclecloud.com"` |  |
| containerEnv.sourceKmsManagementEndpoint | string | `"https://hvsmlw23aaf7a-management.kms.us-chicago-1.oci.oraclecloud.com"` |  |
| containerEnv.sourceModelEncrypted | string | `"true"` |  |
| containerEnv.sourceObjectBucketName | string | `"genai-artifacts"` |  |
| containerEnv.sourceObjectCompartmentId | string | `"ocid1.compartment.oc1..aaaaaaaa2hdk363zzvlegei25gbspsps74cec467hgl3gocyrcsopcguh6pq"` |  |
| containerEnv.sourceObjectPrefix | string | `"rerank-english-02"` |  |
| containerEnv.sourceRegionOverride | string | `"us-chicago-1"` |  |
| containerEnv.sourceSecretName | string | `""` |  |
| containerEnv.sourceVaultId | string | `"ocid1.vault.oc1.us-chicago-1.hvsmlw23aaf7a.abxxeljtqjzeo2ou7o3p5xm2brc65oms2ax5jh7udtr3n6gs6bmysbybzreq"` |  |
| containerEnv.sourceVaultPrefix | string | `"hvsmlw23aaf7a"` |  |
| containerEnv.targetEnableKeyDefinedTag | string | `"false"` |  |
| containerEnv.targetKeyDefinedTagValue | string | `"dummy-tag-vault"` |  |
| containerEnv.targetKeyName | string | `"rerank"` |  |
| containerEnv.targetKeySecretCompartmentId | string | `"ocid1.compartment.oc1..aaaaaaaathgntpo75bdehisnl6wkxfc4slkd6rpheafbt5a6ekm2ri4bmeva"` |  |
| containerEnv.targetKmsCryptoEndpoint | string | `"https://ijsmzj42aabfe-crypto.kms.us-chicago-1.oci.oraclecloud.com"` |  |
| containerEnv.targetKmsManagementEndpoint | string | `"https://ijsmzj42aabfe-management.kms.us-chicago-1.oci.oraclecloud.com"` |  |
| containerEnv.targetObjectBucketName | string | `"partner-models"` |  |
| containerEnv.targetObjectCompartmentId | string | `"ocid1.compartment.oc1..aaaaaaaathgntpo75bdehisnl6wkxfc4slkd6rpheafbt5a6ekm2ri4bmeva"` |  |
| containerEnv.targetObjectPrefix | string | `"rerank-english-02"` |  |
| containerEnv.targetSecretName | string | `""` |  |
| containerEnv.targetVaultId | string | `"ocid1.vault.oc1.us-chicago-1.ijsmzj42aabfe.abxxeljty577dn2qiraqgjctad64pcetkxbwxnyjsdqubju4uq3hsvsceagq"` |  |
| containerEnv.targetVaultPrefix | string | `"ijsmzj42aabfe"` |  |
| containers.command[0] | string | `"/download-agent"` |  |
| containers.command[1] | string | `"download-agent"` |  |
| containers.command[2] | string | `"-c"` |  |
| containers.command[3] | string | `"/configs/partner-download-agent.yaml"` |  |
| fullnameOverride | string | `"partner-download-agent"` |  |
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
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.automountToken | bool | `true` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `"download-agent"` |  |
| tolerations[0].effect | string | `"NoSchedule"` |  |
| tolerations[0].key | string | `"nvidia.com/gpu"` |  |
| tolerations[0].operator | string | `"Exists"` |  |


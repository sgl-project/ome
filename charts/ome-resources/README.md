# ome-resources

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.16.0](https://img.shields.io/badge/AppVersion-1.16.0-informational?style=flat-square)

OME Resources and Controller

## Values

| Key                                                         | Type   | Default                                                          | Description |
|-------------------------------------------------------------|--------|------------------------------------------------------------------|-------------|
| modelAgent.health.port                                      | int    | `8080`                                                           |             |
| modelAgent.hostPath                                         | string | `"/mnt/data/models"`                                             |             |
| modelAgent.image.pullPolicy                                 | string | `"Always"`                                                       |             |
| modelAgent.image.repository                                 | string | `"ghcr.io/moirai-internal/model-agent"`                          |             |
| modelAgent.image.tag                                        | string | `"v0.1.0"`                                                       |             |
| modelAgent.nodeSelector                                     | object | `{}`                                                             |             |
| modelAgent.priorityClassName                                | string | `"system-node-critical"`                                         |             |
| modelAgent.resources.limits.cpu                             | string | `"10"`                                                           |             |
| modelAgent.resources.limits.memory                          | string | `"100Gi"`                                                        |             |
| modelAgent.resources.requests.cpu                           | string | `"10"`                                                           |             |
| modelAgent.resources.requests.memory                        | string | `"100Gi"`                                                        |             |
| modelAgent.serviceAccountName                               | string | `"ome-model-agent"`                                              |             |
| ome.benchmarkJob.cpuLimit                                   | string | `"2"`                                                            |             |
| ome.benchmarkJob.cpuRequest                                 | string | `"2"`                                                            |             |
| ome.benchmarkJob.image                                      | string | `"ghcr.io/moirai-internal/genai-bench"`                          |             |
| ome.benchmarkJob.memoryLimit                                | string | `"2Gi"`                                                          |             |
| ome.benchmarkJob.memoryRequest                              | string | `"2Gi"`                                                          |             |
| ome.benchmarkJob.tag                                        | string | `"0.1.113"`                                                      |             |
| ome.controller.affinity                                     | object | `{}`                                                             |             |
| ome.controller.deploymentMode                               | string | `"RawDeployment"`                                                |             |
| ome.controller.image                                        | string | `"ghcr.io/moirai-internal/ome/manager"`                          |             |
| ome.controller.ingressGateway.additionalIngressDomains      | string | `nil`                                                            |             |
| ome.controller.ingressGateway.disableIngressCreation        | bool   | `false`                                                          |             |
| ome.controller.ingressGateway.disableIstioVirtualHost       | bool   | `false`                                                          |             |
| ome.controller.ingressGateway.domain                        | string | `"svc.cluster.local"`                                            |             |
| ome.controller.ingressGateway.domainTemplate                | string | `"{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}"`            |             |
| ome.controller.ingressGateway.enableGatewayAPI              | bool   | `false`                                                          |             |
| ome.controller.ingressGateway.ingressGateway.className      | string | `"istio"`                                                        |             |
| ome.controller.ingressGateway.ingressGateway.gateway        | string | `"knative-serving/knative-ingress-gateway"`                      |             |
| ome.controller.ingressGateway.ingressGateway.gatewayService | string | `"istio-ingressgateway.istio-system.svc.cluster.local"`          |             |
| ome.controller.ingressGateway.localGateway.gateway          | string | `"knative-serving/knative-local-gateway"`                        |             |
| ome.controller.ingressGateway.localGateway.gatewayService   | string | `"knative-local-gateway.istio-system.svc.cluster.local"`         |             |
| ome.controller.ingressGateway.omeIngressGateway             | string | `""`                                                             |             |
| ome.controller.ingressGateway.pathTemplate                  | string | `""`                                                             |             |
| ome.controller.ingressGateway.urlScheme                     | string | `"http"`                                                         |             |
| ome.controller.nodeSelector                                 | object | `{}`                                                             |             |
| ome.controller.replicaCount                                 | int    | `3`                                                              |             |
| ome.controller.resources.limits.cpu                         | int    | `2`                                                              |             |
| ome.controller.resources.limits.memory                      | string | `"4Gi"`                                                          |             |
| ome.controller.resources.requests.cpu                       | int    | `2`                                                              |             |
| ome.controller.resources.requests.memory                    | string | `"4Gi"`                                                          |             |
| ome.controller.tag                                          | string | `"v0.1.0"`                                                       |             |
| ome.controller.tolerations                                  | list   | `[]`                                                             |             |
| ome.controller.topologySpreadConstraints                    | list   | `[]`                                                             |             |
| ome.kedaConfig.customPromQuery                              | string | `""`                                                             |             |
| ome.kedaConfig.enableKeda                                   | bool   | `true`                                                           |             |
| ome.kedaConfig.promServerAddress                            | string | `"http://prometheus-operated.monitoring.svc.cluster.local:9090"` |             |
| ome.kedaConfig.scalingOperator                              | string | `"GreaterThanOrEqual"`                                           |             |
| ome.kedaConfig.scalingThreshold                             | string | `"10"`                                                           |             |
| ome.metricsaggregator.enableMetricAggregation               | string | `"false"`                                                        |             |
| ome.metricsaggregator.enablePrometheusScraping              | string | `"false"`                                                        |             |
| ome.multinodeProber.cpuLimit                                | string | `"100m"`                                                         |             |
| ome.multinodeProber.cpuRequest                              | string | `"100m"`                                                         |             |
| ome.multinodeProber.image                                   | string | `"ghcr.io/moirai-internal/multinode-prober"`                     |             |
| ome.multinodeProber.memoryLimit                             | string | `"100Mi"`                                                        |             |
| ome.multinodeProber.memoryRequest                           | string | `"100Mi"`                                                        |             |
| ome.multinodeProber.startupFailureThreshold                 | int    | `150`                                                            |             |
| ome.multinodeProber.startupInitialDelaySeconds              | int    | `120`                                                            |             |
| ome.multinodeProber.startupPeriodSeconds                    | int    | `30`                                                             |             |
| ome.multinodeProber.startupTimeoutSeconds                   | int    | `60`                                                             |             |
| ome.multinodeProber.tag                                     | string | `"v0.1"`                                                         |             |
| ome.multinodeProber.unavailableThresholdSeconds             | int    | `600`                                                            |             |
| ome.omeAgent.authType                                       | string | `"InstancePrincipal"`                                            |             |
| ome.omeAgent.compartmentId                                  | string | `"ocid1.compartment.oc1..dummy-compartment"`                     |             |
| ome.omeAgent.fineTunedAdapter.cpuLimit                      | int    | `15`                                                             |             |
| ome.omeAgent.fineTunedAdapter.cpuRequest                    | int    | `15`                                                             |             |
| ome.omeAgent.fineTunedAdapter.memoryLimit                   | string | `"320Gi"`                                                        |             |
| ome.omeAgent.fineTunedAdapter.memoryRequest                 | string | `"300Gi"`                                                        |             |
| ome.omeAgent.image                                          | string | `"ghcr.io/moirai-internal/genai-ome-agent"`                      |             |
| ome.omeAgent.modelInit.cpuLimit                             | int    | `15`                                                             |             |
| ome.omeAgent.modelInit.cpuRequest                           | int    | `15`                                                             |             |
| ome.omeAgent.modelInit.memoryLimit                          | string | `"180Gi"`                                                        |             |
| ome.omeAgent.modelInit.memoryRequest                        | string | `"150Gi"`                                                        |             |
| ome.omeAgent.region                                         | string | `"ap-osaka-1"`                                                   |             |
| ome.omeAgent.tag                                            | string | `"v0.1.0"`                                                       |             |
| ome.omeAgent.vaultId                                        | string | `"ocid1.vault.oc1.ap-osaka-1.dummy.dummy-vault"`                 |             |
| ome.version                                                 | string | `"v0.1.0"`                                                       |             |


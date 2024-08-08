# ome-resources

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.16.0](https://img.shields.io/badge/AppVersion-1.16.0-informational?style=flat-square)

A Helm chart for Kubernetes

## Values

| Key                                                         | Type   | Default                                                                                 | Description |
|-------------------------------------------------------------|--------|-----------------------------------------------------------------------------------------|-------------|
| ome.controller.affinity                                     | object | `{}`                                                                                    |             |
| ome.controller.deploymentMode                               | string | `"RawDeployment"`                                                                       |             |
| ome.controller.image                                        | string | `"ord.ocir.io/idqj093njucb/ome/manager"`                                                |             |
| ome.controller.ingressGateway.disableIstioVirtualHost       | bool   | `false`                                                                                 |             |
| ome.controller.ingressGateway.domain                        | string | `"svc.cluster.local"`                                                                   |             |
| ome.controller.ingressGateway.domainTemplate                | string | `"{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}"`                                   |             |
| ome.controller.ingressGateway.ingressGateway.className      | string | `"istio"`                                                                               |             |
| ome.controller.ingressGateway.ingressGateway.gateway        | string | `"knative-serving/knative-ingress-gateway"`                                             |             |
| ome.controller.ingressGateway.ingressGateway.gatewayService | string | `"istio-ingressgateway.istio-system.svc.cluster.local"`                                 |             |
| ome.controller.ingressGateway.localGateway.gateway          | string | `"knative-serving/knative-local-gateway"`                                               |             |
| ome.controller.ingressGateway.localGateway.gatewayService   | string | `"knative-local-gateway.istio-system.svc.cluster.local"`                                |             |
| ome.controller.ingressGateway.urlScheme                     | string | `"http"`                                                                                |             |
| ome.controller.nodeSelector                                 | object | `{}`                                                                                    |             |
| ome.controller.rbacProxyImage                               | string | `"gcr.io/kubebuilder/kube-rbac-proxy:v0.13.1"`                                          |             |
| ome.controller.replicaCount                                 | int    | `3`                                                                                     |             |
| ome.controller.resources.limits.cpu                         | string | `"500m"`                                                                                |             |
| ome.controller.resources.limits.memory                      | string | `"2Gi"`                                                                                 |             |
| ome.controller.resources.requests.cpu                       | string | `"500m"`                                                                                |             |
| ome.controller.resources.requests.memory                    | string | `"2Gi"`                                                                                 |             |
| ome.controller.tag                                          | string | `"v0.0.1"`                                                                              |             |
| ome.controller.tolerations                                  | list   | `[]`                                                                                    |             |
| ome.controller.topologySpreadConstraints                    | list   | `[]`                                                                                    |             |
| ome.metricsaggregator.enableMetricAggregation               | string | `"false"`                                                                               |             |
| ome.metricsaggregator.enablePrometheusScraping              | string | `"false"`                                                                               |             |
| ome.ociETC.adNumberName                                     | string | `"ad2"`                                                                                 |             |
| ome.ociETC.airportCode                                      | string | `"ORD"`                                                                                 |             |
| ome.ociETC.applicationStage                                 | string | `"prod"`                                                                                |             |
| ome.ociETC.internalDomainName                               | string | `"oracleiaas.com"`                                                                      |             |
| ome.ociETC.namespace                                        | string | `"ax0pqskufyud"`                                                                        |             |
| ome.ociETC.publicDomainName                                 | string | `"oraclecloud.com"`                                                                     |             |
| ome.ociETC.realm                                            | string | `"oc1"`                                                                                 |             |
| ome.ociETC.region                                           | string | `"us-chicago-1"`                                                                        |             |
| ome.ociETC.serviceCompartmentId                             | string | `"ocid1.compartment.oc1..aaaaaaaal2wsx6r54lwfqa7frqad33ujlgo4azwd2uv6ftyqmvbqm6k5dxaa"` |             |
| ome.ociETC.serviceTenancyId                                 | string | `"ocid1.tenancy.oc1..aaaaaaaa6kumpbo73nsadhi7t4ssehtgn6wtnnkertn6sd74i4gzvia6misa"`     |             |
| ome.ociETC.stage                                            | string | `"dev"`                                                                                 |             |
| ome.servingruntime.vllmMedium.image                         | string | `"fra.ocir.io/idqj093njucb/official-vllm-openai"`                                       |             |
| ome.servingruntime.vllmMedium.tag                           | string | `"e3c664b"`                                                                             |             |
| ome.version                                                 | string | `"v0.0.1"`                                                                              |             |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.13.1](https://github.com/norwoodj/helm-docs/releases/v1.13.1)

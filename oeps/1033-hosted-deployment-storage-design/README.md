# Hosted Deployment Storage Resource Design
## Summary:
We need a consistent, Kubernetes-native way to provision and manage PostgreSQL databases and Redis (OCI Cache) users for GenAI Agent/MCP applications per customer tenancy.
This design introduces a set of namespaced Custom Resources (CRDs) and controllers that:
- Represent tenant-level storage clusters (PostgreSQL and Redis)
- Represent application-scoped storage instances (logical databases and Redis users)
- Encapsulate all provisioning logic against OCI PostgreSQL and OCI Cache
The end result: platform and application teams can describe their storage requirements declaratively with CRDs, and the controllers reconcile those specs into OCI resources and K8s Secrets.

## Motivation
Hosted deployments for Agents/MCPs need:
- Isolation between customer tenancies
- A standard way for CP/AgentRuntime/AgentDeployment to request storage without embedding cloud-specific APIs
- A mechanism to scale and evolve storage topologies (shared vs dedicated clusters per app) without changing application code
By centralizing storage management in CRDs and controllers, we: 
- Make storage topology declarative
- Allow per-tenant policies and defaults (shapes, versions, network) 
- Keep application configuration simple (consume Secrets with DATABASE_URL/Redis URL)

## Goals
- Tenant-level isolation by default
  1. Each customer tenancy default gets one shared PostgresSQL and Redis clusters
  2. Each customer tenancy could get more dedicated PostgreSQL and Redis clusters if dedicatedCluster is true
- Per-application storage objects
  1. PostgreSQL: one logical database + app user per application.
  2. Redis: one Redis user per application with a well-defined ACL string.
- Configurable defaults and override
  1. Platform define cluster defaults (shape, version, capacity).
  2. Application-level request with minimal fields
  3. Application request can override config
- Secret management
  1. Automatically generate and store credentials in K8s Secrets

## Non-Goals
- Implement data migration between clusters/tenants (e.g., moving app DBs between clusters).
- Handle schema management inside application databases

## Proposal
### User Stories
1. Application Owner: Request a logical PostgreSQL database in a shared cluster. I want to create an OciPostgresDBInstance under my tenant’s shared OciPostgresCluster, so I can receive a logical database, an app user, and a connection Secret without dealing with OCI APIs.
2. Application Owner: Request a logical PostgreSQL database in a dedicated cluster. want to create an OciPostgresCluster dedicated to my application and then create an OciPostgresDBInstance in that cluster, so my app receives isolated performance and storage.
3. Application Owner: Request a Redis user in a shared Redis cluster. I want to create an OciRedisUser in my tenant’s shared OciRedisCluster, so I get an app-scoped Redis user, ACL string, and credentials Secret without manually configuring OCI Cache.
4. Application Owner: Request a Redis user in a dedicated Redis cluster. I want to create an OciRedisCluster dedicated to my application and then create an OciRedisUser in that cluster, so my app has isolated Redis access while consuming credentials through a standard Secret.

## Technical Design
### High-level diagram
```
+-----------------------------------------+
|   Customer Tenancy NameSpace (TenantA)  |
+-----------------------------------------+
|  Postgres Cluster(TenantA-Shared)       |
|     - DB: appA                          |
|     - DB: appB                          |
| Postgres Cluster(TenantA-AppC-Dedicated)|
|     - DB: appC                          |
+-----------------------------------------+  
+-----------------------------------------+
|  Redis Cluster (TenantA-Shared)         |
|     - User: appA_user (ACL)             |
|     - User: appB_user (ACL)             |
|  Redis Cluster(TenantA-UserC-Dedicated) |
|     - User: appC_user (ACL)             |
+-----------------------------------------+

(K8s namespace: tenant-A-namespace)
+--> OciPostgresCluster    id:tenantA-shared-postgres
+--> OciPostgresDBInstance (per app)
+--> OciPostgresCluster    id:tenantA-appC-dedicated-postgres (dedicated for appC postgres)
+--> OciRedisCluster       id:tenantA-shared-redis
+--> OciRedisUser          (per app)
+--> OciRedisCluster       id:tenantA-UserC-dedicated-redis (dedicated for appC redis)

```
The storage topology is fully described and managed via K8s CRs and controllers.
### OCI PostgreSQL
Each application is assigned a separate database in this tenancy's storage cluster.
A cluster contains multiple Databases for applications in same customer tenancy.
Customer can choose to have a dedicated cluster for selected application.

### OCI Cache
Each application corresponds to a user with a global unique ACL string.
A cluster contains users of applications of same tenant.
Customer can choose to have dedicated cluster for certain application

## Custom Resources Definitions:
All CRs are namespaced and represent tenant-scoped resources.

### OciPostgresCluster
Represents a tenant-level PostgreSQL cluster (backed by an OCI DB System).
The OciPostgresCluster spec defines defaults config for infrastructure settings,for example:
- Default DB system shape
- Default db version
- Default Instance Count
- Default Instance CPU Count, etc
  Application teams can override these defaults per-cluster by explicitly setting fields in the spec

``` go
type OciPostgresClusterSpec struct {
    // +required
    CompartmentId string `json:"compartmentId"`

	// +required
	NetworkDetails DbSystemNetworkDetails `json:"networkDetails"`

	// +optional
	// AuthType selects how we authenticate to OCI.
	// +kubebuilder:validation:Enum=InstancePrincipal;UserPrincipal
	// +kubebuilder:default=InstancePrincipal
	AuthType string `json:"authType,omitempty"`

	// +optional
	// +kubebuilder:default="16"
	DbVersion string `json:"dbVersion"`

	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// +optional
	// +kubebuilder:default="PostgreSQL.VM.Standard.E5.Flex"
	Shape string `json:"shape"`

	// # of nodes (writer + replicas). Size must match InstancesDetails if provided.
	// +optional
	// +kubebuilder:default=1 -- VM instance
	InstanceCount int `json:"instanceCount,omitempty"`

	// per-node OCPU for Flex shapes.
	// +optional
	// +kubebuilder:default=2
	InstanceOcpuCount *int `json:"instanceOcpuCount,omitempty"`

	// per-node memory (GiB) for Flex shapes.
	// +optional
	// +kubebuilder:default=16
	InstanceMemorySizeInGbs *int `json:"instanceMemorySizeInGbs,omitempty"`

	// +optional
	StorageDetails DbSystemStorageDetails `json:"storageDetails"`

	// +optional
	Description string `json:"description,omitempty"`
}

type OciPostgresClusterStatus struct {
   // LifecycleState represents the current lifecycle state of the ocipostgres_cluster (e.g., READY, CREATING)
   // +optional
   LifecycleState       LifecycleState `json:"lifecycleState,omitempty"`
   DbSystemId           string         `json:"dbSystemId,omitempty"`
   Endpoint             string         `json:"endpoint,omitempty"`
   AdminSecretName      string         `json:"adminSecretName,omitempty"`
   AdminSecretNamespace string         `json:"adminSecretNamespace,omitempty"`
   // Conditions represent the latest available observations of an object's state
   // +optional
   // +listType=atomic
   Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```
### OciPostgresDBInstance
Represents a logical database + app user in a given OciPostgresCluster.
``` go
type OciPostgresDBInstanceSpec struct {
   // Reference to owning cluster ID
   DBClusterId string `json:"dbClusterId"`
   // +optional
   // AuthType selects how we authenticate to OCI.
   // +kubebuilder:validation:Enum=InstancePrincipal;UserPrincipal
   // +kubebuilder:default=InstancePrincipal
   AuthType string `json:"authType,omitempty"`
   // +required
   // AdminSecreteName used to fetch admin credential for the cluster to provision db instance
   AdminSecretName string `json:"adminSecretName"`
   // +required
   AdminSecretNamespace string `json:"adminSecretNamespace"`
}

type OciPostgresDBInstanceStatus struct {
   // IsReady indicates whether the storage is ready for use.
   // +required
   IsReady bool `json:"isReady"`
   // LifecycleState represents the current lifecycle state of the PostgreSQLStatus (e.g., READY, CREATING)
   // +optional
   LifecycleState         LifecycleState `json:"lifecycleState,omitempty"`
   Endpoint               string         `json:"endpoint,omitempty"`
   AppUserSecretName      string         `json:"appUserSecretName,omitempty"`
   AppUserSecretNamespace string         `json:"appUserSecretNamespace,omitempty"`
   DatabaseName           string         `json:"databaseName,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +optional
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```
### OciRedisCluster
Represents a tenant-level Redis/OCI Cache cluster.
The OciRedisCluster spec defines config defaults, for example:
- Default node count and node shape
- Default SoftwareVersion
- Default NodeMemoryInGBs
  Tenants/application owners can override any of these by specifying explicit values in the CR.
  When a field is omitted, the controller uses the default value from the spec so basic clusters can be created with minimal configuration.

``` go
type OciRedisClusterSpec struct {
   // +required
   CompartmentId string `json:"compartmentId"`

	// +optional
	// AuthType selects how we authenticate to OCI.
	// +kubebuilder:validation:Enum=InstancePrincipal;UserPrincipal
	// +kubebuilder:default=InstancePrincipal
	AuthType string `json:"authType,omitempty"`

	// +optional
	DisplayName string `json:"displayName"`

	// +required
	SubnetId string `json:"subnetId"`

	// +optional
	// +listType=atomic
	NsgIds []string `json:"nsgIds,omitempty"`

	// +optional
	// +kubebuilder:default=3
	NodeCount int `json:"nodeCount,omitempty"`

	// +optional
	// +kubebuilder:validation:Enum=V7_0_5;REDIS_7_0
	// +kubebuilder:default="V7_0_5"
	SoftwareVersion string `json:"softwareVersion,omitempty"`

	// +optional
	// +kubebuilder:default="16"
	NodeMemoryInGBs string `json:"nodeMemoryInGBs,omitempty"`
}

type OciRedisClusterStatus struct {
   // LifecycleState represents the current lifecycle state of the oci_redis_cluster (e.g., READY, CREATING)
   // +optional
   LifecycleState  LifecycleState `json:"lifecycleState,omitempty"`
   RedisClusterId  string         `json:"redisClusterId,omitempty"`
   SecretName      string         `json:"secretName,omitempty"`
   SecretNamespace string         `json:"secretNamespace,omitempty"`
   // Conditions represent the latest available observations of an object's state
   // +optional
   // +listType=atomic
   Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```
### OciRedisUser
Represents an application-level Redis user in a Redis cluster.
``` go
type OciRedisUserSpec struct {
   RedisClusterId string `json:"redisClusterId"`
   // +optional
   // AuthType selects how we authenticate to OCI.
   // +kubebuilder:validation:Enum=InstancePrincipal;UserPrincipal
   // +kubebuilder:default=InstancePrincipal
   AuthType string `json:"authType,omitempty"`
   
   // +required
   CompartmentId string `json:"compartmentId"`
}

// OciRedisUserStatus defines the observed state of OciRedisUser.
type OciRedisUserStatus struct {
   // CacheUserID is the identifier of the user in the underlying OCI Cache service,
   // +optional
   CacheUserID string `json:"cacheUserId,omitempty"`

	// ACLString is the effective ACL string applied to this user in the Redis cluster.
	// +optional
	ACLString string `json:"aclString,omitempty"`

	// UserSecretName is the name of the Secret that stores this user's credentials.
	// +optional
	UserSecretName string `json:"userSecretName,omitempty"`

	// UserSecretNamespace is the namespace where the credentials Secret is stored.
	// +optional
	UserSecretNamespace string `json:"userSecretNamespace,omitempty"`

	// Conditions represent the latest available observations of the resource's state.
	// Typical types:
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```
## Controller Design
### PostgresCluster Controller
1. Generate admin credential and store in a Kubernetes Secret
2. Provision OCI PostgreSQL DB System using:
    - spec.compartmentId
    - spec.networkDetails.subnetId
    - Shape / storage / backup config from spec
3. Reconcile current state: Create cluster if absent, Poll until DB System is Available.
4. Updates CR status with DbClusterId, EndPoint, adminSecreteName, adminSecretNamespace
### PostgresDBInstance Controller
1. Connects to target cluster using status.dbClusterId and admin credential.
2. Generate app-specific user credential and store in a Secret.
3. Create the logical database if it does not exist.
4. Create the application role/user and grant permissions:
    - LOGIN, CONNECT on the new database
    - R/W on existing tables/sequences in public schema
5. Update CR status with:cluster endpoint, DatabaseName, AppUserSecretName, AppUserSecretNamespace
### RedisCluster Controller
1. Provision OCI Cache (Redis cluster) for a tenant using:
    - spec.compartmentId
    - spec.subnetId
    - Capacity and node configuration
2. Manage high-level lifecycle: Create cluster if redisClusterId missing, Poll until Redis Cluster is Available.
3. Update CR status with: redisClusterId, primaryEndpoint
### RedisUser Controller
1. Create Redis user per application in a given Redis cluster.
2. Generate user credentials and store in Kubernetes Secret.
3. Build a global unique ACL string (e.g., key prefix per app) based on:
    - App ID / application name
    - spec.aclTemplate (e.g. ~appId:* +@read +@write)
4. Call OCI to:
    - Create / update cache user
    - Attach ACL string and password/token
    - Attach User to Redis Cluster
5. Update CR status with: aclString, userSecretName, userSecretNamespace
## Sample YAML
```yaml
apiVersion: ome.io/v1beta1
kind: OciPostgresCluster
metadata:
  name: tenantA-shared-postgres
  namespace: tenantA-namespace
spec:
  compartmentId: ocid1.tenancy.oc1..aaaaaaaasz6cicsgfbqh6tj3xahi4ozoescfz36bjm3kucc7lotk2oqep47q
  networkDetails:
    subnetId: ocid1.subnet.oc1.us-chicago-1.aaaaaaaapuo3dn645e5fw6y2u2vg4gi5qpnwtmmxfocgbm7zqyxjrumxyywq
  authType: UserPrincipal
 ```
```yaml
apiVersion: ome.io/v1beta1
kind: OciPostgresDBInstance
metadata:
  name: 6cicsgfbqh6tj3xahi4ozoescfz36btest
  namespace: tenantA-namespace
spec:
  dbClusterId: ocid1.postgresqldbsystem.oc1.us-chicago-1.amaaaaaacqy6p4qa2pnnd45gh6rdm45n6pafsahepodgcf6o7s7thl3nisha
  adminSecretName: pg-admin
  adminSecretNamespace: demo-cluster-v3
  authType: UserPrincipal
```
```yaml
apiVersion: ome.io/v1beta1
kind: OciRedisCluster
metadata:
  name: tenantA-shared-redis
  namespace: tenantA-namespace
spec:
  compartmentId: ocid1.tenancy.oc1..aaaaaaaasz6cicsgfbqh6tj3xahi4ozoescfz36bjm3kucc7lotk2oqep47q
  subnetId: ocid1.subnet.oc1.us-chicago-1.aaaaaaaapuo3dn645e5fw6y2u2vg4gi5qpnwtmmxfocgbm7zqyxjrumxyywq
  authType: UserPrincipal
```
```yaml
apiVersion: ome.oracle.com/v1beta1
kind: OciRedisUser
metadata:
  name: app-a-redis-user
  namespace: tenantA-namespace
spec:
  redisClusterId: ocid1.redis.oc1.us-chicago-1.amaaaaaacqy6p4qa2pnnd45gh6rdm45n6pafsahepodgcf6o7s7thl3nisha
  authType: UserPrincipal
```
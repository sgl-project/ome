# Dedicated AI Cluster (DAC) CRD to support Capacity Reservations and Team Management

## Overview
The Dedicated AI Cluster (DAC) is a custom resource that represents a dedicated AI cluster within a Kubernetes environment. 
Each Kubernetes cluster can have multiple DACs, and each DAC can manage its own reserved capacity (CPU, GPU, memory, disk). 
DACs are designed to provide fine-grained control over resource allocation across multiple teams, ensuring optimal resource management and flexibility. 
The DAC also supports resource borrowing between teams and can be simplified via cluster profiles for admins.
### Motivation
1. Existing GenAI portfolio has both Dedicated AI Cluster (DAC) and Capacity Reservation (CR). 
The goal of this design is to merge the functionality of DAC and CR that supports both cluster-level capacity reservations and team management.
2. This design introduces additional potentialities for DACs, such as creating a dedicated vCluster for each DAC, 
which can be used to further isolate resources and provide better performance for AI workloads.

## Key Concepts

- ClusterProfile: A reference to a template that simplifies DAC creation for admins.
- ClusterResources: Overall resources reserved for the DAC at the cluster level.
- Teams: Logical groups within the DAC that manage their own resources.
- Borrowing: Allows teams to borrow resources from other teams within the DAC.
- Members: Individuals associated with a team, responsible for managing resources.
- Conditions: Health and operational state of the DAC (e.g., Ready, MemoryPressure).

## Design Details
The Dedicated AI Cluster is initially implemented as a namespace-based construct within Kubernetes. 
The long-term goal is to evolve the DAC to be a vCluster (virtual Kubernetes cluster), 
but for the MVP (Minimum Viable Product), the DAC will be implemented as a namespace. 
Each DAC will have its own resource management system by leveraging Kueue’s ClusterQueue and LocalQueue concepts for each team. 
The operator will manage the merging of ClusterProfile specs with DAC specs.

- Dedicated Namespace: Each DAC is represented as a Kubernetes namespace in the initial implementation. All workloads, policies, and resources for each DAC are scoped to this namespace.
- vCluster Transition: In future iterations, the DAC will be evolved into a vCluster, allowing for stricter isolation and greater autonomy, but namespaces will be the primary mechanism for MVP.
- ClusterQueue: Each DAC will implicitly create a ClusterQueue if not specified (managed by Kueue) for resource management at the DAC level. 
This ClusterQueue will manage the global resources (CPU, memory, GPU) for the DAC and act as a resource pool for all teams within the DAC.
- LocalQueue: Each team within a DAC will be represented by a LocalQueue. 
The LocalQueue will be a child of the ClusterQueue and will handle resource requests for that specific team, ensuring that teams have their own reserved capacity.




## CRD Specification

### DedicatedAIClusterSpec

This section defines the desired state of the DAC.

```go
type DedicatedAIClusterSpec struct {
    // ClusterProfile is a reference to a template that simplifies the cluster creation process for admins.
    // +optional
    ClusterProfile string `json:"clusterProfile,omitempty"`

    // CompartmentID specifies the compartment in which the DAC is created.
    // +optional
    CompartmentID string `json:"compartmentID,omitempty"`

    // Reservation allows referencing an existing ClusterQueue for resource management.
    // If not specified, a new ClusterQueue will be created.
    // +optional
    Reservation string `json:"reservation,omitempty"`

    // ClusterResources defines the overall resources reserved for the DAC at the cluster level.
    // +required
    ClusterResources corev1.ResourceRequirements `json:"clusterResources"`

    // Teams defines the various teams using the DAC, each with its own reserved capacity.
    // +required
    Teams []TeamSpec `json:"teams"`

    // Affinity defines node affinity for scheduling DAC workloads.
    // +optional
    Affinity *corev1.Affinity `json:"affinity,omitempty"`

    // NodeSelector defines the node labels for scheduling DAC workloads.
    // +optional
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`

    // Tolerations for taints to allow DAC scheduling on specific nodes.
    // +optional
    Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}
```
- ClusterProfile: Simplifies cluster creation by referencing a predefined profile.
- CompartmentID: Used to place the DAC within a specific compartment (for OCI environments).
- ClusterResources: Defines the overall resources (CPU, memory, GPU, disk) reserved for the entire DAC.
- Teams: Each DAC can have multiple teams with their own resource quotas and member information.
- Affinity, NodeSelector, Tolerations: Control where workloads from the DAC can be scheduled within the Kubernetes cluster.


### TeamSpec

This section defines the desired state of a team within the DAC.

```go
type TeamSpec struct {
    // Name of the team (e.g., Biology, Chemistry)
    // +required
    Name string `json:"name"`

    // Resources reserved specifically for this team.
    // +required
    Resources corev1.ResourceRequirements `json:"resources"`

    // AllowBorrowing defines if this team can borrow resources from others.
    // +optional
    AllowBorrowing bool `json:"allowBorrowing,omitempty"`

    // BorrowableResources defines how much capacity this team is willing to lend.
    // +optional
    BorrowableResources corev1.ResourceList `json:"borrowableResources,omitempty"`

    // Members involved in the team.
    // +optional
    Members []MemberSpec `json:"members,omitempty"`
}
```
- Name: The name of the team (e.g., Biology Department).
- Resources: Reserved resources for the team, using Kubernetes ResourceRequirements.
- AllowBorrowing: Allows teams to borrow resources if enabled.
- Members: Defines members within each team.

### MemberSpec
Defines the individual members (e.g., researchers, engineers) within each team.
```go
type MemberSpec struct {
    // Name of the member.
    // +required
    Name string `json:"name"`

    // Email of the member.
    // +optional
    Email string `json:"email"`

    // Role of the member (e.g., Professor, Engineer).
    // +optional
    Role string `json:"role,omitempty"`
}
```

- Name: The member’s name.
- Email: The member’s contact information.
- Role: The role of the member within the team.

### Status Specification
```go
type DedicatedAIClusterStatus struct {
    // Capacity represents the total resources available in the DAC (cluster-level resources).
    // +optional
    Capacity corev1.ResourceList `json:"capacity,omitempty"`

    // Allocatable represents the resources that are available for scheduling.
    // +optional
    Allocatable corev1.ResourceList `json:"allocatable,omitempty"`

    // ClusterResourceUsage tracks the current resource usage for the DAC at the cluster level.
    // +optional
    ClusterResourceUsage corev1.ResourceList `json:"clusterResourceUsage,omitempty"`

    // TeamUsages tracks the resource usage for each team in the DAC.
    // +optional
    TeamUsages []TeamUsage `json:"teamUsages,omitempty"`

    // ClusterQueueName tracks the name of the created ClusterQueue for this DAC.
    // +optional
    ClusterQueueName string `json:"clusterQueueName,omitempty"`

    // Conditions is an array of current observed conditions for the DAC.
    // +optional
    Conditions []DedicatedAIClusterCondition `json:"conditions,omitempty"`

    // LifecycleState indicates the current phase of the Dedicated AI Cluster (e.g., "active", "creating", "Failed" etc.).
    LifecycleState DacLifecycleState `json:"dacLifecycleState,omitempty"`

    // DedicatedAIClusterInfo provides metadata information about the DAC.
    // +optional
    DedicatedAIClusterInfo DedicatedAIClusterSystemInfo `json:"dedicatedAIClusterInfo,omitempty"`
}
```

- Capacity: Total resources available in the DAC.
- Allocatable: Resources available for scheduling after system-reserved resources are deducted.
- ClusterResourceUsage: Tracks resource usage across the DAC.
- TeamUsages: Resource usage specific to each team.
- Conditions: Provides insight into the state of the DAC (e.g., Ready, MemoryPressure).
- DedicatedAIClusterInfo: Includes:
  - ProviderID: The OCI OCID for tracking the DAC in cloud environments.
  - KubernetesVersion: The version of Kubernetes running on the DAC.

### DedicatedAIClusterCondition
Defines the current state of the DAC, similar to Kubernetes NodeCondition.
```go
type DedicatedAIClusterCondition struct {
    // Type of condition.
    // +required
    Type DedicatedAIClusterConditionType `json:"type"`

    // Status of the condition.
    // +required
    Status corev1.ConditionStatus `json:"status"`

    // LastTransitionTime is the timestamp when the condition last changed.
    // +required
    LastTransitionTime metav1.Time `json:"lastTransitionTime"`

    // Reason for the condition's last transition.
    // +required
    Reason string `json:"reason"`

    // Message is a human-readable message indicating details about the condition.
    // +required
    Message string `json:"message"`
}
```

### Sample YAML
```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: biology-dac
spec:
  clusterProfile: default-cluster-profile
  reservation: existing-reservation
  compartmentID: comp-1234
  clusterResources:
    requests:
      cpu: "32"
      memory: "128Gi"
    limits:
      cpu: "64"
      memory: "256Gi"
  teams:
    - name: Biology Department
      resources:
        requests:
          cpu: "16"
          memory: "64Gi"
        limits:
          cpu: "32"
          memory: "128Gi"
      members:
        - name: Dr. Jane Doe
          email: jane.doe@university.edu
          role: Professor
    - name: Chemistry Department
      resources:
        requests:
          cpu: "8"
          memory: "32Gi"
        limits:
          cpu: "16"
          memory: "64Gi"
      allowBorrowing: true
      borrowableResources:
        cpu: "4"
        memory: "16Gi"
  allowBorrowing: true
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: "gpu"
                operator: In
                values:
                  - "true"
  nodeSelector:
    dedicated-ai: "true"
  tolerations:
    - key: "dedicated-ai"
      operator: "Equal"
      value: "true"
      effect: "NoSchedule"
 ```
---
title: OME API
content_type: tool-reference
package: ome.io/v1beta1
auto_generated: true
description: Generated API reference documentation for ome.io/v1beta1.
weight: 1
---
<p>Package v1beta1 contains API Schema definitions for the serving v1beta1 API group</p>


## Resource Types 


- [BaseModel](#ome-io-v1beta1-BaseModel)
- [BenchmarkJob](#ome-io-v1beta1-BenchmarkJob)
- [CapacityReservation](#ome-io-v1beta1-CapacityReservation)
- [CapacityReservationList](#ome-io-v1beta1-CapacityReservationList)
- [ClusterBaseModel](#ome-io-v1beta1-ClusterBaseModel)
- [ClusterCapacityReservation](#ome-io-v1beta1-ClusterCapacityReservation)
- [ClusterCapacityReservationList](#ome-io-v1beta1-ClusterCapacityReservationList)
- [ClusterServingRuntime](#ome-io-v1beta1-ClusterServingRuntime)
- [ClusterTrainingRuntime](#ome-io-v1beta1-ClusterTrainingRuntime)
- [DedicatedAICluster](#ome-io-v1beta1-DedicatedAICluster)
- [DedicatedAIClusterProfile](#ome-io-v1beta1-DedicatedAIClusterProfile)
- [FineTunedWeight](#ome-io-v1beta1-FineTunedWeight)
- [InferenceGraph](#ome-io-v1beta1-InferenceGraph)
- [InferenceService](#ome-io-v1beta1-InferenceService)
- [Organization](#ome-io-v1beta1-Organization)
- [Project](#ome-io-v1beta1-Project)
- [RateLimit](#ome-io-v1beta1-RateLimit)
- [ReplicationJob](#ome-io-v1beta1-ReplicationJob)
- [ReplicationJobList](#ome-io-v1beta1-ReplicationJobList)
- [ServiceAccount](#ome-io-v1beta1-ServiceAccount)
- [ServingRuntime](#ome-io-v1beta1-ServingRuntime)
- [TrainingJob](#ome-io-v1beta1-TrainingJob)
- [TrainingRuntime](#ome-io-v1beta1-TrainingRuntime)
- [TrainingRuntimeList](#ome-io-v1beta1-TrainingRuntimeList)
- [User](#ome-io-v1beta1-User)
  

## `BaseModel`     {#ome-io-v1beta1-BaseModel}
    

**Appears in:**



<p>BaseModel is the Schema for the basemodels API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>BaseModel</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-BaseModelSpec"><code>BaseModelSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelStatusSpec"><code>ModelStatusSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `BenchmarkJob`     {#ome-io-v1beta1-BenchmarkJob}
    

**Appears in:**



<p>BenchmarkJob is the schema for the BenchmarkJobs API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>BenchmarkJob</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-BenchmarkJobSpec"><code>BenchmarkJobSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-BenchmarkJobStatus"><code>BenchmarkJobStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `CapacityReservation`     {#ome-io-v1beta1-CapacityReservation}
    

**Appears in:**

- [CapacityReservationList](#ome-io-v1beta1-CapacityReservationList)


<p>CapacityReservation is the Schema for the capacityReservations API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>CapacityReservation</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CapacityReservationSpec"><code>CapacityReservationSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CapacityReservationStatus"><code>CapacityReservationStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `CapacityReservationList`     {#ome-io-v1beta1-CapacityReservationList}
    


<p>CapacityReservationList contains a list of CapacityReservation.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>CapacityReservationList</code></td></tr>
    
  
<tr><td><code>metadata</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>items</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CapacityReservation"><code>[]CapacityReservation</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ClusterBaseModel`     {#ome-io-v1beta1-ClusterBaseModel}
    

**Appears in:**



<p>ClusterBaseModel is the Schema for the basemodels API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ClusterBaseModel</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-BaseModelSpec"><code>BaseModelSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelStatusSpec"><code>ModelStatusSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ClusterCapacityReservation`     {#ome-io-v1beta1-ClusterCapacityReservation}
    

**Appears in:**

- [ClusterCapacityReservationList](#ome-io-v1beta1-ClusterCapacityReservationList)


<p>ClusterCapacityReservation is the Schema for the capacityReservations API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ClusterCapacityReservation</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CapacityReservationSpec"><code>CapacityReservationSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CapacityReservationStatus"><code>CapacityReservationStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ClusterCapacityReservationList`     {#ome-io-v1beta1-ClusterCapacityReservationList}
    


<p>ClusterCapacityReservationList contains a list of ClusterCapacityReservation.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ClusterCapacityReservationList</code></td></tr>
    
  
<tr><td><code>metadata</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>items</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ClusterCapacityReservation"><code>[]ClusterCapacityReservation</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ClusterServingRuntime`     {#ome-io-v1beta1-ClusterServingRuntime}
    

**Appears in:**



<p>ClusterServingRuntime is the Schema for the servingruntimes API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ClusterServingRuntime</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ServingRuntimeSpec"><code>ServingRuntimeSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ServingRuntimeStatus"><code>ServingRuntimeStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ClusterTrainingRuntime`     {#ome-io-v1beta1-ClusterTrainingRuntime}
    

**Appears in:**



<p>ClusterTrainingRuntime is the Schema for the TrainingRuntimes API in cluster scope</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ClusterTrainingRuntime</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TrainingRuntimeSpec"><code>TrainingRuntimeSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `DedicatedAICluster`     {#ome-io-v1beta1-DedicatedAICluster}
    

**Appears in:**



<p>DedicatedAICluster is the Schema for the dedicatedaiclusters API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>DedicatedAICluster</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-DedicatedAIClusterSpec"><code>DedicatedAIClusterSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-DedicatedAIClusterStatus"><code>DedicatedAIClusterStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `DedicatedAIClusterProfile`     {#ome-io-v1beta1-DedicatedAIClusterProfile}
    

**Appears in:**



<p>DedicatedAIClusterProfile is the Schema for the dedicatedaiclusterprofiles API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>DedicatedAIClusterProfile</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-DedicatedAIClusterProfileSpec"><code>DedicatedAIClusterProfileSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-DedicatedAIClusterProfileStatus"><code>DedicatedAIClusterProfileStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `FineTunedWeight`     {#ome-io-v1beta1-FineTunedWeight}
    

**Appears in:**



<p>FineTunedWeight is the Schema for the finetunedweights API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>FineTunedWeight</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-FineTunedWeightSpec"><code>FineTunedWeightSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelStatusSpec"><code>ModelStatusSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `InferenceGraph`     {#ome-io-v1beta1-InferenceGraph}
    

**Appears in:**



<p>InferenceGraph is the Schema for the InferenceGraph API for multiple models</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>InferenceGraph</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-InferenceGraphSpec"><code>InferenceGraphSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-InferenceGraphStatus"><code>InferenceGraphStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `InferenceService`     {#ome-io-v1beta1-InferenceService}
    

**Appears in:**



<p>InferenceService is the Schema for the InferenceServices API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>InferenceService</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-InferenceServiceSpec"><code>InferenceServiceSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-InferenceServiceStatus"><code>InferenceServiceStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `Organization`     {#ome-io-v1beta1-Organization}
    

**Appears in:**



<p>Organization represents an AI platform organization configuration</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>Organization</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-OrganizationSpec"><code>OrganizationSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-OrganizationStatus"><code>OrganizationStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `Project`     {#ome-io-v1beta1-Project}
    

**Appears in:**



<p>Project represents an AI platform project</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>Project</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ProjectSpec"><code>ProjectSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ProjectStatus"><code>ProjectStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `RateLimit`     {#ome-io-v1beta1-RateLimit}
    

**Appears in:**



<p>RateLimit represents rate limit configurations for a project</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>RateLimit</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-RateLimitSpec"><code>RateLimitSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-RateLimitStatus"><code>RateLimitStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ReplicationJob`     {#ome-io-v1beta1-ReplicationJob}
    

**Appears in:**

- [ReplicationJobList](#ome-io-v1beta1-ReplicationJobList)


<p>ReplicationJob is the Schema for the replicationjobs API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ReplicationJob</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ReplicationJobSpec"><code>ReplicationJobSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ReplicationJobStatus"><code>ReplicationJobStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ReplicationJobList`     {#ome-io-v1beta1-ReplicationJobList}
    


<p>ReplicationJobList contains a list of ReplicationJob</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ReplicationJobList</code></td></tr>
    
  
<tr><td><code>metadata</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>items</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ReplicationJob"><code>[]ReplicationJob</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ServiceAccount`     {#ome-io-v1beta1-ServiceAccount}
    

**Appears in:**



<p>ServiceAccount represents a service account within a project</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ServiceAccount</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ServiceAccountSpec"><code>ServiceAccountSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ServiceAccountStatus"><code>ServiceAccountStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ServingRuntime`     {#ome-io-v1beta1-ServingRuntime}
    

**Appears in:**



<p>ServingRuntime is the Schema for the servingruntimes API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>ServingRuntime</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ServingRuntimeSpec"><code>ServingRuntimeSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ServingRuntimeStatus"><code>ServingRuntimeStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `TrainingJob`     {#ome-io-v1beta1-TrainingJob}
    

**Appears in:**



<p>TrainingJob is the Schema for the TrainingJobs API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>TrainingJob</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TrainingJobSpec"><code>TrainingJobSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TrainingJobStatus"><code>TrainingJobStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `TrainingRuntime`     {#ome-io-v1beta1-TrainingRuntime}
    

**Appears in:**

- [TrainingRuntimeList](#ome-io-v1beta1-TrainingRuntimeList)


<p>TrainingRuntime is the Schema for the TrainingRuntimes API</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>TrainingRuntime</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TrainingRuntimeSpec"><code>TrainingRuntimeSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `TrainingRuntimeList`     {#ome-io-v1beta1-TrainingRuntimeList}
    


<p>TrainingRuntimeList contains a list of TrainingRuntime</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>TrainingRuntimeList</code></td></tr>
    
  
<tr><td><code>metadata</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>items</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TrainingRuntime"><code>[]TrainingRuntime</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `User`     {#ome-io-v1beta1-User}
    

**Appears in:**



<p>User represents a user within a project</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody><tr><td><code>apiVersion</code><br/>string</td><td><code>ome.io/v1beta1</code></td></tr>
<tr><td><code>kind</code><br/>string</td><td><code>User</code></td></tr>
    
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-UserSpec"><code>UserSpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-UserStatus"><code>UserStatus</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `APIKeySpec`     {#ome-io-v1beta1-APIKeySpec}
    

**Appears in:**

- [ServiceAccountStatus](#ome-io-v1beta1-ServiceAccountStatus)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name is the API key name</p>
</td>
</tr>
<tr><td><code>apiKeyId</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>APIKeyId is the platform-specific API key ID</p>
</td>
</tr>
<tr><td><code>apiKeySecretRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-SecretReference"><code>SecretReference</code></a>
</td>
<td>
   <p>APIKeySecretRef references the secret containing the API key</p>
</td>
</tr>
</tbody>
</table>

## `AssociationUsage`     {#ome-io-v1beta1-AssociationUsage}
    

**Appears in:**

- [CapacityReservationStatus](#ome-io-v1beta1-CapacityReservationStatus)


<p>AssociationUsage defines the usage of the association.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the association.</p>
</td>
</tr>
<tr><td><code>usage</code> <B>[Required]</B><br/>
<code>[]sigs.k8s.io/kueue/apis/kueue/v1beta1.FlavorUsage</code>
</td>
<td>
   <p>Usage of the association.</p>
</td>
</tr>
</tbody>
</table>

## `BaseModelSpec`     {#ome-io-v1beta1-BaseModelSpec}
    

**Appears in:**

- [BaseModel](#ome-io-v1beta1-BaseModel)

- [ClusterBaseModel](#ome-io-v1beta1-ClusterBaseModel)


<p>BaseModelSpec defines the desired state of BaseModel</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>modelFormat</code><br/>
<a href="#ome-io-v1beta1-ModelFormat"><code>ModelFormat</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>modelType</code><br/>
<code>string</code>
</td>
<td>
   <p>ModelType defines the architecture family of the model (e.g., &quot;bert&quot;, &quot;gpt2&quot;, &quot;llama&quot;).
This value typically corresponds to the &quot;model_type&quot; field in a Hugging Face model's config.json.
It is used to identify the transformer architecture and inform runtime selection and tokenizer behavior.</p>
</td>
</tr>
<tr><td><code>modelFramework</code><br/>
<a href="#ome-io-v1beta1-ModelFrameworkSpec"><code>ModelFrameworkSpec</code></a>
</td>
<td>
   <p>ModelFramework specifies the underlying framework used by the model,
such as &quot;ONNX&quot;, &quot;TensorFlow&quot;, &quot;PyTorch&quot;, &quot;Transformer&quot;, or &quot;TensorRTLLM&quot;.
This value helps determine the appropriate runtime for model serving.</p>
</td>
</tr>
<tr><td><code>modelArchitecture</code><br/>
<code>string</code>
</td>
<td>
   <p>ModelArchitecture specifies the concrete model implementation or head,
such as &quot;LlamaForCausalLM&quot;, &quot;GemmaForCausalLM&quot;, or &quot;MixtralForCausalLM&quot;.
This is often derived from the &quot;architectures&quot; field in Hugging Face config.json.</p>
</td>
</tr>
<tr><td><code>quantization</code><br/>
<a href="#ome-io-v1beta1-ModelQuantization"><code>ModelQuantization</code></a>
</td>
<td>
   <p>Quantization defines the quantization scheme applied to the model weights,
such as &quot;fp8&quot;, &quot;fbgemm_fp8&quot;, or &quot;int4&quot;. This influences runtime compatibility and performance.</p>
</td>
</tr>
<tr><td><code>modelParameterSize</code><br/>
<code>string</code>
</td>
<td>
   <p>ModelParameterSize indicates the total number of parameters in the model,
expressed in human-readable form such as &quot;7B&quot;, &quot;13B&quot;, or &quot;175B&quot;.
This can be used for scheduling or runtime selection.</p>
</td>
</tr>
<tr><td><code>modelCapabilities</code><br/>
<code>[]string</code>
</td>
<td>
   <p>ModelCapabilities of the model, e.g., &quot;TEXT_GENERATION&quot;, &quot;TEXT_SUMMARIZATION&quot;, &quot;TEXT_EMBEDDINGS&quot;</p>
</td>
</tr>
<tr><td><code>modelConfiguration</code><br/>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#RawExtension"><code>k8s.io/apimachinery/pkg/runtime.RawExtension</code></a>
</td>
<td>
   <p>Configuration of the model, stored as generic JSON for flexibility.</p>
</td>
</tr>
<tr><td><code>storage</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>Storage configuration for the model</p>
</td>
</tr>
<tr><td><code>ModelExtensionSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelExtensionSpec"><code>ModelExtensionSpec</code></a>
</td>
<td>(Members of <code>ModelExtensionSpec</code> are embedded into this type.)
   <p>ModelExtension is the common extension of the model</p>
</td>
</tr>
<tr><td><code>servingMode</code> <B>[Required]</B><br/>
<code>[]string</code>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>maxTokens</code><br/>
<code>int32</code>
</td>
<td>
   <p>MaxTokens is the maximum number of tokens that can be processed by the model</p>
</td>
</tr>
<tr><td><code>additionalMetadata</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Additional metadata for the model</p>
</td>
</tr>
</tbody>
</table>

## `BenchmarkJobSpec`     {#ome-io-v1beta1-BenchmarkJobSpec}
    

**Appears in:**

- [BenchmarkJob](#ome-io-v1beta1-BenchmarkJob)


<p>BenchmarkJobSpec defines the specification for a benchmark job.
All fields within this specification collectively represent the desired
state and configuration of a BenchmarkJob.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>huggingFaceSecretReference</code><br/>
<a href="#ome-io-v1beta1-HuggingFaceSecretReference"><code>HuggingFaceSecretReference</code></a>
</td>
<td>
   <p>HuggingFaceSecretReference is a reference to a Kubernetes Secret containing the Hugging Face API key.
The referenced Secret must reside in the same namespace as the BenchmarkJob.
This field replaces the raw HuggingFaceAPIKey field for improved security.</p>
</td>
</tr>
<tr><td><code>endpoint</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-EndpointSpec"><code>EndpointSpec</code></a>
</td>
<td>
   <p>Endpoint is the reference to the inference service to benchmark.</p>
</td>
</tr>
<tr><td><code>serviceMetadata</code><br/>
<a href="#ome-io-v1beta1-ServiceMetadata"><code>ServiceMetadata</code></a>
</td>
<td>
   <p>ServiceMetadata records metadata about the backend model server or service being benchmarked.
This includes details such as server engine, version, and GPU configuration for filtering experiments.</p>
</td>
</tr>
<tr><td><code>task</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Task specifies the task to benchmark, pattern: <!-- raw HTML omitted -->-to-<!-- raw HTML omitted --> (e.g., &quot;text-to-text&quot;, &quot;image-to-text&quot;).</p>
</td>
</tr>
<tr><td><code>trafficScenarios</code><br/>
<code>[]string</code>
</td>
<td>
   <p>TrafficScenarios contains a list of traffic scenarios to simulate during the benchmark.
If not provided, defaults will be assigned via genai-bench.</p>
</td>
</tr>
<tr><td><code>numConcurrency</code><br/>
<code>[]int</code>
</td>
<td>
   <p>NumConcurrency defines a list of concurrency levels to test during the benchmark.
If not provided, defaults will be assigned via genai-bench.</p>
</td>
</tr>
<tr><td><code>maxTimePerIteration</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>MaxTimePerIteration specifies the maximum time (in minutes) for a single iteration.
Each iteration runs for a specific combination of TrafficScenarios and NumConcurrency.</p>
</td>
</tr>
<tr><td><code>maxRequestsPerIteration</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>MaxRequestsPerIteration specifies the maximum number of requests for a single iteration.
Each iteration runs for a specific combination of TrafficScenarios and NumConcurrency.</p>
</td>
</tr>
<tr><td><code>additionalRequestParams</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>AdditionalRequestParams contains additional request parameters as a map.</p>
</td>
</tr>
<tr><td><code>dataset</code><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>Dataset is the dataset used for benchmarking.
It is optional and only required for tasks other than &quot;text-to-<!-- raw HTML omitted -->&quot;.</p>
</td>
</tr>
<tr><td><code>outputLocation</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>OutputLocation specifies where the benchmark results will be stored (e.g., object storage).</p>
</td>
</tr>
<tr><td><code>resultFolderName</code><br/>
<code>string</code>
</td>
<td>
   <p>ResultFolderName specifies the name of the folder that stores the benchmark result. A default name will be assigned if not specified.</p>
</td>
</tr>
<tr><td><code>podOverride</code><br/>
<a href="#ome-io-v1beta1-PodOverride"><code>PodOverride</code></a>
</td>
<td>
   <p>Pod defines the pod configuration for the benchmark job. This is optional, if not provided, default values will be used.</p>
</td>
</tr>
</tbody>
</table>

## `BenchmarkJobStatus`     {#ome-io-v1beta1-BenchmarkJobStatus}
    

**Appears in:**

- [BenchmarkJob](#ome-io-v1beta1-BenchmarkJob)


<p>BenchmarkJobStatus reflects the state and results of the benchmark job. It
will be set and updated by the controller.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>state</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>State represents the current state of the benchmark job: &quot;Pending&quot;, &quot;Running&quot;, &quot;Completed&quot;, &quot;Failed&quot;.</p>
</td>
</tr>
<tr><td><code>startTime</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>StartTime is the timestamp for when the benchmark job started.</p>
</td>
</tr>
<tr><td><code>completionTime</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>CompletionTime is the timestamp for when the benchmark job completed, either successfully or unsuccessfully.</p>
</td>
</tr>
<tr><td><code>lastReconcileTime</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>LastReconcileTime is the timestamp for the last time the job was reconciled by the controller.</p>
</td>
</tr>
<tr><td><code>failureMessage</code><br/>
<code>string</code>
</td>
<td>
   <p>FailureMessage contains any error messages if the benchmark job failed.</p>
</td>
</tr>
<tr><td><code>details</code><br/>
<code>string</code>
</td>
<td>
   <p>Details provide additional information or metadata about the benchmark job.</p>
</td>
</tr>
</tbody>
</table>

## `CapacityReservationCondition`     {#ome-io-v1beta1-CapacityReservationCondition}
    

**Appears in:**

- [CapacityReservationStatus](#ome-io-v1beta1-CapacityReservationStatus)


<p>CapacityReservationCondition defines health and operational status of the capacity reservation.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>type</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CapacityReservationConditionType"><code>CapacityReservationConditionType</code></a>
</td>
<td>
   <p>Type of condition.</p>
</td>
</tr>
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#conditionstatus-v1-core"><code>k8s.io/api/core/v1.ConditionStatus</code></a>
</td>
<td>
   <p>Status of the condition.</p>
</td>
</tr>
<tr><td><code>lastTransitionTime</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>LastTransitionTime is the timestamp when the condition last changed.</p>
</td>
</tr>
<tr><td><code>reason</code><br/>
<code>string</code>
</td>
<td>
   <p>Reason for the condition's last transition.</p>
</td>
</tr>
<tr><td><code>message</code><br/>
<code>string</code>
</td>
<td>
   <p>Message is a human-readable message indicating details about the condition.</p>
</td>
</tr>
</tbody>
</table>

## `CapacityReservationConditionType`     {#ome-io-v1beta1-CapacityReservationConditionType}
    
(Alias of `string`)

**Appears in:**

- [CapacityReservationCondition](#ome-io-v1beta1-CapacityReservationCondition)


<p>CapacityReservationConditionType is a string enumeration type for the condition type of the Capacity Reservation.</p>




## `CapacityReservationLifecycleState`     {#ome-io-v1beta1-CapacityReservationLifecycleState}
    
(Alias of `string`)

**Appears in:**

- [CapacityReservationStatus](#ome-io-v1beta1-CapacityReservationStatus)


<p>CapacityReservationLifecycleState is a string enumeration type for the state of the Capacity Reservation.</p>




## `CapacityReservationSpec`     {#ome-io-v1beta1-CapacityReservationSpec}
    

**Appears in:**

- [CapacityReservation](#ome-io-v1beta1-CapacityReservation)

- [ClusterCapacityReservation](#ome-io-v1beta1-ClusterCapacityReservation)


<p>CapacityReservationSpec defines the desired state of Capacity Reservation.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>resourceGroups</code> <B>[Required]</B><br/>
<code>[]sigs.k8s.io/kueue/apis/kueue/v1beta1.ResourceGroup</code>
</td>
<td>
   <p>ResourceGroups defines the list of resource groups for the Capacity Reservation.
These are the groups of resources that the cluster queue will reserve.
Limits the number of items to 50 to avoid exceeding validation complexity limits in Kubernetes API.</p>
</td>
</tr>
<tr><td><code>cohort</code><br/>
<code>string</code>
</td>
<td>
   <p>Cohort specifies the cohort that the cluster queue belongs to, which is used for grouping cluster queues.</p>
</td>
</tr>
<tr><td><code>preemptionRule</code><br/>
<code>sigs.k8s.io/kueue/apis/kueue/v1beta1.ClusterQueuePreemption</code>
</td>
<td>
   <p>PreemptionRule specifies the preemption behavior of the cluster queue associated to capacity reservation.</p>
</td>
</tr>
<tr><td><code>compartmentID</code><br/>
<code>string</code>
</td>
<td>
   <p>The compartment ID to use for the Capacity Reservation.</p>
</td>
</tr>
<tr><td><code>priorityClassName</code><br/>
<code>string</code>
</td>
<td>
   <p>PriorityClassName is the priority class assigned to workloads associated to the Capacity Reservation.</p>
</td>
</tr>
<tr><td><code>allowBorrowing</code><br/>
<code>bool</code>
</td>
<td>
   <p>AllowBorrowing defines if this capacity reservation can borrow resources from others.</p>
</td>
</tr>
</tbody>
</table>

## `CapacityReservationStatus`     {#ome-io-v1beta1-CapacityReservationStatus}
    

**Appears in:**

- [CapacityReservation](#ome-io-v1beta1-CapacityReservation)

- [ClusterCapacityReservation](#ome-io-v1beta1-ClusterCapacityReservation)


<p>CapacityReservationStatus defines the observed status of CapacityReservation.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>capacity</code><br/>
<code>[]sigs.k8s.io/kueue/apis/kueue/v1beta1.FlavorUsage</code>
</td>
<td>
   <p>Capacity represents the total resources available in this capacity reservation.</p>
</td>
</tr>
<tr><td><code>allocatable</code><br/>
<code>[]sigs.k8s.io/kueue/apis/kueue/v1beta1.FlavorUsage</code>
</td>
<td>
   <p>Allocatable represents the resources that are available for scheduling.</p>
</td>
</tr>
<tr><td><code>associationUsages</code><br/>
<a href="#ome-io-v1beta1-AssociationUsage"><code>[]AssociationUsage</code></a>
</td>
<td>
   <p>Usages of associations
An association can be a DAC or a Workload</p>
</td>
</tr>
<tr><td><code>conditions</code><br/>
<a href="#ome-io-v1beta1-CapacityReservationCondition"><code>[]CapacityReservationCondition</code></a>
</td>
<td>
   <p>Conditions represents health and operational states.</p>
</td>
</tr>
<tr><td><code>capacityReservationLifecycleState</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CapacityReservationLifecycleState"><code>CapacityReservationLifecycleState</code></a>
</td>
<td>
   <p>CapacityReservationLifecycleState indicates the current phase of the CapacityReservation (e.g., &quot;active&quot;, &quot;creating&quot;, &quot;Failed&quot; etc.).</p>
</td>
</tr>
<tr><td><code>lifecycleDetail</code><br/>
<code>string</code>
</td>
<td>
   <p>A message describing the current state in more detail that can provide actionable information.</p>
</td>
</tr>
</tbody>
</table>

## `ComponentExtensionSpec`     {#ome-io-v1beta1-ComponentExtensionSpec}
    

**Appears in:**

- [DecoderSpec](#ome-io-v1beta1-DecoderSpec)

- [EngineSpec](#ome-io-v1beta1-EngineSpec)

- [PredictorSpec](#ome-io-v1beta1-PredictorSpec)

- [RouterSpec](#ome-io-v1beta1-RouterSpec)


<p>ComponentExtensionSpec defines the deployment configuration for a given InferenceService component</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>minReplicas</code><br/>
<code>int</code>
</td>
<td>
   <p>Minimum number of replicas, defaults to 1 but can be set to 0 to enable scale-to-zero.</p>
</td>
</tr>
<tr><td><code>maxReplicas</code><br/>
<code>int</code>
</td>
<td>
   <p>Maximum number of replicas for autoscaling.</p>
</td>
</tr>
<tr><td><code>scaleTarget</code><br/>
<code>int</code>
</td>
<td>
   <p>ScaleTarget specifies the integer target value of the metric type the Autoscaler watches for.
concurrency and rps targets are supported by Knative Pod Autoscaler
(https://knative.dev/docs/serving/autoscaling/autoscaling-targets/).</p>
</td>
</tr>
<tr><td><code>scaleMetric</code><br/>
<a href="#ome-io-v1beta1-ScaleMetric"><code>ScaleMetric</code></a>
</td>
<td>
   <p>ScaleMetric defines the scaling metric type watched by autoscaler
possible values are concurrency, rps, cpu, memory. concurrency, rps are supported via
Knative Pod Autoscaler(https://knative.dev/docs/serving/autoscaling/autoscaling-metrics).</p>
</td>
</tr>
<tr><td><code>containerConcurrency</code><br/>
<code>int64</code>
</td>
<td>
   <p>ContainerConcurrency specifies how many requests can be processed concurrently, this sets the hard limit of the container
concurrency(https://knative.dev/docs/serving/autoscaling/concurrency).</p>
</td>
</tr>
<tr><td><code>timeoutSeconds</code><br/>
<code>int64</code>
</td>
<td>
   <p>TimeoutSeconds specifies the number of seconds to wait before timing out a request to the component.</p>
</td>
</tr>
<tr><td><code>canaryTrafficPercent</code><br/>
<code>int64</code>
</td>
<td>
   <p>CanaryTrafficPercent defines the traffic split percentage between the candidate revision and the last ready revision</p>
</td>
</tr>
<tr><td><code>labels</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Labels that will be add to the component pod.
More info: http://kubernetes.io/docs/user-guide/labels</p>
</td>
</tr>
<tr><td><code>annotations</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Annotations that will be add to the component pod.
More info: http://kubernetes.io/docs/user-guide/annotations</p>
</td>
</tr>
<tr><td><code>deploymentStrategy</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#deploymentstrategy-v1-apps"><code>k8s.io/api/apps/v1.DeploymentStrategy</code></a>
</td>
<td>
   <p>The deployment strategy to use to replace existing pods with new ones. Only applicable for raw deployment mode.</p>
</td>
</tr>
<tr><td><code>kedaConfig</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-KedaConfig"><code>KedaConfig</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ComponentStatusSpec`     {#ome-io-v1beta1-ComponentStatusSpec}
    

**Appears in:**

- [InferenceServiceStatus](#ome-io-v1beta1-InferenceServiceStatus)


<p>ComponentStatusSpec describes the state of the component</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>latestReadyRevision</code><br/>
<code>string</code>
</td>
<td>
   <p>Latest revision name that is in ready state</p>
</td>
</tr>
<tr><td><code>latestCreatedRevision</code><br/>
<code>string</code>
</td>
<td>
   <p>Latest revision name that is created</p>
</td>
</tr>
<tr><td><code>previousRolledoutRevision</code><br/>
<code>string</code>
</td>
<td>
   <p>Previous revision name that is rolled out with 100 percent traffic</p>
</td>
</tr>
<tr><td><code>latestRolledoutRevision</code><br/>
<code>string</code>
</td>
<td>
   <p>Latest revision name that is rolled out with 100 percent traffic</p>
</td>
</tr>
<tr><td><code>traffic</code><br/>
<a href="https://pkg.go.dev/knative.dev/serving/pkg/apis/serving/v1#TrafficTarget"><code>[]knative.dev/serving/pkg/apis/serving/v1.TrafficTarget</code></a>
</td>
<td>
   <p>Traffic holds the configured traffic distribution for latest ready revision and previous rolled out revision.</p>
</td>
</tr>
<tr><td><code>url</code><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis#URL"><code>knative.dev/pkg/apis.URL</code></a>
</td>
<td>
   <p>URL holds the primary url that will distribute traffic over the provided traffic targets.
This will be one the REST or gRPC endpoints that are available.
It generally has the form http[s]://{route-name}.{route-namespace}.{cluster-level-suffix}</p>
</td>
</tr>
<tr><td><code>restURL</code><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis#URL"><code>knative.dev/pkg/apis.URL</code></a>
</td>
<td>
   <p>REST endpoint of the component if available.</p>
</td>
</tr>
<tr><td><code>address</code><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Addressable"><code>knative.dev/pkg/apis/duck/v1.Addressable</code></a>
</td>
<td>
   <p>Addressable endpoint for the InferenceService</p>
</td>
</tr>
</tbody>
</table>

## `CoschedulingPodGroupPolicyConfig`     {#ome-io-v1beta1-CoschedulingPodGroupPolicyConfig}
    

**Appears in:**

- [PodGroupPolicy](#ome-io-v1beta1-PodGroupPolicy)


<p>CoschedulingPodGroupPolicyConfig represents configuration for co-scheduling plugin.
The number of min members in the PodGroupSpec is always equal to the number of nodes.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>scheduleTimeoutSeconds</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Time threshold to schedule PodGroup for gang-scheduling.
If the scheduling timeout is equal to 0, the default value is used.
Defaults to 60 seconds.</p>
</td>
</tr>
</tbody>
</table>

## `CrossReference`     {#ome-io-v1beta1-CrossReference}
    

**Appears in:**

- [ProjectSpec](#ome-io-v1beta1-ProjectSpec)

- [RateLimitSpec](#ome-io-v1beta1-RateLimitSpec)

- [ServiceAccountSpec](#ome-io-v1beta1-ServiceAccountSpec)

- [UserSpec](#ome-io-v1beta1-UserSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name is the name of the referenced resource</p>
</td>
</tr>
<tr><td><code>namespace</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Namespace is the namespace of the referenced resource (optional for cluster-scoped resources)</p>
</td>
</tr>
</tbody>
</table>

## `DacLifecycleState`     {#ome-io-v1beta1-DacLifecycleState}
    
(Alias of `string`)

**Appears in:**

- [DedicatedAIClusterStatus](#ome-io-v1beta1-DedicatedAIClusterStatus)


<p>DacLifecycleState is a string enumeration type for the state of the Dedicated AI Cluster.</p>




## `DecoderSpec`     {#ome-io-v1beta1-DecoderSpec}
    

**Appears in:**

- [InferenceServiceSpec](#ome-io-v1beta1-InferenceServiceSpec)

- [ServingRuntimeSpec](#ome-io-v1beta1-ServingRuntimeSpec)


<p>DecoderSpec defines the configuration for the Decoder component (token generation in PD-disaggregated deployment)
Used specifically for prefill-decode disaggregated deployments to handle the token generation phase.
Similar to EngineSpec in structure, it allows for detailed pod and container configuration,
but is specifically used for the decode phase when separating prefill and decode processes.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>PodSpec</code><br/>
<a href="#ome-io-v1beta1-PodSpec"><code>PodSpec</code></a>
</td>
<td>(Members of <code>PodSpec</code> are embedded into this type.)
   <p>This spec provides a full PodSpec for the decoder component
Allows complete customization of the Kubernetes Pod configuration including
containers, volumes, security contexts, affinity rules, and other pod settings.</p>
</td>
</tr>
<tr><td><code>ComponentExtensionSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ComponentExtensionSpec"><code>ComponentExtensionSpec</code></a>
</td>
<td>(Members of <code>ComponentExtensionSpec</code> are embedded into this type.)
   <p>ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.
Controls scaling behavior and resource allocation for the decoder component.</p>
</td>
</tr>
<tr><td><code>runner</code><br/>
<a href="#ome-io-v1beta1-RunnerSpec"><code>RunnerSpec</code></a>
</td>
<td>
   <p>Runner container override for customizing the main container
This is essentially a container spec that can override the default container
Defines the main decoder container configuration, including image,
resource requests/limits, environment variables, and command.</p>
</td>
</tr>
<tr><td><code>leader</code><br/>
<a href="#ome-io-v1beta1-LeaderSpec"><code>LeaderSpec</code></a>
</td>
<td>
   <p>Leader node configuration (only used for MultiNode deployment)
Defines the pod and container spec for the leader node that coordinates
distributed token generation in multi-node deployments.</p>
</td>
</tr>
<tr><td><code>worker</code><br/>
<a href="#ome-io-v1beta1-WorkerSpec"><code>WorkerSpec</code></a>
</td>
<td>
   <p>Worker nodes configuration (only used for MultiNode deployment)
Defines the pod and container spec for worker nodes that perform
distributed token generation tasks as directed by the leader.</p>
</td>
</tr>
</tbody>
</table>

## `DedicatedAIClusterProfileSpec`     {#ome-io-v1beta1-DedicatedAIClusterProfileSpec}
    

**Appears in:**

- [DedicatedAIClusterProfile](#ome-io-v1beta1-DedicatedAIClusterProfile)


<p>DedicatedAIClusterProfileSpec defines the desired state of DedicatedAIClusterProfile</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>disabled</code><br/>
<code>bool</code>
</td>
<td>
   <p>Set to true to disable use of this profile.</p>
</td>
</tr>
<tr><td><code>count</code><br/>
<code>int</code>
</td>
<td>
   <p>Count is the number of units in the DAC</p>
</td>
</tr>
<tr><td><code>resources</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core"><code>k8s.io/api/core/v1.ResourceRequirements</code></a>
</td>
<td>
   <p>The resource requirements of the DAC, get from spec.type + spec.shape</p>
</td>
</tr>
<tr><td><code>affinity</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core"><code>k8s.io/api/core/v1.Affinity</code></a>
</td>
<td>
   <p>The GPU shape affinity of DAC, get from spec.type + spec.shape</p>
</td>
</tr>
<tr><td><code>tolerations</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core"><code>[]k8s.io/api/core/v1.Toleration</code></a>
</td>
<td>
   <p>Tolerations specifies the tolerations for scheduling the resources on tainted nodes.</p>
</td>
</tr>
<tr><td><code>nodeSelector</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>NodeSelector specifies node selectors for scheduling the resources on specific nodes.</p>
</td>
</tr>
<tr><td><code>priorityClassName</code><br/>
<code>string</code>
</td>
<td>
   <p>PriorityClassName is the priority class assigned to workloads in this Dedicated AI Cluster.</p>
</td>
</tr>
</tbody>
</table>

## `DedicatedAIClusterProfileStatus`     {#ome-io-v1beta1-DedicatedAIClusterProfileStatus}
    

**Appears in:**

- [DedicatedAIClusterProfile](#ome-io-v1beta1-DedicatedAIClusterProfile)


<p>DedicatedAIClusterProfileStatus defines the observed state of DedicatedAIClusterProfile</p>




## `DedicatedAIClusterSpec`     {#ome-io-v1beta1-DedicatedAIClusterSpec}
    

**Appears in:**

- [DedicatedAICluster](#ome-io-v1beta1-DedicatedAICluster)


<p>DedicatedAIClusterSpec defines the desired state of DedicatedAICluster</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>profile</code><br/>
<code>string</code>
</td>
<td>
   <p>DedicatedAIClusterProfileName is the name of the DedicatedAIClusterProfile to use for this DedicatedAICluster.</p>
</td>
</tr>
<tr><td><code>count</code><br/>
<code>int</code>
</td>
<td>
   <p>Count is the number of resources in the DAC</p>
</td>
</tr>
<tr><td><code>resources</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core"><code>k8s.io/api/core/v1.ResourceRequirements</code></a>
</td>
<td>
   <p>The resource requirements of the DAC, get from spec.type + spec.shape</p>
</td>
</tr>
<tr><td><code>affinity</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core"><code>k8s.io/api/core/v1.Affinity</code></a>
</td>
<td>
   <p>The GPU shape affinity of DAC, get from spec.type + spec.shape</p>
</td>
</tr>
<tr><td><code>tolerations</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core"><code>[]k8s.io/api/core/v1.Toleration</code></a>
</td>
<td>
   <p>Tolerations specifies the tolerations for scheduling the resources on tainted nodes.</p>
</td>
</tr>
<tr><td><code>nodeSelector</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>NodeSelector specifies node selectors for scheduling the resources on specific nodes.</p>
</td>
</tr>
<tr><td><code>priorityClassName</code><br/>
<code>string</code>
</td>
<td>
   <p>PriorityClassName is the priority class assigned to workloads in this Dedicated AI Cluster.</p>
</td>
</tr>
<tr><td><code>compartmentID</code><br/>
<code>string</code>
</td>
<td>
   <p>The compartment ID to use for the DAC</p>
</td>
</tr>
<tr><td><code>capacityReservationId</code><br/>
<code>string</code>
</td>
<td>
   <p>CapacityReservation ID that used to create this DedicatedAICluster.</p>
</td>
</tr>
</tbody>
</table>

## `DedicatedAIClusterStatus`     {#ome-io-v1beta1-DedicatedAIClusterStatus}
    

**Appears in:**

- [DedicatedAICluster](#ome-io-v1beta1-DedicatedAICluster)


<p>DedicatedAIClusterStatus defines the observed state of DedicatedAICluster</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>availableGpu</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>The available number of GPU for allocation</p>
</td>
</tr>
<tr><td><code>allocatedGpu</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>The number of GPU already allocated</p>
</td>
</tr>
<tr><td><code>dacLifecycleState</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-DacLifecycleState"><code>DacLifecycleState</code></a>
</td>
<td>
   <p>DacLifecycleState indicates the current phase of the Dedicated AI Cluster (e.g., &quot;active&quot;, &quot;creating&quot;, &quot;Failed&quot; etc.).</p>
</td>
</tr>
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions reflects the current state of the cluster.</p>
</td>
</tr>
<tr><td><code>lifecycleDetail</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>A message describing the current state in more detail that can provide actionable information.</p>
</td>
</tr>
</tbody>
</table>

## `Endpoint`     {#ome-io-v1beta1-Endpoint}
    

**Appears in:**

- [EndpointSpec](#ome-io-v1beta1-EndpointSpec)


<p>Endpoint defines a direct URL-based inference service with additional API configuration.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>url</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>URL represents the endpoint URL for the inference service.</p>
</td>
</tr>
<tr><td><code>apiFormat</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>APIFormat specifies the type of API, such as &quot;openai&quot; or &quot;oci-cohere&quot;.</p>
</td>
</tr>
<tr><td><code>modelName</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>ModelName specifies the name of the model being served at the endpoint.
Useful for endpoints that require model-specific configuration. For instance,
for openai API, this is a required field in the payload</p>
</td>
</tr>
</tbody>
</table>

## `EndpointSpec`     {#ome-io-v1beta1-EndpointSpec}
    

**Appears in:**

- [BenchmarkJobSpec](#ome-io-v1beta1-BenchmarkJobSpec)


<p>EndpointSpec defines a reference to an inference service.
It supports either a Kubernetes-style reference (InferenceService) or an Endpoint struct for a direct URL.
Cross-namespace references are supported for InferenceService but require appropriate RBAC permissions to access resources in the target namespace.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>inferenceService</code><br/>
<a href="#ome-io-v1beta1-InferenceServiceReference"><code>InferenceServiceReference</code></a>
</td>
<td>
   <p>InferenceService holds a Kubernetes reference to an internal inference service.</p>
</td>
</tr>
<tr><td><code>endpoint</code><br/>
<a href="#ome-io-v1beta1-Endpoint"><code>Endpoint</code></a>
</td>
<td>
   <p>Endpoint holds the details of a direct endpoint for an external inference service, including URL and API details.</p>
</td>
</tr>
</tbody>
</table>

## `EngineSpec`     {#ome-io-v1beta1-EngineSpec}
    

**Appears in:**

- [InferenceServiceSpec](#ome-io-v1beta1-InferenceServiceSpec)

- [ServingRuntimeSpec](#ome-io-v1beta1-ServingRuntimeSpec)


<p>EngineSpec defines the configuration for the Engine component (can be used for both single-node and multi-node deployments)
Provides a comprehensive specification for deploying model serving containers and pods.
It allows for complete Kubernetes pod configuration including main containers,
init containers, sidecars, volumes, and other pod-level settings.
For distributed deployments, it supports leader-worker architecture configuration.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>PodSpec</code><br/>
<a href="#ome-io-v1beta1-PodSpec"><code>PodSpec</code></a>
</td>
<td>(Members of <code>PodSpec</code> are embedded into this type.)
   <p>This spec provides a full PodSpec for the engine component
Allows complete customization of the Kubernetes Pod configuration including
containers, volumes, security contexts, affinity rules, and other pod settings.</p>
</td>
</tr>
<tr><td><code>ComponentExtensionSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ComponentExtensionSpec"><code>ComponentExtensionSpec</code></a>
</td>
<td>(Members of <code>ComponentExtensionSpec</code> are embedded into this type.)
   <p>ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.
Controls scaling behavior and resource allocation for the engine component.</p>
</td>
</tr>
<tr><td><code>runner</code><br/>
<a href="#ome-io-v1beta1-RunnerSpec"><code>RunnerSpec</code></a>
</td>
<td>
   <p>Runner container override for customizing the engine container
This is essentially a container spec that can override the default container
Defines the main model runner container configuration, including image,
resource requests/limits, environment variables, and command.</p>
</td>
</tr>
<tr><td><code>leader</code><br/>
<a href="#ome-io-v1beta1-LeaderSpec"><code>LeaderSpec</code></a>
</td>
<td>
   <p>Leader node configuration (only used for MultiNode deployment)
Defines the pod and container spec for the leader node that coordinates
distributed inference in multi-node deployments.</p>
</td>
</tr>
<tr><td><code>worker</code><br/>
<a href="#ome-io-v1beta1-WorkerSpec"><code>WorkerSpec</code></a>
</td>
<td>
   <p>Worker nodes configuration (only used for MultiNode deployment)
Defines the pod and container spec for worker nodes that perform
distributed processing tasks as directed by the leader.</p>
</td>
</tr>
</tbody>
</table>

## `FailureInfo`     {#ome-io-v1beta1-FailureInfo}
    

**Appears in:**

- [ModelStatus](#ome-io-v1beta1-ModelStatus)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>location</code><br/>
<code>string</code>
</td>
<td>
   <p>Name of component to which the failure relates (usually Pod name)</p>
</td>
</tr>
<tr><td><code>reason</code><br/>
<a href="#ome-io-v1beta1-FailureReason"><code>FailureReason</code></a>
</td>
<td>
   <p>High level class of failure</p>
</td>
</tr>
<tr><td><code>message</code><br/>
<code>string</code>
</td>
<td>
   <p>Detailed error message</p>
</td>
</tr>
<tr><td><code>modelRevisionName</code><br/>
<code>string</code>
</td>
<td>
   <p>Internal Revision/ID of model, tied to specific Spec contents</p>
</td>
</tr>
<tr><td><code>time</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>Time failure occurred or was discovered</p>
</td>
</tr>
<tr><td><code>exitCode</code><br/>
<code>int32</code>
</td>
<td>
   <p>Exit status from the last termination of the container</p>
</td>
</tr>
</tbody>
</table>

## `FailureReason`     {#ome-io-v1beta1-FailureReason}
    
(Alias of `string`)

**Appears in:**

- [FailureInfo](#ome-io-v1beta1-FailureInfo)


<p>FailureReason enum</p>




## `FineTunedWeightSpec`     {#ome-io-v1beta1-FineTunedWeightSpec}
    

**Appears in:**

- [FineTunedWeight](#ome-io-v1beta1-FineTunedWeight)


<p>FineTunedWeightSpec defines the desired state of FineTunedWeight</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>baseModelRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ObjectReference"><code>ObjectReference</code></a>
</td>
<td>
   <p>Reference to the base model that this weight is fine-tuned from</p>
</td>
</tr>
<tr><td><code>modelType</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>ModelType of the fine-tuned weight, e.g., &quot;Distillation&quot;, &quot;Adapter&quot;, &quot;Tfew&quot;</p>
</td>
</tr>
<tr><td><code>hyperParameters</code> <B>[Required]</B><br/>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#RawExtension"><code>k8s.io/apimachinery/pkg/runtime.RawExtension</code></a>
</td>
<td>
   <p>HyperParameters used for fine-tuning, stored as generic JSON for flexibility</p>
</td>
</tr>
<tr><td><code>ModelExtensionSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelExtensionSpec"><code>ModelExtensionSpec</code></a>
</td>
<td>(Members of <code>ModelExtensionSpec</code> are embedded into this type.)
   <p>ModelExtension is the common extension of the model</p>
</td>
</tr>
<tr><td><code>configuration</code><br/>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#RawExtension"><code>k8s.io/apimachinery/pkg/runtime.RawExtension</code></a>
</td>
<td>
   <p>Configuration of the fine-tuned weight, stored as generic JSON for flexibility</p>
</td>
</tr>
<tr><td><code>storage</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>Storage configuration for the fine-tuned weight</p>
</td>
</tr>
<tr><td><code>trainingJobRef</code><br/>
<a href="#ome-io-v1beta1-ObjectReference"><code>ObjectReference</code></a>
</td>
<td>
   <p>TrainingJobID is the ID of the training job that produced this weight</p>
</td>
</tr>
</tbody>
</table>

## `HuggingFaceSecretReference`     {#ome-io-v1beta1-HuggingFaceSecretReference}
    

**Appears in:**

- [BenchmarkJobSpec](#ome-io-v1beta1-BenchmarkJobSpec)


<p>HuggingFaceSecretReference defines a reference to a Kubernetes Secret containing the Hugging Face API key.
This secret must reside in the same namespace as the BenchmarkJob.
Cross-namespace references are not allowed for security and simplicity.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the secret containing the Hugging Face API key.
The secret must reside in the same namespace as the BenchmarkJob.</p>
</td>
</tr>
</tbody>
</table>

## `HyperparameterTuningConfig`     {#ome-io-v1beta1-HyperparameterTuningConfig}
    

**Appears in:**

- [TrainingJobSpec](#ome-io-v1beta1-TrainingJobSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>method</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Method specifies the search algorithm to use (grid, random, bayes)</p>
</td>
</tr>
<tr><td><code>metric</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-MetricConfig"><code>MetricConfig</code></a>
</td>
<td>
   <p>Metric defines the objective metric to optimize</p>
</td>
</tr>
<tr><td><code>parameters</code> <B>[Required]</B><br/>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#RawExtension"><code>k8s.io/apimachinery/pkg/runtime.RawExtension</code></a>
</td>
<td>
   <p>Parameters defines the hyperparameters and their search spaces</p>
</td>
</tr>
<tr><td><code>maxTrials</code><br/>
<code>int32</code>
</td>
<td>
   <p>MaxTrials specifies the maximum number of trials to run</p>
</td>
</tr>
</tbody>
</table>

## `InferenceGraphSpec`     {#ome-io-v1beta1-InferenceGraphSpec}
    

**Appears in:**

- [InferenceGraph](#ome-io-v1beta1-InferenceGraph)


<p>InferenceGraphSpec defines the InferenceGraph spec</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>nodes</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-InferenceRouter"><code>map[string]InferenceRouter</code></a>
</td>
<td>
   <p>Map of InferenceGraph router nodes
Each node defines the router which can be different routing types</p>
</td>
</tr>
<tr><td><code>resources</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core"><code>k8s.io/api/core/v1.ResourceRequirements</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>affinity</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core"><code>k8s.io/api/core/v1.Affinity</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>timeout</code><br/>
<code>int64</code>
</td>
<td>
   <p>TimeoutSeconds specifies the number of seconds to wait before timing out a request to the component.</p>
</td>
</tr>
<tr><td><code>minReplicas</code><br/>
<code>int32</code>
</td>
<td>
   <p>Minimum number of replicas, defaults to 1 but can be set to 0 to enable scale-to-zero.</p>
</td>
</tr>
<tr><td><code>maxReplicas</code><br/>
<code>int32</code>
</td>
<td>
   <p>Maximum number of replicas for autoscaling.</p>
</td>
</tr>
<tr><td><code>scaleTarget</code><br/>
<code>int32</code>
</td>
<td>
   <p>ScaleTarget specifies the integer target value of the metric type the Autoscaler watches for.
concurrency and rps targets are supported by Knative Pod Autoscaler
(https://knative.dev/docs/serving/autoscaling/autoscaling-targets/).</p>
</td>
</tr>
<tr><td><code>scaleMetric</code><br/>
<a href="#ome-io-v1beta1-ScaleMetric"><code>ScaleMetric</code></a>
</td>
<td>
   <p>ScaleMetric defines the scaling metric type watched by autoscaler
possible values are concurrency, rps, cpu, memory. concurrency, rps are supported via
Knative Pod Autoscaler(https://knative.dev/docs/serving/autoscaling/autoscaling-metrics).</p>
</td>
</tr>
<tr><td><code>tolerations</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core"><code>[]k8s.io/api/core/v1.Toleration</code></a>
</td>
<td>
   <p>Toleration specifies the toleration for the InferenceGraph.
https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/</p>
</td>
</tr>
<tr><td><code>nodeSelector</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>NodeSelector specifies the node selector for the InferenceGraph.
https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/</p>
</td>
</tr>
<tr><td><code>nodeName</code><br/>
<code>string</code>
</td>
<td>
   <p>NodeName specifies the node name for the InferenceGraph.
https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/</p>
</td>
</tr>
<tr><td><code>serviceAccountName</code><br/>
<code>string</code>
</td>
<td>
   <p>ServiceAccountName specifies the service account name for the InferenceGraph.
https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/</p>
</td>
</tr>
</tbody>
</table>

## `InferenceGraphStatus`     {#ome-io-v1beta1-InferenceGraphStatus}
    

**Appears in:**

- [InferenceGraph](#ome-io-v1beta1-InferenceGraph)


<p>InferenceGraphStatus defines the InferenceGraph conditions and status</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>Status</code> <B>[Required]</B><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status"><code>knative.dev/pkg/apis/duck/v1.Status</code></a>
</td>
<td>(Members of <code>Status</code> are embedded into this type.)
   <p>Conditions for InferenceGraph</p>
</td>
</tr>
<tr><td><code>url</code><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis#URL"><code>knative.dev/pkg/apis.URL</code></a>
</td>
<td>
   <p>Url for the InferenceGraph</p>
</td>
</tr>
</tbody>
</table>

## `InferenceRouter`     {#ome-io-v1beta1-InferenceRouter}
    

**Appears in:**

- [InferenceGraphSpec](#ome-io-v1beta1-InferenceGraphSpec)


<p>InferenceRouter defines the router for each InferenceGraph node with one or multiple steps</p>
<pre style="background-color:#fff"><span style="color:#000;font-weight:bold">kind</span>:<span style="color:#bbb"> </span>InferenceGraph<span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">metadata</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">name</span>:<span style="color:#bbb"> </span>canary-route<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">spec</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">nodes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	  </span><span style="color:#000;font-weight:bold">root</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routerType</span>:<span style="color:#bbb"> </span>Splitter<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>mymodel1<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">weight</span>:<span style="color:#bbb"> </span><span style="color:#099">20</span><span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>mymodel2<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">weight</span>:<span style="color:#bbb"> </span><span style="color:#099">80</span><span style="color:#bbb">
</span><span style="color:#bbb">
</span></pre><pre style="background-color:#fff"><span style="color:#000;font-weight:bold">kind</span>:<span style="color:#bbb"> </span>InferenceGraph<span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">metadata</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">name</span>:<span style="color:#bbb"> </span>abtest<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">spec</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">nodes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	  </span><span style="color:#000;font-weight:bold">mymodel</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routerType</span>:<span style="color:#bbb"> </span>Switch<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>mymodel1<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">condition</span>:<span style="color:#bbb"> </span><span style="color:#d14">&#34;{ .input.userId == 1 }&#34;</span><span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>mymodel2<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">condition</span>:<span style="color:#bbb"> </span><span style="color:#d14">&#34;{ .input.userId == 2 }&#34;</span><span style="color:#bbb">
</span><span style="color:#bbb">
</span></pre><p>Scoring a case using a model ensemble consists of scoring it using each model separately,
then combining the results into a single scoring result using one of the pre-defined combination methods.</p>
<p>Tree Ensemble constitutes a case where simple algorithms for combining results of either classification or regression trees are well known.
Multiple classification trees, for example, are commonly combined using a &quot;majority-vote&quot; method.
Multiple regression trees are often combined using various averaging techniques.
e.g tagging models with segment identifiers and weights to be used for their combination in these ways.</p>
<pre style="background-color:#fff"><span style="color:#000;font-weight:bold">kind</span>:<span style="color:#bbb"> </span>InferenceGraph<span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">metadata</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">name</span>:<span style="color:#bbb"> </span>ensemble<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">spec</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">nodes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	  </span><span style="color:#000;font-weight:bold">root</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routerType</span>:<span style="color:#bbb"> </span>Sequence<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>feast<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">nodeName</span>:<span style="color:#bbb"> </span>ensembleModel<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">data</span>:<span style="color:#bbb"> </span>$response<span style="color:#bbb">
</span><span style="color:#bbb">	  </span><span style="color:#000;font-weight:bold">ensembleModel</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routerType</span>:<span style="color:#bbb"> </span>Ensemble<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>sklearn-model<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>xgboost-model<span style="color:#bbb">
</span><span style="color:#bbb">
</span></pre><p>Scoring a case using a sequence, or chain of models allows the output of one model to be passed in as input to the subsequent models.</p>
<pre style="background-color:#fff"><span style="color:#000;font-weight:bold">kind</span>:<span style="color:#bbb"> </span>InferenceGraph<span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">metadata</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">name</span>:<span style="color:#bbb"> </span>model-chainer<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">spec</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">nodes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	  </span><span style="color:#000;font-weight:bold">root</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routerType</span>:<span style="color:#bbb"> </span>Sequence<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>mymodel-s1<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>mymodel-s2<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">data</span>:<span style="color:#bbb"> </span>$response<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>mymodel-s3<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">data</span>:<span style="color:#bbb"> </span>$response<span style="color:#bbb">
</span><span style="color:#bbb">
</span></pre><p>In the flow described below, the pre_processing node base64 encodes the image and passes it to two model nodes in the flow.
The encoded data is available to both these nodes for classification. The second node i.e. dog-breed-classification takes the
original input from the pre_processing node along-with the response from the cat-dog-classification node to do further classification
of the dog breed if required.</p>
<pre style="background-color:#fff"><span style="color:#000;font-weight:bold">kind</span>:<span style="color:#bbb"> </span>InferenceGraph<span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">metadata</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">name</span>:<span style="color:#bbb"> </span>dog-breed-classification<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb"></span><span style="color:#000;font-weight:bold">spec</span>:<span style="color:#bbb">
</span><span style="color:#bbb">
</span><span style="color:#bbb">	</span><span style="color:#000;font-weight:bold">nodes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	  </span><span style="color:#000;font-weight:bold">root</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routerType</span>:<span style="color:#bbb"> </span>Sequence<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>cat-dog-classifier<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">nodeName</span>:<span style="color:#bbb"> </span>breed-classifier<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">data</span>:<span style="color:#bbb"> </span>$request<span style="color:#bbb">
</span><span style="color:#bbb">	  </span><span style="color:#000;font-weight:bold">breed-classifier</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routerType</span>:<span style="color:#bbb"> </span>Switch<span style="color:#bbb">
</span><span style="color:#bbb">	    </span><span style="color:#000;font-weight:bold">routes</span>:<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>dog-breed-classifier<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">condition</span>:<span style="color:#bbb"> </span>{<span style="color:#bbb"> </span>.predictions.class<span style="color:#bbb"> </span>==<span style="color:#bbb"> </span><span style="color:#d14">&#34;dog&#34;</span><span style="color:#bbb"> </span>}<span style="color:#bbb">
</span><span style="color:#bbb">	    </span>- <span style="color:#000;font-weight:bold">service</span>:<span style="color:#bbb"> </span>cat-breed-classifier<span style="color:#bbb">
</span><span style="color:#bbb">	      </span><span style="color:#000;font-weight:bold">condition</span>:<span style="color:#bbb"> </span>{<span style="color:#bbb"> </span>.predictions.class<span style="color:#bbb"> </span>==<span style="color:#bbb"> </span><span style="color:#d14">&#34;cat&#34;</span><span style="color:#bbb"> </span>}<span style="color:#bbb">
</span><span style="color:#bbb">
</span></pre>

<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>routerType</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-InferenceRouterType"><code>InferenceRouterType</code></a>
</td>
<td>
   <p>RouterType</p>
<ul>
<li>
<p><code>Sequence:</code> chain multiple inference steps with input/output from previous step</p>
</li>
<li>
<p><code>Splitter:</code> randomly routes to the target service according to the weight</p>
</li>
<li>
<p><code>Ensemble:</code> routes the request to multiple models and then merge the responses</p>
</li>
<li>
<p><code>Switch:</code> routes the request to one of the steps based on condition</p>
</li>
</ul>
</td>
</tr>
<tr><td><code>steps</code><br/>
<a href="#ome-io-v1beta1-InferenceStep"><code>[]InferenceStep</code></a>
</td>
<td>
   <p>Steps defines destinations for the current router node</p>
</td>
</tr>
</tbody>
</table>

## `InferenceRouterType`     {#ome-io-v1beta1-InferenceRouterType}
    
(Alias of `string`)

**Appears in:**

- [InferenceRouter](#ome-io-v1beta1-InferenceRouter)


<p>InferenceRouterType constant for inference routing types</p>




## `InferenceServiceReference`     {#ome-io-v1beta1-InferenceServiceReference}
    

**Appears in:**

- [EndpointSpec](#ome-io-v1beta1-EndpointSpec)


<p>InferenceServiceReference defines the reference to a Kubernetes inference service.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name specifies the name of the inference service to benchmark.</p>
</td>
</tr>
<tr><td><code>namespace</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Namespace specifies the Kubernetes namespace where the inference service is deployed.
Cross-namespace references are allowed but require appropriate RBAC permissions.</p>
</td>
</tr>
</tbody>
</table>

## `InferenceServiceSpec`     {#ome-io-v1beta1-InferenceServiceSpec}
    

**Appears in:**

- [InferenceService](#ome-io-v1beta1-InferenceService)


<p>InferenceServiceSpec is the top level type for this resource</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>predictor</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-PredictorSpec"><code>PredictorSpec</code></a>
</td>
<td>
   <p>Predictor defines the model serving spec
It specifies how the model should be deployed and served, handling inference requests.</p>
</td>
</tr>
<tr><td><code>engine</code><br/>
<a href="#ome-io-v1beta1-EngineSpec"><code>EngineSpec</code></a>
</td>
<td>
   <p>Engine defines the serving engine spec
This provides detailed container and pod specifications for model serving.
It allows defining the model runner (container spec), as well as complete pod specifications
including init containers, sidecar containers, and other pod-level configurations.
Engine can also be configured for multi-node deployments using leader and worker specifications.</p>
</td>
</tr>
<tr><td><code>decoder</code><br/>
<a href="#ome-io-v1beta1-DecoderSpec"><code>DecoderSpec</code></a>
</td>
<td>
   <p>Decoder defines the decoder spec
This is specifically used for PD (Prefill-Decode) disaggregated serving deployments.
Similar to Engine in structure, it allows for container and pod specifications,
but is only utilized when implementing the disaggregated serving pattern
to separate the prefill and decode phases of inference.</p>
</td>
</tr>
<tr><td><code>model</code><br/>
<a href="#ome-io-v1beta1-ModelRef"><code>ModelRef</code></a>
</td>
<td>
   <p>Model defines the model to be used for inference, referencing either a BaseModel or a custom model.
This allows models to be managed independently of the serving configuration.</p>
</td>
</tr>
<tr><td><code>runtime</code><br/>
<a href="#ome-io-v1beta1-ServingRuntimeRef"><code>ServingRuntimeRef</code></a>
</td>
<td>
   <p>Runtime defines the serving runtime environment that will be used to execute the model.
It is an inference service spec template that determines how the service should be deployed.
Runtime is optional - if not defined, the operator will automatically select the best runtime
based on the model's size, architecture, format, quantization, and framework.</p>
</td>
</tr>
<tr><td><code>router</code><br/>
<a href="#ome-io-v1beta1-RouterSpec"><code>RouterSpec</code></a>
</td>
<td>
   <p>Router defines the router spec</p>
</td>
</tr>
<tr><td><code>kedaConfig</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-KedaConfig"><code>KedaConfig</code></a>
</td>
<td>
   <p>KedaConfig defines the autoscaling configuration for KEDA
Provides settings for event-driven autoscaling using KEDA (Kubernetes Event-driven Autoscaling),
allowing the service to scale based on custom metrics or event sources.</p>
</td>
</tr>
</tbody>
</table>

## `InferenceServiceStatus`     {#ome-io-v1beta1-InferenceServiceStatus}
    

**Appears in:**

- [InferenceService](#ome-io-v1beta1-InferenceService)


<p>InferenceServiceStatus defines the observed state of InferenceService</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>Status</code> <B>[Required]</B><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Status"><code>knative.dev/pkg/apis/duck/v1.Status</code></a>
</td>
<td>(Members of <code>Status</code> are embedded into this type.)
   <p>Conditions for the InferenceService <!-- raw HTML omitted --></p>
<ul>
<li>PredictorReady: predictor readiness condition; <!-- raw HTML omitted --></li>
<li>RoutesReady (serverless mode only): aggregated routing condition, i.e. endpoint readiness condition; <!-- raw HTML omitted --></li>
<li>LatestDeploymentReady (serverless mode only): aggregated configuration condition, i.e. latest deployment readiness condition; <!-- raw HTML omitted --></li>
<li>Ready: aggregated condition; <!-- raw HTML omitted --></li>
</ul>
</td>
</tr>
<tr><td><code>address</code><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/duck/v1#Addressable"><code>knative.dev/pkg/apis/duck/v1.Addressable</code></a>
</td>
<td>
   <p>Addressable endpoint for the InferenceService</p>
</td>
</tr>
<tr><td><code>url</code><br/>
<a href="https://pkg.go.dev/knative.dev/pkg/apis#URL"><code>knative.dev/pkg/apis.URL</code></a>
</td>
<td>
   <p>URL holds the url that will distribute traffic over the provided traffic targets.
It generally has the form http[s]://{route-name}.{route-namespace}.{cluster-level-suffix}</p>
</td>
</tr>
<tr><td><code>components</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ComponentStatusSpec"><code>map[ComponentType]ComponentStatusSpec</code></a>
</td>
<td>
   <p>Statuses for the components of the InferenceService</p>
</td>
</tr>
<tr><td><code>modelStatus</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelStatus"><code>ModelStatus</code></a>
</td>
<td>
   <p>Model related statuses</p>
</td>
</tr>
</tbody>
</table>

## `InferenceStep`     {#ome-io-v1beta1-InferenceStep}
    

**Appears in:**

- [InferenceRouter](#ome-io-v1beta1-InferenceRouter)


<p>InferenceStep defines the inference target of the current step with condition, weights and data.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Unique name for the step within this node</p>
</td>
</tr>
<tr><td><code>InferenceTarget</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-InferenceTarget"><code>InferenceTarget</code></a>
</td>
<td>(Members of <code>InferenceTarget</code> are embedded into this type.)
   <p>Node or service used to process this step</p>
</td>
</tr>
<tr><td><code>data</code><br/>
<code>string</code>
</td>
<td>
   <p>request data sent to the next route with input/output from the previous step
$request
$response.predictions</p>
</td>
</tr>
<tr><td><code>weight</code><br/>
<code>int64</code>
</td>
<td>
   <p>the weight for split of the traffic, only used for Split Router
when weight is specified all the routing targets should be sum to 100</p>
</td>
</tr>
<tr><td><code>condition</code><br/>
<code>string</code>
</td>
<td>
   <p>routing based on the condition</p>
</td>
</tr>
<tr><td><code>dependency</code><br/>
<a href="#ome-io-v1beta1-InferenceStepDependencyType"><code>InferenceStepDependencyType</code></a>
</td>
<td>
   <p>to decide whether a step is a hard or a soft dependency in the Inference Graph</p>
</td>
</tr>
</tbody>
</table>

## `InferenceStepDependencyType`     {#ome-io-v1beta1-InferenceStepDependencyType}
    
(Alias of `string`)

**Appears in:**

- [InferenceStep](#ome-io-v1beta1-InferenceStep)


<p>InferenceStepDependencyType constant for inference step dependency</p>




## `InferenceTarget`     {#ome-io-v1beta1-InferenceTarget}
    

**Appears in:**

- [InferenceStep](#ome-io-v1beta1-InferenceStep)


<p>Exactly one InferenceTarget field must be specified</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>nodeName</code><br/>
<code>string</code>
</td>
<td>
   <p>The node name for routing as next step</p>
</td>
</tr>
<tr><td><code>serviceName</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>named reference for InferenceService</p>
</td>
</tr>
<tr><td><code>serviceUrl</code><br/>
<code>string</code>
</td>
<td>
   <p>InferenceService URL, mutually exclusive with ServiceName</p>
</td>
</tr>
</tbody>
</table>

## `JobSetTemplateSpec`     {#ome-io-v1beta1-JobSetTemplateSpec}
    

**Appears in:**

- [TrainingRuntimeSpec](#ome-io-v1beta1-TrainingRuntimeSpec)


<p>JobSetTemplateSpec represents a template of the desired JobSet.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>spec</code> <B>[Required]</B><br/>
<code>sigs.k8s.io/jobset/api/jobset/v1alpha2.JobSetSpec</code>
</td>
<td>
   <p>Specification of the desired JobSet which will be created from TrainJob.</p>
</td>
</tr>
</tbody>
</table>

## `JobStatus`     {#ome-io-v1beta1-JobStatus}
    

**Appears in:**

- [TrainingJobStatus](#ome-io-v1beta1-TrainingJobStatus)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the child Job.</p>
</td>
</tr>
<tr><td><code>ready</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Ready is the number of child Jobs where the number of ready pods and completed pods
is greater than or equal to the total expected pod count for the child Job.</p>
</td>
</tr>
<tr><td><code>succeeded</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Succeeded is the number of successfully completed child Jobs.</p>
</td>
</tr>
<tr><td><code>failed</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Failed is the number of failed child Jobs.</p>
</td>
</tr>
<tr><td><code>active</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Active is the number of child Jobs with at least 1 pod in a running or pending state
which are not marked for deletion.</p>
</td>
</tr>
<tr><td><code>suspended</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Suspended is the number of child Jobs which are in a suspended state.</p>
</td>
</tr>
</tbody>
</table>

## `KedaConfig`     {#ome-io-v1beta1-KedaConfig}
    

**Appears in:**

- [ComponentExtensionSpec](#ome-io-v1beta1-ComponentExtensionSpec)

- [InferenceServiceSpec](#ome-io-v1beta1-InferenceServiceSpec)


<p>KedaConfig stores the configuration settings for KEDA autoscaling within the InferenceService.
It includes fields like the Prometheus server address, custom query, scaling threshold, and operator.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>enableKeda</code> <B>[Required]</B><br/>
<code>bool</code>
</td>
<td>
   <p>EnableKeda determines whether KEDA autoscaling is enabled for the InferenceService.</p>
<ul>
<li>true: KEDA will manage the autoscaling based on the provided configuration.</li>
<li>false: KEDA will not be used, and autoscaling will rely on other mechanisms (e.g., HPA).</li>
</ul>
</td>
</tr>
<tr><td><code>promServerAddress</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>PromServerAddress specifies the address of the Prometheus server that KEDA will query
to retrieve metrics for autoscaling decisions. This should be a fully qualified URL,
including the protocol and port number.</p>
<p>Example:
http://prometheus-operated.monitoring.svc.cluster.local:9090</p>
</td>
</tr>
<tr><td><code>customPromQuery</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>CustomPromQuery defines a custom Prometheus query that KEDA will execute to evaluate
the desired metric for scaling. This query should return a single numerical value that
represents the metric to be monitored.</p>
<p>Example:
avg_over_time(http_requests_total{service=&quot;llama&quot;}[5m])</p>
</td>
</tr>
<tr><td><code>scalingThreshold</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>ScalingThreshold sets the numerical threshold against which the result of the Prometheus
query will be compared. Depending on the ScalingOperator, this threshold determines
when to scale the number of replicas up or down.</p>
<p>Example:
&quot;10&quot; - The Autoscaler will compare the metric value to 10.</p>
</td>
</tr>
<tr><td><code>scalingOperator</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>ScalingOperator specifies the comparison operator used by KEDA to decide whether to scale
the Deployment. Common operators include:</p>
<ul>
<li>&quot;GreaterThanOrEqual&quot;: Scale up when the metric is &gt;= ScalingThreshold.</li>
<li>&quot;LessThanOrEqual&quot;: Scale down when the metric is &lt;= ScalingThreshold.</li>
</ul>
<p>This operator defines the condition under which scaling actions are triggered based on
the evaluated metric.</p>
<p>Example:
&quot;GreaterThanOrEqual&quot;</p>
</td>
</tr>
</tbody>
</table>

## `LeaderSpec`     {#ome-io-v1beta1-LeaderSpec}
    

**Appears in:**

- [DecoderSpec](#ome-io-v1beta1-DecoderSpec)

- [EngineSpec](#ome-io-v1beta1-EngineSpec)


<p>LeaderSpec defines the configuration for a leader node in a multi-node component
The leader node coordinates the activities of worker nodes in distributed inference or
token generation setups, handling task distribution and result aggregation.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>PodSpec</code><br/>
<a href="#ome-io-v1beta1-PodSpec"><code>PodSpec</code></a>
</td>
<td>(Members of <code>PodSpec</code> are embedded into this type.)
   <p>Pod specification for the leader node
This overrides the main PodSpec when specified
Allows customization of the Kubernetes Pod configuration specifically for the leader node.</p>
</td>
</tr>
<tr><td><code>runner</code><br/>
<a href="#ome-io-v1beta1-RunnerSpec"><code>RunnerSpec</code></a>
</td>
<td>
   <p>Runner container override for customizing the main container
This is essentially a container spec that can override the default container
Provides fine-grained control over the container that executes the leader node's coordination logic.</p>
</td>
</tr>
</tbody>
</table>

## `LifeCycleState`     {#ome-io-v1beta1-LifeCycleState}
    
(Alias of `string`)

**Appears in:**

- [ModelStatusSpec](#ome-io-v1beta1-ModelStatusSpec)


<p>LifeCycleState enum</p>




## `MLPolicy`     {#ome-io-v1beta1-MLPolicy}
    

**Appears in:**

- [TrainingRuntimeSpec](#ome-io-v1beta1-TrainingRuntimeSpec)


<p>MLPolicy represents configuration for the model training with ML-specific parameters.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>numNodes</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Number of training nodes.
Defaults to 1.</p>
</td>
</tr>
<tr><td><code>MLPolicyConfig</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-MLPolicyConfig"><code>MLPolicyConfig</code></a>
</td>
<td>(Members of <code>MLPolicyConfig</code> are embedded into this type.)
   <p>Configuration for the runtime-specific parameters, such as Torch or MPI.
Only one of its members may be specified.</p>
</td>
</tr>
</tbody>
</table>

## `MLPolicyConfig`     {#ome-io-v1beta1-MLPolicyConfig}
    

**Appears in:**

- [MLPolicy](#ome-io-v1beta1-MLPolicy)


<p>MLPolicyConfig represents the runtime-specific configuration for various technologies.
One of the following specs can be set.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>torch</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TorchMLPolicyConfig"><code>TorchMLPolicyConfig</code></a>
</td>
<td>
   <p>Configuration for the PyTorch runtime.</p>
</td>
</tr>
<tr><td><code>mpi</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-MPIMLPolicyConfig"><code>MPIMLPolicyConfig</code></a>
</td>
<td>
   <p>Configuration for the MPI Runtime.</p>
</td>
</tr>
</tbody>
</table>

## `MPIImplementation`     {#ome-io-v1beta1-MPIImplementation}
    
(Alias of `string`)

**Appears in:**

- [MPIMLPolicyConfig](#ome-io-v1beta1-MPIMLPolicyConfig)


<p>MPIImplementation represents one of the supported MPI implementations.</p>




## `MPIMLPolicyConfig`     {#ome-io-v1beta1-MPIMLPolicyConfig}
    

**Appears in:**

- [MLPolicyConfig](#ome-io-v1beta1-MLPolicyConfig)


<p>MPIMLPolicyConfig represents a MPI runtime configuration.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>numProcPerNode</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Number of processes per node.
This value is equal to the number of slots for each node in the hostfile.</p>
</td>
</tr>
<tr><td><code>mpiImplementation</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-MPIImplementation"><code>MPIImplementation</code></a>
</td>
<td>
   <p>Implementation name for the MPI to create the appropriate hostfile.
Defaults to OpenMPI.</p>
</td>
</tr>
<tr><td><code>sshAuthMountPath</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Directory where SSH keys are mounted.</p>
</td>
</tr>
<tr><td><code>runLauncherAsNode</code> <B>[Required]</B><br/>
<code>bool</code>
</td>
<td>
   <p>Whether to run training process on the launcher Job.
Defaults to false.</p>
</td>
</tr>
</tbody>
</table>

## `MetricConfig`     {#ome-io-v1beta1-MetricConfig}
    

**Appears in:**

- [HyperparameterTuningConfig](#ome-io-v1beta1-HyperparameterTuningConfig)


<p>MetricConfig defines the metric to optimize during hyperparameter tuning</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the metric</p>
</td>
</tr>
<tr><td><code>goal</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Goal indicates whether to minimize or maximize the metric</p>
</td>
</tr>
</tbody>
</table>

## `ModelConfig`     {#ome-io-v1beta1-ModelConfig}
    

**Appears in:**

- [TrainingJobSpec](#ome-io-v1beta1-TrainingJobSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>inputModel</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>InputModel defines the base model name.</p>
</td>
</tr>
<tr><td><code>outputModel</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>OutputModel defines where the finetune weight (output model) stores.</p>
</td>
</tr>
</tbody>
</table>

## `ModelCopies`     {#ome-io-v1beta1-ModelCopies}
    

**Appears in:**

- [ModelStatus](#ome-io-v1beta1-ModelStatus)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>failedCopies</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>How many copies of this predictor's models failed to load recently</p>
</td>
</tr>
<tr><td><code>totalCopies</code><br/>
<code>int</code>
</td>
<td>
   <p>Total number copies of this predictor's models that are currently loaded</p>
</td>
</tr>
</tbody>
</table>

## `ModelExtensionSpec`     {#ome-io-v1beta1-ModelExtensionSpec}
    

**Appears in:**

- [BaseModelSpec](#ome-io-v1beta1-BaseModelSpec)

- [FineTunedWeightSpec](#ome-io-v1beta1-FineTunedWeightSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>displayName</code><br/>
<code>string</code>
</td>
<td>
   <p>DisplayName is the user-friendly name of the model</p>
</td>
</tr>
<tr><td><code>version</code><br/>
<code>string</code>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>disabled</code><br/>
<code>bool</code>
</td>
<td>
   <p>Whether the model is enabled or not</p>
</td>
</tr>
<tr><td><code>vendor</code><br/>
<code>string</code>
</td>
<td>
   <p>Vendor of the model, e.g., &quot;NVIDIA&quot;, &quot;Meta&quot;, &quot;HuggingFace&quot;</p>
</td>
</tr>
<tr><td><code>compartmentID</code><br/>
<code>string</code>
</td>
<td>
   <p>CompartmentID is the compartment ID of the model</p>
</td>
</tr>
</tbody>
</table>

## `ModelFormat`     {#ome-io-v1beta1-ModelFormat}
    

**Appears in:**

- [BaseModelSpec](#ome-io-v1beta1-BaseModelSpec)

- [SupportedModelFormat](#ome-io-v1beta1-SupportedModelFormat)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the format in which the model is stored, e.g., &quot;ONNX&quot;, &quot;TensorFlow SavedModel&quot;, &quot;PyTorch&quot;, &quot;SafeTensors&quot;</p>
</td>
</tr>
<tr><td><code>version</code><br/>
<code>string</code>
</td>
<td>
   <p>Version of the model format.
Used in validating that a runtime supports a predictor.
It Can be &quot;major&quot;, &quot;major.minor&quot; or &quot;major.minor.patch&quot;.</p>
</td>
</tr>
</tbody>
</table>

## `ModelFrameworkSpec`     {#ome-io-v1beta1-ModelFrameworkSpec}
    

**Appears in:**

- [BaseModelSpec](#ome-io-v1beta1-BaseModelSpec)

- [SupportedModelFormat](#ome-io-v1beta1-SupportedModelFormat)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the library in which the model is stored, e.g., &quot;ONNXRuntime&quot;, &quot;TensorFlow&quot;, &quot;PyTorch&quot;, &quot;Transformer&quot;, &quot;TensorRTLLM&quot;</p>
</td>
</tr>
<tr><td><code>version</code><br/>
<code>string</code>
</td>
<td>
   <p>Version of the library.
Used in validating that a runtime supports a predictor.
It Can be &quot;major&quot;, &quot;major.minor&quot; or &quot;major.minor.patch&quot;.</p>
</td>
</tr>
</tbody>
</table>

## `ModelQuantization`     {#ome-io-v1beta1-ModelQuantization}
    
(Alias of `string`)

**Appears in:**

- [BaseModelSpec](#ome-io-v1beta1-BaseModelSpec)

- [SupportedModelFormat](#ome-io-v1beta1-SupportedModelFormat)





## `ModelRef`     {#ome-io-v1beta1-ModelRef}
    

**Appears in:**

- [InferenceServiceSpec](#ome-io-v1beta1-InferenceServiceSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the model being referenced
Identifies the specific model to be used for inference.</p>
</td>
</tr>
<tr><td><code>kind</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Kind of the model being referenced
Defaults to ClusterBaseModel
Specifies the Kubernetes resource kind of the referenced model.</p>
</td>
</tr>
<tr><td><code>apiGroup</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>APIGroup of the resource being referenced
Defaults to <code>ome.io</code>
Specifies the Kubernetes API group of the referenced model.</p>
</td>
</tr>
<tr><td><code>fineTunedWeights</code><br/>
<code>[]string</code>
</td>
<td>
   <p>Optional FineTunedWeights references
References to fine-tuned weights that should be applied to the base model.</p>
</td>
</tr>
</tbody>
</table>

## `ModelRevisionStates`     {#ome-io-v1beta1-ModelRevisionStates}
    

**Appears in:**

- [ModelStatus](#ome-io-v1beta1-ModelStatus)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>activeModelState</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelState"><code>ModelState</code></a>
</td>
<td>
   <p>High level state string: Pending, Standby, Loading, Loaded, FailedToLoad</p>
</td>
</tr>
<tr><td><code>targetModelState</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelState"><code>ModelState</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ModelSizeRangeSpec`     {#ome-io-v1beta1-ModelSizeRangeSpec}
    

**Appears in:**

- [ServingRuntimeSpec](#ome-io-v1beta1-ServingRuntimeSpec)


<p>ModelSizeRangeSpec defines the range of model sizes supported by this runtime</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>min</code><br/>
<code>string</code>
</td>
<td>
   <p>Minimum size of the model in bytes</p>
</td>
</tr>
<tr><td><code>max</code><br/>
<code>string</code>
</td>
<td>
   <p>Maximum size of the model in bytes</p>
</td>
</tr>
</tbody>
</table>

## `ModelSpec`     {#ome-io-v1beta1-ModelSpec}
    

**Appears in:**

- [PredictorSpec](#ome-io-v1beta1-PredictorSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>runtime</code><br/>
<code>string</code>
</td>
<td>
   <p>Specific ClusterServingRuntime/ServingRuntime name to use for deployment.</p>
</td>
</tr>
<tr><td><code>PredictorExtensionSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-PredictorExtensionSpec"><code>PredictorExtensionSpec</code></a>
</td>
<td>(Members of <code>PredictorExtensionSpec</code> are embedded into this type.)
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>baseModel</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>fineTunedWeights</code> <B>[Required]</B><br/>
<code>[]string</code>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ModelState`     {#ome-io-v1beta1-ModelState}
    
(Alias of `string`)

**Appears in:**

- [ModelRevisionStates](#ome-io-v1beta1-ModelRevisionStates)


<p>ModelState enum</p>




## `ModelStatus`     {#ome-io-v1beta1-ModelStatus}
    

**Appears in:**

- [InferenceServiceStatus](#ome-io-v1beta1-InferenceServiceStatus)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>transitionStatus</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TransitionStatus"><code>TransitionStatus</code></a>
</td>
<td>
   <p>Whether the available predictor endpoints reflect the current Spec or is in transition</p>
</td>
</tr>
<tr><td><code>modelRevisionStates</code><br/>
<a href="#ome-io-v1beta1-ModelRevisionStates"><code>ModelRevisionStates</code></a>
</td>
<td>
   <p>State information of the predictor's model.</p>
</td>
</tr>
<tr><td><code>lastFailureInfo</code><br/>
<a href="#ome-io-v1beta1-FailureInfo"><code>FailureInfo</code></a>
</td>
<td>
   <p>Details of last failure, when load of target model is failed or blocked.</p>
</td>
</tr>
<tr><td><code>modelCopies</code><br/>
<a href="#ome-io-v1beta1-ModelCopies"><code>ModelCopies</code></a>
</td>
<td>
   <p>Model copy information of the predictor's model.</p>
</td>
</tr>
</tbody>
</table>

## `ModelStatusSpec`     {#ome-io-v1beta1-ModelStatusSpec}
    

**Appears in:**

- [BaseModel](#ome-io-v1beta1-BaseModel)

- [ClusterBaseModel](#ome-io-v1beta1-ClusterBaseModel)

- [FineTunedWeight](#ome-io-v1beta1-FineTunedWeight)


<p>ModelStatusSpec defines the observed state of Model weight</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>lifecycle</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>LifeCycle is an enum of Deprecated, Experiment, Public, Internal</p>
</td>
</tr>
<tr><td><code>state</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-LifeCycleState"><code>LifeCycleState</code></a>
</td>
<td>
   <p>Status of the model weight</p>
</td>
</tr>
<tr><td><code>nodesReady</code> <B>[Required]</B><br/>
<code>[]string</code>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>nodesFailed</code> <B>[Required]</B><br/>
<code>[]string</code>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
</tbody>
</table>

## `ObjectReference`     {#ome-io-v1beta1-ObjectReference}
    

**Appears in:**

- [FineTunedWeightSpec](#ome-io-v1beta1-FineTunedWeightSpec)


<p>ObjectReference contains enough information to let you inspect or modify the referred object.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the referenced object</p>
</td>
</tr>
<tr><td><code>namespace</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Namespace of the referenced object</p>
</td>
</tr>
</tbody>
</table>

## `OrganizationSpec`     {#ome-io-v1beta1-OrganizationSpec}
    

**Appears in:**

- [Organization](#ome-io-v1beta1-Organization)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>vendor</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-Vendor"><code>Vendor</code></a>
</td>
<td>
   <p>Vendor specifies the AI platform vendor (e.g., &quot;openai&quot;, &quot;anthropic&quot;)</p>
</td>
</tr>
<tr><td><code>disabled</code><br/>
<code>bool</code>
</td>
<td>
   <p>Disabled indicates whether the organization is disabled</p>
</td>
</tr>
<tr><td><code>organizationId</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>OrganizationId is the platform-specific organization ID</p>
</td>
</tr>
<tr><td><code>secretRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-SecretReference"><code>SecretReference</code></a>
</td>
<td>
   <p>SecretRef references the secret containing the API key
optional</p>
</td>
</tr>
<tr><td><code>config</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Config contains vendor-specific configuration</p>
</td>
</tr>
</tbody>
</table>

## `OrganizationStatus`     {#ome-io-v1beta1-OrganizationStatus}
    

**Appears in:**

- [Organization](#ome-io-v1beta1-Organization)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions represent the latest available observations of an object's state</p>
</td>
</tr>
</tbody>
</table>

## `PodGroupPolicy`     {#ome-io-v1beta1-PodGroupPolicy}
    

**Appears in:**

- [TrainingRuntimeSpec](#ome-io-v1beta1-TrainingRuntimeSpec)


<p>PodGroupPolicy represents a PodGroup configuration for gang-scheduling.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>coscheduling</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CoschedulingPodGroupPolicyConfig"><code>CoschedulingPodGroupPolicyConfig</code></a>
</td>
<td>
   <p>Coscheduling plugin from the Kubernetes scheduler-plugins for gang-scheduling.</p>
</td>
</tr>
</tbody>
</table>

## `PodOverride`     {#ome-io-v1beta1-PodOverride}
    

**Appears in:**

- [BenchmarkJobSpec](#ome-io-v1beta1-BenchmarkJobSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>image</code><br/>
<code>string</code>
</td>
<td>
   <p>Image specifies the container image to use for the benchmark job.</p>
</td>
</tr>
<tr><td><code>env</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core"><code>[]k8s.io/api/core/v1.EnvVar</code></a>
</td>
<td>
   <p>List of environment variables to set in the container.</p>
</td>
</tr>
<tr><td><code>envFrom</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envfromsource-v1-core"><code>[]k8s.io/api/core/v1.EnvFromSource</code></a>
</td>
<td>
   <p>List of sources to populate environment variables in the container.</p>
</td>
</tr>
<tr><td><code>volumeMounts</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core"><code>[]k8s.io/api/core/v1.VolumeMount</code></a>
</td>
<td>
   <p>Pod volumes to mount into the container's filesystem.</p>
</td>
</tr>
<tr><td><code>resources</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core"><code>k8s.io/api/core/v1.ResourceRequirements</code></a>
</td>
<td>
   <p>Compute Resources required by this container.
Cannot be updated.
More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/</p>
</td>
</tr>
<tr><td><code>tolerations</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core"><code>[]k8s.io/api/core/v1.Toleration</code></a>
</td>
<td>
   <p>If specified, the pod's tolerations.</p>
</td>
</tr>
<tr><td><code>nodeSelector</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>NodeSelector is a selector which must be true for the pod to fit on a node.
Selector which must match a node's labels for the pod to be scheduled on that node.
More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</p>
</td>
</tr>
<tr><td><code>affinity</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core"><code>k8s.io/api/core/v1.Affinity</code></a>
</td>
<td>
   <p>If specified, the pod's scheduling constraints</p>
</td>
</tr>
<tr><td><code>volumes</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core"><code>[]k8s.io/api/core/v1.Volume</code></a>
</td>
<td>
   <p>List of volumes that can be mounted by containers belonging to the pod.
More info: https://kubernetes.io/docs/concepts/storage/volumes</p>
</td>
</tr>
</tbody>
</table>

## `PodSpec`     {#ome-io-v1beta1-PodSpec}
    

**Appears in:**

- [DecoderSpec](#ome-io-v1beta1-DecoderSpec)

- [EngineSpec](#ome-io-v1beta1-EngineSpec)

- [LeaderSpec](#ome-io-v1beta1-LeaderSpec)

- [PredictorSpec](#ome-io-v1beta1-PredictorSpec)

- [RouterSpec](#ome-io-v1beta1-RouterSpec)

- [WorkerSpec](#ome-io-v1beta1-WorkerSpec)


<p>PodSpec is a description of a pod.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>volumes</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core"><code>[]k8s.io/api/core/v1.Volume</code></a>
</td>
<td>
   <p>List of volumes that can be mounted by containers belonging to the pod.
More info: https://kubernetes.io/docs/concepts/storage/volumes</p>
</td>
</tr>
<tr><td><code>initContainers</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core"><code>[]k8s.io/api/core/v1.Container</code></a>
</td>
<td>
   <p>List of initialization containers belonging to the pod.
Init containers are executed in order prior to containers being started. If any
init container fails, the pod is considered to have failed and is handled according
to its restartPolicy. The name for an init container or normal container must be
unique among all containers.
Init containers may not have Lifecycle actions, Readiness probes, Liveness probes, or Startup probes.
The resourceRequirements of an init container are taken into account during scheduling
by finding the highest request/limit for each resource type, and then using the max of
of that value or the sum of the normal containers. Limits are applied to init containers
in a similar fashion.
Init containers cannot currently be added or removed.
Cannot be updated.
More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/</p>
</td>
</tr>
<tr><td><code>containers</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core"><code>[]k8s.io/api/core/v1.Container</code></a>
</td>
<td>
   <p>List of containers belonging to the pod.
Containers cannot currently be added or removed.
There must be at least one container in a Pod.
Cannot be updated.</p>
</td>
</tr>
<tr><td><code>ephemeralContainers</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#ephemeralcontainer-v1-core"><code>[]k8s.io/api/core/v1.EphemeralContainer</code></a>
</td>
<td>
   <p>List of ephemeral containers run in this pod. Ephemeral containers may be run in an existing
pod to perform user-initiated actions such as debugging. This list cannot be specified when
creating a pod, and it cannot be modified by updating the pod spec. In order to add an
ephemeral container to an existing pod, use the pod's ephemeralcontainers subresource.</p>
</td>
</tr>
<tr><td><code>restartPolicy</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#restartpolicy-v1-core"><code>k8s.io/api/core/v1.RestartPolicy</code></a>
</td>
<td>
   <p>Restart policy for all containers within the pod.
One of Always, OnFailure, Never. In some contexts, only a subset of those values may be permitted.
Default to Always.
More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#restart-policy</p>
</td>
</tr>
<tr><td><code>terminationGracePeriodSeconds</code><br/>
<code>int64</code>
</td>
<td>
   <p>Optional duration in seconds the pod needs to terminate gracefully. May be decreased in delete request.
Value must be non-negative integer. The value zero indicates stop immediately via
the kill signal (no opportunity to shut down).
If this value is nil, the default grace period will be used instead.
The grace period is the duration in seconds after the processes running in the pod are sent
a termination signal and the time when the processes are forcibly halted with a kill signal.
Set this value longer than the expected cleanup time for your process.
Defaults to 30 seconds.</p>
</td>
</tr>
<tr><td><code>activeDeadlineSeconds</code><br/>
<code>int64</code>
</td>
<td>
   <p>Optional duration in seconds the pod may be active on the node relative to
StartTime before the system will actively try to mark it failed and kill associated containers.
Value must be a positive integer.</p>
</td>
</tr>
<tr><td><code>dnsPolicy</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#dnspolicy-v1-core"><code>k8s.io/api/core/v1.DNSPolicy</code></a>
</td>
<td>
   <p>Set DNS policy for the pod.
Defaults to &quot;ClusterFirst&quot;.
Valid values are 'ClusterFirstWithHostNet', 'ClusterFirst', 'Default' or 'None'.
DNS parameters given in DNSConfig will be merged with the policy selected with DNSPolicy.
To have DNS options set along with hostNetwork, you have to specify DNS policy
explicitly to 'ClusterFirstWithHostNet'.</p>
</td>
</tr>
<tr><td><code>nodeSelector</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>NodeSelector is a selector which must be true for the pod to fit on a node.
Selector which must match a node's labels for the pod to be scheduled on that node.
More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</p>
</td>
</tr>
<tr><td><code>serviceAccountName</code><br/>
<code>string</code>
</td>
<td>
   <p>ServiceAccountName is the name of the ServiceAccount to use to run this pod.
More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/</p>
</td>
</tr>
<tr><td><code>serviceAccount</code><br/>
<code>string</code>
</td>
<td>
   <p>DeprecatedServiceAccount is a deprecated alias for ServiceAccountName.
Deprecated: Use serviceAccountName instead.</p>
</td>
</tr>
<tr><td><code>automountServiceAccountToken</code><br/>
<code>bool</code>
</td>
<td>
   <p>AutomountServiceAccountToken indicates whether a service account token should be automatically mounted.</p>
</td>
</tr>
<tr><td><code>nodeName</code><br/>
<code>string</code>
</td>
<td>
   <p>NodeName indicates in which node this pod is scheduled.
If empty, this pod is a candidate for scheduling by the scheduler defined in schedulerName.
Once this field is set, the kubelet for this node becomes responsible for the lifecycle of this pod.
This field should not be used to express a desire for the pod to be scheduled on a specific node.
https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodename</p>
</td>
</tr>
<tr><td><code>hostNetwork</code><br/>
<code>bool</code>
</td>
<td>
   <p>Host networking requested for this pod. Use the host's network namespace.
If this option is set, the ports that will be used must be specified.
Default to false.</p>
</td>
</tr>
<tr><td><code>hostPID</code><br/>
<code>bool</code>
</td>
<td>
   <p>Use the host's pid namespace.
Optional: Default to false.</p>
</td>
</tr>
<tr><td><code>hostIPC</code><br/>
<code>bool</code>
</td>
<td>
   <p>Use the host's ipc namespace.
Optional: Default to false.</p>
</td>
</tr>
<tr><td><code>shareProcessNamespace</code><br/>
<code>bool</code>
</td>
<td>
   <p>Share a single process namespace between all of the containers in a pod.
When this is set containers will be able to view and signal processes from other containers
in the same pod, and the first process in each container will not be assigned PID 1.
HostPID and ShareProcessNamespace cannot both be set.
Optional: Default to false.</p>
</td>
</tr>
<tr><td><code>securityContext</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core"><code>k8s.io/api/core/v1.PodSecurityContext</code></a>
</td>
<td>
   <p>SecurityContext holds pod-level security attributes and common container settings.
Optional: Defaults to empty.  See type description for default values of each field.</p>
</td>
</tr>
<tr><td><code>imagePullSecrets</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core"><code>[]k8s.io/api/core/v1.LocalObjectReference</code></a>
</td>
<td>
   <p>ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
If specified, these secrets will be passed to individual puller implementations for them to use.
More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod</p>
</td>
</tr>
<tr><td><code>hostname</code><br/>
<code>string</code>
</td>
<td>
   <p>Specifies the hostname of the Pod
If not specified, the pod's hostname will be set to a system-defined value.</p>
</td>
</tr>
<tr><td><code>subdomain</code><br/>
<code>string</code>
</td>
<td>
   <p>If specified, the fully qualified Pod hostname will be &quot;<!-- raw HTML omitted -->.<!-- raw HTML omitted -->.<!-- raw HTML omitted -->.svc.<!-- raw HTML omitted -->&quot;.
If not specified, the pod will not have a domainname at all.</p>
</td>
</tr>
<tr><td><code>affinity</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core"><code>k8s.io/api/core/v1.Affinity</code></a>
</td>
<td>
   <p>If specified, the pod's scheduling constraints</p>
</td>
</tr>
<tr><td><code>schedulerName</code><br/>
<code>string</code>
</td>
<td>
   <p>If specified, the pod will be dispatched by specified scheduler.
If not specified, the pod will be dispatched by default scheduler.</p>
</td>
</tr>
<tr><td><code>tolerations</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core"><code>[]k8s.io/api/core/v1.Toleration</code></a>
</td>
<td>
   <p>If specified, the pod's tolerations.</p>
</td>
</tr>
<tr><td><code>hostAliases</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#hostalias-v1-core"><code>[]k8s.io/api/core/v1.HostAlias</code></a>
</td>
<td>
   <p>HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
file if specified.</p>
</td>
</tr>
<tr><td><code>priorityClassName</code><br/>
<code>string</code>
</td>
<td>
   <p>If specified, indicates the pod's priority. &quot;system-node-critical&quot; and
&quot;system-cluster-critical&quot; are two special keywords which indicate the
highest priorities with the former being the highest priority. Any other
name must be defined by creating a PriorityClass object with that name.
If not specified, the pod priority will be default or zero if there is no
default.</p>
</td>
</tr>
<tr><td><code>priority</code><br/>
<code>int32</code>
</td>
<td>
   <p>The priority value. Various system components use this field to find the
priority of the pod. When Priority Admission Controller is enabled, it
prevents users from setting this field. The admission controller populates
this field from PriorityClassName.
The higher the value, the higher the priority.</p>
</td>
</tr>
<tr><td><code>dnsConfig</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#poddnsconfig-v1-core"><code>k8s.io/api/core/v1.PodDNSConfig</code></a>
</td>
<td>
   <p>Specifies the DNS parameters of a pod.
Parameters specified here will be merged to the generated DNS
configuration based on DNSPolicy.</p>
</td>
</tr>
<tr><td><code>readinessGates</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podreadinessgate-v1-core"><code>[]k8s.io/api/core/v1.PodReadinessGate</code></a>
</td>
<td>
   <p>If specified, all readiness gates will be evaluated for pod readiness.
A pod is ready when all its containers are ready AND
all conditions specified in the readiness gates have status equal to &quot;True&quot;
More info: https://git.k8s.io/enhancements/keps/sig-network/580-pod-readiness-gates</p>
</td>
</tr>
<tr><td><code>runtimeClassName</code><br/>
<code>string</code>
</td>
<td>
   <p>RuntimeClassName refers to a RuntimeClass object in the node.k8s.io group, which should be used
to run this pod.  If no RuntimeClass resource matches the named class, the pod will not be run.
If unset or empty, the &quot;legacy&quot; RuntimeClass will be used, which is an implicit class with an
empty definition that uses the default runtime handler.
More info: https://git.k8s.io/enhancements/keps/sig-node/585-runtime-class</p>
</td>
</tr>
<tr><td><code>enableServiceLinks</code><br/>
<code>bool</code>
</td>
<td>
   <p>EnableServiceLinks indicates whether information about services should be injected into pod's
environment variables, matching the syntax of Docker links.
Optional: Defaults to true.</p>
</td>
</tr>
<tr><td><code>preemptionPolicy</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#preemptionpolicy-v1-core"><code>k8s.io/api/core/v1.PreemptionPolicy</code></a>
</td>
<td>
   <p>PreemptionPolicy is the Policy for preempting pods with lower priority.
One of Never, PreemptLowerPriority.
Defaults to PreemptLowerPriority if unset.</p>
</td>
</tr>
<tr><td><code>overhead</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcelist-v1-core"><code>k8s.io/api/core/v1.ResourceList</code></a>
</td>
<td>
   <p>Overhead represents the resource overhead associated with running a pod for a given RuntimeClass.
This field will be autopopulated at admission time by the RuntimeClass admission controller. If
the RuntimeClass admission controller is enabled, overhead must not be set in Pod create requests.
The RuntimeClass admission controller will reject Pod create requests which have the overhead already
set. If RuntimeClass is configured and selected in the PodSpec, Overhead will be set to the value
defined in the corresponding RuntimeClass, otherwise it will remain unset and treated as zero.
More info: https://git.k8s.io/enhancements/keps/sig-node/688-pod-overhead/README.md</p>
</td>
</tr>
<tr><td><code>topologySpreadConstraints</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core"><code>[]k8s.io/api/core/v1.TopologySpreadConstraint</code></a>
</td>
<td>
   <p>TopologySpreadConstraints describes how a group of pods ought to spread across topology
domains. Scheduler will schedule pods in a way which abides by the constraints.
All topologySpreadConstraints are ANDed.</p>
</td>
</tr>
<tr><td><code>setHostnameAsFQDN</code><br/>
<code>bool</code>
</td>
<td>
   <p>If true the pod's hostname will be configured as the pod's FQDN, rather than the leaf name (the default).
In Linux containers, this means setting the FQDN in the hostname field of the kernel (the nodename field of struct utsname).
In Windows containers, this means setting the registry value of hostname for the registry key HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters to FQDN.
If a pod does not have FQDN, this has no effect.
Default to false.</p>
</td>
</tr>
<tr><td><code>os</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podos-v1-core"><code>k8s.io/api/core/v1.PodOS</code></a>
</td>
<td>
   <p>Specifies the OS of the containers in the pod.
Some pod and container fields are restricted if this is set.</p>
<p>If the OS field is set to linux, the following fields must be unset:
-securityContext.windowsOptions</p>
<p>If the OS field is set to windows, following fields must be unset:</p>
<ul>
<li>spec.hostPID</li>
<li>spec.hostIPC</li>
<li>spec.hostUsers</li>
<li>spec.securityContext.appArmorProfile</li>
<li>spec.securityContext.seLinuxOptions</li>
<li>spec.securityContext.seccompProfile</li>
<li>spec.securityContext.fsGroup</li>
<li>spec.securityContext.fsGroupChangePolicy</li>
<li>spec.securityContext.sysctls</li>
<li>spec.shareProcessNamespace</li>
<li>spec.securityContext.runAsUser</li>
<li>spec.securityContext.runAsGroup</li>
<li>spec.securityContext.supplementalGroups</li>
<li>spec.securityContext.supplementalGroupsPolicy</li>
<li>spec.containers[*].securityContext.appArmorProfile</li>
<li>spec.containers[*].securityContext.seLinuxOptions</li>
<li>spec.containers[*].securityContext.seccompProfile</li>
<li>spec.containers[*].securityContext.capabilities</li>
<li>spec.containers[*].securityContext.readOnlyRootFilesystem</li>
<li>spec.containers[*].securityContext.privileged</li>
<li>spec.containers[*].securityContext.allowPrivilegeEscalation</li>
<li>spec.containers[*].securityContext.procMount</li>
<li>spec.containers[*].securityContext.runAsUser</li>
<li>spec.containers[*].securityContext.runAsGroup</li>
</ul>
</td>
</tr>
<tr><td><code>hostUsers</code><br/>
<code>bool</code>
</td>
<td>
   <p>Use the host's user namespace.
Optional: Default to true.
If set to true or not present, the pod will be run in the host user namespace, useful
for when the pod needs a feature only available to the host user namespace, such as
loading a kernel module with CAP_SYS_MODULE.
When set to false, a new userns is created for the pod. Setting false is useful for
mitigating container breakout vulnerabilities even allowing users to run their
containers as root without actually having root privileges on the host.
This field is alpha-level and is only honored by servers that enable the UserNamespacesSupport feature.</p>
</td>
</tr>
<tr><td><code>schedulingGates</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podschedulinggate-v1-core"><code>[]k8s.io/api/core/v1.PodSchedulingGate</code></a>
</td>
<td>
   <p>SchedulingGates is an opaque list of values that if specified will block scheduling the pod.
If schedulingGates is not empty, the pod will stay in the SchedulingGated state and the
scheduler will not attempt to schedule the pod.</p>
<p>SchedulingGates can only be set at pod creation time, and be removed only afterwards.</p>
</td>
</tr>
<tr><td><code>resourceClaims</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podresourceclaim-v1-core"><code>[]k8s.io/api/core/v1.PodResourceClaim</code></a>
</td>
<td>
   <p>ResourceClaims defines which ResourceClaims must be allocated
and reserved before the Pod is allowed to start. The resources
will be made available to those containers which consume them
by name.</p>
<p>This is an alpha field and requires enabling the
DynamicResourceAllocation feature gate.</p>
<p>This field is immutable.</p>
</td>
</tr>
</tbody>
</table>

## `PredictorExtensionSpec`     {#ome-io-v1beta1-PredictorExtensionSpec}
    

**Appears in:**

- [ModelSpec](#ome-io-v1beta1-ModelSpec)


<p>PredictorExtensionSpec defines configuration shared across all predictor frameworks</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>storageUri</code><br/>
<code>string</code>
</td>
<td>
   <p>This field points to the location of the model which is mounted onto the pod.</p>
</td>
</tr>
<tr><td><code>runtimeVersion</code><br/>
<code>string</code>
</td>
<td>
   <p>Runtime version of the predictor docker image</p>
</td>
</tr>
<tr><td><code>protocolVersion</code><br/>
<code>bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants.InferenceServiceProtocol</code>
</td>
<td>
   <p>Protocol version to use by the predictor (i.e. v1 or v2 or grpc-v1 or grpc-v2)</p>
</td>
</tr>
<tr><td><code>Container</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core"><code>k8s.io/api/core/v1.Container</code></a>
</td>
<td>(Members of <code>Container</code> are embedded into this type.)
   <p>Container enables overrides for the predictor.
Each framework will have different defaults that are populated in the underlying container spec.</p>
</td>
</tr>
</tbody>
</table>

## `PredictorSpec`     {#ome-io-v1beta1-PredictorSpec}
    

**Appears in:**

- [InferenceServiceSpec](#ome-io-v1beta1-InferenceServiceSpec)


<p>PredictorSpec defines the configuration for a predictor,
The following fields follow a &quot;1-of&quot; semantic. Users must specify exactly one spec.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>model</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelSpec"><code>ModelSpec</code></a>
</td>
<td>
   <p>Model spec for any arbitrary framework.</p>
</td>
</tr>
<tr><td><code>PodSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-PodSpec"><code>PodSpec</code></a>
</td>
<td>(Members of <code>PodSpec</code> are embedded into this type.)
   <p>This spec is dual purpose. <!-- raw HTML omitted --></p>
<ol>
<li>Provide a full PodSpec for custom predictor.
The field PodSpec.Containers is mutually exclusive with other predictors (i.e. TFServing). <!-- raw HTML omitted --></li>
<li>Provide a predictor (i.e. TFServing) and specify PodSpec
overrides, you must not provide PodSpec.Containers in this case. <!-- raw HTML omitted --></li>
</ol>
</td>
</tr>
<tr><td><code>ComponentExtensionSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ComponentExtensionSpec"><code>ComponentExtensionSpec</code></a>
</td>
<td>(Members of <code>ComponentExtensionSpec</code> are embedded into this type.)
   <p>Component extension defines the deployment configurations for a predictor</p>
</td>
</tr>
<tr><td><code>workerSpec</code><br/>
<a href="#ome-io-v1beta1-WorkerSpec"><code>WorkerSpec</code></a>
</td>
<td>
   <p>WorkerSpec for the predictor, this is used for multi-node serving without Ray Cluster</p>
</td>
</tr>
</tbody>
</table>

## `ProjectSpec`     {#ome-io-v1beta1-ProjectSpec}
    

**Appears in:**

- [Project](#ome-io-v1beta1-Project)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name is the project name</p>
</td>
</tr>
<tr><td><code>description</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Description is the project description</p>
</td>
</tr>
<tr><td><code>organizationRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CrossReference"><code>CrossReference</code></a>
</td>
<td>
   <p>OrganizationRef references the Organization</p>
</td>
</tr>
<tr><td><code>config</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Config contains vendor-specific configuration</p>
</td>
</tr>
</tbody>
</table>

## `ProjectStatus`     {#ome-io-v1beta1-ProjectStatus}
    

**Appears in:**

- [Project](#ome-io-v1beta1-Project)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>projectId</code><br/>
<code>string</code>
</td>
<td>
   <p>ProjectId is the platform-specific project ID</p>
</td>
</tr>
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions represent the latest available observations of an object's state</p>
</td>
</tr>
<tr><td><code>creationTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>CreationTime is the time when the project was created</p>
</td>
</tr>
<tr><td><code>lastUpdatedTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>LastUpdatedTime is the time when the project was last updated</p>
</td>
</tr>
</tbody>
</table>

## `RateLimitConfig`     {#ome-io-v1beta1-RateLimitConfig}
    

**Appears in:**

- [RateLimitSpec](#ome-io-v1beta1-RateLimitSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>type</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Type is the type of rate limit (e.g., &quot;requests&quot;, &quot;tokens&quot;)</p>
</td>
</tr>
<tr><td><code>limit</code> <B>[Required]</B><br/>
<code>int64</code>
</td>
<td>
   <p>Limit is the maximum allowed value</p>
</td>
</tr>
<tr><td><code>window</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Window is the time window for the limit (e.g., &quot;1m&quot;, &quot;1d&quot;)</p>
</td>
</tr>
</tbody>
</table>

## `RateLimitSpec`     {#ome-io-v1beta1-RateLimitSpec}
    

**Appears in:**

- [RateLimit](#ome-io-v1beta1-RateLimit)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>projectRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CrossReference"><code>CrossReference</code></a>
</td>
<td>
   <p>ProjectRef references the Project</p>
</td>
</tr>
<tr><td><code>targetRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CrossReference"><code>CrossReference</code></a>
</td>
<td>
   <p>TargetRef references either a service account or user</p>
</td>
</tr>
<tr><td><code>limits</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-RateLimitConfig"><code>[]RateLimitConfig</code></a>
</td>
<td>
   <p>Limits defines the rate limit configurations</p>
</td>
</tr>
<tr><td><code>config</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Config contains vendor-specific configuration</p>
</td>
</tr>
</tbody>
</table>

## `RateLimitStatus`     {#ome-io-v1beta1-RateLimitStatus}
    

**Appears in:**

- [RateLimit](#ome-io-v1beta1-RateLimit)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions represent the latest available observations of an object's state</p>
</td>
</tr>
</tbody>
</table>

## `ReplicationJobPhase`     {#ome-io-v1beta1-ReplicationJobPhase}
    
(Alias of `string`)

**Appears in:**

- [ReplicationJobStatus](#ome-io-v1beta1-ReplicationJobStatus)


<p>ReplicationJobPhase defines possible states of a ReplicationJob</p>




## `ReplicationJobSpec`     {#ome-io-v1beta1-ReplicationJobSpec}
    

**Appears in:**

- [ReplicationJob](#ome-io-v1beta1-ReplicationJob)


<p>ReplicationJobSpec defines the desired state of ReplicationJob.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>source</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>Source specifies the data source for the replication job.</p>
</td>
</tr>
<tr><td><code>destination</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>Destination specifies the data destination for the replication job.</p>
</td>
</tr>
<tr><td><code>compartmentID</code><br/>
<code>string</code>
</td>
<td>
   <p>The compartment ID to use for the replication job.</p>
</td>
</tr>
<tr><td><code>containerOverride</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core"><code>k8s.io/api/core/v1.Container</code></a>
</td>
<td>
   <p>ContainerOverride defines the custom container configuration used for the replication job.</p>
</td>
</tr>
</tbody>
</table>

## `ReplicationJobStatus`     {#ome-io-v1beta1-ReplicationJobStatus}
    

**Appears in:**

- [ReplicationJob](#ome-io-v1beta1-ReplicationJob)


<p>ReplicationJobStatus represents the current state of the ReplicationJob.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>status</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ReplicationJobPhase"><code>ReplicationJobPhase</code></a>
</td>
<td>
   <p>Status represents the overall phase of the replication job.</p>
</td>
</tr>
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions is an array of current observed job conditions.</p>
</td>
</tr>
<tr><td><code>retryCount</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>RetryCount represents the number of retries the replication job has performed.</p>
</td>
</tr>
<tr><td><code>startTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>StartTime is the time when the replication job was acknowledged by the controller.
This field is updated best-effort and may not strictly reflect real-time operation order.</p>
</td>
</tr>
<tr><td><code>completionTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>CompletionTime is the time when the replication job completed.</p>
</td>
</tr>
<tr><td><code>lastReconcileTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>LastReconcileTime is the most recent time the job was reconciled.</p>
</td>
</tr>
<tr><td><code>message</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Message is a human-readable message indicating details about the job status.</p>
</td>
</tr>
</tbody>
</table>

## `RouterSpec`     {#ome-io-v1beta1-RouterSpec}
    

**Appears in:**

- [InferenceServiceSpec](#ome-io-v1beta1-InferenceServiceSpec)

- [ServingRuntimeSpec](#ome-io-v1beta1-ServingRuntimeSpec)


<p>RouterSpec defines the configuration for the Router component, which handles request routing</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>PodSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-PodSpec"><code>PodSpec</code></a>
</td>
<td>(Members of <code>PodSpec</code> are embedded into this type.)
   <p>PodSpec defines the container configuration for the router</p>
</td>
</tr>
<tr><td><code>ComponentExtensionSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ComponentExtensionSpec"><code>ComponentExtensionSpec</code></a>
</td>
<td>(Members of <code>ComponentExtensionSpec</code> are embedded into this type.)
   <p>ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.</p>
</td>
</tr>
<tr><td><code>runner</code><br/>
<a href="#ome-io-v1beta1-RunnerSpec"><code>RunnerSpec</code></a>
</td>
<td>
   <p>This is essentially a container spec that can override the default container</p>
</td>
</tr>
<tr><td><code>config</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Additional configuration parameters for the runner
This can include framework-specific settings</p>
</td>
</tr>
</tbody>
</table>

## `RunnerSpec`     {#ome-io-v1beta1-RunnerSpec}
    

**Appears in:**

- [DecoderSpec](#ome-io-v1beta1-DecoderSpec)

- [EngineSpec](#ome-io-v1beta1-EngineSpec)

- [LeaderSpec](#ome-io-v1beta1-LeaderSpec)

- [RouterSpec](#ome-io-v1beta1-RouterSpec)

- [WorkerSpec](#ome-io-v1beta1-WorkerSpec)


<p>RunnerSpec defines container configuration plus additional config settings
The Runner is the primary container that executes the model serving or token generation logic.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>Container</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core"><code>k8s.io/api/core/v1.Container</code></a>
</td>
<td>(Members of <code>Container</code> are embedded into this type.)
   <p>Container spec for the runner
Provides complete Kubernetes container configuration for the primary execution container.</p>
</td>
</tr>
</tbody>
</table>

## `RuntimeRef`     {#ome-io-v1beta1-RuntimeRef}
    

**Appears in:**

- [TrainingJobSpec](#ome-io-v1beta1-TrainingJobSpec)


<p>RuntimeRef represents the reference to the existing training runtime.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the runtime being referenced.
When namespaced-scoped TrainingRuntime is used, the TrainJob must have
the same namespace as the deployed runtime.</p>
</td>
</tr>
<tr><td><code>apiGroup</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>APIGroup of the runtime being referenced.
Defaults to <code>ome.io</code>.</p>
</td>
</tr>
<tr><td><code>kind</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Kind of the runtime being referenced.
Defaults to ClusterTrainingRuntime.</p>
</td>
</tr>
</tbody>
</table>

## `ScaleMetric`     {#ome-io-v1beta1-ScaleMetric}
    
(Alias of `string`)

**Appears in:**

- [ComponentExtensionSpec](#ome-io-v1beta1-ComponentExtensionSpec)

- [InferenceGraphSpec](#ome-io-v1beta1-InferenceGraphSpec)


<p>ScaleMetric enum</p>




## `SecretReference`     {#ome-io-v1beta1-SecretReference}
    

**Appears in:**

- [APIKeySpec](#ome-io-v1beta1-APIKeySpec)

- [OrganizationSpec](#ome-io-v1beta1-OrganizationSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name is the name of the secret</p>
</td>
</tr>
<tr><td><code>namespace</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Namespace is the namespace of the secret</p>
</td>
</tr>
<tr><td><code>key</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Key is the key in the secret</p>
</td>
</tr>
</tbody>
</table>

## `ServiceAccountSpec`     {#ome-io-v1beta1-ServiceAccountSpec}
    

**Appears in:**

- [ServiceAccount](#ome-io-v1beta1-ServiceAccount)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name is the service account name</p>
</td>
</tr>
<tr><td><code>projectRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CrossReference"><code>CrossReference</code></a>
</td>
<td>
   <p>ProjectRef references the Project</p>
</td>
</tr>
<tr><td><code>permissions</code><br/>
<code>[]string</code>
</td>
<td>
   <p>Permissions defines the service account permissions</p>
</td>
</tr>
<tr><td><code>role</code><br/>
<code>string</code>
</td>
<td>
   <p>Role defines the service account's role in the project, owner or member</p>
</td>
</tr>
<tr><td><code>config</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Config contains vendor-specific configuration</p>
</td>
</tr>
</tbody>
</table>

## `ServiceAccountStatus`     {#ome-io-v1beta1-ServiceAccountStatus}
    

**Appears in:**

- [ServiceAccount](#ome-io-v1beta1-ServiceAccount)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>serviceAccountId</code><br/>
<code>string</code>
</td>
<td>
   <p>ServiceAccountId is the platform-specific service account ID</p>
</td>
</tr>
<tr><td><code>apiKey</code><br/>
<a href="#ome-io-v1beta1-APIKeySpec"><code>APIKeySpec</code></a>
</td>
<td>
   <span class="text-muted">No description provided.</span></td>
</tr>
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions represent the latest available observations of an object's state</p>
</td>
</tr>
<tr><td><code>creationTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>CreationTime is the time when the service account was created</p>
</td>
</tr>
</tbody>
</table>

## `ServiceMetadata`     {#ome-io-v1beta1-ServiceMetadata}
    

**Appears in:**

- [BenchmarkJobSpec](#ome-io-v1beta1-BenchmarkJobSpec)


<p>ServiceMetadata contains metadata fields for recording the backend model server's configuration and version details.
This information helps track experiment context, enabling users to filter and query experiments based on server properties.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>engine</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Engine specifies the backend model server engine.
Supported values: &quot;vLLM&quot;, &quot;SGLang&quot;, &quot;TGI&quot;.</p>
</td>
</tr>
<tr><td><code>version</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Version specifies the version of the model server (e.g., &quot;0.5.3&quot;).</p>
</td>
</tr>
<tr><td><code>gpuType</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>GpuType specifies the type of GPU used by the model server.
Supported values: &quot;H100&quot;, &quot;A100&quot;, &quot;MI300&quot;, &quot;A10&quot;.</p>
</td>
</tr>
<tr><td><code>gpuCount</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>GpuCount indicates the number of GPU cards available on the model server.</p>
</td>
</tr>
</tbody>
</table>

## `ServingRuntimePodSpec`     {#ome-io-v1beta1-ServingRuntimePodSpec}
    

**Appears in:**

- [ServingRuntimeSpec](#ome-io-v1beta1-ServingRuntimeSpec)

- [WorkerPodSpec](#ome-io-v1beta1-WorkerPodSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>containers</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core"><code>[]k8s.io/api/core/v1.Container</code></a>
</td>
<td>
   <p>List of containers belonging to the pod.
Containers cannot currently be added or removed.
There must be at least one container in a Pod.
Cannot be updated.</p>
</td>
</tr>
<tr><td><code>volumes</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core"><code>[]k8s.io/api/core/v1.Volume</code></a>
</td>
<td>
   <p>List of volumes that can be mounted by containers belonging to the pod.
More info: https://kubernetes.io/docs/concepts/storage/volumes</p>
</td>
</tr>
<tr><td><code>nodeSelector</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>NodeSelector is a selector which must be true for the pod to fit on a node.
Selector which must match a node's labels for the pod to be scheduled on that node.
More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</p>
</td>
</tr>
<tr><td><code>affinity</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core"><code>k8s.io/api/core/v1.Affinity</code></a>
</td>
<td>
   <p>If specified, the pod's scheduling constraints</p>
</td>
</tr>
<tr><td><code>tolerations</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core"><code>[]k8s.io/api/core/v1.Toleration</code></a>
</td>
<td>
   <p>If specified, the pod's tolerations.</p>
</td>
</tr>
<tr><td><code>labels</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Labels that will be add to the pod.
More info: http://kubernetes.io/docs/user-guide/labels</p>
</td>
</tr>
<tr><td><code>annotations</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Annotations that will be add to the pod.
More info: http://kubernetes.io/docs/user-guide/annotations</p>
</td>
</tr>
<tr><td><code>imagePullSecrets</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core"><code>[]k8s.io/api/core/v1.LocalObjectReference</code></a>
</td>
<td>
   <p>ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
If specified, these secrets will be passed to individual puller implementations for them to use.
More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod</p>
</td>
</tr>
<tr><td><code>schedulerName</code><br/>
<code>string</code>
</td>
<td>
   <p>If specified, the pod will be dispatched by specified scheduler.
If not specified, the pod will be dispatched by default scheduler.</p>
</td>
</tr>
<tr><td><code>hostIPC</code><br/>
<code>bool</code>
</td>
<td>
   <p>Use the host's ipc namespace.
Optional: Default to false.</p>
</td>
</tr>
<tr><td><code>dnsPolicy</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#dnspolicy-v1-core"><code>k8s.io/api/core/v1.DNSPolicy</code></a>
</td>
<td>
   <p>Set DNS policy for the pod.
Defaults to &quot;ClusterFirst&quot;.
Valid values are 'ClusterFirstWithHostNet', 'ClusterFirst', 'Default' or 'None'.
DNS parameters given in DNSConfig will be merged with the policy selected with DNSPolicy.
To have DNS options set along with hostNetwork, you have to specify DNS policy
explicitly to 'ClusterFirstWithHostNet'.</p>
</td>
</tr>
<tr><td><code>hostNetwork</code><br/>
<code>bool</code>
</td>
<td>
   <p>Host networking requested for this pod. Use the host's network namespace.
If this option is set, the ports that will be used must be specified.
Default to false.</p>
</td>
</tr>
</tbody>
</table>

## `ServingRuntimeRef`     {#ome-io-v1beta1-ServingRuntimeRef}
    

**Appears in:**

- [InferenceServiceSpec](#ome-io-v1beta1-InferenceServiceSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Name of the runtime being referenced
Identifies the specific runtime environment to be used for model execution.</p>
</td>
</tr>
<tr><td><code>kind</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Kind of the runtime being referenced
Defaults to ClusterServingRuntime
Specifies the Kubernetes resource kind of the referenced runtime.
ClusterServingRuntime is a cluster-wide runtime, while ServingRuntime is namespace-scoped.</p>
</td>
</tr>
<tr><td><code>apiGroup</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>APIGroup of the resource being referenced
Defaults to <code>ome.io</code>
Specifies the Kubernetes API group of the referenced runtime.</p>
</td>
</tr>
</tbody>
</table>

## `ServingRuntimeSpec`     {#ome-io-v1beta1-ServingRuntimeSpec}
    

**Appears in:**

- [ClusterServingRuntime](#ome-io-v1beta1-ClusterServingRuntime)

- [ServingRuntime](#ome-io-v1beta1-ServingRuntime)



<p>ServingRuntimeSpec defines the desired state of ServingRuntime. This spec is currently provisional
and are subject to change as details regarding single-model serving and multi-model serving
are hammered out.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>supportedModelFormats</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-SupportedModelFormat"><code>[]SupportedModelFormat</code></a>
</td>
<td>
   <p>Model formats and version supported by this runtime</p>
</td>
</tr>
<tr><td><code>modelSizeRange</code><br/>
<a href="#ome-io-v1beta1-ModelSizeRangeSpec"><code>ModelSizeRangeSpec</code></a>
</td>
<td>
   <p>ModelSizeRange is the range of model sizes supported by this runtime</p>
</td>
</tr>
<tr><td><code>disabled</code><br/>
<code>bool</code>
</td>
<td>
   <p>Set to true to disable use of this runtime</p>
</td>
</tr>
<tr><td><code>routerConfig</code><br/>
<a href="#ome-io-v1beta1-RouterSpec"><code>RouterSpec</code></a>
</td>
<td>
   <p>Router configuration for this runtime</p>
</td>
</tr>
<tr><td><code>engineConfig</code><br/>
<a href="#ome-io-v1beta1-EngineSpec"><code>EngineSpec</code></a>
</td>
<td>
   <p>Engine configuration for this runtime</p>
</td>
</tr>
<tr><td><code>decoderConfig</code><br/>
<a href="#ome-io-v1beta1-DecoderSpec"><code>DecoderSpec</code></a>
</td>
<td>
   <p>Decoder configuration for this runtime</p>
</td>
</tr>
<tr><td><code>protocolVersions</code><br/>
<code>[]bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants.InferenceServiceProtocol</code>
</td>
<td>
   <p>Supported protocol versions (i.e. openAI or cohere or openInference-v1 or openInference-v2)</p>
</td>
</tr>
<tr><td><code>ServingRuntimePodSpec</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ServingRuntimePodSpec"><code>ServingRuntimePodSpec</code></a>
</td>
<td>(Members of <code>ServingRuntimePodSpec</code> are embedded into this type.)
   <p>PodSpec for the serving runtime</p>
</td>
</tr>
<tr><td><code>workers</code><br/>
<a href="#ome-io-v1beta1-WorkerPodSpec"><code>WorkerPodSpec</code></a>
</td>
<td>
   <p>WorkerPodSpec for the serving runtime, this is used for multi-node serving without Ray Cluster</p>
</td>
</tr>
</tbody>
</table>

## `ServingRuntimeStatus`     {#ome-io-v1beta1-ServingRuntimeStatus}
    

**Appears in:**

- [ClusterServingRuntime](#ome-io-v1beta1-ClusterServingRuntime)

- [ServingRuntime](#ome-io-v1beta1-ServingRuntime)


<p>ServingRuntimeStatus defines the observed state of ServingRuntime</p>




## `StorageSpec`     {#ome-io-v1beta1-StorageSpec}
    

**Appears in:**

- [BaseModelSpec](#ome-io-v1beta1-BaseModelSpec)

- [BenchmarkJobSpec](#ome-io-v1beta1-BenchmarkJobSpec)

- [FineTunedWeightSpec](#ome-io-v1beta1-FineTunedWeightSpec)

- [ModelConfig](#ome-io-v1beta1-ModelConfig)

- [ReplicationJobSpec](#ome-io-v1beta1-ReplicationJobSpec)

- [TrainingJobSpec](#ome-io-v1beta1-TrainingJobSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>path</code><br/>
<code>string</code>
</td>
<td>
   <p>Path is the absolute path where the model will be downloaded and stored on the node.</p>
</td>
</tr>
<tr><td><code>parameters</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Parameters contain key-value pairs to override default storage credentials or configuration.
These values are typically used to configure access to object storage or mount options.</p>
</td>
</tr>
<tr><td><code>key</code><br/>
<code>string</code>
</td>
<td>
   <p>StorageKey is the name of the key in a Kubernetes Secret used to authenticate access to the model storage.
This key will be used to fetch credentials during model download or access.</p>
</td>
</tr>
<tr><td><code>storageUri</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>StorageUri specifies the source URI of the model in a supported storage backend.
Supported formats:</p>
<ul>
<li>OCI Object Storage:   oci://n/{namespace}/b/{bucket}/o/{object_path}</li>
<li>Persistent Volume:    pvc://{pvc-name}/{sub-path}</li>
<li>Vendor-specific:      vendor://{vendor-name}/{resource-type}/{resource-path}
This field is required.</li>
</ul>
</td>
</tr>
<tr><td><code>nodeSelector</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>NodeSelector defines a set of key-value label pairs that must be present on a node
for the model to be scheduled and downloaded onto that node.</p>
</td>
</tr>
<tr><td><code>nodeAffinity</code><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#nodeaffinity-v1-core"><code>k8s.io/api/core/v1.NodeAffinity</code></a>
</td>
<td>
   <p>NodeAffinity describes the node affinity rules that further constrain which nodes
are eligible to download and store this model, based on advanced scheduling policies.</p>
</td>
</tr>
</tbody>
</table>

## `SupportedModelFormat`     {#ome-io-v1beta1-SupportedModelFormat}
    

**Appears in:**

- [ServingRuntimeSpec](#ome-io-v1beta1-ServingRuntimeSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>name</code><br/>
<code>string</code>
</td>
<td>
   <p>TODO this field is being used as model format name, and this is not correct, we should deprecate this and use Name from ModelFormat
Name of the model</p>
</td>
</tr>
<tr><td><code>modelFormat</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelFormat"><code>ModelFormat</code></a>
</td>
<td>
   <p>ModelFormat of the model, e.g., &quot;PyTorch&quot;, &quot;TensorFlow&quot;, &quot;ONNX&quot;, &quot;SafeTensors&quot;</p>
</td>
</tr>
<tr><td><code>modelType</code><br/>
<code>string</code>
</td>
<td>
   <p>DEPRECATED: This field is deprecated and will be removed in future releases.</p>
</td>
</tr>
<tr><td><code>version</code><br/>
<code>string</code>
</td>
<td>
   <p>Version of the model format.
Used in validating that a runtime supports a predictor.
It Can be &quot;major&quot;, &quot;major.minor&quot; or &quot;major.minor.patch&quot;.</p>
</td>
</tr>
<tr><td><code>modelFramework</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelFrameworkSpec"><code>ModelFrameworkSpec</code></a>
</td>
<td>
   <p>ModelFramework of the model, e.g., &quot;PyTorch&quot;, &quot;TensorFlow&quot;, &quot;ONNX&quot;, &quot;Transformers&quot;</p>
</td>
</tr>
<tr><td><code>modelArchitecture</code><br/>
<code>string</code>
</td>
<td>
   <p>ModelArchitecture of the model, e.g., &quot;LlamaForCausalLM&quot;, &quot;GemmaForCausalLM&quot;, &quot;MixtralForCausalLM&quot;</p>
</td>
</tr>
<tr><td><code>quantization</code><br/>
<a href="#ome-io-v1beta1-ModelQuantization"><code>ModelQuantization</code></a>
</td>
<td>
   <p>Quantization of the model, e.g., &quot;fp8&quot;, &quot;fbgemm_fp8&quot;, &quot;int4&quot;</p>
</td>
</tr>
<tr><td><code>autoSelect</code><br/>
<code>bool</code>
</td>
<td>
   <p>Set to true to allow the ServingRuntime to be used for automatic model placement if
this model format is specified with no explicit runtime.</p>
</td>
</tr>
<tr><td><code>priority</code><br/>
<code>int32</code>
</td>
<td>
   <p>Priority of this serving runtime for auto selection.
This is used to select the serving runtime if more than one serving runtime supports the same model format.
The value should be greater than zero.  The higher the value, the higher the priority.
Priority is not considered if AutoSelect is either false or not specified.
Priority can be overridden by specifying the runtime in the InferenceService.</p>
</td>
</tr>
</tbody>
</table>

## `TorchElasticPolicy`     {#ome-io-v1beta1-TorchElasticPolicy}
    

**Appears in:**

- [TorchMLPolicyConfig](#ome-io-v1beta1-TorchMLPolicyConfig)


<p>TorchElasticPolicy represents a configuration for the PyTorch elastic training.
If this policy is set, the <code>.spec.numNodes</code> parameter must be omitted, since min and max node
is used to configure the <code>torchrun</code> CLI argument: <code>--nnodes=minNodes:maxNodes</code>.
Only <code>c10d</code> backend is supported for the Rendezvous communication.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>maxRestarts</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>How many times the training job can be restarted.
This value is inserted into the <code>--max-restarts</code> argument of the <code>torchrun</code> CLI and
the <code>.spec.failurePolicy.maxRestarts</code> parameter of the training Job.</p>
</td>
</tr>
<tr><td><code>minNodes</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Lower limit for the number of nodes to which training job can scale down.</p>
</td>
</tr>
<tr><td><code>maxNodes</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Upper limit for the number of nodes to which training job can scale up.</p>
</td>
</tr>
<tr><td><code>metrics</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#metricspec-v2-autoscaling"><code>[]k8s.io/api/autoscaling/v2.MetricSpec</code></a>
</td>
<td>
   <p>Specification which are used to calculate the desired number of nodes. See the individual
metric source types for more information about how each type of metric must respond.
The HPA will be created to perform auto-scaling.</p>
</td>
</tr>
</tbody>
</table>

## `TorchMLPolicyConfig`     {#ome-io-v1beta1-TorchMLPolicyConfig}
    

**Appears in:**

- [MLPolicyConfig](#ome-io-v1beta1-MLPolicyConfig)


<p>TorchMLPolicyConfig represents a PyTorch runtime configuration.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>numProcPerNode</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Number of processes per node.
This value is inserted into the <code>--nproc-per-node</code> argument of the <code>torchrun</code> CLI.
Supported values: <code>auto</code>, <code>cpu</code>, <code>gpu</code>, or int value.
TODO (andreyvelich): Add kubebuilder validation.
Defaults to <code>auto</code>.</p>
</td>
</tr>
<tr><td><code>elasticPolicy</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TorchElasticPolicy"><code>TorchElasticPolicy</code></a>
</td>
<td>
   <p>Elastic policy for the PyTorch training.</p>
</td>
</tr>
</tbody>
</table>

## `TrainerSpec`     {#ome-io-v1beta1-TrainerSpec}
    

**Appears in:**

- [TrainingJobSpec](#ome-io-v1beta1-TrainingJobSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>image</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Docker image for the training container.</p>
</td>
</tr>
<tr><td><code>command</code> <B>[Required]</B><br/>
<code>[]string</code>
</td>
<td>
   <p>Entrypoint commands for the training container.</p>
</td>
</tr>
<tr><td><code>args</code> <B>[Required]</B><br/>
<code>[]string</code>
</td>
<td>
   <p>Arguments to the entrypoint for the training container.</p>
</td>
</tr>
<tr><td><code>env</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core"><code>[]k8s.io/api/core/v1.EnvVar</code></a>
</td>
<td>
   <p>List of environment variables to set in the training container.
These values will be merged with the TrainingRuntime's trainer environments.</p>
</td>
</tr>
<tr><td><code>numNodes</code> <B>[Required]</B><br/>
<code>int32</code>
</td>
<td>
   <p>Number of training nodes.</p>
</td>
</tr>
<tr><td><code>resourcesPerNode</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core"><code>k8s.io/api/core/v1.ResourceRequirements</code></a>
</td>
<td>
   <p>Compute resources for each training node.</p>
</td>
</tr>
<tr><td><code>numProcPerNode</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Number of processes/workers/slots on every training node.
For the Torch runtime: <code>auto</code>, <code>cpu</code>, <code>gpu</code>, or int value can be set.
For the MPI runtime only int value can be set.</p>
</td>
</tr>
</tbody>
</table>

## `TrainingJobSpec`     {#ome-io-v1beta1-TrainingJobSpec}
    

**Appears in:**

- [TrainingJob](#ome-io-v1beta1-TrainingJob)


<p>TrainingJobSpec defines the base job spec which various training job specs implement.
It defines the desired state of a training job</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>runtimeRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-RuntimeRef"><code>RuntimeRef</code></a>
</td>
<td>
   <p>Reference to the training runtime.
The field is immutable.</p>
</td>
</tr>
<tr><td><code>trainer</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-TrainerSpec"><code>TrainerSpec</code></a>
</td>
<td>
   <p>Trainer defines the trainer to use for the training job.</p>
</td>
</tr>
<tr><td><code>modelConfig</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-ModelConfig"><code>ModelConfig</code></a>
</td>
<td>
   <p>ModelConfig defines the model configuration for the training job.</p>
</td>
</tr>
<tr><td><code>datasets</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-StorageSpec"><code>StorageSpec</code></a>
</td>
<td>
   <p>Datasets defines the datasets for the training job.</p>
</td>
</tr>
<tr><td><code>hyperParameterTuningConfig</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-HyperparameterTuningConfig"><code>HyperparameterTuningConfig</code></a>
</td>
<td>
   <p>HyperParameterTuningConfig defines the hyperparameter configuration and tuning strategy</p>
</td>
</tr>
<tr><td><code>suspend</code> <B>[Required]</B><br/>
<code>bool</code>
</td>
<td>
   <p>Whether the controller should suspend the running TrainJob.
Defaults to false.</p>
</td>
</tr>
<tr><td><code>labels</code> <B>[Required]</B><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Labels to apply for the derivative JobSet and Jobs.
They will be merged with the TrainingRuntime values.</p>
</td>
</tr>
<tr><td><code>annotations</code> <B>[Required]</B><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Annotations to apply for the derivative JobSet and Jobs.
They will be merged with the TrainingRuntime values.</p>
</td>
</tr>
<tr><td><code>compartmentID</code><br/>
<code>string</code>
</td>
<td>
   <p>The compartment ID to use for the training job</p>
</td>
</tr>
</tbody>
</table>

## `TrainingJobStatus`     {#ome-io-v1beta1-TrainingJobStatus}
    

**Appears in:**

- [TrainingJob](#ome-io-v1beta1-TrainingJob)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>jobsStatus</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-JobStatus"><code>[]JobStatus</code></a>
</td>
<td>
   <p>JobsStatus tracks the child Jobs in TrainJob.</p>
</td>
</tr>
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions is an array of current observed job conditions.</p>
</td>
</tr>
<tr><td><code>retryCount</code> <B>[Required]</B><br/>
<code>int</code>
</td>
<td>
   <p>RetryCount represents the number of retries the training job has performed</p>
</td>
</tr>
<tr><td><code>startTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>StartTime represents time when the training job is acknowledged by the controller.
It is not guaranteed to be set in happens-before order across separate operations.
It is represented in RFC3339 form and is in UTC.</p>
</td>
</tr>
<tr><td><code>completionTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>CompletionTime represents time when the training job is completed. It is not guaranteed to
be set in happens-before order across separate operations.
It is represented in RFC3339 form and is in UTC.</p>
</td>
</tr>
<tr><td><code>lastReconcileTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>LastReconcileTime represents last time when the job was reconciled. It is not guaranteed to
be set in happens-before order across separate operations.
It is represented in RFC3339 form and is in UTC.</p>
</td>
</tr>
</tbody>
</table>

## `TrainingRuntimeSpec`     {#ome-io-v1beta1-TrainingRuntimeSpec}
    

**Appears in:**

- [ClusterTrainingRuntime](#ome-io-v1beta1-ClusterTrainingRuntime)

- [TrainingRuntime](#ome-io-v1beta1-TrainingRuntime)


<p>TrainingRuntimeSpec defines the desired state of TrainingRuntime</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>mlPolicy</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-MLPolicy"><code>MLPolicy</code></a>
</td>
<td>
   <p>Configuration for the model training with ML-specific parameters.</p>
</td>
</tr>
<tr><td><code>podGroupPolicy</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-PodGroupPolicy"><code>PodGroupPolicy</code></a>
</td>
<td>
   <p>Configuration for the PodGroup to enable gang-scheduling via supported plugins.</p>
</td>
</tr>
<tr><td><code>template</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-JobSetTemplateSpec"><code>JobSetTemplateSpec</code></a>
</td>
<td>
   <p>JobSet template which will be used by TrainJob.</p>
</td>
</tr>
<tr><td><code>labels</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Labels that will be added to the runtime spec.
More info: http://kubernetes.io/docs/user-guide/labels</p>
</td>
</tr>
<tr><td><code>annotations</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Annotations that will be added to the runtime spec.
More info: http://kubernetes.io/docs/user-guide/annotations</p>
</td>
</tr>
<tr><td><code>compartmentID</code><br/>
<code>string</code>
</td>
<td>
   <p>The compartment ID to use for the training runtime</p>
</td>
</tr>
</tbody>
</table>

## `TransitionStatus`     {#ome-io-v1beta1-TransitionStatus}
    
(Alias of `string`)

**Appears in:**

- [ModelStatus](#ome-io-v1beta1-ModelStatus)


<p>TransitionStatus enum</p>




## `UserSpec`     {#ome-io-v1beta1-UserSpec}
    

**Appears in:**

- [User](#ome-io-v1beta1-User)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>email</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>Email is the user's email address</p>
</td>
</tr>
<tr><td><code>projectRef</code> <B>[Required]</B><br/>
<a href="#ome-io-v1beta1-CrossReference"><code>CrossReference</code></a>
</td>
<td>
   <p>ProjectRef references the Project</p>
</td>
</tr>
<tr><td><code>role</code><br/>
<code>string</code>
</td>
<td>
   <p>Role defines the user's role in the project, owner or member</p>
</td>
</tr>
<tr><td><code>config</code><br/>
<code>map[string]string</code>
</td>
<td>
   <p>Config contains vendor-specific configuration</p>
</td>
</tr>
</tbody>
</table>

## `UserStatus`     {#ome-io-v1beta1-UserStatus}
    

**Appears in:**

- [User](#ome-io-v1beta1-User)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>userId</code> <B>[Required]</B><br/>
<code>string</code>
</td>
<td>
   <p>UserId is the platform-specific user ID</p>
</td>
</tr>
<tr><td><code>conditions</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta"><code>[]k8s.io/apimachinery/pkg/apis/meta/v1.Condition</code></a>
</td>
<td>
   <p>Conditions represent the latest available observations of an object's state</p>
</td>
</tr>
<tr><td><code>creationTime</code> <B>[Required]</B><br/>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta"><code>k8s.io/apimachinery/pkg/apis/meta/v1.Time</code></a>
</td>
<td>
   <p>CreationTime is the time when the user was created</p>
</td>
</tr>
</tbody>
</table>

## `Vendor`     {#ome-io-v1beta1-Vendor}
    
(Alias of `string`)

**Appears in:**

- [OrganizationSpec](#ome-io-v1beta1-OrganizationSpec)


<p>Vendor represents the vendors like OpenAI, Gemini or xAI</p>




## `WorkerPodSpec`     {#ome-io-v1beta1-WorkerPodSpec}
    

**Appears in:**

- [ServingRuntimeSpec](#ome-io-v1beta1-ServingRuntimeSpec)



<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>size</code><br/>
<code>int</code>
</td>
<td>
   <p>Size of the worker, this is the number of pods in the worker.</p>
</td>
</tr>
<tr><td><code>ServingRuntimePodSpec</code><br/>
<a href="#ome-io-v1beta1-ServingRuntimePodSpec"><code>ServingRuntimePodSpec</code></a>
</td>
<td>(Members of <code>ServingRuntimePodSpec</code> are embedded into this type.)
   <p>PodSpec for the worker</p>
</td>
</tr>
</tbody>
</table>

## `WorkerSpec`     {#ome-io-v1beta1-WorkerSpec}
    

**Appears in:**

- [DecoderSpec](#ome-io-v1beta1-DecoderSpec)

- [EngineSpec](#ome-io-v1beta1-EngineSpec)

- [PredictorSpec](#ome-io-v1beta1-PredictorSpec)


<p>WorkerSpec defines the configuration for worker nodes in a multi-node component
Worker nodes perform the distributed processing tasks assigned by the leader node,
enabling horizontal scaling for compute-intensive workloads.</p>


<table class="table">
<thead><tr><th width="30%">Field</th><th>Description</th></tr></thead>
<tbody>
  
<tr><td><code>PodSpec</code><br/>
<a href="#ome-io-v1beta1-PodSpec"><code>PodSpec</code></a>
</td>
<td>(Members of <code>PodSpec</code> are embedded into this type.)
   <p>PodSpec for the worker
Allows customization of the Kubernetes Pod configuration specifically for worker nodes.</p>
</td>
</tr>
<tr><td><code>size</code><br/>
<code>int</code>
</td>
<td>
   <p>Size of the worker, this is the number of pods in the worker.
Controls how many worker pod instances will be deployed for horizontal scaling.</p>
</td>
</tr>
<tr><td><code>runner</code><br/>
<a href="#ome-io-v1beta1-RunnerSpec"><code>RunnerSpec</code></a>
</td>
<td>
   <p>Runner container override for customizing the main container
This is essentially a container spec that can override the default container
Provides fine-grained control over the container that executes the worker node's processing logic.</p>
</td>
</tr>
</tbody>
</table>
  
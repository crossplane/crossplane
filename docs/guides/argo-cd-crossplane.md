---  
title: Argo CD
toc: true  
weight: 270  
indent: true  
---  

# Configuring Crossplane with Argo CD
 
## Overview
 
[Argo CD](https://argoproj.github.io/cd/) and [Crossplane](https://crossplane.io)
are a great combination. Argo CD provides GitOps while Crossplane turns any Kubernetes
cluster into a Universal Control Plane for all of your resources. There are
configuration details required in order for the two to work together properly.
This doc will help you understand these requirements.
 
Argo CD synchronizes Kubernetes resource manifests stored in a Git repository
with those running in a Kubernetes cluster (GitOps). It places a label on
the running resources to know that it should be tracking them. When it sees
that label on a running resource, it expects it to match the manifest in the
repo. Additionally, Argo CD makes use of the Kubernetes Owner Ref field to
correlate additional resources created by operators as a result of the top-level
synced resources. Owner Ref enforces a rule that a namespace scoped resource
cannot be the owner of a cluster scoped resource or a resource in another
namespace. Because of this, Argo CD cannot correlate relationships that are
structured in either of those two manners.
 
When Crossplane creates namespace scoped Composite Resource Claims (XRCs), they
result in cluster scoped Composite Resources (XRs) being created. This is a
relationship structure that Argo CD cannot correlate. One of the nice features
in Argo CD is its UI where we can visualize those owner ref relationships and
gleen information on their state. Because of the inability for a owner ref to be
defined between XRC and XR, we will not see this visualization for Crossplane
in the Argo CD UI. If we do not use XRC, and start with XR, we will see the
visualization. This is a tradeoff that needs to be considered when using the
two together.
 
Another issue when starting from XRC is introduced because Crossplane
propagates labels from XRC to XR. In this case, Argo CD synchronizes the XRC
resource manifest from the repo to the cluster. In doing so, it places its
tracking label on it. Crossplane then propagates that label to the XR. At this
point, Argo CD sees the XR with its tracking label, does not see a match in the
repo, and considers the XR to be an out of sync resource that should be removed.
If 'auto pruning' is enabled, Argo CD will delete the XR. Crossplane will see
that the XR is missing and recreate it. This will continue in a loop until
manual intervention is taken.
 
A final issue (This is not unique to Crossplane) can come into play when we
manage the Crossplane service itself from Argo CD. When there are late
initialized resources or fields as a result of an Argo CD sync, it can become
confused and report these as out of sync. Crossplane will create some late
initialized RBAC resources and fields that can cause this issue to arise.
 
 
### Configuring Argo CD with Crossplane
 
#### 1. When using Argo CD to deploy and manage the Crossplane instance:
 
This is a basic Argo CD Application config that will avoid issues with late
initialized fields:
 
```yaml
project: default
source:
 repoURL: 'https://charts.upbound.io/stable'
 targetRevision: 1.6.3-up.1
 chart: universal-crossplane
destination:
 server: 'https://kubernetes.default.svc'
 namespace: upbound-system
syncPolicy:
 automated: {}
 syncOptions:
   - CreateNamespace=true
   - ApplyOutOfSyncOnly=true
   - Validate=false
   - RespectIgnoreDifferences=true
 retry:
   limit: 2
   backoff:
     duration: 2m
     factor: 2
     maxDuration: 5m0s
ignoreDifferences:
 - group: rbac.authorization.k8s.io
   kind: ClusterRole
   name: crossplane
   jsonPointers:
     - /rules
```
 
#### 2. When using Argo CD to deploy XRC:
 
To avoid having Argo CD see the XR with the  propagated tracking label, we
must currently employ a workaround. Define a `Deny` rule within the Argo CD
project config. This is required for any XRC that is synced from the repo.
 
Within the project config, you will find the `CLUSTER RESOURCE DENY LIST`
input. It takes a `Kind` and `Group` pair input. Add an entry for each XR
Kind that will be created as a result of a synced XRC.
 
This issue is being tracked and is expected to be addressed in the near
future.

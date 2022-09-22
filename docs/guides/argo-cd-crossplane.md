---  
title: Configuring Crossplane with Argo CD
weight: 270
---  
 

[Argo CD](https://argoproj.github.io/cd/) and [Crossplane](https://crossplane.io)
are a great combination. Argo CD provides GitOps while Crossplane turns any Kubernetes
cluster into a Universal Control Plane for all of your resources. There are
configuration details required in order for the two to work together properly.
This doc will help you understand these requirements. It is recommended to use

Argo CD version 2.4.8 or later with Crossplane.
 
Argo CD synchronizes Kubernetes resource manifests stored in a Git repository
with those running in a Kubernetes cluster (GitOps). There are different ways to configure 

how Argo CD tracks resources. With Crossplane, you need to configure Argo CD 
to use Annotation based resource tracking. See the [Argo CD docs](https://argo-cd.readthedocs.io/en/latest/user-guide/resource_tracking/) for additional detail.
 
### Configuring Argo CD with Crossplane
 
To configure Argo CD for Annotation resource tracking, edit the `argocd-cm`

`ConfigMap` in the `argocd` `Namespace`. Add `application.resourceTrackingMethod: annotation`

to the data section as below:

```yaml

apiVersion: v1
data:
  application.resourceTrackingMethod: annotation
kind: ConfigMap
```

On the next Argo CD sync, Crossplane `Claims` and `Composite Resources` will

be considered synchronized and will not trigger auto-pruning.

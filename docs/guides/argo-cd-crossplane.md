---  
title: Configuring Crossplane with Argo CD
weight: 270
---  

[Argo CD](https://argoproj.github.io/cd/) and [Crossplane](https://crossplane.io)
are a great combination. Argo CD provides GitOps while Crossplane turns any Kubernetes
cluster into a Universal Control Plane for all of your resources. There are
configuration details required in order for the two to work together properly.
This doc will help you understand these requirements.

It is recommended to use Argo CD version 2.4.8 or later.

### Resource Tracking with annotations

In order for Argo CD to properly track Crossplane generated resources, tracking method must be updated to annotations:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  application.resourceTrackingMethod: annotation
```

### Resource Exclusions

By default Argo CD uses Kubernetes API discovery to watch all the resources that are part of a cluster. It is possible
to exclude certain resource from being watch an showed in the UI.

#### Decluttering the UI by excluding `ProviderConfigUsage`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  resource.exclusions: |
    - apiGroups:
      - "*"
      kinds:
      - ProviderConfigUsage
```

#### Reducing resource usages by excluding unused CRDs

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
data:
  resource.exclusions: |
    - apiGroups:
      - apigateway.aws.upbound.io
      - cloudwatch.aws.upbound.io
```

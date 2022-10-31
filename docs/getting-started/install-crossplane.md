---
toc_hide: true
---

## Install Crossplane

Crossplane installs into an existing Kubernetes cluster. 

{{< hint type="tip" >}}
If you don't have a Kubernetes cluster create one locally with [kind](https://kind.sigs.k8s.io/).
{{< /hint >}}

### Install the Crossplane Helm chart

Helm enables Crossplane to install all its Kubernetes components through a _Helm Chart_.

Enable the Crossplane Helm Chart repository:

```shell
helm repo add \
crossplane-stable https://charts.crossplane.io/stable
helm repo update
```

Helm supports a `dry-run` to see the changes Helm makes. Run the Helm dry-run to see all the Crossplane components.

```shell
helm install crossplane \
crossplane-stable/crossplane \
--dry-run --debug \
--namespace crossplane-system \
--create-namespace
```

Last, install the Crossplane components using `helm install`.

```shell
helm install crossplane \
crossplane-stable/crossplane \
--namespace crossplane-system \
--create-namespace

NAME: crossplane
LAST DEPLOYED: Sun Oct 16 19:11:27 2022
NAMESPACE: crossplane-system
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
Release: crossplane

Chart Name: crossplane
Chart Description: Crossplane is an open source Kubernetes add-on that enables platform teams to assemble infrastructure from multiple vendors, and expose higher level self-service APIs for application teams to consume.
Chart Version: 1.9.1
Chart Application Version: 1.9.1

Kube Version: v1.25.2
```

Verify Crossplane installed with `kubectl get pods`.

```shell
kubectl get pods -n crossplane-system
NAME                                      READY   STATUS    RESTARTS   AGE
crossplane-d4cd8d784-ldcgb                1/1     Running   0          54s
crossplane-rbac-manager-84769b574-6mw6f   1/1     Running   0          54s
```

Installing Crossplane creates new Kubernetes API end-points. Look at the new API end-points with `kubectl api-resources  | grep crossplane`. In a later step you use the {{< hover label="grep" line="10">}}Provider{{< /hover >}} resource install the Official Provider.

```shell  {label="grep"}
kubectl api-resources  | grep crossplane
compositeresourcedefinitions      xrd,xrds     apiextensions.crossplane.io/v1         false        CompositeResourceDefinition
compositionrevisions                           apiextensions.crossplane.io/v1alpha1   false        CompositionRevision
compositions                                   apiextensions.crossplane.io/v1         false        Composition
configurationrevisions                         pkg.crossplane.io/v1                   false        ConfigurationRevision
configurations                                 pkg.crossplane.io/v1                   false        Configuration
controllerconfigs                              pkg.crossplane.io/v1alpha1             false        ControllerConfig
locks                                          pkg.crossplane.io/v1beta1              false        Lock
providerrevisions                              pkg.crossplane.io/v1                   false        ProviderRevision
providers                                      pkg.crossplane.io/v1                   false        Provider
storeconfigs                                   secrets.crossplane.io/v1alpha1         false        StoreConfig
```

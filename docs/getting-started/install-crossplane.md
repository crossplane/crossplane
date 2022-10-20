---
toc_hide: true
---

## Install Crossplane

Crossplane installs into an existing Kubernetes cluster. 

Install Crossplane either as an upstream open source distribution or from a [CNCF certified](https://github.com/cncf/crossplane-conformance) vendor distribution.

The Crossplane community supports the upstream distribution.  
The Crossplane vendor supports their specific Crossplane distribution. 

{{< hint type="tip" >}}
If you don't have a Kubernetes cluster create one locally with [minikube](https://minikube.sigs.k8s.io/docs/start/) or [kind](https://kind.sigs.k8s.io/).
{{< /hint >}}

{{< tabs "distros" >}}

{{< tab "Community Supported" >}}
### Install the Crossplane Helm chart

Helm enables Crossplane to install all its Kubernetes components through a single _Helm Chart_.

Enable the Crossplane Helm Chart repository:

```shell
helm repo add \
crossplane-stable https://charts.crossplane.io/stable
helm repo update
```

Helm supports a `dry-run` to see the changes Helm makes. Run the Helm dry-run to see all the Crossplane components.

```shell
helm install crossplane \
--dry-run --debug \
--namespace crossplane-system \
crossplane-stable/crossplane
```

Last, install the Crossplane components using `helm install`.

```shell
helm install crossplane \
--namespace crossplane-system \
crossplane-stable/crossplane

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

{{< /tab >}}

{{< tab "Vendor Distributions" >}}

## Upbound
Upbound are the founders of Crossplane. Upbound maintains and supports an open source distribution of Crossplane called [Upbound Universal Crossplane](https://github.com/upbound/universal-crossplane) (_UXP_).

More details about installing and configuring UXP are available in the [Upbound documentation](https://docs.upbound.io)

### Install the Up command-line
The Up command-line helps manage UXP and manage Upbound user accounts. 

Download and install the Upbound `up` command-line.

```shell {copy-lines="all"}
curl -sL "https://cli.upbound.io" | sh
sudo mv up /usr/local/bin/
```

### Install Upbound Universal Crossplane
_UXP_ consists of upstream Crossplane and Upbound-specific enhancements and patches. It's [open source](https://github.com/upbound/universal-crossplane) and maintained by Upbound. 

Install UXP with the Up command-line `up uxp install` command.

```shell
up uxp install
UXP 1.9.1-up.2 installed
```

Verify all UXP pods are `Running` with `kubectl get pods -n upbound-system`. This may take up to five minutes depending on your Kubernetes cluster.

```shell {label="pods"}
kubectl get pods -n upbound-system
NAME                                        READY   STATUS    RESTARTS      AGE
crossplane-7fdfbd897c-pmrml                 1/1     Running   0             68m
crossplane-rbac-manager-7d6867bc4d-v7wpb    1/1     Running   0             68m
upbound-bootstrapper-5f47977d54-t8kvk       1/1     Running   0             68m
xgql-7c4b74c458-5bf2q                       1/1     Running   3 (67m ago)   68m
```
{{< /tab >}}

{{< /tabs >}}

Installing Crossplane creates new Kubernetes API end-points. Take a look at the new API end-points with `kubectl api-resources  | grep crossplane`. In a later step you use the {{< hover label="grep" line="10">}}Provider{{< /hover >}} resource install the Official Provider.

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

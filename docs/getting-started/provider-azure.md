---
title: Azure Quickstart 
weight: 3
---

Connect Crossplane to Microsoft Azure to create and manage cloud resources from Kubernetes with the [Upbound Azure Provider](https://marketplace.upbound.io/providers/upbound/provider-azure).

This guide walks you through the steps required to get started with the Upbound Azure Provider. This includes installing Crossplane, configuring the provider to authenticate to Azure and creating a _Managed Resource_ in Azure directly from your Kubernetes cluster.

- [Prerequisites](#prerequisites)
  - [Install the Azure provider](#install-the-azure-provider)
  - [Create a Kubernetes secret for Azure](#create-a-kubernetes-secret-for-azure)
    - [Install the Azure command-line](#install-the-azure-command-line)
    - [Create an Azure service principal](#create-an-azure-service-principal)
    - [Create a Kubernetes secret with the Azure credentials](#create-a-kubernetes-secret-with-the-azure-credentials)
  - [Create a ProviderConfig](#create-a-providerconfig)
  - [Create a managed resource](#create-a-managed-resource)
  - [Delete the managed resource](#delete-the-managed-resource)
  - [Next steps](#next-steps)

## Prerequisites
This quickstart requires:
* a Kubernetes cluster with at least 3 GB of RAM
* permissions to create pods and secrets in the Kubernetes cluster
* [Helm] version `v3.2.0` or later
* an Azure account with permissions to create an Azure [service principal](https://learn.microsoft.com/en-us/azure/active-directory/develop/app-objects-and-service-principals#service-principal-object) and an [Azure Resource Group](https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal)

{{< hint type="tip" >}}
If you don't have a Kubernetes cluster create one locally with [minikube](https://minikube.sigs.k8s.io/docs/start/) or [kind](https://kind.sigs.k8s.io/).
{{< /hint >}}


{{< hint type="note" >}}
All commands use the current `kubeconfig` context and configuration. 
{{< /hint >}}

{{< include file="install-crossplane.md" type="page" >}}

### Install the Azure provider

Install the provider into the Kubernetes cluster with a Kubernetes configuration file. 

```shell {label="provider",copy-lines="all"}
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: upbound-provider-azure
spec:
  package: xpkg.upbound.io/upbound/provider-azure:v0.16.0
EOF
```

The {{< hover label="provider" line="3">}}kind: Provider{{< /hover >}} uses the Crossplane `Provider` _Custom Resource Definition_ to connect your Kubernetes cluster to your cloud provider.  

Verify the provider installed with `kubectl get providers`. 

{{< hint type="note" >}}
It may take up to five minutes for the provider to list `HEALTHY` as `True`. 
{{< /hint >}}

```shell
kubectl get providers 
NAME                     INSTALLED   HEALTHY   PACKAGE                                          AGE
upbound-provider-azure   True        True      xpkg.upbound.io/upbound/provider-azure:v0.16.0   3m3s
```

A provider installs their own Kubernetes _Custom Resource Definitions_ (CRDs). These CRDs allow you to create Azure resources directly inside Kubernetes.

You can view the new CRDs with `kubectl get crds`. Every CRD maps to a unique Azure service Crossplane can provision and manage.

{{< hint type="tip" >}}
All the supported CRDs are also available in the [Upbound Marketplace](https://marketplace.upbound.io/providers/upbound/provider-azure/latest/crds).
{{< /hint >}}


### Create a Kubernetes secret for Azure
The provider requires credentials to create and manage Azure resources. Providers use a Kubernetes _Secret_ to connect the credentials to the provider.

First generate a Kubernetes _Secret_ from your Azure JSON file and then configure the Provider to use it.

#### Install the Azure command-line
Generating an [authentication file](https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authorization#use-file-based-authentication) requires the Azure command-line.  
Follow the documentation from Microsoft to [Download and install the Azure command-line](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli).

Log in to the Azure command-line.

```command
az login
```
#### Create an Azure service principal
Follow the Azure documentation to [find your Subscription ID](https://docs.microsoft.com/en-us/azure/azure-portal/get-subscription-tenant-id) from the Azure Portal.

Using the Azure command-line and provide your Subscription ID create a service principal and authentication file.

```shell {copy-lines="all"}
az ad sp create-for-rbac \
--sdk-auth \
--role Owner \
--scopes /subscriptions/<Subscription ID> 
```

Save your Azure JSON output as `azure-credentials.json`.

{{< hint type="note" >}}
The [Configuration](https://marketplace.upbound.io/providers/upbound/provider-azure/latest/docs/configuration) section of the Provider documentation describes other authentication methods.
{{< /hint >}}

#### Create a Kubernetes secret with the Azure credentials
A Kubernetes generic secret has a name and contents. Use {{< hover label="kube-create-secret" line="1">}}kubectl create secret{{< /hover >}} to generate the secret object named {{< hover label="kube-create-secret" line="2">}}azure-secret{{< /hover >}} in the {{< hover label="kube-create-secret" line="3">}}crossplane-system{{</ hover >}} namespace.  

<!-- vale gitlab.Substitutions = NO -->
<!-- ignore .json file name -->
Use the {{< hover label="kube-create-secret" line="4">}}--from-file={{</hover>}} argument to set the value to the contents of the  {{< hover label="kube-create-secret" line="4">}}azure-credentials.json{{< /hover >}} file.
<!-- vale gitlab.Substitutions = YES -->
```shell {label="kube-create-secret",copy-lines="all"}
kubectl create secret \
generic azure-secret \
-n crossplane-system \
--from-file=creds=./azure-credentials.json
```

View the secret with `kubectl describe secret`

{{< hint type="note" >}}
The size may be larger if there are extra blank spaces in your text file.
{{< /hint >}}

```shell
kubectl describe secret azure-secret -n crossplane-system
Name:         azure-secret
Namespace:    crossplane-system
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
creds:  629 bytes
```

### Create a ProviderConfig
A `ProviderConfig` customizes the settings of the Azure Provider.  

Apply the {{< hover label="providerconfig" line="2">}}ProviderConfig{{</ hover >}} with the command:
```yaml {label="providerconfig",copy-lines="all"}
cat <<EOF | kubectl apply -f -
apiVersion: azure.upbound.io/v1beta1
metadata:
  name: default
kind: ProviderConfig
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: azure-secret
      key: creds
EOF
```

This attaches the Azure credentials, saved as a Kubernetes secret, as a {{< hover label="providerconfig" line="9">}}secretRef{{</ hover>}}.

The {{< hover label="providerconfig" line="11">}}spec.credentials.secretRef.name{{< /hover >}} value is the name of the Kubernetes secret containing the Azure credentials in the {{< hover label="providerconfig" line="10">}}spec.credentials.secretRef.namespace{{< /hover >}}.


### Create a managed resource
A _managed resource_ is anything Crossplane creates and manages outside of the Kubernetes cluster. This creates an Azure Resource group with Crossplane. The Resource group is a _managed resource_.

{{< hint type="tip" >}}
A resource group is one of the fastest Azure resources to provision.
{{< /hint >}}

```yaml {label="xr",copy-lines="all"}
cat <<EOF | kubectl apply -f -
apiVersion: azure.upbound.io/v1beta1
kind: ResourceGroup
metadata:
  name: example-rg
spec:
  forProvider:
    location: "East US"
  providerConfigRef:
    name: default
EOF
```

Notice the {{< hover label="xr" line="2">}}apiVersion{{< /hover >}} and {{< hover label="xr" line="3">}}kind{{</hover >}} are from the `Provider's` CRDs.

The {{< hover label="xr" line="5">}}metadata.name{{< /hover >}} value is the name of the created resource group in Azure.  
This example uses the name `example-rg`.

The {{< hover label="xr" line="8">}}spec.forProvider.location{{< /hover >}} tells Azure which Azure region to use when deploying resources. The region can be any [Azure geography](https://azure.microsoft.com/en-us/explore/global-infrastructure/geographies/) code.

Use `kubectl get resourcegroup` to verify Crossplane created the resource group.

```shell
kubectl get ResourceGroup
NAME         READY   SYNCED   EXTERNAL-NAME   AGE
example-rg   True    True     example-rg      4m58s
```

### Delete the managed resource
Before shutting down your Kubernetes cluster, delete the resource group just created.

Use `kubectl delete resource-group` to remove the bucket.

```shell
kubectl delete resourcegroup example-rg
resourcegroup.azure.upbound.io "example-rg" deleted
```

### Next steps 
* Explore Azure resources that can Crossplane can configure in the [Provider CRD reference](https://marketplace.upbound.io/providers/upbound/provider-azure/latest/crds).
* Join the [Crossplane Slack](https://slack.crossplane.io/) and connect with Crossplane users and contributors.
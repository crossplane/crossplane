---
title: "Stacks Guide: Azure Setup"
toc: true
weight: 540
indent: true
---

# Stacks Guide: Azure Setup

## Table of Contents

- [Stacks Guide: Azure Setup](#stacks-guide-azure-setup)
  - [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Install the Azure Stack](#install-the-azure-stack)
    - [Validate the installation](#validate-the-installation)
  - [Configure Azure Account](#configure-azure-account)
  - [Set Up Network Configuration](#set-up-network-configuration)
    - [TL;DR](#tldr)
    - [Behind the scenes](#behind-the-scenes)
  - [Configure Resource Classes](#configure-resource-classes)
    - [TL;DR](#tldr-1)
    - [More Details](#more-details)
  - [Post Stack Installation Network Configuration](#post-stack-installation-network-configuration)
  - [Recap](#recap)
  - [Next Steps](#next-steps)

## Introduction

In this guide, we will set up an Azure provider in Crossplane so that we can
install and use the [WordPress sample stack][sample-WordPress-stack], which
depends on MySQL and Kubernetes!

Before we begin, you will need:

- Everything from the [Crossplane Stacks Guide][stacks-guide] before the cloud
  provider setup
  - The `kubectl` (v1.15+) tool installed and pointing to a Crossplane cluster
  - The [Crossplane CLI][crossplane-cli] installed
- An account on [Azure][azure]
- The [jq][jq] tool for interacting with some JSON

At the end, we will have:

- A Crossplane cluster configured to use Azure
- A typical Azure network configured to support secure connectivity between
  resources
- Support in Crossplane cluster for satisfying MySQL and Kubernetes claims
- A slightly better understanding of:
  - The way Azure is configured in Crossplane
  - The way dependencies for cloud-portable workloads are configured in
    Crossplane

We will **not** be covering the core concepts in this guide, but feel free to
check out the [Crossplane concepts document][crossplane-concepts] for that.

## Install the Azure Stack

After Crossplane has been installed, it can be extended with more functionality
by installing a [Crossplane Stack][stack-docs]! Let's install the [stack for
Microsoft Azure][stack-azure] to add support for that cloud provider.

The namespace where we install the stack, is also the one in which the provider
secret will reside. The name of this namespace is arbitrary, and we are calling
it `crossplane-system` in this guide. Let's create it:

```bash
# namespace for Azure stack and provider secret
kubectl create namespace crossplane-system
```

Now we install the Azure stack using Crossplane CLI. Since this is an
infrastructure stack, we need to specify that it's cluster-scoped by passing the
`--cluster` flag.

```bash
kubectl crossplane stack generate-install --cluster 'crossplane/stack-azure:master' stack-azure | kubectl apply --namespace crossplane-system -f -
```

The rest of this guide assumes that the Azure stack is installed within
`crossplane-system` namespace.

### Validate the installation

To check to see whether our stack installed correctly, we can look at the status
of our stack:

```bash
kubectl -n crossplane-system get stack
```

It should look something like:

```bash
NAME        READY   VERSION   AGE
stack-azure   True    0.0.2     45s
```

## Configure Azure Account

We will make use of the following services on Azure:

- Resource Group
- Azure Kubernetes Service
- Azure Database for MySQL
- Virtual Network
- Subnetwork
- Virtual Network Rule

It is essential to make sure that the Azure user credentials are configured in
Crossplane as a provider. Please follow the steps [provider
guide][azure-provider-guide] for more information.

## Set Up Network Configuration

In this section we build a simple Azure virtual network configuration, by
creating corresponding Crossplane managed resources. These resources are cluster
scoped, so don't belong to a specific namespace. This network configuration
enables resources in the WordPress stack to communicate securely. In this guide, we
will use the [sample Azure network configuration][] in the Crossplane repository.
You can read more [here][crossplane-azure-networking-docs] about network secure
connectivity configurations in Crossplane.

### TL;DR

Apply the sample network configuration resources:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/azure/network-config?ref=master
```

And you're done! You can check the status of the provisioning by running:

```bash
kubectl get -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/azure/network-config?ref=master
```

When all resources have the `Ready` condition in `True` state, the provisioning
is complete. You can now move on to the next section, or keep reading below for
more details about the managed resources that we created.

### Behind the scenes

In order to provision Azure resources, a [Resource
Group][azure-resource-group-docs] is needed to to logically group resources
together. In addition, WordPress resources map to an AKS cluster and a SQLServer
database instance. To make the database instance securely accessible from the
cluster, they both need to live within the same Virtual Network. However, a
Virtual Network is not the only Azure resource that is needed to provide
inter-resource connectivity. In general, a **Network Configuration** which
consists of a set of Virtual Networks, Subnets, VNet Rules and other resource is
required for this purpose. For more information, see [Azure resource
connectivity][azure-resource-connectivity] design document.

To inspect the resources that we created above, let's run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/azure/network-config?ref=master > network-config.yaml
```

This will save the sample network configuration resources locally in
`network-config.yaml`. Please note that the Azure parameters that are used in
these resources (like `addresPrefixes`, `location`, etc...) are arbitrarily
chosen in this solution and could be configured to implement other
[configurations][azure-network-configuration].

Below we inspect each of these resources in more details.

- **`ResourceGroup`** Represents an Azure [Resource
  Group][azure-resource-group-docs], that is used to logically group resources
  together.

  ```yaml
  ---
  apiVersion: azure.crossplane.io/v1alpha3
  kind: ResourceGroup
  metadata:
    name: sample-rg
  spec:
    name: my-cool-rg
    location: Central US
    reclaimPolicy: Delete
    providerRef:
      name: azure-provider
  ```

- **`VirtualNetwork`** Represents an Azure [Virtual
  Network][azure-virtual-network].

  ```yaml
  ---
  apiVersion: network.azure.crossplane.io/v1alpha3
  kind: VirtualNetwork
  metadata:
    name: sample-vnet
  spec:
    name: my-cool-vnet
    resourceGroupNameRef:
      name: sample-rg
    location: Central US
    properties:
      addressSpace:
        addressPrefixes:
          - 10.2.0.0/16
    reclaimPolicy: Delete
    providerRef:
      name: azure-provider
  ```

- **`Subnet`** Represents an Azure [Subnet][azure-virtual-network].

  ```yaml
  ---
  apiVersion: network.azure.crossplane.io/v1alpha3
  kind: Subnet
  metadata:
    name: sample-subnet
  spec:
    name: my-cool-subnet
    resourceGroupNameRef:
      name: sample-rg
    virtualNetworkNameRef:
      name: sample-vnet
    properties:
      addressPrefix: 10.2.0.0/24
      serviceEndpoints:
        - service: Microsoft.Sql
    reclaimPolicy: Delete
    providerRef:
      name: azure-provider
  ```

As you probably have noticed, some resources are referencing other resources in
their YAML representations. For instance for `Subnet` resource we have:

```yaml
...
    virtualNetworkNameRef:
      name: sample-vnet
...
```

Such cross resource referencing is a Crossplane feature that enables managed
resources to retrieve other resources attributes. This creates a *blocking
dependency*, preventing the dependent resource from being  created before the referred
resource is ready. In the example above, `Subnet` will be blocked until the
referred `VirtualNetwork` is created, and then it retrieves its `name`. For more
information, see [Cross Resource Referencing][].

## Configure Resource Classes

Once we have the network set up, we also need to tell Crossplane how to satisfy
WordPress's claims (that will be created when we later install the WordPress
stack) for a database and a Kubernetes cluster. The [Resource
Classes][resource-claims-and-classes-docs] serve as templates for the
corresponding resource claims.

In this guide, we will use the [sample Azure resource classes][]in Crossplane
repository.

### TL;DR

Apply the sample Azure resource classes:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/azure/resource-classes?ref=master
```

And you're done! Note that these resources do not immediately provision external
Azure resources, as they only serve as template classes.

### More Details

To inspect the resource classes that we created above, run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/azure/resource-classes?ref=master > resource-classes.yaml
```

This will save the sample resource classes YAML locally in
`resource-classes.yaml`. As mentioned above, these resource classes serve as
templates and could be configured depending on the specific needs that are
needed from the underlying resources. For instance, in the sample resources the
`SQLServerClass` has `storageGB: 25`, which will result in SQLServer databases
of size 25 once a claim is submitted for this class. In addition, it's possible
to have multiple classes defined for the same claim kind, but our sample has
defined only one class for each resource type.

Below we inspect each of these resource classes in more details:

- **`SQLServerClass`** Represents a resource that defines the blueprint for how
  a "standard" [Azure MySQL Server][azure-mysql-database] should be dynamically
  provisioned

  ```yaml
  ---
  apiVersion: database.azure.crossplane.io/v1beta1
  kind: SQLServerClass
  metadata:
    name: standard-mysql
    annotations:
      resourceclass.crossplane.io/is-default-class: "true"
  specTemplate:
    forProvider:
      administratorLogin: my-cool-login
      resourceGroupNameRef:
        name: sample-rg
      location: Central US
      sslEnforcement: Disabled
      version: "5.6"
      sku:
        tier: GeneralPurpose
        capacity: 2
        family: Gen5
      storageProfile:
        storageMB: 25600
    writeConnectionSecretsToNamespace: crossplane-system
    reclaimPolicy: Delete
    providerRef:
      name: azure-provider
  ```

- **`AKSClusterClass`** Represents a resource that serves as a template to
  create an [Azure Kubernetes Engine][azure-aks](AKS).

  ```yaml
  ---
  apiVersion: compute.azure.crossplane.io/v1alpha3
  kind: AKSClusterClass
  metadata:
    name: standard-cluster
    annotations:
      resourceclass.crossplane.io/is-default-class: "true"
  specTemplate:
    writeConnectionSecretsToNamespace: crossplane-system
    resourceGroupNameRef:
      name: sample-rg
    vnetSubnetIDRef:
      name: sample-subnet
    location: Central US
    version: "1.12.8"
    nodeCount: 1
    nodeVMSize: Standard_B2s
    dnsNamePrefix: crossplane-aks
    disableRBAC: false
    writeServicePrincipalTo:
      name: akscluster-net
      namespace: crossplane-system
    reclaimPolicy: Delete
    providerRef:
      name: azure-provider
  ```

These resources will be the default resource classes for the corresponding
claims (`resourceclass.crossplane.io/is-default-class: "true"` annotation). For
more details about resource claims and how they work, see the documentation on
[resource claims][resource-claims-and-classes-docs], and [resource class
selection].

## Post Stack Installation Network Configuration

After the WordPress stack is installed, we will need the AKS Cluster it
provisions to be able to communicate with the MySQL database it provisions. In
Azure, we can do so using a [Virtual Network Rule][azure-vnet-rule]. However,
the rule cannot be created until after the MySQLInstance claim is created and
satisfied, so we will start a short script to continually check if the database
exists, and will create the rule if so.

```bash
cat > vnet-rule.yaml <<EOF
apiVersion: database.azure.crossplane.io/v1alpha3
kind: MySQLServerVirtualNetworkRule
metadata:
  name: sample-vnet-rule
spec:
  name: my-cool-vnet-rule
  serverName: MYSQL_NAME
  resourceGroupNameRef:
    name: sample-rg
  properties:
    virtualNetworkSubnetIdRef:
      name: sample-subnet
  reclaimPolicy: Delete
  providerRef:
    name: azure-provider
EOF

cat > vnetwatch.sh <<'EOF'
#!/usr/bin/env bash

set -e
trap 'exit 1' SIGINT

echo -n "waiting for mysql endpoint..." >&2
while kubectl get mysqlservers -o yaml | grep -q 'items: \[\]'; do
  echo -n "." >&2
  sleep 5
done
echo "done" >&2

export MYSQL_NAME=$(kubectl get mysqlservers -o=jsonpath='{.items[0].metadata.name}')

sed "s/MYSQL_NAME/$MYSQL_NAME/g" vnet-rule.yaml | kubectl apply -f -

EOF

chmod +x vnetwatch.sh && ./vnetwatch.sh
```

The script should be left running in the background while we go through the rest
of the guide and install the WordPress stack.

## Recap

To recap what we've set up now in our environment:

- A Crossplane Provider resource for Azure
- A Network Configuration to have secure connectivity between resources
- An CloudSQLInstanceClass and an GKEClusterClass with the right configuration
  to use the mentioned networking setup.
- A script that will create our Virtual Network Rule when our MySQL database
  name comes available.

## Next Steps

Next we'll set up a Crossplane App Stack and use it! Head [back over to the
Stacks Guide document][stacks-guide-continue] so we can pick up where we left
off.

<!-- Links -->
[crossplane-concepts]: concepts.md
[sample-wordpress-stack]: https://github.com/crossplaneio/sample-stack-wordpress
[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli/tree/release-0.2
[crossplane-azure-networking-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#microsoft-azure
[stacks-guide]: stacks-guide.md
[provider-azure-guide]: cloud-providers/azure/azure-provider.md
[stack-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks
[stack-azure]: https://github.com/crossplaneio/stack-azure
[azure]: https://azure.microsoft.com
[azure-vnet-rule]: https://docs.microsoft.com/en-us/azure/mysql/concepts-data-access-and-security-vnet
[azure-resource-group-docs]: https://docs.microsoft.com/en-us/azure/azure-resource-manager/resource-group-overview
[stacks-guide-continue]: stacks-guide.md#install-support-for-our-application-into-crossplane
[jq]: https://stedolan.github.io/jq/
[azure-virtual-network]: https://docs.microsoft.com/en-us/azure/virtual-network/virtual-networks-overview
[azure-resource-connectivity]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#microsoft-azure
[azure-network-configuration]: https://docs.microsoft.com/en-us/azure/virtual-network/virtual-networks-using-network-configuration-file
[sample Azure resource classes]: https://github.com/crossplaneio/crossplane/tree/master/cluster/examples/workloads/kubernetes/wordpress/azure/resource-classes?ref=master
[azure-mysql-database]: https://azure.microsoft.com/en-us/services/mysql/
[azure-aks]: https://azure.microsoft.com/en-us/services/kubernetes-service/
[resource-claims-and-classes-docs]: https://github.com/crossplaneio/crossplane/blob/master/docs/concepts.md#resource-claims-and-resource-classes
[sample Azure network configuration]: https://github.com/crossplaneio/crossplane/tree/master/cluster/examples/workloads/kubernetes/wordpress/azure/network-config?ref=master
[Cross Resource Referencing]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
[resource class selection]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-simple-class-selection.md
[azure-provider-guide]: cloud-providers/azure/azure-provider.md

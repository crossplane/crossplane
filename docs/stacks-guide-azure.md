---
title: "Stacks Guide: Azure Setup"
toc: true
weight: 540
indent: true
---

# Stacks Guide: Azure Setup

## Table of Contents

1. [Introduction](#introduction)
2. [Install the Azure Stack](#install-the-azure-stack)
3. [Configure Azure Account](#configure-azure-account)
4. [Configure Crossplane Azure Provider](#configure-crossplane-azure-provider)
5. [Set Up Network Resources](#set-up-network-resources)
6. [Configure Provider Resources](#configure-provider-resources)
7. [Recap](#recap)
8. [Next Steps](#next-steps)

## Introduction

In this guide, we will set up an Azure provider in Crossplane so that we
can install and use the [WordPress sample
stack][sample-wordpress-stack], which depends on MySQL and Kubernetes!

Before you begin, you will need:

* Everything from the [Crossplane Stacks Guide][stacks-guide] before the cloud
  provider setup
  - A `kubectl` pointing to a Crossplane control cluster
  - The [Crossplane CLI][crossplane-cli] installed
* An account on [Azure][azure]
* The [jq][jq] tool for interacting with some JSON, or equivalent

At the end, we will have:

* A Crossplane control cluster configured to use Azure
* The boilerplate of an Azure-based project spun up
* Support in the control cluster for managing MySQL and Kubernetes cluster
  dependencies
* A slightly better understanding of:
  - The way cloud providers are configured in Crossplane
  - The way dependencies for cloud-portable workloads are configured in
    Crossplane

We will **not** be teaching first principles in depth. Check out the [concepts
document][crossplane-concepts] for that.

## Install the Azure Stack

After Crossplane has been installed, it can be extended with more
functionality by installing a [Crossplane Stack][stack-docs]! Let's
install the [stack for Microsoft Azure][stack-azure] to add
support for that cloud provider. We can use the [Crossplane
CLI][crossplane-cli] for this operation. Since this is an infrastructure
stack, we need to specify that it's cluster-scoped by passing the
`--cluster` flag.

To install to a specific namespace, we can use the `generate-install`
command and pipe it to `kubectl apply` instead, which gives us more
control over how the stack's installation is handled. Everything is
a Kubernetes object!

```
kubectl crossplane stack generate-install --cluster 'crossplane/stack-azure:master' stack-azure | kubectl apply --namespace crossplane-system -f -
```

If we wanted to use whatever the current namespace is, we could have
used `kubectl crossplane stack install` instead of using
`generate-install`.

We have installed the Azure stack into the `crossplane-system` namespace, but we
want to group our Azure-specific resources in their own environment namespaces.
For the purpose of this guide, we will create all Azure-specific resources in
the `azure-infra-dev` namespace, and all application-specific resources in the
`app-project1-dev` namespace. Let's create these namespaces before we get
started:

```
kubectl create namespace azure-infra-dev
kubectl create namespace app-project1-dev
```

## Configure Azure Account

We will make use of the following services on Azure:

*   Resource Group
*   AKS
*   Azure Database for MySQL
*   Virtual Network
*   Subnetwork
*   Virtual Network Rule

In order to utilize each of these services, you will need to follow the [Adding
Microsoft Azure to Crossplane guide][provider-azure-guide] to obtain
appropriate credentials in a JSON file referred to as
`crossplane-azure-provider-key.json`.

## Configure Crossplane Azure Provider

Before creating any resources, we need to create and configure a cloud provider
in Crossplane. This helps Crossplane know how to connect to the cloud provider.
All the requests from Crossplane to Azure can use that resource as their
credentials. The following command assumes that you have a
`crossplane-azure-provider-key.json` file that belongs to the account you’d like
Crossplane to use.

```
export BASE64ENCODED_AZURE_PROVIDER_CREDS=$(base64 crossplane-azure-provider-key.json | tr -d "\n")
```

Now we’ll create our `Secret` that contains the credential and `Provider`
resource that refers to that secret:

```
cat > provider.yaml <<EOF
---
# Azure Admin service account secret - used by Azure Provider
apiVersion: v1
kind: Secret
metadata:
  name: demo-provider-azure-dev
  namespace: azure-infra-dev
type: Opaque
data:
  credentials: $BASE64ENCODED_AZURE_PROVIDER_CREDS
---
# Azure Provider with service account secret reference - used to provision resources
apiVersion: azure.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: demo-azure
  namespace: azure-infra-dev
spec:
  credentialsSecretRef:
    name: demo-provider-azure-dev
    key: credentials
EOF

kubectl apply -f provider.yaml
```

The name of the `Provider` resource in the file above is `demo-azure`; we'll use
the name `demo-azure` to refer to this provider when we configure and set up
other Crossplane resources.

We also will need to use our Azure subscription id to provision some of the
resources we will need. You can set an environment variable so that you will
have access when creating resources that require it. If you have a JSON
tool like [jq][jq], you can use:

```bash
export SUBSCRIPTION_ID=$(cat crossplane-azure-provider-key.json | jq -j '.subscriptionId')
```

## Set Up Network Resources

Wordpress needs a SQL database and a Kubernetes cluster. But **those** two
resources need a private network to communicate securely. They must also be
deployed into a [Resource Group][azure-resource-group-docs], a service
that Azure uses to logically group resources together. So, we need to
set up these resources before we get to the database and Kubernetes
creation steps. Here's an example network setup:

```
cat > network.yaml <<EOF
---
# Azure Resource Group
apiVersion: azure.crossplane.io/v1alpha2
kind: ResourceGroup
metadata:
  name: demo-rg
  namespace: azure-infra-dev
spec:
  name: demo-rg
  location: Central US
  providerRef:
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
---
# Azure Virtual Network
apiVersion: network.azure.crossplane.io/v1alpha2
kind: VirtualNetwork
metadata:
  name: demo-vnet
  namespace:  azure-infra-dev
spec:
  name: demo-vnet
  resourceGroupName: demo-rg
  location: Central US
  properties:
    addressSpace:
      addressPrefixes:
        - 10.2.0.0/16
  providerRef:
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
---
# Azure Subnet
apiVersion: network.azure.crossplane.io/v1alpha2
kind: Subnet
metadata:
  name: demo-subnet
  namespace: azure-infra-dev
spec:
  name: demo-subnet
  virtualNetworkName: demo-vnet
  resourceGroupName: demo-rg
  properties:
    addressPrefix: 10.2.0.0/24
    serviceEndpoints:
      - service: Microsoft.Sql
  providerRef:
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
EOF

kubectl apply -f network.yaml
```

For more details about networking and what happens when you run this command,
see [this document with more details][crossplane-azure-networking-docs].

It should not take too long for these resources to provision. You can
check their statuses with the following command:

```
kubectl describe -f network.yaml
```

## Configure Provider Resources

Once we have the network set up, we also need to tell Crossplane how to satisfy
WordPress's claims for a database and a Kubernetes cluster. The resource classes
serve as template for the new claims we make. The following resource classes
allow the KubernetesCluster and MySQLInstance claims to be satisfied with the
network configuration we just set up:

```
cat > environment.yaml <<EOF
---
# ResourceClass that defines the blueprint for how a "standard" Azure MySQL Server
# should be dynamically provisioned
apiVersion: database.azure.crossplane.io/v1alpha2
kind: SQLServerClass
metadata:
  name: azure-mysql-standard
  namespace: azure-infra-dev
specTemplate:
  adminLoginName: myadmin
  resourceGroupName: demo-rg
  location: Central US
  sslEnforced: false
  version: "5.6"
  pricingTier:
    tier: GeneralPurpose
    vcores: 2
    family: Gen5
  storageProfile:
    storageGB: 25
    backupRetentionDays: 7
    geoRedundantBackup: false
  providerRef:
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
---
apiVersion: compute.azure.crossplane.io/v1alpha2
kind: AKSClusterClass
metadata:
  name: azure-aks-standard
  namespace: azure-infra-dev
specTemplate:
  resourceGroupName: demo-rg
  vnetSubnetID: /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/demo-rg/providers/Microsoft.Network/virtualNetworks/demo-vnet/subnets/demo-subnet
  location: Central US
  version: "1.12.8"
  nodeCount: 1
  nodeVMSize: Standard_B2s
  dnsNamePrefix: crossplane-aks
  disableRBAC: false
  writeServicePrincipalTo:
    name: akscluster-net
  providerRef:
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
EOF

kubectl apply -f environment.yaml
```

The steps that we have taken so far have been related to things that can
be shared by all resources in all namespaces of the Crossplane control
cluster. Now, we will use a namespace specific to our application, and
we'll populate it with resources that will help Crossplane know what
configuration to use to satisfy our application's resource claims.
You can use any namespace for your app's resources, but for this
tutorial we'll use the `app-project1-dev` namespace we created.

Now we need to tell Crossplane which resource classes should be used to satisfy
our claims in that app namespace. We will create portable classes that have have
reference to non-portable ones that we created earlier. In our claims, we can
refer to those portable classes directly, or label one as the default portable
class to be used in claims that do not have class reference.

For example, `MySQLInstanceClass` is a portable class that can refer to Azure's
`SQLServerClass`, which is a cloud-specific class.

```
cat > namespace.yaml <<EOF
---
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: standard-mysql
  namespace: app-project1-dev
  labels:
    default: "true"
classRef:
  kind: SQLServerClass
  apiVersion: database.azure.crossplane.io/v1alpha2
  name: azure-mysql-standard
  namespace: azure-infra-dev
---
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesClusterClass
metadata:
  name: standard-cluster
  namespace: app-project1-dev
  labels:
    default: "true"
classRef:
  kind: AKSClusterClass
  apiVersion: compute.azure.crossplane.io/v1alpha2
  name: azure-aks-standard
  namespace: azure-infra-dev
---
EOF

kubectl apply -f namespace.yaml
```

For more details about what is happening behind the scenes, read more about
[portable claims in Crossplane][portable-claims].

## Configure Network Connection

After the Wordpress stack is installed, we will need the AKS Cluster it
provisions to be able to communicate with the MySQL database it provisions. In
Azure, we can do so using a [Virtual Network Rule][azure-vnet-rule]. However,
the rule cannot be created until after the MySQLInstance claim is created and
satisfied, so we will start a short script to continually check if the database
exists, and will create the rule if so.

```bash
cat > vnet-rule.yaml <<EOF
apiVersion: database.azure.crossplane.io/v1alpha2
kind: MysqlServerVirtualNetworkRule
metadata:
  name: demo-vnet-rule
  namespace: azure-infra-dev
spec:
  name: demo-vnet-rule
  serverName: MYSQL_NAME
  resourceGroupName: demo-rg
  properties:
    virtualNetworkSubnetId: /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/demo-rg/providers/Microsoft.Network/virtualNetworks/demo-vnet/subnets/demo-subnet
  providerRef:
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
EOF

cat > vnetwatch.sh <<'EOF'
#!/usr/bin/env bash

set -e
trap 'exit 1' SIGINT

echo -n "waiting for mysql endpoint..." >&2
while kubectl -n azure-infra-dev get mysqlservers -o yaml | grep -q  'items: \[\]'; do
  echo -n "." >&2
  sleep 5
done
echo "done" >&2

export MYSQL_NAME=$(kubectl -n azure-infra-dev get mysqlservers -o=jsonpath='{.items[0].metadata.name}')

sed "s/MYSQL_NAME/$MYSQL_NAME/g" vnet-rule.yaml | kubectl apply -f -

EOF

chmod +x vnetwatch.sh && ./vnetwatch.sh
```

The script should be left running in the background while we go through
the rest of the guide and install the Wordpress stack.

## Recap

To recap what we've set up now in our environment:

* Our provider account, both on the provider side and on the Crossplane side.
* A Resource Group to provision service within.
* A Network for our AKS Cluster components.
* A Subnetwork for the AKS cluster to use in the network.
* A GlobalAddress resource for Google’s service connection.
* An AKSClusterClass and a SQLServerClass with the right configuration to use
  the mentioned networking setup.
* A script that will create our Virtual Network Rule when our MySQL database
  name comes available.
* A namespace for our app resources to reside with default MySQLInstanceClass
  and KubernetesClusterClass that refer to our AKSClusterClass and
  SQLServerClass.

## Next Steps

Next we'll set up a Crossplane App Stack and use it! Head [back over to the
Stacks Guide document][stacks-guide-continue] so we can pick up where we left
off.

<!-- Links -->
[sample-wordpress-stack]: https://github.com/crossplaneio/sample-stack-wordpress

[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli/tree/release-0.1
[crossplane-azure-networking-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#microsoft-azure
[stacks-guide]: stacks-guide.md
[provider-azure-guide]: cloud-providers/azure/azure-provider.md

[stack-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks
[stack-azure]: https://github.com/crossplaneio/stack-azure

[crossplane-concepts]: concepts.md
[portable-classes-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-default-resource-class.md

[azure]: https://azure.microsoft.com
[azure-vnet-rule]: https://docs.microsoft.com/en-us/azure/mysql/concepts-data-access-and-security-vnet
[azure-resource-group-docs]: https://docs.microsoft.com/en-us/azure/azure-resource-manager/resource-group-overview

[stacks-guide-continue]: stacks-guide.md#install-support-for-our-application-into-crossplane
[jq]: https://stedolan.github.io/jq/

---
title: "Stacks Guide: GCP Setup"
toc: true
weight: 520
indent: true
---

# Stacks Guide: GCP Setup

## Table of Contents

- [Stacks Guide: GCP Setup](#stacks-guide-gcp-setup)
  - [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Install the GCP Stack](#install-the-gcp-stack)
  - [Configure GCP Account](#configure-gcp-account)
    - [Set up cloud provider credentials](#set-up-cloud-provider-credentials)
  - [Configure Crossplane Provider for GCP](#configure-crossplane-provider-for-gcp)
  - [Set Up Network Configuration](#set-up-network-configuration)
    - [TL;DR](#tldr)
    - [Behind the scenes](#behind-the-scenes)
  - [Configure Resources Classes](#configure-resources-classes)
    - [TL;DR](#tldr-1)
    - [More Details](#more-details)
  - [Recap](#recap)
  - [Next Steps](#next-steps)

## Introduction

In this guide, we will set up a GCP provider in Crossplane so that we can
install and use the [WordPress sample stack][sample-wordpress-stack], which
depends on MySQL and Kubernetes!

Before we begin, you will need:

- Everything from the [Crossplane Stacks Guide][stacks-guide] before the
  cloud provider setup
  - The `kubectl` (v1.15+) tool installed and pointing to a Crossplane cluster
  - The [Crossplane CLI][crossplane-cli] installed
- An account on [Google Cloud Platform][gcp]

At the end, we will have:

- A Crossplane control cluster configured to use GCP
- A typical GCP network configured to support secure connectivity between
  resources
- Support in Crossplane cluster for satisfying MySQL and Kubernetes claims
- A slightly better understanding of:
  - The way GCP is configured in Crossplane
  - The way dependencies for cloud-portable workloads are configured in
    Crossplane

We will **not** be covering the core concepts in this guide, but feel free to
check out the [Crossplane concepts document][crossplane-concepts] for that.

## Install the GCP Stack

After Crossplane has been installed, it can be extended with more
functionality by installing a [Crossplane Stack][stack-docs]! Let's
install the [stack for Google Cloud Platform][stack-gcp] (GCP) to add
support for that cloud provider.

The namespace where we install the stack, is also the one in which the provider
secret will reside. The name of this namespace is arbitrary, and we are calling
it `crossplane-system` in this guide. Let's create it:

```bash
# namespace for GCP stack and provider secret
kubectl create namespace crossplane-system
```

Now we install the GCP stack using Crossplane CLI. Since this is an
infrastructure stack, we need to specify that it's cluster-scoped by passing the
`--cluster` flag.

```bash
kubectl crossplane stack generate-install --cluster 'crossplane/stack-gcp:v0.2.0' stack-gcp | kubectl apply --namespace crossplane-system -f -
```

The rest of this guide assumes that the GCP stack is installed within
`crossplane-system` namespace.

To check to see whether our stack installed correctly, we can look at
the status of our stack:

```bash
kubectl -n crossplane-system get stack
```

It should look something like:

```bash
NAME        READY   VERSION   AGE
stack-gcp   True    0.0.2     5m19s
```

## Configure GCP Account

We will make use of the following services on GCP:

- GKE
- CloudSQL Instance
- Network
- Subnetwork
- GlobalAddress
- Private Service Connection

For all these to work, you need to enable the following [APIs][gcp-enable-apis]
in your GCP project:

- Compute Engine API
- Service Networking API
- Kubernetes Engine API

We will also need to tell Crossplane how to use the credentials for the GCP
account. For this exercise, the GCP account that we will tell Crossplane about
should have the following [roles][gcp-assign-roles] assigned:

- Cloud SQL Admin
- Compute Network Admin
- Kubernetes Engine Admin
- Service Account User

### Set up cloud provider credentials

It is essential to make sure that the GCP user credentials are configured in
Crossplane as a provider. Please follow the steps in the GCP [provider
guide][gcp-provider-guide] for more information.

## Set Up Network Configuration

In this section we build a simple GCP network configuration, by creating
corresponding Crossplane managed resources. These resources are cluster scoped,
so don't belong to a specific namespace. This network configuration enables
resources in the WordPress stack to communicate securely. In this guide, we will use
the [sample GCP network configuration][] in the Crossplane repository. You can read
more [here][crossplane-gcp-networking-docs] about network secure connectivity
configurations in Crossplane.

### TL;DR

Apply the sample network configuration resources:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/network-config?ref=release-0.4
```

And you're done! You can check the status of the provisioning by running:

```bash
kubectl get -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/network-config?ref=release-0.4
```

When all resources have the `Ready` condition in `True` state, the provisioning
is complete. You can now move on to the next section, or keep reading below for
more details about the managed resources that we created.

### Behind the scenes

WordPress needs a MySQL database and a Kubernetes cluster. But these
two resources need a private network to communicate securely. So, we
need to set up the network before we set up the database and the
Kubernetes cluster.

To inspect the resources that we created above, let's run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/network-config?ref=release-0.4 > network-config.yaml
```

This will save the sample network configuration resources locally in
`network-config.yaml`. Please note that the GCP parameters that are used in these
resources (like `ipCidrRange`, `region`, etc...) are arbitrarily chosen in this
solution and could be configured to implement other
[configurations][gcp-network-configuration].

Below we inspect each of these resources in more details.

- **`Network`** Represents a GCP [Virtual Private Cloud (VPC)
  Network][gcp-network-configuration], that all cloud instances we'll create
  will use.

  ```yaml
  ---
  apiVersion: compute.gcp.crossplane.io/v1alpha3
  kind: Network
  metadata:
    name: sample-network
  spec:
    name: my-cool-network
    autoCreateSubnetworks: false
    routingConfig:
      routingMode: REGIONAL
    reclaimPolicy: Delete
    providerRef:
      name: gcp-provider
  ```

- **`Subnetwork`** Represents a GCP [Virtual Private Cloud Subnetwork][gcp-network-configuration], which
  defines IP ranges to be used by GKE cluster.

  ```yaml
  ---
  apiVersion: compute.gcp.crossplane.io/v1alpha3
  kind: Subnetwork
  metadata:
    name: sample-subnetwork
  spec:
    name: my-cool-subnetwork
    region: us-central1
    ipCidrRange: "192.168.0.0/24"
    privateIpGoogleAccess: true
    secondaryIpRanges:
      - rangeName: pods
        ipCidrRange: 10.0.0.0/8
      - rangeName: services
        ipCidrRange: 172.16.0.0/16
    networkRef:
      name: sample-network
    reclaimPolicy: Delete
    providerRef:
      name: gcp-provider
  ```

- **`GlobalAddress`** Represents a GCP [Global Address][gcp-ip-address], which defines the IP
  range that will be allocated for cloud services connecting to the instances in the given Network.

  ```yaml
  ---
  apiVersion: compute.gcp.crossplane.io/v1alpha3
  kind: GlobalAddress
  metadata:
    name: sample-globaladdress
  spec:
    name: my-cool-globaladdress
    purpose: VPC_PEERING
    addressType: INTERNAL
    prefixLength: 16
    networkRef:
      name: sample-network
    reclaimPolicy: Delete
    providerRef:
      name: gcp-provider
  ```

- **`Connection`** Represents a GCP [Connection][gcp-connection], which allows
  cloud services to use the allocated GlobalAddress for communication. Behind
  the scenes, it creates a VPC peering to the network that those service
  instances actually live.

  ```yaml
  ---
  apiVersion: servicenetworking.gcp.crossplane.io/v1alpha3
  kind: Connection
  metadata:
    name: sample-connection
  spec:
    parent: services/servicenetworking.googleapis.com
    networkRef:
      name: sample-network
    reservedPeeringRangeRefs:
      - name: sample-globaladdress
    reclaimPolicy: Delete
    providerRef:
      name: gcp-provider
  ```

As you probably have noticed, some resources are referencing other resources
 in their YAML representations. For instance for `Subnetwork` resource we have:

```yaml
...
    networkRef:
      name: sample-network
...
```

Such cross resource referencing is a Crossplane feature that enables managed
resources to retrieve other resources attributes. This creates a *blocking
dependency*, preventing the dependent resource from being  created before the referred
resource is ready. In the example above, `Subnetwork` will be blocked until the
referred `Network` is created, and then it retrieves its id. For more
information, see [Cross Resource Referencing][].

## Configure Resources Classes

Once we have the network configuration set up, we need to tell Crossplane how to
satisfy WordPress's claims (that will be created when we later install the
WordPress stack) for a database and a Kubernetes cluster. The resource classes
serve as templates for the corresponding resource claims. For more information,
refer to [Resource Classes][resource-claims-and-classes-docs] design document.

In this guide, we will use the [sample GCP resource classes] in Crossplane
repository.

### TL;DR

Apply the sample GCP resource classes:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/resource-classes?ref=release-0.4
```

And you're done! Note that these resources do not immediately provision external GCP resourcs.

### More Details

To inspect the resource classes that we created above, run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/resource-classes?ref=release-0.4 > resource-classes.yaml
```

This will save the sample resource classes YAML locally in
`resource-classes.yaml`. As mentioned above, these resource classes serve as
templates and could be configured depending on the specific needs that are
needed from the underlying resources. For instance, in the sample resources the
`CloudSQLInstanceClass` has `storageGB: 10`, which will result in databases of
size 10GB once a claim is submitted for this class. In addition, it's possible
to have multiple classes defined for the same claim kind, but our sample has
defined only one class for each resource type.

Below we inspect each of these resource classes in more details:

- **`CloudSQLInstanceClass`** Represents a resource that serves as a template to
  create a [Cloud SQL Database Instance][gcp-cloudsql].

  ```yaml
  ---
  apiVersion: database.gcp.crossplane.io/v1beta1
  kind: CloudSQLInstanceClass
  metadata:
    name: standard-mysql
    annotations:
      resourceclass.crossplane.io/is-default-class: "true"
  specTemplate:
    writeConnectionSecretsToNamespace: crossplane-system
    forProvider:
      databaseVersion: MYSQL_5_7
      region: us-central1
      settings:
        tier: db-n1-standard-1
        dataDiskType: PD_SSD
        dataDiskSizeGb: 10
        ipConfiguration:
          privateNetworkRef:
            name: sample-network
    reclaimPolicy: Delete
    providerRef:
      name: gcp-provider
  ```

- **`GKEClusterClass`** Represents a resource that serves as a template to
  create a [Kubernetes Engine][gcp-gke] (GKE).

  ```yaml
  ---
  apiVersion: compute.gcp.crossplane.io/v1alpha3
  kind: GKEClusterClass
  metadata:
    name: standard-cluster
    annotations:
      resourceclass.crossplane.io/is-default-class: "true"
  specTemplate:
    machineType: n1-standard-1
    numNodes: 1
    zone: us-central1-b
    networkRef:
      name: sample-network
    subnetworkRef:
      name: sample-subnetwork
    enableIPAlias: true
    clusterSecondaryRangeName: pods
    servicesSecondaryRangeName: services
    reclaimPolicy: Delete
    providerRef:
      name: gcp-provider
  ```

These resources will be the default resource classes for the corresponding
claims (`resourceclass.crossplane.io/is-default-class: "true"` annotation). For
more details about resource claims and how they work, see the documentation on
[resource claims][resource-claims-and-classes-docs], and [resource class selection].

## Recap

To recap what we've set up now in our environment:

- A Crossplane Provider resource for GCP
- A Network Configuration to have secure connectivity between resources
- An CloudSQLInstanceClass and an GKEClusterClass with the right configuration to use
  the mentioned networking setup.

## Next Steps

Next we'll set up a Crossplane App Stack and use it! Head [back over to
the Stacks Guide document][stacks-guide-continue] so we can pick up
where we left off.

<!-- Links -->
[crossplane-concepts]: concepts.md
[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli/tree/release-0.2
[crossplane-gcp-networking-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#google-cloud-platform
[stacks-guide]: https://github.com/crossplaneio/crossplane/blob/master/docs/stacks-guide.md
[gcp-credentials]: https://github.com/crossplaneio/crossplane/blob/master/cluster/examples/gcp-credentials.sh
[gcp-enable-apis]: https://cloud.google.com/endpoints/docs/openapi/enable-api
[gcp-assign-roles]: https://cloud.google.com/iam/docs/granting-roles-to-service-accounts
[gcp]: https://cloud.google.com/
[stacks-guide-continue]: https://github.com/crossplaneio/crossplane/blob/master/docs/stacks-guide.md#install-support-for-our-application-into-crossplane
[sample-wordpress-stack]: https://github.com/crossplaneio/sample-stack-wordpress
[stack-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks
[stack-gcp]: https://github.com/crossplaneio/stack-gcp
[resource-claims-and-classes-docs]: https://github.com/crossplaneio/crossplane/blob/master/docs/concepts.md#resource-claims-and-resource-classes
[cloud-provider-setup-gcp]: https://github.com/crossplaneio/crossplane/blob/master/docs/cloud-providers/gcp/gcp-provider.md
[gcp-network-configuration]: https://cloud.google.com/vpc/docs/vpc
[Cross Resource Referencing]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
[sample GCP resource classes]: https://github.com/crossplaneio/crossplane/tree/master/cluster/examples/workloads/kubernetes/wordpress/gcp/resource-classes?ref=release-0.4
[gcp-cloudsql]: https://cloud.google.com/sql/
[gcp-gke]: https://cloud.google.com/kubernetes-engine/
[sample GCP network configuration]: https://github.com/crossplaneio/crossplane/tree/master/cluster/examples/workloads/kubernetes/wordpress/gcp/network-config?ref=release-0.4
[gcp-ip-address]: https://cloud.google.com/compute/docs/ip-addresses/
[gcp-connection]: https://cloud.google.com/vpc/docs/configure-private-services-access
[resource class selection]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-simple-class-selection.md
[gcp-provider-guide]: cloud-providers/gcp/gcp-provider.md

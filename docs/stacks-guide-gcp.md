---
title: "Stacks Guide: GCP Setup"
toc: true
weight: 520
indent: true
---

# Stacks Guide: GCP Setup

## Table of Contents

  1. [Introduction](#introduction)
  1. [Install the GCP stack](#install-the-gcp-stack)
  1. [Configure the GCP account](#configure-the-aws-account)
  1. [Configure Crossplane Provider for GCP](#configure-crossplane-provider-for-GCP)
  1. [Set Up Network Configuration](#set-up-network-configuration)
  1. [Configure Cloud-Specific Resource Classes](#configure-cloud-specific-resource-classes)
  1. [Recap](#recap)
  1. [Next Steps](#next-steps)

## Introduction

In this guide, we will set up a GCP provider in Crossplane so that we can
install and use the [WordPress sample stack][sample-wordpress-stack], which
depends on MySQL and Kubernetes!

Before we begin, you will need:

* Everything from the [Crossplane Stacks Guide][stacks-guide] before the
  cloud provider setup
  * A `kubectl v1.15+` pointing to a Crossplane cluster
  * The [Crossplane CLI][crossplane-cli] installed
* An account on [Google Cloud Platform][gcp]

At the end, we will have:

* A Crossplane control cluster configured to use GCP
* A typical GCP network configured to support secure connectivity between
  resources
* Support in Crossplane cluster for satisfying MySQL and Kubernetes claims
* A slightly better understanding of:
  * The way GCP is configured in Crossplane
  * The way dependencies for cloud-portable workloads are configured in
    Crossplane

We will **not** be teaching first principles in depth. Check out the
[Crossplane concepts document][crossplane-concepts] for that.

## Install the GCP Stack

After Crossplane has been installed, it can be extended with more
functionality by installing a [Crossplane Stack][stack-docs]! Let's
install the [stack for Google Cloud Platform][stack-gcp] (GCP) to add
support for that cloud provider.

The namespace where we install the stack, is also the one that the provider
secret will reside. The name of this namespace is arbitrary, and we are calling
it `crossplane-system` in this guide. Let's create it:

```bash
# namespace for GCP stack and provider secret
kubectl create namespace crossplane-system
```

Now we install the AWS stack using Crossplane CLI. Since this is an
infrastructure stack, we need to specify that it's cluster-scoped by passing the
`--cluster` flag.

```bash
kubectl crossplane stack generate-install --cluster 'crossplane/stack-gcp:master' stack-gcp | kubectl apply --namespace crossplane-system -f -
```

The rest of this guide assumes that the AWS stack is installed within
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

* GKE
* CloudSQL Instance
* Network
* Subnetwork
* GlobalAddress
* Private Service Connection

For all these to work, you need to enable the following [APIs][gcp-enable-apis]
in your GCP project:

* Compute Engine API
* Service Networking API
* Kubernetes Engine API

We will also need to tell Crossplane how to use the credentials for the GCP
account. For this exercise, the GCP account that we will tell Crossplane about
should have the following [roles][gcp-assign-roles] assigned:

* Cloud SQL Admin
* Compute Network Admin
* Kubernetes Engine Admin
* Service Account User

### Set up cloud provider credentials

This guide assumes that you have created a JSON file which contains the
credentials for the cloud provider. In later sections, this file will be
referred to as `crossplane-gcp-provider-key.json`. There are quite a few steps
involved, so the steps are in a separate [document][cloud-provider-setup-gcp]
which you should take a look at before moving on to the next section.
Alternatively, you could use the [script][gcp-credentials] in Crossplane
repo which helps with creating the file.

## Configure Crossplane Provider for GCP

Before creating any resources, we need to create and configure a cloud provider
in Crossplane. This helps Crossplane know how to connect to the cloud provider.
All the requests from Crossplane to GCP will use the credentials attached to the
provider object. The following command assumes that you have a
`crossplane-gcp-provider-key.json` file that belongs to the account that will be
used by Crossplane, which has GCP project id. You should be
able to get the project id from the JSON credentials file or from the GCP
console. Without loss of generality, let's assume the project id is
`crossplane-playground` in this guide.

First, let's encode the credential file contents and put it in a variable:

```bash
# base64 encode the gcp credentials
BASE64ENCODED_GCP_PROVIDER_CREDS=$(base64 crossplane-gcp-provider-key.json | tr -d "\n")
```

Now we’ll create the `Secret` resource that contains the credential, and
 `Provider` resource which refers to that secret:

```bash
cat > provider.yaml <<EOF
---
apiVersion: v1
data:
  credentials.json: $BASE64ENCODED_GCP_PROVIDER_CREDS
kind: Secret
metadata:
  name: gcp-account-creds
  namespace: crossplane-system
type: Opaque
---
apiVersion: gcp.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: gcp-provider
spec:
  credentialsSecretRef:
    key: credentials.json
    name: gcp-account-creds
    namespace: crossplane-system
  projectID: crossplane-playground
EOF

# apply it to the cluster:
kubectl apply -f "provider.yaml"

# delete the credentials variable
unset BASE64ENCODED_GCP_PROVIDER_CREDS
```

The name of the `Provider` resource in the file above is `gcp-provider`; we'll
use the name `gcp-provider` to refer to this provider when we configure and set
up other Crossplane resources.

To check on our newly created provider, we can run:

```bash
kubectl -n crossplane-system get provider.gcp.crossplane.io
```

The output should look something like:

```bash
NAME           PROJECT-ID              AGE
gcp-provider   crossplane-playground   37s
```

## Set Up Network Configuration

In this section we build a simple GCP network configuration, by creating
corresponding Crossplane managed resources. These resources are cluster scoped,
so don't belong to a specific namespace. This network configuration enables
resources in WordPress stack to communicate securely. In this guide, we will use
the [sample GCP network configuration] in Crossplane repository.

### TL;DR

Apply the sample network configuration resources:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/network-config?ref=v0.4.0
```

And you're done! You can check the status of the provisioning by running:

```bash
kubectl get -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/network-config?ref=v0.4.0
```

When all resources have the `Ready` condition in `True` state, the provisioning
is completed. You can now move on to the next section, or keep reading below for
more details about the managed resources that we created.

### Behind the scenes

WordPress needs a MySQL database and a Kubernetes cluster. But these
two resources need a private network to communicate securely. So, we
need to set up the network before we set up the database and the
Kubernetes cluster.

To inspect the resources that we created above, let's run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/gcp/network-config?ref=v0.4.0 > network-config.yaml
```

This will save the sample network configuration resources locally in
`network-config.yaml`. Please note that the GCP parameters that are used in these
resources (like `ipCidrRange`, `region`, etc...) are arbitrarily chosen in this
solution and could be configured to implement other
[configurations][crossplane-gcp-networking-docs].

Below we inspect each of these resources in more details.

* **`Network`** Represents a GCP [Virtual Network][], that all cloud instances
  we'll create will use.
    
  ```yaml
  ---
  apiVersion: compute.gcp.crossplane.io/v1alpha2
  kind: Network
  metadata:
    name: example-network
  spec:
    name: example-network
    autoCreateSubnetworks: false
    reclaimPolicy: Delete
    routingConfig:
      routingMode: REGIONAL
    providerRef:
      name: gcp-provider
  ```

* **`Subnetwork`** Represents a GCP[Virtual Subnetwork][], which defines IP
  ranges to be used by GKE cluster.

  ```yaml
  ---
  apiVersion: compute.gcp.crossplane.io/v1alpha2
  kind: Subnetwork
  metadata:
    name: example-subnetwork
  spec:
    reclaimPolicy: Delete
    name: example-subnetwork
    region: us-central1
    ipCidrRange: "192.168.0.0/24"
    privateIpGoogleAccess: true
    secondaryIpRanges:
      - rangeName: pods
        ipCidrRange: 10.0.0.0/8
      - rangeName: services
        ipCidrRange: 172.16.0.0/16
    networkRef:
      name: example-network
    providerRef:
      name: gcp-provider
  ```

* **`GlobalAddress`** Represents a GCP [Global Address][], which defines the IP
  range that will be allocated for cloud services connecting to the instances in the given Network.

  ```yaml
  ---
  apiVersion: compute.gcp.crossplane.io/v1alpha2
  kind: GlobalAddress
  metadata:
    name: example-globaladdress
  spec:
    reclaimPolicy: Delete
    name: example-globaladdress
    purpose: VPC_PEERING
    addressType: INTERNAL
    prefixLength: 16
    networkRef:
      name: example-network
    providerRef:
      name: gcp-provider
  ```

* **`Connection`** Represents a GCP [Connection][gcp-connection], which allows
  cloud services to use the allocated GlobalAddress for communication. Behind
  the scenes, it creates a VPC peering to the network that those service
  instances actually live.

  ```yaml
  ---
  apiVersion: servicenetworking.gcp.crossplane.io/v1alpha2
  kind: Connection
  metadata:
    name: example-connection
  spec:
    reclaimPolicy: Delete
    parent: services/servicenetworking.googleapis.com
    networkRef:
      name: example-network
    reservedPeeringRanges:
      - example-globaladdress
    providerRef:
      name: gcp-provider
  ```

It takes a while to create these resources in GCP. The top-level object
is the `Connection` object; when the `Connection` is ready, everything
else is ready too.

As you probably have noticed, some resources are referencing to `Network` resource
 in their YAML representations. For instance for `Subnetwork` resource we have:

```yaml
...
    networkRef:
      name: example-network
...
```

Such cross resource referencing is a Crossplane feature that enables managed
resources to retrieve other resources attributes. This creates a *blocking
dependency*, avoiding the dependent resource to be created before the referred
resource is ready. In the example above, `Subnetwork` will be blocked until the
referred `Network` is created, and then it retrieves its id. For more
information, see [Cross Resource Referencing][].

## Configure Provider Resources

Once we have the network set up, we also need to tell Crossplane how to
satisfy WordPress's claims for a database and a Kubernetes cluster.
[Resource classes][resource-classes-docs] serve as templates for the new
claims we make. The following resource classes allow the claims for the
database and Kubernetes cluster to be satisfied with the network
configuration we just set up:

```
cat > environment.yaml <<EOF
---
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudsqlInstanceClass
metadata:
  name: standard-cloudsql
  namespace: gcp
specTemplate:
  forProvider:
    databaseVersion: MYSQL_5_7
    region: us-central1
    settings:
      tier: db-n1-standard-1
      dataDiskType: PD_SSD
      dataDiskSizeGb: 10
      # Note from GCP Docs: Your Cloud SQL instances are not created in your VPC network.
      # They are created in the service producer network (a VPC network internal to Google) that is then connected (peered) to your VPC network.
      ipConfiguration:
        privateNetwork: projects/$PROJECT_ID/global/networks/example-network
  providerRef:
    name: gcp-provider
    namespace: gcp
  reclaimPolicy: Delete
---
apiVersion: compute.gcp.crossplane.io/v1alpha2
kind: GKEClusterClass
metadata:
  name: standard-gke
  namespace: gcp
specTemplate:
  machineType: n1-standard-1
  numNodes: 1
  zone: us-central1-b
  network: projects/$PROJECT_ID/global/networks/example-network
  subnetwork: projects/$PROJECT_ID/regions/us-central1/subnetworks/example-subnetwork
  enableIPAlias: true
  clusterSecondaryRangeName: pods
  servicesSecondaryRangeName: services
  providerRef:
    name: gcp-provider
    namespace: gcp
  reclaimPolicy: Delete
EOF

kubectl apply -f environment.yaml
```

The example YAML also exists in [the Crossplane
repository][crossplane-sample-gcp-environment].

We don't need to validate that these are ready, because they don't
require any reconciliation.

The steps that we have taken so far have been related to things that can
be shared by all resources in all namespaces of the Crossplane control
cluster. Now, we will use a namespace specific to our application, and
we'll populate it with resources that will help Crossplane know what
configuration to use to satisfy our application's resource claims.
If you have been following along with the rest of the stacks guide, the
namespace should already be created. But in case it isn't, this is what
you would run to create it:

```
kubectl create namespace app-project1-dev
```

Now that we have a namespace, we need to tell Crossplane which resource
classes should be used to satisfy our claims in that namespace. We will
create [portable classes][portable-classes-docs] that have have
references to the cloud-specific classes that we created earlier.

For example, `MySQLInstanceClass` is a portable class. It may refer to
GCP's `CloudSQLInstanceClass`, which is a non-portable class.

To read more about portable classes, how they work, and how to use them
in different ways, including by specifying default classes when no
reference is provided, see the [portable classes and claims
documentation][portable-classes-docs].

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
  kind: CloudsqlInstanceClass
  apiVersion: database.gcp.crossplane.io/v1alpha2
  name: standard-cloudsql
  namespace: gcp
---
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesClusterClass
metadata:
  name: standard-cluster
  namespace: app-project1-dev
  labels:
    default: "true"
classRef:
  kind: GKEClusterClass
  apiVersion: compute.gcp.crossplane.io/v1alpha2
  name: standard-gke
  namespace: gcp
---
EOF

kubectl apply -f namespace.yaml
```

The example YAML also exists in [the Crossplane
repository][crossplane-sample-gcp-namespace].

We don't need to validate that these are ready, because they don't need
to reconcile for them to be ready.

## Recap

To recap what we've set up now in our environment:

* Our provider account, both on the provider side and on the Crossplane
  side.
* A Network for all instances to share.
* A Subnetwork for the GKE cluster to use in the network.
* A GlobalAddress resource for Google’s service connection.
* A Connection resource that connects Google’s service network to ours
  in order to connect CloudSQL instance in Google’s network with GKE
  cluster in our network.
* A GKEClusterClass and a CloudSQLInstanceClass with the right
  configuration to use the mentioned networking setup.
* A namespace for our app resources to reside with default MySQLInstanceClass
  and KubernetesClusterClass that refer to our GKEClusterClass and CloudSQLInstanceClass.

## Next Steps

Next we'll set up a Crossplane App Stack and use it! Head [back over to
the Stacks Guide document][stacks-guide-continue] so we can pick up
where we left off.

<!-- Links -->
[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli/tree/release-0.1
[crossplane-gcp-networking-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#google-cloud-platform
[stacks-guide]: stacks-guide.md

[crossplane-concepts]: concepts.md

[gcp-credentials]: https://github.com/crossplaneio/crossplane/blob/master/cluster/examples/gcp-credentials.sh
[gcp-enable-apis]: https://cloud.google.com/endpoints/docs/openapi/enable-api
[gcp-assign-roles]: https://cloud.google.com/iam/docs/granting-roles-to-service-accounts
[gcp-create-keys]: https://cloud.google.com/sdk/gcloud/reference/iam/service-accounts/keys/create

[gcp]: https://cloud.google.com/

[stacks-guide-continue]: stacks-guide.md#install-support-for-our-application-into-crossplane
[sample-wordpress-stack]: https://github.com/crossplaneio/sample-stack-wordpress
[stack-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks

[stack-gcp]: https://github.com/crossplaneio/stack-gcp

[resource-claims-docs]: concepts.md#resource-claims-and-resource-classes
[resource-classes-docs]: concepts.md#resource-claims-and-resource-classes
[portable-classes-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-default-resource-class.md

[crossplane-sample-gcp-provider]: https://github.com/crossplaneio/crossplane/blob/master/cluster/examples/workloads/kubernetes/wordpress/gcp/provider.yaml
[crossplane-sample-gcp-network]: https://github.com/crossplaneio/crossplane/blob/master/cluster/examples/workloads/kubernetes/wordpress/gcp/network.yaml
[crossplane-sample-gcp-environment]: https://github.com/crossplaneio/crossplane/blob/master/cluster/examples/workloads/kubernetes/wordpress/gcp/environment.yaml
[crossplane-sample-gcp-namespace]: https://github.com/crossplaneio/crossplane/blob/master/cluster/examples/workloads/kubernetes/wordpress/gcp/namespace.yaml

[cloud-provider-setup-gcp]: cloud-providers/gcp/gcp-provider.md
[gcp-network-configuration]: https://cloud.google.com/vpc/docs/vpc
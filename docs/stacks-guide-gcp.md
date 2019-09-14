---
title: "Crossplane Stacks Guide: GCP Setup"
toc: true
weight: 332
indent: true
---

# Crossplane Stacks Guide: GCP Setup

## Table of Contents

1. [Introduction](#introduction)
2. [Install the GCP Stack](#install-the-gcp-stack)
3. [Configure GCP Account](#configure-gcp-account)
4. [Configure Crossplane GCP Provider](#configure-crossplane-gcp-provider)
5. [Set Up Network Resources](#set-up-network-resources)
6. [Configure Provider Resources](#configure-provider-resources)
7. [Recap](#recap)
8. [Next Steps](#next-steps)

## Introduction

In this guide, we will set up a GCP provider in Crossplane so that we
can install and use the WordPress sample stack, which depends on MySQL
and Kubernetes!

Before you begin, you will need:

* Everything from the [Crossplane Stacks Guide][stacks-guide] before the
  cloud provider setup
  - A `kubectl` pointing to a Crossplane control cluster
  - The [Crossplane CLI][crossplane-cli] installed
* An account on [Google Cloud Platform][gcp]

At the end, we will have:

* A Crossplane control cluster configured to use GCP
* The boilerplate of a GCP-based project spun up
* Support in the control cluster for managing MySQL and Kubernetes
  cluster dependencies
* A slightly better understanding of:
  - The way cloud providers are configured in Crossplane
  - The way dependencies for cloud-portable workloads are configured in
    Crossplane

We will **not** be teaching first principles in depth. Check out the
[concepts document][crossplane-concepts] for that.

## Install the GCP Stack

After Crossplane has been installed, it can be extended with more
functionality by installing a Crossplane Stack! Let's install the stack
for Google Cloud Platform (GCP) to add support for that cloud provider.
We can use the Crossplane CLI for this operation:

```
kubectl crossplane stack install 'crossplane/stack-gcp:master' stack-gcp
```

To install to a particular namespace, you can use the `generate-install`
command and pipe it to `kubectl apply` instead, which gives you more
control over how the stack's installation is handled. Everything is
Kubernetes object!

Since this is an infrastructure stack, we need to specify that it's
cluster-scoped via `--cluster` flag.
```
kubectl create namespace gcp
kubectl crossplane stack generate-install --cluster 'crossplane/stack-gcp:master' stack-gcp | kubectl apply --namespace gcp -f -
```

The namespace that we install the stack to is also the one where our
managed GCP resources will reside. When a developer requests a resource
by creating a **resource claim** in a namespace `mynamespace`, the
managed cloud provider resource and any secrets will be created in the
stack's namespace. Secrets will be copied over to `mynamespace`, and the
claim will be bound to the original resource claim.

For convenience, the next steps assume that you installed GCP stack into
the `gcp` namespace.

## Configure GCP Account

We will make use of the following services on GCP:

*   GKE
*   CloudSQL Instance
*   Network
*   Subnetwork
*   GlobalAddress
*   Private Service Connection

For all these to work, you need to [enable the following
APIs](https://cloud.google.com/endpoints/docs/openapi/enable-api) in
your GCP project:

*   Compute Engine API
*   Service Networking API
*   Kubernetes Engine API

We will also need to tell Crossplane how to use the credentials for the
GCP account. For this exercise, the GCP account that we will tell
Crossplane about should have the following [roles
assigned](https://cloud.google.com/iam/docs/granting-roles-to-service-accounts):

*   Cloud SQL Admin
*   Compute Network Admin
*   Kubernetes Engine Admin
*   Service Account User

You need to get JSON file of the service account you’ll use. In the next sections, this file will be referred to as `crossplane-gcp-provider-key.json`

You can create the JSON file by using the [gcloud
command](https://cloud.google.com/sdk/gcloud/reference/iam/service-accounts/keys/create)
and specifying a file name of `crossplane-gcp-provider-key.json`. If
you use Crossplane's [GCP credentials script][gcp-credentials], this
is taken care of for you.

## Configure Crossplane GCP Provider

Before creating any resources, we need to create and configure a cloud
provider in Crossplane. This helps Crossplane know how to connect to the cloud
provider. All the requests from Crossplane to GCP can use that resource as 
their credentials. The following command assumes that you have a
`crossplane-gcp-provider-key.json` file that belongs to the account
you’d like Crossplane to use. Run the command after changing
`[your-demo-project-id]` to your actual GCP project id. You should be
able to get the project id from the JSON credentials file or from the
GCP Console.

```
export PROJECT_ID=[your-demo-project-id]
export BASE64ENCODED_GCP_PROVIDER_CREDS=$(base64 crossplane-gcp-provider-key.json | tr -d "\n")
```

Now we’ll create our `Secret` that contains the credential and
`Provider` resource that refers to that secret:

```
sed "s/BASE64ENCODED_GCP_PROVIDER_CREDS/$BASE64ENCODED_GCP_PROVIDER_CREDS/g;s/PROJECT_ID/$PROJECT_ID/g" cluster/examples/workloads/kubernetes/wordpress/gcp/provider.yaml | kubectl create -f -
unset PROJECT_ID
unset BASE64ENCODED_GCP_PROVIDER_CREDS
```

The name of the `Provider` resource in the file above is `gcp-provider`;
we'll use the name `gcp-provider` to refer to this provider when we
configure and set up other Crossplane resources.

## Set Up Network Resources

Wordpress needs an SQL database and a Kubernetes cluster. But *those*
two resources need a private network to communicate securely. So, we
need to set up the network before we get to the database and Kubernetes
creation steps. Here's an example network setup:

```
---
# example-network will be the VPC that all cloud instances we'll create will use.
apiVersion: compute.gcp.crossplane.io/v1alpha2
kind: Network
metadata:
  name: example-network
  namespace: gcp
spec:
  name: example-network
  autoCreateSubnetworks: false
  providerRef:
    name: gcp-provider
    namespace: gcp
  reclaimPolicy: Delete
  routingConfig:
    routingMode: REGIONAL
---
# example-subnetwork defines IP ranges to be used by GKE cluster.
apiVersion: compute.gcp.crossplane.io/v1alpha2
kind: Subnetwork
metadata:
  name: example-subnetwork
  namespace: gcp
spec:
  providerRef:
    name: gcp-provider
    namespace: gcp
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
  network: projects/crossplane-playground/global/networks/example-network
---
# example-globaladdress defines the IP range that will be allocated for cloud services connecting
# to the instances in the given Network.
apiVersion: compute.gcp.crossplane.io/v1alpha2
kind: GlobalAddress
metadata:
  name: example-globaladdress
  namespace: gcp
spec:
  providerRef:
    name: gcp-provider
    namespace: gcp
  reclaimPolicy: Delete
  name: example-globaladdress
  purpose: VPC_PEERING
  addressType: INTERNAL
  prefixLength: 16
  network: projects/crossplane-playground/global/networks/example-network
---
# example-connection is what allows cloud services to use the allocated GlobalAddress for communication. Behind
# the scenes, it creates a VPC peering to the network that those service instances actually live.
apiVersion: servicenetworking.gcp.crossplane.io/v1alpha2
kind: Connection
metadata:
  name: example-connection
  namespace: gcp
spec:
  providerRef:
    name: gcp-provider
    namespace: gcp
  reclaimPolicy: Delete
  parent: services/servicenetworking.googleapis.com
  network: projects/crossplane-playground/global/networks/example-network
  reservedPeeringRanges:
    - example-globaladdress
```
You can edit snippet above to customize it or run the following command to apply it:
```
kubectl apply -f cluster/examples/workloads/kubernetes/wordpress/gcp/network.yaml
```

For more details about networking and what happens when you run this
command, see [this document with more details][crossplane-gcp-networking-docs].

It takes a while to create these resources in GCP. The top-level object
is the `Connection` object; when the `Connection` is ready, everything
else is too. We can watch it by running (assumes gcp stack is installed in `gcp` namespace):

```
kubectl -n gcp get connection.servicenetworking.gcp.crossplane.io/example-connection -o custom-columns='NAME:.metadata.name,FIRST_CONDITION:.status.conditions[0].status,SECOND_CONDITION:.status.conditions[1].status'
```

## Configure Provider Resources

Once we have the network set up, we also need to tell Crossplane how to
satisfy WordPress's claims for a database and a Kubernetes cluster.
The resource classes serve as template for the new claimswe make. 
The following resource classes allow the GKECluster and CloudSQL claims
to be satisfied with the network configuration we just set up:

```
---
apiVersion: database.gcp.crossplane.io/v1alpha2
kind: CloudsqlInstanceClass
metadata:
  name: standard-cloudsql
  namespace: gcp
specTemplate:
  databaseVersion: MYSQL_5_7
  tier: db-n1-standard-1
  region: us-central1
  storageType: PD_SSD
  storageGB: 10
  # Note from GCP Docs: Your Cloud SQL instances are not created in your VPC network.
  # They are created in the service producer network (a VPC network internal to Google) that is then connected (peered) to your VPC network.
  privateNetwork: projects/crossplane-playground/global/networks/example-network
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
  network: projects/crossplane-playground/global/networks/example-network
  subnetwork: projects/crossplane-playground/regions/us-central1/subnetworks/example-subnetwork
  enableIPAlias: true
  clusterSecondaryRangeName: pods
  servicesSecondaryRangeName: services
  providerRef:
    name: gcp-provider
    namespace: gcp
  reclaimPolicy: Delete
```
You can edit snippet above to customize it or run the following command to apply it:
```
kubectl apply -f cluster/examples/workloads/kubernetes/wordpress/gcp/environment.yaml
```

The steps that we have taken so far have been related to things that can
be shared by all resources in all namespaces of that Crossplane cluster.
Now, we will keep going with creating an app namespace and populating it
with resources that will help Crossplane know with what configuration it
should satisfy the claims. You can use any namespace for your app's
resources but for this tutorial we'll create a new namespace.

```
kubectl create namespace mynamespace
```

Now we need to tell Crossplane which resource classes should be used to
satisfy our claims in that app namespace. We will create portable
classes that have have reference to non-portable ones that we created
earlier. In our claims, we can refer to those portable classes directly
or label one as the default portable class to be used in claims that do
not have class reference.

> Portable classes are a way of referring to non-portable resource classes in other namespaces. 

For example, MySQLInstanceClass is a portable class that can refer to
GCP's CloudSQLInstanceClass, which is a non-portable class.

```
---
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: standard-mysql
  namespace: mynamespace
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
  namespace: mynamespace
  labels:
    default: "true"
classRef:
  kind: GKEClusterClass
  apiVersion: compute.gcp.crossplane.io/v1alpha2
  name: standard-gke
  namespace: gcp
---
```

You can run the following command for namespace and portable class creation:

```
kubectl apply -f cluster/examples/workloads/kubernetes/wordpress/gcp/namespace.yaml
```

For more details about what is happening behind the scenes, read more
about [portable claims in Crossplane][portable-claims].

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

Next we'll set up the Crossplane Stack and use it! Head [back over to
the Stacks Guide document][stacks-guide-continue] so we can pick up where we left off.

## TODO
This should not go in the final document, but is here for tracking.

* Add references
* Add next steps, with link to WordPress-specific stuff

<!-- Links -->
[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli
[crossplane-gcp-networking-docs]: TODO
[stacks-guide]: stacks-guide.html

[crossplane-concepts]: TODO
[portable-claims]: TODO

[gcp-credentials]: TODO

[gcp]: https://cloud.google.com/

[stacks-guide-continue]: stacks-guide.html#install-support-for-our-application-into-crossplane

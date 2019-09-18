---
title: "Stacks Guide: GCP Setup"
toc: true
weight: 520
indent: true
---

# Stacks Guide: GCP Setup

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
can install and use the [WordPress sample
stack][sample-wordpress-stack], which depends on MySQL and Kubernetes!

Before we begin, you will need:

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
[Crossplane concepts document][crossplane-concepts] for that.

## Install the GCP Stack

After Crossplane has been installed, it can be extended with more
functionality by installing a [Crossplane Stack][stack-docs]! Let's
install the [stack for Google Cloud Platform][stack-gcp] (GCP) to add
support for that cloud provider. We can use the [Crossplane
CLI][crossplane-cli] for this operation. Since this is an infrastructure
stack, we need to specify that it's cluster-scoped by passing the
`--cluster` flag.

To install to a specific namespace, we can use the `generate-install`
command and pipe it to `kubectl apply` instead, which gives us more
control over how the stack's installation is handled. Everything is
a Kubernetes object!

```
kubectl create namespace gcp
kubectl crossplane stack generate-install --cluster 'crossplane/stack-gcp:master' stack-gcp | kubectl apply --namespace gcp -f -
```

If we wanted to use whatever the current namespace is, we could have
used `kubectl crossplane stack install` instead of using
`generate-install`.

The namespace that we install the stack to is where the stack will
create the resources it manages. When a developer requests a resource by
creating a [resource claim][resource-claims-docs] in a namespace `mynamespace`, the
managed cloud provider resource and any secrets will be created in the
stack's namespace. Secrets will be copied over to `mynamespace`, and the
claim will be bound to the original resource claim. For more details
about resource claims and how they work, see the [documentation on
resource claims][resource-claims-docs].

For convenience, the next steps assume that you installed GCP stack into
the `gcp` namespace.

### Validate the installation

To check to see whether our stack installed correctly, we can look at
the status of our stack:

```
kubectl -n gcp get stack
```

It should look something like this:

```
NAME        READY   VERSION   AGE
stack-gcp   True    0.0.1     5m19s
```

## Configure GCP Account

We will make use of the following services on GCP:

*   GKE
*   CloudSQL Instance
*   Network
*   Subnetwork
*   GlobalAddress
*   Private Service Connection

For all these to work, you need to [enable the following
APIs][gcp-enable-apis] in your GCP project:

*   Compute Engine API
*   Service Networking API
*   Kubernetes Engine API

We will also need to tell Crossplane how to use the credentials for the
GCP account. For this exercise, the GCP account that we will tell
Crossplane about should have the following [roles
assigned][gcp-assign-roles]:

*   Cloud SQL Admin
*   Compute Network Admin
*   Kubernetes Engine Admin
*   Service Account User

### Set up cloud provider credentials

This guide assumes that you have created a JSON file which contains the
credentials for the cloud provider. In later sections, this file will be
referred to as `crossplane-gcp-provider-key.json`. There are quite a few
steps involved, so the steps are in [a different document which you
should take a look at][cloud-provider-setup-gcp] before moving to the
next section. Or, there is also [a script in the Crossplane
repo][gcp-credentials] which helps with creating the file.

## Configure Crossplane GCP Provider

Before creating any resources, we need to create and configure a cloud
provider in Crossplane. This helps Crossplane know how to connect to the cloud
provider. All the requests from Crossplane to GCP will use the
credentials attached to the provider object.  The following command
assumes that you have a `crossplane-gcp-provider-key.json` file that
belongs to the account you’d like Crossplane to use. Run the command
after changing `[your-demo-project-id]` to your actual GCP project id.
You should be able to get the project id from the JSON credentials file
or from the GCP Console.

```
export PROJECT_ID=[your-demo-project-id]
export BASE64ENCODED_GCP_PROVIDER_CREDS=$(base64 crossplane-gcp-provider-key.json | tr -d "\n")
```

The environment variable `PROJECT_ID` is going to be used in multiple
YAML files in the next steps, while `BASE64ENCODED_GCP_PROVIDER_CREDS`
is only needed for this step.

Now we’ll create our `Secret` that contains the credential and
`Provider` resource that refers to that secret:

```
cat > provider.yaml <<EOF
---
apiVersion: v1
data:
  credentials.json: $BASE64ENCODED_GCP_PROVIDER_CREDS
kind: Secret
metadata:
  namespace: gcp
  name: gcp-provider-creds
type: Opaque
---
apiVersion: gcp.crossplane.io/v1alpha2
kind: Provider
metadata:
  namespace: gcp
  name: gcp-provider
spec:
  credentialsSecretRef:
    name: gcp-provider-creds
    key: credentials.json
  projectID: $PROJECT_ID
EOF

kubectl apply -f provider.yaml
```

The example YAML also exists in [the Crossplane repository][crossplane-sample-gcp-provider].

The name of the `Provider` resource in the file above is `gcp-provider`;
we'll use the name `gcp-provider` to refer to this provider when we
configure and set up other Crossplane resources.

### Validate

To check on our newly created provider, we can run:

```
kubectl -n gcp get provider.gcp.crossplane.io
```

The output should look something like:

```
NAME           PROJECT-ID              AGE
gcp-provider   crossplane-playground   37s
```

## Set Up Network Resources

Wordpress needs a SQL database and a Kubernetes cluster. But **those**
two resources need a private network to communicate securely. So, we
need to set up the network before we set up the database and the
Kubernetes cluster. Here's an example of how to set up a network:

```
cat > network.yaml <<EOF
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
  network: projects/$PROJECT_ID/global/networks/example-network
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
  network: projects/$PROJECT_ID/global/networks/example-network
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
  network: projects/$PROJECT_ID/global/networks/example-network
  reservedPeeringRanges:
    - example-globaladdress
EOF

kubectl apply -f network.yaml
```

The example YAML also exists in [the Crossplane repository][crossplane-sample-gcp-network].

For more details about networking and what happens when you run this
command, see [this document with more details][crossplane-gcp-networking-docs].

It takes a while to create these resources in GCP. The top-level object
is the `Connection` object; when the `Connection` is ready, everything
else is too. We can watch it by running the following command, which
assumes the GCP stack is installed in `gcp` namespace:

```
kubectl -n gcp get connection.servicenetworking.gcp.crossplane.io/example-connection -o custom-columns='NAME:.metadata.name,FIRST_CONDITION:.status.conditions[0].status,SECOND_CONDITION:.status.conditions[1].status'
```

The output should look something like:

```
NAME                 FIRST_CONDITION   SECOND_CONDITION
example-connection   True              True
```

The conditions we're checking for are `Ready` and `Synced`. The reason
we are using `FIRST_CONDITION` and `SECOND_CONDITION` is because we
don't know what order they'll be in when we run the command.

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

[stacks-guide-continue]: stacks-guide.html#install-support-for-our-application-into-crossplane
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

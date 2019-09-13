---
title: Crossplane Stacks Guide: GCP Setup
toc: true
weight: 330
indent: true
---

# Crossplane Stacks Guide: GCP Setup

*Assuming crossplane has been installed*

*Following instructions apply when you use https://github.com/crossplaneio/crossplane/pull/771*

## Table of Contents

TODO

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
[concepts document][concepts] for that.

## Install the GCP Stack

After Crossplane has been installed, it can be extended with more
functionality by installing a Crossplane Stack! Let's install the stack
for Google Cloud Platform (GCP) to add support for that cloud provider. We
can use the Crossplane CLI for this operation:

```
kubectl crossplane stack install 'crossplane/stack-gcp:master' stack-gcp
```

To install to a particular namespace, you can use the `generate-install`
command and pipe it to `kubectl apply` instead, which gives you more
control over how the stack's installation is handled. Everything is
Kubernetes object!

```
kubectl crossplane stack generate-install 'crossplane/stack-gcp:master' stack-gcp | kubectl apply --namespace gcp -f -
```

The namespace that we install the stack to is also the one where our
managed GCP resources will reside. When a developer requests a resource
by creating a **resource claim** in a namespace `mynamespace`, the managed
cloud provider resource and any secrets will be created in the stack's
namespace. Secrets will be copied over to `mynamespace`, and the claim
will be bound to the original resource claim.

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

## Configure Crossplane Provider

Before creating any resources, we need to create and configure a cloud
provider in Crossplane. This helps Crossplane know how to connect to the cloud
provider.  going to GCP can use that resource as their credentials. The
following command assumes that you have a
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

## Set Up Provider Resources

Wordpress needs a SQL database and a Kubernetes cluster. But *those*
need networks to run in, especially if we want to have secure
networking. So we need to set up the networks before we get to the
database and Kubernetes. Here's how to do that:

```
kubectl apply -f cluster/examples/workloads/kubernetes/wordpress/gcp/environment.yaml
```

For more details about networking and what happens when you run this
command, see [this document with more details][gcp-networking].

It takes a while to create these resources in GCP. The top-level object
is the `Connection` object; when the `Connection` is ready, everything
else is too. We can watch it by running:

```
kubectl -n gcp get connection.servicenetworking.gcp.crossplane.io/example-connection -o custom-columns='NAME:.metadata.name,READY:.status.conditions[1].status'
```

## Configure Provider Resources

Once we have the network set up, we also need to tell Crossplane how to
satisfy WordPress's claims on a database and on a Kubernetes cluster. We
do that using the following configuration:

```
TODO INSERT POLICY HERE
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
* The configuration we need to run a portable workload on Crossplane
  which is backed by GCP.

## Next Steps

Next we'll set up the Crossplane Stack and use it! Head [back over to
the Stacks Guide document][stacks-guide-continue] so we can pick up where we left off.

## TODO
This should not go in the final document, but is here for tracking.

* Add policies for dependencies
* Add table of contents
* Add references
* Add next steps, with link to WordPress-specific stuff
* Add a TOC

<!-- Links -->
[stacks-guide]:
[gcp-credentials]: 
[crossplane-cli]:
[gcp-networking]:
[gcp]:
[concepts]:
[stacks-guide-continue]:
[portable-claims]:

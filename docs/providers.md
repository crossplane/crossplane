---
title: Providers
toc: true
weight: 400
indent: true
---

# Providers

Providers are external infrastructure providers such as Amazon Web Services,
Microsoft Azure, Google Cloud and Alibaba Cloud.

In order to provision a resource in the provider, a Custom Resource Definition(CRD)
needs to be registered in your Kubernetes cluster and its controller should
be watching instances of that Custom Resource Definition. Provider packages
contain many Custom Resource Definitions and their controllers to extend
functionality of Crossplane.

Here is the list of current providers:

* [Amazon Web Services][provider-aws]
  * [API Reference][aws-reference]
* [Google Cloud][provider-gcp]
  * [API Reference][gcp-reference]
* [Microsoft Azure][provider-azure]
  * [API Reference][azure-reference]
* [Rook][provider-rook]
  * [API Reference][rook-reference]
* [Alibaba Cloud][provider-alibaba]
  * [API Reference][alibaba-reference]

## Installation

The core Crossplane controller can install Provider controllers and CRDs for you
through its own provider packaging mechanism, which is triggered by the application
of a `ClusterPackageInstall` resource. For example, in order to request
installation of the `provider-gcp` package, apply the following resource to the
cluster where Crossplane is running:

```yaml
apiVersion: packages.crossplane.io/v1alpha1
kind: ClusterPackageInstall
metadata:
  name: provider-gcp
  namespace: crossplane-system
spec:
  package: "crossplane/provider-gcp:master"
```

The field `spec.package` is where you refer to the container image of the
provider. Crossplane Package Manager will unpack that container, register
CRDs and set up necessary RBAC rules and then start the controllers.

There are a few other ways to install to trigger the installation of provider
packages:

* As part of Crossplane Helm chart by adding the following
  statement to your `helm install` command: `--set clusterPackages.gcp.deploy=true`
  It will install the default version hard-coded in that release of Crossplane
  Helm chart but if you'd like to specif an exact version, you can add:
  `--set clusterPackages.gcp.version=master`.
* Using Crossplane kubectl plugin:
  `kubectl crossplane package install --cluster -n crossplane-system 'crossplane/provider-gcp:master' provider-gcp`

You can delete them by deleting the `ClusterPackageInstall` resource you've created.

## Configuration

In order to authenticate with the provider API, the provider controllers
need to have access to credentials. It could be an AWS IAM User, GCP Service
Account or Azure Service Principal. Every provider has a type called `Provider`
that has information about how to authenticate to the provider API. A `Provider`
resource for Azure looks like the following:

```yaml
apiVersion: azure.crossplane.io/v1alpha3
kind: Provider
metadata:
  name: prod-acc
spec:
  credentialsSecretRef:
    namespace: crossplane-system
    name: azure-prod-creds
    key: credentials
```

You can see that there is a reference to a key in a specific `Secret`. The value
of that key should contain the credentials that the controller will use. The
documentation of each provider should give you an idea of how that credentials
blob should look like. See [Getting Started][getting-started] guide for more details.

The following is an example usage of Azure `Provider`, referred by a `MySQLServer`:

```yaml
apiVersion: database.azure.crossplane.io/v1beta1
kind: MySQLServer
metadata:
  name: prod-sql
spec:
  providerRef: prod-acc
  ...
```

The Azure provider controller will use that provider for this instance of `MySQLServer`.
Since every resource has its own reference to a `Provider`, you can have multiple
`Provider` resources in your cluster referred by different resources.


<!-- Named Links -->

[provider-aws]: https://github.com/crossplane/provider-aws
[aws-reference]: https://doc.crds.dev/github.com/crossplane/provider-aws
[provider-gcp]: https://github.com/crossplane/provider-gcp
[gcp-reference]: https://doc.crds.dev/github.com/crossplane/provider-gcp
[provider-azure]: https://github.com/crossplane/provider-azure
[azure-reference]: https://doc.crds.dev/github.com/crossplane/provider-azure
[provider-rook]: https://github.com/crossplane/provider-rook
[rook-reference]: https://doc.crds.dev/github.com/crossplane/provider-rook
[provider-alibaba]: https://github.com/crossplane/provider-alibaba
[alibaba-reference]: https://doc.crds.dev/github.com/crossplane/provider-alibaba
[getting-started]: getting-started/configure.md

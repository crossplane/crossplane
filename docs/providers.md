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

Since all managed resources are all cluster-scoped, provider packages are
bundled as `ClusterPackage`s. In order to request installation of a provider,
the following `ClusterPackageInstall` needs to be created:

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

There are a few ways that are more convenient than creating a YAML to install
the providers.

* As part of Crossplane Helm chart by adding the following
  statement to your `helm install` command: `--set clusterPackages.gcp.deploy=true`
  It will install the default version hard-coded in that release of Crossplane
  Helm chart but if you'd like to specif an exact version , you can use:
  `--set clusterPackages.gcp.version=master`.
* Using Crossplane kubectl plugin:
  `kubectl crossplane package install --cluster -n crossplane-system 'crossplane/provider-gcp:master' provider-gcp`

You can delete them by deleting the installed `ClusterPackageInstall` resource.

## Configuration

In order to authenticate with the provider API, the provider controllers
need to have access to credentials. It could be an AWS IAM User, GCP Service
Account or Azure Service Principal. Every managed resource like `RDSInstance`
or `CloudSQLInstance` has a field called `spec.providerRef` to refer to what
credentials to use for operations regarding that managed resource. That
reference points to a specific type called `Provider` that contains information
about the authentication method that should be used. A `Provider` resource for
Azure looks like the following:

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
blob should look like.

Keep in mind that it's not a requirement that the authentication should be done
through a credentials blob in a `Secret`. For example, in AWS, `Provider` has
a field called `useServiceAccount` to imply that the provider controller
should use workload identity feature of AWS instead of a `Secret` that contains
IAM credentials.

Since each managed resource refers to a `Provider` resource, you can have multiple
`Provider` resources in your cluster and different managed resources using them.


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

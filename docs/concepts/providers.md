---
title: Providers
toc: true
weight: 101
indent: true
---

# Providers

Providers extend Crossplane to enable infrastructure resource provisioning. In
order to provision a resource, a Custom Resource Definition (CRD) needs to be
registered in your Kubernetes cluster and its controller should be watching the
Custom Resources those CRDs define. Provider packages contain many Custom
Resource Definitions and their controllers.

Here is the list of prominent providers:

### AWS Provider

* [GitHub][provider-aws]
* [API Reference][aws-reference]
* [Amazon Web Services (AWS) IAM User]

### GCP Provider

* [GitHub][provider-gcp]
* [API Reference][gcp-reference]
* [Google Cloud Platform (GCP) Service Account]

### Azure Provider

* [GitHub][provider-azure]
* [API Reference][azure-reference]
* [Microsoft Azure Service Principal]

### Rook Provider

* [GitHub][provider-rook]
* [API Reference][rook-reference]

### Alibaba Cloud Provider

* [GitHub][provider-alibaba]
* [API Reference][alibaba-reference]

## Installing Providers

The core Crossplane controller can install provider controllers and CRDs for you
through its own provider packaging mechanism, which is triggered by the
application of a `Provider` resource. For example, in order to request
installation of the `provider-aws` package, apply the following resource to the
cluster where Crossplane is running:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: "crossplane/provider-aws:master"
```

The field `spec.package` is where you refer to the container image of the
provider. Crossplane Package Manager will unpack that container, register CRDs
and set up necessary RBAC rules and then start the controllers.

There are a few other ways to to trigger the installation of provider packages:

* As part of Crossplane Helm chart by adding the following statement to your
  `helm install` command: `--set
  provider.packages={crossplane/provider-aws:master}`.
* Using the Crossplane CLI: `kubectl crossplane install provider
  crossplane/provider-aws:master`

You can uninstall a provider by deleting the `Provider` resource
you've created.

## Configuring Providers

In order to authenticate with the external provider API, the provider
controllers need to have access to credentials. It could be an IAM User for AWS,
a Service Account for GCP or a Service Principal for Azure. Every provider has a
type called `ProviderConfig` that has information about how to authenticate to
the provider API. An example `ProviderConfig` resource for AWS looks like the
following:

```yaml
apiVersion: aws.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: aws-provider
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: key
```

You can see that there is a reference to a key in a specific `Secret`. The value
of that key should contain the credentials that the controller will use. The
documentation of each provider should give you an idea of how that credentials
blob should look like. See [Getting Started][getting-started] guide for more
details.

The following is an example usage of AWS `ProviderConfig`, referenced by a
`RDSInstance`:

```yaml
apiVersion: database.aws.crossplane.io/v1beta1
kind: RDSInstance
metadata:
  name: prod-sql
spec:
  providerConfigRef:
    name: aws-provider
  ...
```

The AWS provider controller will use that provider for this instance of
`RDSInstance`. Since every resource has its own reference to a `ProviderConfig`,
you can have multiple `ProviderConfig` resources in your cluster referenced by
different resources. When no `providerConfigRef` is specified, the `RDSInstance`
will attempt to use a `ProviderConfig` named `default`.

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
[getting-started]: ../getting-started/install-configure.md
[Google Cloud Platform (GCP) Service Account]: ../cloud-providers/gcp/gcp-provider.md
[Microsoft Azure Service Principal]: ../cloud-providers/azure/azure-provider.md
[Amazon Web Services (AWS) IAM User]: ../cloud-providers/aws/aws-provider.md

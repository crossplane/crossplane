---
title: Providers
toc: true
weight: 101
indent: true
---

# Providers

Providers are Crossplane packages that bundle a set of [Managed
Resources][managed-resources] and their respective controllers to allow
Crossplane to provision the respective infrastructure resource.

Here is the list of prominent providers:

|   |   |   |   |
|---|---|---|---|
| Provider AWS  |  [GitHub Repo](https://github.com/crossplane/provider-aws) | [API Reference](https://doc.crds.dev/github.com/crossplane/provider-aws)  |  [Amazon Web Services (AWS) IAM User] |
| Provider Jet AWS  |  [GitHub Repo](https://github.com/crossplane-contrib/provider-jet-aws) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-aws) |   | 
| Provider GCP |  [GitHub Repo](https://github.com/crossplane/provider-gcp) | [API Reference](https://doc.crds.dev/github.com/crossplane/provider-gcp) |   [Google Cloud Platform (GCP) Service Account] | 
| Provider Jet GCP  |  [GitHub Repo](https://github.com/crossplane-contrib/provider-jet-gcp) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-gcp)  |   | 
| Provider Azure  | [GitHub Repo](https://github.com/crossplane/provider-azure) | [API Reference](https://doc.crds.dev/github.com/crossplane/provider-azure)  |  [Microsoft Azure Service Principal] | 
| Provider Jet Azure  |  [GitHub Repo](https://github.com/crossplane-contrib/provider-jet-azure) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-azure) |   | 
| Provider Alibaba |  [GitHub Repo](https://github.com/crossplane/provider-alibaba) | [API Reference](https://doc.crds.dev/github.com/crossplane/provider-alibaba)  |   | 
|   |   |   |   |
| Provider Rook  |  [GitHub Repo](https://github.com/crossplane/provider-rook) | [API Reference](https://doc.crds.dev/github.com/crossplane/provider-rook)  |  |
| Provider Helm  |  [GitHub Repo](https://github.com/crossplane-contrib/provider-helm) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-helm)  |  |
| Provider Terraform  |  [GitHub Repo](https://github.com/crossplane-contrib/provider-terraform) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-terraform)  |  |
| Provider Kubernetes  |  [GitHub Repo](https://github.com/crossplane-contrib/provider-kubernetes) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-kubernetes)  |  |
| Provider SQL | [GitHub Repo](https://github.com/crossplane-contrib/provider-sql) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-sql)  |  |
| Provider Gitlab  | [GitHub Repo](https://github.com/crossplane-contrib/provider-gitlab) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-gitlab)  |  |
| Provider Equinix Metal | [GitHub Repo](https://github.com/crossplane-contrib/provider-equinix-metal) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-equinix-metal)  |  |
| Provider Digital Ocean | [GitHub Repo](https://github.com/crossplane-contrib/provider-digitalocean) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-digitalocean)  |  |
| Provider Civo | [GitHub Repo](https://github.com/crossplane-contrib/provider-civo) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-civo)  |  |
| Provider IBM Cloud | [GitHub Repo](https://github.com/crossplane-contrib/provider-ibm-cloud) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-civo)  |  |
| Provider Argocd | [GitHub Repo](https://github.com/crossplane-contrib/provider-argocd) | [API Reference](https://doc.crds.dev/github.com/crossplane-contrib/provider-argocd)  |  |


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

[getting-started]: ../getting-started/install-configure.md
[Google Cloud Platform (GCP) Service Account]: ../cloud-providers/gcp/gcp-provider.md
[Microsoft Azure Service Principal]: ../cloud-providers/azure/azure-provider.md
[Amazon Web Services (AWS) IAM User]: ../cloud-providers/aws/aws-provider.md
[managed-resources]: managed-resources.md

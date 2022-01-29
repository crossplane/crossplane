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
| Provider AWS  |  [GitHub Repo][provider-aws] | [API Reference][provider-aws-api]  |  [Amazon Web Services (AWS) IAM User] |
| Provider Jet AWS  |  [GitHub Repo][provider-jet-aws] | [API Reference][provider-jet-aws-api] |   | 
| Provider GCP |  [GitHub Repo][provider-gcp] | [API Reference][provider-gcp-api] |   [Google Cloud Platform (GCP) Service Account] | 
| Provider Jet GCP  |  [GitHub Repo][provider-jet-gcp] | [API Reference][provider-jet-gcp-api]  |   | 
| Provider Azure  | [GitHub Repo][provider-azure] | [API Reference][provider-azure-api]  |  [Microsoft Azure Service Principal] | 
| Provider Jet Azure  |  [GitHub Repo][provider-jet-azure] | [API Reference][provider-jet-azure-api] |   | 
| Provider Alibaba |  [GitHub Repo][provider-alibaba] | [API Reference][provider-alibaba-api]  |   | 
|   |   |   |   |
| Provider Rook  |  [GitHub Repo][provider-rook] | [API Reference][provider-rook-api]  |  |
| Provider Helm  |  [GitHub Repo][provider-helm] | [API Reference][provider-helm-api]  |  |
| Provider Terraform  |  [GitHub Repo][provider-terraform] | [API Reference][provider-terraform-api]  |  |
| Provider Kubernetes  |  [GitHub Repo][provider-kubernetes] | [API Reference][provider-kubernetes-api]  |  |
| Provider SQL | [GitHub Repo][provider-sql] | [API Reference][provider-sql-api]  |  |
| Provider Gitlab  | [GitHub Repo][provider-gitlab] | [API Reference][provider-gitlab-api]  |  |
| Provider Equinix Metal | [GitHub Repo][provider-equinix-metal] | [API Reference][provider-equinix-metal-api]  |  |
| Provider Digital Ocean | [GitHub Repo][provider-digitalocean] | [API Reference][provider-digitalocean-api]  |  |
| Provider Civo | [GitHub Repo][provider-civo] | [API Reference][provider-civo-api]  |  |
| Provider IBM Cloud | [GitHub Repo][provider-ibm-cloud] | [API Reference][provider-ibm-cloud-api]  |  |
| Provider Argocd | [GitHub Repo][provider-argocd] | [API Reference][provider-argocd-api]  |  |
| Provider Styra | [GitHub Repo][provider-styra] | [API Reference][provider-styra-api]  |  |
| Provider Cloudflare | [GitHub Repo][provider-cloudflare] | [API Reference][provider-cloudflare-api]  |  |


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
  package: "crossplane/provider-aws:v0.23.0"
```

The field `spec.package` is where you refer to the container image of the
provider. Crossplane Package Manager will unpack that container, register CRDs
and set up necessary RBAC rules and then start the controllers.

There are a few other ways to to trigger the installation of provider packages:

* As part of Crossplane Helm chart by adding the following statement to your
  `helm install` command: `--set
  provider.packages={crossplane/provider-aws:v0.23.0}`.
* Using the Crossplane CLI: `kubectl crossplane install provider
  crossplane/provider-aws:v0.23.0`

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
[provider-aws]: https://github.com/crossplane/provider-aws
[provider-aws-api]: https://doc.crds.dev/github.com/crossplane/provider-aws
[provider-jet-aws]: https://github.com/crossplane-contrib/provider-jet-aws
[provider-jet-aws-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-aws
[provider-gcp]: https://github.com/crossplane/provider-gcp
[provider-gcp-api]: https://doc.crds.dev/github.com/crossplane/provider-gcp
[provider-jet-gcp]: https://github.com/crossplane-contrib/provider-jet-gcp
[provider-jet-gcp-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-gcp
[provider-azure]: https://github.com/crossplane/provider-azure
[provider-azure-api]: https://doc.crds.dev/github.com/crossplane/provider-azure
[provider-jet-azure]: https://github.com/crossplane-contrib/provider-jet-azure
[provider-jet-azure-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-jet-azure
[provider-alibaba]: https://github.com/crossplane/provider-alibaba
[provider-alibaba-api]: https://doc.crds.dev/github.com/crossplane/provider-alibaba 
[provider-rook]: https://github.com/crossplane/provider-rook
[provider-rook-api]: https://doc.crds.dev/github.com/crossplane/provider-rook
[provider-helm]: https://github.com/crossplane-contrib/provider-helm
[provider-helm-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-helm
[provider-terraform]: https://github.com/crossplane-contrib/provider-terraform
[provider-terraform-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-terraform
[provider-kubernetes]: https://github.com/crossplane-contrib/provider-kubernetes
[provider-kubernetes-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-kubernetes
[provider-sql]: https://github.com/crossplane-contrib/provider-sql
[provider-sql-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-sql
[provider-gitlab]: https://github.com/crossplane-contrib/provider-gitlab
[provider-gitlab-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-gitlab
[provider-equinix-metal]: https://github.com/crossplane-contrib/provider-equinix-metal
[provider-equinix-metal-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-equinix-metal
[provider-digitalocean]: https://github.com/crossplane-contrib/provider-digitalocean
[provider-digitalocean-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-digitalocean
[provider-civo]: https://github.com/crossplane-contrib/provider-civo
[provider-civo-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-civo
[provider-ibm-cloud]: https://github.com/crossplane-contrib/provider-ibm-cloud
[provider-ibm-cloud-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-ibm-cloud
[provider-argocd]: https://github.com/crossplane-contrib/provider-argocd
[provider-argocd-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-argocd
[provider-styra]: https://github.com/crossplane-contrib/provider-styra
[provider-styra-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-styra
[provider-cloudflare]: https://github.com/crossplane-contrib/provider-cloudflare
[provider-cloudflare-api]: https://doc.crds.dev/github.com/crossplane-contrib/provider-cloudflare
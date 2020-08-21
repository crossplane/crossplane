
# Package Format v2

* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Packages extend Crossplane with new functionality. [Historically][packages-v1]
packages were known as 'stacks', and were quite open ended in what they could
extend. Packages were initially designed to install Kubernetes controllers.
Crossplane providers were the most commonly packaged controller variant. Other
variants included 'application' controllers that deployed applications like
Wordpress, and 'templating' controllers that provided [abstractions around
infrastructure][resource-packs]. Crossplane packages are [OCI] images that
contain the information required for Crossplane to install new controllers.

As Crossplane has matured it has become a tool that platform builders use to
configure and offer their own opinionated API abstractions to their platform
consumers - a tool to build platforms. Crossplane enables a platform builder to
[compose and offer][composition] their own APIs by defining composite resources.
A composite resource is a Kubernetes resource that is reconciled by rendering
other resources. A composite resource may consist of other composite resources,
but the 'leaves' of these compositions are always 'managed resources' - a form
of Kubernetes resource that is reconciled by a Crossplane provider.

Several conceptual and API changes are required to focus Crossplane on providing
a great experience around building, operating, and consuming platforms. Part of
this simplification involves focusing packages and the package manager on two
key use cases; extending Crossplane with new providers, and extending Crossplane
with support for new composite resources.

## Goals

The goal of this document is to supplement the existing [package manager v2
design][packages-v2] by formalising the content of a package - the filesystem
that exists inside a package's OCI image. This metadata provides two purposes:

1. It provides the information the Crossplane package manager needs to manage
   the lifecycle of a package - to install or upgrade a provider or a
   configuration.
1. It describes what is packaged. This information is not used by Crossplane,
   but provides an API that other tools such as package registries can use to
   introspect a package.

Note that one goal of this design is to build a user experience around working
with 'providers' and 'configurations', rather than working with 'packages'. The
package itself will be positioned as strictly the delivery mechanism.

## Proposal

This document proposes that:

1. All Crossplane packages contain a `crossplane.yaml` file at their root.
1. The `crossplane.yaml` file describe either a `Provider` or a `Configuration`.
1. Packages may include YAML representations of Crossplane specific custom
   resources (e.g. a `Composition` or a managed resource CRD). Crossplane is not
   opinionated about where in the package filesystem these files exist - the
   package unpack process should detect them automatically.
1. Additional metadata that is opaque to Crossplane be stored in a directory
   named `.crossplane` at the root of the package, alongside `crossplane.yaml`.

### Packaging a Provider

A packaged Crossplane provider must be an OCI image consisting of:

1. A `crossplane.yaml` file containing a `kind: Provider` at its root.
1. One or more `CustomResourceDefinition` resources that must define Crossplane
   provider specific resources, for example a managed resource.
1. Optional additional metadata that is opaque to Crossplane in a `.crossplane`
   directory at the root of the package.

Below is an example `Provider`, at `v1alpha1`:

```yaml
# Required. Must be as below.
apiVersion: meta.packages.crossplane.io/v1alpha1
# Required. Must be as below.
kind: Provider
# Required. Note that Crossplane is aware only of the name and annotations
# metadata fields. Other fields (e.g. labels) will be preserved but opaque.
metadata:
  # Required. Must comply with Kubernetes API conventions.
  name: provider-example
  # Optional. Must comply with Kubernetes API conventions. Annotations are
  # opaque to Crossplane, which will replicate them to the annotations of a
  # PackageRevision when this package is unpacked, but otherwise ignore them.
  # Systems such as package registries may extend this specification to require
  # or encourage specific annotations.
  annotations:
    company: Upbound
    maintainer: Nic Cope <negz@upbound.io>
    keywords: cloud-native, kubernetes
    source: github.com/crossplane-contrib/provider-example
    license: Apache-2.0
    description: |
      The Example provider adds support for example resources to Crossplane.
    provider: example
# Required.
spec:
  # Required. Currently supports only the image field, but may be extended with
  # other fields (e.g. health probes, env vars) in future.
  controller:
    # Required. Specifies an OCI image that must run the Crossplane provider
    # when invoked. Note that this is distinct from the package OCI image.
    image: example/provider-example:v0.1.0
  # Optional. Specifies a Crossplane version that the package is compatible with.
  crossplane: v0.13.0
  # Optional. Used by Crossplane to ensure any dependencies of a provider are
  # installed and running before the provider is installed. Unlikely to be used
  # in practice.
  dependsOn:
    # Required. Specifies an OCI image containing a package dependency. This key
    # may be either 'provider' or 'configuration'. This is sugar; in either case
    # the package manager determines whether the depencency is really a Provider
    # or a Configuration by unpacking it and inspecting its kind.
  - provider: example/provider-uncommon
    # Required. Will be extended to support version ranges in future, but
    # currently treated as a specific version tag.
    version: v0.1.0
  # Optional. Permissions that should be added to the ServiceAccount assigned to
  # the provider controller. The controller will automatically have permissions
  # on any CRDs it installs, but may specify additional permissions as well. The
  # package manager may or may not allow the additional permissions, depending
  # on how it is configured.
  permissionRequests:
  - apiGroups:
      - otherpackage.example.com
    resources:
      - otherresource
      - otherresource/status
    verbs:
      - "*"
  # Optional. Matching paths will be omitted from the OCI image when building a
  # package.
  ignore:
    # Required. Currently only a path, relative to crossplane.yaml, is allowed.
  - path: examples/
```

The above `Provider` example is exhaustive - it contains all supported fields.
It should be treated as the authoritative example of Provider metadata. Note
that while it complies with Kubernetes API conventions _it is not a Kubernetes
custom resource_ - it is never created in an API server and thus no controller
reconciles it, however it does correspond to a `Provider` custom resource in
the `packages.crossplane.io` API group. These are two sides of the same coin:

* The `meta.packages.crossplane.io` `Provider` instructs Crossplane how to run
  the provider, and provides metadata for systems that build on Crossplane -
  i.e. package registries.
* The `packages.crossplane.io` `Provider` is submitted to the Crossplane API
  server in order to declare that a provider should be installed and run. The
  Crossplane package manager runs a provider by unpacking its package and
  extracting its `meta.packages.crossplane.io` `Provider` configuration.

Note that the two have different audiences; the `meta.packages.crossplane.io`
file is authored by the provider maintainer, while the `packages.crossplane.io`
custom resource is authored by the platform operator.

Contemporary packages contain an `install.yaml` file that allows the package
author to specify how the package's controller (e.g. the provider) should be
invoked. This document proposes support for `install.yaml` be removed; packages
will now only install a specific kind of controller - a Crossplane provider. It
is possible that both the maintainer of the provider and the platform operator
will need to influence how the provider is deployed; e.g. by specifying env
vars, health checks, replica counts, or node selectors. Currently all providers
use a minimal `install.yaml`; they do not make use of their ability to influence
how the provider is run. This document recommends that the above `Provider`
types be extended when and as appropriate to configure these concerns.

### Packaging a Configuration

A packaged Crossplane configuration must be an OCI image consisting of:

1. A `crossplane.yaml` file containing a `kind: Configuration` at its root.
1. One or more `CompositeResourceDefinition` and/or `Composition` resources.
1. Optional additional metadata that is opaque to Crossplane in a `.crossplane`
   directory at the root of the package.

Below is an example `Configuration`, at `v1alpha`:

```yaml
# Required. Must be as below.
apiVersion: meta.packages.crossplane.io/v1alpha1
# Required. Must be as below.
kind: Configuration
# Required. Note that Crossplane is aware only of the name and annotations
# metadata fields. Other fields (e.g. labels) will be preserved but opaque.
metadata:
  # Required. Must comply with Kubernetes API conventions.
  name: configuration-example
  # Optional. Must comply with Kubernetes API conventions. Annotations are
  # opaque to Crossplane, which will replicate them to the annotations of a
  # PackageRevision when this package is unpacked, but otherwise ignore them.
  # Systems such as package registries may extend this specification to require
  # or encourage specific annotations.
  annotations:
    company: Upbound
    maintainer: Nic Cope <negz@upbound.io>
    keywords: cloud-native, kubernetes, example
    source: github.com/crossplane-contrib/config-example
    license: Apache-2.0
    description: |
      The Example configuration adds example resources to Crossplane.
    provider: example
# Required.
spec:
  # Optional. Specifies a Crossplane version that the package is compatible with.
  crossplane: v0.13.0
  # Optional. Used by Crossplane to ensure any dependencies of a configuration
  # installed and running before the configuration is installed.
  dependsOn:
    # Required. Specifies an OCI image containing a package dependency. This key
    # may be either 'provider' or 'configuration'. This is sugar; in either case
    # the package manager determines whether the depencency is really a Provider
    # or a Configuration by unpacking it and inspecting its kind.
  - provider: example/provider-example
    # Required. Will be extended to support version ranges in future, but
    # currently treated as a specific version tag.
    version: v0.1.0
    # Required. Specifies an OCI image containing a package dependency. This key
    # may be either 'provider' or 'configuration'. This is sugar; in either case
    # the package manager determines whether the depencency is really a Provider
    # or a Configuration by unpacking it and inspecting its kind.
  - configuration: example/some-dependency
    # Required. Will be extended to support version ranges in future, but
    # currently treated as a specific version tag.
    version: v0.2.0
  # Optional. Matching paths will be omitted from the OCI image when building a
  # package.
  ignore:
    # Required. Currently only a path, relative to crossplane.yaml, is allowed.
  - path: examples/
```

The above `Configuration` example is exhaustive - it contains all supported
fields. It should be treated as the authoritative example of Configuration
metadata. As with the `Provider` example above it is not a Kubernetes custom
resource. It corresponds to a `Configuration` custom resource in the
`packages.crossplane.io` API group. The `meta.packages.crossplane.io` file is
authored by the provider maintainer, while the `packages.crossplane.io` custom
resource is authored by the platform operator.

## Open Question: Unpacking and Indexing Packages

Two known systems consume Crossplane packages and their metadata; the Crossplane
package manager itself and the Upbound Cloud [registry]. In each case the system
introspects the filesystem inside the package OCI image, relying on well known
file paths (e.g. `.registry/app.yaml`) to extract package metadata. This will be
discouraged going forward. Instead packages will be required to supply an OCI
image `ENTRYPOINT` that emits the metadata necessary to run or index a package.
This 'unpack' format is currently undefined, and may influence:

* Whether Crossplane can truly be unopinionated about directory structures and
  paths within a package's OCI image.
* How metadata such as UI annotations and icons should be associated with the
  Crossplane resources they correspond to. This metadata is _mostly_ opaque
  to Crossplane, but the package manager does currently merge it onto the
  relevant Crossplane resources (e.g. CRDs) as annotations.

[packages-v1]: design-doc-packages.md
[packages-v2]: design-doc-packages-v2.md
[resource-packs]: design-doc-resource-packs.md
[OCI]: https://opencontainers.org/
[composition]: design-doc-composition.md
[registry]: https://upbound.io/browse

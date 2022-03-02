
# Package Format v2

* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Accepted

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
design][packages-v2] by formalising package metadata. This metadata serves two
purposes:

1. It provides the information the Crossplane package manager needs to manage
   the lifecycle of a package - to install or upgrade a provider or a
   configuration.
1. It describes what is packaged. This information is not used by Crossplane,
   but provides an API that other tools such as package registries can use to
   introspect a package.

Note that one goal of this design is to build a user experience around working
with 'providers' and 'configurations', rather than working with 'packages'. The
package itself will be positioned as strictly a delivery mechanism.

## Proposal

There are two key audiences of package metadata; authors and consumers. The
former are usually humans - a platform operator might author, describe, and
package Crossplane configuration. The latter are usually software - the package
manager might unpack, parse, and install the aforementioned configuration. This
document therefore proposes that two key 'views' of package metadata exist:

* The 'on disk format(s)'. The format authored by a platform operator.
* The 'packaged format'. The single supported machine readable format.

This document proposes that the packaged format be a [YAML stream] stored at
`/package.yaml` at the root of an OCI image. The 'on disk format' could take
many forms - for example static files or a Python script - as long as that
format is compiled to a `/package.yaml` file stored in an OCI image.

### The Packaged Format

All software that consumes a package must do so via its packaged format - a YAML
stream stored as `/package.yaml` in the base layer of an OCI image. The packaged
format will consist of a `Provider` or `Configuration` YAML document (described
below) and a series of Crossplane-specific YAML documents, which must be either:

* A `CustomResourceDefinition` that defines a Crossplane managed resource.
* A `CustomResourceDefinition` that defines a Crossplane provider configuration.
* A `CompositeResourceDefinition` that defines a Crossplane composite resource.
* A `Composition` that configures how Crossplane should reconciles a particular
  kind of composite resource.

Note that the order and length of this YAML stream is undefined.

Below is an example `Provider`, at `v1alpha1`:

```yaml
# Required. Must be as below.
apiVersion: meta.pkg.crossplane.io/v1alpha1
# Required. Must be as below.
kind: Provider
# Required. Note that Crossplane is aware only of the name and annotations
# metadata fields. Other fields (e.g. labels) will be preserved but opaque.
metadata:
  # Required. Must comply with Kubernetes API conventions.
  name: provider-example
  # Optional. Must comply with Kubernetes API conventions. Annotations are
  # opaque to Crossplane, which will replicate them to the annotations of a
  # ProviderRevision when this package is unpacked, but otherwise ignore them.
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
    # the package manager determines whether the dependency is really a Provider
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
```

The above `Provider` example is exhaustive - it contains all fields that
Crossplane currently intends to support. It is the authoritative example of
Provider metadata. Note that while it complies with Kubernetes API conventions
_it is not a Kubernetes custom resource_ - it is never created in an API server
and thus no controller reconciles it, however it does correspond to a `Provider`
custom resource in the `pkg.crossplane.io` API group. These are two sides of the
same coin:

* The `meta.pkg.crossplane.io` `Provider` instructs Crossplane how to run the
  provider, and provides metadata for systems that build on Crossplane - i.e.
  package registries.
* The `pkg.crossplane.io` `Provider` is submitted to the Crossplane API server
  in order to declare that a provider should be installed and run. The
  Crossplane package manager runs a provider by unpacking its package and
  extracting its `meta.pkg.crossplane.io` `Provider` configuration.

Note that the two have different audiences; the `meta.pkg.crossplane.io` file is
authored by the provider maintainer, while the `pkg.crossplane.io` custom
resource is authored by the platform operator.

Below is an example `Configuration`, at `v1alpha`:

```yaml
# Required. Must be as below.
apiVersion: meta.pkg.crossplane.io/v1alpha1
# Required. Must be as below.
kind: Configuration
# Required. Note that Crossplane is aware only of the name and annotations
# metadata fields. Other fields (e.g. labels) will be preserved but opaque.
metadata:
  # Required. Must comply with Kubernetes API conventions.
  name: configuration-example
  # Optional. Must comply with Kubernetes API conventions. Annotations are
  # opaque to Crossplane, which will replicate them to the annotations of a
  # ConfigurationRevision when this package is unpacked, but otherwise ignore
  # them. Systems such as package registries may extend this specification to
  # require or encourage specific annotations.
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
    # the package manager determines whether the dependency is really a Provider
    # or a Configuration by unpacking it and inspecting its kind.
  - provider: example/provider-example
    # Required. Will be extended to support version ranges in future, but
    # currently treated as a specific version tag.
    version: v0.1.0
    # Required. Specifies an OCI image containing a package dependency. This key
    # may be either 'provider' or 'configuration'. This is sugar; in either case
    # the package manager determines whether the dependency is really a Provider
    # or a Configuration by unpacking it and inspecting its kind.
  - configuration: example/some-dependency
    # Required. Will be extended to support version ranges in future, but
    # currently treated as a specific version tag.
    version: v0.2.0
```

The above `Configuration` example is exhaustive - it contains all fields that
Crossplane currently intends to support. It is the authoritative example of
Configuration metadata. As with the `Provider` example above it is not a
Kubernetes custom resource. It corresponds to a `Configuration` custom resource
in the `pkg.crossplane.io` API group. The `meta.pkg.crossplane.io` file is
authored by the provider maintainer, while the `pkg.crossplane.io` custom
resource is authored by the platform operator.

### An On-Disk Format

It is likely that there will be multiple on-disk formats; e.g. static files,
Python scripts, or a [cdk8s] app. This document describes the first such format;
static files.

Under this format:

1. An on-disk package directory contains a `crossplane.yaml` file at its root.
1. The `crossplane.yaml` file contains either a `Provider` or a `Configuration`.
1. The package directory contains zero or more Crossplane specific custom
   resources (e.g. a `Composition`). Crossplane is not opinionated about where
   under the directory these files exist.

The software responsible for building an OCI image from the on-disk format is
outside the scope of this document, but is expected to:

* Append the content of `crossplane.yaml` to `package.yaml`
* Discover and append all Crossplane resources contained to `package.yaml`
* Create a single-layer OCI image containing `package.yaml`.

The aforementioned software may also be responsible for reading additional
metadata from the package directory and including it in the OCI image.

### Additional Metadata

The Crossplane package format allows for two forms of additional metadata -
passthrough metadata and supplementary metadata.

Passthrough metadata is opaque to the package manager and Crossplane in general,
but is passed through from the package OCI image to the API server against which
Crossplane is running. Examples of passthrough metadata could include:

* Details about a provider or configuration, such as its maintainer or website.
* Information about packaged resources (CRDs, XRDs, etc) that is opaque to
  Crossplane, but that is interesting to other clients of the API server against
  which Crossplane is running.

Passthrough metadata must be included as annotations on the objects in the
`/package.yaml` file. Annotations scoped to the entire package should be on the
`Provider` or `Configuration` resource, and will be propagated unmodified by
Crossplane to their associated `ProviderRevision` or `ConfigurationRevision`.
Annotations scoped to a particular resource (e.g. a CRD) should be included on
that resource. Additional metadata must not be attached to `/package.yaml` in
any other way, including labels or additional resources.

Metadata that is not consumed by Crossplane and that does not need to be passed
through to the API server is known as supplementary metadata. This metadata may
be included in any format its author and consumer(s) see fit, as long as it does
not interfere with `/package.yaml`.

Note that Crossplane is currently unopinionated about what additional metadata a
package should contain. In future Crossplane may recommend standard additional
metadata in order to ensure consumers of such metadata (e.g. package registries
and user interfaces) use consistent metadata.

### Consuming The Packaged Format

Crossplane will consume the packaged format by extracting `/package.yaml` from
the package OCI image. The package manager will ignore all other files in the
OCI image. It expects `/package.yaml` to contain *only*:

* A single `Provider` or `Configuration` in the `meta.pkg.crossplane.io` group.
* Zero or more `CustomResourceDefinitions` (CRDs)
* Zero or more `CompositeResourceDefinitions` (XRDs).
* Zero or more `Compositions`.

The packaged CRDs, XRDs, and Compositions will be created (or updated) verbatim
in the API server against which the package manager is running. Annotations on
the `Provider` or `Configuration` metadata files will be propagated to their
corresponding `ProviderRevision` or `ConfigurationRevision`. Annotations on
CRDs, XRDs, or Compositions will be passed through unmodified. The package will
not install successfully if unsupported resources are found in `/package.yaml`.

### Installing and Running a Provider

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

## Revisions

* 2.2
  * Use a static, rather than dynamic, package format.
* 2.1
  * Removed the requirement for the `Provider` or `Configuration` metadata to be
    the first resource in the YAML stream.
  * Removed the `ignore` metadata field.

[packages-v1]: design-doc-packages.md
[packages-v2]: design-doc-packages-v2.md
[YAML stream]: https://yaml.org/spec/1.2/spec.html#stream//
[entrypoint]: https://github.com/opencontainers/image-spec/blob/79b036d80240ae530a8de15e1d21c7ab9292c693/config.md#properties
[resource-packs]: design-doc-resource-packs.md
[OCI]: https://opencontainers.org/
[composition]: design-doc-composition.md
[cdk8s]: https://github.com/awslabs/cdk8s


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
* The 'output stream'. The single supported machine readable format.

This document proposes that the output stream be a [YAML stream] emitted to
stdout when the package's OCI image [`entrypoint`][entrypoint] is invoked. The
'on disk format' could take many forms - for example static files or a Python
script - as long as that format is rendered as an output stream when invoked by
package consumers.

### The Output Stream

All software that consumes a package must do so via its output stream - a YAML
stream emitted to stdout when the package's OCI image entrypoint is invoked. The
output stream will consist of a `Provider` or `Configuration` YAML document
(described below), followed by a series of Crossplane-specific YAML documents,
which must be either:

* A `CustomResourceDefinition` that defines a Crossplane managed resource.
* A `CustomResourceDefinition` that defines a Crossplane provider configuration.
* A `CompositeResourceDefinition` that defines a Crossplane composite resource.
* A `Composition` that configures how Crossplane should reconciles a particular
  kind of composite resource.

Note that the order and length of this YAML stream is undefined, except that the
`Provider` or `Configuration` must be the first document in the stream.

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
  # Optional. Matching paths will be omitted when rendering the output stream.
  ignore:
    # Required. Currently only a path is allowed.
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
  # Optional. Matching paths will be omitted when rendering the output stream.
  ignore:
    # Required. Currently only a path is allowed.
  - path: examples/
```

The above `Configuration` example is exhaustive - it contains all supported
fields. It should be treated as the authoritative example of Configuration
metadata. As with the `Provider` example above it is not a Kubernetes custom
resource. It corresponds to a `Configuration` custom resource in the
`packages.crossplane.io` API group. The `meta.packages.crossplane.io` file is
authored by the provider maintainer, while the `packages.crossplane.io` custom
resource is authored by the platform operator.

### An On-Disk Format

It is likely that there will be multiple on-disk formats; i.e. static files,
Python scripts, or a [cdk8s] app. This document describes the first such format;
static files.

This document proposes that:

1. The package contain a `crossplane.yaml` file at its root.
1. The `crossplane.yaml` file contain either a `Provider` or a `Configuration`.
1. The package contain zero or more Crossplane specific custom resources (e.g. a
   `Composition` or a managed resource CRD). Crossplane is not opinionated about
   where in the package these files exist.
1. Any extended metadata that is opaque to Crossplane be stored in an
   appropriately named directory starting with a period - e.g. `.upbound`, or
   `.registry` - at the root of the package.

Note that 'the package' in this context means both the content of the OCI image
and the on-disk representation of that content; the OCI image should be built by
simply copying the content of a directory into a layer of the OCI image. The
base layer of packages that use this format should specify an entrypoint process
that will, when invoked:

1. Emit the content of `crossplane.yaml` to stdout.
1. Discover and emit all Crossplane custom resources contained in the package to
   stdout, with the exception of those resources ignored by the `ignore` stanza.

The entrypoint may optionally decorate the YAML stream using any extended
metadata (e.g. the contents of the `.registry` directory). Any such extended
extended metadata must be written as annotations (`metadata.annotations`) on one
or more documents in the YAML stream - it may not modify the documents in any
other way.

### Consuming an Output Stream

Packages must be consumed via their output stream - a YAML stream emitted to
stdout when the package's OCI image entrypoint is invoked. Package consumers
must not inspect the content of the OCI image; it should be treated as opaque.

All package entrypoints must support (or silently ignore) the `--context` flag.
This flag can be used by the package consumer to provide context around how it
is being consumed; i.e. whether the package is being consumed as part of the
Crossplane package manager's unpack and install process or by some other system
such as an indexer. The `--context=unpack` flag will be passed when the package
manager unpacks a package. The `--context` flag must only affect the annotations
(`metadata.annotations`) of zero or more documents in the output stream. It may
not alter the stream in any other way.

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

[packages-v1]: design-doc-packages.md
[packages-v2]: design-doc-packages-v2.md
[YAML stream]: https://yaml.org/spec/1.2/spec.html#stream//
[entrypoint]: https://github.com/opencontainers/image-spec/blob/79b036d80240ae530a8de15e1d21c7ab9292c693/config.md#properties
[resource-packs]: design-doc-resource-packs.md
[OCI]: https://opencontainers.org/
[composition]: design-doc-composition.md
[cdk8s]: https://github.com/awslabs/cdk8s

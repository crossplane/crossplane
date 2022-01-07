# Package Annotations

- Owner: Nic Cope (@negz)
- Reviewers: Crossplane Maintainers
- Status: Proposed

**This one-pager has been superseded by the [`xpkg` specification].**

## Background

[Packages][packages-v2] extend Crossplane with new functionality. The `Provider`
and `Configuration` Crossplane resources are powered by packages; they represent
a declarative intent to install a provider or configuration package.

Crossplane packages are OCI images that deliver a payload of valid Kubernetes
resources. Provider packages deliver CRDs for the managed resources a provider
defines, while Configurations deliver XRDs and Compositions. All packages
contain general package metadata in the form of a `crossplane.yaml` file, per
the [package format one pager][format-v2].

The `crossplane.yaml` file contains Kubernetes-like YAML, and is of kind
`Provider` or `Configuration` in API group `meta.pkg.crossplane.io`. The package
manager propagates any annotations from the `crossplane.yaml` file to its
corresponding `ProviderRevision` or `ConfigurationRevision` in the Kubernetes
API server. Such annotations are referred to as 'passthrough metadata' about a
package, because they are not meaningful to Crossplane or the package manager.

The package format one pager provides an example of a `crossplane.yaml` with
annotations such as `company` and `maintainer`, but is not opinionated about
the format of the annotations, or which annotations should exist.

## Goals

The goal of this document is to encourage - but not require - consistent
package annotations. e.g. To increase the likelihood that all package authors
will align on using the `maintainer` annotation to indicate the package's
maintainer.

## Proposal

This document recommends that packages include the following annotations:

```yaml
apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: example
  annotations:
    # The package's maintainers, as a short opaque text string.
    meta.crossplane.io/maintainer: Example T. Maintainer <example@example.org>

    # The URL at which the package's source can be found.
    meta.crossplane.io/source: https://github.com/crossplane/provider-example

    # The license under which the package's source is released.
    meta.crossplane.io/license: Apache-2.0

    # A one sentence description of the package.
    meta.crossplane.io/description: |
      The Google Cloud Platform (GCP) Crossplane provider adds support for
      managing GCP resources in Kubernetes.

    # A longer description, documentation, etc.
    meta.crossplane.io/readme: |
      provider-example is a really great Crossplane provider...

    # The provider's SVG icon URI. The SVG should be optimized for 24, 48, 65px
    # and 2x versions. Consumers are encouraged to support at least data URIs.
    # https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/Data_URIs
    meta.crossplane.io/iconURI: data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciLz4KBIWXMA

    # A 'human-friendly' name for this package.
    friendly-name.meta.crossplane.io: "Example Provider"

    # A 'human-friendly' name for an API group defined by this package.
    friendly-group-name.meta.crossplane.io/database.example.org: "Databases"

    # A 'human-friendly' name for a an API kind defined by this package. Note
    # that the kind is singular, not plural (i.e. instance not instances).
    friendly-kind-name.meta.crossplane.io/cloudsqlinstance.database.example.org: "CloudSQL Instance"
spec:
  controller:
    image: crossplane/provider-example:v0.1.0
```

Note that these annotations are not constrained to packages specifically. If it
makes sense (for example) for a specific XRD, Composition, or CRD to contain
this metadata it may.

[packages-v2]: design-doc-packages-v2.md
[format-v2]: one-pager-package-format-v2.md
[`xpkg` specification]: ../../docs/reference/xpkg.md

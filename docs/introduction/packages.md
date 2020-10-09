---
title: Packages
toc: true
weight: 104
indent: true
---

# Crossplane Packages

Crossplane packages are opinionated [OCI images] that contain a stream of YAML
that can be parsed by the Crossplane package manager. Crossplane packages come
in two varieties: [Providers] and Configurations. Ultimately, the primary
purposes of Crossplane packages are as follows:

- **Convenient Distribution**: Crossplane packages can be pushed to or installed
  from any OCI-compatible registry.
- **Version Upgrade**: Crossplane can update packages in-place, meaning that you
  can pick up support for new resource types or controller bug-fixes without
  modifying your existing infrastructure.
- **Permissions**: Crossplane allocates permissions to packaged controllers in a
  manner that ensures they will not maliciously take over control of existing
  resources owned by other packages. Installing CRDs via packages also allows
  Crossplane itself to manage those resources, allowing for powerful
  [composition] features to be enabled.
- **Dependency Management**: In future releases, Crossplane will be able to
  resolve dependencies between packages, automatically installing a package's
  dependencies if they are not present in the cluster.

## Building a Package

As stated above, Crossplane packages are just opinionated OCI images, meaning
they can be constructed using any tool that outputs files that comply the the
OCI specification. However, constructing packages using the Crossplane CLI is a
more streamlined experience, as it will performing build-time checks on your
packages to ensure that they are compliant with the Crossplane [package format].

Providers and Configurations vary in the types of resources they may contain in
their packages. All packages must have a `crossplane.yaml` file in the root
directory with package contents. The `crossplane.yaml` contains the package's
metadata, which governs how Crossplane will install the package.

### Provider Packages

A Provider package contains a `crossplane.yaml` with the following format:

```yaml
apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: provider-gcp
spec:
  controller:
    image: crossplane/provider-gcp-controller:master
```

> Note: The `meta.pkg.crossplane.io` group does contain actual CRDs that get
> installed into the cluster. They are strictly used as metadata in a Crossplane
> package.

The `spec.controller.image` fields specifies that the `Provider` desires for a
`Deployment` to be created with the provided image. It is important to note that
this image is separate from the package image itself. In the case above, it is
an image containing the `provider-gcp` controller binary.

A Provider package may optionally contain one or more CRDs. These CRDs will be
installed prior to the creation of the Provider's `Deployment`. Crossplane will
not install _any_ CRDs for a package unless it can determine that _all_ CRDs can
be installed. This guards against multiple Providers attempting to reconcile the
same CRDs. Crossplane will also create a `ServiceAccount` with permissions to
reconcile these CRDs and it will be assigned to the controller `Deployment`.

For an example Provider package, see [provider-gcp].

To build a Provider package, navigate to the package root directory and execute
the following command:

```
kubectl crossplane build provider
```

If the Provider package is valid, you will see a file with the `.xpkg`
extension.

### Configuration Packages

A Configuration package contains a `crossplane.yaml` with the following format:

```yaml
apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: my-org-infra
```

Currently, the only purpose of a Configuration's `crossplane.yaml` is to declare
that it is in fact a Configuration package type. However, future releases will
include additional fields for functionality such as specifying dependencies on
Provider packages.

A Configuration package may also specify one or more of
`CompositeResourceDefinition` and `Composition` types. These resources will be
installed and will be solely owned by the Configuration package. No other
package will be able to modify them.

To build a Configuration package, navigate to the package root directory and
execute the following command:

```
kubectl crossplane build configuration
```

If the Provider package is valid, you will see a file with the `.xpkg`
extension.

## Pushing a Package

Crossplane packages can be pushed to any OCI-compatible registry. If a specific
registry is not specified they will be pushed to Docker Hub.

To push a Provider package, execute the following command:

```
kubectl crossplane push provider crossplane/provider-gcp:master
```

To push a Configuration package, execute the following command:

```
kubectl crossplane push provider crossplane/my-org-infra:master
```

> Note: Both of the above commands assume a single `.xpkg` file exists in the
> directory. If multiple exist or you would like to specify a package in a
> different directory, you can supply the `-f` flag with the path to the
> package.

## Installing a Package

Packages can be installed into a Crossplane cluster using the Crossplane CLI.

To install a Provider package, execute the following command:

```
kubectl crossplane install provider crossplane/provider-gcp:master
```

To install a Configuration package, execute the following command:

```
kubectl crossplane install configuration crossplane/my-org-infra:master
```

Packages can also be installed manually by creating a `Provider` or
`Configuration` object directly. The preceding commands would result in the
creation of the following two resources, which could have been authored by hand:

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: provider-gcp
spec:
  package: crossplane/provider-gcp:master
  revisionActivationPolicy: Automatic
  revisionHistoryLimit: 1
```

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: provider-gcp
spec:
  package: crossplane/provider-gcp:master
  revisionActivationPolicy: Automatic
  revisionHistoryLimit: 1
```

> Note: These types differ from the `Provider` and `Configuration` types we saw
> earlier. They exist in the `pkg.crossplane.io` group rather than the
> `meta.pkg.crossplane.io` group and are actual CRD types installed in the
> cluster.

The `spec.revisionActivationPolicy` and `spec.revisionHistoryLimit` fields are
explained in the following section.

## Upgrading a Package

Once a package is installed, Crossplane makes it easy to upgrade to a new
version. Controlling this functionality is accomplished via the three `spec`
fields shown above. They are explained in detail below.

### spec.package

This is the package image that we built, pushed, and are asking Crossplane to
install. The tag we specify here is important. Crossplane will periodically
check if the installed image matches the digest of the image in the remote
registry. If it does not, Crossplane will create a new _Revision_ (either
`ProviderRevision` or `ConfigurationRevision`). If you do not for Crossplane to
ever update your packages without explicitly instructing it to do so, you should
consider specifying a tag which you know will not have the underlying contents
change unexpectedly (e.g. a specific semantic version, such as `v0.1.0`) or, for
an even stronger guarantee, providing the image with a `@sha256` extension
instead of a tag.

### spec.revisionActivationPolicy

Valid values: `Automatic` or `Manual` (default: `Automatic`)

This field determines what Crossplane does when a new revision is created, be it
manually by you specifying a new tag in the `spec.package` field, or
automatically if a general tag such as `latest` or `master` is used and the
underlying contents change. When a new revision is created for a package, it can
either be `Active` or `Inactive`.

An `Active` package revision attempts to become the _controller_ of all
resources it installs. There can only be one controller of a resource, so if two
`Active` revisions both install the same resource, one will fail to install
until the other cedes control.

An `Inactive` package revision attempts to become the _owner_ of all resource it
installs. There can be an arbitrary number of owners of a resource, so multiple
`Inactive` revisions and a single `Active` revisions can exist for a resource.
Importantly, an `Inactive` package revision will not perform any auxiliary
actions (such as creating a `Deployment` in the case of a `Provider`), meaning
we will not encounter a situation where two revisions are fighting over
reconciling a resource.

### spec.revisionHistoryLimit

Valid values: any integer, disabled by explicitly setting to `0` (default `1`)

When a revision transitions from `Inactive` to `Active`, its revision number
gets set to one greater than the largest revision number of all revisions for
its package. Therefore, as the number of revisions increases, the least recently
`Active` revision will have the lowest revision number. Crossplane will garbage
collect old `Inactive` revisions if they fall outside the
`spec.revisionHistoryLimit`. For instance, if my revision history limit is `3`
and I currently have three old `Inactive` revisions and one `Active` revision,
when I upgrade the next time, the new revision will be given the highest
revision number when it becomes `Active`, the previously `Active` revision will
become `Inactive`, and the oldest `Inactive` revision will be garbage collected.

> Note: In the case that `spec.revisionActivationPolicy: Manual` and you upgrade
> enough times (but do not make `Active` the new revisions), it is possible that
> activating a newer revision could cause the previously `Active` revision to
> immediately be garbage collected if it is outside the
> `spec.revisionHistoryLimit`. 

<!-- Named Links -->

[OCI images]: https://github.com/opencontainers/image-spec
[Providers]: providers.md
[composition]: composition.md
[package format]: https://github.com/crossplane/crossplane/blob/1aa83092172bdf0d2ed64754d33517c612ff7368/design/one-pager-package-format-v2.md
[provider-gcp]: https://github.com/crossplane/provider-gcp/tree/master/package

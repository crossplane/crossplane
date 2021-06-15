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
- **Dependency Management**: Crossplane resolves dependencies between packages,
  automatically installing a package's dependencies if they are not present in
  the cluster, and checking if dependency versions are valid if they are already
  installed.

## Table of Contents

The following packaging operations are covered in detail below:

- [Building a Package](#building-a-package)
  - [Provider Packages](#provider-packages)
  - [Configuration Packages](#configuration-packages)
- [Pushing a Package](#pushing-a-package)
- [Installing a Package](#installing-a-package)
- [The Package Cache](#the-package-cache)
  - [Pre-Populating the Package Cache](#pre-populating-the-package-cache)

## Building a Package

As stated above, Crossplane packages are just opinionated OCI images, meaning
they can be constructed using any tool that outputs files that comply the the
OCI specification. However, constructing packages using the Crossplane CLI is a
more streamlined experience, as it will perform build-time checks on your
packages to ensure that they are compliant with the Crossplane [package format].

Providers and Configurations vary in the types of resources they may contain in
their packages. All packages must have a `crossplane.yaml` file in the root
directory with package contents. The `crossplane.yaml` contains the package's
metadata, which governs how Crossplane will install the package.

### Provider Packages

A Provider package contains a `crossplane.yaml` with the following format:

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-gcp
spec:
  crossplane:
    version: ">=v1.0.0"
  controller:
    image: crossplane/provider-gcp-controller:v0.14.0
    permissionRequests:
    - apiGroups:
      - apiextensions.crossplane.io
      resources:
      - compositions
      verbs:
      - get
      - list
      - create
      - update
      - patch
      - watch
```

See all available fields in the [official documentation][provider-docs].

> Note: The `meta.pkg.crossplane.io` group does not contain custom resources
> that may be installed into the cluster. They are strictly used as metadata in
> a Crossplane package.

A Provider package may optionally contain one or more CRDs. These CRDs will be
installed prior to the creation of the Provider's `Deployment`. Crossplane will
not install _any_ CRDs for a package unless it can determine that _all_ CRDs can
be installed. This guards against multiple Providers attempting to reconcile the
same CRDs. Crossplane will also create a `ServiceAccount` with permissions to
reconcile these CRDs and it will be assigned to the controller `Deployment`.

The `spec.controller.image` fields specifies that the `Provider` desires for the
controller `Deployment` to be created with the provided image. It is important
to note that this image is separate from the package image itself. In the case
above, it is an image containing the `provider-gcp` controller binary.

The `spec.controller.permissionRequests` field allows a package author to
request additional RBAC for the packaged controller. The controller's
`ServiceAccount` will automatically give the controller permission to reconcile
all types that its package installs, as well as `Secrets`, `ConfigMaps`, and
`Events`. Any additional permissions must be explicitly requested.

> Note that the Crossplane RBAC manager can be configured to reject permissions
> for certain API groups. If a package requests permissions that Crossplane is
> configured to reject, the package will fail to be installed. 

The `spec.crossplane.version` field specifies the version constraints for core
Crossplane that the `Provider` is compatible with. It is advisable to use this
field if a package relies on specific features in a minimum version of
Crossplane.

> All version constraints used in packages follow the [specification] outlined
> in the `Masterminds/semver` repository.

For an example Provider package, see [provider-gcp].

To build a Provider package, navigate to the package root directory and execute
the following command:

```
kubectl crossplane build provider
```

If the Provider package is valid, you will see a file with the `.xpkg`
extension.

> Note that the Crossplane CLI will not follow symbolic links for files in the
> root package directory.

### Configuration Packages

A Configuration package contains a `crossplane.yaml` with the following format:

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: my-org-infra
spec:
  crossplane:
    version: ">=v1.0.0"
  dependsOn:
    - provider: crossplane/provider-gcp
      version: ">=v0.14.0"
```

See all available fields in the [official documentation][configuration-docs].

A Configuration package may also specify one or more of
`CompositeResourceDefinition` and `Composition` types. These resources will be
installed and will be solely owned by the Configuration package. No other
package will be able to modify them.

The `spec.crossplane.version` field serves the same purpose that it does in a
`Provider` package.

The `spec.dependsOn` field specifies packages that this package depends on. When
installed, the package manager will ensure that all dependencies are present and
have a valid version given the constraint. If a dependency is not installed, the
package manager will install it at the latest version that fits within the
provided constraints.

> Dependency resolution is an `alpha` feature and depends on the `v1alpha`
> [`Lock` API][lock-api].

For an example Configuration package, see [getting-started-with-gcp].

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
kubectl crossplane push provider crossplane/provider-gcp:v0.14.0
```

To push a Configuration package, execute the following command:

```
kubectl crossplane push configuration crossplane/my-org-infra:v0.1.0
```

> Note: Both of the above commands assume a single `.xpkg` file exists in the
> directory. If multiple exist or you would like to specify a package in a
> different directory, you can supply the `-f` flag with the path to the
> package.

## Installing a Package

Packages can be installed into a Crossplane cluster using the Crossplane CLI.

To install a Provider package, execute the following command:

```
kubectl crossplane install provider crossplane/provider-gcp:v0.12.0
```

To install a Configuration package, execute the following command:

```
kubectl crossplane install configuration crossplane/my-org-infra:v0.1.0
```

Packages can also be installed manually by creating a `Provider` or
`Configuration` object directly. The preceding commands would result in the
creation of the following two resources, which could have been authored by hand:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-gcp
spec:
  package: crossplane/provider-gcp:master
  packagePullPolicy: IfNotPresent
  revisionActivationPolicy: Automatic
  revisionHistoryLimit: 1
```

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: my-org-infra
spec:
  package: crossplane/provider-gcp:master
  packagePullPolicy: IfNotPresent
  revisionActivationPolicy: Automatic
  revisionHistoryLimit: 1
```

> Note: These types differ from the `Provider` and `Configuration` types we saw
> earlier. They exist in the `pkg.crossplane.io` group rather than the
> `meta.pkg.crossplane.io` group and are actual custom resources created in the
> cluster.

The default fields specified above can be configured with different values to
modify the installation and upgrade behavior of a package. In addition, there
are multiple other fields which can further customize how the package manager
handles a specific revision.

### spec.package

This is the package image that we built, pushed, and are asking Crossplane to
install. The tag we specify here is important. Crossplane will periodically
check if the installed image matches the digest of the image in the remote
registry. If it does not, Crossplane will create a new _Revision_ (either
`ProviderRevision` or `ConfigurationRevision`). If you do not wish Crossplane to
ever update your packages without explicitly instructing it to do so, you should
consider specifying a tag which you know will not have the underlying contents
change unexpectedly (e.g. a specific semantic version, such as `v0.1.0`) or, for
an even stronger guarantee, providing the image with a `@sha256` extension
instead of a tag.

### spec.packagePullPolicy

Valid values: `IfNotPresent`, `Always`, or `Never` (default: `IfNotPresent`)

When a package is installed, Crossplane downloads the image contents into a
cache. Depending on the image identifier (tag or digest) and the
`packagePullPolicy`, the Crossplane package manager will decide if and when to
check and see if newer package contents are available. The following table
describes expected behavior based on the supplied fields:

|                                 | `IfNotPresent`                                                                                                                                                                                                                                                                                                                                   | `Always`                                                                                                                                                                                                                                                       | `Never`                                                                                                                   |
|---------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|
| Semver Tag (e.g. `v1.3.0`)      | Package is downloaded when initially installed, and as long as it is present in the cache, it will not be downloaded again. If the cache is lost and the a new version of the package image has been pushed for the same tag, package could inadvertently be upgraded. <br><br>  **Upgrade Safety: Strong**                                      | Package is downloaded when initially installed, but Crossplane will check every minute if new content is available. New content would have to be pushed for the same semver tag for upgrade to take place. <br><br> **Upgrade Safety: Weak**                   | Crossplane will never download content. Must manually load package image in cache. <br><br> **Upgrade Safety: Strongest** |
| Digest (e.g. `@sha256:28b6...`) | Package is downloaded when initially installed, and as long as it is present in the cache, it will not be downloaded again. If the cache is lost but an image with this digest is still available, it will be downloaded again. The package will never be upgraded without a user changing the digest. <br><br>  **Upgrade Safety: Very Strong** | Package is downloaded when initially installed, but Crossplane will check every minute if new content is available. Because image digest is used, new content will never be downloaded. <br><br> **Upgrade Safety: Strong**                                    | Crossplane will never download content. Must manually load package image in cache. <br><br> **Upgrade Safety: Strongest** |
| Channel Tag (e.g. `latest`)     | Package is downloaded when initially installed, and as long as it is present in the cache, it will not be downloaded again. If the cache is lost, the latest version of this package image will be downloaded again, which will frequently have different contents. <br><br> **Upgrade Safety: Weak**                                            | Package is downloaded when initially installed, but Crossplane will check every minute if new content is available. When the image content is new, Crossplane will download the new contents and create a new revision. <br><br> **Upgrade Safety: Very Weak** | Crossplane will never download content. Must manually load package image in cache. <br><br> **Upgrade Safety: Strongest** |

### spec.revisionActivationPolicy

Valid values: `Automatic` or `Manual` (default: `Automatic`)

When Crossplane downloads new contents for a package, regardless of whether it
was a manual upgrade (i.e. user updating package image tag), or an automatic one
(enabled by the `packagePullPolicy`), it will create a new package revision.
However, the new objects and / or controllers will not be installed until the
new revision is marked as `Active`. This activation process is configured by the
`revisionActivationPolicy` field.

An `Active` package revision attempts to become the _controller_ of all
resources it installs. There can only be one controller of a resource, so if two
`Active` revisions both install the same resource, one will fail to install
until the other cedes control.

An `Inactive` package revision attempts to become the _owner_ of all resources
it installs. There can be an arbitrary number of owners of a resource, so
multiple `Inactive` revisions and a single `Active` revision can exist for a
resource. Importantly, an `Inactive` package revision will not perform any
auxiliary actions (such as creating a `Deployment` in the case of a `Provider`),
meaning we will not encounter a situation where two revisions are fighting over
reconciling a resource.

With `revisionActivationPolicy: Automatic`, Crossplane will mark any new
revision as `Active` when it is created, as well as transition any old revisions
to `Inactive`. When `revisionActivationPolicy: Manual`, the user must manually
edit a new revision and mark it as `Active`. This can be useful if you are using
a `packagePullPolicy: Automatic` with a channel tag (e.g. `latest`) and you want
Crossplane to create new revisions when a new version is available, but you
don't want to automatically update to that newer revision.

It is recommended for most users to use semver tags or image digests and
manually update their packages, but use a `revisionActivationPolicy: Automatic`
to avoid having to manually activate new versions. However, each user should
consider their specific environment and choose a combination that makes sense
for them.

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

### spec.packagePullSecrets

Valid values: slice of `Secret` names (secrets must exist in `namespace`
Crossplane was installed in, typically `crossplane-system`)

This field allows a user to provide credentials required to pull a package from
a private repository on a registry. The credentials are passed along to a
packaged controller if the package is a `Provider`, but are not passed along to
any dependencies.

### spec.skipDependencyResolution

Valid values: `true` or `false` (default: `false`)

If `skipDependencyResolution: true`, the package manager will install a package
without considering its dependencies.

### spec.ignoreCrossplaneConstraints

Valid values: `true` or `false` (default: `false`)

If `ignoreCrossplaneConstraints: true`, the package manager will install a
package without considering the version of Crossplane that is installed.

### spec.controllerConfigRef

> This field is only available when installing a `Provider` and is an `alpha`
> feature that depends on the `v1alpha1` [`ControllerConfig` API][controller-config-docs].

Valid values: name of a `ControllerConfig` object

Packaged `Provider` controllers are installed in the form of a `Deployment`.
Crossplane populates the `Deployment` with default values that may not be
appropriate for every use-case. In the event that a user wants to override some
of the defaults that Crossplane has set, they may create and reference a
`ControllerConfig`.

An example of when this may be useful is when a user is running Crossplane on
EKS and wants to take advantage of [IAM Roles for Service Accounts]. This
requires setting an `fsGroup` and annotating the `ServiceAccount` that
Crossplane creates for the controller. This could be accomplished with the
following `ControllerConfig` and `Provider`:

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: ControllerConfig
metadata:
  name: aws-config
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::$AWS_ACCOUNT_ID\:role/$IAM_ROLE_NAME
spec:
  podSecurityContext:
    fsGroup: 2000
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: crossplane/provider-aws:v0.15.0
  controllerConfigRef:
    name: aws-config
```

You can find all configurable values in the [official `ControllerConfig`
documentation][controller-config-docs].

## The Package Cache

When a package is installed into a cluster, Crossplane fetches the package image
and stores its contents in a dedicated package cache. By default, this cache is
backed by an [`emptyDir` Volume][emptyDir-volume], meaning that all cached data
is lost when a `Pod` restarts. Users who wish for cache contents to be persisted
between `Pod` restarts may opt to instead use a [`persistentVolumeClaim`
(PVC)][pvc] by setting the `packageCache.pvc` Helm chart parameter to the name
of the PVC.

### Pre-Populating the Package Cache

Because the package cache can be backed by any storage medium, users are able to
optionally to pre-populate the cache with images that are not present on an
external [OCI registry]. To utilize a package that has been manually stored in
the cache, users must specify the name of the package in `spec.package` and use
`packagePullPolicy: Never`. For instance, if a user built a `Configuration`
package named `mycoolpkg.xpkg` and loaded it into the volume that was to be used
for the package cache (i.e. copied the `.xpkg` file into the storage medium
backing the PVC), the package could be utilized with the following manifest:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: my-cool-pkg
spec:
  package: mycoolpkg
  packagePullPolicy: Never
```

Importantly, as long as a package is being used as the `spec.package` of a
`Configuration` or `Provider`, it must remain in the cache. For this reason, it
is recommended that users opt for a durable storage medium when manually loading
packages into the cache.

In addition, if manually loading a `Provider` package into the cache, users must
ensure that the controller image that it references is able to be pulled by the
cluster nodes. This can be accomplished either by pushing it to a registry, or
by [pre-pulling images] onto nodes in the cluster.


<!-- Named Links -->

[OCI images]: https://github.com/opencontainers/image-spec
[Providers]: providers.md
[provider-docs]: https://doc.crds.dev/github.com/crossplane/crossplane/meta.pkg.crossplane.io/Provider/v1
[configuration-docs]: https://doc.crds.dev/github.com/crossplane/crossplane/meta.pkg.crossplane.io/Configuration/v1
[lock-api]: https://doc.crds.dev/github.com/crossplane/crossplane/pkg.crossplane.io/Lock/v1alpha1
[getting-started-with-gcp]: https://github.com/crossplane/crossplane/tree/master/docs/snippets/package/gcp
[specification]: https://github.com/Masterminds/semver#basic-comparisons
[composition]: composition.md
[IAM Roles for Service Accounts]: https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html
[controller-config-docs]: https://doc.crds.dev/github.com/crossplane/crossplane/pkg.crossplane.io/ControllerConfig/v1alpha1
[package format]: https://github.com/crossplane/crossplane/blob/1aa83092172bdf0d2ed64754d33517c612ff7368/design/one-pager-package-format-v2.md
[provider-gcp]: https://github.com/crossplane/provider-gcp/tree/master/package
[emptyDir-volume]: https://kubernetes.io/docs/concepts/storage/volumes/#emptydir
[pvc]: https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim
[OCI registry]: https://github.com/opencontainers/distribution-spec
[pre-pulling images]: https://kubernetes.io/docs/concepts/containers/images/#pre-pulled-images

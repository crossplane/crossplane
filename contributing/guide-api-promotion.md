# Promoting an API

This guide describes the process for promoting an API in Crossplane from one
version to the next. This process should be handled with care over multiple
releases, as it has proven fairly easy for Crossplane's API promotions of the
past to be problematic for both upgrades and downgrades, as demonstrated by
[#6148], [#5932], and [#4400].

An explanation of Crossplane's feature lifecycle can be found in the [Feature
Lifecycle] docs page.

## Core Issue

The root cause we have encountered in past problematic API promotions is that we
try to drop a particular version from a CRD while there may still be resources
of that version stored in etcd. Kubernetes does not allow this action because it
can result in possible data loss.

Note that this can happen on both upgrades as well as downgrades. Let's briefly
summarize one common downgrade path where we've encountered this issue:

1. A Crossplane API is promoted to Beta and the storage version for the CRD is
   updated to Beta in the same release
1. User installs this new Crossplane version and creates a Beta resource that is
   stored in etcd as Beta
1. User downgrades to the previous Crossplane version where the CRD's Beta
   version did not exist
1. The Crossplane init container tries to update the CRD back to the Alpha
   version, which drops the Beta version
1. The Kubernetes API server rejects this action because there might still be
   Beta resources stored in etcd according to the CRD's
   [`status.storedVersions`] field
1. Crossplane init container crashes and the Crossplane pod restarts in a loop

## Guiding Principles

There are a few simple principles we can follow in order to avoid this common
scenario when promoting Crossplane APIs:

1. Never introduce or drop a CRD version while also bumping the storage version
   in the same release
1. Always migrate resources to the current storage version during Crossplane
   initialization

Adherence to these two principles should result in never dropping a CRD version
that still has resources of that version stored in etcd, for both the upgrade
and downgrade directions.

## Promotion Workflow

The following table outlines the steps to introduce and then promote an API
safely across multiple versions while maintaining safe upgrades and downgrades:

| Version | Description                | Alpha           | Beta            | Migration        |
|---------|----------------------------|-----------------|-----------------|------------------|
| v0.1    | introduce new API as alpha | storage, served | not exist       | none             |
| v0.2    | promote to beta            | storage, served | served          | migrate to alpha |
| v0.3    | bump storage to beta       | served          | storage, served | migrate to beta  |
| v0.4    | drop alpha                 | not exist       | storage, served | none             |

The same process could be applied again when promoting the API from Beta to v1
GA.

## Practical Pointers

This section contains some helpful pointers to areas of the codebase involved
in promoting an API. These are just general direction and not entirely
prescriptive, because you'll need to make specific decisions for your promotion
based on the workflow defined above.

1. Define the API types under the `apis` directory, for example
   [`apis/apiextensions/v1beta1`]
1. Set the storage version `+kubebuilder:storageversion` marker on the correct
   version, e.g., as shown here for [Usage v1alpha1]
1. Duplicate the API to other versions if needed using [`generate.go`], also
   instructing the [duplicate script] to set the storage version if needed
1. Run `nix run .#generate` to generate the CRDs and check them for sanity in the
   [`cluster/crds`] directory
1. Include a [migrator] if needed to ensure resources are migrated to the
   current storage version and the CRD status is updated to declare that it only
   has resources stored in etcd of the current storage version
    * **Note** that the version passed into the migrator is the **"old"**
      version you want to migrate **from**, not the target version you want to
      migrate to
1. Update the [feature flag] for this API to reflect its new maturity level
1. Update the [feature flag docs] to reflect the new maturity level there as well

<!-- Links -->
[#6148]: https://github.com/crossplane/crossplane/issues/6148
[#5932]: https://github.com/crossplane/crossplane/issues/5932
[#4400]: https://github.com/crossplane/crossplane/issues/4400
[Feature Lifecycle]: https://docs.crossplane.io/latest/learn/feature-lifecycle/
[`status.storedVersions`]: https://github.com/kubernetes/apiextensions-apiserver/blob/v0.32.0/pkg/apis/apiextensions/v1/types.go#L368-L376
[`apis/apiextensions/v1beta1`]: https://github.com/crossplane/crossplane/tree/release-1.18/apis/apiextensions/v1beta1
[Usage v1alpha1]: https://github.com/crossplane/crossplane/blob/release-1.18/apis/apiextensions/v1alpha1/usage_types.go#L89
[`generate.go`]: https://github.com/crossplane/crossplane/blob/release-1.18/apis/generate.go
[duplicate script]: https://github.com/crossplane/crossplane/blob/release-1.18/hack/duplicate_api_type.sh
[`cluster/crds`]: https://github.com/crossplane/crossplane/tree/release-1.18/cluster/crds
[migrator]: https://github.com/crossplane/crossplane/blob/release-1.18/cmd/crossplane/core/init.go#L75-L79
[feature flag]: https://github.com/crossplane/crossplane/blob/release-1.18/cmd/crossplane/core/core.go#L112-L134
[feature flag docs]: https://docs.crossplane.io/latest/software/install/#feature-flags
# Namespaced EnvironmentConfig

* Owner: Amund Tenstad (@tenstad)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

`EnvironmentConfig.apiextensions.crossplane.io` is a cluster scoped resource. It
originates from enabling compositions to [patch from generic
datasources](https://github.com/crossplane/crossplane/blob/main/design/one-pager-composition-environment.md)
containing "environment-dependent data". All composite resources (XRs) were
cluster scoped at the time.

Crossplane v2 enabled XRs to be namespaced. However, `EnvironmentConfig`
[remained
cluster-scoped](https://github.com/crossplane/crossplane/blob/main/design/proposal-crossplane-v2.md#ancillary-cluster-scoped-types),
as the resource is "selected and referenced by Compositions", which "remain
(only) cluster scoped". The v2 design also explicitly [disallowed namespaced XRs
to compose cluster-scoped
resources](https://github.com/crossplane/crossplane/blob/main/design/proposal-crossplane-v2.md#namespace-composite-resources),
due to the Kubernetes ownership constraints that [a namespaced resource cannot
own a cluster-scoped
resource](https://github.com/kubernetes/community/blob/95af16e046ae65d7c9d6ba68eb1389bf8462301f/contributors/devel/sig-architecture/api-conventions.md#object-references).

The [original `EnvironmentConfig`
design](https://github.com/crossplane/crossplane/blob/main/design/one-pager-composition-environment.md)
does not support dynamic environments created by namespaced XRs. As demonstrated
by
https://github.com/crossplane-contrib/function-environment-configs/issues/105, a
composition for a namespaced XR instantiating a new environment cannot produce
the cluster-scoped `EnvironmentConfig` containing the environment-dependent data
for other compositions to consume. This makes it difficult to share structured
data between compositions.

## Current Workarounds

- Creating the cluster-scoped `EnvironmentConfig` indirectly through the
  namespaced `Object.kubernetes.m.crossplane.io` managed resource from
  [`provider-kubernetes`](https://github.com/crossplane-contrib/provider-kubernetes).
- Using a `ConfigMap`, although its `data` field is limited to string values,
  whereas `EnvironmentConfig` supports arbitrary structured data.
- Storing the data on the `status` of the XR, and fetching it in other
  compositions.

## Goals

- Allow a Composition targeting a namespaced XR to create a structured schemaless
  environment data resource, without composing a cluster-scoped resource.
- Allow that resource to be selected by compositions of other XRs.
- Leave the existing cluster-scoped `EnvironmentConfig` and its users unchanged.

## Alternatives Considered

- https://github.com/crossplane/org/issues/123 suggests creating a new
  namespaced (and potentially also cluster-scoped) `EnvironmentConfig` as a
  managed resource owned by a provider. It would keep the resource outside of
  crossplane core, enabling evolving the resource further, but makes a core
  primitive a separate install.

## Proposed Solution

To not impact the existing cluster-scoped `EnvironmentConfig`, we suggest adding
a namespaced `NamespacedEnvironmentConfig.apiextensions.crossplane.io/v1beta1`
CRD to Crossplane core. The schema is the same as today's cluster scoped
resource, a single schemaless `data` object.

As a potential future improvement, introduce both resources in a new API group.
Following the standard naming convention of prefixing cluster-scoped resources
with Cluster it would be `ClusterEnvironmentConfig` and `EnvironmentConfig`.

# Controlled Rollout of Composition Functions (v2)

* Owner: Adam Wolfe Gordon (@adamwg)
* Reviewers: @negz, @bobh66
* Status: Draft

## Background

Previously, we proposed [a design][v1] that would allow changes to composition
functions to be rolled out to composite resources in a controlled fashion by
allowing compositions to reference a specific function revision and making it
possible for multiple revisions of a function to be active at once. The
background section of the original design describes the problem we wish to
solve.

This document proposes a new, simpler design to solve the problem, based on the
following observations:

1. Unlike other package types, functions do not package resources that should be
   installed in the cluster (their input types do get installed today, but this
   is not a desired behavior).
2. Unlike providers (the other current package type with a runtime component),
   functions are stateless gRPC servers, not controllers. Multiple instances (of
   the same or different revisions) will not interfere with one another.
3. It is uncommon for functions to have dependencies.

## Goals

From the original design:

* Allow users to progressively roll out changes to functions that compose
  resources. This solves [crossplane#6139].
* Maintain current behavior for users who do not wish to roll out function
  changes progressively.

New for this design:

* Allow users to use arbitrary function packages in their compositions without
  the need to pre-install the functions (as dependencies or otherwise).
* Allow mixing of function installation modes without duplicate runtime
  resources being created.

## Proposal

This document proposes three changes in Crossplane to allow for progressive
rollout of composition functions:

1. Allow composition pipeline steps to reference functions by OCI reference
   rather than by Function resource name.
2. Have the composition revision controller manage functions referenced by OCI
   reference, avoiding the need to pre-install such functions.
3. Introduce dedicated resource types for the package runtime controllers,
   providing a common substrate for functions managed by the composition
   revision controller and those installed the traditional way using the package
   manager. This also makes the package manager conceptually simpler, since each
   resource type will be reconciled by only one controller.

To limit risk, these changes will be introduced behind a feature flag, which
will initially be off by default.

Note that while this design addresses only composition functions, the pipeline
and package manager changes apply also to operation functions. Operations do not
have revisions, so controlled rollout of them is not relevant, but the other
simplifications in this design are applicable.

### Composition Changes

We will add a `package` field to the `functionRef` field in pipeline steps such
that functions can be referenced by OCI reference rather than name:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: custom-api.example.org/v1alpha1
    kind: AcmeBucket
  mode: Pipeline
  pipeline:
  - step: patch-and-transform
    functionRef:
      package: xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2
    input:
      # Removed for brevity
```

The `package` field, if given, must contain a fully-qualified OCI reference
(i.e., include a registry, repository, and tag or digest). It is invalid to
provide both `name` and `package`.

When `package` is provided, the composition revision controller will ensure that
the specified function is running by creating a `FunctionRuntime` resource for
it (see the package manager section below). If a `FunctionRuntime` already
exists, the composition revision will be added as an owner reference.

A user wishing to roll out a new version of `function-patch-and-transform`
simply updates their composition to reference the new version, resulting in
creation of a new composition revision and runtime creation for the new function
version. The existing `compositionUpdatePolicy` and
`compositionRevisionSelector` mechanisms on XRs can be used to progressively
roll out the change:

```yaml
apiVersion: custom-api.example.org/v1alpha1
kind: AcmeBucket
metadata:
  name: my-bucket
spec:
  compositionUpdatePolicy: Manual
  compositionRevisionSelector:
    matchLabels:
      release-channel: alpha
```

In addition to allowing for progressive rollout, this new way of specifying
functions is simpler and safer to use than the existing method. Users don't need
to specify all their functions as dependencies or otherwise pre-install
them. They also don't need to know how the package manager names dependency
packages to construct the right `functionRef.name` in their composition. They
are also guaranteed to get exactly the version they specify regardless of what
other changes happen in the cluster; pacakges can be specified by digest for
further safety.

### Package Manager Changes

Currently, both the package revision controller and the package runtime
controller operate on package revision resources (`FunctionRevision` and
`ProviderRevision`). The revision controller installs resources (CRDs, etc.)
from the package, while the runtime controller creates runtime resources
(deployments, services, etc.) for active revisions of packages that have a
runtime component.

In order to allow runtime-only packages, we will introduce new `FunctionRuntime`
and `ProviderRuntime` resources, which will be reconciled by the package runtime
controller. The revision controller will be changed to create a runtime resource
corresponding to the active revision; the runtime controller will no longer
operate on revisions directly.

This allows the composition revision controller to create its own
`FunctionRuntime` resources to run functions without creating `Function`s or
`FunctionRevision`s. The package revision and composition revision controllers
will use the same scheme to name their `FunctionRevision`s, ensuring that a
particular version of a function runs only once even if it's installed multiple
ways. I.e., a `FunctionRuntime` may be shared between a `FunctionRevision` and
one or more `CompositionRevision`s.

Note that this change has two side effects for functions installed by the
composition revision controller:

1. Their dependencies will not be installed, as there will be no revision
   inserted into the `Lock`.
2. They will not have corresponding package or package revision resources,
   meaning they will not show up in `kubectl get pkg` or similar commands.

### Function Runner Changes

Today, the function runner invoked by the composite controller finds the
function endpoint for a pipeline step by first listing all function revisions
with the `pkg.crossplane.io/package` label set to the function's name, then
finding the single active revision in the list. This logic will change as
follows:

1. If `functionRef.package` is specified, the relevant `FunctionRuntime`
   resource will be looked up directly using the `pkg.crossplane.io/source`
   label.
2. If `functionRef.name` is specified, the existing logic to find the active
   revision is unchanged, but the endpoint will be read from the
   `FunctionRuntime`'s status.

The signature of the `FunctionRunner` in `internal/xfn` will change such that
functions are called by their OCI ref, since there is no longer always a
Function resource name to use as a key. Internally, the runner's gRPC connection
pool will likewise be keyed by OCI ref.

## Alternatives Considered

* Do nothing. For functions that don't take input, users can work around the
  limitation today by installing multiple versions of a function with unique
  names and referencing these names in their compositions. This workaround
  effectively simulates what is proposed in this design without support from
  Crossplane itself. However, it does not work for functions that take input due
  to [crossplane#5294], and does not work well with the package manager's
  dependency resolution system when using Configurations.
* Make it easier for composition functions to take input from external sources
  such as OCI images or git repositories. This would allow function input to be
  versioned separately from both functions and compositions, with new
  composition revisions being created to reference new input versions. However,
  this change would introduce significant additional complexity to the
  composition controller, and would not solve the problem for functions that
  don't take input.
* The [previous design][v1], which had the package manager allow for multiple
  active revisions of a package and added revision selectors in compositions.

[crossplane#6139]: https://github.com/crossplane/crossplane/issues/6139
[crossplane#5294]: https://github.com/crossplane/crossplane/issues/5294
[v1]: ./defunct/one-pager-controlled-rollout-of-composition-functions.md

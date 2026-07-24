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

Though it's not called out explicitly in the original design doc, the underlying
problem that prevents controlled rollout of composition functions is that both
compositions and functions can have revisions, and these revisions interact
(composition revisions call function revisions), but the coupling is loose
(composition revisions don't refer directly to function revisions). The previous
design solved the problem by tightening the coupling (allowing composition
revisions to refer to function revisions explicitly), while this design instead
removes the coupling altogether (removing the need for function revisions).

This document proposes a new, simpler design to solve the problem, based on the
following observations:

1. Unlike other package types, functions do not contain resources that get
   installed in the cluster.
2. Unlike providers (the other current package type with a runtime component),
   functions are stateless gRPC servers, not controllers. Multiple instances (of
   the same or different revisions) will not interfere with one another.

## Goals

From the original design:

* Allow users to progressively roll out changes to functions that compose
  resources. This solves [crossplane#6139].
* Maintain current behavior for users who do not wish to roll out function
  changes progressively.

New for this design:

* Allow users to use arbitrary function packages in their compositions without
  the need to pre-install the functions (as dependencies or otherwise).
* Allow mixing of function installation modes.

## Proposal

This document proposes three changes in Crossplane to allow for progressive
rollout of composition functions:

1. Allow composition pipeline steps to reference functions by OCI reference
   rather than by Function resource name.
2. Have the composition revision controller manage functions and function
   revisions referenced by OCI reference, avoiding the need to pre-install such
   functions.
3. To accommodate (2), introduce the concept of "external revisions" for
   functions.

Note that while this design addresses only composition functions, the pipeline
and package manager changes apply also to operation functions. Operations do not
have revisions, so controlled rollout of them is not relevant, but the other
simplifications in this design are applicable.

### Composition and Operation Changes

We will add a `function` field alongside the `functionRef` field in pipeline
steps such that functions can be called by supplying an OCI reference rather
than a reference to an installed `Function` resource:

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
    function: xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2
    input:
      # Removed for brevity
```

The `function` field, if given, must contain a fully-qualified OCI reference
(i.e., include a registry, repository, and tag or digest). It is invalid to
provide both `function` and `functionRef`.

When `function` is provided, the composition revision controller will ensure
that the specified function is running by creating a `FunctionRevision` resource
for it and a `Function` resource with the revision included in
`spec.externalRevisions` (see the package manager section below). If a matching
`FunctionRevision` already exists, the composition revision will be added as an
owner reference. Similarly, if a matching `Function` already exists, it gets an
owner reference and the revision added to its `externalRevisions`. The same
applies to the operation controller (operations execute in a one-shot manner,
and as such do not have revisions).

A user wishing to roll out a new version of `function-patch-and-transform`
simply updates their composition to reference the new version, resulting in
creation of a new composition revision and function revision. The existing
`compositionUpdatePolicy` and `compositionRevisionSelector` mechanisms on XRs
can be used to progressively roll out the change:

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

In order to allow for `FunctionRevision`s that are not controlled by the package
manager, we will make two backward-compatible API changes for package resources:

1. Make `spec.package` optional.
2. Add `spec.externalRevisionRefs`.

A package with an empty `spec.package` does not have revisions managed by the
package manager. The status of the package is computed from the revisions
referenced in `externalRevisionRefs`. A package may have both `package` and
`externalRevisionRefs` (e.g., if it was installed as a dependency and also by
being referenced in a composition); in this case, the package manager ignores
`externalRevisionRefs`, but they are still present for visibility.

The package manager's resolver controller (which controls the `Lock` resource)
will be updated to allow multiple versions of a package to be installed at
once. This lets composition-controlled functions participate in dependency
resolution, meaning that a function version referred to in a composition and
also depended upon by another package will be installed only once.

### Function Runner Changes

Today, the function runner invoked by the composite controller finds the
function endpoint for a pipeline step by first listing all function revisions
with the `pkg.crossplane.io/package` label set to the function's name, then
finding the single active revision in the list. This logic will change as
follows:

1. If `function` is specified, the relevant `FunctionRevision` resource will be
   looked up directly using the `pkg.crossplane.io/source` label.
2. If `functionRef` is specified, the existing logic to find the active revision
   is unchanged, but the endpoint will be read from the `FunctionRevision`'s
   status.

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
* Introduce a new `FunctionRuntime` API to allow the composition controller to
  spin up functions without interacting with existing package manager types at
  all. This is somewhat simpler than the current proposal, but has two confusing
  side-effects: composition-controlled functions don't participate in dependency
  resolution, and users can't see composition-controlled functions in `kubectl
  get pkg`.
* The [previous design][v1], which had the package manager allow for multiple
  active revisions of a package and added revision selectors in compositions.

[crossplane#6139]: https://github.com/crossplane/crossplane/issues/6139
[crossplane#5294]: https://github.com/crossplane/crossplane/issues/5294
[v1]: ./defunct/one-pager-controlled-rollout-of-composition-functions.md

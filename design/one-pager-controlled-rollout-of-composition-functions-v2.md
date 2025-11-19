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
    function: xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform@sha256:c14068f64cfb2088eec75b1c5cc43425383211b75380b9891f1253b57b3c4fe6
    input:
      # Removed for brevity
```

The `function` field, if given, must contain a fully-qualified OCI reference
with the version specified by digest. We require digests here rather than tags
because it makes composition revisions truly immutable: installing the same
composition revision later or on a different cluster is guaranteed to run the
same function. It is invalid to provide both `function` and `functionRef`.

When `function` is provided, the composition revision controller will ensure
that the specified function is running by creating a `FunctionRevision` resource
for it and a parent `Function` resource with an empty `spec.package` (see the
package manager section below). The `FunctionRevision`'s name is derived
deterministically from the OCI reference, so all composition revisions that
reference the same function version share a single revision. The `Function`'s
name is derived from the OCI repository in the same way the package manager
names dependencies, so that functions installed by the package manager and
functions referenced by OCI reference share a `Function`.

If a matching `FunctionRevision` already exists, the composition revision will
be added as an owner reference. Similarly, if a matching `Function` already
exists, it is used as the parent and gets an owner reference. To avoid
garbage collecting resources it didn't create, the controller adds owner
references only to existing resources that already have at least one owner
(e.g., another composition revision). A `Function` created by the user or
package manager is used as-is. The same applies to the operation controller
(operations execute in a one-shot manner, and as such do not have revisions).

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
other changes happen in the cluster or registry.

### Package Manager Changes

In order to allow for `FunctionRevision`s that are not controlled by the package
manager, we will make two backward-compatible API changes for package resources:

1. Make `spec.package` optional.
2. Add `status.externalRevisionRefs` to `Function`.

An external revision is a `FunctionRevision` that has the
`pkg.crossplane.io/package` label set to the `Function`'s name but is not
controlled by the `Function`. The package manager discovers external revisions
by this label and records them in `status.externalRevisionRefs`; external
controllers don't need to update the `Function` to register their revisions.
The package manager still reconciles external revisions (pulling packages,
managing their runtime, etc.), but has no opinion on whether they are active.

A package with an empty `spec.package` does not have revisions managed by the
package manager. The status of the package is computed from its external
revisions: it is active if it has at least one external revision, and healthy
only if all its external revisions are healthy. Since multiple revisions may be
active, such a package has no current revision. A package may have both
`package` and external revisions (e.g., if it was installed as a dependency and
also by being referenced in a composition); in this case, the package manager
ignores external revisions when managing its own revisions, to avoid competing
with other controllers. Note that in this specific case there could be two
active revisions with the same OCI ref (i.e., same version of the function),
since the package manager and other controllers name revisions differently.

Since each function revision may be active at the same time as other revisions
of the same function, the package runtime will create a `Service` and TLS
server certificate per `FunctionRevision`, rather than per `Function`.

The package manager's resolver controller (which controls the `Lock` resource)
will be updated to allow multiple versions of a package to be installed at
once. External revisions are ignored during dependency resolution, so that a
package that depends on a function always ends up with a package manager-managed
revision. If the package manager needs to install a function as a dependency and
a `Function` with only external revisions already exists, it adopts that
`Function` by setting its `spec.package` and removing its owner references.

### Function Runner Changes

Today, the function runner invoked by the composite controller finds the
function endpoint for a pipeline step by first listing all function revisions
with the `pkg.crossplane.io/package` label set to the function's name, then
finding the single active revision in the list. This logic will change as
follows:

1. If `function` is specified, the relevant `FunctionRevision` resource will be
   looked up directly by the name derived from its OCI reference.
2. If `functionRef` is specified, the existing logic to find the active revision
   is unchanged, except that only revisions controlled by the `Function` (i.e.,
   not external revisions) are considered.

In both cases, the endpoint will be read from the `FunctionRevision`'s status.

The signature of the `FunctionRunner` in `internal/xfn` will change such that
functions are called by `FunctionRevision` name, since there is no longer always
a single active revision per Function resource. Internally, the runner's gRPC
connection pool will likewise be keyed by `FunctionRevision` name.

### Feature Flag

Support for referencing functions by OCI reference will introduced behind the
`--enable-pipeline-oci-references` feature flag, which starts out as alpha and
off by default. When the flag is disabled, pipeline steps that don't specify a
`functionRef` are reported as invalid.

Note that the package manager changes to support external revisions, and the
function runner changes, are not guarded by the feature flag. A user could
theoretically create a `FunctionRevision` and `Function` manually such that the
new external revision machinery is invoked, although without the flag there is
no way to make Crossplane do this itself.

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
* Make multiple sources explicit in the package API (i.e., add a plural
  `spec.packages` to the `Function` resource) and have the package manager
  manage and activate revisions for all specified sources. This keeps revision
  management entirely in the package manager, which is nice. However, it makes
  it impossible to determine the provenance of a particular revision: whether it
  was requested by the user, the dependency resolver, the composition revision
  controller, or multiple of these. This lack of provenance makes it difficult
  to clean up revisions that are no longer needed. The proposed design allows us
  to take advantage of built-in Kubernetes garbage collection for cleanup by
  setting owner references directly on the revisions.
* The [previous design][v1], which had the package manager allow for multiple
  active revisions of a package and added revision selectors in compositions.

[crossplane#6139]: https://github.com/crossplane/crossplane/issues/6139
[crossplane#5294]: https://github.com/crossplane/crossplane/issues/5294
[v1]: ./defunct/one-pager-controlled-rollout-of-composition-functions.md

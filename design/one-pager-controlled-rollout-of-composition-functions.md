# Controlled Rollout of Composition Functions

* Owner: Adam Wolfe Gordon (@adamwg)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

Crossplane allows multiple revisions of a composition to be available via the
CompositionRevision API. An XR can specify which revision to use by name or
label selector, with the newest revision being used by default. This allows
users to gradually roll out new revisions of compositions by either manually or
automatically (via a controller) updating the composition revision in use for
each XR.

With composition functions, some or all composition logic moves out of the
composition itself and into functions. Crossplane allows only a single revision
of a function to be active at once; the active revision is the only one with a
corresponding deployment in the control plane. While composition changes can
still be gradually rolled out, function changes are all or nothing: the new
version of a function is used by all composition revisions, and therefore all
XRs, immediately.

With "generic" functions such as `function-go-templating` or `function-kcl` that
take source code written inline in the composition as input, this is generally a
tolerable problem. The functions themselves change slowly compared with the
compositions using them, and the code that directly composes resources is
versioned with the compositions as it would be when using patch and
transform. The requirement to inline code into YAML also naturally keeps the
custom code users write for these functions relatively small and easy to
inspect.

With non-generic functions, the code responsible for composing resources lives
entirely outside of the composition and may be arbitrarily complex. This means
composition revisions cannot be used to gradually roll out changes to
composition logic: the logic is in functions, which have only one active
revision.

## Goals

* Allow users to progressively roll out changes to functions that compose
  resources. This solves [crossplane#6139].
* Maintain current behavior for users who do not wish to roll out function
  changes progressively.

## Proposal

This document proposes two changes in Crossplane to allow for progressive
rollout of composition functions:

1. Allow multiple revisions of a function to be active (i.e., able to serve
   requests) at once, and
2. Allow composition pipeline steps to reference a specific function revision so
   that different revisions of a composition can use different revisions of a
   function.

To limit risk, both of these changes will be introduced behind a feature flag,
which will initially be off by default. Since neither change is useful by
itself, a single feature flag will control both.

### Package Manager Changes

Crossplane already supports multiple revisions of function packages; however,
only one revision at a time can be active. The active revision is the only one
with a running deployment and there is a single endpoint (Kubernetes Service)
for each function. The service is updated by Crossplane to point at the active
revision’s deployment, but the endpoint is recorded in each function revision
resource and the composition controller looks up endpoints by finding the active
revision.

To allow for multiple active revisions, we will add a new field to the
`Function` resource called `activeRevisionLimit`. This setting controls how many
revisions Crossplane will keep active at any given time. Its name mirrors the
`revisionHistoryLimit` setting and its value must be no greater than the
`revisionHistoryLimit`. By default, the `activeRevisionLimit` will be 1,
maintaining today’s behavior. For all package types other than `Function`, 1
will be the only valid value for `activeRevisionLimit`, since multiple active
revisions do not make sense for providers or configurations.

For example, to maintain up to four revisions and up to two active revisions,
the user would create a function resource like the following:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-patch-and-transform
spec:
  package: xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2
  revisionHistoryLimit: 4
  activeRevisionLimit: 2
```

When `revisionActivationPolicy` is `Automatic`, the revisions with the highest
revision numbers (up to the limit) will be active. If a new revision is created
and `activeRevisionLimit` revisions are already active, the active one with the
lowest revision number will be deactivated. When `revisionActivationPolicy` is
`Manual`, `activeRevisionLimit` is ignored by the package manager and it's left
to users to activate and deactivate revisions as they wish.

We will update the package manager’s runtime to name services after the
associated package revision rather than the package, and create a service per
active revision. This is required to allow multiple active revisions to serve
traffic. The endpoints used by the composition controller to connect to
functions are already recorded in the function revision resources, so this is a
natural change (each active function revision will now have a distinct
endpoint).

One additional change is necessary in the package manager to enable the
composition changes described below. The package manager needs to copy labels
from packages to their revisions, so that users can set a label on a function
and then use it to select the relevant revision in their composition. Labels on
compositions already work this way (they get copied to composition revisions),
so this change will make package revisions more similar to composition
revisions.

### Composition Changes

To enable compositions to refer to specific function revisions, we will add two
new optional fields to composition pipeline steps: `functionRevisionRef` and
`functionRevisionSelector`, mirroring the `compositionRevisionRef` and
`compositionRevisionSelector` found in XRs. These new fields will allow
compositions to select a specific function revision by name or by label.

For example, a user wishing to roll out a new version of
`function-patch-and-transform` to only certain XRs may update their existing
installation of the function by applying the following manifest:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: function-patch-and-transform
  labels:
    release-channel: alpha
spec:
  package: xpkg.crossplane.io/crossplane-contrib/function-patch-and-transform:v0.8.2
  revisionHistoryLimit: 4
  activeRevisionLimit: 2
```

The package manager will create a new function revision with the
`release-channel: alpha` label. The user would then update their composition to
reference the labeled function revision, causing a new composition revision to
be created:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
  labels:
    release-channel: alpha
spec:
  compositeTypeRef:
    apiVersion: custom-api.example.org/v1alpha1
    kind: AcmeBucket
  mode: Pipeline
  pipeline:
  - step: patch-and-transform
    functionRef:
      name: function-patch-and-transform
    functionRevisionSelector:
      matchLabels:
        release-channel: alpha
    input:
      # Removed for brevity
```

Finally, they would update one or more XRs to use the new composition revision:

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

Today, the function runner invoked by the composition controller finds the
function endpoint for a pipeline step by first listing all function revisions
with the `pkg.crossplane.io/package` label set to the function's name, then
finding the single active revision in the list. This logic will change as
follows:

1. If `functionRevisionRef` is specified, the revision will be fetched directly
   by name.
2. If `functionRevisionSelector` is specified, the relevant `matchLabels` will
   be used when listing revisions (in addition to `pkg.crossplane.io/package`).
3. When iterating over multiple returned revisions (because the
   `functionRevisionSelector` is not given or matches multiple revisions), the
   highest numbered active revision will be used.

Note that in the case where no function revision is specified, and there is only
one active revision for the function, the behavior will not change from today.

If no active function revision is found for a pipeline step, the composition
pipeline will fail. In this case, the composition controller should set an error
condition on the XR and raise an event.

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

[crossplane#6139]: https://github.com/crossplane/crossplane/issues/6139
[crossplane#5294]: https://github.com/crossplane/crossplane/issues/5294

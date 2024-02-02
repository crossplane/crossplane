# Beta Composition Environment

- Owner: Philippe Scorsolini (@phisco)
- Reviewers: @turkenh 
- Status: Draft

# Proposal

This document proposes not promoting the Composition Environment feature to beta
in v1.15, nor setting a timeline for its promotion to beta, investing in
[enabling][xfn-extra-resources] Composition Functions to request extra resources
allowing them to reimplement the same functionality while exploring other
possible approaches. Read below for more details about the issues with the
current implementation and the proposed next steps.

# Background

The "Composition Environment" concept was introduced in the original
[one-pager](one-pager-composition-environment.md) as a way "to patch from
environment-dependent data sources".

To achieve that, a new resource was introduced, `EnvironmnentConfig`, a
cluster-scoped and typed `ConfigMap`-like resource, alongside the concept of an
`in-memory environment`.

As of Crossplane v1.14, for Patch-and-Transform Compositions, the initial
`in-memory environment` is built as follows:

```go
// simplified logic implemented by the composite.APIEnvironmentFetcher
func buildInitialEnvironment(composition) (env environment) {
  // start from the defaults, if any
	env := composition.spec.environment.defaultData

	// get all the EnvironmentConfigs selected by the composition, either by name or via label selectors
	for _, envConfig = range composition.GetSelectedEnvironmentConfigs() {
		env = merge(env, envConfig.data)
	}

	return env
}
```

And is then used to compose resources as follows:

```go
// simplified logic implemented by composite.PTComposer
func compose(composition, xr) {
	// build the initial in-memory environment
	env := buildInitialEnvironment(composition)

	// apply patches between the XR (composite resource) and the environment, spec.environment.patches
	env = composition.ApplyEnvironmentPatches(env, xr)

	// run the composition pipeline, applying all patches (spec.resources[*].patches) for each composed resource.
  // Patching between XR, env and composed resource
	for _, resource := range composition.spec.resources {
		for _, patch := range resource.patches {
			env = patch.ApplyToObjects(env, xr, resource)
		}
	}

	// env is now discarded
}
```

This way, “Patch and Transform” (P&T) Compositions became a 3-way operation, as
shown in the diagram below.

![beta-composition-environment-1][beta-composition-environment-1]

The initial implementation was expanded with additional features such as:

- resolve and resolution policies for EnvironmentConfigs, and various required
  improvements to the label-based selection, see
  [here](https://github.com/crossplane/crossplane/pull/3981).
- default environment per Composition, see
  [here](https://github.com/crossplane/crossplane/issues/4274).
- `FromFieldPathPolicy` for the `FromCompositeFieldPath` of
  `EnvironmentSourceSelectorLabelMatcher`, see
  [here](https://github.com/crossplane/crossplane/pull/4547).
- … and [more](https://github.com/crossplane/crossplane/issues/3770) are in
  flight at the time of writing.

Since its initial implementation, other Crossplane features were introduced too,
e.g.:

- Beta Composition Functions, see [here](https://github.com/crossplane/crossplane/issues/3751).
- Management Policies, see [here](https://github.com/crossplane/crossplane/pull/4421).
- Usage API, see [here](https://github.com/crossplane/crossplane/pull/4215).

We saw significant adoption of the Composition Environment feature, and,
although in alpha, many people already rely on it and hope we won't introduce
significant breaking changes, or at least that we will provide a migration path
of some kind. We should take it into account.

Early adopters were pretty vocal about this feature being hard to understand,
and we'll get into more details below.

# Current implementation

## Concepts

Part of the confusion around this functionality is due to the overlapping terms,
so let's first try providing clear definitions given the current implementation:

- `EnvironmentConfig`: a cluster-scoped and structured, but still schema-less,
  `ConfigMap`-like resource.
- `in-memory environment`: an in-memory object created and thrown away for each
  Composite Resource's reconciliation loop.

## Components

The Composition Environment feature can be divided into the following
independent components:

- `EnvironmentConfig` alpha resource itself
- sections of the `Compositions` stable API:
    - selecting and merging multiple `EnvironmentConfig`s as defined at
      `spec.environment.environmentConfigs`:
        - merging it with the default environment defined at
          `spec.environment.defaultData`
        - according to the resolve and resolution policies defined at
          `spec.environment.policy`
    - the `in-memory environment` as an additional source and target for
      patches:
        - to and from the Composite Resource via `CompositeFieldPath` patches
          defined at `spec.environment.patches`
        - to and from Composed Resources via `EnvironmentFieldPath` patches
          defined at `spec.resources[*].patches`

## Beta Composition Functions' Context

Beta Composition Functions added support for the Composition Environment by
[introducing](https://github.com/crossplane/crossplane/pull/4632) the concept
of `Context`, a key-value structure initially populated by Crossplane and then
passed down the whole pipeline of functions, feeding each one of them the
output of the previous one. The `in-memory environment` built by merging the
`EnvironmentConfigs` becomes just the value at a well-known key of the
`Context`, `apiextensions.crossplane.io/environment`, that functions such as
[crossplane-contrib/function-patch-and-transform](https://github.com/crossplane-contrib/function-patch-and-transform)
can rely on and modify as they see fit, as they can do for any other key in the
`Context`.

Composition Functions have been promoted to Beta quite recently. Still, the
overall feeling is that Functions-based `Composition`s could potentially
replace classical P&T soon, so we should probably keep that in mind while
designing the future evolution of this API.

So the composition logic becomes the following:

```go
// simplified logic implemented by composite.FunctionComposer
func compose(composition, xr) {
	// build the initial context, embedding the initial in-memory environment at a well-known key
	context := map[well]any{
		"apiextensions.crossplane.io/environment": buildInitialEnvironment(composition)
	}

	// run the function pipeline, feeding the env
	for _, function := range composition.spec.pipeline {
			context = function.run(context, xr)
	}

	// context (and so the embedded environment) is now discarded
}
```

## Feedbacks

### Usage patterns

We saw early adopters using this feature to:

- inject environment-specific information into Compositions by manually
  creating `EnvironmentConfig`s with the required information specific to each
  `environment`, selecting and using them as needed from Compositions.
- share information across composite resources by creating `EnvironmentConfig`s
  from a first `Composition` and consuming them from another one.
- use the `in-memory environment` to patch between composed resources without
  setting up fields in the composite resource's status.
- use the `in-memory environment` to temporarily hold data from different
  sources (the composite resource and/or different composed resources) to
  combine them in a subsequent patch.
- select `EnvironmentConfig`s by label based on info from the Composite
  resource.

### Issues

Early adopters' feedback clearly showed the following issues of the current
implementation:

- naming is confusing:
    - expecting `ToEnvironment` patches to persist the state to some
      `EnvironmentConfig`, showing confusion between `in-memory environment`
      and `EnvironmentConfig`s.
    - expecting the `environment` to be shared across all Composite resources
      using a `Composition`, hence shared by all Composite Resources using a
      specific `Composition`.
- debugging is complex because of the following issues:
    - confusion about the order in which patches are applied between the
      Composite Resource, the `in-memory environment`, and Composed Resources.
    - the lack of visibility of intermediate results for the `in-memory
      environment`
- the `environment`related part of the `Composition` API is perceived as overly
  complex and error-prone, see
  [https://github.com/crossplane/crossplane/issues/4738](https://github.com/crossplane/crossplane/issues/4738).
- having to create an `EnvironmentConfig` to be able to access some information
  from an independent Managed Resource from a `Composition` is considered to be
  cumbersome.

## Promotion to beta

Given all the above, if we imagined splitting the functionality into its
components and promoting them independently, we, the maintainers, would feel
comfortable promoting to beta the following parts of the `Composition
Environment` :

- the `EnvironmentConfig` resource itself
- the `in-memory environment` as an additional source and target for patches
  defined at:
    - `spec.resources[*].patches`
    - `spec.environment.patches`, although these showed some discoverability
      issues and could benefit some more thinking, also keeping into
      consideration these are ignored for function-based Compositions.

While we wouldn't feel so comfortable promoting the EnvironmentConfig selection
part of the API at `spec.environment.environmentConfigs` in its current shape.

Before promoting the entire feature to beta, it's essential to address the known
issues and make the remaining parts of the Composition Environment more
straightforward.

### Independent Managed Resource referencing

Currently, a Composition author who wanted to address some information external
to the Composite and Composed Resources would have to go through the following
decision tree:

![beta-composition-environment-2][beta-composition-environment-2]

However, all options for existing MRs have some drawbacks:

- A: depending on how many times the Composition is used, could lead to many
  identical ObserveOnly MRs, one for each instantiation
- B: the created EnvironmentConfig is not automatically updated and feels
  unnecessary.
- C: it feels unnecessary, although it helps to have clear and defined
  interfaces between the two compositions; however, it requires some care to
  properly behave in case of multiple Composite resources using the same
  Composition, resulting in multiple EnvironmentConfigs being created.
- D: needs to be actively supported by providers, and there is a will to drop
  it one day to reduce the maintenance burden possibly; usually, they don't
  cross a single provider's boundaries.

For these reasons, a
[discussion](https://github.com/crossplane/crossplane/issues/4583) has been
going on about adding arbitrary Crossplane resource referencing capabilities to
the `Composition Environment`, to simplify the above decision tree above as
follows:

![beta-composition-environment-3][beta-composition-environment-3]

### Naming things is hard

Although the initial one-pager and the SIG channel on Slack were named after
the broader concept of `Composition Environment`, since its inception, the
functionality took the name of only one of its parts, `EnvironmentConfig`. This
was even reflected at implementation time by the chosen feature flag,
`--enable-environment-configs`. This caused a lot of confusion as it blurred
the thin line between `EnvironmentConfig` , the resource, and the `in-memory
environment` as a patch source/destination.

Patch types `FromEnvironmentFieldPath` and `ToEnvironmentFieldPath` refer to an
`Environment` which is actually the `in-memory environment`, according to the
definitions above. This `in-memory environment` is decoupled from the selected
`EnvironmentConfig`s and any change to it is not persisted back to any
`EnvironmentConfig` by default. This caused the aforementioned confusion.

As we already saw, beta Composition Functions added support for the `in-memory
environment` by wrapping it into a `Context` object at a well-known key,
`apiextensions.crossplane.io/environment`.

Referencing the `environment` twice in `spec.environment.environmentConfigs`
feels [redundant](https://github.com/crossplane/crossplane/issues/4591),
reinforcing the confusion between the `in-memory environment` and
`EnvironmentConfigs`.

### Selecting and merging EnvironmentConfigs

The API at `spec.environment.environmentConfigs` grew and now feels
uncomfortably complex. But at the same time, we are
[discussing](https://github.com/crossplane/crossplane/issues/4583) adding even
more complexity to it by adding generic Crossplane resource references to the
`Composition Environment` in some way.

To the best of our knowledge, it is mostly power users who are selecting and
merging multiple `EnvironmentConfig`s from a single Composition, potentially
abusing the functionality at times, instead of using simpler approaches.
However, we would still need the same knobs as long as we allow selecting by
labels, which we know is a widely used functionality as it allows us to refer to
dynamically created `EnvironmentConfig`s.

This could also be evaluated in light of the need to add arbitrary Crossplane
resource referencing capabilities to the `Composition Environment`, as
`EnvironmentConfigs` could become just another Crossplane resource to select,
as long as we preserve the current capabilities in some other way or form.

### Debugging

Debugging patch-and-transform (P&T) Compositions is known to be difficult, and
adding the `Composition Environment` to the mix complicated the situation
further.

Currently, users can already output the `in-memory environment` at any stage to
the Composite resource, any Composed resource, or a dedicated ad-hoc resource,
either as some annotation or to the resource `status`. However, this is
cumbersome for the user and still hard to understand because of the lack of
clarity around the patch application order.

We could define a few stages where we could in some way make it easier for
users to gain visibility into the `in-memory environment`, for example:

- resulting from the merge between the default data and all selected
  `EnvironmentConfig`s
- after having applied the patches at `spec.environment.patches`
- after having applied all patches, before throwing it away

See [here](https://github.com/crossplane/crossplane/pull/4702)  and
[here](https://github.com/crossplane/crossplane/issues/3967) for some related
discussions and possible approaches.

On the other hand, beta composition functions have already improved this aspect
by allowing the very same logic to be run locally using `crossplane beta
render`, possibly running against deployed functions in the near future, and
enabling the injection of arbitrary code at any point in the pipeline through
the usage of a dedicated function, e.g. `function-debug`. Allowing Composition
authors to do "print debugging" as in many other languages. So, we could
postpone any action on this and rely on the improved experience enabled by
Composition Functions, given we are
[proposing](https://github.com/crossplane/crossplane/issues/4746) to deprecate
the native P&T Compositions in the near future.

## Next steps

We currently believe https://github.com/crossplane/crossplane/issues/4739 would
solve all the above issues:

- Independent Managed Resource referencing:
    - functions could request any Crossplane resource, so EnvironmentConfigs when it make sense to, or resources directly when it doesn't.
- Naming Things:
    - the concept of `in-memory environment` would just a convention between functions, relying on the [`apiextensions.crossplane.io/environment`](http://apiextensions.crossplane.io/environment) key in the context, as `function-patch-and-transform` already does.
    - `--enable-environment-configs` would really mean only enabling `EnvironmentConfigs` as the rest of the `Composition Environment` would be taken care of by external functions, making the split between the two concepts clearer.
- Selecting and merging EnvironmentConfigs:
    - selection would be done by a dedicated function, e.g. `function-select-extra-resources`.
    - merging into the environment would be done by another dedicated function, e.g. `function-set-environment` or a more generic `function-set-context`.
    - **N.B.**: whether and how to preserve resolve policy `IfNotPresent` behavior would be left to the function implementation.
- Debugging:
    - visibility already addressed in the dedicated section above
    - selection would be more visible too by allowing passing resources via `--extra-resources` to `crossplane beta render`.

So the next steps would be to:
- Focus on https://github.com/crossplane/crossplane/issues/4739 for 1.15.
- Build the required functions on top of the above functionality to allow
  migrating from the current "native" `Composition Environment` implementation
  smoothly.
- Redirect new functionalities development to the above Functions.

But, what should we do regarding the "Composition Environment" feature promotion
to beta?

We could:
- **Option 1**: promote it as is in 1.15.
- **Option 2**: hold back promoting it to beta.

**Option 1** would mean promoting to beta an API we know we are uncomfortable
with. This could confuse users, and all the issues we discussed above
would still apply, but in beta. On the other hand, we would be able to promote
`EnvironmentConfigs` to beta, which we know is something we want to do. Then, in
a future release, we could reconsider the choice and deprecate or modify the API
as we see fit, when discussing its promotion to GA, respecting the deprecation
policy and timeline.

**Option 2** would mean that `EnvironmentConfigs` would not be promoted to beta
which is something we know we want to do. The issues above would still
apply, but at least not get promoted. We would postpone any decision to a future
release, possibly Crossplane 2.0, when we'll have more context around the future
of native patch and transform Compositions, and Composition Functions.

In both scenarios, if Crossplane 2.0 was to be released in the meantime, we
would maintain the current API in the 1.x releases, but could take the chance to
introduce breaking changes just in 2.0.

We decided to proceed with **Option 2** and therefore not to promote this
feature in v1.15, nor setting a timeline for its promotion to beta, investing in
[Composition Functions requesting extra resources][xfn-extra-resources] while exploring other
possible approaches.

<!-- Images -->
[beta-composition-environment-1]: assets/one-pager-composition-environment-beta/beta-composition-environment-1.png
[beta-composition-environment-2]: assets/one-pager-composition-environment-beta/beta-composition-environment-2.png
[beta-composition-environment-3]: assets/one-pager-composition-environment-beta/beta-composition-environment-3.png
[xfn-extra-resources]: https://github.com/crossplane/crossplane/issues/4739

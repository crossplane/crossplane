# Composition Validating Webhook

* Owner: Philippe Scorsolini (@phisco)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Validating webhooks in Kubernetes allow users to immediately know whether the
resources they are applying are correct or contain any issue that could result
in an error later on.

Validating webhooks have already been introduced in Crossplane for
`CompositeResourceDefinition`s ([here][original-webhook-pr]) after having
discussed them more broadly through a dedicated [design
doc][original-webhook-design-doc]. In the original design document
Composition’s validation was already mentioned and the conversation has
continued in a dedicated issue,
[#1476][original-composition-validation-webhook-issue].

Compositions describe how Crossplane should compose resources into higher level
Composite resources. From a user’s perspective, Compositions might be complex
to write as they can embed many other resources at `.spec.resources` and it is
relatively easy to introduce trivial errors, e.g. a misspelled field, either in
the `base` or the `patches` part of each of them. For this reason, having a
tighter feedback loop is of the utmost importance to guarantee a good Developer
Experience.

Existing implementations of Compositions’ validation are:

- In Crossplane itself, Compositions are only validated once actually selected
  for a Composite Resource. Validation is performed through a chain of
  validation functions, performing only a few important checks. The actual
  validation is done by rendering all the Composed resources and dry-run
  applying them, leveraging the `APIDryRunRenderer` component. This allows it
  to check the resources’ validity before actually applying them to reconcile
  any change required.
- A [Visual Studio Code plugin][vscode-plugin] was developed by Upbound
  implementing the Language Server Protocol (LSP) as a sub-command of the
  [upbound/up][upbound/up] CLI. The plugin is validating Compositions as part
  of a Package, after having pulled down all the Package’s dependencies
  declared in the `crossplane.yaml` manifest. It uses a forked version of
  Crossplane’s Composite resource’s reconciliation loop, stripped of any side
  effect, which allows it to generate all the Composed resources to be
  validated against the retrieved schemas. It’s able to validate Composition’s
  errors end to end, taking into consideration both `patches` and `bases`.
- In [upbound/upjet][upbound/upjet] in order to properly migrate from
  `crossplane-contrib/provider-` to `upbound/provider-` providers some
  validation logic around patches’ fieldpaths was developed. Similarly to the
  language server approach above, CRDs are available locally and fieldPaths are
  validated against the proper schema.
- [crossplane-contrib/crossplane-lint][crossplane-contrib/crossplane-lint] was
  recently released. It pulls down all the dependencies specified in a
  `.crossplane-lint.yaml` file so that it can validate all the resources in a
  Package using their schema. The main difference with the LSP above is firstly
  that it is implemented as a CLI tool that can be run locally or in CI
  pipelines, but it also differs in the approach, as it implements only a few
  specific validation rules, e.g. fieldPaths are correct according to the
  schemas, but does not validate the end to end result, which means it’s not
  reporting missing required fields in the generated Managed Resources for
  example.

## Goals

The goal of this one-pager is not to discuss whether a Validating Webhook for
Compositions would be useful, as it’s pretty clear and agreed that implementing
it would improve the developer experience for users working with Compositions.
Instead it should focus more on the definition and possible implementation of
it, given that it is not at all trivial.

## Proposal

As we saw, there are different implementations of composition validations out
in the wild, each focusing on different aspects of Compositions, so we need
first to define what we mean by validating Compositions.

### Validating Compositions

A Composition is made of a list of `resources`, each specifying a `base` and a
series of `patches`  to be applied on top of it. Different kinds of `patches`
are available, which might be using information coming from different sources,
e.g. values from the associated Composite Resource or EnvironmentConfigs.

We could split each `resource`’s validation in the following parts:

1. the `base` , which should be valid according to its schema.
2. `patches` , of which each should be validated according to its type and
   specified policies, but overall we can say that each of them should:
    1. have a valid path as a `fromFieldPath` according to the schema of the
       source object, if defined for the type of patch and required by the
       `PatchPolicy` defined.
    2. have a valid path as a `toFieldPath` according to the schema of the
       destination object, if defined for the type of patch.
    3. have correctly configured types, e.g. if `fromFieldPath` is a string,
       `toFieldPath` is an integer, the patch should be rejected, but not if
       there is a mapping correctly setting it.
3. the result of all the `patches` being applied to the `base` should be valid
   according to the Managed Resource schema.

Compositions are also defining the following fields that we should take into
consideration while validating:

- `patchSets`: a list of named patches that can be referenced by resources.
  Depending on the type of patch it could be impossible to validate them
  independently from the resource they would be used on. We could think of only
  validating them once they are dereferenced and associated to a specific
  resource.
- `functions`: a list of Composition Functions that can be used to generate or
  modify the Composed Resources. Given that it’s an `alpha` feature, we can
  think of not covering this in the first implementation, however we should
  keep in mind that having at least a function defined would invalidate the
  assumption that rendered Managed Resources should be valid according to the
  schema. Moreover, taking them into consideration during the validation phase
  would mean adding another external dependency, which could both reduce
  reliability and add latency.
- `environment`: contains a list of `EnvironmentConfigs` that can be referenced
  by specific types of patches and a list of`EnvironmentPatches` . Also an
  `alpha` feature, similar implications to what already said for `functions`,
  would require additional information and can invalidate the assumption that
  we render valid Composed Resources if defined.
- `writeConnectionSecretsToNamespace` : the name of the namespace the `Secret`
  containing the connection details should be created into. We could check
  whether it exists already, but that would be against the behaviour Kubernetes
  users are used to, e.g. a `Deployment` mounting a `Secret` which does not
  exist yet is considered valid but remains pending until it’s not created.
- `publishConnectionDetailsWithStoreConfigRef` : a reference by name to a
  `StoreConfig`, we could check that it exists, but similarly to the namespace
  it would go against the behaviour Kubernetes users might be expecting.

### Additional Constraints for the ValidatingWebhook use case

A few constraints that makes validating Compositions from a Validating Webhook
harder w.r.t. the cases mentioned in the Background:

1. The CRDs of the Managed Resources defined as `base` and traversed by
   `patches` might not yet have been created when the Composition is being
   validated.
2. The CRD of the Composite Resources specified by `CompositeTypeRef` and
   traversed by `patches` might not yet have been created when the Composition
   is being validated.
3. We do not have all the required CRDs locally, so we should get them from
   the API server, if available.
4. We do not have actual input values supplied from the Composite Resource that
   are required to render valid and applicable Managed Resources.
5. Some `alpha` features could require some additional interactions with
   external systems, e.g. `CompositionFunctions`.

### Solution Proposed

In order to be able to have Compositions properly validated we should cover all
the `resources` parts, while also taking into consideration all the constraints
above.

In order to address the possible absence of Managed Resources' CRDs, Composite
Resources' CRDs or any other required external resource at validation time we
should allow explicitly setting one of 2 modes, `strict` and `loose`(default) .
These should be configurable through an annotation, e.g.
`crossplane.io/composition-schema-aware-validation-mode`, on a Composition or maybe even
globally via a flag, and would respectively imply that any missing resource
should result in a direct rejection or a possibly incomplete validation.

In `loose` mode we should emit warnings in case any of the preconditions for a
check are not met, and skip further validation related to that check.

Regarding the approach, I propose to implement a Composition Validating Webhook
by validating each `resource` and final rendered `Composed
Resource`, querying the API Server to get the needed schemas.

I suggest we avoid validating `bases`. Doing that would require to ignore
validating required fields, as those could be set at a later stage by patches
or transformations. Assuming we implement full `patches` type coverage,
validating `bases` per se can be avoided, as any error should either be caught
validating the patches or can be attributed to the `base` itself.

To validate `patches`, we will need to implement ad-hoc functions, because the
current patching code is dealing directly with unstructured data and does not
take into consideration schemas at all, therefore not reporting any schema
related error. The code implemented for [upbound/upjet][upject-validation-code]
or [crossplane-contrib/crossplane-lint][crossplane-lint-validation-code] could
be good starting points for this.

To render the Composed Resources, we will need to reuse the Composite resources
reconciliation loop as much as possible, extracting the rendering logic to
remove any write operation it currently performs. Which is similar to what has
been done in the LSP [implementation][lsp-validation-code].

All `patches` will have to be applied in order to have the final Managed
Resource properly validated, so we'll need to provide all the required inputs
to the reconciler, to properly apply them. Inputs could be either actual
resources available or mocked ones.

Given that the current patching logic is only setting fields if they are
available in the source object, any mocked input object will need to have the
minimum required fields set in order to have valid rendered Managed Resources.
This is fine, as we could actually consider wrong patches setting required
values for the target and not required by the source, as that would lead to
missing required values in the final rendered resources.

I suggest, as a first implementation, to only focus on the `strict` validation
mode and perform only obvious checks things in `loose` (default) mode, as
introducing the uncertainty of missing resources would make the code much more
complex.

As said, some kinds of `patches` or specific features could introduce some
non-deterministic behaviors, e.g. Composition Functions. Their usage should be
identified early on in the validation logic and should short-circuit any
unnecessary check in both `strict` and `loose` modes.

#### Notes

A few additional notes worth highlighting or making more explicit w.r.t. the description above:

* We identified 3 increasingly complex types of validation, that we will
    probably introduce in different phases and PRs:
    1. **Logical validation**: checking that a Composition is valid per se,
       therefore respecting any constraint that might not have been enforced by
       the schema itself, but is expected to hold at runtime, e.g. patches of a
       specific type should define specific fields, or regexes should compile.
    1. **Schema validation**: checking that a Composition is valid according to
       all the schemas available, e.g. paths in patches are accessing valid
       fields according to the schema, or that input and output types of
       patches are correct according the source and target schemas. Can only be
       performed if needed schemas are available.
    1. **Rendered resources validation**: rendered Managed Resources that would
       be produced given a mocked minimal Composite Resource which selects the
       given Composition should be valid according to their schema. This will
       require we run the full Composite Resources reconciliation loop, or part
       of it if we identify a subset that satisfies our needs, providing mocked
       or read-only real resources as inputs. Can only be performed if needed
       schemas are available.
* To address **Rendered resources validation**, we will have to provide to the
    Composite Resources Reconciler a **mocked minimal Composite Resource**, at
    least given its current implementation. By "minimal" we mean having the
    minimal set of required fields set according to its schema so that it can
    be considered valid.

### Expected behaviour

Given the following valid Composition:

```
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xpostgresqlinstances.aws.database.example.org
  annotations:
    crossplane.io/composition-schema-aware-validation-mode: <selected_mode> # "loose" or "strict"
  labels:
    provider: aws
    guide: quickstart
    vpc: default
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance
  resources:
    - name: rdsinstance
      base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        spec:
          forProvider:
            region: us-east-1
            dbInstanceClass: db.t2.small
            masterUsername: masteruser
            engine: postgres
            engineVersion: "12"
            skipFinalSnapshotBeforeDeletion: true
            publiclyAccessible: true
          writeConnectionSecretToRef:
            namespace: crossplane-system
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-postgresql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.allocatedStorage"
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - fromConnectionSecretKey: port

```

The Validating Webhook should:

- reject it if in `strict` mode and no `RDSInstance` Managed resource and/or
  `XPostgreSQLInstance` Composite Type is deployed yet
- accept it in any other case

While, any of the errors highlighted in the following invalid example should
result in the Composition being rejected, except if in `loose` mode and both
`XPostgreSQLInstance` and `RDSInstance` have not yet been deployed.:

```
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xpostgresqlinstances.aws.database.example.org
  annotations:
    crossplane.io/composition-schema-aware-validation-mode: <selected_mode> # "loose" or "strict"
  labels:
    provider: aws
    guide: quickstart
    vpc: default
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance
  resources:
    - name: rdsinstance
      base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        spec:
          forProvider:
            # region: us-east-1                 # <== base missing required field error: e.g. region is not defined, but is a required field for RDSInstance and it's not set by any patch later
            dbInstanceClass: db.t2.small
            masterUsername: masteruser
            engin: postgres                     # <== base defining field not allowed error: e.g. engin instead of engine, field not accepted by RDSInstance
            engineVersion: "12"
            skipFinalSnapshotBeforeDeletion: true
            publiclyAccessible: true
          writeConnectionSecretToRef:
            namespace: crossplane-system
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-postgresql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.allocatedstorage"      # <== toFieldPath patch error: e.g. allocatedstorage instead of allocatedStorage, field not accepted by RDSInstance
        - fromFieldPath: "spec.parameters.storageGB"            # <== typing patch error: from an integer type, as defined by XPostgreSQLInstance, to a boolean, as defined by RDSInstance
          toFieldPath: "spec.forProvider.publiclyAccessible"    #
        - fromFieldPath: "spec.parameters.wrongParameter"       # <== fromFieldPath patch error: wrongParameter not defined by XPostgreSQLInstance
          toFieldPath: "spec.forProvider.engine"
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - fromConnectionSecretKey: port

```

### Additional validations

On top of the above schema based validations, we should also perform some
logical checks:

- that the supplied Composition does not attempt to mix named and anonymous
  templates. Already implemented [here][RejectMixedTemplates].
- that all template names are unique within the supplied Composition. Already
  implemented [here][RejectDuplicateNames].
- that all templates are named when Composition Functions are in use. Already
  implemented [here][RejectAnonymousTemplatesWithFunctions].
- that all Composition Functions have the required configuration for their
  type. Already implemented [here][RejectFunctionsWithoutRequiredConfig].
- that not both `writeConnectionSecretsToNamespace` and
  `publishConnectionDetailsWithStoreConfigRef` are used, as the latter
  deprecates the former.

### Known errors

The following classes of errors should be addressed by the Validating Webhook
if preconditions are met:

- final rendered resource not specifying a required field
- final rendered resource specifying an invalid field
- final rendered resource field type mismatch
- patches accessing invalid fields in either the source or destination object
- patches source and destination type mismatches taking into consideration
  transformations
- patches setting required target fields from non-required source fields
  fields

### Future Works

Ideally, the validation logic should be implemented as much as possible keeping
in mind that it should be reusable for the following use-cases too:

- linter
- language server
- future webhooks validating resources resulting in Compositions, e.g. Packages

This does not mean that an initial implementation should be structured as a
reusable library rightaway, but that it should at least try as much as possible
avoiding obvious blockers for its usage in such use-cases. So that, it could be
easily refactored later on to accommodate those usages once we get to work on
them.

## Alternatives Considered

Already covered in the Background section with pros and cons.

[original-webhook-pr]: https://github.com/crossplane/crossplane/pull/2919
[original-webhook-design-doc]: https://github.com/crossplane/crossplane/blob/master/design/design-doc-webhooks.md
[original-composition-validation-webhook-issue]: https://github.com/crossplane/crossplane/issues/1476
[vscode-plugin]: https://github.com/upbound/vscode-up
[upbound/up]: https://github.com/upbound/up/blob/main/internal/xpkg/snapshot/composition.go#L66
[upbound/upjet]: https://github.com/upbound/upjet
[crossplane-contrib/crossplane-lint]: https://github.com/crossplane-contrib/crossplane-lint
[upjet-validation-code]: https://github.com/upbound/upjet/blob/b1ed9245d05c5a0ace979f19f80fadd1b86f8e18/pkg/migration/patches.go#L96
[crossplane-lint-validation-code]: https://github.com/crossplane-contrib/crossplane-lint/blob/d58af636f06467151cce7c89ffd319828c1cd7a2/internal/xpkg/lint/linter/rules/composition_fieldpath.go#L127
[lsp-validation-code]: https://github.com/upbound/up/blob/e40b973f885d33707879d926f59bf4644fcaa6f5/internal/xpkg/snapshot/composition.go#L80
[RejectMixedTemplates]: https://github.com/crossplane/crossplane/blob/e89ef3c90ccb1bc9f1cb2bd0f2ba3451afbc7332/internal/controller/apiextensions/composite/composition_validate.go#L69
[RejectDuplicateNames]: https://github.com/crossplane/crossplane/blob/e89ef3c90ccb1bc9f1cb2bd0f2ba3451afbc7332/internal/controller/apiextensions/composite/composition_validate.go#L92
[RejectAnonymousTemplatesWithFunctions]: https://github.com/crossplane/crossplane/blob/e89ef3c90ccb1bc9f1cb2bd0f2ba3451afbc7332/internal/controller/apiextensions/composite/composition_validate.go#L110
[RejectFunctionsWithoutRequiredConfig]: https://github.com/crossplane/crossplane/blob/e89ef3c90ccb1bc9f1cb2bd0f2ba3451afbc7332/internal/controller/apiextensions/composite/composition_validate.go#L132


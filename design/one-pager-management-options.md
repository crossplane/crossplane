# Management Options

* Owners: Bob Haddleton (@bobh66)
* Reviewers: Crossplane maintainers
* Status: Proposed

## Background

Crossplane managed resources expose a `managementPolicies` field: an
array of strings that controls which reconciler actions are permitted on
the external resource. The field was introduced by the [ignore-changes
design][ignore-changes], which replaced the earlier enum-based
`managementPolicy` (`FullControl`, `OrphanOnDelete`, `ObserveOnly`) from
the [observe-only design][observe-only]. It is implemented in
`crossplane-runtime` by the [ManagementPoliciesResolver], which the
[Managed Reconciler] consults before each lifecycle phase.

The supported action values are:

- `Observe` - sync `status.atProvider` from the external resource.
- `Create` - create the external resource from `spec.forProvider`.
- `Update` - apply `spec.forProvider` changes to the external resource.
- `Delete` - remove the external resource when the managed resource is
  deleted.
- `LateInitialize` - populate unspecified `spec.forProvider` fields from
  the external resource.
- `*` - shorthand for all of the above.

The default value is `["*"]`, which preserves the historical full-control
behavior. An empty array (`[]`) pauses reconciliation. Common patterns
are expressed by listing subsets of actions, for example `["Observe"]`
for observe-only mode or omitting `Delete` to orphan the external
resource on managed resource deletion.

`deletionPolicy` (`Orphan` / `Delete`) predates `managementPolicies` and
remains on the API. During the transition, `LegacyManagementPoliciesResolver`
reconciles conflicts between the two fields. The [ignore-changes design]
[ignore-changes] intended `managementPolicies` to eventually replace
`deletionPolicy`, but the array representation has made that migration
difficult in practice.

### Limitations of `managementPolicies`

The array-based implementation has three structural limitations that
motivate this proposal:

1. **No per-action configuration.** Each action is either present or
   absent in the array. There is no place to attach action-specific
   behavior, such as whether Create should import an existing external
   resource.

2. **Orphan semantics require absence.** Expressing orphan-on-delete
   requires omitting `Delete` from the array. This is difficult to
   migrate from `deletionPolicy: Orphan` because the default
   `managementPolicies` value is `["*"]`, which includes Delete. A
   resource with `deletionPolicy: Orphan` and no explicit
   `managementPolicies` must be migrated to 
   `managementPolicies: ["Observe", "Create", "Update", "LateInitialize"]`
   which is cumbersome at best.

3. **Validation via enumerated combinations.** The
   [ManagementPoliciesResolver] validates against a fixed whitelist of
   action set combinations. Any new cross-action behavior requires adding
   another combination to the whitelist rather than composing per-action
   options.

This document proposes adding a `managementOptions` field that controls
both whether each action runs and how it behaves. `managementPolicies`
remains on the API and is not deprecated until `managementOptions`
reaches beta. See the [ignore-changes design][ignore-changes] for the
original use cases that motivated granular management control.

## Goals

* Introduce a `managementOptions` object that supports per-action
  configuration alongside the existing `managementPolicies` field.
* Graduate `managementOptions` to beta, then deprecate `managementPolicies`
  and remove it after a migration period.
* Preserve default behavior equivalent to today's `managementPolicies:
  ["*"]` / full control.
* Make common patterns (observe only, orphan on delete, no late init,
  pause) expressible without excessive YAML.
* Provide a clean, machine-migratable path from `deletionPolicy` and
  `managementPolicies`.
* Enable future per-action options without API churn.

## Non-goals

* Changing the fundamental reconciler phases (Create, Update, Delete,
  Observe, LateInitialize).

## Proposal

Add a `spec.managementOptions` object alongside the existing
`spec.managementPolicies` field. Each key in `managementOptions`
corresponds to one management action. Each value is an object with an
`enabled` field plus optional action-specific attributes.

`managementPolicies` remains fully supported while `managementOptions` is
in alpha. See [Coexistence and precedence](#coexistence-and-precedence)
and [Maturity and deprecation](#maturity-and-deprecation).

The five management actions remain the same as in the [ignore-changes
design][ignore-changes]:

- `observe` - Update `status.atProvider` to reflect the state of the
  external resource.
- `create` - Create the external resource using `spec.forProvider` fields.
- `update` - Update the external resource using `spec.forProvider`
  fields.
- `delete` - Delete the external resource when the managed resource is
  deleted. Orphan-on-delete is expressed by setting `delete.enabled` to
  `false`.
- `lateInitialize` - Update unspecified `spec.forProvider` fields to
  reflect the state of the external resource.

A special `*` (wildcard) key enables all actions. A `paused` flag
disables all actions.

### API shape

```yaml
spec:
  managementOptions:
    paused: false           # optional; disables all actions when true
    "*": {}                 # optional; enables all actions
    observe: {}
    create:
      importExistingResources: true
    update: {}
    delete: {}
    lateInitialize: {}
```

### Core types

```go
// ManagementOptions controls which reconciler actions run and how.
type ManagementOptions struct {
    // Paused disables all reconciler actions. Equivalent to
    // managementPolicies: [].
    // +optional
    Paused *bool `json:"paused,omitempty"`

    // All is the wildcard key ("*") that enables all actions.
    // +optional
    All *AllActionOptions `json:"*,omitempty"`

    Observe        *ObserveOptions        `json:"observe,omitempty"`
    Create         *CreateOptions         `json:"create,omitempty"`
    Update         *UpdateOptions         `json:"update,omitempty"`
    Delete         *DeleteOptions         `json:"delete,omitempty"`
    LateInitialize *LateInitializeOptions `json:"lateInitialize,omitempty"`
}

// ManagementActionOptions is the common base for all action options.
type ManagementActionOptions struct {
    // Enabled controls whether this action is allowed.
    // +optional
    // +kubebuilder:default=false
    Enabled *bool `json:"enabled,omitempty"`
}

type ObserveOptions struct {
    ManagementActionOptions `json:",inline"`
}

type CreateOptions struct {
    ManagementActionOptions `json:",inline"`
    // ImportExistingResources determines whether Create should take over
    // an existing external resource or fail.
    // +optional
    // +kubebuilder:default=true
    ImportExistingResources *bool `json:"importExistingResources,omitempty"`
}

type UpdateOptions struct {
    ManagementActionOptions `json:",inline"`
}

type DeleteOptions struct {
    ManagementActionOptions `json:",inline"`
}

type LateInitializeOptions struct {
    ManagementActionOptions `json:",inline"`
}
```

Action fields are pointers so the API can distinguish between a key
that is absent in YAML (`nil`) and a key that is present with an empty
object (`{}`).

### Coexistence and precedence

Both `managementPolicies` and `managementOptions` may be present on the
same managed resource while they coexist. The resolver applies the
following rules.

When `EnableAlphaManagementOptions` is **disabled**, `managementOptions`
is ignored completely — regardless of whether the field is omitted, empty,
or populated. Reconciliation uses `managementPolicies` and `deletionPolicy`
only, exactly as today.

When `EnableAlphaManagementOptions` is **enabled**:

| `managementOptions` | `managementPolicies` | Effective source |
|---------------------|----------------------|------------------|
| omitted | omitted | Either field alone; both default to full control (no conflict) |
| omitted | set | `managementPolicies` (converted to resolved options) |
| set | omitted | `managementOptions` |
| set | set | `managementOptions` takes precedence; `managementPolicies` is ignored |

When both fields are omitted, there is no conflict: `managementPolicies`
defaults to `["*"]` and `managementOptions` defaults to
`{"*": {"enabled": true}}`, which produce the same effective behavior.

When both fields are set, the resolver uses `managementOptions` only.
If the two fields imply different behavior, emit a warning condition so
the user can remove the redundant `managementPolicies` field.

A field counts as **set** for `managementOptions` when it is present in
the spec, including `{}`. A field counts as **omitted** when it is not
present in the spec at all (`nil` after unmarshaling).

## Defaulting and Resolution

`managementOptions` uses an opt-out model at the action-key level: only
explicitly listed actions (or those covered by `*`) are enabled.
Unlisted actions are disabled.

Defaulting happens at three levels:

| Level | When | Effective value |
|-------|------|-----------------|
| Field absent | `managementOptions` omitted from spec | `{"*": {"enabled": true}}` (or `managementPolicies` if that is set) |
| Field present, empty | `managementOptions: {}` | All actions disabled (paused) |
| Wildcard key | `"*": {}` present | All actions enabled |
| Action key present | e.g. `"observe": {}` | That action enabled; others disabled (unless `*` also present) |
| `enabled` attribute | Explicit on an action sub-object | Schema default `false`; see below |
| Action sub-object | Key present, value is `{}` | Defaults to `{"enabled": true}` |
| Action attrs only | Key present, only extra attrs set | `enabled` implied `true` |

The apparent tension between "`enabled` defaults to `false`" and
`"create": {}` means enabled" is resolved by **presence of the action
key**:

- **Key absent** (and not covered by `*`): action disabled.
- **Key present** with `{}`, omitted `enabled`, or only action-specific
  attributes: `enabled` defaults to `true`.

### Resolution algorithm

`Resolve()` produces a `ResolvedManagementOptions` with one fully merged
entry per action. The reconciler reads from the resolved output, not
the raw spec.

```
INPUT:  raw ManagementOptions (may be nil), ManagementPolicies (may be nil),
        feature.EnableAlphaManagementOptions (bool)
OUTPUT: ResolvedManagementOptions per action

1. If EnableAlphaManagementOptions is disabled → ignore managementOptions
     entirely; resolve from managementPolicies / deletionPolicy only; return

2. If managementOptions is set → resolve managementOptions per steps 3-6
   If managementOptions is omitted and managementPolicies is set →
     convert managementPolicies to ResolvedManagementOptions; return
   If both omitted → treat as {"*": {enabled: true}}; return

3. If paused == true → all actions disabled; return

4. Determine base from "*":
   - If "*" key absent:
       base.enabled = false for all actions
       baseAttrs = empty
   - If "*" key present:
       base.enabled = true for all actions
       baseAttrs = attributes from "*" sub-object

5. For each action A in [observe, create, update, delete, lateInitialize]:
   a. Start with base.enabled and baseAttrs
   b. If per-action key A is present:
        - Merge per-action sub-object over base:
          * If per-action.enabled is set → use it (can disable)
          * If per-action.enabled is omitted → enabled: true
          * Merge per-action-specific attrs over baseAttrs
   c. If "*" absent and per-action key A absent → disabled

6. Return resolved per-action config
```

When `*` is present, it enables all actions except any that are
explicitly set to `enabled: false`. Per-action keys merge their
additional attributes into the result.

### Verified equivalences

The following equivalences hold under the resolution algorithm:

| Input | Resolved result | `managementPolicies` equivalent |
|-------|-----------------|--------------------------------|
| both fields **omitted** | all actions enabled | `["*"]` |
| `managementOptions` **omitted** | defers to `managementPolicies` | (uses `managementPolicies` value) |
| `managementOptions: {}` | all actions disabled | `[]` |
| `managementOptions: {paused: true}` | all actions disabled | `[]` |
| `managementOptions: {"*": {}}` | all actions enabled | `["*"]` |
| `managementOptions: {observe: {}}` | observe only | `["Observe"]` |

> **Important:** Omitting `managementOptions` and setting it to `{}` are
> **not** the same. Omitting the field means full control. Setting `{}`
> explicitly pauses reconciliation.

### Delete semantics

```
delete enabled (key present, enabled true or omitted)
  → delete external resource when the managed resource is deleted

delete disabled (key absent or enabled: false)
  → do not call provider Delete; external resource is orphaned
```

Orphan-on-delete is expressed solely by disabling the delete action.
This provides an unambiguous migration target for `deletionPolicy:
Orphan`: `delete: {enabled: false}`. When `*` enables all actions,
disable delete explicitly:

```yaml
managementOptions:
  "*": {}
  delete:
    enabled: false
```

## Examples

### Default — both fields omitted

```yaml
spec:
  forProvider:
    region: us-east-1
# managementOptions omitted → {"*": {"enabled": true}}
# managementPolicies omitted → ["*"]
# No conflict; same effective behavior
```

### Full control — explicit wildcard

```yaml
spec:
  managementOptions:
    "*": {}
```

### Observe only

```yaml
spec:
  managementOptions:
    observe: {}
# Equivalent to managementPolicies: ["Observe"]
```

### Pause

```yaml
spec:
  managementOptions:
    paused: true
# Equivalent to managementPolicies: []
```

An empty object is also equivalent:

```yaml
spec:
  managementOptions: {}
```

### Orphan on delete (migrates `deletionPolicy: Orphan` or `managementPolicies` without Delete)

```yaml
spec:
  managementOptions:
    "*": {}
    delete:
      enabled: false
# Equivalent to managementPolicies:
# ["Create", "Update", "Observe", "LateInitialize"]
```

### No late initialization

```yaml
spec:
  managementOptions:
    "*": {}
    lateInitialize:
      enabled: false
# Equivalent to managementPolicies:
# ["Create", "Update", "Delete", "Observe"]
```

### Create with import control (attrs without `enabled`)

```yaml
spec:
  managementOptions:
    "*": {}
    create:
      importExistingResources: false
# Equivalent to {"*": {}, "create": {"importExistingResources": false}}
# All actions enabled via "*"; create merges importExistingResources: false
```

### Wildcard with per-action override

```yaml
spec:
  managementOptions:
    "*": {}
    lateInitialize:
      enabled: false
```

## `deletionPolicy` Deprecation

`deletionPolicy` is deprecated in favor of
`managementOptions.delete.enabled`. `deletionPolicy: Orphan` maps to
`delete: {enabled: false}`.
`deletionPolicy: Delete` maps to an enabled delete action (the default
when `*` is present or delete is listed explicitly). During the
transition:

1. If only `deletionPolicy` is set (legacy): honor it via
   `LegacyManagementOptionsResolver`.
2. If only `managementOptions` is set: use it directly.
3. If both are set and conflict: honor the non-default field; emit a
   warning condition.

## Implementation

### Management options resolver

The new resolver is implemented in `crossplane-runtime`, primarily in
the [Managed Reconciler][Managed Reconciler]. A new
`ManagementOptionsResolver` reads from `ResolvedManagementOptions` when
`EnableAlphaManagementOptions` is enabled. When the flag is disabled,
`managementOptions` is ignored and the existing
[ManagementPoliciesResolver] is used. When the flag is enabled but
`managementOptions` is omitted, the resolver falls back to
`managementPolicies`. Both fields remain on the `Manageable` interface.

For example:

- Create - [Create Section]
- Update - [Update Section]
- Delete - [Delete Section]
- Late Init - [Late Init Section]
- Observe - still required for external state; ensure
  `status.atProvider` is updated only when observe is enabled.

Resolver methods retain the same surface:

- `ShouldCreate()`
- `ShouldUpdate()`
- `ShouldDelete()`
- `ShouldLateInitialize()`
- `ShouldOnlyObserve()`
- `IsPaused()`
- `Validate()`

When resolving from `managementOptions`, the combination whitelist in
`ManagementPoliciesResolver` is not used. Validation becomes per-action
schema validation plus semantic rules (e.g. at least one action enabled
unless paused). The whitelist remains for resources that specify only
`managementPolicies`.

### API changes (`crossplane` repo)

**Files:** `apis/core/v2/policies.go`, `resource_cluster.go`,
`resource_namespace.go`

1. Define `ManagementOptions` and per-action option types.
2. Add `ManagementOptions` field to `ClusterManagedResourceSpec` and
   `ManagedResourceSpec`.
3. Keep `ManagementPolicies` on the API without deprecating it while
   `managementOptions` is alpha. `DeletionPolicy` deprecation is
   unchanged from today.
4. Implement `ApplyDefaults()`, `Resolve()`, and `Validate()` helpers.
5. Regenerate deepcopy, CRD schemas, and OpenAPI validation.

`ApplyDefaults()` sets `enabled: true` when an action key is present
but `enabled` is omitted. This cannot be expressed purely in CRD schema
defaults; a mutating admission webhook or in-controller defaulting is
required.

### Interface changes (`crossplane-runtime` repo)

Add `GetManagementOptions()` / `SetManagementOptions()` to the
`Manageable` interface alongside the existing `GetManagementPolicies()` /
`SetManagementPolicies()` methods.

Implement `LegacyManagementOptionsResolver` for resources still using
`deletionPolicy` during the transition. When `managementOptions` is
omitted, convert `managementPolicies` via the existing resolver logic.

Add `feature.EnableAlphaManagementOptions` to `crossplane-runtime/pkg/
feature/features.go` and gate `ManagementOptionsResolver` behind it via
`managed.WithManagementOptions()`.

### Migrating existing resources

Existing resources that specify only `managementPolicies` continue to
work unchanged while `managementOptions` is in alpha. Users may migrate
to `managementOptions` at their own pace. Suggested equivalences:

| managementPolicies | deletionPolicy | managementOptions |
|--------------------|----------------|-------------------|
| omitted / `["*"]` | Delete | omit both fields |
| omitted / `["*"]` | Orphan | `"*": {}` + `delete: {enabled: false}` |
| `["Observe"]` | * | `observe: {}` |
| without Delete / `["Create","Update","Observe","LateInitialize"]` | * | `"*": {}` + `delete: {enabled: false}` |
| `[]` | * | `paused: true` or `{}` |
| `["Create","Update","Delete","Observe"]` | * | `"*": {}` + `lateInitialize: {enabled: false}` |

When migrating, set `managementOptions` and remove `managementPolicies`
from the spec. While both are present, `managementOptions` takes
precedence.

An optional conversion webhook or CLI tool may translate
`managementPolicies` and `deletionPolicy` values to `managementOptions`
to aid migration. This is not required for backward compatibility.

### Maturity and deprecation

`managementOptions` and `managementPolicies` follow separate maturity
tracks:

| Phase | `managementOptions` | `managementPolicies` |
|-------|---------------------|----------------------|
| **Alpha (this design)** | Introduced behind `EnableAlphaManagementOptions` | Fully supported; **not** deprecated |
| **Beta** | Promoted to `EnableBetaManagementOptions` (or equivalent) | Deprecated; users encouraged to migrate |
| **GA / removal** | Default management control API | Removed after a migration period |

`managementPolicies` is not marked deprecated in the API or documentation
until `managementOptions` reaches beta. Premature deprecation would
discourage adoption of `managementOptions` while it is still alpha.

After `managementPolicies` is deprecated, it will be removed from the
API following a migration period. Target releases for beta graduation
and removal will be announced when this design is accepted.

### Feature gating

Introduce a new alpha feature flag for `managementOptions`, separate
from the existing `EnableBetaManagementPolicies` flag that gates
`managementPolicies`. The new flag is independent; enabling
`managementOptions` does not require or imply any change to how
`managementPolicies` is gated today.

**New flag (crossplane-runtime):**

- Constant: `feature.EnableAlphaManagementOptions`
- Provider reconciler option: `managed.WithManagementOptions()`
- Provider CLI flag: `--enable-alpha-management-options`

The flag is off by default. Providers opt in when they are ready to
support `managementOptions`. The field appears in managed resource CRD
schemas regardless of the flag.

When `EnableAlphaManagementOptions` is **disabled**:

- `managementOptions` is ignored completely, including when the field is
  present and non-empty in the spec.
- Reconciliation behavior is unchanged from today: `managementPolicies`
  under `EnableBetaManagementPolicies` and `deletionPolicy` on
  `LegacyManaged` resources continue to apply.
- No reconcile error is returned solely because `managementOptions` is
  set on a managed resource.

When `EnableAlphaManagementOptions` is **enabled**:

- `managementOptions` is honored per this design.
- `managementPolicies` continues to work as a fully supported field when
  `managementOptions` is omitted.

When `managementOptions` graduates to **beta**, promote the feature flag
to `EnableBetaManagementOptions` (or rename the existing alpha flag) and
deprecate `managementPolicies` in the API at that time.

## Alternatives Considered
### managementPoliciesOptions
Extend the existing `managementPolicies` with a `managementPoliciesOptions`
object that specifies the additional options for each action but does not
control whether the action is enabled. This was determined to be a
partial solution that does not completely solve the existing
limitations.

[observe-only]: design-doc-observe-only-resources.md
[ManagementPoliciesResolver]: https://github.com/crossplane/crossplane-runtime/blob/main/pkg/reconciler/managed/policies.go
[Managed Reconciler]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go
[Create Section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L943-L1031
[Delete Section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L865-L922
[Update Section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L1061-L1096
[Late Init Section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L1033-L1046
[ignore-changes]: one-pager-ignore-changes.md

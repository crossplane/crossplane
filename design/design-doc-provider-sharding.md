# Provider Sharding

* Owner: Erik Miller (@erik.miller)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Crossplane currently runs a single controller instance per provider. When a
Provider is installed, the package manager creates one ProviderRevision, which
creates one Deployment with one replica (`runtime.go`, default replicas=1).
That single provider pod is responsible for reconciling every managed resource
of every type the provider supports.

The [smaller providers design][design-smaller-providers] addressed a related
scaling dimension — too many CRDs per provider — by splitting large providers
into service-scoped packages. That design also documented the resource
consumption characteristics of upjet-based providers: ~600MB base memory for
the Crossplane provider process, ~300MB per Terraform provider process, and
roughly one CPU core to simultaneously reconcile 10 managed resources
(design-doc-smaller-providers.md, "Compute Resource Impact" section).

This proposal addresses the orthogonal dimension: the number of *resources*
being reconciled by a single provider instance. As the number of managed
resources grows, a single provider pod must maintain informer caches for all
of them and serialize reconciliation through a single process. Today, the
only scaling mechanism is vertical — increasing CPU and memory limits on the
provider Deployment via `DeploymentRuntimeConfig`. Horizontal scaling (running
multiple provider replicas) is not feasible because all replicas would watch
and attempt to reconcile the same resources, leading to conflicts.

This constraint is particularly relevant for upjet-based providers (such as
provider-aws, provider-azure, and OpenTofu/Terraform-based providers), which
spawn external processes (Terraform CLI and Terraform provider binaries) per
reconciliation, amplifying per-resource resource consumption relative to
native providers (design-doc-smaller-providers.md, "Compute Resource Impact"
section).

### Prior Art

#### Crossplane Ecosystem

- **[design-doc-smaller-providers.md][design-smaller-providers]**: Broke large
  providers into service-scoped packages to reduce CRD count. Documents
  upjet-based provider resource consumption characteristics. Complementary to
  this proposal.
- **[one-pager-crd-scaling.md][one-pager-crd-scaling]**: Analyzed performance
  issues with large numbers of CRDs, including client-side throttling and API
  server resource consumption. This proposal addresses resource count scaling,
  not CRD count scaling.
- **[one-pager-performance-characteristics-of-providers.md][one-pager-perf]**:
  Proposed tooling for measuring provider CPU utilization, memory utilization,
  and time-to-readiness for managed resources.
- **[provider-terraform#270][provider-terraform-sharding]**: An open PR adding
  label-based sharding to provider-terraform. Uses a `--shard-name` flag and
  `terraform.crossplane.io/shard` label with `cache.ByObject` label selectors
  for informer-level filtering. Nearly identical to this proposal, validating
  the approach. Notably documents that garbage collection of local state (e.g.,
  Terraform working directories) must list all resources globally — not just the
  shard's resources — to avoid incorrectly deleting another shard's state.
- **[provider-ansible#365][provider-ansible-sharding]**: A merged PR adding
  sharding to provider-ansible using a different mechanism: lease-based shard
  acquisition with FNV hash-based event filtering. Replicas acquire numbered
  shards via Coordination leases, then use a predicate
  (`hash(name) % totalShards == myShard`) to filter reconcile events. This
  filters at the reconciler level (events reach the informer but are dropped by
  predicates), which is less efficient than the informer-level filtering
  proposed here and in provider-terraform#270.

#### Kubernetes Ecosystem

- **[kubernetes-controller-sharding][k8s-controller-sharding]**: A generic
  framework (v0.13.0) for Kubernetes controller sharding. Uses a
  `ControllerRing` CRD, a central "sharder" component that assigns objects to
  shards via labels, lease-based shard membership, and a drain protocol for
  safe rebalancing. See [Relationship to kubernetes-controller-sharding]
  (#relationship-to-kubernetes-controller-sharding) for detailed analysis.
- **[KusionStack controller-mesh][controller-mesh]**: A sidecar-proxy-based
  approach to controller sharding. Injects a `ctrlmesh-proxy` container into
  operator pods that intercepts API server connections and injects label
  selectors into requests to enforce shard boundaries. Uses namespace-based
  hash labels (`ctrlmesh.kusionstack.io/sharding-hash`, values 0-31) for
  partitioning. The proxy-based approach is transparent to controllers but
  adds operational complexity (sidecar injection, proxy management). The
  project has not seen updates since mid-2024 (last release v0.2.0, July
  2024).
- **controller-runtime label selectors**: controller-runtime supports
  `cache.Options.DefaultLabelSelector` and per-GVK `cache.Options.ByObject`
  overrides, enabling filtered informer caches. This is the mechanism we
  build on.

### Why Core Functionality

provider-terraform#270 and provider-ansible#365 demonstrate that individual
providers are already implementing sharding independently, each with different
mechanisms:

| Provider | Mechanism | Filter Level | Shard Assignment |
|----------|-----------|-------------|------------------|
| provider-ansible | FNV hash + lease | Predicate (reconciler) | Automatic (hash) |
| provider-terraform | Label + cache selector | Informer (watch) | Manual (user labels) |

This fragmentation has several costs:

- **Duplicated effort**: Each provider reimplements sharding logic, including
  subtle correctness concerns (e.g., the GC safety invariant in
  provider-terraform#270 where shard-local GC must list all resources globally).
- **Inconsistent UX**: Different label keys (`terraform.crossplane.io/shard`
  vs a hash-based model), different CLI flags, different shard assignment models.
  Operators managing multiple providers must learn multiple sharding schemes.
- **No integration with Crossplane lifecycle**: Ad-hoc sharding requires
  operators to manually create and manage shard Deployments outside of
  Crossplane's package manager. Provider upgrades, RBAC, TLS certificates,
  and webhook configuration must be duplicated manually for each shard.
- **No label propagation**: Without core support, the `crossplane.io/shard`
  label is not propagated from Claims/XRs to composed resources. Each provider
  must independently handle shard assignment at the resource level rather than
  the composition level.

By making sharding a core Crossplane capability:

1. **crossplane-runtime** provides the `--shard-id` flag and cache
   configuration, so providers get sharding support without code changes.
2. **Crossplane core** propagates the shard label through the composition
   chain, so operators assign shards at the Claim/XR level — not per-resource.
3. **DeploymentRuntimeConfig** manages shard Deployments, so provider
   upgrades, RBAC, and TLS certificates are handled automatically for all
   shards.
4. A single, consistent label key (`crossplane.io/shard`) works across all
   providers.

## Goals

1. Enable horizontal scaling of providers by distributing managed resources
   across multiple provider instances using shard-based partitioning.
2. Maintain full backward compatibility — existing deployments with no sharding
   configuration continue to work unchanged.
3. Require minimal changes to Crossplane core. The bulk of the mechanism should
   live in crossplane-runtime, where providers already set up their watches.
4. Support gradual adoption — users can shard some resources while leaving
   others on a default (unsharded) provider instance.

## Non-Goals

1. Automatic shard assignment or load-based rebalancing. The initial
   implementation requires explicit user action to assign resources to shards.
2. Sharding of Crossplane core controllers (composition, claim syncing). Only
   provider controllers are sharded.
3. Cross-shard resource management. A shard provider only reconciles resources
   in its shard. However, cross-shard reference *resolution* (reading another
   shard's resources to resolve a reference) should be supported.

## Proposal

### Overview

We introduce a new label, `crossplane.io/shard`, that partitions managed
resources across provider instances. The mechanism has three parts:

1. **Shard label propagation**: The `crossplane.io/shard` label set on a Claim
   or XR is automatically propagated to all composed resources (managed
   resources), alongside existing labels like `crossplane.io/composite`.

2. **Shard-aware providers**: Providers accept a `--shard-id` flag (or
   `SHARD_ID` environment variable). When set, the provider configures its
   controller-runtime cache to watch only resources matching
   `crossplane.io/shard={shardID}`. Non-sharded resource types (ProviderConfig,
   Secrets) are exempted from this filter.

3. **Multiple provider deployments**: `DeploymentRuntimeConfig` gains a
   `shards` field. The ProviderRevision reconciler creates one Deployment per
   shard (plus a default instance for unlabeled resources).

### Shard Label Propagation

The label `crossplane.io/shard` flows through the standard Crossplane ownership
chain:

```
Claim (user sets label) → XR → Composed Resources (MRs)
```

**Claim → XR**: Already works. The claim syncer (`syncer_csa.go:84`,
`syncer_ssa.go:106`) propagates all non-reserved-k8s labels from the Claim to
the XR via `meta.AddLabels(xr, withoutReservedK8sEntries(cm.GetLabels()))`.
The `crossplane.io/shard` label will flow through this path with no code
changes.

**XR → Composed Resources**: Requires a change. Today, `RenderComposedResourceMetadata`
in `composition_render.go` (lines 105-113) propagates only three specific labels:
`crossplane.io/composite`, `crossplane.io/claim-name`, and
`crossplane.io/claim-namespace`. The shard label must be added to this set:

```go
// In composition_render.go, after the claim label block:
if v := xr.GetLabels()[xcrd.LabelKeyShard]; v != "" {
    metaLabels[xcrd.LabelKeyShard] = v
}
```

**Direct XR usage (no Claim)**: Users can set `crossplane.io/shard` directly on
the XR. The XR → Composed Resource propagation works the same way.

### Shard-Aware Providers

The key change lives in crossplane-runtime, not Crossplane core. When a provider
starts with `--shard-id=X`:

1. The controller manager's **reconciliation cache** is configured with a
   default label selector: `crossplane.io/shard=X`. This determines which
   resources trigger reconcile loops.
2. Per-GVK overrides exempt non-sharded types from the selector:
   - ProviderConfig (all types)
   - ProviderConfigUsage
   - StoreConfig
   - Secrets (used for credentials)
   - ConfigMaps
3. A separate **uncached client** (or unfiltered cache) is available for
   cross-resource reference resolution, allowing the provider to read
   resources in other shards without reconciling them. See
   [Cross-Shard Reference Resolution](#cross-shard-reference-resolution).
4. The managed reconciler itself requires no changes — it reconciles whatever
   the informer cache provides.

```go
// Pseudocode for provider main() setup
if shardID != "" {
    mgr, err := ctrl.NewManager(cfg, ctrl.Options{
        Cache: cache.Options{
            DefaultLabelSelector: labels.SelectorFromSet(labels.Set{
                "crossplane.io/shard": shardID,
            }),
            ByObject: map[client.Object]cache.ByObject{
                &v1alpha1.ProviderConfig{}:      {Label: labels.Everything()},
                &v1alpha1.ProviderConfigUsage{}: {Label: labels.Everything()},
                &corev1.Secret{}:                {Label: labels.Everything()},
            },
        },
    })
}
```

### Default Provider Behavior

When sharding is configured, the default provider instance (no `--shard-id`)
must handle resources that have no `crossplane.io/shard` label. There are two
possible approaches:

**Option A: Default watches only unlabeled resources.** The default provider
uses a `!crossplane.io/shard` label selector (label-does-not-exist). This
ensures each resource is reconciled by exactly one provider instance — either
the default or its assigned shard. No dual reconciliation is possible.

**Option B: Default watches everything.** The default provider applies no label
selector. It reconciles all resources, including those with shard labels. This
means sharded resources are reconciled by both the default and the shard
provider, creating dual reconciliation.

**Recommendation: Option A.** The default provider should use a
`!crossplane.io/shard` selector to avoid dual reconciliation. This is
expressible as a `metav1.LabelSelectorRequirement` with operator
`DoesNotExist`:

```go
// Default provider (no --shard-id) watches only unlabeled resources
if shardingEnabled && shardID == "" {
    mgr, err := ctrl.NewManager(cfg, ctrl.Options{
        Cache: cache.Options{
            DefaultLabelSelector: labels.Parse("!crossplane.io/shard"),
            // ... same ByObject overrides as shard providers
        },
    })
}
```

When sharding is NOT configured (no `spec.shards` in `DeploymentRuntimeConfig`),
the single provider instance applies no label selector at all, preserving
today's behavior exactly.

### Multiple Provider Deployments

`DeploymentRuntimeConfig` gains a `shards` field. When configured, the
ProviderRevision reconciler creates one Deployment per shard, plus a default
Deployment for unlabeled resources:

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  name: sharded-aws
spec:
  shards:
  - id: shard-a
  - id: shard-b
  - id: shard-c
  deploymentTemplate:
    spec:
      replicas: 1
      # ... other deployment settings shared across all shards
```

The ProviderRevision reconciler's Post hook (`runtime_provider.go`) iterates
over `spec.shards` and creates one Deployment per shard. Each shard Deployment
is named `{provider}-{shard-id}` and receives the `SHARD_ID` environment
variable. If the combined name exceeds the Kubernetes 63-character limit, the
provider name portion is truncated and a short hash suffix is appended to
ensure uniqueness (following the same pattern used by ReplicaSet pod naming).
A default Deployment (no `SHARD_ID`) is always created to handle unlabeled
resources.

When no `spec.shards` field is present (or it is empty), behavior is identical
to today: a single Deployment is created. This ensures backward compatibility.

All shard Deployments share the same:
- ServiceAccount (created by the ProviderRevision reconciler)
- RBAC (ClusterRoleBinding from the RBAC binding controller)
- Webhook Service (stateless validation load-balanced across pods)
- Provider image and configuration from `deploymentTemplate`

### Shard Lifecycle Operations

#### Adding a Shard

1. Add the shard to `DeploymentRuntimeConfig.spec.shards`. The ProviderRevision
   reconciler creates a new Deployment for the shard.
2. The new provider instance starts and watches for resources labeled
   `crossplane.io/shard=new-shard`. Initially it finds none.
3. Label existing Claims/XRs with `crossplane.io/shard=new-shard` to move
   resources to the new shard.
4. The shard label propagates to composed resources. The new shard provider
   begins reconciling them; the old provider (default or another shard) stops
   seeing them in its filtered cache.

#### Removing a Shard

Removing a shard requires draining it first. Resources labeled for a removed
shard will have no provider reconciling them.

1. **Drain**: Relabel all Claims/XRs currently assigned to the shard being
   removed. Change `crossplane.io/shard` to another shard or remove it
   entirely (moving resources to the default provider).
2. **Verify**: Confirm the shard provider's reconcile queue is empty (no
   resources match its selector).
3. **Remove**: Remove the shard from `DeploymentRuntimeConfig.spec.shards`.
   The ProviderRevision reconciler deletes the shard Deployment.

**Safety mechanism**: Before removing a shard Deployment, the reconciler
should check whether any managed resources still carry that shard label and
block removal with a status condition if resources remain. This prevents
accidental orphaning of resources.

#### Shard Rebalancing

Moving a resource between shards involves changing the `crossplane.io/shard`
label on the Claim or XR. The label change propagates to composed resources on
the next reconciliation cycle.

**What happens during the transition**:
- The old shard provider's filtered informer stops tracking the resource
  (it no longer matches the label selector). controller-runtime does not
  enqueue a reconcile event for objects that fall out of a filtered cache —
  the informer simply stops watching them.
- Even if a stale reconcile were somehow triggered, the managed reconciler
  (`crossplane-runtime/pkg/reconciler/managed/reconciler.go`) only deletes
  external resources when `meta.WasDeleted(managed)` returns true (line 1165),
  which requires an explicit deletion timestamp on the Kubernetes object.
  A relabeled object has no deletion timestamp; the deletion path is not
  entered.
- The new shard provider's filtered informer picks up the resource on its
  next list/watch sync and begins reconciling it.

This transition is safe because:
1. The Kubernetes object is never deleted — only relabeled. No deletion
   timestamp is set.
2. The managed reconciler's deletion logic is gated on `meta.WasDeleted()`,
   not on informer cache presence.
3. The external cloud resource is untouched.
4. There is a brief window where neither shard is actively reconciling.
   This window is bounded by the new shard provider's informer resync
   interval or the next watch event on the resource.

### Enabling on Existing Systems

Sharding is fully opt-in and backward compatible:

1. **No changes required for existing deployments**. Without the
   `crossplane.io/shard` label on any resources, and without any shard
   provider deployments, behavior is identical to today.

2. **Gradual adoption path**:
   a. Add `spec.shards` to the `DeploymentRuntimeConfig` referenced by the
      Provider. The reconciler creates shard Deployments alongside the
      default Deployment.
   b. Start labeling new Claims/XRs with `crossplane.io/shard=<shard-id>`.
      New resources go to shard providers.
   c. Optionally migrate existing resources by adding the shard label to their
      Claims/XRs.
   d. The default provider continues handling unlabeled resources indefinitely.

3. **Feature flag**: A feature flag `EnableAlphaSharding` gates the shard
   label propagation in Crossplane core. Providers can independently support
   `--shard-id` regardless of the core feature flag, since the flag only
   affects cache configuration.

### Who Assigns Shard Labels

#### Manual Assignment

Users explicitly set `crossplane.io/shard` on their Claims or XRs. This is
the simplest model and gives operators full control over resource distribution.

Unlabeled resources are NOT rejected. They are handled by the default provider
instance. This preserves backward compatibility and avoids a disruptive
migration for existing systems.

#### Policy-Based Assignment (Future)

A validating/mutating webhook or Crossplane admission policy could auto-assign
shards based on rules:
- Round-robin across configured shards.
- Namespace-based mapping (e.g., all resources in namespace `team-a` go to
  `shard-a`).
- Resource-type-based (e.g., all `rds.aws.upbound.io` resources go to
  `shard-rds`).

This would be implemented as a separate component (webhook or Composition
Function) rather than built into Crossplane core, and is out of scope for
the initial design.

#### Automatic Assignment (Future)

A controller could monitor provider load metrics and automatically assign or
rebalance shards. This is significantly more complex and is also out of scope
for the initial design.

### Observability

Operators need visibility into shard distribution and health to make informed
decisions about adding, removing, or rebalancing shards.

**Resource-level**: The `crossplane.io/shard` label on each managed resource is
queryable via `kubectl get <type> -l crossplane.io/shard=<shard-id>`. This
allows operators to count resources per shard and identify imbalances.

**Provider-level**: Each shard Deployment is a standard Kubernetes Deployment
with its own pod metrics (CPU, memory, restart count). Existing monitoring
(Prometheus scrape annotations are already added by the provider runtime in
`runtime_provider.go`) works per-shard pod without changes.

**ProviderRevision status**: The ProviderRevision status conditions should
report per-shard Deployment health. When a shard Deployment is unhealthy
(e.g., crash-looping), the ProviderRevision should surface this as a status
condition identifying the affected shard.

**Shard count metrics**: Providers could emit a Prometheus gauge
`crossplane_managed_resources_total{shard="<id>"}` indicating how many
resources each shard instance is reconciling. This enables alerting on shard
imbalance.

### Known Challenges and Mitigations

#### Cross-Shard Reference Resolution

A shard provider only **reconciles** (watches and manages the lifecycle of)
resources matching its shard label. However, providers also need to **read**
resources for cross-resource reference resolution — for example, an EKS
Cluster MR in shard A may reference a VPC MR in shard B to resolve its VPC ID.

If the provider's entire informer cache is filtered by shard label, it cannot
find the VPC in shard B. To support cross-shard reference resolution, the
shard label selector must apply only to the **reconciliation watches** (the
informers that trigger reconcile loops), not to all resource lookups.

**Implementation approach**: Reference resolution in crossplane-runtime should
use either:
1. An **uncached client** for reference lookups, bypassing the filtered
   informer cache and reading directly from the API server. This is slightly
   more expensive but simple and correct.
2. A **secondary, unfiltered cache** dedicated to reference resolution. This
   avoids per-lookup API server round-trips but adds memory overhead.

Approach (1) is recommended for the initial implementation due to its
simplicity. Reference resolution is infrequent relative to reconciliation (it
runs once to resolve a value, then the resolved value is cached in the resource
spec). The per-lookup API server cost is acceptable.

Note that all composed resources within a single XR hierarchy naturally share a
shard (the label propagates uniformly from XR to all children). Cross-shard
references only arise when referencing resources owned by a *different* XR.

#### Webhooks and Services

Today, each Provider has one webhook Service. The Service selector uses
`pkg.crossplane.io/revision` and the package type label (`runtime.go:332-349`).
Shard Deployments must carry the same pod labels so that the Service routes
traffic to all shard pods. Since webhook validation/mutation logic is stateless
(it validates the resource spec, not reconciliation state), load balancing
across shard pods is safe. The ProviderRevision reconciler must ensure that
shard Deployment pod templates include these standard labels.

#### RBAC

All shard Deployments share the same ServiceAccount created by the
ProviderRevision reconciler. The existing ClusterRoleBinding grants that
ServiceAccount access to all relevant CRDs. No RBAC changes are needed.

#### Composition Functions

Composition functions run in Crossplane core's XR reconciler, not in the
provider. Functions produce desired resource state; the XR reconciler applies
it. The shard label is added by `RenderComposedResourceMetadata` *after* the
function pipeline runs. Functions do not need to be shard-aware.

#### Garbage Collection

Owner-reference-based Kubernetes GC ties composed resources to the XR, not to
the provider. If a shard provider is temporarily down, the Kubernetes objects
remain and the XR controller may report composed resources as not-ready. This
is identical to today's behavior when a single provider is down. No new GC
concerns arise.

#### Status Aggregation

Each shard provider independently updates the `.status` of the managed
resources it reconciles. The XR controller reads status from all composed
resources (regardless of which shard provider updated them) and aggregates
readiness. No cross-shard coordination is needed.

#### ProviderConfigUsage

Each managed resource creates a `ProviderConfigUsage` to track its reference
to a `ProviderConfig`. These are cluster-scoped and exempted from the shard
label selector (so the ProviderConfig finalizer controller can see all usages).
This means every shard provider will list all ProviderConfigUsages, not just
those for its shard. This is acceptable because ProviderConfigUsage
reconciliation is lightweight (it only maintains a finalizer on the referenced
ProviderConfig). If the volume of ProviderConfigUsages becomes a concern at
extreme scale, a future optimization could add the shard label to
ProviderConfigUsages and have a dedicated, unsharded controller handle the
ProviderConfig finalizer logic.

#### ProviderConfig Sharing

ProviderConfigs are not sharded. All shard providers need access to the same
ProviderConfigs. The per-GVK cache override (`cache.Options.ByObject`) exempts
ProviderConfig types from the shard label selector, making them visible to all
shard instances.

### Relationship to kubernetes-controller-sharding

The [kubernetes-controller-sharding][k8s-controller-sharding] project (v0.13.0)
provides a generic framework for Kubernetes controller sharding. It introduces
a `ControllerRing` CRD, a central "sharder" component that assigns objects to
shards via labels, lease-based shard membership, and a drain protocol for safe
rebalancing. We evaluated adopting it directly vs building Crossplane-native
sharding.

**What it would give us:**
- Automatic shard assignment via a central sharder — no manual labeling.
- A drain protocol for safe rebalancing: the sharder sets a drain label, the
  current shard acknowledges by removing shard + drain labels, and the sharder
  reassigns. This eliminates the brief window during relabeling where neither
  shard is reconciling.
- Lease-based membership: shards announce themselves via Leases, and the
  sharder auto-detects new/dead shards. No need for explicit shard
  configuration — just scale the Deployment replica count.

**Why we chose not to adopt it directly:**

1. **Composition hierarchy mismatch**: The sharder assigns labels to individual
   objects. It does not understand Crossplane's Claim → XR → Composed Resource
   hierarchy. If the sharder independently assigns composed resources to
   different shards than their parent XR, a shard provider would see partial
   compositions. Crossplane must propagate the shard label from XR to composed
   resources itself, which means the sharder can only operate at the XR/Claim
   level — requiring Crossplane-specific integration that negates much of the
   "generic framework" benefit.

2. **New cluster-scoped dependency**: Adopting it requires installing a
   `ControllerRing` CRD and a sharder Deployment. This is a new component to
   install, monitor, and upgrade alongside Crossplane. If the sharder goes
   down, new resources are not assigned to shards (existing assignments are
   sticky via labels, so reconciliation continues, but new resources are
   unassigned).

3. **Label key coupling**: The project uses
   `shard.alpha.sharding.timebertt.dev/<ring-name>` as the label key.
   Crossplane would either adopt this external label convention (coupling to a
   third-party project's alpha API) or fork the label key (losing drop-in
   compatibility).

4. **Provider changes still required**: The sharder assigns labels, but
   providers still need to filter their watches by that label. The
   crossplane-runtime cache configuration changes are needed regardless.
   Additionally, the drain protocol requires providers to check for the drain
   label and acknowledge it — additional reconciler logic beyond simple cache
   filtering.

5. **DeploymentRuntimeConfig integration lost**: The sharder expects a single
   Deployment scaled by replica count — each replica acquires its own shard
   Lease. This is simpler but means Crossplane's package manager does not know
   about individual shards. There would be no per-shard health status in
   ProviderRevision and no safety checks on shard removal.

**What we adopt from it:**

The drain protocol is a valuable idea that we reference in [Future Work]
(#future-work). If the project matures and stabilizes its API, Crossplane
could consider adopting it as an optional sharder component for automatic shard
assignment, while retaining the Crossplane-native label propagation and
DeploymentRuntimeConfig integration.

## Implementation Scope

### crossplane core

- Define `LabelKeyShard = "crossplane.io/shard"` in `internal/xcrd/schemas.go`.
- Add shard label propagation in `RenderComposedResourceMetadata`
  (`composition_render.go`).
- Add `EnableAlphaSharding` feature flag in `internal/features/features.go`.
- Extend `DeploymentRuntimeConfig` with `spec.shards` field
  (`apis/pkg/v1beta1/deployment_runtime_config_types.go`).
- Update ProviderRevision reconciler to create N+1 Deployments
  (N shards + 1 default) in `runtime_provider.go`.
- Add shard health status to ProviderRevision status conditions.
- Add safety checks: block shard removal if labeled resources still exist.

### crossplane-runtime

- Add `--shard-id` flag to provider CLI helpers.
- When shard ID is set, configure `cache.Options.DefaultLabelSelector`.
- Add `cache.Options.ByObject` overrides for ProviderConfig, ProviderConfigUsage,
  StoreConfig, Secrets, ConfigMaps.
- Ensure reference resolution uses an uncached client to support cross-shard
  lookups.

### User responsibility

- Configure `DeploymentRuntimeConfig.spec.shards` for providers they want to
  shard.
- Label Claims/XRs with `crossplane.io/shard`.

## Future Work

- **Drain protocol for safe rebalancing**: The
  [kubernetes-controller-sharding][k8s-controller-sharding] project implements
  a drain protocol where a sharder sets a drain label on resources being
  reassigned, the current shard acknowledges by removing the shard and drain
  labels, and the sharder then reassigns to the new shard. This prevents the
  brief window during relabeling where neither shard is reconciling. Adopting
  a similar protocol could improve rebalancing safety for long-running
  reconciliations.
- Webhook or admission policy for automatic shard assignment.
- Shard load monitoring and rebalancing tooling.
- Per-shard `deploymentTemplate` overrides (e.g., different resource limits
  per shard).
- Optional integration with kubernetes-controller-sharding as an external
  sharder component, if the project matures and stabilizes its API.

## Alternatives Considered

### Hash-Based Sharding

[provider-ansible#365][provider-ansible-sharding] implements sharding using FNV
hash-based event filtering: each replica acquires a numbered shard via
Coordination leases, then applies `hash(resource.name) % totalShards == myShard`
as a predicate filter.

This has some advantages:
- No user-facing labels required; shard assignment is automatic.
- Resources are distributed uniformly by name hash.

We chose label-based sharding over this approach because:
- Hash-based filtering operates at the predicate (reconciler) level, not the
  informer level. Events for all resources still reach every replica's informer
  cache, consuming memory and network bandwidth. Label-based filtering uses
  `cache.Options.DefaultLabelSelector` which filters at the API server watch
  level — only matching events are sent over the wire.
- Hash-based assignment is opaque to operators. There is no way to inspect
  which shard owns a resource without computing the hash. Labels are visible
  via `kubectl get -l crossplane.io/shard=X`.
- Rebalancing (adding/removing shards) with hash-based assignment requires
  rehashing, which moves a large fraction of resources simultaneously.
  Label-based assignment allows granular, operator-controlled migration.
- Hash-based assignment cannot honor Crossplane's composition hierarchy.
  Label-based assignment propagates from Claim → XR → composed resources,
  ensuring all resources in a composition tree are co-located on one shard.

### Multiple Replicas with Leader Election per Resource

Run N replicas of the same provider and use leader election or consistent
hashing to assign individual resources to specific replicas. This avoids
user-facing shard labels but adds significant complexity:
- Requires a coordination mechanism (etcd, configmap-based election per
  resource, or consistent hashing).
- controller-runtime's built-in leader election is all-or-nothing (one leader
  reconciles everything), not per-resource.
- Consistent hashing requires all replicas to agree on the hash ring, adding
  a distributed systems coordination problem.

We rejected this because label-based sharding is simpler, uses native
Kubernetes mechanisms (label selectors on informer caches), and gives
operators explicit control.

### Namespace-Based Partitioning

Assign resources to namespaces and have each provider instance watch a specific
namespace. This is a common pattern in multi-tenant Kubernetes controllers.

We rejected this because:
- Crossplane managed resources are cluster-scoped, not namespaced.
- Composite resources may be cluster-scoped or namespaced.
- Forcing namespace-based partitioning would require architectural changes to
  resource scoping.

### Sidecar Proxy-Based Sharding

[KusionStack controller-mesh][controller-mesh] takes a proxy-based approach:
a sidecar container is injected into operator pods and intercepts API server
connections, injecting label selectors into watch/list requests to enforce
shard boundaries. This is transparent to the controller — no code changes
required.

We chose not to pursue this approach because:
- It adds operational complexity (sidecar injection, proxy lifecycle
  management, mTLS between proxy and API server).
- It uses namespace-based hash partitioning (`sharding-hash` values 0-31),
  which does not work for Crossplane's cluster-scoped managed resources.
- The project has not seen updates since mid-2024 (last release v0.2.0),
  raising concerns about long-term maintenance.
- The proxy intercepts all API server traffic for the pod, not just sharded
  resource types. This makes it harder to exempt non-sharded types
  (ProviderConfig, Secrets) from the shard filter.

### Virtual Clusters (vcluster)

Run each shard as a separate virtual cluster, each with its own Crossplane
installation. This provides strong isolation but:
- Dramatically increases operational complexity.
- Makes cross-shard visibility and management difficult.
- Is overkill for the stated goal of horizontal provider scaling.

## Critical Files

| File | Change |
|------|--------|
| `internal/xcrd/schemas.go` | Add `LabelKeyShard` constant |
| `internal/controller/apiextensions/composite/composition_render.go` | Propagate shard label to composed resources |
| `internal/features/features.go` | Add `EnableAlphaSharding` feature flag |
| `apis/pkg/v1beta1/deployment_runtime_config_types.go` | Add `Shards` field |
| `internal/controller/pkg/runtime/runtime_provider.go` | Multi-deployment creation per shard |

Changes in **crossplane-runtime** (separate repo):

| File | Change |
|------|--------|
| Provider CLI setup helpers | Add `--shard-id` flag |
| Manager/cache configuration | Apply label selector when shard ID is set |
| ByObject exemptions | Exempt ProviderConfig and related types |

[design-smaller-providers]: design-doc-smaller-providers.md
[one-pager-crd-scaling]: one-pager-crd-scaling.md
[one-pager-perf]: one-pager-performance-characteristics-of-providers.md
[provider-terraform-sharding]: https://github.com/crossplane-contrib/provider-terraform/pull/270
[provider-ansible-sharding]: https://github.com/crossplane-contrib/provider-ansible/pull/365
[k8s-controller-sharding]: https://github.com/timebertt/kubernetes-controller-sharding
[controller-mesh]: https://github.com/KusionStack/controller-mesh

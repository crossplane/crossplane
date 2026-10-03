# Resource Discovery and Import

* Owner: Ross Golder (@rossigee)
* Reviewers: Crossplane Maintainers
* Status: Draft

> **Note**: This design document has not yet been through the GitHub proposal
> issue phase described in [design/README.md]. It is recommended to open a
> proposal issue for community feedback before requesting formal review of this
> design.

## Background

Crossplane providers today manage external resources through a keyed lookup model.
A managed resource (MR) identifies an external resource via the
`crossplane.io/external-name` annotation and keeps it in sync with the desired
state declared in the MR's spec. This model works well for resources already
known to Crossplane—those explicitly created as MRs.

However, two important scenarios are blocked by the lack of resource enumeration:

### Scenario 1: Fresh Cluster Bootstrap (One-shot Adoption)

A new Crossplane cluster is deployed into an environment where external resources
already exist—e.g., MinIO buckets, AWS RDS instances, Kubernetes namespaces—
managed by tools other than Crossplane, or created directly via cloud console.
The operator wants to adopt these existing resources under Crossplane management,
at least to track and observe them. Today, this requires:

1. Manually enumerating resources (e.g., `mc ls`, `aws s3 ls`, `kubectl get ns`)
2. Writing one MR per resource by hand or script, annotating with
   `crossplane.io/external-name`
3. Applying the MRs to let Crossplane adopt them

This is tedious at scale and error-prone (copy-paste, naming conflicts).

### Scenario 2: Unmanaged Resource Detection (Ongoing Drift)

After bootstrap, new external resources may be created outside Crossplane
(by humans, other tools, policies, etc.). There is currently no way for Crossplane
to detect these "orphan" resources at all. The per-resource reconcile loop
(triggered by `--poll-interval`) only reconciles resources with existing MRs;
it cannot answer "are there external resources with no corresponding MR?"

This is a form of drift, distinct from spec-vs-observed drift on an existing MR.
Without visibility into unmanaged resources, operators cannot audit compliance
with Crossplane management or detect accidental manual changes.

### Why the Current ExternalClient Interface Cannot Solve This

The `ExternalClient` interface (defined in crossplane-runtime) requires a
managed resource instance:

```go
type ExternalClient interface {
    Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error)
    Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error)
    Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error)
    Delete(ctx context.Context, mg resource.Managed) error
}
```

Each method is keyed by the managed resource object—implicitly by its
`external-name` annotation. There is no `List()` or `Discover()` method, and
adding one would require a breaking change to the interface contract.

### Distinction from Prior Work: Querying and Filtering

The [design-doc-observe-only-resources.md](design-doc-observe-only-resources.md)
(§ "Querying and Filtering", lines 592–682) addresses a related but distinct
problem: finding *one matching* external resource based on filters (e.g.,
"the default VPC", "the most-recent AMI"). This problem:

- Occurs within a Composition or Claim context (an upper layer owns the lookup)
- Tolerates 0 or 1+ matches (may need to handle ambiguity)
- Requires query/filter parameters beyond just "list all of this kind"

That design deferred the solution to a future `Query Resource` kind or
Composition Function approach, both requiring more infrastructure.

This proposal addresses **exhaustive enumeration**: "give me all external
resources of this kind, as identifiers only." It is simpler, scoped differently
(per-resource, not per-composition), and solves a more immediate need.

## Goals

1. **Define an optional provider discovery interface** — a contract that
   ExternalClient implementations may opt into, allowing enumeration of external
   resources of a given kind without breaking existing providers.

2. **Enable one-shot resource adoption** — a workflow (CLI tool or operator)
   that discovers all external resources of a kind, diffs them against existing
   MRs, and emits YAML for unmanaged ones, ready to `kubectl apply`.

3. **Enable recurring unmanaged-resource detection** — an optional controller
   that runs on a coarse interval and surfaces external resources with no
   corresponding MR, flagging them for operator review or optional auto-adoption.

4. **Reuse existing adoption machinery** — any MRs created or applied by
   discovery must use the same `{Observe, LateInitialize}`
   [managementPolicy](apis/core/v2/policies.go) combo already used for
   manual resource import. No new adoption or reconciliation semantics.

5. **Maintain safety defaults** — discovery is feature-gated, opt-in, and
   surfacing (not auto-adopting) is the default; auto-import is controlled
   by explicit configuration.

## Non-Goals

1. **Real-time discovery** — this is a coarse-interval operation (like
   `--sync-interval`, default 1 hour), not a high-frequency per-resource
   poll (like `--poll-interval`, default 1 minute). Discovering new resources
   as they appear in milliseconds is out of scope.

2. **Default-on auto-adoption** — for safety, discovery must be opt-in, and
   auto-importing discovered resources must be explicitly enabled. The default
   is to surface findings only, leaving adoption to the operator.

3. **Single-match filtered lookup** — that remains the domain of the deferred
   Query Resource or Composition Function work. This proposal focuses on
   exhaustive enumeration only.

4. **Cross-resource inference** — discovering dependencies, ordering, or
   relationships between discovered resources is out of scope. Each resource
   is discovered independently.

5. **Multi-account or multi-provider aggregation** — discovery is per
   ProviderConfig, per resource kind. Aggregating results across accounts or
   clouds is left to higher-level tooling.

## Proposal

### 1. New Optional Interface: ExternalLister (crossplane-runtime)

In the `crossplane-runtime` repository (`github.com/crossplane/crossplane-runtime`),
add an optional interface that providers may implement:

```go
// ExternalListResult is returned by ExternalLister.List.
type ExternalListResult struct {
    // ExternalNames is a list of external resource identifiers discovered.
    // Providers must ensure these identifiers match the format expected by
    // ExternalClient.Observe when used as crossplane.io/external-name.
    ExternalNames []string

    // NextPageToken, if non-empty, indicates there are more results.
    // Callers should invoke List again with this token.
    NextPageToken string
}

// ExternalLister enumerates external resources of a kind that exist in the
// provider's external system. It is an optional interface; providers that do
// not wish to support discovery may leave it unimplemented.
type ExternalLister interface {
    // List enumerates external resources of the kind represented by the MRD.
    // The ProviderConfig (passed via context or method argument) scopes the
    // search to a specific cloud account, project, etc.
    //
    // Callers must check NextPageToken and invoke List again until it is empty.
    // Implementations should respect reasonable rate limiting and timeouts
    // to avoid overwhelming the external API.
    List(ctx context.Context, pc resource.ProviderConfig, pageToken string) (ExternalListResult, error)
}
```

**Design rationale:**

- **Optional / non-breaking** — Providers implement `ExternalLister` via type
  assertion in calling code (same pattern already used in the managed reconciler
  for other optional capabilities). Providers that don't implement it simply
  return "not supported" or "disabled"; existing providers are unaffected.

- **ProviderConfig-scoped** — Discovery is always within a specific
  ProviderConfig (account, project, credentials). This bounds the blast radius
  of any IAM or API call permissions and prevents accidental cross-account
  listing.

- **External names only** — `List()` returns identifiers, not full state.
  `Observe()` will be called separately on any adopted resource to fetch full
  state and populate status fields. This keeps `List()` cheap and focused.

- **Paginated** — External APIs often return results in pages. Pagination
  tokens are opaque to Crossplane; the provider controls the format.

### 2. Consumption Path A: One-shot Import Workflow

#### CLI Tool: `crossplane beta import`

A new CLI subcommand discovers external resources and emits YAML for adoption:

```bash
crossplane beta import \
  --kind=Bucket \
  --group=s3.minio.crossplane.io \
  --provider-config=default \
  --namespace=crossplane-system \
  [--dry-run]  # (default: true)
```

**Behavior:**

1. Look up the MRD for the given `kind` and `group`.
2. Fetch the named ProviderConfig.
3. Instantiate the provider's ExternalClient for that ProviderConfig.
4. If the client implements `ExternalLister`, call `List()` repeatedly until
   all pages are consumed, collecting external names.
5. Query the Kubernetes API for all existing MRs of that kind (using
   `kubectl get <kind> -A`).
6. Extract the `crossplane.io/external-name` annotation from each existing MR.
7. Compute the set difference: external names not yet managed.
8. For each unmanaged name, emit a CR with:
   - `metadata.name` = generated or user-provided (e.g., hash of external name)
   - `metadata.annotations["crossplane.io/external-name"]` = external name
   - `spec.managementPolicies: [Observe, LateInitialize]`
   - `spec.providerConfigRef.name` = the ProviderConfig used
   - All other spec fields left empty or filled with defaults
9. Write YAML to stdout (or file, if `--output` is given).
10. If `--dry-run=false` and `--auto-import=true`, `kubectl apply` the YAML
    and print a summary.

**Example output:**

```yaml
apiVersion: s3.minio.crossplane.io/v1alpha1
kind: Bucket
metadata:
  name: bucket-existing-bucket-1
  namespace: default
  annotations:
    crossplane.io/external-name: existing-bucket-1
spec:
  managementPolicies: [Observe, LateInitialize]
  providerConfigRef:
    name: default
---
apiVersion: s3.minio.crossplane.io/v1alpha1
kind: Bucket
metadata:
  name: bucket-existing-bucket-2
  namespace: default
  annotations:
    crossplane.io/external-name: existing-bucket-2
spec:
  managementPolicies: [Observe, LateInitialize]
  providerConfigRef:
    name: default
```

**Safety notes:**

- `--dry-run=true` by default; requires explicit `--auto-import=true` to apply.
- Emits warnings if duplicate external names are found (e.g., multiple MRs
  already adopt the same external resource).
- Does not delete or modify existing MRs; only reports new ones to be adopted.

### 3. Consumption Path B: Recurring Orphan-Detection Controller

#### Optional Controller: DiscoveryController

A new controller, gated behind `--enable-alpha-resource-discovery`, runs
periodically (default `--discovery-interval=1h`, tunable per MRD via annotation)
and surfaces external resources with no corresponding MR.

**Setup and registration:**

The controller follows the same `Setup()/Reconciler` pattern as other
core Crossplane controllers (see
[internal/controller/apiextensions/managed/setup.go](internal/controller/apiextensions/managed/setup.go)):

```go
// In a new file: internal/controller/apiextensions/discovery/setup.go
func Setup(mgr ctrl.Manager, o apiextensionscontroller.Options) error {
    // Register the DiscoveryReconciler for each active MRD whose provider
    // implements ExternalLister.
}
```

**Reconciler behavior:**

For each ManagedResourceDefinition with a provider that implements
`ExternalLister`:

1. Fetch all external names via `List()`.
2. Query Kubernetes for all MRs of that kind (all namespaces, all
   ProviderConfigs).
3. Extract the `crossplane.io/external-name` annotation from each MR.
4. Compute unmanaged = external names not in the MR set.
5. Emit a Kubernetes Event or update a lightweight DiscoveryReport CRD
   (open question; see below) with findings.
6. If `--auto-import=true`, optionally create new MRs for unmanaged resources
   (same as CLI path). Default: off.

**Output: Events or DiscoveryReport CRD?** (Open question to be resolved in
community review)

**Option A: Kubernetes Events**

Emit an Event for each run, grouped by MRD:
```yaml
apiVersion: v1
kind: Event
metadata:
  name: mrd-discovery.abc123def456
  namespace: crossplane-system
involvedObject:
  apiVersion: apiextensions.crossplane.io/v2
  kind: ManagedResourceDefinition
  name: buckets.s3.minio.crossplane.io
reason: DiscoveryComplete
message: "Found 2 unmanaged resources: external-bucket-1, external-bucket-2"
type: Warning
count: 3
lastTimestamp: "2025-01-15T10:30:00Z"
```

**Pros:** Uses existing Kubernetes primitives; low overhead; clear audit trail
in `kubectl describe mrd`.  
**Cons:** Events are ephemeral (default 1 hour TTL); no persistent state or
summary.

**Option B: DiscoveryReport CRD**

Define a new cluster-scoped CRD to store discovery results:
```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: DiscoveryReport
metadata:
  name: buckets-s3-minio-crossplane-io
spec:
  mrdRef:
    apiVersion: apiextensions.crossplane.io/v2
    kind: ManagedResourceDefinition
    name: buckets.s3.minio.crossplane.io
  interval: 1h
status:
  conditions:
  - type: DiscoverySucceeded
    status: "True"
    reason: ListingComplete
  discoveredResourceCount: 5
  unmanagedResourceCount: 2
  unmanagedResources:
  - externalName: external-bucket-1
  - externalName: external-bucket-2
  lastDiscoveryTime: "2025-01-15T10:30:00Z"
  nextDiscoveryTime: "2025-01-15T11:30:00Z"
```

**Pros:** Persistent; queryable; can be exported to metrics systems; clear
status and progress tracking.  
**Cons:** Adds a new CRD; more operational overhead.

**Recommendation for this draft:** Start with Events (simpler), allow for
future addition of DiscoveryReport if persistent audit/metrics are needed.

### 4. Implementation Scope: What Lives Where

**crossplane-runtime** (separate repository)
- `ExternalLister` interface and `ExternalListResult` struct
- Optional integration in the managed reconciler to enable discovery workflows
  (no required changes to existing reconcile logic)

**crossplane/crossplane** (this repository)
- CLI command `crossplane beta import` (or similar packaging)
- DiscoveryReconciler in `internal/controller/apiextensions/discovery/` (if
  orphan detection is implemented)
- Feature gate: `--enable-alpha-resource-discovery` (default: false)
- Per-resource opt-in: `crossplane.io/discovery-interval` annotation on MRD

**Provider repositories** (e.g., provider-aws, provider-gcp, provider-minio)
- Implement `ExternalLister.List()` for each managed resource kind that wishes
  to support discovery
- Opt-in; not required for existing resources
- Migrate incrementally

### 5. Safety and Scale Considerations

#### IAM Permissions

`List()` requires permission to enumerate resources, which is often a different
IAM action than per-resource `Get` (used by `Observe()`). Providers must
document the permission required (e.g., `s3:ListBucket`, `ec2:DescribeInstances`).

ProviderConfig-scoped discovery bounds the blast radius: listing is only
within the account/project/credentials of that ProviderConfig.

#### Rate Limiting and Cost

Calling `List()` on large external systems can be expensive (API quota, cost).

- Discovery runs on a long interval (default 1 hour), not per-resource polling
  (default 1 minute).
- Providers may implement their own rate limiting and caching within `List()`.
- Operators can disable discovery entirely or tune the interval per MRD.

#### Duplicate Adoption Races

If two controllers or CLI invocations try to adopt the same external resource
simultaneously, both may create an MR. Crossplane's existing duplicate-detection
(external-name uniqueness per MRD within the cluster) will surface the conflict,
and the later one will be blocked by the existing MR's update conflicts.

This is acceptable (rare in practice); the operator should resolve it by
deleting the duplicate and retaining the first one.

#### Feature Gating

Discovery is gated behind `--enable-alpha-resource-discovery` (default: false),
following the precedent in
[design-doc-observe-only-resources.md](design-doc-observe-only-resources.md)
(§ "Feature Gating"). This allows:

- Safe experimentation without affecting users who have not opted in.
- Clear signaling that the feature is alpha and subject to change.
- Easy rollback if issues arise.

### 6. Open Questions

1. **CLI packaging** — should `crossplane beta import` be a subcommand of the
   existing `crossplane` CLI, or a separate tool? How does it access the
   cluster and ProviderConfigs?

2. **DiscoveryReport CRD vs. Events** — should we start with Events and defer
   DiscoveryReport, or implement both? See § "Output" above.

3. **Orphan auto-adoption** — should the controller auto-create MRs for
   unmanaged resources, or only surface them? Proposal: only surface by default;
   auto-create requires explicit `--auto-import=true` per controller instance.

4. **Namespaced MRs** — in Crossplane v2, some MR kinds may be namespaced
   (CompositeResourceDefinitions). How does discovery interact with this?
   (MRDs are cluster-scoped, so discovery is per-MRD, but the MRs themselves
   may be namespaced. Proposal: discovery is per-MRD; adoption respects the
   MR's scope via a configurable namespace or label selector.)

5. **Credentials for List()** — does `List()` use the ProviderConfig's
   credentials directly, or are there cases where it needs additional
   secrets/IRSA? Proposal: use the same mechanism as `Observe()` (existing
   ProviderConfig plumbing).

## Alternatives Considered

### Alternative 1: Composition-Function-Based Discovery

Extend [design-doc-observe-only-resources.md](design-doc-observe-only-resources.md)
(§ "Querying and Filtering", Option B) to define a Discovery Composition
Function that enumerates resources and creates Observe-Only MRs.

**Pros:**
- Reuses Composition infrastructure; no new reconciler needed.
- Lazy: only runs when a Composition requests it.

**Cons:**
- Composition Functions are for single XR contexts; no way to run a global
  discovery without an XR already existing.
- Duplicates ProviderConfig authentication plumbing (already solved in
  ExternalClient).
- Does not solve the bootstrap scenario (no Composition/XR yet).
- Deferred as out-of-scope in the original design-doc; needs more maturity
  before relying on it.

**Recommendation:** Reject for now. Once Composition Functions are more mature,
they could be built on top of `ExternalLister` for higher-level workflows.

### Alternative 2: Query Resource Kind (Full Querying and Filtering)

Implement the full Option A from
[design-doc-observe-only-resources.md](design-doc-observe-only-resources.md)
(§ "Querying and Filtering"): a new `VPCQuery`, `BucketQuery`, etc. kind per
MRD, supporting filters and parameterized lookup.

**Pros:**
- Flexible; supports single-match filtered lookups.
- Clear separation of concerns: Query Resources own discovery, Managed
  Resources own reconciliation.

**Cons:**
- Doubles the number of CRDs.
- Solves a different problem (filtered lookup, not exhaustive enumeration).
- Deferred in the original design-doc for a reason; significant infrastructure
  work.
- Requires its own schema and configuration per provider and resource kind.

**Recommendation:** Reject as the primary mechanism. A simpler `ExternalLister`
interface is a better foundation. Query Resources could be built on top of it
in the future if filtered lookup becomes a priority.

### Alternative 3: Status Quo (Manual Enumeration Scripts)

Continue asking operators to write per-provider scripts (`mc ls`, `aws s3 ls`,
etc.) and hand-generate YAML. Examples: MinIO bucket adoption workflow
discussed in prior design sessions.

**Pros:**
- No new infrastructure; works today.
- Operators retain full control.

**Cons:**
- Error-prone; duplicates effort across deployments.
- Does not scale to many resources or multi-team environments.
- Does not address ongoing orphan detection at all.

**Recommendation:** Keep as a fallback for providers that choose not to
implement `ExternalLister`, but provide `ExternalLister` to make the common
case easier and safer.

## Future Work

1. **Query Resource Kind** — once `ExternalLister` is stable and in use, design
   and implement a `Query Resource` or similar higher-level abstraction for
   filtered single-match lookups, built on top of `ExternalLister`.

2. **Metrics and observability** — expose discovery results as Prometheus
   metrics (e.g., unmanaged resource counts per MRD, discovery latency).

3. **Multi-cloud aggregation** — tools to correlate discovery results across
   ProviderConfigs or resource kinds.

4. **Drift reconciliation policies** — extend `managementPolicies` with an
   action to auto-remediate discovered unmanaged resources (auto-delete them
   vs. auto-adopt them), with strong safety gates.

## Conclusion

Resource discovery is a natural extension of Crossplane's managed resource model.
The `ExternalLister` interface provides a minimal, backward-compatible contract
for providers to offer enumeration. The two consumption paths—one-shot CLI
import and recurring orphan detection—address the immediate pain points of
bootstrap and compliance auditing, respectively. This proposal can be implemented
incrementally, starting with the interface in crossplane-runtime, followed by
the CLI tool in this repository, and finally the optional controller for
ongoing detection.

[design-doc-observe-only-resources.md]: design-doc-observe-only-resources.md
[apis/core/v2/policies.go]: ../apis/core/v2/policies.go
[internal/controller/apiextensions/managed/setup.go]: ../internal/controller/apiextensions/managed/setup.go

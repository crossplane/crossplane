# Per-Resource Poll Interval

* Owner: Yordis Prieto (@yordis)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

The `--poll-interval` flag controls how often Crossplane reconciles resources,
but it applies globally to all resources managed by a controller. This creates a
dilemma for operators:

* A short interval (e.g. the 10m default) causes unnecessary API calls for
  stable, rarely-changing resources (databases, DNS zones, tunnels), risking
  rate limiting against external APIs.
* A long interval causes slow drift detection for resources that need active
  watching.
* Pausing a resource stops reconciliation entirely and requires manual
  intervention to resume.

There is no middle ground. All resources share the same reconciliation frequency
regardless of how often they actually change. This is especially painful when a
single provider manages both stable long-lived resources and dynamic ones
against APIs with strict rate limits.

Additionally, there is no way to trigger an immediate reconciliation of a
specific resource without modifying its spec. Operators sometimes need to force a
re-sync after an out-of-band change or to verify drift correction, without
waiting for the next poll cycle.

See [issue #7204] for the original proposal.

## Goals

* Allow operators to override the controller-level poll interval on individual
  resources via an annotation.
* Provide a mechanism to trigger immediate reconciliation of a specific resource.
* Support both composite resources (XRs, reconciled by Crossplane core) and
  managed resources (MRs, reconciled by providers).
* Remain fully backwards compatible — resources without annotations continue to
  use the controller default.

## Proposal

Introduce two annotations, inspired by similar patterns in [Flux CD][flux-reconcile],
that give operators fine-grained control over reconciliation behavior on a
per-resource basis.

### `crossplane.io/poll-interval`

Overrides the controller-level `--poll-interval` for a specific resource.

```yaml
apiVersion: postgresql.sql.crossplane.io/v1alpha1
kind: Database
metadata:
  name: my-database
  annotations:
    crossplane.io/poll-interval: "24h"
```

**Behavior:**

* Accepts any valid Go duration string (e.g. `30m`, `1h`, `24h`).
* Enforces a configurable minimum via a `--min-poll-interval` flag (defaults to
  `1s`) to prevent tight reconciliation loops. This is similar to how
  `--max-function-cache-ttl` lets operators cap the function cache TTL.
* Invalid values fall back to the controller default. Values below
  `--min-poll-interval` are clamped to the configured minimum.
* When absent, the controller-level `--poll-interval` applies as today.
* The same ±10% jitter that applies to the controller default also applies to
  annotation-specified intervals.

### `crossplane.io/reconcile-requested-at`

Triggers an immediate reconciliation when its value changes. This follows the
pattern established by [Flux CD's `reconcile.fluxcd.io/requestedAt`][flux-reconcile].

```yaml
apiVersion: postgresql.sql.crossplane.io/v1alpha1
kind: Database
metadata:
  name: my-database
  annotations:
    crossplane.io/reconcile-requested-at: "2024-01-15T10:30:00Z"
```

**Behavior:**

* The annotation value is an opaque token (typically a timestamp or UUID).
* Setting or changing the value triggers reconciliation through the Kubernetes
  watch mechanism.
* The reconciler records the handled token in
  `status.lastHandledReconcileAt` so operators can confirm the
  request was processed.
* A Kubernetes event (`ReconcileRequestHandled`) is emitted when the token is
  processed.
* If the token matches the last handled value, it is a no-op.

**Confirming processing:**

```bash
# Request reconciliation
kubectl annotate database my-database \
  crossplane.io/reconcile-requested-at="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --overwrite

# Verify it was handled
kubectl get database my-database -o jsonpath='{.status.lastHandledReconcileAt}'
```

## Scope: XR vs MR Reconciliation

The original issue ([#7204]) describes the managed resource (MR) case, but this
feature affects two distinct reconciliation paths that are owned by different
repositories:

| | Composite Resources (XRs) | Managed Resources (MRs) |
|---|---|---|
| **Reconciled by** | Crossplane core | Providers (via crossplane-runtime) |
| **Code location** | `internal/controller/apiextensions/composite/` | `crossplane-runtime/pkg/reconciler/managed/` |
| **Poll purpose** | Re-sync composed resources, detect drift | Detect out-of-band changes to external resources |
| **Who benefits** | Platform teams managing compositions | End users managing cloud resources |

Both paths use the same poll-and-requeue pattern and both benefit from
per-resource control. The annotation contract is identical for both — the
difference is purely in where the reconciler code lives.

## Implementation

Annotation constants and parsing helpers live in crossplane-runtime's `pkg/meta`
package so they are shared by both reconcilers:

* `GetPollInterval(obj) (time.Duration, bool)` — parses the annotation and
  returns the duration and whether a valid interval was present.
* `GetReconcileRequest(obj) (string, bool)` — reads the reconcile-requested-at
  token.
* `SetReconcileRequest(obj, token)` — sets the token (for programmatic use).

### Composite Resources (XRs)

XR reconciliation lives in Crossplane core at
`internal/controller/apiextensions/composite/reconciler.go`. The changes are:

1. An `effectivePollInterval(xr)` method checks the annotation on the composite
   resource and falls back to the controller default if absent or invalid.
2. The reconciler's final `RequeueAfter` uses `effectivePollInterval(xr)`
   instead of the hardcoded `r.pollInterval`.
3. Reconcile-request token tracking is added near the top of the `Reconcile()`
   method. When a new token is detected, it is recorded in
   `status.lastHandledReconcileAt` and an event is emitted.

### Managed Resources (MRs)

MR reconciliation lives in [`crossplane-runtime`][crossplane-runtime] and is
consumed by every provider. The managed reconciler receives the same
`effectivePollInterval` and reconcile-request token tracking as the XR
reconciler. Because the managed reconciler lives in crossplane-runtime, providers
inherit this behavior when they upgrade their crossplane-runtime dependency
without any code changes.

### Precedence

The effective poll interval for a resource is determined by:

1. **Annotation value** (`crossplane.io/poll-interval`) if present and valid.
2. **Controller flag** (`--poll-interval`) as the default.
3. **Realtime compositions mode** (poll interval = 0) for XRs when enabled.

### API Surface

The `ObservedStatus` type in `apis/core/v2/observation.go` gains a new field:

```go
type ObservedStatus struct {
    // ...existing fields...

    // LastHandledReconcileAt holds the value of the
    // reconcile-requested-at annotation from the most recent
    // reconciliation that was triggered by a reconcile request.
    LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`
}
```

## Alternatives Considered

### Spec Field Instead of Annotation

A `spec.pollInterval` field was considered but rejected because:

* It would require CRD schema changes across all resource types.
* Annotations are the established Kubernetes pattern for operational hints that
  do not affect the desired state of the resource itself.
* Flux CD, Argo CD, and other controllers use annotations for similar
  reconciliation control.

[issue #7204]: https://github.com/crossplane/crossplane/issues/7204
[flux-reconcile]: https://fluxcd.io/flux/components/source/api/v1/#source.toolkit.fluxcd.io/v1.GitRepository
[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime

# Sketch: pending resources in status, and typed edges

Two proposed API changes, sketched far enough to argue with.

1. `status.crossplane.pendingResources` — the composed resources the graph is
   holding back, in either direction, and why.
2. `dependsOn` as a list of objects rather than a list of strings, so an edge
   can say what kind of thing it points at.

They are separable. (1) is additive and can ship alone. (2) is a breaking
change to an alpha field, so its deadline is beta, not its usefulness.

Decided so far: no `field` on an edge in v1 - the object form earns its place
on `type` alone, and adding `field` later costs nothing once edges are
objects. The status field is named `pendingResources`. Per-resource blocked
reasons become structured, and the hand-built strings go.

## Why pending resources need somewhere to live

Today they have nowhere. `composition_functions.go` drops a held-back resource
from `resourceRefs` on purpose:

> Don't reference a resource we've deliberately not applied. The reference is
> written before the apply so a created resource can't be leaked, but nothing
> was created here - the graph is holding it back. Referencing it anyway
> points anything reading `spec.resourceRefs`, `crossplane trace` included, at
> an object that doesn't exist, which reads as an error rather than as
> waiting.

That is a choice between leaking a reference to a nonexistent object and
losing the information, and it picks losing it. What remains is prose:
`teardownBlockedMessage` and its sibling build strings with `fmt.Sprintf`, and
anything that wants the state has to scrape one. `xpgraph` cannot draw a
pending node for this reason.

A field of its own resolves the tension instead of trading against it.
`resourceRefs` keeps meaning "these exist"; the new field carries what the
graph is holding back; neither has to lie.

### It must not be authoritative

The argument against function-controlled deletion is that the graph survives
on the XR with no pipeline. If any part of teardown came to depend on status,
a restore that drops status would lose ordering.

So: **teardown never reads this field.** Pending resources are re-derived from
the function on every reconcile, so losing the field costs nothing but
visibility until the next pass. That keeps the split clean rather than
duplicative:

| | Holds | Read by | Authoritative |
| --- | --- | --- | --- |
| `spec.crossplane.resourceRefs` | what exists, and the edges between | teardown, creation, tooling | yes |
| `status.crossplane.pendingResources` | what the graph won't let happen yet, and why | tooling, humans | no |

A pending creation has no entry in `resourceRefs`, and a pending deletion has
one. They are complementary rather than duplicative: the reference says what a
resource is and what it depends on, the pending entry says what Crossplane
will not do to it yet.

### It is where required-resource edges finally belong

`UpdateComposedResourceRefs` drops them deliberately — "an edge to a required
resource gates creation only [...] so it has no bearing on teardown and no
reference to live in." Gating creation is precisely what this field is about.
A composition waiting on a Secret the user has not created yet is invisible in
the API today; Modelplane's inference cluster depends on that pattern.

## The types

### A dependency

One type for both fields, so a consumer drawing a graph has one edge shape.

```go
// A DependencyType says what kind of thing a dependency points at.
type DependencyType string

const (
	// DependencyTypeComposedResource is another resource the same composite
	// composes, named by its composition resource name.
	DependencyTypeComposedResource DependencyType = "ComposedResource"

	// DependencyTypeRequiredResource is a resource the pipeline required
	// rather than composed. Crossplane never deletes a resource it didn't
	// compose, so these order creation only.
	DependencyTypeRequiredResource DependencyType = "RequiredResource"
)

// A Dependency is one ordering constraint declared over a resource.
type Dependency struct {
	// Type of thing this depends on. Defaults to ComposedResource.
	// +optional
	// +kubebuilder:validation:Enum=ComposedResource;RequiredResource
	Type DependencyType `json:"type,omitempty"`

	// Name is the composition resource name depended on. Set when Type is
	// ComposedResource.
	// +optional
	Name string `json:"name,omitempty"`

	// Requirement identifies a resource the pipeline required. Set when Type
	// is RequiredResource.
	// +optional
	Requirement *RequirementDependency `json:"requirement,omitempty"`
}

// A RequirementDependency identifies a required resource, or one resource
// within the set a requirement matched.
type RequirementDependency struct {
	// Name of the requirement - the key into the pipeline's requirements.
	Name string `json:"name"`

	// ResourceName optionally narrows the dependency to a single resource
	// within the set the requirement matched. If unset, every match must be
	// ready.
	// +optional
	ResourceName string `json:"resourceName,omitempty"`

	// Namespace of ResourceName, for a namespaced resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
```

`Composed.DependsOn` changes type, and nothing else about it changes:

```go
type Composed struct {
	// ... apiVersion, kind, name, namespace, resourceName unchanged

	// DependsOn is the ordering constraints declared over this resource.
	// Crossplane creates it only once everything it depends on is ready, and
	// deletes it only once nothing depends on it any more.
	//
	// Only ComposedResource dependencies appear here. A required resource
	// gates creation only, and has no bearing on the teardown this field
	// exists to order.
	DependsOn []Dependency `json:"dependsOn,omitempty"`
}
```

### A pending resource

"Pending" has to cover both directions, and that is not obvious until you
look at what the code already does. `blockedReport` handles the apply-side and
the delete-side gate through one function, and says why:

> Both the apply-side and delete-side gates report through here. They used to
> carry their own copies of this rule, and the copies drifted the moment a
> decision moved from one side to the other.

A resource removed from desired state but not yet deletable is held back by
the graph exactly as much as one that cannot be created yet, and
`composition_functions.go` reports it that way deliberately - "a resource that
can't be deleted yet is exactly as invisible as one that can't be created yet,
and just as worth explaining."

So if only pending *creations* became structured, the delete side would keep
its string, and the rule would be split across a field and a `fmt.Sprintf`
again. `Operation` keeps them together:

```go
// An Operation is the change the graph is holding back.
type Operation string

const (
	// OperationCreate - the resource does not exist yet.
	OperationCreate Operation = "Create"

	// OperationDelete - the resource exists and is no longer desired, but
	// something still depends on it.
	OperationDelete Operation = "Delete"
)

// A Pending is a composed resource with a change the graph has not allowed
// yet, and the reason it hasn't.
type Pending struct {
	// APIVersion of the resource.
	APIVersion string `json:"apiVersion"`

	// Kind of the resource.
	Kind string `json:"kind"`

	// Name of the resource. Set only when Operation is Delete, because only
	// then does the object exist.
	//
	// Crossplane assigns composed resource names before it applies them, so a
	// pending creation could carry one - but a name for an object that does
	// not exist is the thing resourceRefs avoids, and everything that reads a
	// name reads it as a pointer.
	// +optional
	Name string `json:"name,omitempty"`

	// ResourceName is the composition resource name - the key the function
	// uses for it, and what the edges in DependsOn refer to.
	ResourceName string `json:"resourceName"`

	// Operation the graph is holding back.
	// +kubebuilder:validation:Enum=Create;Delete
	Operation Operation `json:"operation"`

	// DependsOn is what this resource is waiting for. Set for a pending
	// creation; a pending deletion already carries its edges on its entry in
	// spec.crossplane.resourceRefs, and Reason names what still depends on it.
	// +optional
	DependsOn []Dependency `json:"dependsOn,omitempty"`

	// Reason says why the operation is held back, in a form meant to be read
	// by a person.
	//
	// It must be stable while the situation is: no elapsed times, no
	// timestamps, no counters. The reconciler skips the status write when
	// status is unchanged, and a reason that moves on its own turns every
	// reconcile into a write, and every write into a watch event on the XR -
	// which is also a token from the watch circuit breaker. How long
	// something has been waiting belongs in the condition's
	// lastTransitionTime, which records it for free and stays put.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Deadlocked is true when waiting cannot resolve this - a cycle, or a
	// dependency on something that will never be created. Someone has to act.
	// +optional
	Deadlocked bool `json:"deadlocked,omitempty"`
}
```

`Deadlocked` is the field that earns its place fastest. The ordering package
already computes it (`ordering.Decisions.Blocked[name].Deadlocked`) and it is
currently spent on a string prefix, `"deadlocked: " + reason`. Nothing can
alert on a prefix.

### What this deletes

`blockedReport` returns a `summary string` that exists only to be joined into
an event message. With `Reason` on the entry, that return value goes, and so
does the per-resource string formatting on both sides.

`blockedMessage` should shrink rather than disappear. It truncates at five
resources today —

```go
if len(blocked) > listed {
	shown = blocked[:listed]
	suffix = fmt.Sprintf(" (and %d more)", len(blocked)-listed)
}
```

— which is a good argument for the field: an event has to stay small, and a
50-deep chain holds back far more than five. But the event also carries how
long the pipeline took, and the comment explains why that matters: "the
interval between two of these events is the ordering wave time; if that
interval is far larger than the pipeline duration, the delay is in being woken
up rather than in doing the work." That is not per-resource information and
has nowhere else to go.

So the event becomes a count and a duration - "Ordering is holding back 37
composed resource(s). Function pipeline took 84ms." - and the detail that used
to be truncated into it lives in full on the XR.

The teardown messages are the other half. `teardownWaitingMessage` and
`teardownBlockedMessage` build the Deleting condition, and a condition message
is the right place for a summary someone reads in `kubectl describe`. They
stay, but they stop being the only copy of the information, and
`teardownBlockedMessage`'s deadlock prefix can read `Deadlocked` rather than
rebuilding the distinction.

### Where it hangs

Modern XRs nest machinery under `spec.crossplane`, so status matches:

```go
// In the XR's status, for modern (namespaced and cluster) XRs.
type CompositeStatusCrossplane struct {
	// PendingResources are composed resources the pipeline declared that
	// Crossplane has not created yet.
	// +optional
	PendingResources []Pending `json:"pendingResources,omitempty"`
}
```

Legacy v1 XRs don't get it: ordering is a v2 feature, so there is no reason to
carry a second path for a schema that will never report one. Generated in
`crossplane-runtime/pkg/xcrd/schemas.go`, alongside the existing
`resourceRefs` props, with `XListType: atomic` to match — the controller
replaces the whole array every pass.

## What it looks like on an XR

Mid-convergence, a gateway stack:

```yaml
spec:
  crossplane:
    resourceRefs:
    - apiVersion: helm.crossplane.io/v1beta1
      kind: Release
      name: stack-cert-manager-7xk2p
      resourceName: cert-manager
      dependsOn:
      - name: provider-config-helm
    - apiVersion: helm.crossplane.io/v1beta1
      kind: Release
      name: stack-envoy-gateway-m4p8q
      resourceName: envoy-gateway
      dependsOn:
      - name: provider-config-helm
      - name: cert-manager
status:
  crossplane:
    pendingResources:
    - apiVersion: gateway.networking.k8s.io/v1
      kind: GatewayClass
      resourceName: gateway-class
      operation: Create
      dependsOn:
      - name: envoy-gateway
      reason: waiting for envoy-gateway to be ready
    - apiVersion: v1
      kind: Secret
      resourceName: cluster-backend
      operation: Create
      dependsOn:
      - type: RequiredResource
        requirement: {name: cluster-kubeconfig}
      reason: waiting for requirement cluster-kubeconfig to match a resource
    - apiVersion: helm.crossplane.io/v1beta1
      kind: Release
      name: stack-cert-manager-7xk2p
      resourceName: cert-manager
      operation: Delete
      reason: no longer desired; envoy-gateway still depends on it
```

The second entry is the case with no representation at all today. The third
is the delete-side gate, which is reported today but only as a line in a
truncated event - and note it is the same resource as the first
`resourceRefs` entry, seen from the other side: the reference says what it is
and what it depends on, the pending entry says what Crossplane will not do to
it yet.

## Settled

**No `field` in v1.** The object form is justified by `type` alone, which
carries a distinction the proto draws and the persisted form currently throws
away. `field` can be added later at no migration cost once edges are objects,
and leaving it out avoids implying that Crossplane will populate it. The
proposal orders; it does not move data.

**`pendingResources`, not `dependencyRefs`.** These are not references - which
is the whole reason they cannot live in `resourceRefs`.

**Structured reasons replace the per-resource strings.** See "What this
deletes" above.

## Still open

**Status churn** turned out to be solved already, as long as the field obeys
one rule. The reconciler snapshots status at the top of a reconcile and writes
only when it differs:

```go
statusBefore, _, _ := kunstructured.NestedFieldCopy(xr.Object, "status")
...
if !cmp.Equal(statusBefore, xr.Object["status"]) {
	return result, errors.Wrap(r.client.Status().Update(updateCtx, xr), errUpdateStatus)
}
```

So `pendingResources` costs a write only on the passes where the pending set
actually changes - roughly one per wave, against the seven or so reconciles a
wave provokes. The rule is that `Reason` must not move on its own; see its
godoc above.

Fixing this turned up the same bug already shipped: the teardown path wrote
status unconditionally every pass, and its message embedded
`time.Since(ts).Round(time.Second)`, so a teardown wrote the XR every few
seconds for its whole duration. Both are fixed, and
`TestTeardownWaitingMessageIsStable` pins the property.

**Lifecycle.** `dependsOn` deliberately does not persist
`create-before-destroy`, because the question cannot arise during teardown.
The object form makes it addable later without a second migration, which is an
argument for the shape independent of anything else.

**Cost.** Measured on a live XR, an edge is about 9 bytes with five-character
resource names and 30-50 with realistic ones. Without `field`, an object edge
is close to that - `- name: vpc` against `- vpc` - because `type` is omitted
whenever it is the default. The ceiling stays where it is.

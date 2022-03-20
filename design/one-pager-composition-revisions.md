# Composition Revisions

* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

In Crossplane _Composition_ allows platform teams to define and offer bespoke
infrastructure APIs to the teams of application developers they support.
Resources within these APIs are known as _Composite Resources_ (XRs). Crossplane
powers each XR by composing one or more _Managed Resources_ (MRs). When an XR is
created Crossplane uses a `Composition` to determine which MRs are required to
satisfy the XR. Note that Composition is used in two ways here; _Composition_ is
the name of the feature, while "a `Composition`" is one of the Crossplane
resources that configures the feature.

Platform engineers can use Crossplane to define an arbitrary number of XR types,
and an arbitrary number of `Compositions`. Each `Composition` declares that it
satisfies a particular type of XR, in the sense that the `Composition` tells
Crossplane what resources should be composed to satisfy the XR's desired state.
Any XR can be satisfied by one `Composition` at any point in time. There is a
one-to-many relationship between a `Composition` and the XRs that it satisfies.

![XR to Composition relationship][xr-to-composition]

Note that in the above diagram the `example-a` and `example-b` `CompositeWidget`
XRs are both satisfied by one `Composition`; `large`. Meanwhile `example-c` is
satisfied by a different `Composition`.

Today it is possible to update a `Composition` in place, but doing so is risky.
_All_ XRs that use said `Composition` will be updated instantaneously. These
updates will often be surprising, because the party making the update and the
parties affected by the update will typically be different people. That is,
typically a platform engineer would update the `Composition` and that update
would instantly cause changes to various XRs provisioned and owned by app teams.

Ideally it would be possible for an updated `Composition` to be introduced then
rolled out in a controlled fashion to the various XRs it satisfies. It should be
possible to do this in a fashion that enables the separation of concerns; i.e.
to support one team (typically the platform team) introducing a new
`Composition` and potentially another team (e.g. an app team) choosing when
their XR should start consuming that `Composition`.

## Goals

Functionality wise, this design intends to:

* Allow a `Composition` that is in use to be updated in a measured fashion.
* Respect the separation of concerns; don't assume that the person introducing
  the new `Composition` is the same person who will update the XRs that consume
  it.

It must be possible to introduce this functionality in a measured, backward
compatible way. Crossplane's behaviour and v1 APIs should not change for anyone
who does not opt into this new functionality.

## Proposal

This document proposes the introduction of a new type - `CompositionRevision`.
XRs will use a `CompositionRevision`, not a `Composition` to determine which
managed resources should satisfy an XR.

Platform teams will still create and update `Composition` resources - the new
`CompositionRevision` resources will be created automatically by Crossplane. A
controller will create an immutable `CompositionRevision` corresponding to each
update to (or 'revision' of) a `Composition`. This allows an XR to be 'pinned'
to a particular revision of a `Composition`, thus allowing the `Composition` to
be updated without inherently affecting the XRs that (indirectly) use it. The
new `CompositionRevision` resource's schema will be a superset of the existing
`Composition` schema.

Each time a `Composition` is updated a controller will automatically create a
`CompositionRevision` - an immutable snapshot of that particular 'revision' of
the `Composition`. The `CompositionRevision` schema will be a superset of the
`Composition` schema. Each `CompositionRevision` will be labelled the name of
the `Composition` it is derived from, and a hash of that composition's spec.
Updates to a `Composition` that do not introduce a new version of the spec will
be deduplicated; i.e. a new `CompositionRevision` will not be created when a
`CompositionRevision` labelled with a hash of the Composition's latest spec
already exists.

![XR to CompositionRevision relationship][xr-to-revision]

For example the `Composition` below would result in the creation of the
subsequent `CompositionRevision`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  writeConnectionSecretsToNamespace: crossplane-system
  resources:
    - name: cloudsqlinstance
      base:
        apiVersion: database.gcp.crossplane.io/v1beta1
        kind: CloudSQLInstance
        spec:
          forProvider:
            databaseVersion: POSTGRES_12
            region: us-central1
            settings:
              tier: db-custom-1-3840
              dataDiskType: PD_SSD
      patches:
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.settings.dataDiskSizeGb"
```

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: CompositionRevision
metadata:
  # The revision's name is derived from the composition's name using the API
  # server's GenerateName support.
  name: example-fklem2
  labels:
    crossplane.io/composition-name: example
    crossplane.io/composition-spec-hash: 707e5bf63687fce7
  ownerReferences:
  # The Composition is the controller reference of the revision. When a
  # Composition is deleted all of its CompositionRevisions are garbage
  # collected.
  - apiVersion: apiextensions.crossplane.io/v1
    kind: Composition
    name: example
    controller: true
spec:
  # Each revision includes an integer revision number. This number increases
  # monotonically as the associated composition is updated.
  revision: 1
  # Apart from the revision number the rest of the revision spec is identical
  # to the composition spec.
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  writeConnectionSecretsToNamespace: crossplane-system
  resources:
    - name: cloudsqlinstance
      base:
        apiVersion: database.gcp.crossplane.io/v1beta1
        kind: CloudSQLInstance
        spec:
          forProvider:
            databaseVersion: POSTGRES_12
            region: us-central1
            settings:
              tier: db-custom-1-3840
              dataDiskType: PD_SSD
      patches:
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.settings.dataDiskSizeGb"
```

Two additional fields will be added to each type of XR and XRC.

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: gcp
  compositionRef:
    name: example-gcp
  # A new optional field, typically automatically late initialized to the latest
  # (highest) revision of the referenced composition when the XR.
  compositionRevisionRef:
    name: example-gcp-fk2ks
  # A new optional field. Determines whether the above compositionRevisionRef is
  # updated automatically to reflect the latest available revision, or whether
  # it must be updated manually. Valid values are Automatic or Manual. Defaults
  # to Automatic.
  compositionUpdatePolicy: Automatic
```

This design is backward compatible with Crossplane's current behaviour. An XR or
XRC author may be ignorant of the new `CompositionRevision` type. The XR will
default to `compositionUpdatePolicy: Automatic`, and thus always select the
latest revision of the desired `Composition`. Only when the XR or XRC author
explicitly specifies `compositionUpdatePolicy: Manual` will the behaviour
diverge.

`CompositionRevision` support will be introduced in a phased fashion, with
support initially being off by default behind a feature flag and with minimal
changes to XR reconcile logic. Currently XR reconcile logic is roughly:

1. Fetch the XR from the API server.
2. Select an appropriate `Composition` for the XR.
3. Fetch the `Composition` from the API server.
4. Validate the `Composition`.
5. Use the `Composition` to compose managed resources.

Step 3 is currently a controller-runtime `client.Client` `Get` call. e.g.:

```go
comp := &v1.Composition{}
if err := r.client.Get(ctx, meta.NamespacedNameOf(cr.GetCompositionReference()), comp); err != nil {
  return reconcile.Result{RequeueAfter: shortWait}, nil
}
```

This will be replaced with a call to a new `CompositionFetcher` interface, e.g.:

```go
// A CompositionFetcher fetches an appropriate Composition for the supplied
// composite resource.
type CompositionFetcher interface {
  Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error)
}

// r.composition is a CompositionFetcher.
comp, err := r.composition.Fetch(ctx, cr)
if err != nil {
  return reconcile.Result{RequeueAfter: shortWait}, nil
}
```

Each XR reconciler will default to using a `CompositionFetcher` that defaults to
the same logic we use today; i.e. a `client.Client` `Get`. When Crossplane is
started with a feature flag such as `--enable-alpha-composition-revisions` this
default `CompositionFetcher` implementation will be replaced with one that
instead:

1. Fetches the `Composition` from the API server.
2. Selects the appropriate `CompositionRevision`.
3. Returns the selected `CompositionRevision` converted to a `Composition`.

Converting the `CompositionRevision` to a `Composition` allows us to introduce
support for revisions with minimal changes to the XR reconcile logic. This will
allow us to become comfortable with a `v1alpha1` iteration of the new API type
before we commit to it. If and when we are comfortable with the `v1alpha1`
`CompositionRevision` API we can promote it to `v1beta1` and have it on by
default. At this point the XR reconcile logic would be updated to deal directly
with the `CompositionRevision` API type, without any conversion back to the
`Composition` type. Notably this would involve migrating the patch logic that is
currently defined as methods on the `Composition` type to being methods on the
`CompositionRevision` type.

## Future Considerations

Two key future considerations are garbage collection of revisions, and allowing
platform engineers to enforce a `compositionUpdatePolicy`.

### Garbage Collection

'Revisions' are an established pattern in the Kubernetes ecosystem per the
"Prior Art" section below. It's common for controllers that create revisions to
garbage collect older revisions in order to avoid bloating the API server with
unused objects. Typically the latest N revisions of a type are kept, and
anything older is automatically garbage collected. Most revision implementations
are designed such that only one revision will be 'active' at any one time. For
example there is only ever one active `ProviderRevision` for each Crossplane
`Provider` resource. This means it's generally fairly safe and easy to garbage
collect old revisions; you need only avoid garbage collecting the active one.

The design proposed by this document requires a more nuanced approach to
garbage collecting old revisions, because many revisions may be in use at one
time. The policy would likely need to be "garbage collect revisions older than N
if they're not in use", which would require the garbage collector to keep track
of which revisions are in use across an arbitrary set of XR types. This document
proposes that garbage collection be deferred - it's possible that `Composition`
updates will be infrequent enough that manual garbage collection will be
sufficient.

### Enforced Update Policies

Crossplane allows XR and XRC authors to choose which `Composition` is used to
compose resources by default. However, XRD authors (i.e. platform engineers) may
choose to override this behaviour by enforcing a `Composition`. Similarly, XRD
authors may wish to enforce a `compositionUpdatePolicy`, requiring that
particular types of XR either always (`Automatic`) or never (`Manual`) track the
latest revision of a `Composition`.

Building such support would presumably involve adding a new field such as
`spec.enforceCompositionUpdatePolicy` to the `CompositeResourceDefinition` type,
then using that field to override the XR's `spec.compositionUpdatePolicy`. The
XRD is already plumbed down to each XR reconciler, so this is likely to be
straightforward to build if there is demand for the functionality.

## Prior Art

* Crossplane [package revisions][packages-v2]
* Metacontroller [controller revisions][metacontroller-controller-revisions]
* Kubernetes [controller revisions][kubernetes-controller-revisions]

## Alternatives Considered

* Treat `Compositions` as create-only; require composed resources to be updated
  directly in-place after creation.
* Prefer forking and introducing new `Compositions` rather than updating them in
  place.

[packages-v2]: design-doc-packages-v2.md
[metacontroller-controller-revisions]: https://metacontroller.github.io/metacontroller/api/controllerrevision.html
[kubernetes-controller-revisions]: https://kubernetes.io/docs/tasks/manage-daemon/rollback-daemon-set/#understanding-daemonset-revisions
[xr-to-composition]: images/xr-to-composition.svg
[xr-to-revision]: images/xr-to-revision.svg

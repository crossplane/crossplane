# Foreground Cascading Deletion Support

* Owner: Bob Haddleton (@bobh66)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

When a Claim is deleted, Crossplane deletes the associated Composite resource
using the default "Background" kubernetes propagation policy. Crossplane
delegates the deletion of all the Composed resources from that Composite to
kubernetes via the ownerReference on each resource.  When the "root" Composite
resource is deleted by the claim reconciler, kubernetes garbage collection
deletes all the Composed resources for the entire graph, which sets the
deletionTimestamp attribute on each resource.  That triggers essentially
simultaneous deletion of all Composite and Managed Resources that are related to
the root Composite.

Crossplane places finalizers on the objects that it creates to ensure that the
objects are processed by the Crossplane resource controllers before they are
deleted.  Crossplane directly manages the deletion of the Composite and Managed
resources that it creates, calling the appropriate Delete() function for the
resources when the deletionTimestamp attribute shows up on an object.

While all the Composed objects (Composite and Managed) have Controller
ownerReferences that point to the composite that created them, there is no
ownerReference on the "root" Composite that references the Claim.  This is due
to the fact that the Composite object is cluster-scoped and the Claim is
namespace-scoped.  Kubernetes does not allow a cluster-scoped
object to be "owned" by a namespaced object.  This renders the
_--cascade=foreground_ option non-functional for Claim objects. Kubernetes does
not find any objects that refer to the Claim in an ownerReference, so it doesn't
set the _foregroundDeletion_ finalizer, and it marks the Claim as deleted. The
Claim reconciler sees the deletionTimestamp indicator and calls Delete() on the
associated Composite, using the default Propagation Policy of "background".

If a standalone Composite object is deleted with --cascade=foreground, all the
"leaf" node Managed Resources are immediately deleted as described above, and
the ConnectionDetails are removed from all the Composite resources, and then the
Composite resources are deleted from etcd from the "bottom up" by garbage
collection using the _foregroundDeletion_ finalizer.

As a result of the above:
- The _--cascade=foreground_ option to kubectl is effectively non-functional on
  all Crossplane resources.
- Deletion of the claim or composite returns immediately, while actual managed
  resource deletion may take a considerable amount of time.
- Failed deletion of managed resources is not visible to the user. [1612]

### Goals

We would like to come up with a design that:

- Will not break any existing functionality.
- Will support the use of Foreground Cascading Deletion on Composite objects.
- Will support a simulated Foreground Cascading Deletion on Claim objects.

## Out of Scope

This design does not consider any changes to the Claim/Composite reference
design that would allow _kubectl delete --cascade=foreground_ to work from the
command line on a Claim resource.

This design is not intended to block resource deletion requests to the
Kubernetes API.

This design does not address use of the Orphan propagation policy when deleting
Claims or standalone Composite objects.

## Design

The intent of this design is to allow Foreground Cascading Deletion to work on
Crossplane resources.

Foreground Cascading Deletion requires the _blockOwnerDeletion_ attribute to be
set to _true_ on the Controller ownerReference that Crossplane adds to all
Composed resources.

Given the restrictions on Claim/Composite references, in order to simulate
Foreground Cascading Deletion on claims, a new attribute is added to the Claim
which enables the claim controller to call _Delete()_ on the composite resource
with propagation policy _Foreground_.

### API

#### Claim: compositeDeletePolicy

A _compositeDeletePolicy_ attribute is added to the Claim spec to indicate to
the claim reconciler that it should use a specific Propagation Policy when
deleting the associated Composite,   This attribute has possible values of
Background (default) and Foreground.  The value "Orphan" is intentionally not
supported as there is no requirement to leave the Composite resource in place
(and under reconciliation) while deleting the Claim.

The Claim reconciler uses the value of compositeDeletePolicy in the Delete API
call for the associated Composite.  If Background is used the Composite deletion
will occur as it does today and all associated resources will be marked for
deletion immediately and then reconciled.

If the value Foreground is used then the Composite and all of it's composed
resources will be deleted using Foreground Cascading Deletion and the delete
process with start at the leaf nodes and proceed back to the composite.

**Examples:**

Delete the Composite resource using Background propagation policy (existing
scenario):

```yaml
spec:
  compositeDeletePolicy: Background
```

Delete the Composite resource using Foreground propagation policy:

```yaml
spec:
  compositeDeletePolicy: Foreground
```

### User Experience - Before

#### Claim Delete - Background
- User executes _kubectl delete -n namespace <claim name>_.
- Kubernetes adds the deletionTimestamp attribute to the claim's metadata.
- Crossplane claim reconciler detects the deletion and executes a Kubernetes API
  Delete on the associated Composite with the default Background propagation
  policy.
- Kubernetes generates a graph of resources using ownerReferences and marks all
  resources as deleted with deletionTimestamp.
- Crossplane composite and managed reconcilers detect the deletion:
  - The Composite reconcilers delete the associated ConnectionDetails and
    remove the Crossplane finalizer.
  - The Managed reconcilers delete the remote external resources and remove the
    Crossplane finalizer.
- Kubernetes garbage collection removes the Composite/Managed objects after the
  Crossplane finalizers are removed.

#### Claim Delete - Foreground
- User executes _kubectl delete -n namespace --cascade=foreground <claim name>_.
- Kubernetes adds the deletionTimestamp attribute to the claim's metadata.
- There are no resources with ownerReferences that point at the claim so no
  finalizers are added.
- Crossplane claim reconciler detects the deletion and executes a Kubernetes API
  Delete on the associated Composite with the default Background propagation
  policy.
- Kubernetes generates a graph of resources using ownerReferences and marks all
  resources as deleted with deletionTimestamp.
- Crossplane composite and managed reconcilers detect the deletion.
  - The Composite reconcilers delete the associated ConnectionDetails and
    remove the Crossplane finalizer.
  - The Managed reconcilers delete the remote external resources and remove the
    Crossplane finalizer.
- Kubernetes garbage collection removes the Composite/Managed objects after the
  Crossplane finalizers are removed

#### Standalone Composite Delete - Background
- User executes _kubectl delete <composite name>_.
- Kubernetes adds the deletionTimestamp attribute to the composite's metadata.
- Kubernetes generates a graph of resources using ownerReferences and marks all
  resources as deleted with deletionTimestamp.
- Crossplane composite and managed reconcilers detect the deletion.
  - The Composite reconcilers delete the associated ConnectionDetails and
    remove the Crossplane finalizer.
  - The Managed reconcilers delete the remote external resources and remove the
    Crossplane finalizer.
- Kubernetes garbage collection removes the Composite/Managed objects after the
  Crossplane finalizers are removed.

#### Composite Delete - Foreground
- User executes _kubectl delete --cascade=foreground <composite name>_.
- Kubernetes adds the deletionTimestamp attribute to the composite's metadata.
- Kubernetes generates a graph of resources using ownerReferences and marks all
  resources as deleted with deletionTimestamp.
- Crossplane composite and managed reconcilers detect the deletion.
  - The Composite reconcilers delete the ConnectionDetails and remove the
    Crossplane finalizer.
  - The Managed reconcilers delete the remote external resources and remove the
    Crossplane finalizer.
- Kubernetes garbage collection removes the Composite/Managed objects after the
  Crossplane finalizers are removed.

### User Experience - After

#### Claim Delete - Background (no change)

#### Claim Delete - Foreground
- A Claim exists with compositeDeletePolicy: Foreground.
- User executes _kubectl delete -n namespace <claim name>_.
- Kubernetes adds the deletionTimestamp attribute to the claim's metadata.
- There are no resources with ownerReferences that point at the claim so no
  finalizers are added at this stage.
- Crossplane claim reconciler detects the deletion and executes a Kubernetes API
  Delete on the associated Composite with the Foreground propagation policy
- Kubernetes generates a graph of resources using ownerReferences, adds
  _foregroundDeletion_ finalizers to all "controller" objects, and marks all
  resources as deleted with deletionTimestamp.
- Crossplane composite and managed reconcilers detect the deletion:
  - The Composite reconcilers delete the associated ConnectionDetails and
    remove the Crossplane finalizer.  The _foregroundDeletion_ finalizer on the
    Composites block garbage collection from deleting the resource.
  - The Managed reconcilers delete the remote external resources and remove the
    Crossplane finalizer, and garbage collection removes the resources.
- When all the Composed Resources for a Composite have been removed, Kubernetes
  removes the _foregroundDeletion_ finalizer from the Composite, and it is
  removed by garbage collection.
- This process repeats itself until all the Composites have been deleted,
  including the root Composite.
- The Claim reconciler detects that the Composite has been deleted and removes
  the Crossplane finalizer.
- Kubernetes garbage collection removes the Claim object after the Crossplane
  finalizer has been removed.

#### Standalone Composite Delete - Background (no change)

#### Composite Delete - Foreground
- User executes _kubectl delete --cascade=foreground <composite name>_.
- Kubernetes adds the deletionTimestamp attribute to the composite's metadata.
- Kubernetes generates a graph of resources using ownerReferences, adds
  _foregroundDeletion_ finalizers to all "controller" objects, and marks all
  resources as deleted with deletionTimestamp.
- Crossplane composite and managed reconcilers detect the deletion:
  - The Composite reconcilers delete the associated ConnectionDetails and
    remove the Crossplane finalizer.  The _foregroundDeletion_ finalizer on the
    Composites block garbage collection from deleting the resource.
  - The Managed reconcilers delete the remote external resources and remove the
    Crossplane finalizer, and garbage collection removes the resources.
- When all the Composed Resources for a Composite have been removed, Kubernetes
  removes the _foregroundDeletion_ finalizer from the Composite, and it is
  removed by garbage collection.
- This process repeats itself until all the Composites have been deleted,
  including the root Composite.

### Implementation

Support for Foreground Cascading Deletion requires:
- setting the _blockOwnerDeletion_ flag to true on all Controller
  ownerReferences created by Crossplane
- adding a compositeDeletePolicy attribute to the Claim API
- update the claim reconciler to use the compositeDeletePolicy when calling
  Delete() on the composite
- update the claim reconciler to requeue and wait for the composite to finish
  deletion in the Foreground case

Setting the _blockOwnerDeletion_ flag in the Controller ownerReference is
required to indicate that Kubernetes should set the _foregroundDeletion_
finalizer on the owner resource when foreground cascading deletion is specified.
This is the also expected configuration for controller owner references as
described in the Kubernetes server.

We can simulate foreground cascading deletion on Claim objects by adding a
compositeDeletePolicy attribute to the Claim specification.  This attribute will
determine the propagation policy that should be used by the Claim reconciler
when it calls the Delete() function on the associated Composite object.  If the
compositeDeletePolicy value is "Foreground", then the Composite will be deleted
with the Foreground propagation policy and the Claim reconciler will requeue as
long as the Composite resource is not deleted.


[foreground]: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/
[block]: https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/controller_utils.go#L510
[1612]: https://github.com/crossplane/crossplane/issues/1612

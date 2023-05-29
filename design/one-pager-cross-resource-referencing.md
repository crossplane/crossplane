
# Cross Resource Referencing

> Note that while the problem and API proposed by this design document remain
> relevant, some of the implementation details (i.e. the code that resolves
> references) have since changed.

* Owner: Javad Taheri (@soorena776)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

Crossplane provisions and monitors external resources in third party cloud
providers, following the declared `spec` in the corresponding managed resource
objects. With the rise of [*gitops*][gitops doc] style provisioning, in a lot of
scenarios where multiple interconnected resources need to be provisioned,
resource `r2` might consume some attribute of resource `r1`. This article
discusses such cross resource references and proposes solutions to address the
challenges in Crossplane.

## Definitions

* **Managed Resource Solution**

  In a lot of scenarios a single resource is not operational or very useful on
  its own. In order to achieve a desired functionality, usually a group of
  resources need to be provisioned and configured to communicate with each
  other. For example in order to have an `EKS` cluster set up in AWS, in
  addition to an `EKS` instance we also need to provision and configure the
  required network resources like `VPC` and `Subnet`, and an `RDS` security
  group for database access. In this article we call such set of resources which
  together form a desired functionality or configuration, a *Managed Resource
  Solution*, or just a *Solution* for brevity.

* **Blocking and Non-Blocking Dependencies**

  Imagine two resources `r1` and `r2` in a solution. We define `r2` has a
  *blocking* dependency on `r1`, if provisioning `r2` requires any attributes of
  `r1`. For instance, a `Subnet` has a blocking dependency on a `VPC`, since
  `VPC.Id` is required for provisioning the `Subnet`.

  In addition we define `r2` has a *non-blocking* dependency on `r1`, if `r2`
  doesn't require any attributes of `r1` to be provisioned but the functionality
  desired by the solution requires existence of both `r2` and `r1`. For example,
  an `EKS` cluster needs to have the right `IAMRolePolicyAttachment` resource in
  order to have to required permissions when accessing resources, even though it
  does not need any attributes of `IAMRolePolicyAttachment` to be provisioned.

* **Non-deterministic Resource Attribute**

  We define the attribute `foo` of resource `r` *non-deterministic*, if `foo`'s
  value only becomes known after `r` is provisioned. For example `vpcID` is a
  non-deterministic attribute of a `VPC` resource.

* **Composite Resource Attribute**

  We define the attribute `foo` of resource `r` *composite*, if `foo`'s value is
  deterministic and composed of the attributes of other resources. As an
  example, `network` attribute of `Subnetwork` type in GCP, is formed as

  `/projects/[gcp-project]/global/networks/[network-name]`

  where `[gcp-project]` and `[network-name]` are attributes of other resources.

## Objectives

### Goals

* Support `gitops` style declarative resource provisioning. Apply a directory of
  YAML resources, which will eventually become a series of online and
  functioning managed resources. This requires:
  * Support to reference attributes of other resources in a resource YAML object
  * Support for cross resource blocking dependency

* Support using existing external resources to be referenced by the YAML objects

### Non Goals

* Support non-blocking dependencies between resources

## Proposal

Let's consider the following sample solution where a `VPC`, and a `Subnet` needs
to be provisioned. The YAML object will look like following:

```yaml
---
apiVersion: network.aws.crossplane.io/v1alpha2
kind: VPC
metadata:
  namespace: cool-ns
  name: my-vpc
spec:
  ...
---
apiVersion: network.aws.crossplane.io/v1alpha2
kind: Subnet
metadata:
  namespace: cool-ns
  name: my-subnet
spec:
  # this is the vpcId of the external vpc, represented by my-vpc
  vpcId: [my-vpc_vpcId]
  ...
```

### Cross referencing using Attribute Referencers

In this example since `vpcId` is non-deterministic we will need a mechanism to
indicate this cross reference in the YAML object. We propose the notion of
*Attribute Referencer*, as a `go` interface as following:

```go
type AttributeReferencer interface {

	// GetStatus looks up the referenced objects in K8S api and returns a list
	// of ReferenceStatus
	GetStatus(context.Context, CanReference, client.Reader) ([]ReferenceStatus, error)

	// Build retrieves referenced resource, as well as other non-managed
	// resources (like a `ProviderConfig`), and builds the referenced attribute
	Build(context.Context, CanReference, client.Reader) (string, error)

	// Assign accepts a managed resource object, and assigns the given value to the
	// corresponding property
	Assign(CanReference, string) error
}
```

Having this interface, we can implement `VpcIDRefResolver` as

```go
type VpcIDRefResolver struct {
  // the object that is needed for resolving vpcID
  ObjectReference `json:"inline"`
}
```

And then we implement the `AttributeReferencer` method sets.

* **Simplifying Assumption** We assume that each `AttributeReferencer` field
  only needs to refer to *one* resource object. If more resources are needed,
  those resources can be referenced using other attributes of the source, or the
  referenced resource. This assumption helps us have a consistent API with
  Kubernetes referencer fields (e.g. with a `Ref` suffix), where an
  `ObjectReference` field is used for referencing.


Using `VpcIDRefResolver`, we then can modify [Subnet type] as:

```diff
- // VPCID is the ID of the VPC.
- VPCID string `json:"vpcId"`
+ // VPCIDRef resolves the VPCID from the refenreced VPC
+ VPCIDRef *VpcIDRefResolver `json:"vpcIdRef" resource:"attributereferencer"`
```

Now we can update the `Subnet` YAML object in the sample solution as following:

```yaml
---
apiVersion: network.aws.crossplane.io/v1alpha2
kind: Subnet
metadata:
  namespace: cool-ns
  name: my-subnet
spec:
  # reference to API objects from which the vpcId will be resolved
  vpcIdRef:
    name: my-vpc
  ...
---
```

Note here that we added a `Ref` suffix to the `vpcIdRef`, emphasizing that it is
different than `vpcId`. In addition we used `resource:"attributereferencer"`
tag, to explicitly indicate that this field implements that interface, which is
used for type validation, code readability and showing the intention of the
field explicitly.

This mechanism resolves the referenced non-deterministic attributes, as it waits
for the specified object to become available using `GetStatus` method. Once the
referenced resource is ready, the corresponding `Build` method executes and
retrieves the required attributes from various objects and builds the desired
attribute. Finally, the `Assign` method assigns the built value to the right
field in the owning object.

Using the same mechanism, composite attributes could also be referenced and
built. In this case, the `Build` method potentially would implement a more complex
composition logic.

#### Implementation in Crossplane

To implement the above mentioned cross referencing in Crossplane, we modify the
[Managed Reconciler] in crossplane-runtime to add logic to call `ResolveReferences`.
`ResolveReferences` will check to see if any of the fields in the give API type are of
interface type `AttributeReferencer`, and if so, attempts to resolve them. If
resolution for any reason is not completed, reconciliation gets rescheduled.
Once a reference field is resolved, its value will be stored in the equivalent
non `Ref` field, and reconciler proceeds to the next steps.

### Maintain High Fidelity

When provisioning resources, it is desirable to support existing external
resources which are not managed by Crossplane. For instance assume that in the
`VPC` and `Subnet` sample solution, the `VPC` resource already exists and we
only want to provision the `Subnet`. Since we changed the `VPCID` to `VPCIDRef`
in the `Subnet` type, it won't be possible to use the external VPCID.

To support this case, we need to keep the `VPCID` field in `Subnet`, so we
update the modification as:

```diff
  // VPCID is the ID of the VPC.
- VPCID string `json:"vpcId"`
+ VPCID string `json:"vpcId,omitempty"`
+ // VPCIDRef resolves the VPCID from the refenreced VPC
+ VPCIDRef VpcIDRefResolver `json:"vpcIdRef,omitempty"`
```

Note that we added `omitempty` rule to both fields, making them optional.


If both fields are provided `VPCIDRef` takes priority and overwrites `VPCID`
with the resolved value.

### Project Blocking Dependency in Resource Status

To show the status of resolving references of a resource, we add the new
condition type `ReferencesResolved` to the existing Managed resources
Conditions. Resolving a referenced attribute results in one of the following
outcomes, ordered with higher priority:

1. An error occurs during resolving references. In this case `ReconcileError` condition is
   added to resources conditions, and the resource is rescheduled for
   reconciliation.

2. The referenced object *doesn't exist*, or is not yet *Ready*. In this case
   the resource will be assigned with a `ReferencesResolved` condition with
   `Status=ConditionFalse`, and its `Reason` listing the resources that don't
   exist or are not ready. Also, the resolving should be
   re-scheduled with a *long wait*.

3. The referenced object is *Ready*. In this case
   the resource will be assigned with a `ReferencesResolved` condition with
   `Status=ConditionTrue`.


If two or more referenced objects have different outcomes, the status of the
resource should be updated to the outcome with the higher priority.

### Cleaning Up a Solution

After a solution is created in Crossplane by running:
> `kubectl apply -f <directory of YAML>`

it's also desirable to be able to delete it by running:

> `kubectl delete -f <directory of YAML>`

When deleting the managed resources of a solution in Crossplane, it is possible
that some of the corresponding external resources cannot be deleted, as other
resources depend on them and the cloud provider doesn't allow such deletion. For
example in the sample solution described above, AWS blocks deletion of the
external resource of `VPC` as long as there is an external `Subnet` which
consumes that `VPC`.

This problem will be solved automatically as all managed resources eventually
reconcile after retrying. In the sample solution example, the `Subnet` external
resource gets eventually deleted and hence the next attempt to delete the `VPC`
will succeed as there will be no more depending resources. This could later be
improved by leveraging [Foreground cascading deletion] mechanism, where
dependent objects are deleted first.


### Related Issues

* [Inter-resource attribute
  references](https://github.com/crossplane/crossplane/issues/707)
* [Honoring inter-resource dependency when creating/deleting
  resources](https://github.com/crossplane/crossplane/issues/708)
* [Resource
  Connectivity](https://github.com/crossplane/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md)

[gitops doc]: (https://www.weave.works/blog/what-is-gitops-really) 
[Subnet type]:
(https://github.com/crossplane/provider-aws/blob/master/apis/network/v1alpha2/subnet_types.go#L25-L37)
[Subnetwork type]:
(https://github.com/crossplane/provider-gcp/blob/master/apis/compute/v1alpha2/subnetwork_types.go#L144)

[Managed Reconciler]:
https://github.com/crossplane/crossplane-runtime/blob/master/pkg/reconciler/managed/reconciler.go
[Foreground cascading deletion]:
(https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#foreground-cascading-deletion)

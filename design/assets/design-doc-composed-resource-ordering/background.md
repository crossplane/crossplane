# Background

Crossplane's Composition engine does not support the ordered creation and
deletion of resources by design. It follows Kubernetes patterns where
resources are created asynchronously and controllers operate to implement
the requested desired state. If an AWS network is created in a Composition,
the VPC, Subnets, and Routes are created in parallel. When
the Composite Resource (XR) is deleted all the Composed resources are marked for deletion
in parallel.

Unfortunately, the infrastructure Crossplane manages outside the cluster
is not as forgiving as Kubernetes resources are. There are many instances
when infrastructure platforms need to track the dependencies between
resources under management:

* Blocking deletion of a Kubernetes Cluster until all the workloads running on
  it have been deleted.
* Delaying the creation of Users and Schemas into a Database until the Database
  has been provisioned and is healthy.
* Predicting the scope of proposed infrastructure changes in regulated
  environments to reduce risk.

These requirements have led to the development of a number of solutions within
Crossplane and the broader community:

* Using a
  [`fromFieldPath.policy: Required`](https://docs.crossplane.io/latest/guides/function-patch-and-transform/#fromfieldpath-policy)
  in patch-and-transform to delay rendering of a Managed Resource (MR)until a field
  is present.
* [Usages](https://github.com/crossplane/crossplane/blob/main/design/one-pager-generic-usage-type.md)
  block deletion at the API server and are commonly used to enforce deletion ordering.
* Provider [Managed Resource
  References](https://github.com/crossplane/crossplane/blob/main/design/one-pager-cross-resource-referencing.md)
  cause a Managed Resource to return an error until the Referred resource
  exists.
* Within function logic using the Observed state of other Objects in a
  conditional statement.
* Adding a pipeline Function like
  [function-sequencer](https://github.com/crossplane-contrib/function-sequencer)
  create ordering by controlling the rendering of desired state.
* Via function frameworks as in
  [function-pythonic's](https://github.com/crossplane-contrib/function-pythonic#composed-resource-dependencies)
  automatic dependency creation.
* Using tools like
  [Kyverno](https://github.com/crossplane/crossplane/discussions/4072) to block
  deletion of a resource.

While these solutions solve real problems, they place the responsibility on the
Crossplane Community who must piece together various solutions and introduce
complexity into their environments.

Building on the success of
[Composition
Functions](https://github.com/crossplane/crossplane/blob/main/design/design-doc-composition-functions.md),
I believe Crossplane can implement a resource graph in the Composition engine,
significantly reducing the use of Usages and other workarounds.

## Resource Ordering in Composition Functions

Let's go into more detail how resource ordering works in practice today.
Composition Functions run as
an ordered pipeline of stateless functions. Each
function receives, via a gPRC `RunFunctionRequest`, the full desired and observed
state accumulated by the functions before it, and is required to pass forward
anything it doesn't have an opinion on. The observed state is provided to the function
by Crossplane and Providers: it contains the latest observations of all resources
in the Composition. The function's role is to use this information to create the desired state,
and populate the status of the Composite resource.

Composed resources are identified by
name, as keys into `State.Resources`. Within a function a VPC's ID would be found at the
field path `observed.resources.["vpc-name"].resource.status.atProvider.id`.

`RunFunctionRequest.observed.resources` carries the full composed resource
objects, including their status conditions, so a function can control resource
creation on observed state.

In functions this pattern is used to manage creation ordering: don't add the Subnet to desired state until the VPC
appears in observed state as ready. Crossplane never creates the Subnet until
the function says so. An example of this pattern in KCL is 
[configuration-aws-eks](https://github.com/upbound/configuration-aws-eks/blob/5b6c3d384fc9c2c66223f4433dd6bc1ba6478d70/functions/eks/main.k#L222).

Because each step of the function pipeline receives the output fromm the previous step, we can have a function
that managing creation and deletion ordering:
[`function-sequencer`](https://github.com/crossplane-contrib/function-sequencer/blob/main/README.md), lets a
Composition author declare ordering as data in a pipeline step's input:

```yaml
  - step: sequence-creation
    functionRef:
      name: function-sequencer
    input:
      apiVersion: sequencer.fn.crossplane.io/v1beta1
      kind: Input
      resetCompositeReadiness: true
      rules:
        - sequence:
          - first-resource-.*
          - second-resource
        - sequence:
          - first-resource-.*
          - third-resource
```

It reads accumulated desired state, and holds a resource back by removing it
from desired until every predecessor is ready. For deletion it composes `Usage` resources, which
block deletion at admission until the dependent is gone.

The third major pattern is to embed dependency ordering a framework or SDK.
This technique is followed by function-pythonic, skipping rendering on
creation and Creating Usages to control deletion.

A function that stops returning a resource in desired state causes Crossplane to
garbage collect it, so resources can be torn down a wave at a time. Once an XR is deleted, the function is no longer invoked and there is no ability to control the order of deletion. This is being addressed in [Pull Request7242](https://github.com/crossplane/crossplane/pull/7242), and
is complementary to this proposal.

## Ordering via Managed Resources References

Providers also have a method of defining dependencies: a Managed Resource can have fields
that are set from a reference to another Managed Resource on the cluster instead of the Composition.

For example, a subnet contains several
ways to define a reference to the required [`vpcId`](https://marketplace.upbound.io/providers/upbound/provider-aws-ec2/v2.7.2/resources/ec2.aws.m.upbound.io/Subnet/v1beta1#doc:spec-forProvider-vpcId):

* `vpcId`: the cloud Provider's id for the resource, usually the Crossplane
  [external-name](https://docs.crossplane.io/latest/concepts/managed-resources/#naming-external-resources)
  annotation. This is a static field set by the Composition.
* `vpcIdRef`: A Reference to a Kubernetes Managed Resource object by Name and
  optional Namespace. The Kubernetes Kind and API Group is hardcoded in the Provider,
  and the object is fetched from the API Server by the Provider. The Provider
  then extracts the ID and updates the spec of the requesting object.
* `vpcIdSelector`: Instead of using a Kubernetes Name/Namespace the Provider can
  match multiple objects on the cluster using labels. The Provider proceeds to fetch each
  matching resource and extract the required field.

An example use of Managed Resource References can be seen in the KCL
implementation of
[configuration-aws-network](https://github.com/upbound/configuration-aws-network):

```kcl
 ec2v1beta1.RouteTableAssociation{
        metadata = _metadata("rta-" + _formatSubnet(s)) | {
            labels = {
                "networks.aws.platform.upbound.io/network-id" = oxr.spec.parameters.id
            }
        }
        spec = _defaults | {
            forProvider = {
                region = oxr.spec.parameters.region
                routeTableIdSelector = {
                    matchControllerRef = True
                }
                subnetIdSelector = {
                    matchControllerRef = True
                    matchLabels = {
                        if s.type == "private":
                            access = "private"
                        else:
                            access = "public"
                        zone = s.availabilityZone
                    }
                }
            }
        }
```

Note that the `matchControllerRef` configures the behavior of the reference: if
set to `true` it looks within the Composition. Otherwise it looks for the
resource outside of the Composition. `matchControllerRef` predates Crossplane
Functions and Required Resources and overlaps them in functionality:

* Functions have access to the Observed state of the Composition during every
  execution and can extract any fields directly.
* Required Resources can import state from Resources outside the composition.
  Crossplane is responsible for the fetching of the resources instead of the
  Provider.

While Managed Resource References can make Composition authoring simpler, there are notable
concerns with their use for resource ordering:

* The resolver is implemented in the Provider code and only supports certain fields.
* The Provider implements the `ResolveReferences` method, making calls to Kubernetes API server to fetch other resources.
* If nothing matches, the resolver generates a `ReconcileError` and requeues the request.
* It does not utilize `Ready` status: references populate the spec field as soon as the Provider is able to fetch a value.
* There is no support for deletion ordering, only ordering creation. On delete reference resolution is halted.
* Generated SDKs from Provider CRDs currently do not include type information about the reference. It must be inferred from the comments.
* The XR does not have visibility on the relationship, only that the resource has an `Synced=False` Condition. The XR cannot tell if the resource is unable to Sync, or whether the wait is based on existence of health of the matching resource.
* `policy.resolution: Optional` allows the resource to proceed without matching or generating any events. This introduces a silent condition into the Composition.

### Problems with Current Approaches

These examples demonstrate that while resource ordering is possible, it is
ad-hoc and inconsistent. This leads to less-than-optimal results in an
infrastructure platform:

* **"Not desired" and "desired, but later" are equivalent.** A function
  delays a resource by omitting it from desired state, which is
  indistinguishable from deciding the resource shouldn't exist. Crossplane
  therefore reports an XR as `Synced` and possibly `Ready` while composition is
  deliberately incomplete.
  
  `function-sequencer` ships a
  `resetCompositeReadiness` flag specifically to address this, described
  in its own documentation as
  preventing the XR from "entering the `Ready` state prematurely when there are
  pending resources that the composite reconciler is unaware of."

* **Ordering stops at creation.** Removing a live resource from desired state
  would delete it, so a function that delays by omission can only ever order
  creates. `function-sequencer` skips any resource already present in observed
  state, deliberately and correctly. `function-pythonic` supports creating
  dependencies using a combination of selective rendering and creation of
  `Usage` objects.

* **Deletion ordering costs a Kubernetes Object per protected Resource.**
  Expressing teardown order through `Usage` objects means one `Usage` per
  dependency pair: each with its
  own reconcile, finalizer, and admission round trip. It also requires deleting
  the XR with `--cascade=foreground`, which is the one propagation mode that
  defeats function-controlled deletion (see "Interaction with
  Function-Controlled Deletion" below).

  In a complex project like
  [modelplane](https://github.com/modelplaneai/modelplane) a single deployment
  can contain over a dozen `Usage` objects on the cluster.

* **It is difficult to test changes to Managed Resource References.** Function
  authors must use Observed Resources during testing to simulate ordering.

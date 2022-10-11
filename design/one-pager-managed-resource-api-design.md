# Managed Resources API Patterns
* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Accepted, Revision 1.3

## Revisions

* 1.1 - Daniel Mangum (@hasheddan)
  * Added [Embedded Structs with Mixed
    Fields](#embedded-structs-with-mixed-fields) and [Optional Embedded Structs
    with One Field](#optional-embedded-structs-with-one-field) sections
  * Updated examples with cluster-scoped managed resource and `ProviderConfig` objects
* 1.2 - Muvaffak Onus (@muvaf)
  * Added [how to handle sensitive input fields](#sensitive-input-fields).
  * Added [external resource labelling](#external-resource-labeling).
  * Added [cross-resource reference edge case](#pointer-types-and-markers) for
    types.
  * Added a condition to enforce the status fields to be reproducible in section
    [High Fidelity](#high-fidelity).
* 1.3 - Daniel Mangum (@hasheddan)
  * Removed definition for _Claim_ as claims are to be deprecated per
    [#1479](https://github.com/crossplane/crossplane/issues/1479).
  * Added section on choosing a [GVK for a resource](#group-version-kind-gvk).
* 1.4 - Muvaffak Onus (@muvaf)
  * Expanded [immutability section](#immutable-properties) to cover selector fields..
  * Updated [labelling section](#external-resource-labeling) with current implementation.
* 1.5 - Alper Rifat Ulucinar (@ulucinar)
  * Added a new section on the [pause annotation](#pause-annotation) for
  the managed resources.

## Terminology

* _External resource_. An actual resource that exists outside Kubernetes,
  typically in the cloud. AWS RDS or GCP Cloud Memorystore instances are
  external resources.
* _Managed resource_. The Crossplane representation of an external resource. The
  `RDSInstance` and `CloudMemorystoreInstance` kinds are managed resources.
* _Provider_. Cloud provider such as GCP, AWS, Azure offering IaaS, cloud
  networking, and managed services.
* _Spec_. The sub-resource of Kubernetes resources that represents the desired
  state of the user.
* _Status_ The sub-resource of Kubernetes resources that represents the most
  up-to-date information about the resource.

For other terms, see [terminology].

## Background
Crossplane extends Kubernetes heavily and the maintainers always try to adopt
the patterns that are already seen in Kubernetes resources, notably declarative
style API. However, it's not always intuitive for everyone to see what goes to
where in a way that is consistent and just as expected by the consumers of those
resources.

This document tries to capture how the bridge between Crossplane and provider is
shaped. So, the features of managed resources that are generic to all of them
are not included here, for example reclaim policies. This is more about the
things that a developer should keep in mind when they try to implement a new
managed resource.

A few questions that this document needs to help answering:
- What goes into `Spec` and what goes into `Status` of a managed resource?
- Should _all_ information from the provider be represented in the Custom
  Resource?
- What parts of the Custom Resource should be allowed to be consumed by the
  consumers?
- Do we represent _desired_ state, _observed_ state or both?
- How should we differentiate the fields that are used by provider from
  Crossplane's fields?

While this document is heavily opinionated, each provider can wildly vary
between each other. So, treat this document as a starting point, try to stick to
it for consistency but deviate if required and open an issue about why you had
to deviate so that we can reassess and improve it.

## Guiding Principles

Our users are our stakeholders. They influence how we make decisions. There are
two types of users for managed resources:
* Input Users: Users who configure/create/manage the resource. We want our
  `Spec` to be the source of truth for them to configure everything about the
  resource.
* Output Users: Users who refer to/query/use/extract information from the
  managed resource. We want them to be able to perform all read-only operations
  on the given resource as if they have the cloud provider's API available. This
  can include configurations and metadata information about the resource.

### Group Version Kind (GVK)

All Kubernetes Custom Resources belong to a group and are of a certain kind and
version. There may be multiple kinds in a group and multiple versions of the
same kind. In Crossplane, Custom Resources should mirror the provider API type
that they represent as closely as possible in group and kind. For instance, AWS
has a `Cluster` type that is part of the `eks` [package]. The first version of
the `Cluster` custom resource should be of `kind: Cluster` and `apiVersion:
eks.aws.crossplane.io/v1alpha1`.
  
### Naming Conventions

There are two naming decisions here to make:
* Managed resource's Custom Resource (CR) name
* Identifier string of the external resource in the cloud provider.

_Name_ in this context means the identifier string for the resource in its
environment. CR name is the identifier for resource in Kubernetes environment.
But for external resources, this can be either `id`, `uid` or `name` properties
of the resource. The one we want to name is the one that is used when _referring
to_ that resource in the same environment. For example, in AWS, VPCs do not have
a name but `VpcID` properties that is used for identification.

#### Custom Resource Instance Name

The name of the managed resource CR, which lives in our control cluster. The
claim controller who creates the managed resource gives its name or some other
entity might create the managed resource, like user themselves. So, do not
assume that all the managed resources will have the same naming format anywhere.

Crossplane lives on Kubernetes and both the user and admin interacts with it
through Kubernetes APIs. For better UX we want to mirror what Kubernetes uses
for identification of a resource as much as possible, which is name and
namespace pair. So, the claim controller should use these two entities of the
Claim that it's processing when it decides on a name. However, the resources in
the cloud provider are in the same environment, either per project (GCP) or same
account (AWS). In order to provide a unique name that incorporates name and
namespace, claim controller should make use of `GenerateName` feature of
resource creation. The claim controller should assign `namespace-name` to
`GenerateName` field of the `ObjectMeta` of the managed resource and Kubernetes
will automatically append `-<5 characters random string>` to that when the
creation is completed. For example, the following will be created by the claim
controller:
```yaml
apiVersion: database.gcp.crossplane.io/v1alpha2
kind: CloudSQLInstance
metadata:
  generateName: myappnamespace-mydatabase-
```
After Kubernetes API server completes the creation, we'll see something like the
following:
```yaml
apiVersion: database.gcp.crossplane.io/v1alpha2
kind: CloudSQLInstance
metadata:
  name: myappnamespace-mydatabase-5sc8a
```
`mydatabase` is the name of the `Claim` and `myappnamespace` is the namespace
that `Claim` lives in. That way we get a managed resource that user can relate
to what they actually created.

However, there is a common case that for some resources, the provider does not
allow you to specify a name or the claim might get bound to an existing managed
resource without any need to create a new one. To show the actual name of the
resource that's shown in the provider's UI, claim controller should copy
`crossplane.io/external-name` annotation's value from the managed resource to
its own `crossplane.io/external-name` annotation after the managed resource is
bound to the claim.

We also want to give the ability to specify the `crossplane.io/external-name` in
the claim level. So, if `crossplane.io/external-name` is given _before_ the
creation of managed resource, we should copy its value to managed resource's
`crossplane.io/external-name` annotation before we create it. Note that you
never override managed resource's annotation if it already exists. Then in the
next reconciles, we always get the value from managed resource and copy it to
`crossplane.io/external-name` annotation of `Claim`.

Note that `crossplane.io/external-name` always shows the final value of the
external resource name.

#### External Resource Name

Related to https://github.com/crossplane/crossplane/issues/624

The decision for an external resource name is made by the controller of that
managed resource in `Create` phase. Possible cases are as following:
* The provider doesn't allow us to specify a name. Fetch it after its client's
  `POST` call. Override `crossplane.io/external-name` annotation's value with
  what you get as name.
* The provider allows to specify a name.
  * If `crossplane.io/external-name` annotation has a value use it. If it's
    empty;
  * Use managed resource's name and write that into
    `crossplane.io/external-name` annotation.

Use the value of that annotation as external resource name in _all_ queries.

#### External Resource Labeling

If the external resource supports labelling, we should label it with the managed
resource name, kind and provider. This is helpful in variety of scenarios like:
* Identify services with non-deterministic naming, for example AWS VPC.
* Provider level operations that are done via label filtering, for example
  search and batch operations.
* Find a resource in the provider UI and construct an easy `kubectl` query on
  Crossplane cluster to find the associated resource, like `kubectl get <kind>
  <name>`

The keys to use in labels are like the following:
```
A tag set for a VPC in AWS:
  "crossplane-kind": "vpc.network.aws.crossplane.io"
  "crossplane-name": "myappnamespace-mynetwork-5sc8a"
  "crossplane-provider": "aws-provider"
```

In cases where the characters `.` and `/` are not allowed in key string, `-`
should be used. If that's not allowed, too, then those characters should be
omitted.

#### Field Names

The thinking process of naming decision could respect provider's decision as the
starting point and deviate from that if there is a very good reason to. Naming
parity makes inter-resource references easier and lowers the entry barrier for
both developers and users. This is especially handy in cases where Crossplane
does not yet support the _referred_ resource or user may not want some of their
resources in their Crossplane environment.

In some cases, we might have collisions or cases where provider field name is
too similar to another field in the managed resource's Custom Resource fields.
The section [Owner-based Struct Tree](#owner-based-struct-tree) tries to tackle
that.

### High Fidelity

Related to https://github.com/crossplane/crossplane/issues/621 and
https://github.com/crossplane/crossplane/issues/530

Crossplane managed resources should expose everything that provider exposes to
its users as much as possible. A few benefits of that high fidelity:

* The ability to read and set every 'knob' the external resource supports. This
  would make it easy to support wide variety of customizations without having to
  think of them in development time.
* An obvious mapping of Crossplane's fields to cloud provider API fields, exact
  name match as much as possible. This would make it easier to work with
  Crossplane for users who are familiar with the given provider in many ways
  including easier troubleshooting and cooperation with other existing
  infrastructure tools.

What goes into `Spec`:
* Does the provider allow configuration of this parameter at _any_ point in the
  life cycle of the resource? Include if yes. This includes the fields that are
  late-initialized, meaning some fields could be populated by the provider when
  you actually create the resource but it also gives you the ability to change
  them. An example for this could be auto-generated resource tags or
  resource-specific defaults. If the provider tags the resource without us
  telling them to do so, controller should update `Spec` and input user should
  make changes on that current value of the field. Related to
  https://github.com/crossplane/crossplane-runtime/issues/10

Note that the controller should make updates only to `Spec` fields that are
empty. We do not override user's desired state and if they have no control over
it in any case, do not include it in `Spec`.

What goes into `Status`:

* Can the value of this field be reproduced when the whole `Status` is deleted?
  Do not include if the answer is no. `Status` sub-resource is the
  _representation_ of the current state, so, the controller should be able to
  reproduce it as long as the resource is still there. In practice, this means
  controller of that managed resource should not have to rely on `Status` fields
  of its custom resource while operating.

* All fields except the ones that are chosen for `Spec`.

For both `Status` and `Spec`:
* Is this field represented as standalone managed resource? Do not include if
  the answer is yes. We should not manage an external resource in the CR other
  than its original external resource. For example, Azure VirtualNetwork object
  allows you to create Subnets by providing an array of Subnet objects as one of
  its fields. However, Subnet is already another managed resource supported by
  Crossplane. In that case, we should not allow configuration of Subnets through
  VirtualNetwork CR but require people to do it via Subnet CR. Though you might
  refer to it for configuration purposes, having two controllers managing one
  resource (VirtualNetwork CR and Subnet CR controllers) would not work well.
  Since the corresponding CR's controller does not manage those fields, we don't
  include it in the `Status` as well.

  What if the sub-resource is not yet supported as managed resource in
  Crossplane? In that case, you should first consider implementing that managed
  resource, if not suitable, only then include it in the CR.

For details, see [Kubernetes API Conventions - Spec and Status].

#### Embedded Structs with Mixed Fields

Some provider APIs include an embedded struct that may contain some fields that
are appropriate for `spec.forProvider` and some that are meant for
`status.atProvider`. For example:

```go
// This is the provider's representation of the API object
type ProviderAPIObject struct {
  // Configurable field in top-level object
  FieldOne *string `json:"fieldOne,omitempty"`

  // Non-Configurable field in top-level object
  FieldTwo *string `json:"fieldTwo,omitempty"`
  
  // Embedded struct in top-level object
  EmbeddedStructOne *EmbeddedStruct `json:"embeddedStructOne,omitempty"`
}

type EmbeddedStruct struct {
  // This field is configurable so it should be in spec.forProvider
  SomeConfigurableField *string `json:"someConfigurableField,omitempty"`

  // This field is configurable so it should be in spec.forProvider
  AnotherConfigurableField *string `json:"anotherConfigurableField,omitempty"`

  // This field is not configurable so it should be in status.atProvider
  SomeNonConfigurableField string `json:"someNonConfigurableField,omitempty"`
}
```

In this case, the solution is to divide the embedded struct into
`EmbeddedStructSpec` and `EmbeddedStructStatus`.

```go
// This is the Crossplane representation of the API object spec
type CrossplaneAPIObjectSpec struct {
  // Configurable field in top-level object
  // +optional
  FieldOne *string `json:"fieldOne,omitempty"`
  
  // Embedded struct in top-level object
  // +optional
  EmbeddedStructOne *EmbeddedStructSpec `json:"embeddedStructOne,omitempty"`
}

// Only the configurable fields in EmbeddedStruct
type EmbeddedStructSpec struct {
  // This field is configurable so it should be in spec.forProvider
  // +optional
  SomeConfigurableField *string `json:"someConfigurableField,omitempty"`

  // This field is configurable so it should be in spec.forProvider
  // +optional
  AnotherConfigurableField *string `json:"anotherConfigurableField,omitempty"`
}

// This is the Crossplane representation of the API object status
type CrossplaneAPIObjectStatus struct {
  // Non-Configurable field in top-level object
  FieldTwo *string `json:"fieldTwo,omitempty"`
  
  // Embedded struct in top-level object
  EmbeddedStructOne *EmbeddedStructStatus `json:"embeddedStructOne,omitempty"`
}

// Only the non-configurable fields in EmbeddedStruct
type EmbeddedStructStatus struct {
  // This field is not configurable so it should be in status.atProvider
  SomeNonConfigurableField string `json:"someNonConfigurableField,omitempty"`
}
```

#### Sensitive Input Fields

Some cloud services can take sensitive input such as passwords, certificates or
tokens. However, exposing those fields on the CR is not the best way to handle
it. For such fields, the field name should be `<field-name>SecretRef` and the
type of that field should be `SecretKeySelector`([from crossplane-runtime]). The
controller should fetch the value from the given secret and populate the field
in the provider's corresponding SDK object.

In case the secret reference is not provided **and** the field is a required
one, such as password, the controller should generate it randomly. Then it has
to make sure that:
* GoDoc comment explicitly states that if `<field-name>SecretRef` is not
  populated, a random value will be generated for it.
* The generated value ends up in the connection details secret that is published
  by the controller so that no information is lost or not exposed to the user.

### Owner-based Struct Tree

Related to https://github.com/crossplane/crossplane/issues/728

The provider API can be assumed as the owner of its configuration and output
fields since the decisions around these fields are left to them. It makes sense
to separate those fields from Crossplane or Kubernetes-related fields for a few
reasons:
* Prevent name collisions (or too much similarity) as the decision-maker of
  provider API's naming is different than other fields' decision maker.
* A better representation of what goes to/comes from provider and what's
  Crossplane or Kubernetes specific.
* Let the user know what configuration is being sent and received as much as
  possible in the provider's API language.

A good separation method can be having provider fields as a sub-struct of `Spec`
and `Status` instead of directly embedding them.

Note that, in all cases, we should be able to guarantee that `spec.forProvider`
is the last representation of the things that we'll include in our CRUD calls
after all override/default/dependency operations are done. Also
`status.atProvider` represents the most up-to-date state in the provider as raw
as possible. In other words, when user sees an error, they should be able to
tell what exact configuration was used that caused that error by looking at
`spec.forProvider` _and_ they should be able to tell whatever operation is done
`status.atProvider` is the current state.

An actual example:

Embedding:

```yaml
apiVersion: compute.gcp.crossplane.io/v1alpha1
kind: Network
metadata:\
  name: my-legacy-network
  selfLink: /apis/compute.gcp.crossplane.io/v1alpha1/networks/my-legacy-network
spec:
  name: monus-legacy-network-res
  providerConfigRef:
    name: monus-gcp
  reclaimPolicy: Delete
  routingConfig:
    routingMode: REGIONAL
status:
  IPv4Range: 10.240.0.0/16
  conditions:
  - lastTransitionTime: "2019-08-29T05:55:16Z"
    reason: Successfully reconciled managed resource
    status: "True"
    type: Synced
  creationTimestamp: "2019-08-28T10:45:27.445-07:00"
  gatewayIPv4: 10.240.0.1
  id: 75666386684937048
  kind: compute#network
  name: monus-legacy-network-res
  routingConfig:
    routingMode: REGIONAL
  selfLink: https://www.googleapis.com/compute/v1/projects/crossplane-playground/global/networks/monus-legacy-network-res
```

Sub-struct:

```yaml
apiVersion: compute.gcp.crossplane.io/v1alpha1
kind: Network
metadata:
  name: my-legacy-network
  selfLink: /apis/compute.gcp.crossplane.io/v1alpha1/networks/my-legacy-network
spec:
  providerConfigRef:
    name: monus-gcp
  reclaimPolicy: Delete
  writeConnectionSecretToRef: {}
  forProvider:
    name: monus-legacy-network-res
    routingConfig:
      routingMode: REGIONAL
status:
  conditions:
  - lastTransitionTime: "2019-08-29T05:55:16Z"
    reason: Successfully reconciled managed resource
    status: "True"
    type: Synced
  atProvider:
    IPv4Range: 10.240.0.0/16
    creationTimestamp: "2019-08-28T10:45:27.445-07:00"
    gatewayIPv4: 10.240.0.1
    id: 75666386684937048
    kind: compute#network
    name: monus-legacy-network-res
    routingConfig:
      routingMode: REGIONAL
    selfLink: https://www.googleapis.com/compute/v1/projects/crossplane-playground/global/networks/monus-legacy-network-res
```

### Pointer Types and Markers

Related to https://github.com/crossplane/crossplane/issues/741

For any field, the developer can choose whether to use pointer type or value
type, i.e. `*bool` vs `bool`. The main difference here is their zero-value,
which gets used when the field is left empty. We follow Kubernetes conventions
for this decision:
https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#optional-vs-required

Short summary:
* Optional fields are pointer type -> `*AwesomeType` and marked as optional via
  a comment marker `//+optional` and `omitempty` struct tag.
* Required fields are exact opposite, i.e. not pointer type, not marked as
  `//+optional` and do not have `omitempty` struct tag.

Here is the flow to decide how to make the decision for `Spec` fields:
1. Start with assuming all fields are optional.
2. Convert the fields to required if any of CRUD calls you make requires them,
   i.e. it should be possible to call all CRUD operations with all those
   required fields filled.

There is an edge case where `FieldA` is required but there is also `FieldARef`,
which means in the runtime that reference will be resolved and `FieldA` will be
populated. If there is a guarantee that no call will be made to the provider
until `FieldA` is populated (at the time of this writing, there is), you need to
add `omitempty` as struct tag so that Kubernetes allows the creation of the
resource with only `FieldARef` populated.

The decision flow for `Status` fields are different. The values for those fields
are provided by the provider and overridden by the latest observation no matter
what and we know that we'll get a full object body. However;
* In error cases we don't want to show Go zero-values since this would be
  misleading about the current status. But in cases where the value is actually
  Go zero-value, we should not omit it. So, using `omitempty` makes sense.
* `// +optional` doesn't really make sense since we always get a full object
  body.
* Pointer type should be used only if the corresponding field is pointer type in
  the provider's SDK type.

Note that some required fields by all CRUD calls might be late-initialized. For
example, a config parameter can only be fetched from the provider and CRUD
operations except `Create` requires it. In those cases, mark the field as
optional since at any step we may not have it.

> By pointer types, developer tries to show that the value of the field may not
exist and zero-value defaulting does not make sense for that type.

#### Example

```
type GlobalAddressParameters struct {
  ...

  // IPVersion: The IP version that will be used by this address. Valid
  // options are IPV4 or IPV6.
  //
  // Possible values:
  //   "IPV4"
  //   "IPV6"
  //   "UNSPECIFIED_VERSION"
  // +optional
  IPVersion *string `json:"ipVersion,omitempty"`

  // Name of the resource. The name must be 1-63 characters long, and comply
  // with RFC1035. Specifically, the name must be 1-63 characters long and
  // match the regular expression `[a-z]([-a-z0-9]*[a-z0-9])?`. The first
  // character must be a lowercase letter, and all following characters
  // (except for the last character) must be a dash, lowercase letter, or
  // digit. The last character must be a lowercase letter or digit.
  Name string `json:"name"`

  // Network: The URL of the network in which to reserve the address. This
  // field can only be used with INTERNAL type with the VPC_PEERING
  // purpose.
  // +optional
  Network *string `json:"network,omitempty"`
  
  ...
}
```

#### Optional Embedded Structs with One Field

Some provider APIs include an embedded struct that is optional and contains only
a single field. If this field is configurable, the embedded struct should be
part of `spec.forProvider`. However, if the struct itself is optional, the
single field it contains should always be **required**, *even if the provider
marks it as optional*. For example:

```go
// This is the provider's representation of the API object
type ProviderAPIObject struct {
  // Configurable field in top-level object
  FieldOne *string `json:"fieldOne,omitempty"`
  
  // Embedded struct in top-level object
  EmbeddedStructOne *EmbeddedStruct `json:"embeddedStructOne,omitempty"`
}

type EmbeddedStruct struct {
  // This field is configurable so it should be in spec.forProvider
  SomeConfigurableField *string `json:"someConfigurableField,omitempty"`
}
```

The corresponding Crossplane representation of the object's `Spec` should look
as follows:

```go
// This is the Crossplane representation of the API object spec
type CrossplaneAPIObjectSpec struct {
  // Configurable field in top-level object
  // +optional
  FieldOne *string `json:"fieldOne,omitempty"`
  
  // Embedded struct in top-level object
  // +optional
  EmbeddedStructOne *EmbeddedStructSpec `json:"embeddedStructOne,omitempty"`
}

// Only the configurable fields in EmbeddedStruct
type EmbeddedStructSpec struct {
  // This field is configurable so it should be in spec.forProvider
  // It is required because its parent is optional and it is the only field of its parent
  SomeConfigurableField string `json:"someConfigurableField"`
}
```

The reasoning for designing in this manner is to avoid the presence of
meaningless entries in `YAML` configuration. If we *did not* do so, the
following configuration would be valid:

```yaml
...
spec:
  providerConfigRef:
    name: provider-gcp
  reclaimPolicy: Delete
  writeConnectionSecretToRef: {}
  forProvider:
    embeddedStructOne: # valid if we do not require embedded field
    fieldOne: my-cool-field
```

### Immutable Properties

Related to https://github.com/crossplane/crossplane/issues/727

Some of the fields that include in `Spec` can only be configured in creation
call of the resource, later you cannot update them. However, Kubernetes Custom
Resources validation mechanisms do not yet support that behavior, see
https://github.com/kubernetes/enhancements/pull/1099 for details. Until that KEP
lands, we recommend using the following marker for the fields that are deemed to
be immutable once set:
```
//+immutable
```

Note that there are many fields that are immutable and can be populated by a
reference, which could be resolved via its label selector. In such cases, the
raw config value, and the reference to the resource that the value will be fetched
from are immutable. But since the selector is only a set of instructions to find
a reference, it should not be marked as immutable.

There are some solutions like admission webhooks to enforce immutability of some
of the fields, however, current behavior is that Crossplane shows the user what
the error is received when the call that includes a change in the immutable
property is made and the most up-to-date status under `Status`. However, there
are scenarios where `Update` call of the provider doesn't include the immutable
fields at all. In that case, there will be no error to show.

#### Example

```

type SubnetworkParameters struct {
  // Name: The name of the resource...
  // +immutable
  Name string `json:"name"`

  // Network: The URL of the network to which this subnetwork belongs,
  // provided by the client when initially creating the subnetwork. Only
  // networks that are in the distributed mode can have subnetworks. This
  // field can be set only at resource creation time.
  // +immutable
  Network string `json:"network"`

  // Region: URL of the region where the Subnetwork resides. This field
  // can be set only at resource creation time.
  // +optional
  // +immutable
  Region string `json:"region,omitempty"`

  // SecondaryIPRanges: An array of configurations for secondary IP ranges
  // for VM instances contained in this subnetwork. The primary IP of such
  // VM must belong to the primary ipCidrRange of the subnetwork. The
  // alias IPs may belong to either primary or secondary ranges. This
  // field can be updated with a patch request.
  // +optional
  SecondaryIPRanges []*GCPSubnetworkSecondaryRange `json:"secondaryIpRanges,omitempty"`
}
```

### Pause Annotation
Managed resources, which are reconciled with the [managed reconciler], support
a special annotation named `crossplane.io/paused`. If a managed resource has
this annotation with the value `true` such as the following resource:
```yaml
apiVersion: ec2.aws.upbound.io/v1beta1
kind: VPC
metadata:
  name: paused-vpc
  annotations:
    crossplane.io/paused: "true"
...
```
, then further reconciliations on the managed resource will be paused after
emitting an event with the type `Synced`, the status `False`,
and the reason `ReconcilePaused`. Reconciliations will resume when
this annotation is removed, or set to some other value than `true`.

## Future Considerations

### Spec and Observation Drifts

In cases where user changes the `Spec` and for some reason we could not update
the external resource. Most of the time, the reconciliation will result in an
error but that's not guaranteed. For example, `Update` call might be picking
only the values that are update-able and if you changed an immutable one, there
will be no error to let you know that we are out of sync. So, in `Observe` call
of the controller there should be a comparison of `Spec` fields and the received
object's body. In case it's not in sync, we should indicate that through a
`Condition`.

Generic managed reconciler's `ExternalObservation` struct could be extended by
adding a field about that sync status and reconciler can mark the sync status in
one of the `Condition`s we already have or add a new one.

[package]: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/eks
[terminology]: https://github.com/crossplane/crossplane/blob/master/docs/concepts/terminology.md
[from crossplane-runtime]: https://github.com/crossplane/crossplane-runtime/blob/ca4b6b4/apis/core/v1alpha1/resource.go#L77
[Kubernetes API Conventions - Spec and Status]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
[managed reconciler]: https://github.com/crossplane/crossplane-runtime/blob/84e629b9589852df1322ff1eae4c6e7639cf6e99/pkg/reconciler/managed/reconciler.go#L637

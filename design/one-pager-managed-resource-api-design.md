# Managed Resources API Patterns
* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Terminology

* _External resource_. An actual resource that exists outside Kubernetes, typically in the cloud. AWS RDS or GCP Cloud
  Memorystore instances are external resources.
* _Provider_. The entity that owns the physical layer of the external resource. AWS, GCP and Azure are examples of this.
* _Managed resource_. The Crossplane representation of an external resource. The `RDSInstance` and `CloudMemorystoreInstance`
  kinds are managed resources.
* _Claim_. The Crossplane representation of a request for the allocation of a managed resource. Resource claims
  typically represent the need for a managed resource that implements a particular protocol. MySQLInstance and
  RedisCluster are examples of resource claims.
* _Provider_. Cloud provider such as GCP, AWS, Azure offering IaaS, cloud networking, and managed services.
* _Spec_. The sub-resource of Kubernetes resources that represents the desired state of the user.
* _Status_ The sub-resource of Kubernetes resources that represents the most up-to-date information about the resource.

For other terms, see [glossary].

## Background
Crossplane extends Kubernetes heavily and the maintainers always try to adopt the patterns that are already seen in 
Kubernetes resources, notably declarative style API. However, it's not always intuitive for everyone to see what goes to
where in a way that is consistent and just as expected by the consumers of those resources.

This document tries to capture how the bridge between Crossplane and provider is shaped. So, the features of managed
resources that are generic to all of them are not included here, for example reclaim policies. This is more about the
things that a developer should keep in mind when they try to implement a new managed resource.

A few questions that this document needs to help answering:
- What goes into `Spec` and what goes into `Status` of a managed resource?
- Should _all_ information from the provider be represented in the Custom Resource?
- What parts of the Custom Resource should be allowed to be consumed by the consumers?
- Do we represent _desired_ state, _observed_ state or both?
- How should we differentiate the fields that are used by provider from Crossplane's fields?

While this document is heavily opinionated, each provider can wildly vary between each other. So, treat this document as
a starting point, try to stick to it for consistency but deviate if required and open an issue about why you had to
deviate so that we can reassess and improve it.

## Guiding Principles

Our users are the stakeholders on how we make decisions. There are two types of users for managed resources:
* Input Users: Users who configure/create/manage the resource. Crossplane Claim Controller, Cluster Admin are a few
  examples of this type. We want our `Spec` to be sole place for them to configure everything about the resource.
* Output Users: Users who refer to/query/use/extract information from the managed resource. We want them to be able to
  perform all read-only operations on the given resource as if they have the cloud provider's API available. This can
  include configurations and metadata information about the resource. We also want them to access only `Status`
  sub-resource of the managed resource because this is where the most up-to-date information about the resource is
  stored and we are able to limit RBAC of these users to access only `Status`.
  
### Naming Conventions

There are two naming decisions here to make:
* Managed resource CR name
* Identifier string of the external resource in the cloud provider.

_Name_ in this context means the identifier string for the resource in its environment. CR name is the identifier
for resource in Kubernetes environment. But for external resources, this can be either `id`, `uid` or `name` properties
of the resource. The one we want to name is the one that is used when _referring to_ that resource in the same
environment. For example, in AWS, VPCs do not have a name but `VpcID` properties that is used for identification.

#### Custom Resource Instance Name

The name of the managed resource CR, which lives in our control cluster. The claim controller who creates the managed
resource gives its name or some other entity might create the managed resource, like user themselves. So, do not assume
that all the managed resources will have the same naming format anywhere.
One enforcing condition is that it has to be unique amongst other instances of the CRD. The best way to guarantee that
is to include Kubernetes UID in the name. The following format is valid as long as the length of the name is not longer
than 253 characters. Given that we know UID is 36 characters long, name for the service you choose should be
differentiable enough with 217 characters.

For example, the following would be a good name for a CloudSQL managed resource CR:
```
cloudsql-7cb46d78-4556-4904-93d2-b34f6d5ccadf
```

#### External Resource Name

Related to https://github.com/crossplaneio/crossplane/issues/624

The decision for an external resource name is made by the controller of that managed resource in `Create` phase. The
following is the possible cases:
* The provider doesn't allow us to specify a name. Fetch it after its client's `POST` call.
* The provider allows to specify a name.
  1. Check whether CR name is suitable as resource's external name. If so, use it. If not;
  2. Use `servicename-k8s UID of CR`, i.e. `cloudsql-7cb46d78-4556-4904-93d2-b34f6d5ccadf`. If that doesn't comply;
  3. Generate your own name in the following format in a compliant way:
```
servicename-<random-string>
<random-string>: lower case letters, upper case letters, integers and dash (-). Dash (-) should be neither in the
beginning nor in the end. Until 36 characters(k8s UID length), the longer random string the better.
```
  4. If 3 doesn't work for you, come up with your own format that includes the name of the service if possible.

Note that the managed resource might be created manually by the user, so, the CR name can be anything and you need to
have a validation step for the CR name before you use it.

In all cases, write the name to annotation with key `crossplane.io/external-name` if a value does not already exist. Use 
the value of that annotation as external resource name in _all_ queries.

#### Field Names

The thinking process of naming decision could respect provider's decision as the starting point and deviate from that
if there is a very good reason to. Naming parity makes inter-resource references easier and lowers the entry barrier for
both developers and users. This is especially handy in cases where Crossplane does not yet support the _referred_ resource
or user may not want some of their resources in their Crossplane environment.

Ideally, for the resources that do have Crossplane representations, you can append `Ref` to the field name,
i.e. `network (string)` -> `networkRef (Network)`

In some cases, we might have collisions or cases where provider field name is too similar to another field in the
managed resource's Custom Resource fields. The section [Owner-based Struct Tree](#owner-based-struct-tree) tries to
tackle that.

#### Example

```yaml

---
# example-network will be the VPC that all cloud instances we'll create will use.
apiVersion: compute.gcp.crossplane.io/v1alpha2
kind: Network
metadata:
  name: network-7cb46d78-4556-4904-93d2-b34f6d5ccadf
  namespace: gcp
  annotations:
    crossplane.io/external-name: "network-7cb46d78-4556-4904-93d2-b34f6d5ccadf"
spec:
  ...
```

```yaml

---
# example-network will be the VPC that all cloud instances we'll create will use.
apiVersion: compute.gcp.crossplane.io/v1alpha2
kind: Network
metadata:
  name: <too-long-string>
  namespace: gcp
  annotations:
    crossplane.io/external-name: "network-123-my-random-gen-id"
spec:
  ...
```

### High Fidelity

Related to https://github.com/crossplaneio/crossplane/issues/621 and https://github.com/crossplaneio/crossplane/issues/530

Crossplane managed resources should expose everything that provider exposes to its users as much as possible. A few 
benefits of that high fidelity:

* The ability to read and set every 'knob' the external resource supports. This would make it easy to support wide 
variety of customizations without having to think of them in development time.
* An obvious mapping of Crossplane's fields to cloud provider API fields, exact name match as much as possible. This 
would make it easier to work with Crossplane for users who are familiar with the given provider in many ways 
including easier troubleshooting and cooperation with other existing infrastructure tools.

Ideally, what we want is to expose all possible configurations in `Spec` for input users and whatever might be needed
 by the output users of this resource in the `Status`. Generally, it's better to just copy all fields from the
 provider's API as starting point and eliminate them one by one if needed.

What goes into `Spec`:
* Does the provider allow configuration of this parameter at _any_ point in the life cycle of the resource? Include if yes.
  This includes the fields that are late-initialized, meaning some fields could be populated by the provider when you
  actually create the resource but it also gives you the ability to change them. An example for this could be
  auto-generated resource tags or resource-specific defaults. If the provider tags the resource without us telling them to
  do so, controller should update `Spec` and input user should make changes on that current value of the field.
  Related to https://github.com/crossplaneio/crossplane-runtime/issues/10
* Is this field represented as standalone managed resource? Do not include if the answer is yes.
  We should not manage an external resource in the CR other than its original external resource. For example, Azure
  VirtualNetwork object allows you to create Subnets by providing an array of Subnet objects as one of its fields. However,
  Subnet is already another managed resource supported by Crossplane. In that case, we should not allow configuration of
  Subnets through VirtualNetwork CR but require people to do it via Subnet CR. Though you might refer to it for
  configuration purposes, having two controllers managing one resource (VirtualNetwork CR and Subnet CR controllers) would
  not work well. So, do not include it in the fields and be careful during your provider calls not to affect the other
  resource.
  
  What if the sub-resource is not yet supported as managed resource in Crossplane? In that case, you should first
  consider implementing that managed resource, if not suitable, only then include it in the CR.

Note that you should make updates to `Spec` fields that are empty. We do not override user's desired state and if they
have no control over it in any case, do not include it in `Spec`.

What goes into `Status`:
* Does the provider give the information of that field on the first level of the query, `GET`? Include if yes.
  For the subresource that are represented as standalone managed resource by Crossplane, include only identification
  information instead of full object representation. For example, if received Azure VirtualNetwork object has an array
  of Subnet objects (which is represented as standalone), represent each one by their name in a string array.

### Owner-based Struct Tree

Related to https://github.com/crossplaneio/crossplane/issues/728

The provider API can be assumed as the owner of its configuration and output fields since the decisions around these
fields are left to them. It makes sense to separate those fields from Crossplane or Kubernetes-related fields for a few
reasons:
* Prevent name collisions (or too much similarity) as the decision-maker of provider API's naming is different than
  other fields' decision maker.
* A better representation of what goes to/comes from provider and what's Crossplane or Kubernetes specific.
* Let the user know what configuration is being sent and received as much as possible in the provider's API language.

A good separation method can be having provider fields as a sub-struct of `Spec` and `Status` instead of directly
embedding them.

Note that, in all cases, we should be able to guarantee that `spec.forProvider` is the last representation of the things
that we'll include in our CRUD calls after all override/default/dependency operations are done. Also `status.atProvider`
represents the most up-to-date state in the provider as raw as possible. In other words, when user sees an error, they
should be able to tell what exact configuration was used that caused that error by looking at `spec.forProvider` _and_
they should be able to tell whatever operation is done `status.atProvider` is the current state.

An actual example:

Embedding:

```yaml
apiVersion: compute.gcp.crossplane.io/v1alpha1
kind: Network
metadata:\
  name: my-legacy-network
  namespace: crossplane-system
  selfLink: /apis/compute.gcp.crossplane.io/v1alpha1/namespaces/crossplane-system/networks/my-legacy-network
spec:
  name: monus-legacy-network-res
  providerRef:
    name: monus-gcp
    namespace: crossplane-system
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
  namespace: crossplane-system
  selfLink: /apis/compute.gcp.crossplane.io/v1alpha1/namespaces/crossplane-system/networks/my-legacy-network
spec:
  providerRef:
    name: monus-gcp
    namespace: crossplane-system
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

Related to https://github.com/crossplaneio/crossplane/issues/741

For any field, the developer can choose whether to use pointer type or value type, i.e. `*bool` vs `bool`. The main
difference here is their zero-value, which gets used when the field is left empty. We follow Kubernetes conventions
for this decision: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#optional-vs-required

Short summary:
* Optional fields are pointer type -> `*AwesomeType` and marked as optional via a comment marker `//+optional` and
`omitempty` struct tag.
* Required fields are exact opposite, i.e. not pointer type, not marked as `//+optional` and do not have `omitempty`
struct tag.

Here is the flow to decide how to make the decision for `Spec` fields:
1. Start with assuming all fields are optional.
2. Convert the fields to required if any of CRUD calls you make requires them, i.e. it should be possible to call all
CRUD operations with all those required fields filled.

The decision flow for `Status` fields are different. The values for those fields are provided by the provider and
overridden by the latest observation no matter what. So, it's not really important whether the fields are marked optional
or not but for the sake of consistency we can treat all fields that are populated by the observation from provider
as optional.

Note that some required fields by all CRUD calls might be late-initalized. For example, a config parameter can only be
fetched from the provider and CRUD operations except `Create` requires it. In those cases, mark the field as optional
since at any step we may not have it.

> By pointer types, developer tries to show that the value of the field may not exist and zero-value defaulting does not
make sense for that type.

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

### Immutable Properties

Related to https://github.com/crossplaneio/crossplane/issues/727

Some of the fields that include in `Spec` can only be configured in creation call of the resource, later you cannot
update them. However, Kubernetes Custom Resources validation mechanisms do not yet support that behavior, see https://github.com/kubernetes/enhancements/pull/1099
for details. Until that KEP lands, we recommend using the following marker for the fields that are deemed to be immutable
once set:
```
//+immutable
```

There are some solutions like admission webhooks to enforce immutability of some of the fields, however, current behavior
is that Crossplane shows the user what the error is received when the call that includes a change in the immutable
property is made and the most up-to-date status under `Status`. However, there are scenarios where `Update` call of the
provider doesn't include the immutable fields at all. In that case, there will be no error to show.

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

[glossary]: https://github.com/crossplaneio/crossplane/blob/master/docs/concepts.md#glossary
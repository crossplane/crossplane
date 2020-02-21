---
title: v1beta1 Checklist
toc: true
weight: 740
indent: true
---
# v1beta1 Checklist

Crossplane adheres to the [Kubernetes definition] of API versioning. This means
that when an API type reaches a `beta` level of quality it should be
well-tested, enabled by default, and will be supported through subsequent
releases, even if the details of the type change. As a community, we implement
`beta` API types when the following are true:

- An `alpha` version of the API types currently exists.
- We are committed to long-term support the API type.
- There is demonstrated interest for support of the API type in the end-user
  community.

When the above are all true, the first version of a `beta` level API type,
`v1beta1`, should be implemented in accordance with the specifications described
in the remainder of this document.

## Areas of Quality

Crossplane controllers are comprised of four main components: the cloud provider
API type, the Crossplane API type, the client, and the controller. Each of these
components serves a specific purpose that should not overlap with any of the
other components. It is helpful to conceptualize these components in the
following manner:

- **Cloud Provider API Type**: the language the controller speaks to manage the
  cloud provider resource
- **Crossplane API Type**: the language the controller speaks to manage the
  Crossplane resource (Kubernetes [CustomResource])
- **Client**: the translator between the Crossplane API type and the cloud
  provider API type
- **Controller**: the actor that speaks to both Kubernetes and the cloud
  provider

Typically, the components are implemented in the following order:

1. Cloud Provider API Type: defined outside of the Crossplane / Kubernetes
   ecosystem and must be adhered to without modification. Usually imported into
   the controller by a cloud provider's Go SDK.
2. Crossplane API Type: defined as nested Go structs, which are then compiled
   into Kubernetes CustomResourceDefinitions using [code generation]. Follows
   the conventions of [Kubernetes API design specifications] and the Crossplane
   [API patterns design doc].
3. Client: a set of functions that translate the Crossplane API type to the
   cloud provider API type. Handles operations such as late initialization and
   comparison between the two APIs.
4. Controller: uses [`crossplane-runtime`] library to provide custom CRUD
   operations to a shared reconciler that is used across all Crossplane
   API types. Calls upon the client to translate the objects it receives from
   the cloud provider API and the Kubernetes API server.

## Cloud Provider API Type

As mentioned previously, the cloud provider API type is external to Crossplane
and cannot be modified. However, it will inform how all other components are
implemented, so it is important to have a strong understanding of both the
fields it exposes and how they are used. Many cloud provider's will use default
values for omitted fields that may not be intuitive or equal to the [zero-value]
of the Go type they are implemented in by the SDK. It is usually not enough to
just read the code exposed by the API. Authors of `beta` Crossplane APIs should
be familiar with how the cloud provider responds to and modifies each field
either by reading additional documentation or testing different configurations
directly against the API.

## Crossplane API Type

The Crossplane API type is a Kubernetes representation of the cloud provider
API. All `beta` Crossplane API types should be a [high fidelity] representation
of the cloud provider API type. This means that any functionality that can be
achieved using the cloud provider API should also be possible by creating the
Crossplane representation of that API type. For full detail about how to design
a Crossplane API type, please see our [API patterns design doc].

- [ ] Any fields related to naming the external resource have been removed from
  `spec.ForProvider`. They should be handled by the `crossplane-runtime`
  [external name annotation].
- [ ] All fields that can be configured at any point in the lifetime of the
  external resource have been added to the `spec.forProvider` struct of the
  Crossplane API type.
- [ ] All fields that involve creating another external resource type that can
  also be represented by a [separate Crossplane API] type have been removed from
  `spec.ForProvider`.
- [ ] All fields that cannot be configured at any point in the lifetime of the
  external resource have been added to the `status.atProvider` struct of the
  Crossplane API type.
- [ ] There is a code comment on every field that is descriptive of its
  functionality.
- [ ] All *optional* fields in `spec.forProvider` have been denoted with a `//
  +optional` tag and are a pointer type.
- [ ] All fields in `spec.forProvider` that cannot be modified after assigned a
  value have been denoted with a `// +immutable` tag.
- [ ] An additional `_____Ref` field exists for any field that is used to
  reference another external resource for which there is currently a Crossplane
  API type representation. The necessary [attribute referencer] methods have
  also been added in accordance with the Crossplane [cross-resource reference
  design document].
- [ ] Any fields that require [sensitive input] (e.g. password for a database)
  have been replaced with a `_____SecretRef` field in accordance with the API
  patterns design doc.

In addition to this checklist, it is recommended that an author also looks at
other `v1beta1` resources across the Crossplane ecosystem to ensure that their
design decisions are in accordance with existing patterns. The GCP
[CloudSQLInstance] resource is a good example.

## Client

## Controller

[Kubernetes definition]: https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning
[CustomResource]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[code generation]: https://book.kubebuilder.io/reference/generating-crd.html
[Kubernetes API design specifications]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
[`crossplane-runtime`]: https://github.com/crossplane/crossplane-runtime
[zero-value]: https://golang.org/ref/spec#The_zero_value
[high fidelity]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-managed-resource-api-design.md#high-fidelity
[API patterns design doc]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-managed-resource-api-design.md
[external name annotation]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-managed-resource-api-design.md#external-resource-name
[attribute referencer]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md#cross-referencing-using-attribute-referencers
[cross-resource reference design document]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
[sensitive input]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-managed-resource-api-design.md#sensitive-input-fields
[CloudSQLInstance]: https://github.com/crossplane/stack-gcp/blob/master/apis/database/v1beta1/cloudsql_instance_types.go
[separate Crossplane API]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-managed-resource-api-design.md#high-fidelity
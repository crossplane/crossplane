# Resource Claim and Binding Definition
* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

See the following issues this design doc intends to address:

* https://github.com/crossplaneio/crossplane/issues/1105
* https://github.com/crossplaneio/crossplane/issues/1106
* https://github.com/crossplaneio/crossplane/issues/1107

TODO(negz): Write this up.

## Goals

* Allow infrastructure operators to define claim kinds.
* Allow a claim to be satisfied by multiple resources.
* Surface claim to resource compatibility in Crossplane's API.

TODO(negz): Write this up.

## Proposal

This document proposes the introduction of two new, cluster scoped, resources
named `ResourceClaimDefinition` and `ResourceBindingDefinition`.

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClaimDefinition
metadata:
  name: mysqlinstance
spec:
  group: database.crossplane.io
  names:
    kind: MySQLInstance
    listKind: MySQLInstanceList
    plural: mysqlinstances
    singular: mysqlinstance
  version: v1alpha1
  validation:
    openAPIV3Schema:
      description: A MySQLInstance is a portable resource claim that may be satisfied
        by binding to a MySQL managed resource such as an AWS RDS instance or a GCP
        CloudSQL instance.
      properties:
        spec:
          properties:
            engineVersion:
              description: EngineVersion specifies the desired MySQL engine version,
                e.g. 5.7.
              enum:
              - "5.6"
              - "5.7"
              type: string
```

```yaml
---
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceBindingDefinition
metadata:
  name: mysqlinstance-cloudsqlinstance
spec:
  claim:
    apiVersion: database.crossplane.io/v1alpha1
    kind: MySQLInstance
  resources:
  - resource:
      apiVersion: database.gcp.crossplane.io/v1beta1
      kind: CloudSQLInstance
    class:
      apiVersion: database.gcp.crossplane.io/v1beta1
      kind: CloudSQLInstanceClass
      strategies:
        provisioning:
        - overlay
        - translate
        overlay:
        - fromClassField: ".specTemplate"
          toResourceField: ".spec"
        translate:
        - fromClaimField: ".spec.engineVersion"
          toResourceField: ".spec.forProvider.databaseVersion"
          transforms:
            toupper: true
            replace:
              string: "."
              with: "_"
---
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceBindingDefinition
metadata:
  name: mysqlinstance-cloudsqlinstance
spec:
  claim:
    apiVersion: database.crossplane.io/v1alpha1
    kind: MySQLInstance
  resources:
  - resource:
      apiVersion: database.azure.crossplane.io/v1beta1
      kind: MySQLServer
    class:
      apiVersion: database.azure.crossplane.io/v1beta1
      kind: SQLServerClass
      strategies:
        selection:
        - match
        match:
        - fromClaimField: ".spec.engineVersion"
          toClassField: ".specTemplate.forProvider.version"
          transforms:
            toupper: true
  - resource:
      apiVersion: database.azure.crossplane.io/v1alpha3
      kind: MySQLServerVirtualNetworkRule
    class:
      apiVersion: database.azure.crossplane.io/v1alpha3
      kind: MySQLServerVirtualNetworkRuleClass
```

`ResourceClaimDefinition` is an abstraction that provides guard rails and
opinions around the process of authoring a `CustomResourceDefinition` that is
intended for use as a resource claim. Crossplane resource claims are of many
different kinds, but all have certain things in common. These include:

* Declaring use of the status subresource.
* Supporting standard Kubernetes object and type metadata.
* Supporting a binding phase and conditioned status in their `.status`.
* Supporting a class selector, class reference, resource reference, and
  connection secret reference in their `.spec`.
* Supporting a standard set of kubectl printer columns.

A `ResourceClaimDefinition` allows an infrastructure operator to define a
resource claim without having to specify all of these things correctly. They can
instead focus on what they're concerned with - defining the purpose specific
`.spec` fields of their claim. A `ResourceClaimDefinition` controller will watch
for `ResourceClaimDefinition` instances, filling in the blanks to create a
corresponding `CustomResourceDefinition`.

`ResourceBindingDefinition` maps a kind of resource claim to the kinds of
managed resource (and class) that may satisfy it. In addition to declaring the
relationship between a claim kind and one or more managed resource kinds, a
`ResourceBindingDefinition` defines the _strategies_ that should be used when
dynamically provisioning a set of managed resources to satisfy a claim. There
are two types of strategy:

* _Selection strategies_ constrain which resource classes can satisfy a
  resource claim, for example by requiring that a particular claim field match
  a particular resource class field, perhaps after a transform is applied.
* _Provisioning strategies_ configure how dynamic provisioning is handled, i.e.
  how to form a managed resource from a claim and a class. One example might be
  using the resource class's `.specTemplate` as an overlay on the managed
  resource's `.spec`.

Random notes:

* It's possible to use `ResourceBindingDefinition` without using
  `ResourceClaimDefinition`.
* Creating a `ResourceBindingDefinition` spins up claim scheduling, defaulting,
  binding, etc controllers for the resource claim kind it is concerned with. I'm
  about 80% sure it's possible to build controllers using this RCD that operate
  on unstructured.Unstructured.
* This proposal implies a claim can bind to more than one managed resource, and
  can thus reference more than one class for dynamic provisioning. We know how
  many resources must be bound for the claim to be ready thanks to the RCD.
* Dynamic provisioning would occur in the order the class kinds were specified
  in the schema. Classes may use affinity / anti-affinity labels so that e.g. a
  compatible vnet rule class will be used for the chosen MySQLServer class.
* If a resource claim needs three resources of a particular kind to be satisfied
  you must repeat that resource (and its class) three times in its resource
  group. We could instead add a 'count' field but that would require three
  identically configured resources are dynamically provisioned.
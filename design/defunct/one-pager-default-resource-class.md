# Default Resource Classes in Crossplane
* Owner: Daniel Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Defunct

The Crossplane ecosystem exposes the concepts of [Resource Classes and Resource
Claims](https://crossplane.io/docs/v0.2/concepts.html). Classes serve to define
the configuration for a certain underlying resource, which may be anything from
a managed cloud provider service to a traditional server. Claims are requests to
create an instance of a resource and currently reference a specific deployed
Class in order to abstract the implementation details. This document serves to
illustrate Crossplane's design iterations on resource class defaulting and
portability, with the ultimate goal of supporting multiple classes of service
that can be consumed in a portable manner across cloud providers. 

## Revisions

* 2.0
  * Updating document to reflect the current method of defaulting using portable
    resource classes
  * Introducing portable classes as a class of service definition

## Goals

* Allow for claim portability by using portable classes, much how the generic
  `ResourceClass` operated
* Enable claims with no class reference 

## Non-Goals

- Default resource classes do not aim to lock a developer into a certain
  underlying resource class, but simply allow them the opportunity to default to
  whatever has been deemed an acceptable option for the resource they desire

## Background

### Original State

Originally, resource claims had to explicitly declare the underlying
resource class that they want to inherit the configuration from on deployment.
For example, the following resource class could be declared for a Postgres RDS
database instance on AWS:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: cloud-postgresql
  namespace: crossplane-system
parameters:
  class: db.t2.small
  masterUsername: masteruser
  securityGroups: "sg-ab1cdefg,sg-05adsfkaj1ksdjak"
  size: "20"
provisioner: rdsinstance.database.aws.crossplane.io/v1alpha1
providerRef:
  name: aws-provider
reclaimPolicy: Delete
```

This class would likely be created by an operator as a type of database that
developers may deploy as part of their application. Originally, for a developer
to deploy an RDS instance on AWS, they would have to explicitly reference it:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: cloud-postgresql-claim
  namespace: demo
spec:
  classRef:
    name: cloud-postgresql
    namespace: crossplane-system
  engineVersion: "9.6"
```

This provided a nice separation of concerns for the developer and the operator,
but required the developer knowing about the `cloud-postgresql` class, and
likely having to examine some of the configuration details for it.

### Default Class Reference v0

While it remained possible to explicitly reference an underlying resource class,
the first iteration of default classes allowed developers to have the option to
omit the class reference and rely on falling back to whatever operators deemed
an appropriate default. The default resource class was distinguished via the
`{api}/default` label:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: cloud-postgresql
  namespace: crossplane-system
  labels:
    postgresqlinstance.database.crossplane.io/default: "true"
parameters:
  class: db.t2.small
  masterUsername: masteruser
  securityGroups: "sg-ab1cdefg,sg-05adsfkaj1ksdjak"
  size: "20"
provisioner: rdsinstance.database.aws.crossplane.io/v1alpha1
providerRef:
  name: aws-provider
reclaimPolicy: Delete
```

If a resource claim of type PostgreSQLInstance was then created without a class
reference, it would default to using this class:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: cloud-postgresql-claim
  namespace: demo
spec:
  engineVersion: "9.6"
```

Internally, Crossplane would first check to see if a resource class is
referenced. If not, it checked to see if a class annotated as `default` had been
created for the given kind. Ultimately, if one did not exist, it failed to
provision the resource.

### Default Class Reference v1

With the implementation of [strongly-typed resource
classes](./one-pager-strongly-typed-class.md), the generic `ResourceClass`
became obsolete and the `Policy` kind was introduced. Each claim kind had a
corresponding policy kind (e.g. `MySQLInstancePolicy` for `MySQLInstance`,
etc.). Claims could no longer simply omit a `classRef` because their controllers
would not know what `kind` they intended to bind to. While the `MySQLInstance`
claim controller previously knew to look for objects of type `ResourceClass`
that specified `mysqlinstance.database.crossplane.io/default:true`, there were
now many different class `kinds` that the claim could potentially reference
(e.g. GCP `CloudSQLInstanceClass`, AWS `RDSInstanceClass`, etc.). `Policies`
were introduced in order to allow the previously implemented defaulting behavior
to continue to exist. `Policies` were namespaced and would specify a specific
class instance by `group`, `version`, and `kind` for a claim to fall back on if
the `classRef` was omitted.

An administrator would create a strongly-typed class that would be suitable to
be referenced by a `MySQLInstance` claim:

```yaml
---
apiVersion: database.aws.crossplane.io/v1alpha1
kind: RDSInstanceClass
metadata:
  name: rdsmysql
  namespace: crossplane-system
specTemplate:
  class: db.t2.small
  masterUsername: masteruser
  securityGroups:
   - sg-ab1cdefg
   - sg-05adsfkaj1ksdjak
  size: 20
  engine: mysql
  providerRef:
    name: example
    namespace: crossplane-system
  reclaimPolicy: Delete
```

They would then create a new `namespace` (e.g. `my-app-namespace`), followed by
a `MySQLInstancePolicy` that referenced this class:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstancePolicy
metadata:
  name: mysql-policy
  namespace: my-app-namespace
defaultClassRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: standard-mysql
  namespace: crossplane-system
```

Then, for any `MySQLInstance` claim that was created in namespace
`my-app-namespace` without a `classRef`, the `MySQLInstance` default class
controller would automatically assign the class which was referenced by the
`MySQLInstancePolicy` in that namespace:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: my-app-namespace
spec:
  engineVersion: "9.6"
```

## Proposal: Default Class Reference v2 & Claim Portability

The `Policy` method continues to enable default class references, but
strongly-typed resource classes introduce a reduction in portability of resource
claims (i.e. the ability for claims to be used across providers). Previously,
claims could reference a generic `ResourceClass` by `name` and `namespace`, and
could be satisfied by a compatible managed resource regardless of cloud
provider. Now, claims must omit a `classRef` and rely on the existence of a
`Policy` to achieve portability.

In addition, because claims must reference resource classes using their full
group, version, and kind, this means that the creator of a claim forfeits the
ability to select a *generic* "class of service" (e.g. `mysql-large`). This is
an example of a `MySQLInstance` claim that references a strongly-typed
`RDSInstanceClass` resource class:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
spec:
  classRef:
    kind: RDSInstanceClass
    apiVersion: database.aws.crossplane.io/v1alpha1
    name: standard-mysql
    namespace: crossplane-system
  writeConnectionSecretToRef:
    name: rdsmysql
  engineVersion: "5.6"
```

To continue to provide the same level of portability for claims that was
originally present, two enhancements to the current model can be made:

1. Allow for multiple classes of service in a single namespace.
1. Denote the default class of service via label.

### Introducing Portable Classes

Instead of the currently used policy per claim model where a default class for a
`MySQLInstance` is dictated by a `MySQLInstancePolicy`, we propose
**deprecating** the `MySQLInstancePolicy` in favor of a `MySQLInstanceClass`.
Initially, the portable class will closely reflect the functionality of a policy
and will serve to define a "class of service" for a given claim `kind` by
referencing a strongly-typed resource class instance. However, in the future,
these portable classes may be expanded to define ranges and constraints for
portable claim kinds that reference them.

### Multiple Classes of Service Per Namespace

Currently, default class controllers, which act on claims that omit a
`classRef`, will fail to reconcile if multiple `Policies` exist within the
claim's namespace. For example, if two `MySQLInstancePolicies` are created in
namespace `crossplane-system`, then a `MySQLInstance` claim created in
`crossplane-system` that omits a `classRef` will not be assigned either one of
the classes referenced by the two `MySQLInstancePolicy` objects respectively.

In order to reintroduce the ability to select a class of service in claim, the
following steps can be taken:

1. Change all `Policy` types to `Class` types and include a `classRef` field
   instead of a `defaultClassRef` field. This would require updating the
   `crossplane-runtime` embeddable struct
   [`Policy`](https://github.com/crossplane/crossplane-runtime/blob/e4d61ee2805af680baf16fc2a1d8f79538d0f9bb/apis/core/v1alpha1/resource.go#L93),
   then bumping the dependency in core Crossplane. It would also require all
   embedded `Policy` structs to be renamed to `Class`.
1. Alter all claim controllers (which live in provider stacks) to accept a
   portable class `kind` in addition the claim, strongly typed class, and
   managed `kind` they currently accept. If a portable class `kind` is provided
   to the `NewClaimReconciler()` function, the shared claim reconciler should
   use the `classRef` of the claim to first obtain the portable class instance,
   and then use its `classRef` to get the strongly typed class instance. If no
   portable class is provided to the `NewClaimReconciler()` function, then it
   will assume the claim is referencing a strongly typed resource class and will
   use it directly (this functionality should not be used until the concept of a
   strongly typed claim is introduced). This will involve updating the logic of
   the [shared claim
   reconciler](https://github.com/crossplane/crossplane-runtime/blob/e4d61ee2805af680baf16fc2a1d8f79538d0f9bb/pkg/resource/claim_reconciler.go#L281)
   in `crossplane-runtime`. It should require minimal updates to the actual
   claim controllers in each of the provider stacks in order to indicate the
   portable class `kind` that they should use (example below).
1. Add a `HasPortableClassReferenceKind()` predicate in `crossplane-runtime`
   that accepts a portable class `GroupVersionKind` and a strongly typed class
   `GroupVersionKind`. Its logic should first check that the claim's `classRef`
   references the correct portable class `kind` by `name` and `namespace`, then
   should check that the portable class's `classRef` references the correct
   strongly typed class `kind`.

In this model, the MySQL `RDSInstance` claim reconciler would be updated from
its current state, which looks like this:

```go
// SetupWithManager adds a controller that reconciles MySQLInstance instance claims.
func (c *MySQLInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
  r := resource.NewClaimReconciler(mgr,
    resource.ClaimKind(databasev1alpha1.MySQLInstanceGroupVersionKind),
    resource.ClassKind(v1alpha1.RDSInstanceClassGroupVersionKind),
    resource.ManagedKind(v1alpha1.RDSInstanceGroupVersionKind),
    resource.WithManagedConfigurators(
      resource.ManagedConfiguratorFn(ConfigureMyRDSInstance),
      resource.NewObjectMetaConfigurator(mgr.GetScheme()),
    ))

  name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.MySQLInstanceKind, controllerName))

  return ctrl.NewControllerManagedBy(mgr).
    Named(name).
    Watches(&source.Kind{Type: &v1alpha1.RDSInstance{}}, &resource.EnqueueRequestForClaim{}).
    For(&databasev1alpha1.MySQLInstance{}).
    WithEventFilter(resource.NewPredicates(resource.HasClassReferenceKind(resource.ClassKind(v1alpha1.RDSInstanceClassGroupVersionKind)))).
    Complete(r)
}
```
To look as follows:

```go
// SetupWithManager adds a controller that reconciles MySQLInstance instance claims.
func (c *MySQLInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
  r := resource.NewClaimReconciler(mgr,
    resource.ClaimKind(databasev1alpha1.MySQLInstanceGroupVersionKind),
    resource.ClassKinds{Portable: databasev1alpha1.MySQLInstanceGroupVersionKind, NonPortable: v1alpha1.RDSInstanceClassGroupVersionKind},
    resource.ManagedKind(v1alpha1.RDSInstanceGroupVersionKind),
    resource.WithManagedConfigurators(
      resource.ManagedConfiguratorFn(ConfigureMyRDSInstance),
      resource.NewObjectMetaConfigurator(mgr.GetScheme()),
    ))

  name := strings.ToLower(fmt.Sprintf("%s.%s", databasev1alpha1.MySQLInstanceKind, controllerName))

  return ctrl.NewControllerManagedBy(mgr).
    Named(name).
    Watches(&source.Kind{Type: &v1alpha1.RDSInstance{}}, &resource.EnqueueRequestForClaim{}).
    For(&databasev1alpha1.MySQLInstance{}).
    WithEventFilter(resource.NewPredicates(resource.HasClassReferenceKind(mgr.GetClient(), resource.ClassKinds{Portable: databasev1alpha1.MySQLInstanceGroupVersionKind, NonPortable: v1alpha1.RDSInstanceClassGroupVersionKind}))).
    Complete(r)
}
```

A claim referencing a portable class will now look as follows:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: my-app
spec:
  classRef:
    name: standard-mysql
  writeConnectionSecretToRef:
    name: rdsmysql
  engineVersion: "5.6"
```

It must reference a portable class within its `namespace`, but the portable
class itself may reference a strongly-typed class in any `namespace`, allowing
for a class of service to be fulfilled by differing underlying infrastructure
across namespaces (e.g. `standard-mysql` in the `my-app` namespace may reference
an `RDSInstanceClass` while `standard-mysql` in `my-other-app` namespace
references a `CloudsqlServerInstanceClass`).

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: standard-mysql
  namespace: my-app
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: standard-mysql
  namespace: crossplane-system
```

### Denote Default via Label

This feature is similar to the original default class model in that it uses
labels to specify which class of service to use as default when a `classRef` is
omitted. For each `namespace`, there must be only one portable class instance
that has the `default` label:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: standard-mysql
  namespace: crossplane-system
  labels:
    default: true
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: standard-mysql
  namespace: crossplane-system
```

To implement this functionality, the following steps must be taken:

1. Update [default class controller
   predicates](https://github.com/crossplane/crossplane-runtime/blob/e4d61ee2805af680baf16fc2a1d8f79538d0f9bb/pkg/resource/predicates.go#L60)
   to accept resource claims that do not have a `classRef`.
1. Update the [shared default class
   reconciler](https://github.com/crossplane/crossplane-runtime/blob/e4d61ee2805af680baf16fc2a1d8f79538d0f9bb/pkg/resource/defaultclass.go#L106)
   to set the `classRef` of a claim to the portable class in its `namespace`
   with the `default` label. If multiple portable classes for that claim kind
   with the `default` label (e.g. multiple default `MySQLInstanceClass` for a
   `MySQLInstance` claim) exist in the `namespace`, the controller should fail
   to reconcile.

Both of the above changes should be implemented in `crossplane-runtime`, but
will require updates to the default class controllers in core `crossplane` to
pass in portable class kinds instead of policy kinds.

## Future Considerations

Introducing this new layer to the Crossplane dynamic provisioning pattern allows
for possible future expansion of the functionality of portable classes, which
may include "intelligent" class defaulting for claims, or referencing multiple
resource classes in the same portable class and picking one based on specific
parameters defined in the claim.

It is also likely that strongly-typed claims could be introduced at the provider
stack level in order to dynamically provision resources that are not portable
across providers. This functionality is enabled by allowing the claim
reconcilers to omit a portable class kind.

## Questions and Open Issues

* Loose classRef matching for resource claims -
  [#723](https://github.com/crossplane/crossplane/issues/723)
* Claim portability improvements -
  [#703](https://github.com/crossplane/crossplane/issues/703)
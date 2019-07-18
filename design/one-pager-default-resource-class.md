# Default Resource Classes in Crossplane
* Owner: Daniel Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Accepted

The Crossplane ecosystem exposes the concepts of [Resource Classes and Resource Claims](https://crossplane.io/docs/v0.2/concepts.html). Classes serve to define the configuration for a certain underlying resource, which may be anything from a managed cloud provider service to a traditional server. Claims are requests to create an instance of a resource and currently reference a specific deployed Class in order to abstract the implementation details. This document serves to define the concept of a *Default Resource Class*, which allows for a more general abstraction of an underlying resource.

## Goals

- Allow operators and administrators the opportunity to provide a well defined, sane default class of commonly used resources that developers commonly submit claims for within a team or organization
- Minimize the burden of determining acceptable resource claims to submit for approval to an operations team. The ability to fall back on the underlying resource that has been deemed acceptable reduces unnecessary workflow stoppages and side channel communication
- Provide an *optional* feature that does not necessarily have to be implemented within a team or organization

## Non-Goals

- Default resource classes do not aim to lock a developer into a certain underlying resource class, but simply allow them the opportunity to default to whatever has been deemed an acceptable option for the resource they desire

## Current State

Currently, resource claims must explicitly declare the the underlying resource class that they want to inherit the configuration from on deployment. For example, the following resource class may be declared for a Postgres RDS database instance on AWS:

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

This class would likely be created by an operator as a type of database that developers may deploy as part of their application. Currently, for a developer to deploy an RDS instance on AWS, they would have to explicitly reference it:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: cloud-postgresql-claim
  namespace: demo
spec:
  classReference:
    name: cloud-postgresql
    namespace: crossplane-system
  engineVersion: "9.6"
```

While this provides a nice separation of concerns for the developer and the operator, it requires the developer knowing about the `cloud-postgresql` class, and likely having to examine some of the configuration details for it.

## Proposed Workflow

While it will remain possible to explicitly reference an underlying resource class, developers will now have the option to omit the class reference and rely on falling back to whatever operators have deemed an appropriate default. The default resource class will be distinguished via the `defaultForClaimKinds` field:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: cloud-postgresql
  namespace: crossplane-system
  labels:
    postgresqlinstance.storage.crossplane.io/default: "true"
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

If a resource claim of type PostgreSQLInstance is then created without a class reference, it will default to using this class:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: cloud-postgresql-claim
  namespace: demo
spec:
  engineVersion: "9.6"
```

Internally, Crossplane will first check to see if a resource class is referenced. If not, it will check to see if a class annotated as `default` has been created for the given kind. Ultimately, if one does not exist, it will fail to provision the resource.

## Controllers

Currently, each Crossplane resource kind (i.e. GKE Cluster, AWS S3 Bucket, etc.) has a controller that reconciles claims for that resource by binding them to the corresponding managed type. These controllers use [predicates](https://github.com/crossplaneio/crossplane/blob/master/pkg/resource/predicates.go) to ensure that there is a provisioner defined for the class referenced by the claim. If the claim contains no reference to a class, the controller will not act on the claim.

Default resource classes require a single additional controller that watches for claims of any resource kind that have no class reference defined. The controller will check for this using predicates in the same fashion as the claim controllers. Upon discovery of a claim without a class reference, the controller searches for a class with an `defaultForClaimKinds` field that contains the `Kind` specified in the claim.

Finally, the controller will set the `ClassRef` of the claim to the discovered default class. The claim will now pass the predicates of the resource claim controller for the specified resource kind, and will be bound using the default class implementation.

## Future Considerations

As Crossplane evolves, it is possible that the implementation of this functionality is manifested slightly differently. One area that might affect implementation is the introduction of [strongly typed resource classes](https://github.com/crossplaneio/crossplane/issues/90). However, the workflow for developers and operators would remain largely the same in regards to usage of default resource classes.

Additionally, some resources in Crossplane that are not portable (i.e. do not have comparable resources across providers) may eventually be implemented as their own concrete resource types. This may introduce the desire to have multiple resource claim kinds for which a resource class serves as default. For example, a claim may specify its `Kind` as either a `NoSQLInstance` or a more specific `DynamoDBInstance` and operators may want to specify the same default resource class for both of these claim kinds. The proposed implementation in this document would make this functionality possible because `defaultForClaimKinds` allows for multiple values.

In this scenario, a default resource class may look like this:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: cloud-nosql
  namespace: crossplane-system
parameters:
  ...
defaultForClaimKinds:
- nosqlinstance.database.crossplane.io
- dynamodbinstance.storage.crossplane.io
provisioner: dynamodbinstance.database.aws.crossplane.io/v1alpha1
providerRef:
  name: aws-provider
reclaimPolicy: Delete
```

And a resource claim that defaulted to the resource class above could look like this in the general case:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: NoSQLInstance
metadata:
  name: dynamo-claim
  namespace: demo
spec:
  ...
```

Or in the more specific case:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: DynamoDBInstance
metadata:
  name: dynamo-claim
  namespace: demo
spec:
  ...
```

## Questions and Open Issues

* Default Resource Classes: [#151](https://github.com/crossplaneio/crossplane/issues/151)
* Strongly Typed Resource Classes: [#90](https://github.com/crossplaneio/crossplane/issues/90)
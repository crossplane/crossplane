# Strongly Typed Resource Classes
* Owner: Daniel Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Revisions
* 1.1
  * Added additional motivation by describing the use of annotations for UI metadata on strongly typed resource class CRD's as outlined in [#605](https://github.com/crossplane/crossplane/pull/605)
* 1.2
  * Note that policies have been replaced by simple resource class selection.

## Terminology

* **Abstract Resource**: a resource type that a `ResourceClaim` can specify as its kind. They are general and portable across managed service providers (i.e. `MySQLInstance`).
* **Resource Class**: a Kubernetes resource that contains implementation details specific to a certain environment or deployment, and policies related to a kind of resource.
* **Resource Claim**: a Kubernetes resource that captures the desired configuration of a resource from the perspective of a workload or application.

## Background

Crossplane provisions resources by allowing resource claims to specify a reference to a resource class, or fall back on a [default resource class](one-pager-default-resource-class.md) for the claim's kind. The `kind` of a resource claim is a portable infrastructure type that may have concrete implementations on multiple providers (i.e. `MySQLInstance`, `Bucket`, `RedisCluster`, etc.). Resource classes are all of `kind: ResourceClass` and specify a provisioner that dictates the actual provider implementation. For example, the following resource class is of `kind: ResourceClass` and specifies its `provisioner` as `rdsinstance.database.aws.crossplane.io/v1alpha1`. 

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: standard-mysql
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

The resource claim below specifies that it is of `kind: MySQLInstance`, and references the above resource class in its `classRef`.

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: demo
spec:
  classRef:
    name: standard-mysql
    namespace: crossplane-system
  engineVersion: "9.6"
```

If using a default resource class, the class and claim would be adjusted as follows.

Resource Class:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: standard-mysql
  namespace: crossplane-system
  labels:
    mysqlinstance.storage.crossplane.io/default: "true"
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

Resource Claim:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: demo
spec:
  engineVersion: "9.6"
```

This model is powerful because it allows an application developer to create a resource claim without having to know the implementation details or even the underlying provider. However, the fact that every resource class is of the same `kind` presents a key issue: The required parameters for a resource class may vary widely, and they are currently only provided as an arbitrary map that is eventually read by the controller for the specified `provisioner`. Therefore, an administrator who is creating resource classes does not know what fields are required and will not be notified of missing or extraneous fields until the provisioning of a resource that references the class.

The `parameters` supplied by the resource class are used to populate the `spec` of the managed resource (i.e. the Kubernetes representation of the external resource) when it is created. For instance, the creation of `mysql-claim`, which references the `standard-mysql` class, is watched by the claim controller for AWS RDS instances. It brings together the information provided in the claim and class to create the `RDSInstance` managed resource. Specifically, it calls the `ConfigureMyRDSInstance()` function. As part of the configuration, the function creates the `spec` of the `RDSInstance` managed resource from the `parameters` of the `ResourceClass`:

```go
spec := v1alpha1.NewRDSInstanceSpec(cs.Parameters)
```

Looking at the `NewRDSInstanceSpec()` function reveals how the parameters are being parsed:

```go
// NewRDSInstanceSpec from properties map
func NewRDSInstanceSpec(properties map[string]string) *RDSInstanceSpec {
	spec := &RDSInstanceSpec{
		ResourceSpec: xpv1.ResourceSpec{
			ReclaimPolicy: xpv1.ReclaimRetain,
		},
	}

	val, ok := properties["masterUsername"]
	if ok {
		spec.MasterUsername = val
	}

	val, ok = properties["engineVersion"]
	if ok {
		spec.EngineVersion = val
	}

	val, ok = properties["class"]
	if ok {
		spec.Class = val
	}

	val, ok = properties["size"]
	if ok {
		if size, err := strconv.Atoi(val); err == nil {
			spec.Size = int64(size)
		}
	}

	val, ok = properties["securityGroups"]
	if ok {
		spec.SecurityGroups = append(spec.SecurityGroups, strings.Split(val, ",")...)
	}

	val, ok = properties["subnetGroupName"]
	if ok {
		spec.SubnetGroupName = val
	}

	return spec
}
```

This is a very rough parsing process and can result in missing parameters without notifying the Crossplane user.

This problem can be solved by the implementation of "strongly typed" resource classes. Instead of every resource class being of `kind: ResourceClass`, they would map one-to-one with the concrete managed resource kind for which they provided specifications (i.e. `kind: RDSInstanceClass`).

## Goals

* Enable schema documentation and validation for resource classes.
* Enable the ability to inject type-specific annotations into resource class kinds.
* Continue to support configuring a default resource class to be set upon resource claims that do not specify a class.
* Assume that the universe of possible default resource class kinds for a specific claim kind will grow arbitrarily, so a default class controller must be able to be made aware of new resource class kinds. This goal will become specifically important as we begin to [separate providers](https://github.com/crossplane/crossplane/issues/531) from the core Crossplane project in the form of infrastructure stacks.

## Proposal

**Note: while the parts of this proposal pertaining to strongly typed resource
  classes are still broadly accurate the concept of policies has been removed.
  refer to the [simple resource class selection](one-pager-simple-class-selection.md)
  design for details.**

Unfortunately, strongly typed resource classes make it much more difficult to identify default resource classes. Currently, there is one default class controller per claim kind that watches for creation of their specified claim kind, then lists objects of `kind: ResourceClass` that are labeled as default for the claim kind. With an unknown number of possible kinds of resource classes that could be installed and serve as default for a claim `kind`, a default class controller can no longer list a single `ResourceClass` kind.

One solution would instead be to map resource classes one-to-one with claim kinds (i.e. `MySQLInstanceClass`), but that results in the same issue of varying required parameters that is currently being experienced.

In order to satisfy both the requirements of strong typing and default resource classes, a new set of CRD's can be implemented to specify when an instance of a strongly typed resource class kind should serve as default for a given claim kind. These default policies should be able to be defined at both the cluster and namespace level, so there must exist two separate CRD's for each claim kind. For example, the `MySQLInstance` claim kind will be accompanied by the `MySQLInstancePolicy` (namespace-scoped) and `MySQLInstanceClusterPolicy` (cluster-scoped) kinds. Importantly, when single instances of both the `MySQLInstancePolicy` and `MySQLInstanceClusterPolicy` exist, the `MySQLInstancePolicy` would take precedence because it is at the more granular (namespace) level.

## Workflow

The workflow for a namespace-scoped policy (`kind: MySQLInstancePolicy`) would proceed as follows:

1. An admin installs Crossplane, which specifies the portable claim kinds that are available as part of the core Crossplane.
2. An admin installs an infrastructure stack (i.e. a set of CRD's and controllers for a provider's managed service offerings).
3. An admin creates an instance of an installed resource class CRD (i.e. `kind: RDSInstanceClass`). It could look as follows:

```yaml
apiVersion: database.aws.crossplane.io/v1alpha1
kind: RDSInstanceClass
metadata:
  name: standard-mysql
  namespace: crossplane-system
specTemplate:
  class: db.t2.small
  masterUsername: masteruser
  securityGroups:
    - sg-ab1cdefg
    - sg-05adsfkaj1ksdjak
  size: 20
  providerRef:
    name: aws-provider
  reclaimPolicy: Delete
```

4. An admin creates a new `MySQLInstancePolicy` object, specifying that the previously created resource class (`kind: RDSInstanceClass`) instance should serve as default for a given claim kind `mysqlinstance.storage.crossplane.io`. A `MySQLInstancePolicy` could look as follows:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstancePolicy
metadata:
  name: mysql-aws-policy
  namespace: demo
defaultClassRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: standard-mysql
  namespace: crossplane-system
```

5. A developer creates a new resource claim (`kind: MySQLInstance`), omitting a class reference so as to fall back on the class defined as default for the claim kind:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: demo
spec:
  engineVersion: "9.6"
```

6. Upon creation, the `MySQLInstance` default class controller searches for `MySQLInstancePolicy` and `MySQLInstanceClusterPolicy` objects. In this case, there is a single `MySQLInstancePolicy` that exists in the same namespace as the `MySQLInstance` claim, so the claim's `classRef` is set to match the `defaultClassref` of the `MySQLInstancePolicy`. The following scenarios could have occurred to cause a different outcome:
    * If there had been multiple `MySQLInstancePolicy` instances in the claim's namespace, the claim would have failed to be reconciled by the policy controller because it would not be able to choose only one, despite the presence of a `MySQLInstanceClusterPolicy` or not (i.e. an undecidable namespace level policy situation does not fall back on a decidable cluster level policy, it just fails).
    * If there had been no instances of `MySQLInstancePolicy` in the claim's namespace, but there had been a `MySQLInstanceClusterPolicy` instance defined in the claim's cluster, the claim's `classRef` would have been set to the `MySQLInstanceClusterPolicy` instance's `defaultClassRef`.
    * If there had been no instances of `MySQLInstancePolicy` in the claim's namespace, but there had been multiple instances of `MySQLInstanceClusterPolicy` defined in the claim's cluster, the claim would have failed to be reconciled by the policy controller because it would not be able to choose only one.
7. The claim is reconciled by the claim controller now that it passes the class reference predicate.

In addition, a resource claim that directly references an instance of a specific strongly typed resource class (as opposed to falling back on the default) must now specify the `kind` and `apiVersion` of the resource class:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: demo
spec:
  classRef:
    kind: RDSInstanceClass
    apiVersion: database.aws.crossplane.io/v1alpha1
    name: standard-mysql
    namespace: crossplane-system
  engineVersion: "9.6"
```

## Future State

### Expanding Policies

The construction of these policy kinds allows for the future option of adding additional claim fields that can also be defaulted via the policy object. For example, if `providerRef`, which is currently a field for resource classes, was moved to the claim level, the policy could be expanded to specify default provider behavior as well. The `MySQLInstancePolicy` is used again for demonstration:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstancePolicy
metadata:
  name: mysql-aws-policy
  namespace: crossplane-system
defaultClassRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: standard-mysql
  namespace: crossplane-system
defaultProviderRef:
  kind: Provider
  apiVersion: aws.crossplane.io/v1alpha1
  name: my-aws-account
  namespace: crossplane-system
```

### Resource Class Annotations

The movement to strongly typed resource classes provides an opportunity to inject additional metadata in the form of annotations into a class's CRD when it is added to Crossplane. This metadata may include resource specific information that would be useful in creating an enhanced user experience via a GUI or some other presentation format. These annotations would be applied in a formal manner via the [package manager](design-doc-packages.md).

## Implementation

* The different kinds of resource classes will live with their infrastructure stacks, and will map one-to-one with the managed resources that the stack implements.
* The existing `ResourceClass` kind will be removed from `core.crossplane.io` as there will be no generic resource classes.
* The claim kinds will continue to exist as they do currently as part of the core Crossplane. However, the claim controllers will have to be modified to accommodate the presence of strongly typed resource classes.
* The `Policy` and `ClusterPolicy` resources will be added as part of the core Crossplane.
* The default class controllers will be refactored as policy controllers, and will behave as demonstrated in the workflow above.

## Relevant Issues

[#90](https://github.com/crossplane/crossplane/issues/90)
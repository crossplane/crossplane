# Generic Resource References

* Owner: Predrag Knezevic (@pedjak)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Crossplane providers provision and monitor external resources in third party
cloud providers, following the declared specification in the corresponding managed
resources. Very often, creation of these objects cannot happen in parallel, i.e. some
fields can be set only after a given dependency is created.

[Cross resource referencing] solves this problem for majority of cases by
following [Kubernetes API object reference] convention:

```yaml
spec:
  fooRef:
    name: bar
```

Field `spec.fooRef` contains the name of the resource where the value for `spec.foo` is to
be found. The kind of referred resource and the field containing the value used later for
setting `spec.foo` is determined during compile time.

Although the used convention covers the majority of use cases, it lacks the
following properties:

* The referred object kind and the source field cannot be changed at 
  runtime for a given object
* The referencer field may not always target a single referred kind. For
  example, a URL field in Route53 could get populated by a URL of an S3 bucket
  or access endpoint of an RDS Instance
* The referencer field may not always target a single field in the referred
  object
* The value that is about to be assigned to the field require some sort of
  transformation

Some of the above issues can be solved by defining a composition. However, there
are cases where one would like to use and refer to a resource not bound by a 
composite instance.

__**NOTE**__: [a prior design exists on this topic](https://github.com/crossplane/crossplane/pull/2385).

## Goals

The generic cross-reference should be able to:

* be configurable at runtime
* use the existing [Kubernetes API object reference] convention 
  as much as possible
* provide alternative convention in cases where [Kubernetes API object reference]
  convention is not suitable
* be implemented without or requiring only light changes in RBAC
* support referring multiple objects (namespaced and cluster-scoped) and their
  fields
* support value transformation before assignment

## Proposal

We would like to introduce a new cluster-scoped `Referable` (alternative name
could be `InjectableValue`) type whose instances declare values that can be
assigned to an object field:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: referable-vpc-id
spec:
  objects:
    - apiVersion: ec2.aws.crossplane.io/v1alpha1
      kind: VPC
      name: main-vpc
      fieldPath: status.atProvider.id
```

The syntax of the `fieldPath` field is similar to what we
use in Composition, in line with the [Kubernetes API Conventions on field selection].

After deploying the above instance to the cluster, its reconciliation gets 
triggered and once the referred object exists and the field is set, 
the referable value is emitted in the object status part:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: referable-vpc-id
spec:
  objects:
    - apiVersion: ec2.aws.crossplane.io/v1alpha1
      kind: VPC
      name: main-vpc
      fieldPath: status.atProvider.id
status:
  value: foo-vpc-id # the value found in the requested field
  conditions:
    - type: Ready
      status: True
      reason: Available
```

Now, the value can be referred in the usual way within a managed resource:

```yaml
spec:
  vpcIdRef: referable-vpc-id
```

The reference resolver could first try to find the `referable-vpc-id` `VPC` instance, 
and fallback to `referable-vpc-id` `Referable` if the former does not exist. 
If the instance is ready, the value is read from `status.value` and 
set to `spec.vpcId` of the managed resource. Of course, if needed, and if it 
makes more sense, the resolver could first look for the existence of `Referable` 
object first. With that strategy, we would be able to overwrite the referencing
mechanism set at the compile-time.

Referring to a namespaced object would be possible as well:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: region
spec:
  objects:
    - apiVersion: v1
      kind: ConfigMap
      name: common-settings
      namespace: crossplane-system
      fieldPath: data.region
```

or we could match an object with a label selector if its name is unknown or not
static:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: referable-vpc-id
spec:
  objects:
    - apiVersion: ec2.aws.crossplane.io/v1alpha1
      kind: VPC
      matchingLabels:
        class: main-vpc
      fieldPath: status.atProvider.id
```

### Value Assigning Without Counterpart Ref Field

If a value needs to be assigned to a field that does not have a
corresponding `Ref` field, the reference can be expressed using `spec.refs` block:

```yaml
spec:
  refs:
    - name: referable-vpc-id
      toFieldPath: spec.myVPCId
```

The syntax of the `toFieldPath` field is similar to what we
use in Composition, in line with the [Kubernetes API Conventions on field selection].

Alternatively, if we would like to avoid changing the managed resource schema, the
above can be stated using annotations as well:

```yaml
metadata:
  annotations:
    "referable.upbound.io/referable-vpc-id": "spec.myVPCId"
```

### Referring Value Existing Potentially in Multiple Source

If a value could be found in multiple objects of different kinds, we can mark
them as optional and pick the value from the first one found:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: multi-ref
spec:
  objects:
    - apiVersion: example.com/v1
      kind: Foo
      name: foo
      optional: true
      fieldPath: spec.id
    - apiVersion: example.com/v1
      kind: Bar
      name: bar
      optional: true
      fieldPath: spec.myId
```

The above syntax can be used as well when a value might appear in a several
places within single object.

### Value Transformation

Sometimes the referable value needs to be transformed or constructed from
several other values. `Referable` type can be enriched to support that 
by adding `spec.mapping` block:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: service-url
spec:
  objects:
    - apiVersion: example.com/v1
      kind: Foo
      name: foo
      fieldPath: spec.host
      id: host # optional variable name in template context
    - apiVersion: example.com/v1
      kind: Foo
      name: foo
      fieldPath: spec.port
      id: port # optional variable name in template context
  mapping:
    value: "https://{{ .host }}:{{ .port }}"
```

The used templating engine in `mapping.value` is [Go templates]. Each object 
reference with assigned `id` becomes available under that name in the template context.
A number of useful transformation functions could be made available to that context.

### Advantages

* All referred values are declared on managed resources using already familiar
  mechanism, keeping their discovery simple
* Value extraction is detached from its consumption. The same `Referable` can be
  used by multiple managed resources
* `Referable` instances can be watched and used by non-Crossplane controllers as
  well
* `Referable` instances can be properly garbage collected if owner
  or [`Usage`](https://github.com/crossplane/crossplane/blob/master/design/one-pager-generic-usage-type.md)
  instances are declared
* Multiple objects/fields can be referred as value sources
* Value can be transformed using a Go template

### Disadvantages

* Providers might need to get read access to `Referable` type 
  in order to resolve generic cross-resource references

### Implementation

It consists of two parts:

* Adding new Crossplane controller for the `Referable` type
* Update [crossplane-runtime](https://github.com/crossplane/crossplane-runtime/)
  to support usage of `Referable` instances

#### Referable Controller

The controller is responsible for:

* Defining proper `Usage` instance to guard against improper deletion
  of `Referable` instances
* Retrieving the declared objects and the field value, based on the provided
  references
* Transforming the value (if requested) and exposing it under `status.value`
* Marking the instance as ready

We assume currently that the value does not change after it gets exposed.
Allowing updates and propagating them to the managed resources is out
of the scope for this proposal version.

No additional RBAC rules are needed, since Crossplane already poses very broad
permissions within the cluster.

#### Crossplane-runtime Changes

* [APIResolver](https://github.com/crossplane/crossplane-runtime/blob/master/pkg/reference/reference.go#L280)
  should be enriched to support `Referable` instances
* If additional references need to be declared via the `spec.refs` block or
  annotations, the [ResolveReferences function generator](https://github.com/crossplane/crossplane-tools/blob/master/internal/method/resolver.go#L33)
  should be enriched to support them
* After fetching values, proper `Usage` instance need to be defined to guard
  against improper deletion

Finally, providers need to be upgraded to use the new version of crossplane-runtime 
and for each type, the `ResolveReference` function needs to be regenerated.

## Alternatives Considered

* [A previous design](https://github.com/crossplane/crossplane/pull/2385) was
  proposed for this topic.

  It embeds the schema similar `Referable` type into a number of `spec.patches.fromObject`
  fields in managed resources. Such an approach would require that each provider 
  has access to a broad set of resources (even from other providers), demanding
  a broad set of RBAC rules. Furthermore, if a certain value transformation is
  needed on multiple managed resources, it would be required to repeat its 
  declaration on every MR.

* `Referable` instances could also contain the reference 
   to the destination managed resource and its field.
    - Such approach would make the discovery of references for a given managed
      resource harder
    - Referable controller logic becomes more complex, because the destination
      managed resource might not exist
    - One more controller can now update a managed resource, increasing
      potential conflict rate
    - `Referable` instance could not be consumed by multiple managed resources
    - However, providers would not need the read access to `Referable` instances

[Cross resource referencing]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
[Kubernetes API object reference]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#object-references
[Kubernetes API Conventions on field selection]: https://github.com/kubernetes/community/blob/744e270/contributors/devel/sig-architecture/api-conventions.md#selecting-fields
[Go templates]: https://pkg.go.dev/text/template
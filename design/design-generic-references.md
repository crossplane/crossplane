# Generic Resource References

* Owner: Muvaffak Onuş (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

In many cases, Crossplane managed resources require information that is not
available when the user creates the resource. One example is that if you'd like to create
a `Subnet`, you need to supply a VPC ID but if you create the `VPC` at the same
time then the ID will be available only after the creation, so you need to extract
the ID and use it in `Subnet`. Today, it's possible to let the provider do this
operation using cross-resource references. However, this requires knowing the kind of
the referenced resource and how to extract the value during development time of
the provider. There are several cases where this is not possible:

* The referencer field may not always target a single kind. For example, a URL
  field in Route53 could get populated by a URL of an S3 bucket or access endpoint
  of an RDS Instance.
* Instead of hard-coding region for all managed resources, you may want to refer
  to a `ConfigMap` or a `Secret` and let the managed resource take that
  information from there.
* In Composition, you can use values of the fields only between the children of
  the same composite instance, but there are cases where you'd like to use a
  separate resource that is not bound to any composite instance to store the
  information and serve all composites.

In order to enable such use cases, we need a generic version of the cross-resource
references where you can specify all metadata required to find the referenced
resource as well as details about how to extract the value you need and where to
put it in the referencer object.

## Proposal

All managed resources will have a top-level spec field that lists the generic
references. They will be resolved in order and failure of any will cause
reconciliation to fail. Inspired from the [latest developments in API conventions](https://github.com/kubernetes/community/pull/5748)
regarding multi-kind references, the API will look like the following:
```yaml
# A generic reference to another managed resource.
spec:
  externalValues:
  - fromObject:
      group: ec2.aws.crossplane.io
      version: v1alpha1
      resource: vpcs
      name: main-vpc
      fieldPath: spec.forProvider.cidrBlock
    toFieldPath: spec.cidrBlock
```
```yaml
# A generic reference to a namespaced Kubernetes resource.
spec:
  externalValues:
  - fromObject:
      version: v1
      resource: configmaps
      name: common-settings
      namespace: crossplane-system
      fieldPath: data.region
    toFieldPath: spec.forProvider.region
```

The syntax of `fieldPath` and `toFieldPath` fields will be similar to what we
use in Composition, in line with [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/744e270/contributors/devel/sig-architecture/api-conventions.md#selecting-fields).

The provider pods will resolve the references via managed reconciler, hence they
will need at least read-only permissions for any resource that's been referenced
by an instance. Currently, every provider [has read access](https://github.com/crossplane/crossplane/blob/d8f57a8/internal/controller/rbac/provider/roles/roles.go#L66)
to all of its own kinds, `Secret`s, `ConfigMap`s, `Lease`s and `Event`s in all
namespaces. In order to reference resources other than the ones listed, users will
have to create necessary RBAC resources manually to grant the permissions.

### Implementation

The new field will exist in all managed resources and the cross-resource reference
fields will stay as is and we'll continue adding them wherever possible. They will
still be the main way to refer to another resource. The generic references will
help in cases where that's not possible or not implemented yet.

The API struct and the generic resolver will live in Crossplane Runtime and be
integrated into managed reconciler. Providers will only have to update their
runtime dependency and regenerate their CRDs to get the feature in.

One of the caveats with following the new conventions is that controller-runtime
utilities mostly assume that you have Group-Version-Kind of the resource you're
operating with. In order to use Group-Version-Resource, we will make use of
[`EquivalentResourceRegistry`](https://github.com/kubernetes/apimachinery/blob/bf1bfd9/pkg/runtime/mapper.go#L44)
object as a translator between the two, implemented as an alternative to
[`MustCreateObject`](https://github.com/crossplane/crossplane-runtime/blob/406fe0b/pkg/resource/resource.go#L145).
After getting a `runtime.Object`, all controller-runtime utilities should be
available to use.

The resolver will make use of `fieldpath` library we're using for composition to
get and set the values unknown types and once it is done, an update request will
be issued. In line with cross-resource references, if the field on the referenced
object, pointed by `toFieldPath`, already has a value then the resolver will
skip it, meaning it will resolve only once and users will need to delete that
value for resolver to set it again.

## Future Considerations

### Transforms

The retrieved value may not be in the form that's useful for the referencer
resource directly and may need some formatting. If we see a great need for that,
we can move the transform machinery from Crossplane to Crossplane Runtime and
reuse it.

### More Limited RBAC

Currently, providers have access to all `Secret`s in the cluster. While this will
provide a nice UX for generic reference users, it may not be desirable for security
minded folks since it'd cause giving managed resource permission mean giving access
to any `Secret` in the cluster. See [#2384](https://github.com/crossplane/crossplane/issues/2384).

We can consider new settings in `Provider` or `ControllerConfig` to let users
control this behavior with more granularity.

### Composite Resources

We could possibly have this on composite resources as well to keep them similar
to managed resources, hence improving nested composition construction experience.
But while its implementation would be straight-forward, there are some issues like
whether we should let claim authors propagate this field as well and would that
mean letting them access everything provider has access to, which is not few.

### Obscure References

There could be cases where you'd like to fetch a value from another resource but
don't want to expose it on the managed resource object, i.e. make it available to
controller to be inclduded in the calls but not visible on the CR. We could think
of ways letting controller know that by either giving `toFieldPath: OBSCURED` or
`makeAvailableAs: <some key in a map>`.

## Alternatives Considered

### Simpler API

```yaml
spec:
  externalValues:
    - group: ec2.aws.crossplane.io
      version: v1alpha1
      resource: vpcs
      name: main-vpc
      fromFieldPath: spec.forProvider.cidrBlock
      toFieldPath: spec.cidrBlock
```

We could certainly have gone with this approach as well but having everything at
the same level increases the risk of having to break the API to add new features.
Although it's probably safe that we will not see many new types of sources be
added, because a lot of use cases are already covered by composition patch types,
it could be desirable to have multiple source objects to construct a single value
for example and that'd be a new type. So, we're taking the safer approach of
letting the API be more open-ended for future developments and pay a small UX
fee in exchange.

### Multiple Source References

Composition allows information to flow from single resource to another due to
the nature of how patches are designed. However, there could be cases where you'd
like to construct a single value using information from multiple resources, like
an ARN. An API that would allow this could look like the following:

```yaml
spec:
  externalValues:
  - combine:
    - version: v1
      resource: configmaps
      name: common-settings
      namespace: crossplane-system
      fieldPath: data.region
    - version: v1
      resource: secrets
      name: accounts
      namespace: crossplane-system
      fieldPath: data.prodAWSAccountID
    fmt: "arn:aws:clouddirectory:%s:%s:schema/published/cognito/1.0"
    toFieldPath: "spec.forProvider.directoryArn"
```
Or alternatively:
```yaml
spec:
  externalValues:
  - variables:
    - name: region
      fromRef:
        version: v1
        resource: configmaps
        name: common-settings
        namespace: crossplane-system
        fieldPath: data.region
    - name: accountId
      fromRef:
        version: v1
        resource: secrets
        name: accounts
        namespace: crossplane-system
        fieldPath: data.prodAWSAccountID
    template: "arn:aws:clouddirectory:{{ .region }}:{{ .accountId }}:schema/published/cognito/1.0"
    toFieldPath: "spec.forProvider.directoryArn"
```

The advantage of this option is that it will allow composite/managed resources to
get information from multiple resources to construct a single value, which is not
possible today in any way. However, it's questionable how necessary it is since
we haven't seen many users reporting that they need this functionality.

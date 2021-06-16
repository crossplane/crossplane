# Crossplane Agent for Consumption

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
  to a `ConfigMap` and let the managed resource take that information from there.
* In Composition, you can trade values of the fields only between the children of
  the same composite instance, but there are cases where you'd like to use a separate
  composite-independent resource to store the information and serve all composites.

In order to enable such use cases, we need a generic version of the cross-resource
references where you can specify all metadata required to find the referenced
resource as well as details about how to extract the value you need and where to
put it in the referencer object.

## Proposal

All managed resources and composite resources will have a top-level spec field
that lists the generic references. They will be resolved in order and failure of
any will cause reconciliation to fail. Inspired from the
[latest developments in API conventions](https://github.com/kubernetes/community/pull/5748)
regarding multi-kind references, the API will look like the following:
```yaml
# A generic reference to another managed resource.
spec:
  references:
  - group: ec2.aws.crossplane.io
    version: v1alpha1
    resource: vpcs
    name: main-vpc
    fromFieldPath: spec.forProvider.cidrBlock
    toFieldPath: spec.cidrBlock
```
```yaml
# A generic reference to a namespaced Kubernetes resource.
spec:
  references:
  - version: v1
    # group is omitted since it is empty for ConfigMap.
    resource: configmaps
    name: common-settings
    namespace: crossplane-system
    fromFieldPath: data.region
    toFieldPath: spec.forProvider.region
```

The syntax of `fromFieldPath` and `toFieldPath` fields will be similar to what we
use in Composition, in line with [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#selecting-fields).

The provider pods will resolve the references via managed reconciler, hence they
will need at least read-only permissions for any resource that's been referenced
by an instance. Currently, every provider [has read access](https://github.com/crossplane/crossplane/blob/master/internal/controller/rbac/provider/roles/roles.go#L66)
to all of its own kinds, `Secret`s, `ConfigMap`s, `Lease`s and `Event`s in all
namespaces. In order to reference resources other than the ones listed, users will
have to create necessary RBAC resources manually.

### Implementation

Since the field will exist in all managed resources, the API struct and the generic
resolver will live in Crossplane Runtime and be integrated into managed reconciler.
Providers will only have to update their runtime dependency and regenerate their
CRDs to get the feature in.

The API struct will look roughly as the following:
```go
type GenericReference struct {
    Resource      string `json:"resource"`
    Name          string `json:"name"`
    FromFieldPath string `json:"fromFieldPath"`
    ToFieldPath   string `json:"toFieldPath"`
    
    // +optional
    Group *string `json:"group,omitempty"`
    // +optional
    Version *string `json:"version,omitempty"`
    // +optional
    Namespace *string `json:"namespace,omitempty"`
}
```

One of the caveat with following the new conventions is that controller-runtime
utilities mostly assume that you have Group-Version-Kind of the resource you're
operating with. In order to use Group-Version-Resource, we will make use of
`EquivalentResourceRegistry` object as a translator between the two, implemented
as an alternative to [`MustCreateObject`](https://github.com/crossplane/crossplane-runtime/blob/406fe0b/pkg/resource/resource.go#L145).
After getting a `runtime.Object`, all controller-runtime utilities should be
available to use.

The resolver will make use of fieldpath library we're using for composition to
get and set the values unknown types and once it is done. In line with cross-resource
references, if the field on the referenced object, pointed by `toFieldPath`, already
has a value then the resolver will skip it, meaning it will resolve only once and
users will need to delete that value for resolver to set it again.

## Alternatives Considered

### More Open-ended API

A variation of the following suggestion was made in the initial issue [#1770](https://github.com/crossplane/crossplane/issues/1770).

```yaml
spec:
  fieldModifiers:
    - fromRef:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: VPC
        name: my-vpc
        fieldPath: metadata.annotations[crossplane.io/external-name]
      toFieldPath: spec.forProvider.vpcId
```

This API would allow future source additions to `fromRef` by exposing a type
discriminator. However, it's likely that any additions will start to compete with
what you can do with composition patches. Converting this to the new API convention,
it's a pretty good alternative to the proposal above since it's safer but I'd like
to see more use cases where we'd like to implement different source types.

### Multiple Source References

Composition allows information to flow from resource to another due to the nature
of how patches are designed. However, there could be cases where you'd like to
construct a single value using information from multiple resources, like an ARN.
An API that would allow this could look like the following:

```yaml
spec:
  references:
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
  references:
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

The advantage of this option is that it will allow composite resource to get
information from multiple resources to construct a single value, which is not
possible today. However, it's questionable how much necessary this is since
we haven't seen many users reporting that they need this functionality.

## Future Considerations

### Transforms

The retrieved value may not be in the form that's useful for the target resource
directly and may need some formatting. If we see a great need for that, we can
move the transforms from composition to Crossplane Runtime and use it in
`GenericReference` object as well.

### More Limited RBAC

Currently providers have access to all `Secret`s in the cluster. While this will
provide a nice UX for generic reference users, it may not be desirable for security
minded folks since it'd cause giving managed resource permission mean giving access
to any `Secret` in the cluster. See [#2384](https://github.com/crossplane/crossplane/issues/2384).
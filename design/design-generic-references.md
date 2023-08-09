# Generic Resource References

* Owner: Predrag Knezevic (@pedjak)
* Reviewers: Nic Cope (@negz), Hasan Türken (@turkenh), Bob Haddleton (@bobh66)
* Status: Draft

## Background

Crossplane providers provision and monitor external resources in third party
cloud providers, following the declared specification in the corresponding
managed resources. Very often, creation of these objects cannot happen in
parallel, i.e. some fields can be set only after a given dependency is created.

[Cross resource referencing] solves this problem for majority of cases by
following [Kubernetes API object reference] convention:

```yaml
spec:
  fooRef:
    name: bar
```

Field `spec.fooRef` contains the name of the resource where the value
for `spec.foo` is to be found. The kind of referred resource and the path to the
field containing the value used later for setting `spec.foo` is fixed, i.e. it
is embedded in the resolving mechanism used by the provider.

Although the used convention covers the majority of use cases, user experience
shows that there is still a need to refer a value on an arbitrary path within an
object of arbitrary kind. For example, 
[`datasync.aws.upbound.io` `Task`](https://github.com/upbound/provider-aws/blob/main/apis/datasync/v1beta1/zz_task_types.go#L375)
has [`spec.forProvider.sourceLocationArn`](https://github.com/upbound/provider-aws/blob/main/apis/datasync/v1beta1/zz_task_types.go#L359)
field that can obtain value from an instance of the following object types:

* LocationS3
* LocationSmb
* LocationObjectStorage
* LocationNfs
* LocationHdfs
* LocationFsxWindowsFileSystem
* LocationFsxOpenzfsFileSytem
* LocationFsxLustreFileSystem
* LocationEfs

However, [the implemented provider resolving mechanism](https://github.com/upbound/provider-aws/blob/main/apis/datasync/v1beta1/zz_generated.resolvers.go#L104-L112)
looks only for the value in `LocationS3` instance named
by `spec.forProvider.sourceLocationArnRef` field.

Currently, the issue can be solved by defining the following composition:

* Declare under managed resources the needed source object (one of `Location*`)
  types with [observe only management policy]
* Patch composite object with the value from the source object
* Patch Task's `spec.forProvider.sourceLocationArn` field with the composite
  value

The proposed solution has the following drawbacks:

* For each claim, additional observe only resource gets created, just to be able
  to work around the limitations of the current reference mechanism on certain
  fields. In a system with a high number of claims, that would put additional
  pressure on k8s API and etcd.
* Composite object needs to expose the patched value, although fully unneeded
  from API design perspective

__**NOTE**__: [a prior design exists on this topic](https://github.com/crossplane/crossplane/pull/2385).

## Goals

The generic cross-reference should be able to:

* Refer a value on an arbitrary path within an object of arbitrary kind
* Use the existing [Kubernetes API object reference] convention as much as
  possible
* Be implementable without or requiring only light changes in RBAC

In cases when the generic cross-references are unavailable for a given managed
resource field, our compositions should not require declaring observe-only
resource, for sole purpose of referring a value.

## Proposal

Until now, providers were supporting only reference fields of
type [`Reference`](https://github.com/pedjak/crossplane-runtime/blob/master/apis/common/v1/resource.go#L116-L123):

```go
type Reference struct {
// Name of the referenced object.
Name string `json:"name"`

// Policies for referencing.
// +optional
Policy *Policy `json:"policy,omitempty"`
}
```

### GenericReference Type

We would like to introduce `GenericReference` field type:

```go
type GenericReference struct {
  Reference
  
  // ApiVersion of the referenced object.
  ApiVersion string `json:"apiVersion"`
  
  // Kind of the referenced object.
  Kind string `json:"kind"`
  
  // FieldPath of the value within the referenced object.
  FieldPath string `json:"fieldPath"`
}
```

The syntax of the `fieldPath` field is the one used in Composition, in line with
the [Kubernetes API Conventions on field selection].

Reference fields of the above type describe completely the location of 
the value that is about to be set on the counterpart field. In case of 
`datasync.aws.upbound.io` `Task`, an instance might look like:

```yaml
apiVersion: datasync.aws.upbound.io/v1beta1
kind: Task
metadata:
  name: datasync-task-example
spec:
  forProvider:
    .
    .
    sourceLocationArnRef:
      name: source-location
      apiVersion: datasync.aws.upbound.io/v1beta1
      kind: LocationS3
      fieldPath: metadata.annotations[crossplane.io/external-name]
```

Such approach lets a user to point the `sourceLocationArnRef` field to a different
resource at the runtime.

### GenericSelector Type

Similar to `GenericReference`, we are going to introduce `GenericSelector` type:

```go
type GenericSelector struct {
  // MatchLabels ensures an object with matching labels is selected.
  MatchLabels map[string]string `json:"matchLabels,omitempty"`
  
  // Policies for selection.
  // +optional
  Policy *Policy `json:"policy,omitempty"`
  
  // ApiVersion of the referenced object.
  ApiVersion string `json:"apiVersion"`
  
  // Kind of the referenced object.
  Kind string `json:"kind"`
  
  // FieldPath of the value within the referenced object.
  FieldPath string `json:"fieldPath"`
}
```

so that one can reference the external object when its name is
unknown/irrelevant:

```yaml
apiVersion: datasync.aws.upbound.io/v1beta1
kind: Task
metadata:
  name: datasync-task-example
spec:
  forProvider:
    .
    .
    sourceLocationArnSelector:
      matchLabels:
        foo: bar
      apiVersion: datasync.aws.upbound.io/v1beta1
      kind: LocationS3
      fieldPath: metadata.annotations[crossplane.io/external-name]
```

Although the proposed solution sounds like a natural evolution of the existing
reference mechanism, its adoption would require that:

* Providers implement the support for it.
* RBAC permissions need to be relaxed, if provider `A` needs to refer values from
  instances owned by provider `B`. In case of provider families, each provider has
  enough rights to access all resources within the family, requiring no changes
  in RBAC rules. Adding new rules should be done at deployment, by tweaking RBAC
  rules for the given provider service account.

### Referable Type

If adding a number of RBAC rules is not an option for users, cross-provider
referencing could be supported by introducing a new cluster-scoped `Referable` 
(alternative name could be `InjectableValue`) type whose instances declare values
that can be assigned to an object field:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: ref-source-location
spec:
  source:
    name: source-location
    apiVersion: datasync.aws.upbound.io/v1beta1
    kind: LocationS3
    fieldPath: metadata.annotations[crossplane.io/external-name]
```

The syntax of the `fieldPath` field is the one used in Composition, in line with
the [Kubernetes API Conventions on field selection].

The change in RBAC rules is very limited - only the read permission for `Referable`
type needs to be added for a given provider.

After deploying the above instance to the cluster, its reconciliation gets
triggered and once the referred object exists and the field is set, the
referable value is emitted in the object status part:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: ref-source-location
spec:
  source:
    name: source-location
    apiVersion: datasync.aws.upbound.io/v1beta1
    kind: LocationS3
    fieldPath: metadata.annotations[crossplane.io/external-name]
status:
  value: arn:aws:datasync:us-east-2:111222333444:location/loc-07db7abfc326c50aa # the value found in the requested field
  conditions:
    - type: Ready
      status: True
      reason: Available
```

Now, the value can be referred within a managed resource:

```yaml
apiVersion: datasync.aws.upbound.io/v1beta1
kind: Task
metadata:
  name: datasync-task-example
spec:
  forProvider:
    .
    .
    sourceLocationArnRef:
      name: ref-source-location
      apiVersion: apiextensions.crossplane.io/v1alpha1
      kind: Referable
      fieldPath: status.value
```

Of course, it should be possible to declare a source for the `Referable` 
using labels:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Referable
metadata:
  name: ref-source-location
spec:
  source:
    matchingLabels:
      foo: bar
    apiVersion: datasync.aws.upbound.io/v1beta1
    kind: LocationS3
    fieldPath: metadata.annotations[crossplane.io/external-name]
```

### Composition

Instead of declaring resources with [observe only management policy], we would
like to enable adding values from external resources to the environments,
similar to how environment configs are referred.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
# Removed for Brevity
spec:
  environment:
    environmentConfigs:
      - type: Reference
        ref:
          name: source-location
          apiVersion: datasync.aws.upbound.io/v1beta1
          kind: LocationS3
          fromFieldPath: metadata.annotations[crossplane.io/external-name]
          toFieldPath: source.location # path where the value is inserted into the environment

  resources:
  # Removed for Brevity
```

or if the name of resource is unknown/irrelevant, we can use selectors:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
# Removed for Brevity
spec:
  environment:
    environmentConfigs:
      - type: Selector
        selector:
          matchLabels:
            foo: bar
          apiVersion: datasync.aws.upbound.io/v1beta1
          kind: LocationS3
          fromFieldPath: metadata.annotations[crossplane.io/external-name]
          toFieldPath: source.location # path where the value is inserted into the environment

  resources:
  # Removed for Brevity
```

After initializing the environment, an appropriate patch & transformation
strategy can be applied to set managed resource fields.

__**Out of proposal scope**__: renaming `environmentConfigs` field to something
more generic, e.g. `sources`.

### Advantages

* All referred values are still declared on managed resources using already
  familiar mechanism
* Optional `Referable` instances can be watched and used by non-Crossplane controllers as
  well
* Avoid system pollution with [observe only management policy] objects
* Using compositions, reference could be set even if provider does not support
  the proposed reference mechanism

### Disadvantages

* Support for `Referable` type requires new controller on Crossplane side
* Provider RBAC rules should be extended to include read permission 
  for `Referable` type
* Providers needs to support the new reference mechanism on their side

### Implementation

* Update [crossplane-runtime](https://github.com/crossplane/crossplane-runtime/)
  to support `GenericReference` and `GenericSelector` types
* Extend [`EnvironmentConfigs`](https://github.com/crossplane/crossplane/blob/master/apis/apiextensions/v1/composition_environment.go#L107-L123)
  to support referencing an arbitrary k8s object
* Adding new Crossplane controller for the `Referable` type

#### Crossplane-runtime Changes

* [APIResolver](https://github.com/crossplane/crossplane-runtime/blob/master/pkg/reference/reference.go#L280)
  should be enriched to support `GenericReference` and `GenericSelector`
  instances
* [ResolveReferences function generator](https://github.com/crossplane/crossplane-tools/blob/master/internal/method/resolver.go#L33)
  should be enriched to support generic references
* Patch [Upjet](https://github.com/upbound/upjet) to support `GenericReference`
  and `GenericSelector` field types

Finally, providers based on Upjet need to be regenerated so that they can expose
generic references to users for the fields where `1:N` relationships exist.

#### Composition Changes

`EnvironmentSourceReference` should be upgraded to:

```go
type EnvironmentSourceReference struct {
  // The name of the object.
  Name string `json:"name"`
  
  // ApiVersion of the referenced object.
  ApiVersion *string `json:"apiVersion"`
  
  // Kind of the referenced object.
  Kind *string `json:"kind"`
  
  // FieldPath of the value within the referenced object.
  FromFieldPath *string `json:"fromFieldPath"`
  
  // ToFieldPath of the value within the referenced object.
  ToFieldPath *string `json:"toFieldPath"`
}
```

Similar goes for `EnvironmentSourceSelector`:

```go
type EnvironmentSourceSelector struct {

  // Mode specifies retrieval strategy: "Single" or "Multiple".
  // +kubebuilder:validation:Enum=Single;Multiple
  // +kubebuilder:default=Single
  Mode EnvironmentSourceSelectorModeType `json:"mode"`
  
  // MaxMatch specifies the number of extracted EnvironmentConfigs in Multiple mode, extracts all if nil.
  MaxMatch *uint64 `json:"maxMatch,omitempty"`
  
  // SortByFieldPath is the path to the field based on which list of EnvironmentConfigs is alphabetically sorted.
  // +kubebuilder:default="metadata.name"
  SortByFieldPath string `json:"sortByFieldPath"`
  
  // MatchLabels ensures an object with matching labels is selected.
  MatchLabels []EnvironmentSourceSelectorLabelMatcher `json:"matchLabels,omitempty"`
  
  // ApiVersion of the referenced object.
  ApiVersion *string `json:"apiVersion"`
  
  // Kind of the referenced object.
  Kind *string `json:"kind"`
  
  // FieldPath of the value within the referenced object.
  FromFieldPath *string `json:"fromFieldPath"`
  
  // ToFieldPath of the value within the referenced object.
  ToFieldPath *string `json:"toFieldPath"`
}
```

Finally, the logic for constructing the environment should be updated.

#### `Referable` Controller

The controller is responsible for:

* Retrieving the declared object and the field value, based on the provided
  reference
* Exposing the retrieved value under `status.value`
* Marking the instance as ready

No additional RBAC rules are needed, since Crossplane already poses very broad
permissions within the cluster.

## Alternatives Considered

* [A previous design](https://github.com/crossplane/crossplane/pull/2385) was
  proposed for this topic.

  It embeds the schema similar to `GenericReference` type into a number of
  new `spec.patches.fromObject` fields in managed resources. The existing 
  ref and selector fields remains available in the schema, although they 
  cannot be used for usecases referred throughout this proposal. 
  The proposed reference mechanism enables patching any field, even if it 
  does not have counterpart ref and selector fields. Although powerful,
  it overlaps significantly with compositions.


* `Referable` instances could also contain the reference to the destination
  managed resource and its field.
    - Such approach would make the discovery of references for a given managed
      resource harder, i.e. some would be declared on MR itself, but some 
      other would be spread across of number `Referable` instances
    - Referable controller logic becomes more complex
    - It starts to look like an alternative to compositions, potentially
      confusing users
    - One more controller can now update a managed resource, increasing
      potential conflict rate
    - `Referable` instance could not be consumed by multiple managed resources
    - However, providers would not need the read access to `Referable` instances

[Cross resource referencing]: https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md

[Kubernetes API object reference]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#object-references

[Kubernetes API Conventions on field selection]: https://github.com/kubernetes/community/blob/744e270/contributors/devel/sig-architecture/api-conventions.md#selecting-fields

[observe only management policy]: https://docs.crossplane.io/knowledge-base/guides/import-existing-resources/#apply-the-observeonly-managementpolicy

[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime/
# Ignore changes

* Owners: Lovro Sviben (@lsviben)
* Reviewers: @turkenh, @negz
* Status: Accepted

## Background

When Crossplane manages a resource, it reconciles all the parameters
under  `spec.forProvider` to the external providers both during the
creation and subsequent updates to the managed resource. But in some cases,
a parameter of the external resource could change due to it being managed
outside Crossplane. In the current state, if that happens, Crossplane
will try to "fix" the situation, and update the parameter back to the one it
has in the `spec.forProvider` as it is designed to be the only source of truth.

Some examples of this case:
- [AutoScalingGroup DesireCapacity], where when using an autoscaler, the
  parameter gets changed externally.
- [DynamoDB Tables] with using autoscaling ([Policy] + [Target]). The
  autoscaler scales the read and write capacity, and then Crossplane resets
  it back to the `readCapacity` and `writeCapacity` defined on the Table,
  which effectively makes autoscaling useless.
- [NodeGroup] while using a node autoscaler. There is a race condition
  between crossplane and node autoscaler. Node autoscaler increases the
  `desiredSize` due to resource requirement but Crossplane decreases it to
  `desiredSize` set in composition.
- GCP [Node Pool], where when `management.autoUpgrade` is enabled. After
  creation, the provider late initializes `spec.forProvider.version`,
  which when the nodes get upgraded starts conflicting with the 
  external nodes version. This causes an endless loop of downgrades
  and upgrades.
- Tags are a use case that affects multiple resources. Many cloud users
  have external systems that add tags to resources for billing, tracking
  or similar purposes. This can conflict with the tags set in 
  `spec.forProvider.tags`.
- There is an use case with [AzureAD Group] where initially the group is
  created certain with `members` and `owners` but is then managed externally
  over time as members need to be added or removed. For now the
  workaround is to use terraform provider with [ignore_changes].

In these cases, it would make sense to make Crossplane ignore select
parameters during updates. Terraform addresses this use case with its
[ignore_changes] functionality.

The obvious way would be to just not set the field in the `spec.forProvider`.
However, due to [Late Initialization], some fields are being set in
`spec.forProvider` without any user input. Plus there are some cases
where the field is required/wanted upon creation, but not on updates.

So in this document, the aim will be to introduce a solution in Crossplane
for ignoring some managed resource parameters during updates.

## Goals

* Design a functionality that allows for ignoring select fields during
  resource updates.

## Proposal

Proposed solution is to rework the new [managementPolicies] feature which came
with the [ObserveOnly] feature and transform it into a set of enum values
representing what Crossplane should do with the managed resource.

- `Create` - Create the external resource using `spec.forProvider`
  and `spec.initProviders` fields.
- `Update` - Update the external resource using `spec.forProvider` fields.
- `Delete` - Delete the external resource when the managed resource is
  deleted.
- `Observe` - Update the `status.atProvider` to reflect the state of the
  external resource.
- `LateInitialize` - Update unspecified `spec.forProvider` fields to reflect
  the state of the external resource, typically with the defaults from the cloud
  provider.

This will allow users to fine-tune how Crossplane manages the external
resource, in a manner which is very explicit and easy to understand.

Some examples on how the management policies would work and how they would
replace the current `managementPolicies` and `deletionPolicy`:

```yaml
# Default
spec:
  managementPolicies: FullControl
  deletionPolicy: Delete

# would be replaced with:
spec:
  managementPolicies: ["Create", "Update", "Delete", "Observe", "LateInitialize"]
  # or
  managementPolicies: ["*"]

# ObserveOnly
spec:
  managementPolicies: ObserveOnly

# would be replaced with:
spec:
  managementPolicies: ["Observe"]

# OrphanOnDelete
spec:
  managementPolicies: OrphanOnDelete

# would be replaced with:
spec:
  managementPolicies: ["Create", "Update", "Observe", "LateInitialize"]

# pause can be achieved by setting managementPolicies to empty list instead of
# using the annotation
spec:
  managementPolicies: []

# Turn off late initialization
spec:
  managementPolicies: ["Create", "Update", "Delete", "Observe"]
```

In addition to the new management policy, we will also add a new field
`spec.initProvider` which would contain parameters that should only be
used at the creation time. This would allow users to specify fields that
should be ignored on updates, but still be used during creation.

## Implementation

### Management policy

The new management policy would be implemented in the `crossplane-runtime`.
Most of the work will be in the [Managed Reconciler], where we will need to add
management policy checks to sections of code where we create, update etc.

For example:
- Create - [Create Section]
- Update - [Update Section]
- Delete - [Delete Section]
- Late Init - [Late Init Section]
- Observe - this one is tricky because we still need to observe/get a resource,
  so we just need to make sure that we don't update `status.atProvider`.

### Deletion policy

`deletionPolicy` was planned to be deprecated in favour of the new management
policies according to [the ObserveOnly design doc.][ObserveOnly], but still
retain some functionality if a non-default value was set. In practice, it
meant that if the `deletionPolicy` was set to `Orphan`, and the
`managementPolicies` set to `FullControl`, the external resource would be
orphaned.

In the new design, we could still follow this approach, by orphaning the
resource even if the `managementPolicies` includes `Delete`, if the
`deletionPolicy` is set to `Orphan`, until we entirely remove the deletion
policy.

Keep in mind that the `deletionPolicy` still keeps its full functionality if the
management policy alpha feature is not enabled.

### ["Create", "Update", "Delete", "Observe", "LateInitialize"] or ["*"] by default

Except for the cosmetic value, this decision affects how future additions to
the management policy would be handled. If we go with the `["*"]` approach,
every new management policy would be automatically included in the resources,
which may be what we want. Similarly, if we go with the `["Create", "Update",
"Delete", "Observe", "LateInitialize"]` approach, every new management policy
would need to be added manually or through a migration.

So this is something to keep in mind when developing new management policies.
For this proposal, we will go with the `["*"]` approach as its more
future-proof.

### Migrating existing resources

The `managementPolicies` feature is alpha, so it should be ok to break the
API. The combinations of `managementPolicies` and `deletionPolicy` would look
like this in the new `managementPolicies` field.

| managementPolicies | deletionPolicy | new managementPolicies                              |
|------------------|----------------|---------------------------------------------------|
| FullControl      | Delete         | ["*"]                                             |
| FullControl      | Orphan         | ["Create", "Update", "Observe", "LateInitialize"] |
| OrphanOnDelete   | Delete         | ["Create", "Update", "Observe", "LateInitialize"] |
| OrphanOnDelete   | Orphan         | ["Create", "Update", "Observe", "LateInitialize"] |
| ObserveOnly      | Delete         | ["Observe"]                                       |
| ObserveOnly      | Orphan         | ["Observe"]                                       |

As this will be a breaking change, if users want to keep the old
`managementPolicies` behaviour, we suggest pausing the reconciliation of the MR,
upgrading Crossplane, and then updating the `managementPolicies` to the desired
value before unpausing the reconciliation.

In reality this is only needed for the `ObserveOnly` and
(`OrphanOnDelete` + `Delete`) combinations, as the `FullControl` as in other
cases the default new management policy (`["*"]`) won't change the behaviour.

### initProvider

The idea here was to have a field that would be used only during creation,
called `initProvider`. The `initProvider` field would have the same schema
as `forProvider`. The `initProvider` fields would be merged with the
`forProvider` fields during the `Create` step of the managed resource
reconciliation loop. Fields that are specified in both `initProvider` and
`forProvider` give precedence to the `forProvider` fields.

```yaml
apiVersion: eks.aws.crossplane.io/v1alpha1
kind: NodeGroup
metadata:
  name: my-group
spec:
  initProvider:
    scalingConfig:
      desiredSize: 1
  forProvider:
    region: us-east-1
    scalingConfig:
      maxSize: 5
      minSize: 1
```

This would allow users to specify fields that would only be used on creation,
but not on updates.

When it comes to the implementation, the handling would all need to be done
on the provider level. The `initProvider` fields should only be used in the
`Create` step of the managed resource reconciliation loop.

Support for `initProvider` would need to be a manual change in each provider.
From our side we could provide a helper function to merge the `initProvider`
and `forProvider` fields which would be used in the `Create` step.

We know that for now the `initProvider` field would be useful just for a 
handful of cases that we are aware of. So provider developers do not need to
support `initProvider` for all resources right away. Rather, they can add
support for those resources that are known to require it, and later on based
on a request basis.

### Feature Gating

The new management policy would use the same feature gate already introduced
by `ObserveOnly`. So it would be an alpha feature that will be off by default
and will be controlled by `--enable-alpha-management-policies` flag in
Providers. It will also enable the `initProvider` feature.

### Examples how the new functionality would affect the use cases

Some use cases mentioned in the beginning of the document would be
solved just by omitting the management policy `LateInitialize`. For
others, the `initProvider` field would need to be used, mostly because
the fields in question are required on creation.

#### AutoScalingGroup

The `desiredCapacity` is not required on creation, but populated by late
initialization. It can be solved just by omitting `Late Initialize` management
policy.

```yaml
spec:
  managementPolicies: ["Create", "Update", "Delete", "Observe"]
  forProvider:
    maxSize: 5
    minSize: 1
    launchConfigurationNameRef:
      name: sample-launch-config
    ...
```

#### DynamoDB Table

The `readCapacity` and `writeCapacity` are not required, but they are required
if the `billingMode` is set to `PROVISIONED`. So the `initProvider` field
would need to be used alongside omitting `LateInitialize` management policy.

```yaml
spec:
  managementPolicies: ["Create", "Update", "Delete", "Observe"]
  initProvider:
    readCapacity: 1
    writeCapacity: 1
  forProvider:
    billingMode: PROVISIONED
...
```

#### EKS NodeGroup

The `scalingConfig.desiredSize` is required so `initProvider` would need
to be used alongside omitting `LateInitialize` management policy. This way
the autoscaler would be able to control the `desiredSize` after creation.

```yaml
spec:
  managementPolicies: ["Create", "Update", "Delete", "Observe"]
  initProvider:
    scalingConfig:
      desiredSize: 1
  forProvider:
    region: us-east-1
    scalingConfig:
      maxSize: 5
      minSize: 1
```

#### GCP Node Pool

Just omitting the `LateInitialize` management policy would be enough as the
`version` is not required on creation.

```yaml
spec:
  managementPolicies: ["Create", "Update", "Delete", "Observe"]
  forProvider:
  ...
```

#### Azure AD Group

The use case where `members` and `owners` want to be set on creation and
then ignored on updates would be solved by using `initProvider` alongside
omitting the `LateInitialize` management policy.

Example:
```yaml
spec:
  managementPolicies: ["Create", "Update", "Delete", "Observe"]
  initProvider:
    members:
      - user1
      - user2
    owners:
      - user3
  forProvider:
    displayName: my-group
    securityEnabled: true
```

#### Tags

Setting initial tags should work with `initProvider`. However, it seems that
the providers are setting some default tags in the into the `forProvider`
field in the [Initialize] step and are updating the resource, so this would
need to be changed to use `initProvider` instead, or we can check if those
tags are actually needed or just skip setting them similarly to how
ObserveOnly does it.

Ref: [Upjet Initialize] or [AWS community provider tag example].

## Solution alternative considered

### PartialControl management policy + initProvider

Proposed solution is to use the new [managementPolicies] field which came with
the [ObserveOnly] feature and add a new management policy that will
skip late initialization. The loss the information that the
late initialization was providing would be offset by the `status.atProvider`
which contains the resource state on the provider side and is being
added alongside [ObserveOnly].

In addition to the new management policy, we will also add a new field
`spec.initProvider` which would contain parameters that should only be
used at the creation time. This would allow users to specify fields that
should be ignored on updates, but still be used during creation. This can
be added as a separate feature.

#### Management policy

Proposed name for the new management policy is `PartialControl`, which
would fit with the current `FullControl` and insinuate that the resource
is not fully managed by Crossplane. Some other proposals for the naming
are: `ExplicitControl`, `SelectiveControl`, `IgnoreLateInit`.

The new management policy would be implemented in the `crossplane-runtime`
and would just ignore the [ResourceLateInitialize] condition and
skip the managed resource update [here][Late Initialization Update] if
the management policy is set to `PartialControl`.

#### Deletion policy

`DeletionPolicy` was planned to be deprecated in favour of the new management
policies according to [the ObserveOnly design doc.][ObserveOnly] However,
with `PartialControl` it's not clear if it should delete or orphan the
external resources. So to avoid creating `PartialControl` and
`PartialControlOrphanOnDelete` management policies, we will keep the
`DeletionPolicy` to be able to specify the behaviour. In that case, we will
need to remove the `OrphanOnDelete` management policy as it won't be needed
anymore. As it was an alpha feature, this should not be a problem, but we
should still make sure to communicate this change to the users.

So the management policies would be:
- `FullControl`
- `PartialControl`
- `ObserveOnly`

#### initProvider

The idea is the same as in the proposal.

#### Examples how the new functionality would affect the use cases

Some use cases mentioned in the beginning of the document would be
solved just by setting the new management policy to `PartialControl`. For
others, the `initProvider` field would need to be used, mostly because
the fields in question are required on creation.

#### AutoScalingGroup

The `desiredCapacity` is not required on creation, but populated by late
initialization. It can be solved just using the `PartialControl` management
policy.

```yaml
spec:
  managementPolicies: PartialControl
  forProvider:
    maxSize: 5
    minSize: 1
    launchConfigurationNameRef:
      name: sample-launch-config
...
```

#### DynamoDB Table

The `readCapacity` and `writeCapacity` are not required, but they are required
if the `billingMode` is set to `PROVISIONED`. So the `initProvider` field
would need to be used alongside `PartialControl` management policy.

```yaml
spec:
  managementPolicies: PartialControl
  initProvider:
    readCapacity: 1
    writeCapacity: 1
  forProvider:
    billingMode: PROVISIONED
...
```

#### EKS NodeGroup

The `scalingConfig.desiredSize` is required so `initProvider` would need
to be used alongside `PartialControl` management policy. This way the
autoscaler would be able to control the `desiredSize` after creation.

```yaml
spec:
  managementPolicies: PartialControl
  initProvider:
    scalingConfig:
      desiredSize: 1
  forProvider:
    region: us-east-1
    clusterNameRef:
      name: sample-cluster
    subnetRefs:
      - name: sample-subnet1
    nodeRoleRef:
      name: somenoderole
    scalingConfig:
      maxSize: 5
      minSize: 1
```

#### GCP Node Pool

Just using the `PartialControl` management policy would be enough as the
`version` is not required on creation.

```yaml
spec:
  managementPolicies: PartialControl
  forProvider:
  ...
```

#### Azure AD Group

The use case where `members` and `owners` want to be set on creation and
then ignored on updates would be solved by using `initProvider` alongside
`PartialControl` management policy.

Example:
```yaml
spec:
  managementPolicies: PartialControl
  initProvider:
    members:
      - user1
      - user2
    owners:
      - user3
  forProvider:
    displayName: my-group
    securityEnabled: true
```

#### Tags

Setting initial tags should work with `initProvider`. However, it seems that
the providers are setting some default tags in the into the `forProvider`
field in the `Initialize` step and are updating the resource, so this would
need to be changed to use `initProvider` instead, or we can check if those
tags are actually needed or just skip setting them similarly to how
ObserveOnly does it.

Ref: [Upjet Initialize] or [AWS community provider tag example].

### ignoreChanges field

The idea is to introduce a new `ignoreChanges` string array field in the
`spec` of the managed resource, that would work similarly
to Terraform's [ignore_changes]. The items in the array would be the field
paths of the fields that should be ignored on updates.

```yaml
apiVersion: eks.aws.crossplane.io/v1alpha1
kind: NodeGroup
metadata:
  name: my-group
spec:
  ignoreChanges:
    - scalingConfig.desiredSize
  forProvider:
    region: us-east-1
    clusterNameRef:
      name: sample-cluster
    subnetRefs:
      - name: sample-subnet1
    nodeRoleRef:
      name: somenoderole
    scalingConfig:
      desiredSize: 1
      maxSize: 5
      minSize: 1
    updateConfig:
      maxUnavailablePercentage: 50
      force: true
  providerConfigRef:
    name: example
```

The solution should affect just the Update part of the Crossplane lifecycle,
so create and late initialization should work as before, even though a
field is marked under `ignoreChanges`. Additionally, as Crossplane will soon
support `status.atProvider`, users will be able to observe the actual values
of the ignored fields.

#### Ignoring required fields

As there is no standard to distinguish which required fields are
required for create and update and which just for creat, we can only
advise to use caution when putting required fields into `ignoreChanges`.
If such a need arises, it should preferably be handled as a special case
on the provider side, and not through the `ignoreChanges` feature.

#### Implementation

The implementation would be done on the provider level, so we would need to
update all the providers over time to use the new field.

For providers using Upjet the solution will be to leverage Terraform's
[ignore_changes] lifecycle field. So Upjet generation should be updated
with a step that transfers the `spec.ignoreChanges` field into Terraform's
`ignore_changes`. Something like this in the Upjet method that writes the
main TF file:

```go
// WriteMainTF writes the content main configuration file that has the desired
// state configuration for Terraform.
func (fp *FileProducer) WriteMainTF() error {

    fp.parameters["lifecycle"] = map[string]interface{}{
        // If the resource is in a deletion process, we need to remove the
        // deletion protection.
        "prevent_destroy": !meta.WasDeleted(fp.Resource),
        // Add fields which should be ignored on updates.
        "ignore_changes": fp.Resource.GetIgnored(),
    }
...
```

For other providers we can implement a helper function in the
[crossplane-runtime] repo that would use the existing code in
[crossplane-runtime/fieldpath] to unset the fields of a managed resource
that are under `ignoreChanges`. That way provider maintainers could use this
function in the Update method without the need to invent their own solution.

Helper function would look something this (WIP):
```go
func UnsetIgnored(managed resource.Managed) error {
	p, err := PaveObject(managed)
	if err != nil {
		return err
	}
	
	for _, s := range managed.GetIgnored() {
		err = p.DeleteField(s)
		if err != nil {
			return err
		}
	}
	return nil
}
```

#### Feature Gating

Similar to all other new features being added to Crossplane, we will ship this
new policy as an alpha feature that will be off by default and will be
controlled by the `--enable-alpha-ignore-changes` flag in Providers.

This will not prevent the field from appearing in the schema of the managed
resources. However, we will ignore the `spec.ignoreChanges` when the feature
is not enabled.

#### Alternative feature gating approach

To avoid changing the API, we could ship the `ignoreChanges` field as an
optional annotation as a first solution. This would allow us to survey
users and see how much this feature is useful before adding it to the API.

#### Why this solution was not chosen

This solution was originally chosen, but it was later decided that it would
be better to have a clear distinction of what is being reconciled to the
external provider and what is not. Imagine a case where a field is both set
in `spec.forProvider` and `spec.ignoreChanges`. This could happen as an
effect of late initialization or if the user sets the field in both places.
In that case, the field would be ignored on updates, but could cause confusion
as it would still be in the`spec.forProvider` field. Of course, we could also
introduce something like `PartialControl` management policy that turns off
late init to minimize that case.

That's where the `initProvider` solution has a slight advantage, as it clear
that the field is just used on create and not on updates. Therefore, we can
keep `spec.forProvider` as the source of truth for the rest of the resource
lifecycle.

All in all, this solution is still valid and the decision to go with
the `initProvider` is because it seems it would bring less confusion to the
users.

### Case by case

This approach would just tackle the ignored fields on a case-by-case basis,
instead of implementing a generic solution. The premise here is that not many
resources would need to use this feature.

For instance in the [NodeGroup] example, we would add a field `initialSize`,
along with the current `desiredSize` and the updates could be handled in code.

#### Why this solution was not chosen

While this approach is valid, making changes case-by-case could lead to 
bad user experience as they would need time to identify the issue and wait 
for provider code change, instead of just setting a field. On the other hand,
if this issue is not that widespread, we could have an easy fix.

[AutoScalingGroup DesireCapacity]: https://doc.crds.dev/github.com/upbound/provider-aws/autoscaling.aws.upbound.io/AutoscalingGroup/v1beta1@v0.21.0#spec-forProvider-desiredCapacity
[DynamoDB Tables]: https://doc.crds.dev/github.com/upbound/provider-aws/dynamodb.aws.upbound.io/Table/v1beta1@v0.24.0
[Policy]: https://doc.crds.dev/github.com/upbound/provider-aws/appautoscaling.aws.upbound.io/Policy/v1beta1@v0.24.0
[Target]: https://doc.crds.dev/github.com/upbound/provider-aws/appautoscaling.aws.upbound.io/Target/v1beta1@v0.24.0
[NodeGroup]: https://doc.crds.dev/github.com/upbound/provider-aws/eks.aws.upbound.io/NodeGroup/v1beta1@v0.24.0
[Node Pool]: https://doc.crds.dev/github.com/upbound/provider-gcp/container.gcp.upbound.io/NodePool/v1beta1@v0.28.0
[AzureAD Group]: https://marketplace.upbound.io/providers/upbound/provider-azuread/v0.5.0/resources/groups.azuread.upbound.io/Group/v1beta1
[Late Initialization]: https://docs.crossplane.io/v1.11/concepts/managed-resources/#late-initialization
[Managed Reconciler]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go
[Create section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L943-L1031
[Delete section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L865-L922
[Update section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L1061-L1096
[Late Init section]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L1033-L1046
[Initialize]: https://github.com/crossplane/crossplane-runtime/blob/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/reconciler/managed/reconciler.go#L742
[managementPolicies]: https://github.com/crossplane/crossplane-runtime/blob/229b63d39990935b8130cf838e6488dcba5c085a/apis/common/v1/policies.go#L21
[ObserveOnly]: https://github.com/crossplane/crossplane/blob/019ddb55916396d654e53a86d9acf1cde49aee31/design/design-doc-observe-only-resources.md
[ResourceLateInitialize]: https://github.com/crossplane/crossplane-runtime/blob/00239648258e9731c274fb1f879f8255b948c79a/pkg/reconciler/managed/reconciler.go#L1033
[Late Initialization Update]: https://github.com/crossplane/crossplane-runtime/blob/00239648258e9731c274fb1f879f8255b948c79a/pkg/reconciler/managed/reconciler.go#L1033
[ignore_changes]: https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle#ignore_changes
[Upjet Initialize]: https://github.com/upbound/upjet/blob/645d7260d814cb67db2280e92988051d30774a09/pkg/config/resource.go#L227-L249
[AWS community provider tag example]: https://github.com/crossplane-contrib/provider-aws/blob/37542c0fbb1f83f1fc18a099393bba18fddecc1d/pkg/controller/dynamodb/table/hooks.go#L142C78-L166
[crossplane-runtime]: https://github.com/crossplane/crossplane-runtime/
[crossplane-runtime/fieldpath]: https://github.com/crossplane/crossplane-runtime/tree/1316ae6695eec09cf47abdfd0bc6273aeaab1895/pkg/fieldpath
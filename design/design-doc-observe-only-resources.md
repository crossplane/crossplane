 # Observe Only Resources

* Owners: Hasan Turken (@turkenh)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

When using Crossplane to manage resources, we typically create a Managed
resource that represents the desired state of the resource at a provider.
Crossplane takes ownership of the resource, starts acting as the source of
truth, and ensures configuration matches the desired state.

Sometimes, we want to "observe" an existing resource without taking ownership of
it. This could be useful in several scenarios, which could be grouped as
follows:

- Referencing existing resources without managing them
  - In your composition, you want to reference network resources like VPC and
  subnets that are managed by another tool or team.
- Fetching data from existing resources
  - You need information about an existing VPC, such as its CIDR range and the
  subnets it contains.
- Gradual migration of existing/legacy infrastructure to Crossplane
  - You have existing infrastructures managed by Terraform, and you want to
  migrate them gradually to Crossplane.
  - You have a legacy infrastructure that you want to migrate to Crossplane,
  but you want to experiment the managed resources before taking ownership of
  the underlying resources.
  - For an existing resource, you don’t want to provide full configuration
  spec that might override the actual configuration. You want to late-initialize
  all fields, including the ones that would be required otherwise.
- Only observing some fields after the initial creation
  - You want to create an EKS Node Group with a scaling configuration where you
  configured an initial desired size. After the creation, you want to only
  observe changes in the size which is now being controlled by the cluster
  autoscaler.

Currently, Crossplane does not have a built-in way of observing resources
without taking ownership of them. There are two workarounds used by the
community as an interim solution for this gap:

- Using the provider-terraform to observe resources with the help of Terraform
data sources.
- Wrapping resources with provider-kubernetes to observe resources managed by
Crossplane but as a shared object between multiple Compositions.

In this document, we aim to introduce a solution to observe resources with
Crossplane without taking ownership of them.
This would allow users to integrate existing cloud resources with the Crossplane
ecosystem without giving full ownership.

## Goals

- Introduce a way to observe existing resources without taking ownership of them
in Crossplane.
- Enable seamless integration of existing cloud resources with the Crossplane
ecosystem.

### Non-goals

- Partially managing a resource by observing a subset of fields.

This may seem similar to the concept of observing resources, but there is a
fundamental difference. In this scenario, we want to use certain parameters
during the creation of the resource, whereas observing resources is intended to
be a completely read-only operation that should never make any changes to the
external system, including during the creation process.

## Proposal

To support observing resources without taking ownership, we will introduce a new
spec named `managementPolicy` to the Managed Resources. We will also deprecate
the existing `deletionPolicy` in favor of the new spec since they will be
controlling the same behavior; that is, how should the changes on the CR affect
the external cloud resource.

This new policy will have the following values:

- `FullControl`(Default): Crossplane will fully manage and control the external
resource, including deletion when the CR is deleted
(same as `deletionPolicy: Delete`).
- `ObserveCreateUpdate`: Crossplane will observe and perform create and update
operations on the external resource but will not make any deletions
(same as `deletionPolicy: Orphan`).
- `ObserveOnly`: Crossplane will only observe the external resource and will not
make any changes or deletions.

Please note while `ObserveCreateUpdate` may sound verbose, it accurately
describes the actions that Crossplane will take on the external resource. This
naming convention also focuses on what Crossplane will do, rather than what it
won't do, making it more intuitive and extensible for future policy options,
such as `ObserveDelete`.

As indicated above, `FullControl` and `ObserveCreateUpdate` policies will behave
precisely the same as the deletion policies we have today, including keeping the
default behavior the same. We will introduce the new behavior with the
`ObserveOnly` option, which would be pretty similar to what we have today to
[import existing managed resources], but instead of starting to manage after
import, we will not make any modifications to the external resource and only
sync status back.

```yaml
apiVersion: ec2.aws.crossplane.io/v1beta1
kind: VPC
metadata:
  annotations:
    crossplane.io/external-name: vpc-12345678
  name: observe-vpc
spec:
  managementPolicy: ObserveOnly
  forProvider:
    region: us-east-1
```

### Implementation

Crossplane providers already manage external resources by implementing the
Crossplane runtime's ExternalClient interface, which includes the four methods
listed below.

```go
type ExternalClient interface {
	Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error)
	Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error)
	Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error)
	Delete(ctx context.Context, mg resource.Managed) error
}
```

We will leverage the fact that we have an already implemented Observe method for
all managed resources by calling only it when the Management Policy is set to
`ObserveOnly`. This will require minor modifications in the Managed Reconciler
code that will return early in the reconcile loop and prevent invocation of the
other methods, namely, Create, Update and Delete. These modifications will
implement the following logic at a high level:

Right after the `Observe` method invocation, if `ObserveOnly`:

- Return error if the resource does not exist
- Publish connection details
- Call `client.Update` if the resource was late initialized
- Report success and return early

#### Feature Gating

Similar to all other new features being added to Crossplane, we will ship this
new policy as an alpha feature that will be off by default and will be
controlled by `--enable-alpha-management-policies` flag.

This will not prevent the field from appearing in the schema of the managed
resources. However, we will ignore the `spec.managementPolicy` when the feature
is not enabled.

#### Deprecation of `deletionPolicy`

With the new `managementPolicy` covering the existing `deletionPolicy`, we will
deprecate the latter in favor of the former.

Until we drop the `deletionPolicy` from the schema altogether, we need to be
careful with the conflicting combinations shown in the below table which only
exists with deletion:

| Deletion Policy | Management Policy | Should Observe? | Should Create? | Should Update? | Should Delete? |
| --- | --- | --- | --- | --- | --- |
| Delete | Full | Yes | Yes | Yes | Yes |
| Orphan | ObserveCreateUpdate | Yes | Yes | Yes | No |
| Delete | ObserveOnly | Yes | No | No | Conflict (No) |
| Orphan | Full | Yes | Yes | Yes | Conflict (No) |
| Delete | ObserveCreateUpdate | Yes | Yes | Yes | Conflict (No) |
| Orphan | ObserveOnly | Yes | No | No | No |

For conflicting cases, we will decide based on the non-default configuration
which means "not deleting the external resource" for all 3 conflicting cases.
This way, we will also err on the side of caution by leaving the actual resource
untouched, avoiding any accidental deletion or modification.

> Another solution could be simply throwing an error and preventing
> reconciliation during conflict. This would be more explicit but would require
> some manual actions and degraded UX for the usage of the feature, for example:
> 
> - Creating an ObserveOnly resource will require both setting `managementPolicy`
> to `ObserveOnly` and `deletionPolicy` to `Orphan` .
> - If there are existing resources with `deletionPolicy: Orphan` when the feature
> is enabled, they will start failing to reconcile until their
> `managementPolicy`’s updated to `ObserveCreateUpdate`.

#### Schema Changes

The proposed approach here involves utilizing the same CR, hence schema, for
both managing and observing resources. The caveat here is that some fields are
required for creating the resources but not for observing. This means that it
won’t be possible to create an observe-only resource without providing a value
for any required fields, because the Kubernetes API checks for the presence of
these fields before allowing the CR to be created.

> Please note this is already the case with [importing existing managed resources]
> today.

We will fix this by leveraging the [Common Expression Language (CEL)], which was
graduated to beta (i.e. enabled by default) as of Kubernetes 1.25. See the
following diff for the required changes we need for making [CIDRBlock] parameter
of AWS VPC required only if not `ObserveOnly`:

```diff
		// CIDRBlock is the IPv4 network range for the VPC, in CIDR notation. For
        // example, 10.0.0.0/16.
-       // +kubebuilder:validation:Required
        // +immutable
-       CIDRBlock string `json:"cidrBlock"`
+       CIDRBlock *string `json:"cidrBlock,omitempty"`

        // The IPv6 CIDR block from the IPv6 address pool. You must also specify Ipv6Pool
        // in the request. To let Amazon choose the IPv6 CIDR block for you, omit this
@@ -170,6 +169,7 @@ type VPC struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

+       // +kubebuilder:validation:XValidation:rule="self.managementPolicy == 'ObserveOnly' || has(self.forProvider.cidrBlock)",mess
age="cidrBlock is a required parameter"
        Spec   VPCSpec   `json:"spec"`
        Status VPCStatus `json:"status,omitempty"`
 }
```

## Future Work

### Querying and Filtering

Querying and filtering cloud resources is another common use case that could be
relevant to making an observation. Terraform uses [Data Sources] to observe
existing resources by supporting some querying and filtering with a set of
parameters specific to data source type. One can find and fetch data for the
[most recent AMI] and a VPC with [desired tags].

We will not support querying and filtering at *managed resources level* since it
violates a fundamental principle with the managed resources, that is, having a
one-to-one relationship between a managed resource and the external resource
that it represents. When it comes to querying and filtering, it is possible
that:

- There are more than one matching resources
- There are no matching resources
- The matching resource may change in time, e.g., most-recent AMI

Hence, we will leave implementing this functionality *in an upper layer*, which
in turn will own managed resources with `managementPolicy: ObserveOnly`. In this
model, it is totally fine if matching resources change in time, including having
more than one or no matches where we would expect corresponding managed
resources to come and go at runtime.

We have two options to implement this functionality:

**Option A: Introduce a new resource type `Query Resource`:**

- This will no longer be a Managed Resource but a new type positioned on top of
it, which will own and manage the lifecycle of Observe Only Managed Resources.
- They will have their own kind and schema, e.g., to query/filter VPCs; we will
have a `VPCQuery` resource.
- Each provider implements Query Resources per type.
- Leverages existing mechanisms in the provider (secret, IRSA, workload
identity, etc.) to authenticate to the Cloud API.

<img src="images/observe-only-query-resource.png" width="750" />


**Option B: Defer this to the Composition layer, specifically, Compositions Functions:**

- Compositions already operate as an upper layer by owning and managing the
lifecycle of managed resources.
- Querying and filtering are more like an imperative action that does not change
the state of the external world and could be considered as part of auxiliary
actions for compositing the infrastructure.
- Authentication to the Cloud APIs is a problem that needs to be solved which is
the biggest caveat of this approach. In the first pass of the composition
functions design, even [passing sensitive configuration] to functions is not
covered yet, and we would eventually need support for other authentication
mechanisms.

We expect a composition like the following to output an `Observe Only` managed
VPC that could be referenced by other composed resources.

```yaml
apiVersion: apiextensions.crossplane.io/v2alpha1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance
  functions:
    - name: query-aws
      type: Container
      container:
        image: xkpg.io/query-aws:0.1.0
        # We need to access AWS API to make the queries. 
        network: Accessible
      config:
        apiVersion: query.aws.upbound.io/v1alpha1
        kind: VPC
        metadata:
          name: find-default-vpc
        spec:
          region: us-east-1
          default: true
```

Both options have some pros and cons and there could also be other options like
combining both approaches, e.g. once/if a [`type: Webhook` composition function]
supported, providers could expose an API and functions may leverage them to make
Cloud API calls.

For now, we want to leave this open as a future work until we get composition
functions feature landed and matured a bit. In the meantime, we can focus on
implementing the management policy and support Observe Only resources as
proposed and collect more ideas on the best possible solution for querying and
filtering.

## Alternatives Considered

### **Dedicated types that only Observe**

This was about introducing a new kind with a dedicated schema that only observes
existing resources. This would be closer to the Terraform's [Data Sources] where
they have a separate type for fetching data from external resources.

If we don't want to support querying and filtering, this approach would not add
much more value than the proposed approach other than being more explicit
(i.e. a `VPCObservation` kind vs `VPC` with `managementPolicy: ObserveOnly`) at
the cost of doubling the number of CRDs. Another possible advantage is having
dedicated schemas for observation types which in turn having less fields than
the managed resource type which could provide a better UX for the users.

Supporting querying and filtering by leveraging dedicated schemas (e.g. we could
have a `mostRecent: true` field which does only make sense for an Observation
type) would add some real value compared to the proposed approach. However,
this wouldn't fit well with the current definition of a _Managed Resource_ where
we always have a one-to-one relationship between a managed resource and the
external resource that it represents. Careful readers may have noticed that the
first option _Querying and Filtering_ above (Option A) is quite similar to this
approach. However, instead of creating and owning a Managed Resource, the
resulting data would be at the status of the Observation Resource. In this case,
we would not be able to use the existing resource referencing mechanism, and we
would lose the benefits of having a one-to-one relationship such as leveraging
it as a migration path to Crossplane.

[import existing managed resources]: https://docs.crossplane.io/v1.10/concepts/managed-resources/#importing-existing-resources
[Common Expression Language (CEL)]: https://kubernetes.io/blog/2022/09/23/crd-validation-rules-beta/
[CIDRBlock]: https://github.com/crossplane-contrib/provider-aws/blob/ff84c3884b18befa693d87d37c51954b7f18903f/apis/ec2/v1beta1/vpc_types.go#L82
[Data Sources]: https://developer.hashicorp.com/terraform/language/data-sources
[most recent AMI]: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/ami#most_recent
[desired tags]: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/vpc#tags
[passing sensitive configuration]: https://github.com/crossplane/crossplane/pull/2886#discussion_r862615416
[`type: Webhook` composition function]: https://github.com/crossplane/crossplane/blob/master/design/design-doc-composition-functions.md#using-webhooks-to-run-functions

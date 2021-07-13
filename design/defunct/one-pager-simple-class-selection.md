# Simple Resource Class Selection
* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Background

In Crossplane 0.3 we introduced support for "provider specific" (aka
"non-portable", aka "[strongly typed]") resource classes. During the development
cycle [we identified] that if a claim must explicitly reference a provider
specific resource class in order to enable dynamic provisioning, then the claim
itself is now provider specific and no longer portable across providers. We
solved this issue by introducing [portable resource classes].

A portable resource class is effectively an indirection to a non-portable
resource class. Portable resource classes can be set as the default for a
particular namespace, and will be used by any resource claims of their
corresponding kind that do not specify a resource class for dynamic
provisioning, or a managed resource for static provisioning. This can be thought
of as "publishing" non-portable resource classes (which may exist in a distinct
namespace used to logically group infrastructure) to other namespaces, where
applications may use them to satisfy claims for their infrastructure
requirements.

This pattern is flexible and powerful, but Crossplane maintainers and [community
members] have observed that it's verbose; three distinct Kubernetes resources (a
claim, portable class, and non-portable class) must exist for dynamic
provisioning to occur. Community members have also [provided feedback] that
having to create portable resource classes for every new application namespace
can be onerous.

## Terminology

Most terminology is covered by the [Crossplane glossary]. Key terms used in this
document include:

* Crossplane uses a _provider_ to satisfy infrastructure needs. Google Cloud
  Platform (GCP), Elastic Cloud, and Kubernetes are all providers.
* A resource is said to be _portable_ if it is not tightly coupled to any one
  provider. Managed resources like `RDSInstance` are coupled to a provider
  because they expose provider specific configuration parameters, whereas
  resource claims like `PostgreSQLInstance` are portable because they do not.
* _Dynamic provisioning_  is the act of satisfying a _portable_ request for
  infrastructure (i.e. a resource claim) by automatically provisioning a
  non-portable managed resource.
* _Resource classes_ enable dynamic provisioning. A resource class determines
  what class of managed resource should be created to satisfy a resource claim.
* _Separation of concern_ happens when an opinionated party can publish classes of
  infrastructure to be consumed by parties unconcerned with such details.
  Imagine an organisation in which an infrastructure team supports many product
  teams who own the lifecycle of their infrastructure. Separation of concerns
  allows the infrastructure team to define a 'production database' class of
  service that happens to be a high availability, SSD backed GCP CloudSQL
  instance. Product teams can then self-service by provisioning a production
  database and trust that it will be configured appropriately.

Note that two forms (classes?) of resource class currently exist; see [portable
resource classes] for details.

## Goals

The goal of this proposal is to __make it easier for potential Crossplane users
to learn and take advantage of the portability and separation of concerns__ that
are enabled by dynamic provisioning. A _secondary_ goal is to maintain the
flexibility and power that Crossplane offers complex organisations with many
application and infrastructure namespaces.

## Proposal

This document proposes Crossplane retire the concept of portable resource
classes. Resource claims would instead be matched directly to non-portable
resource classes (referred to henceforth as simply 'resource classes') using
[label selectors]. Providers, managed resources, and resource classes would be
cluster scoped rather than namespaced.

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  namespace: acme-team
  name: gitlab-database
spec:
  classSelector:
    matchLabels:
      stack: gitlab
      grade: experimental
      region: us-east-1
```

Consider the hypothetical resource claim above, created by an equally
hypothetical GitLab [stack]. It declares a need for an experimental grade
PostgreSQL instance in region us-east-1 that is suitable for use with the GitLab
stack. Any compatible resource class (whether it be an `RDSInstanceClass` or a
`CloudSQLInstanceClass`) matching the resource claim's labels would be eligible
to satisfy it. If more than one such class existed one would be selected at
random.

This pattern is likely familiar to Kubernetes and to Crossplane users - it's
used to select which nodes a pod may run on, which pods are endpoints of a
service, and which Kubernetes cluster a Crossplane workload may run on.

### Infrastructure Scope

Crossplane 0.3 introduced the concept of an 'infrastructure namespace' - a
logical grouping of infrastructure resources such as providers, resource
classes, and managed resources. These namespaces might group infrastructure by
its geographical region, or its environment (e.g. "production"), distinct from
application namespaces. This potentially many to many application to
infrastructure namespace pattern - currently established by portable resource
classes - makes it difficult to mentally model the relationship between
applications and the infrastructure that supports them.

It is an uncommon pattern in Kubernetes to namespace infrastructure resources.
Existing infrastructure kinds such as nodes, persistent volumes, and storage
classes are cluster scoped concepts. This document proposes Crossplane Services
resources (with the exception of resource claims) become cluster scoped. This
provides a single, easy to find scope in which resource claim authors can
discover the resource classes available to them, and the managed resources
produced by their claim.

Under this proposal the logical groupings provided by infrastructure namespaces
that we frequently think of as “production”, “development”, or “east-coast” can
be modelled using labels. One thing we would lose is the ability to bind RBAC
policies to _groups_ of infrastructure; this [cannot be done using label
selectors]. Instead RBAC policy would only be applicable at kind (e.g. all
`RDSInstance` resources) or resource (e.g. the `RDSInstance` named `example`)
granularity.

### Exact Matching and Static Provisioning

Resource claims would retain their existing `.spec.classRef` field under this
proposal. In fact the `.spec.classSelector` would be used to set the `classRef`,
just like Crossplane's [workload scheduling]. Resource claim authors who desired
dynamic provisioning but had a specific resource class in mind could bypass the
label selection process by explicitly referencing a resource class, for example:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  namespace: acme-team
  name: gitlab-database
spec:
  classRef:
    apiVersion: database.gcp.crossplane.io/v1alpha2
    kind: CloudSQLInstanceClass
    name: gitlab-experimental
  writeConnectionSecretToRef:
    name: app-postgresql-connection
```

Static provisioning is unaffected by this proposal; setting the
`.spec.resourceRef` field of a resource claim would continue to bypass resource
classes entirely. The resource claim would attempt to bind to the explicitly
referenced managed resource.

### Rudimentary Scheduling

While not a goal of this proposal, it's worth noting that the label selector
pattern enables rudimentary scheduling of resource claims to resource classes.
Class and claim authors _could_ define a labelling schema that allowed claim
authors to describe the desired properties of their claim using labels. The
below example illustrates a labelling schema that could be used to describe a
`RedisCluster` claim that needs a replicated Redis 3.2 instance running on the
west coast of the USA:

```yaml
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  name: my-cache
spec:
  classSelector:
    matchLabels:
      engineVersion: "3.2"
      region: us-west
      replicated: "true"
```

These resource classes specify that they can satisfy claims for replicated Redis
3.2 instances on the west coast of the USA via their labels:

```yaml
apiVersion: cache.gcp.crossplane.io/v1alpha2
kind: CloudMemorystoreInstanceClass
metadata:
  name: gcp-dev
  labels:
    engineVersion: "3.2"
    region: us-west
    replicated: "true"
specTemplate:
  tier: STANDARD_HA
  region: us-west2
  memorySizeGb: 1
  redisVersion: REDIS_3_2
---
apiVersion: cache.azure.crossplane.io/v1alpha2
kind: RedisClass
metadata:
  name: azure-dev
  labels:
    engineVersion: "3.2"
    region: us-west
    replicated: "true"
specTemplate:
  resourceGroupName: group-westus-1
  location: West US
  sku:
    name: Basic
    family: C
    capacity: 0
```

Crossplane could (at some future time) solve the specific example above by
[teaching each resource claim controller] how to match standardised resource
claim fields to their provider specific resource class fields, but this is true
only for a very small class of fields that are both highly relevant to claim
authors and highly likely to translate to every conceivable managed resource
that could satisfy a claim. Geographic region, node count, and database size are
examples of such fields.

Label selectors approximate this functionality while it does not yet exist and
will compliment it when it does. There will likely be characteristics of
resource classes that claim authors want to match on that cannot be first class
configurable fields of the resource claim `spec` because they do translate to
all managed resources.

### Unopinionated Resource Claims

In the absence of spec based scheduling a resource claim that omits the
`.spec.classSelector` field, as well as the `.spec.classRef` and
`.spec.resourceRef` fields that allow them to explicitly specify a class or
existing managed resource respectively will be deemed unopinionated about what
resource class it needs.

Kubernetes [states that]:

> The semantics of empty or non-specified selectors are dependent on the
> context, and API types that use selectors should document the validity and
> meaning of them.

This document proposes that Crossplane fall back to a default resource class at
the cluster scope in order to satisfy an unopinionated resource claim. The
`resourceclass.crossplane.io/is-default-class: "true"` annotation (not label)
would indicate the default resource class. If there is no default resource class
the resource claim will not be satisfied and will remain unbound. If there are
multiple default resource classes one will be chosen at random.

## Technical Implementation

Under the hood, resource class selection for any resource class that specified a
`classSelector` (and did not specify a `classRef` or `resourceRef`) would be
implemented as a race between a series of "class scheduler" controllers.

In Crossplane there is not one controller for each resource claim kind, but
rather one controller for each possible (resource claim, managed resource)
tuple. This allows an infrastructure stack that adds support for a managed
resource to implement the dynamic provisioning and claim binding logic for said
managed resource without having to touch Crossplane core. Put otherwise, the GCP
stack can enable `CloudSQLInstance` resources to satisfy `MySQLInstance` claims
without teaching `crossplane/crossplane` how to dynamically provision a
`CloudSQLInstance`. This same pattern would be applied to class scheduler
controllers - one controller would be responsible for each possible (resource
claim, resource class) tuple.

In more detail:

* All resource claim controllers are updated to only reconcile resources with
  either a `resourceRef` or a `classRef` set.
* A class scheduler controller is introduced for each (resource claim, resource
  class) tuple. Its job is to set the `classRef` for resource classes that omit
  it (and omit `resourceRef`), either by using their `classSelector` or the
  default resource class.

Upon the creation of a `PostgreSQLInstance` without a `classRef` or
`resourceRef`:

1. Every class scheduler watching for `PostgreSQLInstance` claims has a
   reconcile queued. This example discusses the `(PostgreSQLInstance,
   CloudSQLInstanceClass)` scheduler , but `(PostgreSQLInstance,
   RDSInstanceClass)` and `(PostgreSQLInstance, MySQLServerClass)` schedulers
   would run through the process in parallel.
1. The scheduler lists all `CloudSQLInstanceClass` resources that matched the
   `PostgresSQLInstance` claim's label selectors.
1. If no `CloudSQLInstanceClass` matched the labels, the reconcile is done.
   Otherwise, one of the matching `CloudSQLInstanceClass` resources is selected
   at random.
1. The scheduler sleeps for a small, randomly jittered amount of time. This
   increases the randomness of the potential race between controllers to set the
   claim's `classRef`. Without this jitter it's more likely that one controller
   consistently wins the race, for example because it must list and "randomly
   choose" from only one matching resource class while other controllers must
   choose from many.
1. The scheduler sets the `classRef` of the `PostgreSQLInstance` to the selected
   `CloudSQLInstanceClass`. If two controllers try to set the `classRef` at the
   same time one will fail to commit the change due to the `PostgreSQLInstance`
   claim's resource version having changed since it was read.
1. The reconcile is done. With the `classRef` set the `PostgresSQLInstance` now
   passes the watch predicates of the `(PostgreSQLInstance, CloudSQLInstance)`
   resource claim reconciler, which dynamically provisions and binds it.

A similar process would be enacted when an unopinionated resource claim - one
that also omitted its `classSelector` - was encountered. This process would
implicitly select on the `resourceclass.crossplane.io/is-default-class: "true"`
annotation rather than labels. This is why Crossplane would not enforce that
only one default class existed and fall back to selecting one at random when
multiple existed; because any other behaviour would require coordination between
resource class controllers.

All portable resource class definitions and defaulting controllers would be
removed from Crossplane core.

## Future Considerations

The following are explicitly out of scope for this initial proposal, but may be
considered in future.

### Enforcing a Single Default Class

Under the proposal put forward by this document Crossplane would allow multiple
classes to be annotated as the default, and pick one at random if there were
more than one. This is a strategy informed by technical limitations rather than
desired user experience - it requires no coordination between claim scheduler
controllers. It this behaviour proved problematic in practice it should be
possible to enforce that only a single default resource class exist with little
coordination by employing a series of admission webhooks. When a resource class
of a particular kind was annotated as the default its admission webhook would
take the lease, alerting other admission webhooks for other resource class kinds
that they should not allow a default class to be set.

### Weighted Classes

It may be possible to improve upon the random scheduling of resource claims to
resource classes by supporting weighted random scheduling. Under such a proposal
resource classes would have a spec field or annotation specifying their weight,
with the highest weight winning. This could be used by infrastructure
administrators to ensure conservative (or affordable) behaviour in cases where a
resource claim matches multiple resource classes.

Consider the following resource claim:

```yaml
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  namespace: acme-team
  name: gitlab-cache
spec:
  classSelector:
    matchLabels:
      stack: gitlab
      region: us-west
```

Matched against the below resource classes:

```yaml
apiVersion: cache.gcp.crossplane.io/v1alpha2
kind: CloudMemorystoreInstanceClass
metadata:
  name: really-quite-a-lot-of-memory
  labels:
    stack: gitlab
    region: us-west
    grade: production
specTemplate:
  tier: STANDARD_HA
  region: us-west2
  memorySizeGb: 99999999999999999999999
  redisVersion: REDIS_3_2
---
apiVersion: cache.gcp.crossplane.io/v1alpha2
kind: CloudMemorystoreInstanceClass
metadata:
  name: cheap-redis
  labels:
    stack: gitlab
    region: us-west
    grade: experimental
specTemplate:
  tier: STANDARD_HA
  region: us-west2
  memorySizeGb: 1
  redisVersion: REDIS_3_2
```

Both classes match the resource claim's desired properties, but one is much more
expensive than the other. Given the resource claim did not specifically request
a production database, it should probably be scheduled to use the cheaper class.
Weighting could be used to approximate the "cost" of resource classes - higher
weights are cheaper (or safer) resource classes.

Note that anything other than random scheduling would require the various class
scheduler controllers to collaborate, increasing the complexity of the system.
In the case of weighted scheduling each controller would need to determine the
highest weighted matching resource class, then determine whether its highest
weight was higher than that chosen by any other scheduler controller.

### Webhooks instead of Controllers

The matching of a resource claim to a resource class is a one time affair; it's
unlikely that we'd ever want to change a claim's class. An [admission webhook]
could be more appropriate for one off processes like this. Crossplane has been
reluctant to introduce webhooks due to concerns about their performance and
configuration complexity. Controllers are well understood and the path of least
resistance, but class matching is likely a good use case to port to a webhook in
future.

[strongly typed]: https://github.com/crossplane/crossplane/blob/69e69c/design/one-pager-strongly-typed-class.md
[we identified]: https://github.com/crossplane/crossplane/issues/723
[portable resource classes]: https://github.com/crossplane/crossplane/blob/69e69c/design/one-pager-default-resource-class.md
[community members]: https://github.com/crossplane/crossplane/issues/703#issuecomment-536958545
[provided feedback]: https://github.com/crossplane/crossplane/issues/922
[Crossplane glossary]: https://crossplane.io/docs/v0.3/concepts.html#glossary
[label selectors]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
[stack]: https://github.com/crossplane/crossplane/blob/f7cfadb090/design/design-doc-stacks.md
[teaching each resource claim controller]: https://github.com/crossplane/crossplane/issues/113
[Workload scheduling]: https://github.com/crossplane/crossplane/blob/master/design/design-doc-workload-scheduler.md
[lowest common denominator]: https://thenewstack.io/avoiding-least-common-denominator-approach-hybrid-clouds/
[states that]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ 
[cannot be done using label selectors]: https://github.com/kubernetes/kubernetes/issues/44703#issuecomment-324826356
[admission webhook]: https://book.kubebuilder.io/reference/admission-webhook.html

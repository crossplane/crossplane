---
title: Multi-Tenant Crossplane
toc: true
weight: 240
indent: true
---

# Multi-Tenant Crossplane

This guide describes how to use Crossplane effectively in multi-tenant
environments by utilizing Kubernetes primitives and compatible policy
enforcement projects in the cloud-native ecosystem.

## TL;DR

Infrastructure operators in multi-tenant Crossplane environments typically
utilize composition and Kubernetes RBAC to define lightweight, standardized
policies that dictate what level of self-service developers are given when
requesting infrastructure. This is primarily achieved through exposing abstract
resource types at the namespace scope, defining `Roles` for teams and
individuals within that namespace, and patching the `spec.providerConfigRef` of
the underlying managed resources so that they use a specific `ProviderConfig`
and credentials when provisioned from each namespace. Larger organizations, or
those with more complex environments, may choose to incorporate third-party
policy engines, or scale to multiple Crossplane clusters. The following sections
describe each of these scenarios in greater detail.

- [Background](#background)
    - [Cluster Scoped Managed Resources](#cluster-scoped-managed-resources)
    - [Namespace Scoped Claims](#namespace-scoped-claims)
- [Single Cluster Multi Tenancy](#single-cluster-multi-tenancy)
  - [Composition as an Isolation Mechanism](#composition-as-an-isolation-mechanism)
  - [Namespaces as an Isolation Mechanism](#namespaces-as-an-isolation-mechanism)
  - [Policy Enforcement with Open Policy Agent](#policy-enforcement-with-open-policy-agent)
- [Multi Cluster Multi Tenancy](#multi-cluster-multi-tenancy)
  - [Reproducible Platforms with Configuration Packages](#reproducible-platforms-with-configuration-packages)
  - [Control Plane of Control Planes](#control-plane-of-control-planes)
  - [Service Provisioning using Open Service Broker API](#service-provisioning-using-open-service-broker-api)

## Background

Crossplane is designed to run in multi-tenant environments where many teams are
consuming the services and abstractions provided by infrastructure operators in
the cluster. This functionality is facilitated by two major design patterns in
the Crossplane ecosystem. 

### Cluster-Scoped Managed Resources

Typically, Crossplane providers, which supply granular [managed resources] that
reflect an external API, authenticate by using a `ProviderConfig` object that
points to a credentials source (such as a Kubernetes `Secret`, the `Pod`
filesystem, or an environment variable). Then, every managed resource references
a `ProviderConfig` that points to credentials with sufficient permissions to
manage that resource type.

For example, the following `ProviderConfig` for `provider-aws` points to a
Kubernetes `Secret` with AWS credentials.

```yaml
apiVersion: aws.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: cool-aws-creds
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: key
```

If a user desired for these credentials to be used to provision an
`RDSInstance`, they would reference the `ProviderConfig` in the object manifest:

```yaml
apiVersion: database.aws.crossplane.io/v1beta1
kind: RDSInstance
metadata:
  name: rdsmysql
spec:
  forProvider:
    region: us-east-1
    dbInstanceClass: db.t3.medium
    masterUsername: masteruser
    allocatedStorage: 20
    engine: mysql
    engineVersion: "5.6.35"
    skipFinalSnapshotBeforeDeletion: true
  providerConfigRef:
    name: cool-aws-creds # name of ProviderConfig above
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: aws-rdsmysql-conn
```

Since both the `ProviderConfig` and all managed resources are cluster-scoped,
the RDS controller in `provider-aws` will resolve this reference by fetching the
`ProviderConfig`, obtaining the credentials it points to, and using those
credentials to reconcile the `RDSInstance`. This means that anyone who has been
given [RBAC] to manage `RDSInstance` objects can use any credentials to do so.
In practice, Crossplane assumes that only folks acting as infrastructure
administrators or platform builders will interact directly with cluster-scoped
resources.

### Namespace Scoped Claims

While managed resources exist at the cluster scope, composite resources, which
are defined using a **CompositeResourceDefinition (XRD)** may exist at either
the cluster or namespace scope. Platform builders define XRDs and
**Compositions** that specify what granular managed resources should be created
in response to the creation of an instance of the XRD. More information about
this architecture can be found in the [Composition] documentation.

Every XRD is exposed at the cluster scope, but only those with `spec.claimNames`
defined will have a namespace-scoped variant.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositemysqlinstances.example.org
spec:
  group: example.org
  names:
    kind: CompositeMySQLInstance
    plural: compositemysqlinstances
  claimNames:
    kind: MySQLInstance
    plural: mysqlinstances
...
```

When the example above is created, Crossplane will produce two
[CustomResourceDefinitions]:
1. A cluster-scoped type with `kind: CompositeMySQLInstance`. This is referred
   to as a **Composite Resource (XR)**.
2. A namespace-scoped type with `kind: MySQLInstance`. This is referred to as a
   **Claim (XRC)**.

Platform builders may choose to define an arbitrary number of Compositions that
map to these types, meaning that creating a `MySQLInstance` in a given namespace
can result in the creations of any set of managed resources at the cluster
scope. For instance, creating a `MySQLInstance` could result in the creation of
the `RDSInstance` defined above.

## Single Cluster Multi-Tenancy

Depending on the size and scope of an organization, platform teams may choose to
run one central Crossplane control plane, or many different ones for each team
or business unit. This section will focus on servicing multiple teams within a
single cluster, which may or may not be one of many other Crossplane clusters in
the organization.

### Composition as an Isolation Mechanism

While managed resources always reflect every field that the underlying provider
API exposes, XRDs can have any schema that a platform builder chooses. The
fields in the XRD schema can then be patched onto fields in the underlying
managed resource defined in a Composition, essentially exposing those fields as
configurable to the consumer of the XR or XRC.

This feature serves as a lightweight policy mechanism by only giving the
consumer the ability to customize the underlying resources to the extent the
platform builder desires. For instance, in the examples above, a platform
builder may choose to define a `spec.location` field in the schema of the
`CompositeMySQLInstance` that is an enum with options `east` and `west`. In the
Composition, those fields could map to the `RDSInstance` `spec.region` field,
making the value either `us-east-1` or `us-west-1`. If no other patches were
defined for the `RDSInstance`, giving a user the ability (using RBAC) to create
a `CompositeMySQLInstance` / `MySQLInstance` would be akin to giving the ability
to create a very specifically configured `RDSInstance`, where they can only
decide the region where it lives and they are restricted to two options.

This model is in contrast to many infrastructure as code tools where the end
user must have provider credentials to create the underlying resources that are
rendered from the abstraction. Crossplane takes a different approach, defining
various credentials in the cluster (using the `ProviderConfig`), then giving
only the provider controllers the ability to utilize those credentials and
provision infrastructure on the users behalf. This creates a consistent
permission model, even when using many providers with differing IAM models, by
standardizing on Kubernetes RBAC.

### Namespaces as an Isolation Mechanism

While the ability to define abstract schemas and patches to concrete resource
types using composition is powerful, the ability to define Claim types at the
namespace scope enhances the functionality further by enabling RBAC to be
applied with namespace restrictions. Most users in a cluster do not have access
to cluster-scoped resources as they are considered only relevant to
infrastructure admins by both Kubernetes and Crossplane.

Building on our simple `CompositeMySQLInstance` / `MySQLInstance` example, a
platform builder may choose to define permissions on `MySQLInstance` at the
namespace scope using a `Role`. This allows for giving users the ability to
create and and manage `MySQLInstances` in their given namespace, but not the
ability to see those defined in other namespaces.

Futhermore, because the `metadata.namespace` is a field on the XRC, patching can
be utilized to configure managed resources based on the namespace in which the
corresponding XRC was defined. This is especially useful if a platform builder
wants to designate specific credentials or a set of credentials that users in a
given namespace can utilize when provisioning infrastructure using an XRC. This
can be accomplished today by creating one or more `ProviderConfig` objects that
include the name of the namespace in the `ProviderConfig` name. For example, if
any `MySQLInstance` created in the `team-1` namespace should use specific AWS
credentials when the provider controller creates the underlying `RDSInstance`,
the platform builder could:

1. Define a `ProviderConfig` with name `team-1`.

```yaml
apiVersion: aws.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: team-1
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: team-1-creds
      key: key
```

2. Define a `Composition` that patches the name of the Claim reference in the XR
   to the `providerConfigRef` of the `RDSInstance`.

```yaml
...
resources:
- base:
    apiVersion: database.aws.crossplane.io/v1beta1
    kind: RDSInstance
    spec:
      forProvider:
      ...
  patches:
  - fromFieldPath: spec.claimRef.namespace
    toFieldPath: spec.providerConfigRef.name
    policy:
      fromFieldPath: Required
```

This would result in the `RDSInstance` using the `ProviderConfig` of whatever
namespace the corresponding `MySQLInstance` was created in.

> Note that this model currently only allows for a single `ProviderConfig` per
> namespace. However, future Crossplane releases should allow for defining a set
> of `ProviderConfig` that can be selected from using [Multiple Source Field
> patching]. 

### Policy Enforcement with Open Policy Agent

In some Crossplane deployment models, only using composition and RBAC to define
policy will not be flexible enough. However, because Crossplane brings
management of external infrastructure to the Kubernetes API, it is well suited
to integrate with other projects in the cloud-native ecosystem. Organizations
and individuals that need a more robust policy engine, or just prefer a more
general language for defining policy, often turn to [Open Policy Agent] (OPA).
OPA allows platform builders to write custom logic in [Rego], a domain-specific
language. Writing policy in this manner allows for not only incorporating the
information available in the specific resource being evaluated, but also using
other state represented in the cluster. Crossplane users typically install OPA's
[Gatekeeper] to make policy management as streamlined as possible.

> A live demo of using OPA with Crossplane can be viewed [here].

## Multi-Cluster Multi-Tenancy

Organizations that deploy Crossplane across many clusters typically take
advantage of two major features that make managing multiple control planes much
simpler.

### Reproducible Platforms with Configuration Packages

[Configuration packages] allow platform builders to package their XRDs and
Compositions into [OCI images] that can be distributed via any OCI-compliant
image registry. These packages can also declare dependencies on providers,
meaning that a single package can declare all of the granular managed resources,
the controllers that must be deployed to reconcile them, and the abstract types
that expose the underlying resources using composition.

Organizations with many Crossplane deployments utilize Configuration packages to
reproduce their platform in each cluster. This can be as simple as installing
Crossplane with the flag to automatically install a Configuration package
alongside it.

```
helm install crossplane --namespace crossplane-system crossplane-stable/crossplane --set configuration.packages={"registry.upbound.io/xp/getting-started-with-aws:latest"}
```

### Control Plane of Control Planes

Taking the multi-cluster multi-tenancy model one step further, some
organizations opt to manage their many Crossplane clusters using a single
central Crossplane control plane. This requires setting up the central cluster,
then using a provider to spin up new clusters (such as an [EKS Cluster] using
[provider-aws]), then using [provider-helm] to install Crossplane into the new
remote cluster, potentially bundling a common Configuration package into each
install using the method described above.

This advanced pattern allows for full management of Crossplane clusters using
Crossplane itself, and when done properly, is a scalable solution to providing
dedicated control planes to many tenants within a single organization.

### Service Provisioning using Open Service Broker API

Another way to achieve multi-cluster multi-tenancy is by leveraging the
possibilities of the [Open Service Broker API] specification and tie it
together with Crossplane.

A possible architecture could look like this: Crossplane and the
[Crossplane Service Broker] are running on the central control plane cluster.
The Crossplane objects which represent the service offerings and service plans,
the XRDs and Compositions, leverage [provider-helm] to spin up service instances
on one or many service clusters. The end-user uses the [Kubernetes Service Catalog]
to order services via the Crossplane Service Broker. A demo of this concept can be
found under [vshn/application-catalog-demo].

This way even a tight integration of Crossplane in to [Cloudfoundry] is possible.

<!-- Named Links -->
[managed resources]: ../concepts/managed-resources.md
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[Composition]: ../concepts/composition.md
[CustomResourceDefinitions]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[Open Policy Agent]: https://www.openpolicyagent.org/
[Rego]: https://www.openpolicyagent.org/docs/latest/policy-language/
[Gatekeeper]: https://open-policy-agent.github.io/gatekeeper/website/docs/
[here]: https://youtu.be/TaF0_syejXc
[Multiple Source Field patching]: https://github.com/crossplane/crossplane/pull/2093
[Configuration packages]: ../concepts/packages.md
[OCI images]: https://github.com/opencontainers/image-spec
[EKS Cluster]: https://doc.crds.dev/github.com/crossplane/provider-aws/eks.aws.crossplane.io/Cluster/v1beta1@v0.17.0
[provider-aws]: https://github.com/crossplane/provider-aws
[provider-helm]: https://github.com/crossplane-contrib/provider-helm
[Open Service Broker API]: https://github.com/openservicebrokerapi/servicebroker
[Crossplane Service Broker]: https://github.com/vshn/crossplane-service-broker
[Cloudfoundry]: https://www.cloudfoundry.org/
[Kubernetes Service Catalog]: https://github.com/kubernetes-sigs/service-catalog
[vshn/application-catalog-demo]: https://github.com/vshn/application-catalog-demo

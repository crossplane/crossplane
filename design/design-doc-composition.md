# Resource Composition

* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Crossplane is a control plane for applications and the infrastructure upon which
they run. Contemporary Crossplane applications are modelled as arbitrary custom
resources. Application reconcile logic is provided by a [Stack] - a package of
[Custom Resource Definitions] (CRDs) and the controllers that reconcile them.
The infrastructure needs of applications are modelled as a set of custom
resources - resource claims - provided by Crossplane and its infrastruture
providers. These resource claim custom resources are resolved and bound to
provider specific, high fidelity custom resources known as managed resources.

Applications and infrastructure are modelled inconsistently in Crossplane. Each
requires learning and applying a distinct set of patterns and concepts. Stacks
enable bespoke, composable applications at the expense of requiring application
developers to maintain code or configuration outside of the Kubernetes API in
order to specify how their applications should be reconciled. Infrastructure is
configured entirely within the Kubernetes API at the expense of flexibility and
composability.

### Applications

Contemporary Crossplane applications are packaged as Stacks. Each Stack defines
a set of custom resources and the controllers that should reconcile them. Stack
controllers may be implemented as bespoke code (for example using [kubebuilder])
or as a [Template Stack]. A Template Stack invokes a [Kustomization] or a [Helm]
chart in order to reconcile a custom resource.

```yaml
---
apiVersion: apps.crossplane.io/v1alpha1
kind: Wordpress
metadata:
  name: example-wordpress-app
spec:
  image: wordpress:4.6.1-apache
```

When an application is modelled as a Kubernetes custom resource - for example a
`Wordpress` resource - Crossplane's goal is to use said resource to render one
or more lower level resources that will actually trigger a deployment of
Wordpress. Crossplane may for example render a `MySQLInstance`, a `Deployment`,
and a `Service`. This process is known as reconciliation; the input resource is
reconciled with the output resources. It's also an example of composition; the
`Wordpress` is reconciled by composing resources in the Kubernetes API, as
opposed to being reconciled by orchestrating an external system.

The composition of a Crossplane application depends on two inputs; the custom
resource that represents the application and a template for the outputs. The
template could be modelled in code as part of a bespoke controller, or as a Helm
chart or Kustomization. These templates are modelled and managed outside of the
Kubernetes API boundary; they are not stored as Kubernetes custom resources.
This property limits Crossplane's ability to validate its inputs; Crossplane
cannot determine whether the templates required to render an application are
valid until they are invoked to render the application. It also makes the
rendering process opaque to application operators, who must leave the Kubernetes
API and learn an external system (i.e. Helm, Kustomize, or kubebuilder) in order
to determine how their application will be rendered.

Note that Crossplane also defines a resource of `KubernetesApplication`, which
models an opaque bundle of arbitrary Kubernetes resources. This resource was
originally intended as a user-facing model for applications, but has in practice
seen more use as a lower level mechanism to schedule and deliver applications to
remote Kubernetes clusters. A `Wordpress` may render a `KubernetesApplication`
in order to deploy the component resources of a `Wordpress` to a cluster
distinct from that upon which Crossplane runs.

### Infrastructure

Crossplane uses a [class and claim] model to provision and manage infrastructure
in an external system, such as a cloud provider. _External resources_ in the
provider's API are modelled as _managed (custom) resources_ in the Kubernetes
API server. Managed resources are the domain of infrastructure operators;
they're cluster scoped infrastructure like a `Node` or `PersistentVolume`.
Application operators may claim a managed resource for a particular purpose by
creating a namespaced _resource claim_. Managed resources may be provisioned
explicitly before claim time (static provisioning), or automatically at claim
time (dynamic provisioning). The initial configuration of dynamically
provisioned managed resources is specified by a _resource class_.

```yaml
---
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: example-resource-claim
spec:
  engineVersion: "5.7"
  writeConnectionSecretToRef:
    name: sql
  classSelector:
    matchLabels:
      example: "true"
---
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstanceClass
metadata:
  name: example-resource-class
  labels:
    example: "true"
specTemplate:
  forProvider:
    databaseVersion: MYSQL_5_6
    region: us-west2
    settings:
      tier: db-n1-standard-1
      dataDiskType: PD_SSD
      dataDiskSizeGb: 10
      ipConfiguration:
        ipv4Enabled: true
  writeConnectionSecretsToNamespace: crossplane-system
  providerRef:
    name: example
```

A managed resource is a _high-fidelity_ representation of its corresponding
external resource. High-fidelity in this context means two things:

* A managed resource maps to exactly one external resource - one API object.
* A managed resource is as close to a direct translation of its corresponding
  external API object as is possible without violating [API conventions].

These properties make managed resources - Crossplane's lowest level
infrastructure primitive - flexible and self documenting. Managed resources in
and of themselves hold few opinions about _how_ they should be used, and are
easily related back to the APIs they represent. This provides a solid foundation
upon which to build Crossplane's multicloud capability.

Application operators are typically prevented by [RBAC] from creating and
modifying managed resources directly; they are instead expected to dynamically
provision the managed resources they require by submitting a resource claim.
Crossplane provides claim kinds for common, widely supported resource variants
like `MySQLInstance` and `KubernetesCluster`. There is a one-to-one relationship
between claims and the managed resources they bind to; a `KubernetesCluster`
claim binds to exactly one `GKECluster` managed resource. However, a solitary
resource is often not particularly useful without supporting infrastructure, for
example:

* An RDS instance may be inaccessible without a security group.
* An Azure SQL instance may be inaccessible without a virtual network rule.
* A GKE, EKS, or AKS cluster (control plane) may not be able to run pods without
  a node group.

Crossplane infrastructure providers frequently model this supporting
infrastructure (there is a `SecurityGroup` managed resource, for example), but
it cannot be dynamically provisioned or bound to a resource claim. Instead a
cluster operator must statically provision any supporting managed resources
ahead of time, then author resource classes that reference them. This can be
limiting:

* Often supporting resources must reference the managed resource they support,
  for example an Azure `MySQLServerVirtualNetworkRule` must reference the
  `MySQLServer` it applies to. Dynamically provisioned managed resources such as
  a `MySQLServer` have non-deterministic names, making it impossible to create a
  `MySQLServerVirtualNetworkRule` until the `MySQLServer` it must reference has
  been provisioned.
* When a resource class references a statically provisioned managed resource
  every managed resource that is dynamically provisioned using that class will
  reference that specific managed resource. For example if a `GKEClusterClass`
  references a `Subnetwork` then every `GKECluster` dynamically provisioned
  using said class will attempt to share said `Subnetwork`, despite it often
  being desirable to create a unique `Subnetwork` for each dynamically
  provisioned `GKECluster`.

The one-to-one relationship between resource claims and resource classes thus
weakens portability, separation of concerns, and support for [GitOps]. An
infrastructure operator can publish a resource class representing a single
managed resource that an application operator may dynamically provision, but in
the likely event that managed resource requires supporting managed resources to
function usefully the application operator must ask an infrastructure operator
to provision them.

Furthermore, defining a core set of portable resource claims has begun to limit
Crossplane. Resource claims are subject to the [lowest common denominator]
problem; when a claim may provide configuration inputs that may be used to match
or provision many kinds of managed resource it may support only the settings
that apply to _all_ compatible managed resources. This is in part why Crossplane
defines relatively few resource claim kinds. Meanwhile, it's possible that an
infrastructure operator deploys Crossplane to provide an opinionated abstraction
for the application operators in their organisation and that said organisation
only uses AWS. If this organisation values Crossplane's separation of concerns
but does not need its portability there is no reason that its application
operators should be limited to resource claims that are constrained by
supporting all possible providers.

## Goals

The goal of this document is to propose a simple, consistent, Kubernetes native
model for composing applications and their infrastructure needs.

TODO(negz): Elaborate on the below goals.

* Nested composition
* User defined abstractions
* Shareable abstractions
* Composable infrastructure
* Discoverable abstraction to composition relationships
* Compositions configured inside API boundary

## Use Cases

The proposal put forward by this document is intended to address a handful of
use cases around application and infrastructure provisioning.

### Application Composition

An _application developer_ wishes to simplify and standardise the work of the
_application operators_ who will deploy and operate their application. They wish
to encode the deployment and infrastructure needs of their application, and to
expose a subset of the configuration of said deployment and infrastructure needs
to the _application operators_.

For example, the _(application) developers_ of Wordpress wish to encode the fact
that each deployment of Wordpress requires one or more web servers running
wordpress, an SQL database, and some kind of network ingress (e.g. a load
balancer) to get requests to the web servers.

Wordpress’s _application developers_ wish to allow _application operators_ to
configure only how much storage their SQL database has, whether the database
uses SSD or spinning disk, and how replicated the web server will be.

All other configuration details should be selected from a “class” or “profile”of
application. This class may decide whether the web server is containerized or a
VM, whether the SQL database is PostgreSQL or MySQL, etc.

The input to this example is a `Wordpress` custom resource. The outputs depend
on the class of application, but could include a `Deployment`, and a
`MySQLInstance`, where a `MySQLInstance` is a namespaced composite
infrastructure resource.

GitLab is a more advanced example of an application composition, in which the
GitLab application is composed in turn of many smaller composite applications
each roughly as complex as this Wordpress example.

In this example:

* The _schema of the input_ (`Wordpress`) is defined by an _app developer_.
* The _configuration of the input_ is defined by an _application operator_.
* The _schema of the output_ composed resources (e.g. `MySQLInstance`) may be
  defined by either an _application developer or an infrastructure operator_.
* The _configuration of the output_ composed resources is defined by an
  _application developer_.

### Infrastructure Composition

An _infrastructure provider_ or _infrastructure operator_ wishes to simplify and
standardise the work of _other infrastructure operators_. They wish to encode a
common configuration of infrastructure resources as a conceptually new resource
so that _infrastructure operators_ will not need to repeat themselves when they
wish to publish this common configuration for use by their _application
operators_.

For example, configuring a useful Google Cloud Platform [managed instance group]
in fact requires configuring up to eight primitive managed resources, including
an instance template, autoscaler, and instance group manager. The GCP
_infrastructure provider_ wishes to represent these eight resources as a single
conceptual resource - a managed instance group - that _infrastructure operators_
may provide to their _application operators_.

The GCP _infrastructure provider_ wishes to allow _infrastructure operators_ to
configure a subset of the many configuration parameters of the underlying
primitive managed resources, for example the zone, size, and load balancer port
of the instance group. All other configuration details should be fixed.

The input to this example is a `ManagedInstanceGroup`. The outputs include
an `Autoscaler`, an `InstanceTemplate`, and a `InstanceGroupManager`.

In this example:

* The _schema of the input_ (`ManagedInstanceGroup`) is defined by an
  _infrastructure provider or infrastructure operator_.
* The _configuration of the input_ is defined by an _infrastructure operator_.
* The _schema of the output_ composed resources (e.g. `Autoscaler`) is
  defined by an _infrastructure provider_ or _infrastructure operator_.
* The _configuration of the output_ composed resources is defined by the
  _infrastructure provider_ or _infrastructure operator_.

### Infrastructure Publication

An _infrastructure operator_ wishes to simplify and standardize the work of
_application operators_ who may need to use the infrastructure they steward. The
infrastructure operator wishes to expose only the application-focused
configuration details of the infrastructure they offer.

For example, an _infrastructure operator_ wishes to publish a MySQL database as
a kind of infrastructure resource their _application operators_ may use. A MySQL
database requires at least an instance capable of running MySQL (e.g. a CloudSQL
instance) and a database created on that instance (e.g. a CloudSQL database).
Further infrastructure resources may be required, for example to ensure the
database is reachable from a particular VPC network.

_Application operators_ should only be able to configure the version, region,
database size, and disk type (spinning or SSD) of the MySQL instance. All other
configuration details should be selected from a “class” or “profile” of
infrastructure. This class may decide whether the SQL instance runs in GCP or
AWS, whether it’s production or development grade, etc.

The input to this example is a `MySQLDatabase` (in the `acme.example.org` API
group). On Azure these outputs may include a `MySQLServer`, a `MySQLDatabase`
(in the `database.azure.crossplane.io` API group - Azure resource naming is very
literal) and a `MySQLServerVirtualNetworkRule`.

In this example:

* The _schema of the input_ (`MySQLDatabase`) is defined by an _infrastructure
  operator_.
* The _configuration of the input_ is defined by an _application operator_.
* The _schema of the output_ composed resources (e.g. `MySQLServer`) is defined
  by either an _infrastructure provider or infrastructure operator_.
* The _configuration of the output_ composed resources is defined by an
  _infrastructure operator_.

## Proposal

This document proposes the introduction of four new Crossplane resource kinds:

* `ApplicationDefinition` - Defines a new kind of Kubernetes custom resource
  that represents an application.
* `InfrastructureDefinition` - Defines a new kind of Kubernetes custom resource
  that represents a logical group of infrastructure.
* `InfrastructurePublication` - Defines a new kind of Kubernetes custom resource
  that binds a logical group of infrastructure to an application by "publishing"
  a resource defined by an `InfrastructureDefinition`.
* `Composition` - Configures how one or more custom resources should be rendered
  in response to the creation or modification of a custom resource defined by an
  `ApplicationDefinition` or `InfrastructureDefinition`.

![Architecture diagram][architecture-diagram]

### Composition

A `Composition` configures how one or more custom resource should be rendered in
response to the creation or modification of a custom resource defined by an
`ApplicationDefinition` or `InfrastructureDefinition`. No controller watches for
the `Composition` kind; a `Composition` is loosely analogous to the existing
Crossplane concept of a resource class. Unlike resource classes, compositions:

* May define how to compose applications as well as infrastructure.
* May compose more than one resource, including other composed resources.
* Explicitly configure their relationship with a defined application or
  infrastructure resource kind.

Here's an example composition:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: private-mysql-server
  labels:
    connectivity: private
spec:
  # This composition declares that its input values will be read 'from' a
  # resource of the specified kind. The field name 'from' denotes the
  # relationship with the 'fromFieldPath' notation below.
  from:
    apiVersion: database.example.org/v1alpha1
    kind: MySQLInstance
  # This composition declares that its input values will be written 'to' the
  # below resources. The field name 'to' denotes the relationship with the
  # 'toFieldPath' notation below.
  to:
  - base:
      apiVersion: azure.crossplane.io/v1alpha3
      kind: ResourceGroup
      metadata:
      spec:
        location: West US
        providerRef:
          name: example
        reclaimPolicy: Delete
    patches:
    - fromFieldPath: "spec.region"
      toFieldPath: "spec.forProvider.location"
      transforms:
      - type: map
        map:
          us-west: "West US"
          us-east: "East US"
  - base:
      apiVersion: database.azure.crossplane.io/v1beta1
      kind: MySQLServer
      spec:
        forProvider:
          administratorLogin: myadmin
          resourceGroupNameSelector:
            matchComposite: true
          location: West US
          sslEnforcement: Disabled
          version: "5.6"
          sku:
            tier: Basic
            capacity: 1
            family: Gen5
          storageProfile:
            storageMB: 20480
        writeConnectionSecretToRef:
          namespace: crossplane-system
        providerRef:
          name: example
        reclaimPolicy: Delete
    patches:
    - fromFieldPath: "metadata.uid"
      toFieldPath: "spec.writeConnectionSecretToRef.name"
    - fromFieldPath: "spec.engineVersion"
      toFieldPath: "spec.forProvider.version"
    - fromFieldPath: "spec.storageGB"
      toFieldPath: "spec.forProvider.storageMB"
      transforms:
      - type: math
        math:
          multiply: 1024
    - fromFieldPath: "spec.region"
      toFieldPath: "spec.forProvider.location"
      transforms:
      - type: map
        map:
          us-west: "West US"
          us-east: "East US"
    # Specifies the (potentially sensitive) connection details that this 'to'
    # resource should expose to the 'from' resource. Names are unique across all
    # 'to' resources within this composition. Ignored by application resources.
    connectionDetails:
    - name: username
      fromConnectionSecretKey: username
    - name: password
      fromConnectionSecretKey: password
    - name: endpoint
      fromConnectionSecretKey: endpoint
  - base:
      apiVersion: database.azure.crossplane.io/v1alpha3
      kind: MySQLServerVirtualNetworkRule
      spec:
        name: my-cool-vnet-rule
        serverNameSelector:
          matchComposite: true
        resourceGroupNameSelector:
          matchComposite: true
        properties:
          virtualNetworkSubnetIdRef:
            name: sample-subnet
        reclaimPolicy: Delete
        providerRef:
          name: azure-provider
```

This document proposes compositions be immutable. This property would allow
updates to a 'from' resource to be propagated to the various 'to' resources but
keep the set of 'to' resources stable over the lifetime of the 'from' resource,
allowing controllers to avoid complex update and garbage collection logic.

### ApplicationDefinition

An `ApplicationDefinition` defines a new kind of custom resource that represents
an application - for example a `Wordpress` custom resource. An application
resource is namespaced, and may only compose resources within its namespace.
Applications are typically defined by an _application developer_ - the developer
of Wordpress would author the `ApplicationDefinition` that defines the schema of
a `Wordpress` resource. Application resources are typically authored by an
_application operator_.

Here's an example that defines a new `Wordpress` application resource:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: ApplicationDefinition
metadata:
  # ApplicationDefinition names are subject to the constraints of Kubernetes
  # CustomResourceDefinition names. They must be of the form <plural>.<group>.
  name: wordpresses.apps.example.org
spec:
  # Defines the structural schema and GroupVersionKind of this application. Only
  # a single API version of the application may exist. Additional fields will be
  # injected to support composition machinery.
  crdSpecTemplate:
    group: apps.example.org
    version: v1alpha1
    names:
      kind: Wordpress
      listKind: WordpressList
      plural: wordpresses
      singular: wordpress
    validation:
      openAPIV3Schema:
        properties:
          administratorLogin:
            type: string
          storageSize:
            type: int
          storageType:
            type: string
        type: object
  # An optional service account that will be used to reconcile Wordpress
  # resources. This allows the use of RBAC to restrict which resources a
  # Wordpress application may be composed of. The specified service account must
  # have full access to Wordpress resources, and 'get' access to Component
  # resources.
  #
  # If the service account is omitted Crossplane will use its pod service
  # account to manage Wordpress resources. This implies that anyone with
  # sufficient RBAC permissions to create a Composition and to create a
  # Wordpress resource in a particular namespace will be able to compose their
  # Wordpress of any resource Crossplane is able to create. Crossplane will
  # refuse to create resources at the cluster scope or outside of the namespace
  # in which the Wordpress was created.
  serviceAccountRef:
    namespace: crossplane-system
    name: wordpresses.apps.example.org
  # An optional default composition that will be set automatically for any
  # Wordpress custom resources that omit both their compositeSelector and their
  # compositeRef.
  defaultComposition:
    apiVersion: apiextensions.crossplane.io/v1alpha1
    kind: Composition
    name: local-wordpress
  # An optional forced composition that will be set automatically for any
  # Wordpress custom resource, overriding their compositeSelector and their
  # compositeRef. If defaultComposition and forceComposition are both set, the
  # forced composition wins.
  forceComposition:
    apiVersion: apiextensions.crossplane.io/v1alpha1
    kind: Composition
    name: wordpresses.apps.example.org
```

When an application developer authors the above `ApplicationDefinition`
Crossplane will automatically create a `CustomResourceDefinition`, that
allows application operators to author the below custom resource:

```yaml
apiVersion: example.org/v1alpha1
kind: Wordpress
metadata:
  namespace: default
  name: coolblog
spec:
  # The schema for the following three fields is defined by the above
  # ApplicationDefinition.
  administratorLogin: admin
  storageSize: 2
  storageType: SSD
  # The application object schema is automatically injected into the
  # CustomResourceDefinition that is created by the ApplicationDefinition that
  # defines the Wordpress resource.
  application:
    # Multiple compositions may potentially satisfy a particular kind of
    # application. Each application instance may influence which composition is
    # used via label selectors. This could be used, for example, to determine
    # whether a Wordpress application renders to a KubernetesApplication or to a
    # plain old Kubernetes Deployment.
    compositionSelector:
      matchLabels:
        compute: kubernetes
        database: mysql
    # The Wordpress author may explicitly select which composition should be
    # used by setting the compositionRef. In the majority of cases the author
    # will ignore this field and it will be set by a controller, similar to the
    # contemporary classRef field.
    compositionRef:
    - name: wordpress-kubernetes-mysql
    # Each application maintains an array of the resources they compose.
    # Composed resources are always in the same namespace as the application
    # resource. Any namespaced resource may be composed; composed resources
    # model their relationship with the application resource via their
    # controller reference. The application must maintain this array because
    # there is currently no user friendly, performant way to discover which
    # resources (of arbitrary kinds) are controlled by a particular resource per
    # https://github.com/kubernetes/kubernetes/issues/54498
    composedRefs:
    - apiVersion: database.example.org/v1alpha1
      kind: MySQLInstanceRequirement
      name: coolblog-3jmdf
    - apiVersion: workload.crossplane.io/v1alpha1
      kind: KubernetesApplication
      name: coolblog-3mdm2
```

### InfrastructureDefinition

An `InfrastructureDefinition` defines a new kind of custom resource that
represents infrastructure - for example a `MachineLearningCluster` resource. An
infrastructure resource is cluster scoped and may only compose other cluster
scoped infrastructure resources. Infrastructure resources include the
"primitive" infrastructure resources that are implemented by infrastructure
providers as well as other composite infrastructure resources.

Infrastructure is typically defined by an _infrastructure operator_, though it
is expected that _infrastructure providers_ will frequently define composite
infrastructure. An infrastructure operator would author an instance of the above
`MachineLearningCluster`. Application operators may _indirectly_ author
infrastructure resources that have been published as an infrastructure
requirement - more on that below.

Here's an example that defines a new `MySQLInstance` infrastructure resource.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: InfrastructureDefinition
metadata:
  # InfrastructureDefinition names are subject to the constraints of Kubernetes
  # CustomResourceDefinition names. They must be of the form <plural>.<group>.
  name: mysqlinstances.database.example.org
spec:
  # Any composition that intends to satisfy an infrastructure resource must
  # expose each of the named connection details exactly once in any of its
  # connectionDetails objects. The connection secret published by the defined
  # infrastructure resource will include only these connection details.
  connectionDetails:
  - username
  - password
  - endpoint
  # Defines the structural schema and GroupVersionKind of this infrastructure.
  # Only a single API version of the application may exist. Additional fields
  # will be injected to support composition machinery.
  crdSpecTemplate:
    group: database.example.org
    version: v1alpha1
    names:
      kind: MySQLInstance
      listKind: MySQLInstanceList
      plural: mysqlinstances
      singular: mysqlinstance
    validation:
      openAPIV3Schema:
        properties:
          engineVersion:
            type: string
          region:
            type: string
          storageGB:
            type: int
        type: object
  # An optional service account that will be used to reconcile MySQLInstance
  # resources. This allows the use of RBAC to restrict which resources a
  # MySQLInstance may be composed of. The specified service account must have
  # full access to MySQLInstance resources, and 'get' access to Component
  # resources.
  #
  # If the service account is omitted Crossplane will use its pod service
  # account to manage MySQLInstance resources. This implies that anyone with
  # sufficient RBAC permissions to create a Composition and to create a
  # MySQLInstance will be able to compose their MySQLInstance of any
  # infrastructure resource that Crossplane is able to create.
  serviceAccountRef:
    namespace: crossplane-system
    name: mysqlinstances.database.example.org
  # An optional default composition that will be set automatically for any
  # MySQLInstance custom resources that omit both their compositeSelector and
  # their compositeRef.
  defaultComposition:
    apiVersion: apiextensions.crossplane.io/v1alpha1
    kind: Composition
    name: cheap-rds
  # An optional forced composition that will be set automatically for any
  # MySQLInstance custom resource, overriding their compositeSelector and their
  # compositeRef. If defaultComposition and forceComposition are both set, the
  # forced composition wins.
  forceComposition:
    apiVersion: apiextensions.crossplane.io/v1alpha1
    kind: Composition
    name: mysqlinstances.database.example.org
```

When an application developer authors the above `InfrastructureDefinition`
Crossplane will automatically create a `CustomResourceDefinition`, that allows
application operators to author the below custom resource:

```yaml
apiVersion: database.example.org/v1alpha1
kind: MySQLInstance
metadata:
  name: sql
spec:
  # The schema for the following three fields is defined by the above
  # InfrastructureDefinition.
  engineVersion: "5.7"
  storageGB: 10
  region: us-west
  # The infrastructure object schema is automatically injected into the
  # CustomResourceDefinition that is created by the InfrastructureDefinition
  # that defines the MySQLInstance resource.
  infrastructure:
    # Multiple compositions may potentially satisfy a particular kind of
    # infrastructure. Each infrastructure instance may influence which
    # composition is used via label selectors. This could be used, for example,
    # to determine whether a GCP CloudSQLInstance or an Azure SQLServer based
    # composition satisfied this MySQLInstance.
    compositionSelector:
     matchLabels:
       connectivity: private
    # The MySQLInstance author may explicitly select which composition should be
    # used by setting the compositionRef. In the majority of cases the author
    # will ignore this field and it will be set by a controller, similar to the
    # contemporary classRef field.
    compositionRef:
    - name: private-mysql-server
    # Each infrastructure resource maintains an array of the resources it
    # composes. Composed resources are always cluster scoped, and always either
    # primitive or composite infrastructure resources. Composed resources model
    # their relationship with the infrastructure resource via their controller
    # reference. The infrastructure resource must maintain this array because
    # there is currently no user friendly, performant way to discover which
    # resources (of arbitrary kinds) are controlled by a particular resource per
    # https://github.com/kubernetes/kubernetes/issues/54498
    composedRefs:
    - apiVersion: azure.crossplane.io/v1alpha3
      kind: ResourceGroup
      name: sql-34jd2
    - apiVersion: database.azure.crossplane.io/v1beta1
      kind: MySQLServer
      name: sql-3i3d1
    - apiVersion: database.azure.crossplane.io/v1alpha3
      kind: MySQLServerVirtualNetworkRule
      name: sql-2mdus
    # The MySQLInstance author must specify where the MySQLInstance will write
    # its connection details as a Kubernetes secret. The keys of the secret are
    # specified by the InfrastructureDefinition.
    writeConnectionSecretToRef:
      namespace: crossplane-system
      name: sql
    # This cluster scoped MySQLInstance _may_ bind to exactly one namespaced
    # MySQLInstanceRequirement. See InfrastructurePublication below for details.
    requirementRef:
      apiVersion: database.example.org/v1alpha1
      kind: MySQLInstanceRequirement
      namespace: default
      name: sql
    # The reclaim policy determines what happens to this infrastructure
    # resource and all of the infrastructure resources it composes if it is
    # bound and then released. The policy may be either 'Delete' or 'Retain'.
    reclaimPolicy: Retain
```

Note that the use of controller references to model relationships between an
infrastructure resource and the other infrastructure resources it composes means
there is no binding phase or reclaim policy between a composite infrastructure
resource and the resources it composes. No concept of static provisioning or
(not) reclaiming exists; the lifecycle of composed resources is strictly tied to
that of the resource that composes them due to the use of controller references,
which control Kubernetes garbage collection.

### InfrastructurePublication

An `InfrastructurePublication` defines a new kind of custom resource that
indicates an application's requirement of a logical group of infrastructure by
"publishing" a kind of resource defined by a `InfrastructureDefinition`. The
`MySQLInstance` resource from the example above is cluster scoped and thus may
only be authored (directly) by an _infrastructure operator_. It may however be
"published" for use by _application operators_ by authoring an
`InfrastructurePublication`. Doing so create a new, namespaced kind of resource
that corresponds to the `MySQLInstance`, of kind `MySQLInstanceRequirement`.
This `MySQLInstanceRequirement` managed resource inherits all spec fields of a
`MySQLInstance`. Infrastructure requirements are analogous to contemporary
Crossplane resource claims. They maintain all of their features and
functionality of a resource claim (e.g. static and dynamic binding). Unlike a
resource claim, a resource requirement propagates any field updates made by the
application operator on to its bound composite resource, and thus onto the
composed resources.

Here's an example that publishes the above `MySQLInstance` resource:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: InfrastructurePublication
metadata:
  # The name of the InfrastructurePublication must match the name of the
  # infrastructure resource it publishes. A defined infrastructure resource may
  # be published at most once. Only defined infrastructure resources may be
  # published; i.e. a primitive managed resource such as a CloudSQLInstance may
  # not be published for direct use by application operators.
  name: mysqlinstances.database.example.org
spec:
  infrastructureDefinitionReference:
    name: mysqlinstances.database.example.org
```

When an infrastructure operator authors the above `InfrastructurePublication`
Crossplane will automatically create a `CustomResourceDefinition` allowing
application operators to author the below custom resource:

```yaml
# The API version of the requirement is always the same as that of the resource
# it publishes. The kind is the kind of the published resource suffixed with the
# word 'Requirement'. This enables users to distinguish between requirements and
# the resources they require.
apiVersion: database.example.org/v1alpha1
kind: MySQLInstanceRequirement
metadata:
  namespace: default
  name: sql
spec:
  # The schema for the following three fields is inherited from the
  # InfrastructureDefinition referenced by the InfrastructurePublication that
  # publishes this kind of resource. Put otherwise, the schema identically
  # matches this requirement's cluster scoped equivalent.
  engineVersion: "5.7"
  storageGB: 10
  region: us-west
  # The infrastructure object schema is automatically injected into the
  # CustomResourceDefinition that is created by the InfrastructurePublication
  # that publishes the MySQLInstanceRequirement resource.
  infrastructure:
    # An infrastructure requirement binds to exactly one defined, cluster scoped
    # infrastructure resource. In the case of this MySQLInstanceRequirement the
    # kind of the bound resource will always be MySQLInstance. There is no
    # binding phase; if the requirement and the required resource reference each
    # other they are considered to be bound. An application operator may specify
    # this resource reference explicitly in order to bind to a  MySQLInstance
    # that was provisioned in advance by an infrastructure operator.
    resourceRef:
    - apiVersion: database.example.org/v1alpha1
      kind: MySQLInstance
      name: default-sql-dd02m
    # In the (common) case in which the application operator omits the above
    # resource reference a MySQLInstance will be dynamically provisioned to
    # satisfy this MySQLInstanceRequirement. When this is the case the below
    # compositionSelector and compositionRef (if any) are copied verbatim to the
    # newly created MySQLInstance.
    compositionSelector:
     matchLabels:
       connectivity: private
    compositionRef:
    - name: private-mysql-server
    # The MySQLInstanceRequirement author must specify where the requirement
    # will write its connection details as a Kubernetes secret. The secret is an
    # exact copy of the bound MySQLInstance's connection secret.
    writeConnectionSecretToRef:
      name: sql
```

The pattern of "publishing" a pre-defined cluster scoped infrastructure resource
that may separately be required by namespaced applications has several desirable
properties:

* Infrastructure may be composed arbitrarily at the cluster scope by defining
  new infrastructure resource kinds and how they should be composed of other,
  predefined infrastructure resource kinds.
* There is a one-to-one relationship between a namespaced infrastructure
  requirement and a cluster scoped infrastructure resource. In the static
  provisioning case a requirement maps to exactly one kind of cluster scoped
  infrastructure resource, and exactly one instance of that kind of
  infrastructure resource.
* In the dynamic provisioning case a cluster scoped resource may be created by
  simply copying the spec of the requirement to that of the cluster scoped
  resource, avoiding the "double definition" problem.

Presuming an infrastructure operator wanted to publish a
`MySQLInstanceRequirement` for their application operators to use, the
infrastructure operator would:

1. Author a `InfrastructureDefinition` defining the schema and connection
   details of a cluster scoped `MySQLInstance`.
1. Author at least one `Composition`, configuring how a `MySQLInstance` may be
   satisfied - for example by provisioning a `CloudSQLInstance`.
1. Author an `InfrastructurePublication` to publish the
   `MySQLInstanceRequirement` kind for use by application operators.

Note that the `MySQLInstance` defined in step 1 of this process is inherently
composable into other cluster scoped infrastructure resources. An infrastructure
operator who wishes to define a new kind of infrastructure resource that may
only be authored (or used in a `Composition`) by infrastructure operators uses
the same process and resources as above; they simply omit step 3.

#### The Double Definition Problem

Presume an infrastructure operator wishes to expose two primitive infrastructure
managed resources that an application requires; their goal is to allow an
application operator to author a single namespaced resource and in return be
allocated two primitive infrastructure resources. Perhaps the application
operator will author a `KubernetesCluster` resource and in return be allocated a
`GKECluster` and a `NodePool`. These two primitive resources may either exist
already, or be created on demand to satisfy the `KubernetesCluster`.

It's dramatically simpler to select and bind a single existing cluster scoped
infrastructure resource than it is two select two existing resources. If the
`KubernetesCluster` were to explicitly select a `GKECluster` and a `NodePool` it
would need to ensure the `GKECluster` and `NodePool` were part of the same
cluster, presumably by understanding their cross resource references. The
`KubernetesCluster` would then need to maintain references to both selected
resources across the "scope boundary" - from namespace to cluster scope. This
precludes the use of Kubernetes owner references.

So it's desirable for the application focused infrastructure resource to bind to
exactly one cluster scoped infrastructure resource, whether that cluster scoped
resource already exists or must be provisioned on-demand. The infrastructure
operator is faced with the "double definition" problem when they must define the
schema for both the `KubernetesCluster` resource and the "intermediate" cluster
scoped infrastructure resource.

A `GKECluster` managed resource has around 100 configurable spec fields. A
`NodePool` managed resource has around 30. The infrastructure operator must
first determine which of these fields should be represented on their new cluster
scoped composite resource. All 130? Should the fields have the same names as
their composed resources? Once the infrastructure operator has defined what
seems like a reasonable set of fields for their cluster scoped managed resource
they _must_ go through this process again in order to define an application
focused, namespaced scope `KubernetesCluster`. The infrastructure operator must
define two layers of abstraction when functionally they desire only one - a
namespaced infrastructure resource (`KubernetesCluster`) that allows application
operators to bind two cluster scoped infrastructur resources (a `GKECluster` and
a `NodePool`).

### Transform Functions

A `Composition` produces a set of composed resources by taking a set of 'base'
resource templates and 'patching' them by copying fields from the composite
resource to the composed resources, using [field path notation] to map a field
path 'from' the composite resource 'to' a field path in the composed resource:

```yaml
patches:
- fromFieldPath: "spec.region"
  toFieldPath: "spec.forProvider.location"
```

This alone is sufficient if the 'from' field's value is always a valid value of
the 'to' field, but that is often not the case. Imagine for example a composite
resource that could be satisfied by two compositions; one for Azure and another
for GCP. Both clouds have similar geographic regions, but represent them
differently; "West US 2" vs "us-west2". It's not possible to directly map one
value from the composite resource to both sets of composed resources; you must
pick either "West US 2" or "us-west2". This compromises the portability of the
composite resource; it's values are only applicable to one cloud. In some cases
this may be fine, but transform functions help where portability is desired.

A transform function transforms the 'from' value, returning an altered 'to'
value. For example the below map transform would take the value 'us-west' from
the `spec.region` field of the composite resource and transform it to "West US"
before writing it to the `spec.forProvider.location` field of the composed
resource.

```yaml
patches:
- fromFieldPath: "spec.region"
  toFieldPath: "spec.forProvider.location"
  transforms:
  - type: map
    map:
      us-west: "West US"
      us-east: "East US"
```

Transform functions are intended to be simple and validatable as YAML, rather
than inventing a DSL inside YAML. Multiple transforms can be stacked if
necessary and will be applied in the order they are specified. A conservative
number of transform functions will be added at first, with more being added as
use cases appear. This document proposes the following transforms be added to
Crossplane initially, in addition to the map transform above:

```yaml
patches:
- fromFieldPath: "spec.storageGB"
  toFieldPath: "spec.forProvider.storageProfile.storageMB"
  transforms:
    - type: math
      math:
        multiply: 1024
```

A math transform that allows for simple math operations, like converting MB to
GB.

```yaml
patches:
- fromFieldPath: "metadata.uid"
  toFieldPath: "spec.writeConnectionSecretToRef.name"
  transforms:
  - type: string
    string:
      fmt: "%s-postgresqlserver"
```

A string format transform (using the Go [fmt syntax]). This allows string values
to be prefixed or suffixed, and allows number values to be converted to strings.
The string format transform can be used to propagate the external name
annotation from one composite resource to many composed resources of the same
kind while ensuring their external names do not conflict. For example:

```yaml
patches:
- fromFieldPath: "metadata.annotations[crossplane.io/external-name]"
  toFieldPath: "metadata.annotations[crossplane.io/external-name]"
  transforms:
  - type: string
    string:
      # If the composite resources external name was 'example', this composed
      # resource's external name would be 'example-a'.
      fmt: "%s-a"
```

### Composite References

Crossplane allows certain fields of primitive managed resources to be set to a
value inferred from another primitive managed resource using [cross resource
references]. Take for example a resource that must specify the VPC network in
which it should be created, by specifying the `network` field:

```yaml
spec:
  forProvider:
    network: /projects/example/global/networks/desired-vpc-network
```

Cross resource references allow this resource to instead reference the `Network`
resource that represents the desired VPC Network:

```yaml
spec:
  forProvider:
    networkRef:
      name: desired-vpc-network
```

The managed resource reconcile logic resolves this reference and populates the
`network` field, resulting in the following configuration:

```yaml
spec:
  forProvider:
    # Network is populated with the value calculated by networkRef.
    network: /projects/example/global/networks/desired-vpc-network
    networkRef:
      name: desired-vpc-network
```

This functionality must be extended in order to support composite resource
classes. Consider a `Composite` that may be used to dynamically provision
the following primitive managed resources:

* `Subnetwork` A
* `GKECluster` A
* `ServiceAccount` A
* `ServiceAccount` B
* `GKENodePool` A
* `GKENodePool` B

The `Composite` author would like to configure the resources such that:

1. `Subnetwork` A is created in an existing, statically provisioned `Network`.
1. `GKECluster` A is created in `Subnetwork` A.
1. `GKENodePool` A uses `ServiceAccount` A.
1. `GKENodePool` B uses `ServiceAccount` B.
1. Both `GKENodePool` resources join `GKECluster` A.

The author cannot use a contemporary cross resource reference for requirements
two through five. Managed resources are referenced by name, and the names of
dynamically provisioned resources are non-deterministic; they are not known
until they have been provisioned.

This document proposes the introduction of a reference _selector_, which allows
a managed resource to describe the properties of the distinct resource it wishes
to reference, rather than explicitly naming it.

```yaml
spec:
  forProvider:
    networkSelector:
      # Match only managed resources that are part of the same composite, i.e.
      # managed resources that have the same controller reference as the
      # selecting resource.
      matchControllerRef: true
      # Match only managed resources with the supplied labels.
      matchLabels:
        example: label
```

The combination of these two fields allows a managed resource to uniquely
identify a distinct managed resource within the same composite. In the previous
example the `GKENodePool` resources need only use `matchControllerRef` to match
the `GKECluster` they wish to join, because there is only one `GKECluster` for
them to match within their composite resource. They need to use
`matchControllerRef` and `matchLabels` to match their desired `ServiceAccount`;
the labels distinguish of the two composed `ServiceAccount` resources are
matched.

If a reference field is set, its corresponding selector field is ignored. If the
selector field is unset, it is ignored. If the specified selector matches
multiple managed resources one is chosen at random, though specifying both
`matchControllerRef` and `matchLabels` can always guarantee that at most one
provisioned managed resource will match the selector.

### Connection Secrets

Contemporary Crossplane infrastructure resources - resource claims and managed
resources - expose their connection details as secrets. Connection details may
include sensitive data required to connect to the infrastructure resource such
as authentication credentials, or non-sensitive data such as a URL or endpoint.
Composite infrastructure resources and their requirements must also expose
connection details in order to enable applications to leverage the underlying
composed resources.

Composite resource cannot naively aggregate the various connection secrets of
their composed resources. Secrets are a map of strings to byte arrays and there
is no guarantee that two composed resources won't expose connection details
using the same key; for example two composed resources could both expose the
`username` detail.

This document proposes that the connection secrets exposed by a composite
resource define a contract that must be fulfilled by the composed resources. For
example:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: InfrastructureDefinition
# ...
spec:
  connectionDetails:
  - username
  - password
  - endpoint
```

The above `InfrastructureDefinition` declares that the defined resource exposes
three connection details in its secret; `username`, `password`, and `endpoint`.
Each connection detail must be provided by exactly one composed resource.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
# ...
spec:
  to:
  - base:
      # ...
    patches:
      # ...
    connectionDetails:
    - name: username
      fromConnectionSecretKey: admin-username
    - fromConnectionSecretKey: password
    - fromConnectionSecretKey: endpoint
```

The above `Composition` satisfies the contract established by its corresponding
`InfrastructureDefinition`. It composes a resource that publishes `username`,
`password`, and `endpoint` as connection details. Note that the composed
resource _actually_ publishes a connection detail under the secret key
`admin-username`, but explicitly declares by including the `name` of the detail
that its `admin-username` key should correspond to the composite's `username`
key. The other two details don't specify a `name`, and thus it is inferred that
the `fromConnectionSecretKey` is the same as the required detail name.

Note that this contract could be satisfied by more than one composed resource,
as long as each required detail is satisfied exactly once. In the below example
the first composed resource satisfies the requirement for a username and
password, while the second satsifies the requirment for an endpoint.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
# ...
spec:
  to:
  - base:
      # ...
    patches:
      # ...
    connectionDetails:
    - name: username
      fromConnectionSecretKey: admin-username
    - fromConnectionSecretKey: password
  - base:
      # ...
    patches:
      # ...
    connectionDetails:
    - fromConnectionSecretKey: endpoint
```

### Backward Compatibility

TODO(negz): Explain how this design is backward compatible with our current
controllers. I'm _pretty sure_ this design is backward compatible, and could
thus live alongside our existing resource classes, claims, etc. This would allow
us to deprecate the existing classes (and claim controllers) and migrate away
from them cleanly.

## Alternatives Considered

TODO(negz): Capture the various sketches and ideas that lead here.

## Terminology

_Application developer_. An application developer (or app dev) is responsible
for writing and providing an application.

_Application operator_. An application operator (or app op) is responsible for
deploying an application and keeping it running. The application they deploy
will likely consist of compute “workloads” (e.g. OCI containers) and their
infrastructure dependencies (e.g. a Redis cache).

_Infrastructure provider_. An infrastructure provider is responsible for
exposing cloud infrastructure as primitive resources, for example a
CloudSQLInstance or ReplicationGroup.

_Infrastructure operator_. An infrastructure operator (or infra op) is
responsible for configuring the types and shapes of infrastructure that is
available to their organisation’s application operators.

_Primitive resource_. A Kubernetes custom resource that does not compose other
resources (using Crossplane composition).

_Composite resource_. A Kubernetes custom resource that is composed of other
resources (using Crossplane composition).

[Custom Resource Definitions]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[Stack]: design-doc-stacks.md
[kubebuilder]: https://book.kubebuilder.io/
[Template Stack]: one-pager-template-stacks.md
[Helm]: https://helm.sh/
[Kustomization]: https://kustomize.io/
[class and claim]: https://static.sched.com/hosted_files/kccncna19/5e/eric-tune-kcon-slides-final.pdf
[API conventions]: https://github.com/kubernetes/community/blob/862de062acf8bbd84f7a655914fa08972498819a/contributors/devel/sig-architecture/api-conventions.md
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[GitOps]: https://www.weave.works/technologies/gitops/
[background]: #background
[lowest common denominator]: https://thenewstack.io/avoiding-least-common-denominator-approach-hybrid-clouds/
[_composite_]: https://en.wikipedia.org/wiki/Composite_data_type
[_primitive_]: https://en.wikipedia.org/wiki/Primitive_data_type
[declarative configuration best practices]: https://github.com/kubernetes/community/blob/5d62001/contributors/design-proposals/architecture/declarative-application-management.md#declarative-configuration
[cross resource reference]: one-pager-cross-resource-referencing.md
[above example]: #example-google-cloud-sql-instance
[managed instange group]: https://cloud.google.com/compute/docs/instance-groups/creating-groups-of-managed-instances
[architecture-diagram]: design-doc-composition.png
[field path notation]: https://github.com/kubernetes/community/blob/26337/contributors/devel/sig-architecture/api-conventions.md#selecting-fields
[fmt syntax]: https://golang.org/pkg/fmt/

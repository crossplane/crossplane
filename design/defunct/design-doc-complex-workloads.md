# Complex Workloads in Crossplane
* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Background
[Crossplane][1] is an open source multi cloud control plane. It introduces
workload and resource abstractions on-top of existing managed services to enable
a high degree of workload portability across cloud providers. A Crossplane
`Workload` models an application that may be deployed to a Kubernetes cluster;
it is a unit of scheduled work that cannot be split across multiple clusters.
Crossplane managed clusters are represented by resource claim named
`KubernetesCluster`; a `Workload` scheduled to a `KubernetesCluster` is
analogous to a `Pod` scheduled to a `Node`.

A contemporary Crossplane `Workload`:
```yaml
---
apiVersion: compute.crossplane.io/v1alpha1
kind: Workload
metadata:
  name: demo
spec:
  clusterSelector:
    provider: gcp
  resources:
  - name: demo
    secretName: demo
  targetDeployment:
    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: wordpress
      labels:
        app: wordpress
    spec:
      selector:
        app: wordpress
      template:
        metadata:
          labels:
            app: wordpress
        spec:
          containers:
            - name: wordpress
              image: wordpress:4.6.1-apache
              ports:
                - containerPort: 80
  targetNamespace: demo
  targetService:
    apiVersion: v1
    kind: Service
    metadata:
      name: wordpress
    spec:
      ports:
        - port: 80
      selector:
        app: wordpress
      type: LoadBalancer
```

Workloads are modeled in Crossplane 0.1 as a [Custom Resource Definition][12]
(CRD) embedding a Kubernetes `Namespace`, `Deployment` and `Service` -
`.spec.targetNamespace`, `.spec.targetDeployment` and `.spec.targetService`
respectively. Once the scheduler has scheduled the `Workload` to a cluster the
workload controller connects to said cluster and creates the templated
`Deployment` and `Service`. The controller polls the status of the `Deployment`
and `Service` during its sync phase, persisting them inline in the `Workload`'s
`.status` field. Each `Workload` may also contain a set of references to
Crossplane resources or resource claims upon which the Workload depends -
modeled as distinct Kubernetes resources - in order to replicate their
connection `Secrets` to the cluster upon which the `Workload` is scheduled.

Complex applications such as Gitlab exceed the capabilities of today's
`Workload` resource. Gitlab recommends deploying to Kubernetes via Helm. When
configured to use managed services for Redis, SQL, and Buckets the chart renders
to almost 4,800 lines of YAML including 14 `Deployments`, 1 `StatefulSet`,
3 `Jobs`, 9 `Services`, 16 `ConfigMaps`, and many other resources. Crossplane
must be able to model complex applications as complex workloads.


## Goals
The goal of this document is to design _part_ of the best possible user
experience for deploying complex applications with Crossplane; `Workload` will
not be responsible for the entire application installation and lifecycle
management but rather be a building block that may be managed by higher level
constructs.

It is important that:
* Workloads can model any Kubernetes resource, including built in resources and
  those defined by CRDs.
* Users do not need to connect to the cluster to which a `Workload` is scheduled
  in order to determine the status of the resources (`Deployments`, etc) managed
  by said `Workload`.
* Each `Workload` is a unit of scheduling; it may not be spread across multiple
  `KubernetesClusters`.
* The proposed design lay a foundation for supporting workloads that are not
  containerised.

The following are out of scope for the Workload resource:
* Deploying a single workload to multiple clusters simultaneously.
* Configuration and/or templating. Each `Workload` will be a 'static' resource;
  the task of generating or altering `Workloads` given a set of inputs will
  be that of a higher level construct.
* Package and dependency management. `Workloads` will not model dependencies on
  or relationships to other `Workloads`. Any resource types upon which a
  `Workload` depends are presumed to have been defined via CRD before
  instantiating the `Workload`.

## Design

### Custom Resource Definitions
This document proposes the `Workload` kind within the
`compute.crossplane.io/v1alpha1` API group be replaced with the
`KubernetesApplication` kind in the `workload.crossplane.io/v1alpha` group. The
`.spec` of each `KubernetesApplication` consists of a `KubernetesCluster` label
selector used for scheduling, and a series of resource templates representing
resources to be deployed to the scheduled `KubernetesCluster`.

A `KubernetesApplication` will not template arbitrary resources directly, but
rather via an interstitial resource; `KubernetesApplicationResource`. Each
`KubernetesApplication` therefore consists of one or more templated
`KubernetesApplicationResources`, each of which templates exactly one arbitrary
Kubernetes resource (for example a `Deployment` or `ConfigMap`).

#### Schema
Each `KubernetesApplicationResource` represents a single Kubernetes resource to
be deployed to a `KubernetesCluster`. The `KubernetesApplicationResource`
encapsulates the resource, including type and object metadata, in its
`.spec.template` field. If the templated resource kind exposes a `.status` field
when deployed, said field will be copied verbatim to the
`KubernetesApplicationResource`'s `.status.remote` field.
`KubernetesApplicationResources` will also specify a list of `Secrets` presumed
to be the automatically created resource connection secrets for Crossplane
managed resources upon which its templated Kubernetes resource depends. These
`Secrets` will be propagated into the same namespace as the templated resource.

Crossplane will model the template using the
[`*unstructured.Unstructured`][3] type internally. Unstructured types must
include Kubernetes type and object metadata but are otherwise opaque. Status
will be completely opaque - i.e. a [`json.RawMessage`][4] - to the controller
code. The controller will copy the remote resource's `.status` field into the
`KubernetesApplicationResource`'s `.status`.remote field. `.status.remote`
will be absent from `KubernetesApplicationResources` that template resource
kinds that do not expose a `.status` field.

An example complex workload:
```yaml
---
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: wordpress-demo
  namespace: complex
  labels:
    app: wordpress-demo
spec:
  clusterSelector:
    matchLabels:
      app: wordpress-demo
  # Each resource template is used to create a KubernetesApplicationResource.
  resourceTemplates:
  - metadata:
      # Metadata of the KubernetesApplicationResource. The namespace is ignored;
      # KubernetesApplicationResources are always created in the namespace of
      # their controlling KubernetesApplication. This matches the behaviour of
      # Deployments and ReplicaSets.
      name: wordpress-demo-namespace
      labels:
        app: wordpress-demo
    spec:
      # This template specifies the actual resource to be deployed and managed
      # in a remote Kubernetes cluster by this KubernetesApplicationResource.
      # Note the two layers of templating; a KubernetesApplication templates
      # KubernetesApplicationResources, which template arbitrary resources.
      template:
        # These templates must contain type as well as object metadata, because
        # we allow templating of arbitrary resource kinds.
        apiVersion: v1
        kind: Namespace
        metadata:
          name: wordpress
          labels:
            app: wordpress
  - metadata:
      name: wordpress-demo-deployment
      labels:
        app: wordpress-demo
    spec:
      secrets:
      # sql is the name of a connection secret. It will be propagated to the
      # namespace of this KubernetesApplicationResource's template (i.e.
      # wordpress) as a Secret named wordpress-demo-deployment-sql.
      - name: sql
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          namespace: wordpress
          name: wordpress
          labels:
            app: wordpress
        spec:
          selector:
            matchLabels:
              app: wordpress
          template:
            metadata:
              labels:
                app: wordpress
            spec:
              containers:
                - name: wordpress
                  image: wordpress:4.6.1-apache
                  ports:
                    - containerPort: 80
                      name: wordpress
  - metadata:
      name: wordpress-demo-service
      labels:
        app: wordpress-demo
    spec:
      template:
        apiVersion: v1
        kind: Service
        metadata:
          namespace: wordpress
          name: wordpress
          labels:
            app: wordpress
        spec:
          ports:
            - port: 80
          selector:
            app: wordpress
          type: LoadBalancer
```

Listing resources associated with a Kubernetes application:
```bash
$ kubectl -n complex get kubernetesapplication wordpress-demo
NAME             CLUSTER                  STATUS               DESIRED   SUBMITTED
wordpress-demo   wordpress-demo-cluster   PartiallySubmitted   3         2

$ kubectl -n complex get kubernetesapplicationresource --selector app=wordpress-demo
NAME                        TEMPLATE-KIND   TEMPLATE-NAME   CLUSTER                  STATUS
wordpress-demo-deployment   Deployment      wordpress       wordpress-demo-cluster   Submitted
wordpress-demo-namespace    Namespace       wordpress       wordpress-demo-cluster   Submitted
wordpress-demo-service      Service         wordpress       wordpress-demo-cluster   Failed
```

#### Naming
The proposed `KubernetesApplication` and especially
`KubernetesApplicationResource` names are rather verbose when compared to their
contemporary: `Workload`. These names are best justified by breaking them down
into their parts:

_Kubernetes_ represents the deployment vector of the application. Prefixing the
kind with Kubernetes leaves room to define applications that are deployed using
other methods. This design proposes the explicit prefix Kubernetes rather than
the abstract prefix _Containerized_ because the proposed CRD is tightly coupled
to Kubernetes; it could not be used to deploy a containerized application via
Amazon ECS or Docker Swarm. The scheme chosen by this design impacts future
implementations; would an application targeting Amazon Lambda be named a
ServerlessApplication or a LambdaApplication? Kubernetes is arguably ubiquitous
enough to be analogous with generic resource kind names like
ServerlessApplication or VMApplication.

_Application_ distinguishes a workload from a compute resource when interacting
with Crossplane. It is synonymous in this context with _Workload_, which is
implied by the workload.crossplane.io API namespace. Including Application would
thus be redundant except that the API namespace is typically omitted when
interacting with the API server. Assume `KubernetesApplication` was instead
named `Kubernetes`, relying on the API namespace to indicate that it was a
workload. In this scenario `kubectl get kubernetes` would return Kubernetes
workloads while `kubectl get kubernetescluster` would return Crossplane managed
Kubernetes clusters. These names are close enough that it's not unlikely
Crossplane users would expect `kubectl get kubernetes` to return Kubernetes
clusters rather than workloads. Application is preferable to Workload to avoid
[stuttering][5] when the API namespace is considered, and provides symmetry with
similar concepts like sig-apps' [Application][6].

_Resource_ templates an arbitrary Kubernetes resource of which an application
consists. A resource could template a compute resource such as a `Deployment`,
`StatefulSet`, or `Job`; a configuration resource such as a `ConfigMap` or
`Secret`; or a networking resource such as a `Service` or `Ingress`. The term
'Resource' is overloaded in the Crossplane world; it can refer to both a generic
Kubernetes resource (roughly synonymous with 'object' in Kubernetes parlance) as
well as a Crossplane 'managed resource', for example an `SQLInstance` resource
claim or an `RDSInstance` as a concrete managed resource. This document uses
'resource template' interchangeably with `KubernetesApplicationResource` and
explicitly refers to managed resources as 'managed resources'.

_workload.crossplane.io_ is the API namespace in which applications and their
resource templates exist, regardless of whether the application targets
Kubernetes or something else. Moving the kinds from `compute.crossplane.io` to
`workload.crossplane.io` clearly delineates compute resources from things that
run on compute resources.

#### Namespacing
Kubernetes resource kinds may be namespace or cluster scoped. The former exist
within a namespace that must be created before the resource, allowing a named
resource to be instantiated multiple times; once per namespace. The latter are
singletons; only one named instance of a resource can exist per cluster. Most
Kubernetes resource kinds are namespaced. Cluster scoped resources include
`CustomResourceDefinition`, `ClusterRole`, `PersistentVolume`, and `Namespace`
itself. Cluster scoped resources use the same object metadata schema as
namespaced resources but ignore the `.metadata.namespace` field.

The contemporary `Workload` templates two namespaced resources (a `Deployment`
and `Service`) and one cluster scoped resource (a `Namespace`). This document
proposes that application resource templates avoid special handling of
namespaces; an application could consist of three resource templates -
templating a `Namespace` named `coolns`, a `Deployment` in namespace `coolns`,
and a `Deployment` without a namespace. Templated resources of a namespaced kind
that do not specify a namespace will be created in the namespace default as
would any other Kubernetes resource. No relationship will exist between the
namespace of the `KubernetesApplication` or `KubernetesApplicationResource` in
the Crossplane API server and the namespace of templated resources to be
deployed to a cluster.

At first glance this may seem more complicated than requiring a namespace be
specified one time at the application level. On the contrary, doing so would
both complicate Crossplane's controller logic and result in surprising
behaviours for users. Recall that a `KubernetesApplicationResource` may
template any valid Kubernetes resource kind, _including those unknown to the
Crossplane API server_. This means a `KubernetesApplication` specifying an
explicit target namespace for its resource templates could consist of
`KubernetesApplicationResources` that template cluster scoped resources,
including other namespaces, that cannot be created in said target namespace.

This confusing behaviour could be eliminated by eliminating support for cluster
scoped resources; such resources are typically more closely related to clusters
themselves than the workloads running upon them. Unfortunately the ability to
require templated resources be namespaced is mutually exclusive with the ability
to template resource kinds unknown to the Crossplane API server. Namespaced and
cluster scoped resources are indistinguishable. Both use standard Kubernetes
object metadata, but cluster scoped resources ignore `.metadata.namespace`. It
is possible to determine whether a resource is namespaced by inspecting its
kind's API resource definition, but this would require resource definitions be
applied to the Crossplane API server before Crossplane was able to template
their resources.

The main arguments for specifying target namespaces at the application rather
than resource template level involve avoiding repetition. Most applications will
be composed of several namespaced resources deployed to one namespace.
Specifying the namespace via a resource template's object metadata would require
an application with ten resource templates to repeat the namespace ten times. In
cases where one application is deployed per cluster this is a moot point; there
is no need for namespacing when a cluster runs only one application. Simply omit
the namespace altogether and let resources be created in the namespace `default`
as is the Kubernetes API server's standard behaviour.

References to dependent managed resources are also specified at the resource
template level in the proposed design. Recall that the contemporary `Workload`
contains a set of references to managed resources. This allows Crossplane to
propagate their connection `Secrets` to the cluster upon which the `Workload` is
scheduled. `Secrets` are namespaced, and may only be consumed from within their
own namespace, so Crossplane must ensure secrets are propagated to the same
namespace as their consumers. It could be repetitive to specify dependent
managed resources at the resource template level, for example if an application
was composed of three `Deployments` all connecting to the same message queue.
Each resource template of a `Deployment` would need to reference the same
message queue resource.

On the other hand, this repetition is born of _explicitness_. Imagine a complex
workload consisting of three `Deployments` dependent upon two `SQLInstances`.
Specifying resource dependencies at the resource template level makes it
explicit which `Deployment` depends upon which `SQLInstance`. In this case it's
less ideal to model dependent resources at the application level, as doing so
would effectively represent that "some of the resource templates of this
application depend on some of these managed resources" rather than "this
Kubernetes resource depends on exactly these managed resources".

An application and its resource templates are static representations of a
complex workload to be deployed to a cluster. Requiring that templated resources
exist in exactly one namespace specified at the application scope complicates
Crossplane's controller code and results in surprising behaviours. This document
proposes that applications be unopinionated about resource namespaces and
instead rely on convention. Most workloads will be generated via a higher level
tool such as Helm. Such tools are the better place for strong opinions; they can
easily take a namespace as an input and output a `KubernetesApplication`
consisting of a `KubernetesApplicationResource` templating a `Namespace` along
with several other `KubernetesApplicationResources` templating resources to be
deployed to that namespace.

#### Secret Propagation
As mentioned in [Namespacing](#namespacing) this document proposes that the set
of Crossplane managed resource references used to propagate connection secrets
be scoped at the resource, not application level.

```go
type ResourceReference struct {
    // These first seven fields are in reality an embedded
    // corev1.ObjectReference.
	Kind            string
	Namespace       string
	Name            string
	UID             types.UID
	APIVersion      string
	ResourceVersion string
	FieldPath       string

	SecretName      string
}
```

The resources field of the contemporary `Workload` is a slice of
`ResourceReference` structs. These references are used, by convention, to refer
to either a Crossplane resource binding (e.g. `SQLInstance`) or a concrete
Crossplane resource (e.g. `RDSInstance`), but could just as easily refer to a
`Deployment` or `ConfigMap` that does not make sense in this context. In
practice, the contemporary workload controller code only uses
`ResourceReference`'s `SecretName` and `Name` fields. If `SecretName` is
specified a `Secret` of that name will be retrieved. If `SecretName` is not
specified a secret named `Name` will be retrieved. In either case all other
fields of the `ResourceReference`, including `Namespace`, are ignored. The
contemporary controller always looks for connection secrets in the `Workload`'s
namespace. Naming this field `.resources` makes it seem that a user could simply
provide a set of resource claims or concrete resources and let Crossplane figure
out the rest, but this is not the case. The user must either provide a set of
resources that follow Crossplane's default convention of storing their
connection secret in a `Secret` with the same name as the resource, or
explicitly tell Crossplane which `Secret` name to propagate.

```go
type KubernetesApplicationResourceSpec struct {
	Template *unstructured.Unstructured
	Secrets  []corev1.LocalObjectReference
}

type LocalObjectReference struct {
	Name
}
```

Given that the only purpose of the contemporary resources field is to load
resource connection `Secrets` for propagation, and given that the contemporary
workload only loads `Secrets` from within the `Workload`'s namespace,
`KubernetesApplicationResource` instead uses a slice of
`corev1.LocalObjectReference` in a field named `.secrets`. Doing so clarifies
the purpose and constraints of the field without having to read documentation or
the controller code.

## Controllers
The contemporary `Workload` is watched by two controllers within Crossplane -
the scheduler and the workload controller. The former is responsible for
allocating a `KubernetesCluster` to a `Workload` while the latter is responsible
for connecting to said cluster and managing the lifecycle of the `Workload`'s
`Namespace`, `Deployment` and `Service`.

This document proposes the responsibilities of the existing workload controller
be broken up between two controllers - application and resource. Under this
proposal the three controllers would have the following responsibilities:

* The *scheduler controller* watches for `KubernetesApplications`. It allocates
  each application to a `KubernetesCluster`. This is unchanged from today's
  scheduler implementation.
* The *application controller* watches for scheduled `KubernetesApplications`.
  It is responsible for:
    * Creating, updating, and deleting `KubernetesApplicationResources`
      according to its templates.
    * Ensuring the [controller reference][7] is set on its extant
      `KubernetesApplicationResources`.
    * Updating the application's `.status.desiredResources` and
      `.status.submittedResources` fields. The former represents the number of
      resource templates the application specifies. The latter represents the
      subset of those resource templates that have been successfully submitted
      to their scheduled Kubernetes cluster.
* The *resource controller* watches for scheduled
  `KubernetesApplicationResources`. It is responsible for:
    * Propagating its `.secrets` to its scheduled `KubernetesCluster`.
      Propagated `Secret` names are derived from the
      `KubernetesApplicationResource` and connection secret names in order to
      avoid conflicts when two resource templates reference the same `Secret`.
      For example a `Secret` named `mysql` referenced by a resource template
      named `wordpress-deployment` would be propagated to the scheduled cluster
      as a `Secret` named `wordpress-deployment-mysql`.
    * Creating or updating the resource templated in its `.spec.template` (e.g.
      a `Deployment`, `Service`, `Job`, `ConfigMap`, etc) in its scheduled
      KubernetesCluster.
    * Copying the templated resource's `.status` into its own `.status.remote`.

This design ensures `KubernetesApplication` is our atomic unit of scheduling,
while making it possible to reflect the status of each templated resource on the
`KubernetesApplicationResource` that envelopes it. Resources templated by a
`KubernetesApplicationResource` are opaque to the Crossplane API server - their
group, version, and kind need only be known to the Kubernetes cluster upon which
they're scheduled. A `KubernetesApplicationResource` may be retroactively added
to or removed from a `KubernetesApplication` after it has been created by
updating the application's templates.

### Controller Ownership
Kubernetes object metadata allows any resource to reflect that it is owned by
one or more resources. Exactly one owner of a resource may be [marked as its
controller][7]. A `Pod` may mark a `ReplicaSet` as its controller, which in turn
may mark a `Deployment` as its controller. Controllers are expected to respect
this metadata in order to avoid fighting over a resource.

This is relevant in the case of two `KubernetesApplications` both containing a
template for a `KubernetesApplicationResource` named `cool`. Despite the desired
one-to-many application-to-resource relationship both controllers would assume
they owned the `KubernetesApplicationResources`, resulting in a potential
many-to-many relationship and undefined, racy behaviour. The application
controller must use controller references to claim its templated
`KubernetesApplicationResources`.

The relationship between an application and its resource templates is as
follows:

1. The application controller takes a watch on all `KubernetesApplications` and
   `KubernetesApplicationResources`. Any activity for either kind triggers a
   reconciliation of the `KubernetesApplication`.
1. During each reconciliation the controller:
   * Attempts to create or update a `KubernetesApplicationResource` for each of
     its extant templates. This will fail if a named template conflicts with an
     existing `KubernetesApplicationResource` not controlled (in the controller
     reference sense) by the `KubernetesApplication`.
   * Iterates through all extant `KubernetesApplicationResources`, deleting any
     resource that is controlled by the application but that does not match the
     name of an extant template within the application's spec.
1. The application controller uses the `foregroundDeletion` finalizer. This
   ensures all of an application's controlled resource templates are
   [garbage collected][8] (i.e. deleted) upon deletion of the application.

A `KubernetesApplication` can *only* ever be associated with the
`KubernetesApplicationResources` that it templates; a `KubernetesApplication`
will never orphan or adopt orphaned `KubernetesApplicationResources`. This is
in line with the [controller reference design][7], which states:

> If a controller finds an orphaned object (an object with no ControllerRef)
> that matches its selector, it **may** try to adopt the object by adding a
> ControllerRef. Note that **whether or not the controller should try to adopt
> the object depends on the particular controller and object**.

The controller reference pattern applies only to resources defined in the same
API server. It uses a [`metav1.OwnerReference`][11] that assumes the controlling
resource exists in the same cluster and namespace as the controlled resource.
Consider two resource templates, both owned by the same application and thus scheduled
to the same cluster:

* A `KubernetesApplicationResource` named `coolns/cooldeployment`, templating a
  `Deployment` named `remotens/cooldeployment`
* A `KubernetesApplicationResource` named `coolns/lamedeployment`, _also_
  templating a `Deployment` named `remotens/cooldeployment`, but with a
  different `.spec.template.spec`.

In this example the two resource templates will race to create or update
`remotens/cooldeployment`. The resource controller will avoid this race by
adding annotations to the remote resource templated by a particular resource
and obeying the three laws of controllers. All remote resources owned by a
`KubernetesApplicationResource` will be annotated with key
`kubernetesapplicationresource.workload.crossplane.io/uid` set to the UID of
the `KubernetesApplicationResource` that created the remote resource.

### Validation Webhooks
All Crossplane resources, including `KubernetesApplication` and
`KubernetesApplicationResource`, are [CRDs][12]. CRDs are validated against an
OpenAPI v3 schema, but some kinds of validation [require][13] the use of a
[`ValidatingAdmissionWebhook`][14]. In particular a webhook is required to
enforce immutability; it's not possible via OpenAPI schema alone to specify
fields that may be set at creation time but that may not be subsequently
altered.

The design proposed by this document requires a handful of fields be immutable.
Updating a `KubernetesApplication`'s `.spec.clusterSelector` would require all
resources be removed from the old cluster and recreated on the new cluster. This
is more cleanly handled by deleting and recreating the application. The cluster
selector should be immutable.

A `KubernetesApplicationResource`'s `.spec.template.kind`,
`.spec.template.apiVersion`, `.spec.template.name`, and
`.spec.template.namespace` fields must also be immutable. Changing any of these
fields after creation time would cause the templated resource to be orphaned and
a new resource created with the new kind, API version, name, or namespace. The
controller-runtime library upon which Crossplane is built [does not expose the
old version of an object during updates][15], making it impossible to determine
whether these fields have changed, but [validating webhooks do][16].

Crossplane does not currently leverage Kubernetes webhooks, controller-runtime
has [support for both validating and mutating admission webhooks][17]. This
document proposes two validating webhook be added to Crossplane; one each of
`KubernetesApplication` and `KubernetesApplicationResource` to enforce
immutability of the aforementioned fields.

## Alternatives Considered
The following alternative designs were considered and discarded or deferred in
favor of the design proposed by this document.

### Loosely Coupled KubernetesApplicationResources
The proposed relationship between a `KubernetesApplication` and its
`KubernetesApplicationResources` is unlike that of any built in Kubernetes
controller resources and their controlled resources. Most controller resources
(as opposed to controller logic) include a single template that is used to
create one or more identical replicas of the templated resource; `ReplicaSet` is
an example of this pattern; a `ReplicaSet` includes a single pod template that
is used to instantiate N homogenous replicas. A `KubernetesApplication` on
the other hand includes one or more _heterogenous_ resource templates that are
used to instantiate one or more heterogenous resources. This pattern is closer
to the relationship between a `Pod` and its containers, except that Kubernetes
does not model containers as a distinct API resource.

Managing a set of heterogeneous resources is more complicated than managing
several homogenous replicas. A `ReplicaSet` can support only a handful of
operations:

* Increase running replicas by instantiating `N` randomly named `Pod` resources
  from its current pod template.
* Decrease running replicas by deleting `N` random controlled `Pods`.
* Update its pod template. Note that doing so does not affect running `Pods`,
  only `Pods` that are created in future scale ups.

A `KubernetesApplication` must support:

* Creating a `KubernetesApplicationResource` that has been added to its set of
  templates. This resource template has an explicit, non-random name, increasing
  the likelihood of an irreconcilable conflict with an existing
  `KubernetesApplicationResource`.
* Deleting a `KubernetesApplicationResource` that has been removed from its set
  of templates. There's no reliable way to observe the previous generation of
  the application, so the controller logic must assume any resource template
  referencing the application as its controller that does not match an extant
  template's name should be deleted.
* Updating a `KubernetesApplicationResource`.

One alternative to the pattern proposed by this design is closer to the loosely
coupled relationship between a `Service` and its backing `Pods`; the Crossplane
user would submit a series of `KubernetesApplicationResources`, then group them
all into a co-scheduled unit via a `KubernetesApplication` via a label selector.
A `KubernetesApplication` would be associated with its constituent
`KubernetesApplicationResources` purely via label selectors (and controller
references) rather than actively managing their lifecycles based on templates
encoded in its `.spec`. This defers conflict resolution to the Crossplane user
and avoids unwieldy, potentially gigantic, `KubernetesApplication` resources.

The main drawback of this loosely coupled approach is that the system is
eventually consistent with the user's intent. When all desired resources are
specified as templates in the application's `.spec` it's always obvious how many
resources the user desired and how many have been successfully submitted. If a
resource template is invalid the entire application will be rejected by the
Crossplane API Server. In the loosely coupled approach the invalid
`KubernetesApplicationResource` would be rejected by the API server, but the
`KubernetesApplication` would, according to the API server, otherwise appear to
be a healthy application that happens to desire one less resource than the user
intended.

### Monolithic Workloads
This alternative proposes a 'monolithic' workload. A monolithic workload is
similar to the design proposed by this document but with the various resources
and statuses nested directly within the `KubernetesApplication` rather than via
the interstitial `KubernetesApplicationResource` resource.

An example monolithic complex workload:
```yaml
---
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: demo
spec:
  clusterSelector:
    provider: gcp
  resources:
  - name: demo
    secretName: demo
  resourceTemplates:
  # The monothlic workload does not template KubernetesApplicationResources, but
  # instead templates arbitrary Kubernetes resources directly.
  - apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: wordpress
      labels:
        app: wordpress
    spec:
      selector:
        app: wordpress
      template:
        metadata:
          labels:
            app: wordpress
        spec:
          containers:
            - name: wordpress
              image: wordpress:4.6.1-apache
              ports:
                - containerPort: 80
status:
  cluster:
    namespace: cool
    name: theperfectkubernetescluster
  conditions:
  - lastTransitionTime: 2018-10-02T12:25:39Z
    lastUpdateTime: 2018-10-02T12:25:39Z
    message: Successfully submitted cool/supercoolwork
    status: "True"
  remote:
  # There's no distinct API resource within the Crossplane API server with which
  # to associate the status of each remote resource, so instead we maintain an
  # array of statuses 'keyed' by their resource's type and object metadata.
  - apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: wordpress
      labels:
        app: wordpress
    status:
      replicas: 2
      availableReplicas: 2
      unavailableReplicas: 2
      observedGeneration: 3
      conditions:
      - lastTransitionTime: 2016-10-04T12:25:39Z
        lastUpdateTime: 2016-10-04T12:25:39Z
        message: Replica set "nginx-deployment-4262182780" is progressing.
        reason: ReplicaSetUpdated
        status: "True"
        type: Progressing
```

The monolithic workload design is functionally close to that proposed by this
document, but has two major drawbacks:

* Representing the status of remote resources would become unwieldy. Each
  `KubernetesApplication` would need to maintain a map of resource statuses
  keyed by their type and object metadata.
* It precludes breaking out the logic of the workload controller into separate
  application and resource controllers, resulting in a single more complicated
  controller.

It's worth noting that this monolithic design has a lot of symmetry with the
relationship between a `Pod` and its containers. Containers are not modelled as
distinct Kubernetes API resources, and are always coscheduled to a node, much as
resources under the monolithic design are always coscheduled to a Kubernetes
cluster and are not modelled as distinct API resources in the Crossplane API
server. Container status is modeled as an array 'keyed' by container name.

### Not Polling KubernetesApplicationResource Status
Both the contemporary and proposed workload designs poll the status of the
resources they create in their scheduled cluster, reflecting them in the status
of the `Workload` or `KubernetesApplicationResource` that created them. This
allows a Crossplane user to inspect the status of the resources they created in
a remote cluster without ever explicitly connecting to said cluster.

Resource statuses have arbitrary schemas; there is no standard even amongst
built in types. This makes it impossible to consistently model the health of a
resource managed by a resource. The status field exposed by a healthy
`Deployment` is completely different from the status field exposed by a healthy
`Ingress`, let alone the status field exposed by a custom resource. This forces
both the controller code and the `KubernetesApplicationResource` CRD OpenAPI
validation specification to treat status as an opaque JSON object.

One alternative would be to avoid polling the status altogether; resource
templates would simply reflect that they had submitted their templated resource
to their scheduled `KubernetesCluster` either successfully or unsuccessfully. It
would be left as an exercise for the Crossplane user to connect to the scheduled
cluster, locate the managed resources, and inspect them directly.

### 'Federation Style' Envelope Resources
The Kubernetes [Federation project][18] has similar but not identical goals to
Crossplane's workloads. Federation defines Kubernetes resources in one cluster
which runs controllers that propagate said resources to another set of clusters.

Federation v2 uses 'envelope' resources similar to the proposed
`KubernetesApplicationResource`, but with stronger typing. A federated resource
of kind `<K>` is specified using a `Federated<K>`, for example a `Service` is
modeled using a `FederatedService`. These `Federated<K>` envelopes are CRDs
generated via a [command line tool][19] that [introspects the underlying
resource][20]. `Federated<K>` is associated with `<K>` via a
[`FederatedTypeConfig`][21]. The federation controller watches for
`FederatedTypeConfig`, [creating two more controllers][22] for each
`Federated<K>` referenced by a `FederatedTypeConfig`. One controller is
responsible for propagating the `Federated<K>`'s templated `<K>` resource to the
clusters upon which it is scheduled while the other is responsible for polling
the status of the managed resources.

Crossplane could replace `KubernetesApplicationResource` with a series of
resources similar to the `Federated<K>` envelope resources, for example
`Cross<K>`. This is appealing because it allows for stronger typing; generating
a `Cross<K>` analog to a resource would require introspecting `<K>`, allowing
the `Cross<K>` to derive the schema for its `.spec.template` and
`.status.remote` fields from the underlying `<K>` kind.

Unfortunately this approach has several detractors:

* It requires the Crossplane API server to understand each kind of resource that
  it wishes to propagate as part of a workload. Assuming a resource of kind
  `Cool` is specified via a CRD, said CRD must be applied to the Crossplane API
  server before a `CrossCool` can be generated.
* Even when the `Cool` CRD has been applied to the Crossplane API server
  Crossplane does not have a Go object to associate with said CRD and thus must
  resort to using `*unstructured.Unstructured` and `json.RawMessage` to
  represent the kind's template and status.
* Additional complexity is introduced in order to generate strongly typed
  envelopes. The Federation project requires the operator to explicitly create
  these envelope CRDs by running a command line tool. A Crossplane controller
  could automate this by watching for [`APIResource`][23].
* Applications cannot be associated with several different resource kinds by
  label selector alone. It's possible to get all of a particular resource kind
  by label (e.g. `kubectl get pod -l thislabel=cool`) but it's not possible to
  get all resources (e.g. `kubectl get all -l thislabel=cool`). Workloads would
  need to be associated to strongly typed envelope kinds via either an array of
  `corev1.ObjectReferences`, or a label selector and an array of kinds.

A Federated resource status is still a `map[string]any` in the
controller code:
```go
type FederatedResource struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	ClusterStatus []ResourceClusterStatus
}

type ResourceClusterStatus struct {
	ClusterName string
	Status      map[string]any
}
```

### Propagating 'Remote Controller' Resources
One alternative to a simple annotation representing that a remote resource is
owned by a `KubernetesApplicationResource` is to model said ownership using a
distinct resource in the `KubernetesCluster` to which a
`KubernetesApplicationResource` is scheduled. This resource would act as the
controller reference of the remote, templated resource. Assuming we named this
intermediary resource `CrossplaneApplicationResourceReference` a `Deployment`
templated by a `KubernetesApplicationResource` in the Crossplane API server
would be 'owned' (in the controller reference sense) by a
`CrossplaneApplicationResourceReference` in the remote cluster:

```yaml
---
apiVersion: workload.crossplane.io/v1alpha1
kind: CrossplaneApplicationResourceReference
metadata:
  name: demo
remote:
  apiServer: https://some.crossplane.apiserver.example.org
  # Everything below represents the controlling resource in the controlling
  # Crossplane API server.
  apiVersion: workload.crossplane.io/v1alpha1
  kind: KubernetesApplication
  metadata:
    name: demo
    namespace: demo
  uid: some-cool-uuid
```

An intermediary resource would provide context to uninitiated users of the
remote Kubernetes as to what a Crossplane is and which Crossplane instance is
managing a particular resource, but comes at the expense of increased
complexity. Crossplane would need to propagate the
`CrossplaneApplicationResourceReference` CRD to each cluster it managed, and
manage a `CrossplaneApplicationResourceReference` for every actual remote
resource. This complexity is only worthwhile if it is expected that Crossplane
will frequently deploy applications to clusters that are also used directly by
users who are unfamiliar with Crossplane.

### Special Handling for Cluster Scoped Resources
Namespaced resources often depend on cluster scoped resources; `Namespace` and
`CustomResourceDefinition` for example are cluster scoped resources that are
used by namespaced resources. The order in which the resource templates of an
application are reconciled are undefined. This means that, for example, an
application consisting of a resource templating a `Namespace` and another
resource templating a `Deployment` to be created in said namespace may take a
few reconcile loops to be created:

1. Random chance causes the resource templating the `Deployment` to be
   submitted first. This fails due to the `Deployment` targeting a name that has
   yet to be created. The reconcile of this resource is requeued.
1. The resource containing the `Namespace` is submitted successfully.
1. The resource containing the `Deployment` tries again. It now succeeds.

One way to avoid this would be to break a large application up into smaller
ones, applied sequentially. The issue here is that there is no guarantee the
second `KubernetesApplication` will be scheduled to the same cluster as the
first. The first application could add a label to the `KubernetesCluster` it is
scheduled to that the second could select, but this devolves into a flawed
dependency system. The requirements of the second `KubernetesApplication` are
not considered when the first is scheduled, despite the fact that they must be
co-scheduled.

Another alternative is to allow `KubernetesApplicationResources` to be
associated directly with a `KubernetesCluster` (instead of a
`KubernetesApplication`) via a label selector. This circumvents the scheduling
of a `KubernetesApplication`; the `KubernetesCluster` controller would find all
associated resource templates and explicitly 'schedule' them to itself when
instantiated. This pattern could be used to model resource templates that were more
strongly associated with the cluster itself rather than applications running
upon it, for example ensuring every `KubernetesCluster` ran a functional ingress
controller or had a base set of `ClusterRoles` available.

Per [Secret Propagation](#secret-propagation) this document proposes
`KubernetesApplicationResources` use a set of `Secret` references rather than a
set of managed resource references. Doing so makes the purpose of the field
clearer given that it is in practice only used to propagate connection
`Secrets`. If there are worthwhile uses for associating managed resources or
managed resource claims with a `KubernetesApplicationResource` beside connection
`Secret` propagation it would be preferable to maintain the contemporary
`Workload` pattern of taking a set of managed resource references rather than
`Secrets`. One speculative use could be to automatically ensure connectivity
between said managed resources and the `KubernetesCluster` to which their
consuming Kubernetes resources are scheduled.

Referencing Crossplane managed resources or resource claims in a fashion that
avoids the flaws of the contemporary design (see [Secret
Propagation](#secret-propagation) for details) is complicated by the fact that
the controller must know whether the referenced managed resource is concrete or
a claim (i.e. an `RDSInstance` or a `SQLInstance`). This is difficult because
Crossplane managed resources and claims are Kubernetes resources with arbitrary
kinds, e.g. `RedisCluster`, `Bucket`, `RDSInstance`, `CloudMemorystoreInstance`,
etc.

[1]: https://crossplane.io
[2]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
[3]: https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured#Unstructured
[4]: https://golang.org/pkg/encoding/json/#RawMessage
[5]: https://michaelwhatcott.com/go-code-that-stutters/
[6]: https://github.com/kubernetes-sigs/application/blob/0cbebd3/README.md#application-objects
[7]: https://github.com/kubernetes/community/blob/a6dcf86/contributors/design-proposals/api-machinery/controller-ref.md
[8]: https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#controlling-how-the-garbage-collector-deletes-dependents
[9]: https://github.com/kubernetes/community/blob/a6dcf86/contributors/design-proposals/api-machinery/controller-ref.md#the-three-laws-of-controllers
[10]: https://github.com/kubernetes/community/blob/a6dcf86/contributors/design-proposals/api-machinery/controller-ref.md#adoption
[11]: https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#OwnerReference
[12]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[13]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#advanced-features-and-flexibility
[14]: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook
[15]: https://github.com/kubernetes-sigs/controller-runtime/issues/399
[16]: https://godoc.org/k8s.io/api/admission/v1beta1#AdmissionRequest
[17]: https://book.kubebuilder.io/beyond_basics/what_is_a_webhook.html
[18]: https://github.com/kubernetes-sigs/federation-v2
[19]: https://github.com/kubernetes-sigs/federation-v2/blob/f9555f7/cmd/kubefed2/main.go
[20]: https://github.com/kubernetes-sigs/federation-v2/blob/2f49b7b/pkg/kubefed2/enable/enable.go#L317
[21]: https://godoc.org/sigs.k8s.io/federation-v2/pkg/apis/core/v1alpha1#FederatedTypeConfig
[22]: https://github.com/kubernetes-sigs/federation-v2/blob/46d3f9a/pkg/controller/federatedtypeconfig/controller.go#L221
[23]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#apiresource-v1-meta

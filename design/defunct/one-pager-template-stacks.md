# Templates Stacks

* Owner: Marques Johansson (@displague)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Outline

* [Background](#Background)
* [Terms and Acronyms](#Terms-and-Acronyms)
* [Benefits of Template Stacks](#Benefits-of-Template-Stacks)
  * [Simple Variable Substitution](#Simple-Variable-Substitution)
  * [No Custom Controller Needed](#No-Custom-Controller-Needed)
  * [Composition](#Composition)
* [Template Stack Manager](#Template-Stack-Manager)
  * [Deployment](#Deployment)
  * [Owner References](#Owner-References)
  * [RBAC](#RBAC)
  * [Life Cycle and Updates](#Life-Cycle-and-Updates)
  * [Unknown Types](#Unknown-Types)
* [Examples](#Examples)
* [Related Issues](#Related-Issues)

## Background

Early thinking on Stacks included the idea of simplifying the overhead involved
in life-cycle management for complex workloads, especially as this relates to
consuming Crossplane managed services. However, to contribute a new Stack in the
Stack eco-system today, it is necessary to create a new Controller, typically in
Go using Kubebuilder.

We believe that a more robust Stack ecosystem would emerge if the requirement to
write new controllers was removed. The experience for users would remain the
same, while developers would have a simpler alternative for simpler use-cases.

Clearly, there must be a controller or actor at some level to produce the
desired state, operating within a cluster or outside of a cluster.

A template controller would be some process that could examine template
containing records (Custom Resources) within a cluster. It would then expand
those records into new records rendered by some template engine. The templates
would be processed using the template itself and some set of user supplied data
and some set of cluster state.

## Terms and Acronyms

* **Stacks**
  : Crossplane Stacks provide a means of deploying isolated and appropriately
  role restricted `Deployment` resources to manage a set of CRDs.
  
  UNDER THIS PROPOSAL, additional features would be added.
* **Crossplane Control Cluster**
  : Any Kubernetes cluster where a Crossplane Deployment is installed. This does
  not include Crossplane Managed Clusters
  
* **Crossplane Managed Cluster**
  : A Kubernetes cluster being managed by Crossplane, which may or may not have
  Crossplane installed within that cluster
* **SM (Stack Manager)** : The cluster privileged deployment that manages
  `Stack`, `ClusterStackInstall` and `StackInstall` resources. One per
  Crossplane cluster. The Stack Manager creates `Deployment` resources as
  defined by a `Stack` to handle resources of the CRD types that `Stack`
  manages. The Stack Manager creates `Stack` resources based on `StackInstall`
  and `ClusterStackInstall` resources.
* **CRD (Custom Resource Definition)** : A standard Kubernetes Custom Resource
  Definition
* **CR (Custom Resource)** : An instance of a Kubernetes type that was defined
  using a CRD
* **GVK (Group Version Kind)** : The API Group, Version, and Kind for a type of
  Kubernetes resource (including CRDs)
* **TS (Template Stack)** : UNDER THIS PROPOSAL, a Stack that declares dependent
  resources and includes managed CRDs, but includes templates to be rendered for
  each CR rather than a `Deployment` controller
* **TSM (Template Stack Manager)** : UNDER THIS PROPOSAL, a restricted
  `Deployment` that manages resources for a Template Stack's CRDs

## Introduction

### In Scope

This design introduces the concept and benefits of Template Stacks as well as
exploring a proposed Template Stack Manager which may facilitate this
experience. Implementation details and challenges for the existing Stack Manager
and the proposal for a Template Stack Manager are in scope. The life cycle and
security boundaries of these applications are in scope.

### Out of Scope

Throughout the design, Go templates are used to illustrate potential usage, but
this is an implementation detail that will be the subject of additional design
documents. In fact, Template Stacks should eventually benefit from having more
than one template engine option.

This design is also not heavily concerned with the format of the `Stack` object
or the layout of the Stack filesystem. Those topics will be discussed in another
one-pager or subsequent updates to this design. It is assumed that the reader
has some familiarity with [`Stack`
resources](https://github.com/crossplane/crossplane/blob/master/design/design-doc-packages.md#stack-crd)
and the way [they are automatically created from StackInstall resources](https://github.com/crossplane/crossplane/blob/master/design/design-doc-packages.md#installation-flow).

Stack resources that are managed as Template Stacks should require Namespaced
scoping. Cluster scoped Template Stacks may require additional thinking.

The ultimate Stack format will need to be open for use by multiple Template
Stack Engines, as the format will certainly undergo changes from this proposal through the
[Template Stacks UX design](https://github.com/crossplane/crossplane/issues/915).

Additional thinking to support multiple template engines is not in scope for
this proposal. Can multiple engines be used from a single Template Stack? Can
these engines share resources and resource values? How does engine choice affect
the Stack fields (if at all) and the validation of resources at build and
install time. This proposal will not investigate those questions.

Finally, the specific resources that a Template Stack may render is not in scope
for this design. There is some thinking on this included in [Concerning the use
of Core Types](#Concerning-the-use-of-Core-Types).

## Benefits of Template Stacks

The current design of Stacks allows for a new CRD to be managed by an
independent controller. A CR will be handled by the Stack controller in any way
that controller sees fit. This includes issuing remote API calls for additional
inputs or creating external outputs, accessing local resources, or performing
complex computations.

Stacks may also opt to take advantage of the Crossplane Runtime to simplify the
life cycle management of resources.

In contrast, this proposal introduces Template Stacks with the limited ability
to use the CR as input to create, update, or delete dependent Kubernetes
resources. Once the dependent resources have been created they can also be used
as input, their outputs can be recycled as inputs.

Application owners and managers can take advantage of Template Stacks to package
complex deployments, while users benefit from a simple installation and usage
interface. Developers can use Template Stacks as application owners would, to
create Kubernetes resource machinery using template engine logic, circumventing
the need for more complex controllers.

The Template Stack Manager will permit the use of template variables to create
and manage a set of resources provided as a set of strings included in the
definition of the Stack.

The benefits of Template Stacks are as follows:

* [Simple Variable Substitution](#Simple-Variable-Substitution)
  Stack Template resources can take input from a CR spec or from sibling
  resources.
* [No Custom Controller Needed](#No-Custom-Controller-Needed)
  A single all-purpose controller manages Template Stacks. The existing Stack
  Manager will wire up the Template Stack Manager to manage the resources defined in a Template Stack.
* [Composition](#Composition)
  Stacks can be nested, and may reside in Crossplane managed clusters or control clusters.
  
  Common charts and custom YAML files used today reside on local disks.
  With Template Stacks these templates and can reside in the cluster.

  Template Stacks introduce template variables that can take advantage of the
  field values from other template rendered resources. Template variable
  substitution is performed in each Kubernetes reconciliation pass, affecting
  all template resources for a given Stack CRD.

### Simple Variable Substitution

If an application owner wanted to parameterize a single resource within a
more complex set of YAML resources, say the creamer type for a Coffee resource,
we could imagine placing a `creamer` template variable in that `Coffee`.

Supposing we used an inline handlebar template syntax, the `Coffee` would contain:

```yaml
apiVersion: template.stacks.example.org/v1alpha1
kind: Coffee
metadata:
  name: delicious
spec:
  creamer: {{.coffeeCreamer}}
```

Today, Kustomize overlays can be used to provide this level of functionality, as
can Helm chart values. However, the capabilities provided by these tools are
limited in two ways:

* Original template values are not stored in the cluster
* Replacement values can not be taken from other resources

Template Stacks will provide both of these advantages. _([Cross resource
references](https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md)
also provide this feature for Stacks with custom controllers.)_

Common template value files today live on developer or operator laptops where
the whole file or past versions can be lost. They do not benefit from the same
security and retention policies put in place to protect the cluster. Template
Stacks values are stored in the CR, which make them easy to locate and examine.

Even with backups, git, or online storage techniques, the template values are
not available within the cluster, which is where processing happens. Common
template values today can not be dynamically updated based on the conditions of
the environment in which they operate. Custom controllers or external scripts
are needed to fetch values from the Kubernetes cluster. Template Stacks provide
these capabilities, within the cluster, where the resources and operations are
regulated as they are for all Kubernetes resources.

*To apply the example above to this Template Stacks proposal,
`{{.coffeeCreamer}}` would actually be `{{.spec.coffeeCreamer}}`. A `Breakfast`
resource would include a `Coffee` resource template. The `.spec.coffeeCreamer`
property of a `Breakfast` resource would be applied to the template and rendered
as new, updated, or removed Kubernetes `Coffee` resource with the appropriate
creamer.*

#### Default values

Default values for Template Stacks should be supplied by the CRD author.

The following example CRD shows a `Breakfast` resource definition with a
`coffeeCreamer` `spec` property
[defaulted](https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/20190426-crd-defaulting.md)
to `none`.

```yaml
apiVersion: "apiextensions.k8s.io/v1beta1"
kind: "CustomResourceDefinition"
metadata:
  name: "breakfast.example.crossplane.io"
spec:
  group: "example.crossplane.io"
  version: "v1alpha1"
  scope: "Namespaced"
  names:
    plural: "breakfasts"
    singular: "breakfast"
    kind: "Breakfast"
  validation:
    openAPIV3Schema:
      required: ["spec"]
      properties:
        spec:
          required: ["coffeeCreamer"]
          properties:
            coffeeCreamer:
              type: "string"
              default: "none"
```

This specification does not propose any additional means to supply default field
values. The omission of an alternative mechanism for setting default values
should encourage resource authors to take advantage of the latest [Kubernetes
CRD features](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#defaulting).

The Template Stack Manager should be capable of replacing controllers in
a range of use-cases. To that end CRDs should not need to be tailored for use by
the TSM. The pros and cons of this viewpoint are left for future discussion.

Offering a means to provide default values would fit Template Stacks to more CRD
adoption scenarios, especially in cases where Template Stacks were not
considered at the inception of the CRD.

### No Custom Controller Needed

This is where the value of Template Stacks starts to materialize. A stack author
doesn't need to write their own controller, because a common controller is
provided with the Template Stacks Manager.

Existing Stacks must specify a `Deployment`. These are typically written in Go
and generally require understanding some combination of client-go,
controller-runtime, crossplane-runtime, and Kubebuilder.

For some applications, the intended work of the controller could be simple:

* For every new resource, create a set of Kubernetes resources
* Use the spec fields of that resource to affect the Kubernetes resources
* Upon changes to these spec fields, update the Kubernetes resources

These are responsibilities the Template Stack Manager will handle.

### Composition

#### Vertical Composition

Crossplane Stacks manage multiple CRDs. Template Stacks should be no different.

Template Stack `template` definitions will need to express in text the resources
to render for each CRD that is managed. The GVK can be used as a map key to
denote the CRD template being described.

```yaml
kind: Stack
metadata:
  name: RedisStack
spec:
  customresourcedefinitions:
  - kind: Redis
    apiVersion: redis.example.org/v1
  templates:
    redis.example.org/v1:
      deployment: |
          …
          containers:
          - name: redis-controller
            image: example/redis-controller:{{.spec.redisVersion}}
```

*In this example, templates are found within a map keyed by the GVK and a
template grouping name. The reasoning for this map format will be explained in
a future design.*

With a more complex example, we see that each template needed to render a GVK
should be provided with a name that represents the intent of that template.
This distinguishing name can be reused to reference the template and its single
resource (see [Horizontal Composition](#Horizontal-Composition)).

##### Nesting Template Stacks

In the example below, distinct templates are defined for the Redis CRD and the
CachingWebService. CachingWebService includes two templates, one of which relies
on the other.

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  name: CachingWebServiceStack
spec:
  customresourcedefinitions:
  - kind: Redis
    apiVersion: redis.example.org/v1
  - kind: CachingWebService
    apiVersion: cachingwebservice.example.org/v1
  templates:
    redis.example.org/v1:
      deployment: |
        kind: Deployment
        …
          containers:
          - name: redis-controller
            image: example/redis-controller:{{.spec.redisVersion}}
    cachingwebservice.example.org/v1:
      cache: |
        apiVersion: example.org/v1
        kind: Redis
        …
        spec:
          redisVersion: "5"
      web: |
        kind: Deployment
        …
          containers:
          - name: nginx-controller
            image: example/nginx-controller:{{.spec.nginxVersion}}
--
apiVersion: example.org/v1
kind: CachingWebService
metadata:
  name: cacheme
spec:
  nginxVersion: "1.17.4"
```

*When the `CachingWebService` instance is reconciled by the TSM, Kubernetes
resources represented by the `cache` and `web` templates will be produced. These
templates are effectively concatenated by the TSM, as if to form a single
`cachingwebservice.yaml`. The TSM then applies the template variables and
installs these resources as though `kubectl apply -f cachingwebservice.yaml` was
invoked.*

The `nginxVersion` is defined in the `spec` of the `CachingWebService` instance,
while `redisVersion` has been declared in the `cache` template for
`cachingwebservice.example.org/v1`. In each case, a template variable has been
passed into a dependent template. One was user supplied, the other was template
supplied.

An additional form of composition and reuse is available by referencing Template
Stack kinds within new Template Stacks. For example, `MegaWebsite` could include
`CachingWebServer` in the `dependsOn` field of that Stack.

#### Using Reconciled Values as Template Variables

Kubernetes resources typically set their state within a `status` field, which is
often available as a Status Sub-Resource. In fact, CRD resources specifically
support scale and status sub-resources. Access to this sub-resource can be
granted independently from the resource itself.

The template capabilities available to a Template Stack should not be restricted
for use within the `spec` or other resource creation features.

Template Stack managed resources should be able to manage their status, as any
controller managed resource would. Arguably, the most useful values to expose
through this status would be derived from resources created with templates.

While maintaining that the `spec.templates` field of a `Stack` can present the
entire template body of CRD resources, let `templateStatus` present a template
for a resource's `status` body. As these `templateStatus` templates will define
the `status` of a single CR, we will only need a GVK key to identify which
`templateStatus` is being described.

```yaml
…
kind: Stack
metadata:
  name: Foo
spec:
  customresourcedefinitions:
  - kind: Foo
    apiVersion: foo.example.org/v1
  template:
    foo.example.org/v1:
      templateA: |
        kind: DependentThing
        spec:
        …
  templateStatus:
    foo.example.org/v1: |
        statusField: {{.some.value}}
```

*If the CRD for `example.example.org/v1` includes the `status` sub-resource, the
Template Stack Manager can potentially make `status` sub-resource setting
optimizations. Otherwise, the TSM can set the status by updating the while
`Foo` resource.*

Crossplane Stacks, like all Kubernetes resources, benefit from an active state
reconciliation loop. Status values may not be immediately available. The
template syntax should allow for templates to overcome these situations, for
example (with Go templates):

```yaml
  templateStatus:
    example.example.org/v1: |
      {{if pipeline}}
      statusField: {{.some.value}}
      {{end}}
```

##### Restrictions on Use

In order for templates to be resolved when their corresponding Kubernetes
resources are not available, some restrictions must be made on where templates
can be used. At install or build time, each resource template should be
inspected to verify that apiVersion, kind, and name exist within the template
and can be parsed from the template without the need for dependent or mutable variables.

A single unique name declared within a template creates a limitation of its own.
With that limitation, a Template Stack could only have one resource defined per
namespace. Fortunately, the UID of the resource is immutable and would be
available for use within templates.

##### Labels and Annotations

Labels and Annotations provide users with countless extensibility and
categorization options. Template Stacks should permit these field values to be
retained between resource updates. Likewise, the Template Stack author should
have the ability to reset the labels and annotations. Through the use of list
merging functions, any Template Stack engine should give the Template Stack
author the means to define the desired behavior for their Stack.

```yaml
  templates:
    example.example.org/v1:
      fooresource: |
      ...
      metadata:
        annotations:
          {{- range $key, $value := (mergeFn .fooresource.metadata.annotations ["foo"]) }}
          - {{$key}}: {{$value}}
          {{- end }}
```

#### Horizontal Composition

We've now demonstrated several properties of Template Stacks:

* a means to concatenate Kubernetes Resource YAML
* nesting supported CRD types or the CRDs of other Stacks
* a means to set the status of a resource using source or template generated
  properties

Another property that is possible in a home-grown controller, is the ability to
use the status of any one resource that controller manages to affect other
resources. Template Stacks should offer this same capability.

```yaml
kind: Stack
metadata:
  name: Foo
spec:
  templateStatus:
    foo.example.org/v1: |
        statusField: {{.some.value}}
  template:
    foo.example.org/v1:
      controller: |
        kind: Thing
        spec:
        …
      other: |
        kind: DependentThing
        spec:
          someInput: {{.controller.status.statusField}}
```

*The `DependentThing` included in the `other` template relies on the resolved
`statusField` from the `Thing` included in the `controller` template.*

We will not focus on how the names of adjacent templates are exposed in a
template syntax, just that they should be made available.

The set of variables to present to a template should include:

* The Stack managed object (the `Foo`)
* The set of dependent resource objects (the `DependentThing`)
* The set of errors encountered rendering each dependent resource
  This will allow `Foo` to reflect errors rendering `DependentThing` as a conditioned status using `templateStatus`.
* Helper functions for string manipulation, list handling, and simple numeric
  operations, such as <https://github.com/leekchan/gtf#index> and <https://github.com/Masterminds/sprig>

As Go text templates, sibling dependent resources including their templates,
values, and errors could be exposed as either properties or functions.

In the future, we may choose to expose:

* The errors encountered rendering the templateStatus
* Current and new resource values when possible
* The raw templates themselves. This may be useful for storing partial templates
  in the Stack. The key name would not match a GVK in this case, which means it
  could be an arbitrary identifier. The key could resemble a filepath, which
  could help in the migration of chart style "includes". With the addition of an
  "include" function in the template engine, this could be possible:

   ```yaml
   kind: Stack
   spec: { name: "Foo" }
   templates:
     this/name/can/be/anything:
       so/can/this/name: |
         Reusable partial template {{.someVar}}.
         The keys do not have to match GVKs owned by this Stack,
         permitting this use case.
     foo.group/version:
       resourceTemplate:
          kind: DependentThing
          spec:
             text: {{ include "this/name/can/be/anything" "so/can/this/name" }}
   ```

Introducing a depending on reconciled fields is bound to inject latency into a
full resolution. Instead of relying on a static set of inputs, our inputs can
now be dynamic. The inputs are driven by an independent, yet necessarily
sibling, resource.

While it may be possible to draw on values from resources outside of the CRD
whose template is being defined, we will not entertain that possibility. But if
we were to entertain it, we might take advantage of a template function capable
of looking up and returning the specified resource.

Accessing sibling resources properties will require:

* Determining the type of the sibling resource

  The type is provided in the resource template.
* Determining if the resource is `Cluster` scoped or `Namespaced`

  This scoping can be determined once the type is known.
* Identifying the resource

  The TSM can assume the namespace it is operating in for a namespaced Template
  Stack, and may need to enforce this from templates.
* Accessing to the resource

  The TSM has the necessary roles to access any objects it created.
* Walking the property map of the resource
* Returning a string representation of the resource property

## Template Stack Manager

Currently, Crossplane `Stack` and `StackInstall` resources are managed by the
Stack Manager. This controller will reconcile new `StackInstall` resources by
examining the metadata included within the requested Stack's filesystem and
creating a `Stack` resource that includes `Deployment` to act as a controller
for CRDs managed by that Stack. This `Deployment` is defined in the Stack's
`spec.controller.deployment` field.

```yaml
kind: Stack
metadata:
  name: RedisStack
spec:
  customresourcedefinitions:
  - kind: Redis
    apiVersion: redis.example.org/v1alpha1
  controller:
    deployment:
      <typical Kubernetes Deployment resource>
…
```

*The `deployment` value is populated from an `install.yaml` file contained
within the Stack filesystem which the Stack Manager discovers.*

Drawing on the existing design for a `Stack` resource, the
`spec.controller.deployment` field could potentially support the use of template
variables.

Should the `controller.deployment` value be treated as a template? This could
have unintended consequences for existing Stack deployments, especially if their
definitions includes pieces that match the template format.

To avoid disturbing existing `controller.deployment` values, let's use a new
template aware `templates` spec field. We'll take that approach in the following
sections.

### Deployment

While Template Stacks will use a `templates` spec field for template bodies,
they can still take advantage of the `spec.controller.deployment` field of
`Stack` resources. Rather than using a unique controller chosen for each
Template Stack, the Stack Manager can define a singular common controller for
Template Stack resources. Assignment of the deployment could be done at install
time by the Stack Manager or at build time, utilizing the existing
`install.yaml` facilities. This may need to be constrained in future designs.

This deployed controller will be responsible for handling **all of the
resources** that the Template Stack "owns". It will need a way to identify all
of the CRDs and templates the Stack defines. Each GVK could be supplied to the
TSM as arguments, but the templates would still need to be resolved. The
controller should therefor take a reference to the Stack which contains all of
the necessary information for the TSM.

```yaml
kind: Stack
spec:
  controller:
    deployment:
      <typical Deployment fields>
      …
      containers:
      - name: template-stack-manager
        image: crossplane/template-stack-manager:latest
        args: ["--stack", "stackname"]
```

A `--stack` argument to the TSM would allow the controller to find a matching
`Stack` resource. The TSM and Stack should reside within the same namespace. The
Stack Manager will abide.

### Owner References

The [Controller
Reference](https://godoc.org/sigs.k8s.io/controller-runtime/pkg/controller/controllerutil#SetControllerReference)
will be set on the TSM `Deployment` so the TSM is removed when the Stack is
removed. The TSM can identify the Stack to work on through this controller
reference, but this approach may make it difficult for admins to identify which
deployments are operating on behalf of Stacks (and their CRDs).

`Deployment` and `ServiceAccounts` records are deployed in the same namespace by
the SM with predictable names. It is not necessary for Stacks to take on a
reference to the `Deployment` and `ServiceAccount`. Object names in Kubernetes
are immutable, so there is no assurance concern here.

What of the CRDs? Revised CRDs in an updated Stack will be updated because they
are unique, but how will Stack CRDs that have been removed be deleted? This is a
trickier problem to be considered in future designs.

The TSM should set a Controller Reference on all template rendered resources.
The appropriate Controller Reference in this case would be the CR of the managed
type. For example, if a Stack defined a `Wordpress` type whose template included
a `MySQLInstance`, that `MySQLInstance` would use `Wordpress` as the Controller
Reference. If `Wordpress` is deleted, the `MySQLInstance` will be deleted (based
on reclaim policy).

The TSM should make the `Stack` the [Owner
Reference](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents)
for any managed instances of the managed kind (`Wordpress`). This will ensure
that when the Stack is removed, all managed resources created from that Stack
are also removed.

### RBAC

The Stack Manager currently creates a Service Account for Stack Deployments,
restricted by the Owned and Dependent types declared by the Stack. A Template
Stack Manager will benefit from this same processing with little or no changes
needed. The roles for each TSM controller will be tailored to the types that
Stack needs to handle and render. The TSM will also need a `Role` permitting it
to read the `Stack` it is operating on. It is important for each TS to take
advantage of the Stack `spec.dependsOn` field to define all of the resources
defined in the template.

An alternative to deploying a TSM per Stack would be to have the existing Stack
Manager handle all new TSM defined types. The SM runs with cluster privileges,
giving Template Stacks too much control. Under this approach, the existing Stack
Manager could still create Service Accounts and take advantage of User
Impersonation when reconciling for types defined within each Template Stack.
This approach would not offer simple visibility into the roles the deployment is
actively using.

By creating a TSM deployment for each Template Stack that is installed, we are
able to scope the permissions given to each TSM instance to the permissions
requested by the Stack (i.e. the CRDs it owns and depends on). This is analogous
to how the Stack Manager currently creates a `Deployment` to host each
controller-runtime based Stack’s controller which is also scoped down to the
needed permissions.

#### Concerning the use of Core Types

In several examples throughout this design, Template Stacks are demonstrated
generating `Deployment` resources by including them in template text. There are
compelling justifications for both permitting and denying Template Stacks from
declaring a dependency on core and extension types, such as
`deployment.apps/v1`.

The ability to use Template Stacks to render arbitrary Kubernetes resources is a
powerful feature. Such a privilege would change the boundaries of a Stack,
permitting it to function outside of Crossplane managed types. Template Stacks
would compete with `KubernetesApplication` on this capability.

If Template Stacks are prohibited from using core and extension types, the
complement of `KubernetesApplication` and a local `KubernetesCluster` can be
used to overcome the limitation while providing additional security boundaries.

When considering that the Stack-Manager does not depend on Crossplane, nor
Crossplane on the Stack-Manager (strictly speaking), we can see that Template
Stacks, and the Stack Manager, may have general utility outside of Crossplane.
When used with Crossplane, the Stack Manager administrator may desire a more
opinionated approach, restricting Template Stacks to use on known types,
specifically Crossplane managed types.

These are features and challenges to investigate in a [future design](https://github.com/crossplane/crossplane/issues/899).

### Life Cycle and Updates

#### Resource life cycle

On each reconciliation loop for a TSM managed kind, the rendering process of the
TSM will use a single pass at applying templates.

* Get the current resource
* Get the `templates` from the `Stack` resource that owns this CRD
* All of the dependent resources (generated from templates) are fetched.
* The current resource and dependent resources are combined to form variables to
  pass to the template.
* Template substitutions are made for each resource and the `templateStatus`
* All of the dependent, template created, resources are applied to the API,
  including a pass at deleting resources with empty template results.

Because this process operates in a single read then write pass, it can not get
caught in a circular dependency loop. However, Template Stack creators should
avoid crafting their templates in a way that they would never resolve as
intended.

The rendered template will replace changes to dependent resources that are found
in the Kubernetes API. More importantly, since the values are obtained directly
from the API on each pass, if a resource is updated by a user or some other
process, those values will be used when rendering.

If a templates rendering contains invalid resource YAML, or contains fields and
values are not valid for the resource being generated, the Template Stack
Manager will need a defined means of surfacing these error conditions. Just as
sibling resources should be made available through templates, the error messages
should be made available through templates. The implementation of the TSM will
determine if this is done through functions or structured values. This approach
lets Template Stack and CRD authors embed status conditions in a way that best
meets the needs of their application.

Error conditions will also be logged by the TSM and pushed to the Kubernetes API
as Events.

*The crossplane-cli could be adapted to detect these conditions but that is not
a concern that will be discussed in this design.*

##### Watches and Timers

The TSM is responsible for updating the `Foo` (Stack resource) and
`DependentThing` template resources. Let's explore how the TSM is triggered.

The TSM will
[watch](https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes)
the resources belonging to its set of CRDs (`Foo`). But it does not need to
detect changes on the dependent resources, in this same way.

Rather, a
[`Result{RequeueAfter}`](https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/reconcile/reconcile.go#L30-L32)
on `Foo` would be sufficient for an initial implementation of Template Stacks.

On each reconciliation, the dependent resources will be fetched (for use as
template variables), and then the template will be rendered, and the dependent
resources will be updated (or created or deleted).

There should be advantages to [watching each dependent
resource](https://book-v1.book.kubebuilder.io/beyond_basics/controller_watches.html#watching-arbitrary-resources),
creating a more event driven "loop" (requeueing), but a timer based approach
should be sufficient for initial implementations.

#### Stack life cycle

During the first reconciliation of a `Stack`, the SM sets the `Stack` as an
owner reference on the created `ServiceAccount` and `Deployment`. For Template
Stacks this will continue to be the case. When the `Stack` is deleted, the
`ServiceAccount` and `Deployment` are deleted.

What should happen if the Stack `Deployment` is manually deleted or modified?
Should the Stack Manager recreate or modify it?

What happens when the `dependsOn` or `customresourcedefinitions` field of a
`Stack` is updated? The new CRDs should be added, and removed CRDS should be
deleted. When and how should the RBAC roles be updated?

These are questions left for future designs affecting all types of Stacks, not
just Template Stacks. See the [Stacks versioning and upgrading epic (#879)](https://github.com/crossplane/crossplane/issues/879) for more on that.

##### Template Stack life cycle

For the sake of initial Template Stack design, let's consider what should happen
when a Template Stack's `Stack` record is updated. The `template` field,
`templateStatus` field, and the responsibilities of the TSM make this life cycle
unique from that of existing Stacks.

How should RBAC rule changes be handled when the owned or dependent types
change? The SM generated `ServiceAccount` roles will have to be updated to
include or exclude the updated types. Likewise, the TSM may need to change the
types that it is handles.

Roles are checked server side on each API call, so a set of Role updates that
the Stack Manager may issue to update the `ServiceAccount` assigned to a `Stack`
will take affect immediately. Unless the deployment is stopped during updates,
race conditions could permit template rendered resources more or less roles than
needed. If Template Stacks are made to only operate on Crossplane Managed kinds,
this may be less concerning. Future designs should explore the effect of role
changes on Template Stacks that can create core and extension types.

#### Unknown Types

Hand-crafted controllers have the advantage of knowing the types and structure
of the resources they manage. In Go, this allows for the advantages of a
strongly typed environment including compile time error checking and code
completion during development.

A Template Stack would not include a controller. An external controller will
have to handle reconciliation for types that the controller could not have been
aware of at compile time.

How can an external controller manage arbitrary types declared at runtime?

```go
var obj unstructured.Unstructured
obj.SetAPIVersion("foo/v1")
obj.SetKind("Bar")

err := ctrl.NewControllerManagedBy(mgr).
    For(&obj).
    Owns(...).
    Complete(&FooReconciler{})
```

*In this example the strings `foo/v1` and `Bar`, must be taken at runtime from
the `Stack` resource. The controller is told to handle this GVK and the
reconciler will receive resources as references to `unstructured.Unstructured`
types, which implement `runtime.Object`.*

This proposal does not require any special or predefined CRD fields to exist
aside from the Status Templates which would require a `status`. This leaves CRD
fields entirely up to the developer which makes it easy to adopt existing CRDs
and replace their controllers with the Template Stack Manager.

## Examples

### Hello World

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  name: HelloWorld
spec:
  customresourcedefinitions:
  - kind: HelloWorld
    apiVersion: helloworld.crossplane.example.com/v1
  templateStatus:
    helloworld.crossplane.example.com/v1: |
      greeting: "Hello, {{.spec.name}}!"
```

```yaml
apiVersion: helloworld.crossplane.example.com
kind: HelloWorld
metadata:
  name: world
spec:
  name: "World"
```

After the TSM reconciles this resource:

```yaml
apiVersion: helloworld.crossplane.example.com
kind: HelloWorld
metadata:
  name: world
spec:
  name: "World"
status:
  greeting: "Hello, World!"
```


<details>
<summary>CRD for HelloWorld</summary>

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: helloworld.crossplane.example.com
spec:
  group: crossplane.example.com
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                name:
                  type: string
            status:
              type: object
              properties:
                greeting:
                  type: string
      subresources:
        status: {}
  scope: Namespaced
  names:
    plural: helloworlds
    singular: helloworld
    kind: HelloWorld
```

</details>

### Plus One

The following demonstrates a `PlusOne` resource `Stack` which would append "+ "
to the single `status` field of the resource on each reconciliation.

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  name: PlusOne
spec:
  customresourcedefinitions:
  - kind: PlusOne
    apiVersion: plusses.crossplane.example.com/v1
  templateStatus:
    plusses.crossplane.example.com/v1: |
      output: "+ {{.status.output}}"
```

```yaml
apiVersion: plusses.crossplane.example.com
kind: PlusOne
metadata:
  name: plusses
```

After the first reconciliation, this resource will have a `status.output` value
of `+ `. On the second pass, `+ + `, third, `+ + + `, and so on until the size
of this resource can no longer be accommodated.

Basic string concatenation has not been demonstrated because the TSM template
engine may or may not provide math functions like addition.

<details>
<summary>CRD for PlusOne</summary>

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: plusone.crossplane.example.com
spec:
  group: crossplane.example.com
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            status:
              type: object
              properties:
                output:
                  type: string
      subresources:
        status: {}
  scope: Namespaced
  names:
    plural: plusones
    singular: plusone
    kind: PlusOne
```

</details>

### Usage Walk-through

If a developer has a CRD for `foo.group/version` and they want to use the Stack Manager and Template Stacks to handle these, they could create a Template Stack containing the relevant templates and other Stack metadata (out of scope here), and then install it:

```sh
# this syntax is just an example
kubectl crossplane stack install example/foo-template-stack
```

This creates a `StackInstall` resource:

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackInstall
metadata:
  name: Foo
spec:
  package: example/foo-template-stack:latest
```

In response to a `StackInstall` request, the Stack Manager will do [its normal
work](https://github.com/crossplane/crossplane/blob/master/design/design-doc-packages.md#installation-flow)
and [some extra work](#Resource-life-cycle) to create a `Stack` resource and
apply the appropriate `template` and `templateStatus` bodies to that `Stack`
resource. It will also create a `Deployment` and `ServiceAccount` for the TSM.

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
  name: Foo
spec:
  customresourcedefinitions:
  - kind: Foo
    apiVersion: group/version
  templates:
    # the GVK for the CRD the Stack consumer will create
    foo.group/version:
      # the template for the resources that the TSM will create
      templateA: |
        kind: athing
        spec:
          foovar: {{ .spec.foo }}
  # the template for the status of the resource the Stack consumer created
  templateStatus:
    foo.group/version: |
      statusthing: {{ .templateA.status.bar }}
```

Given a user request of:

```yaml
apiVersion: group/version
kind: Foo
spec:
  foo: "foo"
```

The TSM will generate the following:

```yaml
kind: athing
spec:
  foovar: "foo"
```

Imagine the `athing` type is reconciled by an outside controller, and a `status` emerges:

```yaml
kind: athing
spec:
  foovar: "foo"
status:
  bar: "bar"
```

On the next TSM pass, the Foo resource will be updated:

```yaml
apiVersion: group/version
kind: Foo
spec:
  foo: "foo"
status:
  statusthing: "bar"
```

## Related Issues

* [#853](https://github.com/crossplane/crossplane/issues/853) **Epic**
  Template based Stacks
* [#877](https://github.com/crossplane/crossplane/issues/877) Produce a
  one-pager design for a Template Stack Controller #877
* [#878](https://github.com/crossplane/crossplane/issues/878) Template Stacks
  should be reconciled through some controller
* [#879](https://github.com/crossplane/crossplane/issues/879) Stacks
  versioning and upgrading
* [#914](https://github.com/crossplane/crossplane/issues/914) Document key
  problems that template stacks solve for k8s app deployments
* [#915](https://github.com/crossplane/crossplane/issues/915) Template Stacks
  UX: overall experience; modeling, design thinking; UX

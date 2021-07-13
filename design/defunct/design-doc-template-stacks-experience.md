# Template Stacks Experience

* Owner: Daniel Suskin (@suskin)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Outline

* [Background](#background)
* [Terms](#terms)
* [Goals](#goals)
* [Non-goals](#non-goals)
* [How to read this document](#how-to-read-this-document)
* [Design](#design)
  * [Writing](#writing)
    * [Adding a CRD](#adding-a-crd)
  * [Building and publishing](#building-and-publishing)
  * [Consuming](#consuming)
    * [Install](#install)
    * [Create](#create)
    * [Delete](#delete)
    * [Uninstall](#uninstall)
    * [Upgrade and update](#upgrade-and-update)
  * [The stack.yaml](#the-stackyaml)
    * [Specifying how to process render requests](#specifying-how-to-process-render-requests)
  * [Specifying default values](#specifying-default-values)
  * [How processing objects works under the covers](#how-processing-objects-works-under-the-covers)
  * [Templating/configuration engine](#templatingconfiguration-engine)
  * [Lifecycle hooks](#lifecycle-hooks)
  * [Internal representation of templates](#internal-representation-of-templates)
  * [The template stack controller](#the-template-stack-controller)
* [Example use-cases](#example-use-cases)
  * [Helm charts](#helm-charts)
* [Further reading](#further-reading)

## Background

Managing configuration of bespoke Kubernetes applications has been a
topic of much interest and discussion in the Kubernetes community. A
[design document written by Brian
Grant](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/declarative-application-management.md)
gives a nice overview of the space, and proposes some of the properties
and techniques that a unified solution would have.

A summary of the properties would be:

* Usage of Kubernetes APIs and patterns without going outside of them,
  so that existing and future tooling can easily integrate with what
  is built.
* Support for overlay-style configuration overrides, for their
  maintainability and expressiveness.
* Support for template-style configuration, for its ease of use for
  simple cases.
  * Alternatively, simple cases may be handled individually instead
    of via a general templating mechanism.

Users could choose an overlay-oriented approach or a template-style
approach. In general, overlay-oriented configuration is considered
better when the user has a large and complex configuration codebase, or
when extending a third-party configuration is needed. Template-oriented
configuration is considered easier to use for simple cases, especially
for people who are not accustomed to overlay-oriented configuration.

The most prominent overlay-oriented configuration tool is
[kustomize](https://github.com/kubernetes-sigs/kustomize), which
recently became part of the mainline kubectl tool. The most prominent
template-oriented configuration tool is [helm](https://helm.sh/), which
is the de-facto standard tool for managing resource configuration
bundles in Kubernetes.

On the Crossplane side, the extensibility model is still relatively new.
In the most recent release, the concept of
[Stacks](https://github.com/crossplane/crossplane/blob/master/design/design-doc-packages.md)
was introduced, and the first version of some [tooling to help write
Stacks](https://github.com/crossplane/crossplane-cli) was also
introduced. The Crossplane project has been working toward making Stacks
[easier to
write](https://github.com/crossplane/crossplane/issues/853), ideally
to the point where an author doesn't need to write a full controller.
The easier version of Stacks is being called "Template Stacks". There is
now [a repository with some
examples](https://github.com/suskin/template-stack-experience) of what
writing and interacting with a Template Stack may look like for
different use-cases.

## Terms

* A **render request** is an instance of a resource Kind recognized by
  the Template Stack (such as a Kind defined by a custom resource). It
  tells the stack that some configuration is desired.
* A **template** is a chunk of configuration which will be reused by
  the Template Stack to create configuration.
* A **render** is the configuration which is the output of the stack's
  controller in response to a render request. The configuration is
  created by processing the stack's input templates using the desired
  engine, and using the render request for context.
* **Behaviors** are the configuration settings for a Template Stack.
  For example, so that its controller knows how to respond to a render
  request of a known type.


## Goals

The goal of this document is to propose and discuss what the developer
and user experience would look like for Template Stacks. This
encompasses the complete lifecycle of a Template Stack: project setup,
development, testing, publishing, and consuming.

Some of the design goals include:

* Reuse existing Kubernetes APIs and patterns wherever possible.
* Support overlays, for their maintainability and extensibility.
* Support templates, for their ease of use for simple use-cases, or a
  solution of equivalent simplicity for simple use-cases.
* Allow developers to avoid writing a controller if their Stack fits
  the pattern of provisioning some set of resources.

We plan to support overlays in the long run, but plan to start with
templates, because they are easier for users to get into when they're
not familiar with overlays.

## Non-goals

Because this is the first iteration of the Template-style Stacks
Experience, there are many things which are out of scope. These include:

* Injecting values into stacks from outside of the stack (such as
  external resources or other stacks); also known as live value
  referencing.
* Creating the final form of template stacks; this is expected to
  evolve over time as people use it and gain insights about how to
  improve it.
* Dynamic name prefixing or suffixing.
* Automatically wrapping application configurations as Crossplane
  workloads.
* Automatically creating resources (other than CRDs) when a stack is
  installed.
* Updating output; the high-level idea of re-rendering and letting the
  other stacks take care of the updates seems close enough for now.
* Updating a stack. This should happen as part of the other thinking
  about versioning and how to update a stack's version.
* Updating the stack manager / shared controller. Eventually we'll
  need a controller version, and knowing that seems good enough for
  now.
* Setting status in the render request. This would be nice to have, but
  it seems independent from the other stuff we're working on. It
  duplicates information already in the system, so it's not a topmost
  priority at this time.

Also out of scope is the internal representation of configuration, and
the implementation of the stack manager. See the [complementary
internals-oriented Template Stack design doc](https://github.com/crossplane/crossplane/blob/master/design/one-pager-template-stacks.md)
for those details.

## How to read this document

This document is intended to be read alongside the quick start example
in the [template stacks experience repository][quick-start-example],
which contains an example of a complete user scenario of installing an
application and its infrastructure using Crossplane and template stacks.

Additionally, for more detail about the internals of template stacks on
the stack manager and Crossplane side, see the [design doc focused on
this](https://github.com/crossplane/crossplane/blob/master/design/one-pager-template-stacks.md)

## Design

The overall flow of authoring and consuming a template stack will be
very similar for all scenarios. It is expected that template stacks will
support a wide variety of scenarios, so this section will show some
examples to help explain them.

### Writing

Creating a template stack from scratch involves only a couple steps. The
project must first be initialized, and then the yamls must be put in a
particular directory. The rbac requirements for the stack must also be
configured, though some of that is done by the stack manager at runtime.
At a high level, the steps would be as follows:

1. Create a project directory.
2. In the project directory, `kubectl crossplane stack init --template
   myorg/mystack` to create the boilerplate of the stack layout.
3. Relative to the project directory, put the configuration yamls in
   `config/stack/manifests/resources`, or a different
   directory if desired, so long as the directory is in the right
   location in the stack artifact.
4. Add CRD definitions; this could be done by putting CRD yaml
   definitions in `config/stack/manifests/resources` or in
   `config/crd/bases`. We plan to make this simpler for the user by
   adding a crossplane-cli command for it.
5. Add configuration to `stack.yaml` to
   specify how configurations are rendered.
6. Edit `stack.yaml` to specify that the stack will be working with all
   of the kinds that it will be working with (using the `dependsOn`
   field).
7. Edit `stack.yaml` as appropriate.

Note that we plan to merge `app.yaml` and `stack.yaml` into a single
document (`stack.yaml`) in the future, so this set of steps is written
as though that has already happened.

#### Specifying configuration and templates

Because we plan to support multiple engines, this document won't get
into the specifics of what templates look like. That said, if a helm
chart were being used (for example), then the helm chart could be placed
in a folder (such as `wordpress`) in the resources directory, and the
stack would be configured to use that directory. Using the folder
structure described above, that would mean that root of the helm chart
would exist at `configu/stack/manifests/resources/wordpress`.

#### Adding a CRD

The tooling will make it simpler to add a CRD from scratch. For example,
the following would create a basic CRD in the appropriate folder, so
that it becomes part of the stack:

```
kubectl crossplane stack crd init WordpressInstance wordpress.samples.stacks.crossplane.io
```

The command will generate a reasonable version (such as `v1alpha1`), and
sensible list, plural, and singular names from the input; the generated
values can be adjusted by the user in the CRD file.

There will also be a convenience flag in the `stack init` command so
that people starting from scratch can initialize the stack and the first
CRD with a single command:

```
$ kubectl crossplane stack init --template mygroup/mystackname --init-crd
> CRD name: WordpressInstance
> CRD api group: wordpress.samples.stacks.crossplane.io
```

In the future, we will likely do more work in the are of making CRDs and
their schemas simpler to write.

### Building and publishing

To build and publish a stack, the standard steps for building and
publishing a stack are used.

Building:

```
kubectl crossplane stack build
```

Publishing:

```
kubectl crossplane stack publish
```

### Consuming

Consuming has two steps: installation and creation. Installation is
installing the stack into the Crossplane control cluster. Creation is
creating a Kubernetes resource which the stack recognizes, so that the
stack will do something in response to it. In the case of template
stacks, that usually will look something like rendering a set of yamls
and applying the rendered output to the cluster.

#### Install

Installation is much the same as installing any stack:

```
kubectl crossplane stack install myorg/mystack mystack
```

This installs the stack from the `myorg/mystack` image, using the name `mystack`.

#### Create

Instantiation is also very similar to creating any object. Here is some
sample yaml:

```yaml
apiVersion: thing.samples.stacks.crossplane.io/v1alpha1
kind: ThingInstance
metadata:
  name: thinginstance-sample
spec:
  myfield: myvalue
```

The difference is that underneath, the ThingInstance will be used as the
input for rendering a template.

#### Delete

When the ThingInstance is deleted, the corresponding resources will also
be deleted. This is the same behavior as in any stack.

The instance is also deleted if the stack is uninstalled.

#### Uninstall

Uninstall looks the same as any other stack:

```
kubectl crossplane stack uninstall mystack
```

#### Upgrade and update

Upgrading a stack version, and updating an instance of an object managed
by the stack, are out of scope of this document. However, one could
imagine that the process of upgrading a stack version would change a
version number on object instances which go with the stack, and that the
version number change would underneath cause the template to be rendered
again and applied to the cluster.

### The stack.yaml

Configuration for the template stack will be in a `stack.yaml` file at
the build root of the stack in the repository. For most cases, this will
mean the `stack.yaml` lives at the root of the repository.

#### Specifying how to process render requests

A Stack author must create a `stack.yaml` in order to configure how
templates are rendered in response to a render request. Here's an
example, with comments explaining the directives:

```yaml
# This field configures which templates are rendered in response
# to a given object type. An instance of the object type is
# considered to be a "render request".
behaviors:
  # An engine configuration here will apply to the rest of the
  # configuration, but it could also be specified at a per-crd
  # level, or as low as a per-hook level. Engine configurations
  # nested deeper in the configuration hierarchy will override
  # ones which are higher up in the hierarchy, so a hook-level
  # configuration would override this one.
  engine:
    type: kustomize

  crds:
    # This is a particular CRD which is being configured. When the
    # controller sees an object of this type, it will do something.
    wordpressinstance.wordpress.samples.stacks.crossplane.io:
      # This is a top-level object to group hook configurations.
      # There can be hooks for multiple different types of events.
      hooks:
        # Post create is triggered after an instance is created
        postCreate:
          # These are the templates which should be rendered when an object
          # of the type above is seen.
          #
          # Defaults for the variables can be defined in the default values
          # for the CRD fields, using the standard Kubernetes mechanism for
          # specifying CRD field default values.
          # Note that this is a list of objects, so multiple can be
          # specified.
          - directory: wordpress
        # Post update is triggered after an instance is changed
        postUpdate:
          - directory: wordpress
```

The `stack.yaml` should be in the build root of the repository. For most
single-stack repositories, this means the root of the repository.

For a realistic sample for a complete user scenario, see the [quick
start example][quick-start-example] in the template stack experience
repository.

### Specifying default values

Default values will be specified in the CRD itself, using the [standard
mechanism][kubernetes-default-values] for specifying default values for
CRD fields. Here is an example excerpt from a realistic CRD, where the
CRD's `spec.image` field is configured with a default:

```yaml
kind: CustomResourceDefinition
metadata:
  creationTimestamp: null
  name: wordpressinstances.wordpress.samples.stacks.crossplane.io
spec:
  group: wordpress.samples.stacks.crossplane.io
  names:
    kind: WordpressInstance
    plural: wordpressinstances
  scope: ""
  validation:
    openAPIV3Schema:
      description: WordpressInstance is the Schema for the wordpressinstances API
      properties:
        ...
        spec:
          type: object
          properties:
            ...
            image:
              type: string
              description: A custom wordpress container image id to use
              # Defaults are specified like this, using the schema validation for CRD fields.
              # For more about how this works with CRDs, see:
              # https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#defaulting
              default: "wordpress:4.6.1-apache"
```

For the full example, see the sample in the
[template stack experience repository](https://github.com/suskin/template-stack-experience/tree/master/wordpress-workload/go-templates/default-variables).

### How processing objects works under the covers

The imagined implementation for how objects sent to the stack for
rendering become resources is that the object's fields become the input
for the template rendered by the controller.

For example, given this object as input:

```yaml
apiVersion: wordpress.samples.stacks.crossplane.io/v1alpha1
kind: WordpressInstance
metadata:
  name: "my-wordpress-app-from-helm2"
  namespace: dev
spec:
  engineVersion: "8.0"
```

The `engineVersion` field would be available to the templating engine
which was being used, and would have a value of `8.0`. This means that
if the engine being used were helm, the behavior would be equivalent to
if there were a `values.yaml` passed in which looked like this:

```yaml
engineVersion: "8.0"
```

We expect the configuration to support the same things as the underlying
engine. So, for example, nested configuration values would be supported
just as well as they would be with a regular `values.yaml` for a helm
chart.

For the complete example, see the [quick start
example][quick-start-example] in the template stack experience
repository. The snippet was taken from the part of the example where the
app stack is used.

For more details, see the [design document about the
internals](https://github.com/crossplane/crossplane/blob/master/design/one-pager-template-stacks.md).

### Templating/configuration engine

Template stacks will not be opinionated about which templating engine is
used. We plan to support multiple configuration engines. The engine will
be configurable by setting values in the `stack.yaml`. Here's an example
of a snippet from a `stack.yaml` which specifies a particular engine:

```yaml
behaviors:
  engine:
    type: kustomize
```

In some cases, the engine configuration line may be optional; the system
may be able to infer the engine based on the structure of the stack. For
example, if no engine is specified, and a `kustomization.yaml` is found,
the engine could be inferred to be kustomize.

The engine can be specified at multiple levels; setting it under
`behaviors` will set a default, but setting it lower down will override
a value set at a higher level. It can be configured per hook.

For a more complete example of a user scenario, including usage of
multiple different engines, see the [quick start
example][quick-start-example]. There are also some more details in the
[stack yaml section of this document](#the-stackyaml), and in the [helm
charts section](#helm-charts).

### Lifecycle hooks

We expect to eventually support lifecycle hooks. See the [speculative
design in the template stacks experience repo](https://github.com/suskin/template-stack-experience/tree/master/wordpress-workload/go-templates/lifecycle-hook)
for more details about what that could look like. Lifecycle hooks are
out of the scope of this document, and will be revisited in the future.

### Internal representation of templates

See the [design doc on the internals of the Template Stack
implementation](https://github.com/crossplane/crossplane/blob/master/design/one-pager-template-stacks.md).

### The template stack controller

See the [design doc on the internals of the Template Stack
implementation](https://github.com/crossplane/crossplane/blob/master/design/one-pager-template-stacks.md).

## Example use-cases

For a coherent user scenario, see the [quick start
example][quick-start-example].

The scenario shows what it might look like for a user to set up an
application and its infrastructure from scratch using Crossplane and
Template Stacks. Multiple configuration engines are shown.

### Helm charts

To author a [template stack from a simple helm
chart][helm-engine-example], these steps could be followed:

1. Create and initialize a template stack project, with a helm flag:
   `kubectl crossplane stack init --template`.
2. Create a CRD: `kubectl crossplane stack crd init WordpressInstance
   wordpress.samples.stacks.crossplane.io`
3. Put the chart's contents into the templates folder.
4. Create a `stack.yaml` in the root of the repostory, and configure it
   to use the templates for the created CRD.
5. To set up default values:
    1. Put the contents of the chart's `values.yaml` into the CRD's
       [default field value definitions][kubernetes-default-values].
6. Bundle and publish the stack.

When consuming the stack, the default template values can be overridden
by specifying fields on the render request object.

This use-case will probably need additional thought if we want to
support all permutations of helm chart. For more realistic and complete
examples, see the helm variation of the app stack in the [quick start
example][quick-start-example] in the template stack experience
repository.

## Further reading

* [Template Stack Experience - Quick Start Example](https://github.com/suskin/template-stack-experience/tree/master/wordpress-workload/quick-start)
* [Declarative Application Management in Kubernetes](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/declarative-application-management.md)
* [Template Stacks internals design doc](https://github.com/crossplane/crossplane/blob/master/design/one-pager-template-stacks.md)
* [Stacks CLI design](https://github.com/crossplane/crossplane/blob/master/design/one-pager-stack-cli.md)
* [Kubernetes default field values][kubernetes-default-values]

<!-- Reference-style links -->
[kubernetes-default-values]: https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#defaulting
[helm-engine-example]: https://github.com/suskin/template-stack-experience/tree/master/wordpress-workload/quick-start/app-stack/helm2
[quick-start-example]: https://github.com/suskin/template-stack-experience/tree/master/wordpress-workload/quick-start

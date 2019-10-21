# Template Stacks Experience

* Owner: Daniel Suskin (@suskin)
* Reviewers: Crossplane Maintainers
* Status: Draft

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
[Stacks](https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md)
was introduced, and the first version of some [tooling to help write
Stacks](https://github.com/crossplaneio/crossplane-cli) was also
introduced. The Crossplane project has been working toward making Stacks
[easier to
write](https://github.com/crossplaneio/crossplane/issues/853), ideally
to the point where an author doesn't need to write a full controller.
The easier version of Stacks is being called "Template Stacks". There is
now [a repository with some
examples](https://github.com/suskin/template-stack-experience) of what
writing and interacting with a Template Stack may look like for
different use-cases.

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
* Support for Kustomize / overlays.
* Automatically wrapping application configurations as Crossplane
  workloads.
* Automatically creating resources (other than CRDs) when a stack is
  installed.
* Updating output; the high-level idea of re-rendering and letting the
  other stacks take care of the updates seems close enough for now.
* Updating stack. This should happen as part of the other thinking
  about versioning and how to update a stack's version.
* Updating the stack manager / shared controller. Eventually we'll
  need a controller version, and knowing that seems good enough for
  now.
* Setting status in the CRD. This would be nice to have, but it seems
  independent from the other stuff we're working on. It duplicates
  information already in the system, so it's not a topmost priority at
  this time.

Also out of scope is the internal representation of configuration, and
the implementation of the stack manager. See the [complementary
internals-oriented Template Stack design doc](https://github.com/crossplaneio/crossplane/pull/928)
for those details.

## Terms

* A **render request** is an instance of a CRD recognized by the
  Template Stack. It tells the stack that some configuration is
  desired.
* A **template** is a chunk of configuration which will be reused by
  the Template Stack to create configuration.
* A **render** is the configuration which is the output of the stack's
  controller in response to a render request. The configuration is
  created by processing the stack's input templates using the desired
  engine, and using the render request for context.
* **Behaviors** are the configuration settings for a Template Stack.
  For example, so that its controller knows how to respond to a render
  request of a known type.

## Design

The overall flow of authoring and consuming a template stack will be
very similar for all scenarios. It is expected that template stacks will
support a wide variety of scenarios, so this section will show some
examples to help explain them.

For a more complete treatment of example scenarios, see the [template
stack experience repository](https://github.com/suskin/template-stack-experience).
### Writing

Creating a template stack from scratch involves only a couple steps. The
project must first be initialized, and then the yamls must be put in a
particular directory. The rbac requirements for the stack must also be
configured, though some of that is done by the stack manager at runtime.
At a high level, the steps would be as follows:

1. Create a project directory.
2. In the project directory, `kubectl crossplane stack init --template
   myorg/mystack` to create the boilerplate of the stack layout.
3. Relative to the project directory, put the yamls in
   `config/stack/manifests/resources/templates` .
4. Add CRD definitions; this could be done by putting CRD yaml
   definitions in `config/stack/manifests/resources` or in
   `config/crd/bases`.
5. Add configuration to `config/stack/manifests/behaviors.yaml` to
   specify how configurations are rendered.
6. Edit `config/stack/manifests/resources/app.yaml` to specify that the
   stack will be working with all of the kinds that it will be working
   with (using the `dependsOn` field).
7. Edit `config/stack/manifests/resources/app.yaml` as appropriate.

#### Adding a CRD

The tooling will make it simpler to add a CRD from scratch. For example,
the following would create a basic CRD in the appropriate folder, and
would update the `dependsOn` field of the stack's `app.yaml`:

```
kubectl crossplane stack crd init WordpressInstance wordpressinstances wordpress.samples.stacks.crossplane.io
```

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

Instantiation is also very similar to any stack:

```
cat > my-instance.yaml <<EOF
apiVersion: thing.samples.stacks.crossplane.io/v1alpha1
kind: ThingInstance
metadata:
  name: thinginstance-sample
spec:
  myfield: myvalue
EOF

kubectl apply -f my-instance.yaml
```

The difference is that underneath, the ThingInstance will be used as the
input for rendering a template.

#### Delete

When the ThingInstance is deleted, the corresponding resources will also
be deleted. This is the same behavior as in any stack.

```
cat > my-instance.yaml <<EOF
apiVersion: thing.samples.stacks.crossplane.io/v1alpha1
kind: ThingInstance
metadata:
  name: thinginstance-sample
spec:
  myfield: myvalue
EOF

kubectl delete -f my-instance.yaml
```

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

### Specifying how to process render requests

A Stack author must create a `behaviors.yaml` in order to configure how
templates are rendered in response to a render request. Here's an
example, with comments explaining the directives:

```
cat > config/stack/manifests/behaviors.yaml <<EOF
# This field configures which templates are rendered in response
# to a given object type. An instance of the object type is
# considered to be a "render request".
crdOutputs:
  # This is a particular CRD which is being configured. When the
  # controller sees an object of this type, it will do something.
  wordpressinstance.wordpress.samples.stacks.crossplane.io/v1alpha1:
    # These are the templates which should be rendered when an object
    # of the type above is seen.
    resources:
      # These are individual files which will be rendered.
      - kubernetescluster.yaml
      - mysqlinstance.yaml
      - kubernetesapplication.yaml
    # This configures default values for the variables in the
    # templates being rendered.
    defaults:
      # This is a list of files to use for default values. They
      # will be merged together in the specified order to create
      # the default template variables in the context.
      - defaults.yaml
EOF
```

### How processing objects works under the covers

The imagined implementation for how objects sent to the stack for
rendering become resources is that the object's fields become the input
for the template rendered by the controller. For more details, see the
[detailed examples](https://github.com/suskin/template-stack-experience)
or the [design document about the
internals](https://github.com/crossplaneio/crossplane/pull/928).

### Internal representation of templates

See the [design doc on the internals of the Template Stack
implementation](https://github.com/crossplaneio/crossplane/pull/928).

### The template stack controller

See the [design doc on the internals of the Template Stack
implementation](https://github.com/crossplaneio/crossplane/pull/928).

## Example use-cases

See the [Template Stack
Experience](https://github.com/suskin/template-stack-experience) code
examples for some realistic examples of how different use-cases work
with a realistic workload.

Examples covered by the repository all currently use go templates as the
templating engine. The following cases are covered by the examples:

* No variables at all in a set of configurations.
* Variables in configurations with default variable values.
* Variables in configurations, with values specified by users when a
  render is requested.
* Resource packs.
* Multiple CRDs defined in a single Stack.
* Multiple rendered variants of a given configuration.
* Composing configuration using multiple Stacks.

### Helm charts

To author a template stack from a simple helm chart, these steps could
be followed:

1. Create and initialize a template stack project, with a helm flag:
   `kubectl crossplane stack init --template`.
2. Create a CRD: `kubectl crossplane stack crd init WordpressInstance
   wordpressinstances wordpress.samples.stacks.crossplane.io`
3. Put the chart's contents into the templates folder.
4. Create a `behaviors.yaml` and configure it to use the templates for
   the created CRD.
5. To set up default values:
    1. Put the contents of the chart's `values.yaml` in the templates
       folder, and configure `behaviors.yaml` to use it as the file with
       the default values in it.
6. Bundle and publish the stack.

When consuming the stack, the default template values can be overridden
by specifying fields on the render request object.

This use-case will probably need additional thought if we want to
support all permutations of helm chart.

## Further reading

* [Template Stack Experience](https://github.com/suskin/template-stack-experience) examples
* [Declarative Application Management in Kubernetes](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/declarative-application-management.md)
* [Template Stacks internals design doc](https://github.com/crossplaneio/crossplane/pull/928)
* [Stacks CLI design](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-stack-cli.md)

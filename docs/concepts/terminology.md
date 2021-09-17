---
title: Terminology
toc: true
weight: 110
indent: true
---

# Terminology

## A Note on Style

Each type of Kubernetes resource has a ‘Pascal case’ name - i.e. a title case
name with no spaces between each word. Examples include ‘DaemonSet’ and
‘PersistentVolumeClaim’. Often these names are written using fixed width fonts
to draw attention to the fact that they’re a concrete type of resource within
the API - e.g. `PersistentVolumeClaim`.

Crossplane follows this convention. We often use names like RDSInstance or
CompositeResourceDefinition when discussing Crossplane types. Crossplane also
has “classes of types” - i.e. concepts that aren’t a distinct type of API
resource, but rather describe a group of conceptually similar types. For example
there is no ManagedResource type in Crossplane - instead types like RDSInstance
and GKECluster are said to be “a managed resource”.

Use your discretion as to whether you use pascal case when writing about a
distinct type - e.g. “RDS Instance” and “RDSInstance” are both fine. The pascal
case form makes more sense in contexts like documentation where you’re referring
to Crossplane’s RDSInstance managed resource rather than the general concept of
“an RDS instance”. Avoid using Pascal case when talking about classes of types -
i.e. always write “managed resource”, not “ManagedResource”. Each of the below
terms clarify whether they correspond to a single type, or a class of types.

### Why 'X'?

You may notice that Crossplane uses “X” as shorthand for “Crossplane” and/or
“Composite”. This is because some of our concepts - specifically Composite
Resources (XRs) and Composite Resource Definitions (XRDs) are modelled on
similar Kubernetes concepts - Custom Resources (CRs) and Custom Resource
Definitions (CRDs). We chose to abbreviate to (e.g.) XRD instead of CRD to avoid
confusion.

## Crossplane Terms

The below terms are commonly used in the Crossplane ecosystem.

### Composition

The term Composition has two related but distinct meanings.

“Composition” refers broadly to the feature of Crossplane that allows teams to
define their own opinionated platform APIs.

“A Composition” or `Composition` (fixed width) refers to the key Crossplane API
type that configures how Crossplane should compose resources into a higher level
“composite resource”. A Composition tells Crossplane “when someone creates
composite resource X, you should respond by creating resources Y and Z”.

The latter use of Composition represents a distinct Crossplane API type so
Pascal case and fixed width fonts are appropriate. We also tend to capitalise
the former use, representing the feature in general, but fixed width fonts are
not appropriate in that context.

> Folks accustomed to Terraform might think of a Composition as a Terraform
> module; the HCL code that describes how to take input variables and use them
> to create resources in some cloud API. Folks accustomed to Helm might think of
> a Composition as a Helm chart’s templates; the moustache templated YAML files
> that describe how to take Helm chart values and render Kubernetes resources.

### Composite Resource

A “Composite Resource” or “XR” is an API type defined using Crossplane. A
composite resource’s API type is arbitrary - dictated by the concept the author
wishes to expose as an API, for example an “AcmeCoDB”. A common convention is
for types to start with "X" - e.g. "XAcmeCoDB".

We talk about Crossplane being a tool teams can use to define their own
opinionated platform APIs. Those APIs are made up of composite resources; when
you are interacting with an API that your platform team has defined, you’re
interacting with composite resources.

A composite resource can be thought of as the interface to a Composition. It
provides the inputs a Composition uses to compose resources into a higher level
concept. In fact, the composite resource _is_ the high level concept.

The term “Composite Resource” refers to a class of types, so avoid using Pascal
case - “Composite Resource” not CompositeResource. Use pascal case when
referring to a distinct type of composite resource - e.g. a XAcmeCoDB.

> Folks accustomed to Terraform might think of a composite resource as a
> `tfvars` file that supplies values for the variables a Terraform module uses
> to create resources in some cloud API. Folks accustomed to Helm might think of
> a composite resource as the `values.yaml` file that supplies inputs to a Helm
> chart’s templates.

### Composite Resource Claim

A “Composite Resource Claim”, “XRC”, or just “a claim” is also an API type
defined using Crossplane. Each type of claim corresponds to a type of composite
resource, and the pair have nearly identical schemas. Like composite resources,
the type of a claim is arbitrary.

We talk about Crossplane being a tool platform teams can use to offer
opinionated platform APIs to the application teams they support. The platform
team offers those APIs using claims. It helps to think of the claim as an
application team’s interface to a composite resource. You could also think of
claims as the public (app team) facing part of the opinionated platform API,
while composite resources are the private (platform team) facing part.

A common convention is for a claim to be of the same type as its corresponding
composite resource, but without the "X" prefix. So an "AcmeCoDB" would be a type
of claim, and a "XAcmeCoDB" would be the corresponding type of composite
resource. This allows claim consumers to be relatively ignorant of Crossplane
and composition, and to instead simply think about managing “an AcmeCo DB” while
the platform team worries about the implementation details.

The term “Composite Resource Claim” refers to a class of types, so avoid using
Pascal case - “Composite Resource Claim” not CompositeResourceClaim. Use Pascal
case when referring to a distinct type of composite resource claim - e.g. an
AcmeCoDB.

> Claims map to the same concepts as described above under the composite
> resource heading; i.e. `tfvars` files and Helm `values.yaml` files. Imagine
> that some `tfvars` files and some `values.yaml` files were only accessible to
> the platform team while others were offered to application teams; that’s the
> difference between a composite resource and a claim.

### Composite Resource Definition

A “Composite Resource Definition” or “XRD” is the API type used to define new
types of composite resources and claims. Types of composite resources and types
of claims exist because they were defined into existence by an XRD. The XRD
configures Crossplane with support for the composite resources and claims that
make up a platform API.

XRDs are often conflated with composite resources (XRs) - try to avoid this.
When someone uses the platform API to create infrastructure they’re not creating
XRDs but rather creating composite resources (XRs). It may help to think of a
composite resource as a database entry, while an XRD is a database schema. For
those familiar with Kubernetes, the relationship is very similar to that between
a Custom Resource Definition (CRD) and a Custom Resource (CR).

A `CompositeResourceDefinition` is a distinct Crossplane API type, so Pascal
case and fixed width fonts are appropriate.

> There isn’t a direct analog to XRDs in the Helm ecosystem, but they’re a
> little bit like the variable blocks in a Terraform module that define which
> variables exist, whether those variables are strings or integers, whether
> they’re required or optional, etc.

### Managed Resource

Managed resources are granular, high fidelity Crossplane representations of a
resource in an external system - i.e. resources that are managed by Crossplane.
Managed resources are what Crossplane enables platform teams to compose into
higher level composite resources, forming an opinionated platform API. They're
the building blocks of Crossplane.

You’ll often hear three related terms used in the Crossplane ecosystem; composed
resource, managed resource, and external resource. While there are some subtle
contextual differences, these all broadly refer to the same thing. Take an
RDSInstance for example; it is a managed resource. A distinct resource within
the Crossplane API that represents an AWS RDS instance. When we make a
distinction between the managed resource and an external resource we’re simply
making the distinction between Crossplane’s representation of the thing (the
`RDSInstance` in the Kubernetes API), and the actual thing in whatever external
system Crossplane is orchestrating (the RDS instance in AWS's API). When we
mention composed resources, we mean a managed resource of which a composite
resource is composed.

Managed resources are a class of resource, so avoid using Pascal case - “managed
resource” not “ManagedResource”.

> Managed resources are similar to Terraform resource blocks, or a distinct
> Kubernetes resource within a Helm chart.

### Package

Packages extend Crossplane, either with support for new kinds of composite
resources and claims, or support for new kinds of managed resources. There are
two types of Crossplane package; configurations and providers.

A package is not a distinct type in the Crossplane API, but rather a class of
types. Therefore Pascal case is not appropriate.

### Configuration

A configuration extends Crossplane by installing conceptually related groups of
XRDs and Compositions, as well as dependencies like providers or further
configurations. Put otherwise, it configures the opinionated platform API
that Crossplane exposes.

A `Configuration` is a distinct type in the Crossplane API, therefore Pascal
case and fixed width fonts are appropriate.

### Provider

A provider extends Crossplane by installing controllers for new kinds of managed
resources. Providers typically group conceptually related managed resources; for
example the AWS provider installs support for AWS managed resources like
RDSInstance and S3Bucket.

A `Provider` is a distinct type in the Crossplane API, therefore Pascal case and
fixed width fonts are appropriate. Note that each Provider package has its own
configuration type, called a `ProviderConfig`. Don’t confuse the two; the former
installs the provider while the latter specifies configuration that is relevant
to all of its managed resources.

> Providers are directly analogous to Terraform providers.

### Crossplane Resource Model

The Crossplane Resource Model or XRM is neither a distinct Crossplane API type,
or a class of types. Rather it represents the fact that Crossplane has a
consistent, opinionated API. The strict definition of the XRM is currently
somewhat vague, but it could broadly be interpreted as a catchall term referring
to all of the concepts mentioned on this page.

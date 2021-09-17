---
title: Composite Resources
toc: true
weight: 103
indent: true
---

# Composite Resources

## Overview

Crossplane Composite Resources are opinionated Kubernetes Custom Resources that
are _composed_ of [Managed Resources][managed-resources]. We often call them XRs
for short.

![Diagram of claims, XRs, and Managed Resources][xrs-and-mrs]

Composite Resources are designed to let you build your own platform with your
own opinionated concepts and APIs without needing to write a Kubernetes
controller from scratch. Instead, you define the schema of your XR and teach
Crossplane which Managed Resources it should compose (i.e. create) when someone
creates the XR you defined.

If you're already familiar with Composite Resources and looking for a detailed
configuration reference or some tips, tricks, and troubleshooting information,
try the [Composition Reference][xr-ref].

Below is an example of a Composite Resource:

```yaml
apiVersion: database.example.org/v1alpha1
kind: XPostgreSQLInstance
metadata:
  name: my-db
spec:
  parameters:
    storageGB: 20
  compositionRef:
    name: production
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: my-db-connection-details
```

You define your own XRs, so they can be of whatever API version and kind you
like, and contain whatever spec and status fields you need.

## How It Works

The first step towards using Composite Resources is configuring Crossplane so
that it knows what XRs you'd like to exist, and what to do when someone creates
one of those XRs. This is done using a `CompositeResourceDefinition` (XRD)
resource and one or more `Composition` resources.

Once you've configured Crossplane with the details of your new XR you can either
create one directly, or use a _claim_. Typically only the folks responsible for
configuring Crossplane (often a platform or SRE team) have permission to create
XRs directly. Everyone else manages XRs via a lightweight proxy resource called
a Composite Resource Claim (or claim for short). More on that later.

![Diagram combining all Composition concepts][how-it-works]

> If you're coming from the Terraform world you can think of an XRD as similar
> to the `variable` blocks of a Terraform module, while the `Composition` is
> the rest of the module's HCL code that describes how to use those variables to
> create a bunch of resources. In this analogy the XR or claim is a little like
> a `tfvars` file providing inputs to the module.

### Defining Composite Resources

A `CompositeResourceDefinition` (or XRD) defines the type and schema of your XR.
It lets Crossplane know that you want a particular kind of XR to exist, and what
fields that XR should have. An XRD is a little like a `CustomResourceDefinition`
(CRD), but slightly more opinionated. Writing an XRD is mostly a matter of
specifying an OpenAPI ["structural schema"][crd-docs].

The XRD that defines the `XPostgreSQLInstance` XR above would look like this:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xpostgresqlinstances.database.example.org
spec:
  group: database.example.org
  names:
    kind: XPostgreSQLInstance
    plural: xpostgresqlinstances
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              parameters:
                type: object
                properties:
                  storageGB:
                    type: integer
                required:
                - storageGB
            required:
            - parameters
```

You might notice that the `XPostgreSQLInstance` example above has some fields
that don't appear in the XRD, like the `writeConnectionSecretToRef` and
`compositionRef` fields. This is because Crossplane automatically injects some
standard Crossplane Resource Model (XRM) fields into all XRs.

### Configuring Composition

A `Composition` lets Crossplane know what to do when someone creates a Composite
Resource. Each `Composition` creates a link between an XR and a set of one or
more Managed Resources - when the XR is created, updated, or deleted the set of
Managed Resources are created, updated or deleted accordingly.

You can add multiple Compositions for each XRD, and choose which should be used
when XRs are created. This allows a Composition to act like a class of service -
for example you could configure one Composition for each environment you
support, such as production, staging, and development.

A basic `Composition` for the above `XPostgreSQLInstance` might look like this:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
  labels:
    crossplane.io/xrd: xpostgresqlinstances.database.example.org
    provider: gcp
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance
  resources:
  - name: cloudsqlinstance
    base:
      apiVersion: database.gcp.crossplane.io/v1beta1
      kind: CloudSQLInstance
      spec:
        forProvider:
          databaseVersion: POSTGRES_9_6
          region: us-central1
          settings:
            tier: db-custom-1-3840
            dataDiskType: PD_SSD
            ipConfiguration:
              ipv4Enabled: true
              authorizedNetworks:
                - value: "0.0.0.0/0"
    patches:
    - type: FromCompositeFieldPath
      fromFieldPath: spec.parameters.storageGB
      toFieldPath: spec.forProvider.settings.dataDiskSizeGb
```

The above `Composition` tells Crossplane that when someone creates an
`XPostgreSQLInstance` XR Crossplane should create a `CloudSQLInstance` in
response. The `storageGB` field of the `XPostgreSQLInstance` should be used to
configure the `dataDiskSizeGb` field of the `CloudSQLInstance`. This is only a
small subset of the functionality a `Composition` enables - take a look at the
[reference page][xr-ref] to learn more.

> We almost always talk about XRs composing Managed Resources, but actually an
> XR can also compose other XRs to allow nested layers of abstraction. XRs don't
> support composing arbitrary Kubernetes resources (e.g. Deployments, operators,
> etc) directly but you can do so using our [Kubernetes][provider-kubernetes]
> and [Helm][provider-helm] providers.

### Claiming Composite Resources

Crossplane uses Composite Resource Claims (or just claims, for short) to allow
application operators to provision and manage XRs. When we talk about using XRs
it's typically implied that the XR is being used via a claim. Claims are almost
identical to their corresponding XRs. It helps to think of a claim as an
application team’s interface to an XR. You could also think of claims as the
public (app team) facing part of the opinionated platform API, while XRs are the
private (platform team) facing part.

A claim for the `XPostgreSQLInstance` XR above would look like this:

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  namespace: default
  name: my-db
spec:
  parameters:
    storageGB: 20
  compositionRef:
    name: production
  writeConnectionSecretToRef:
    name: my-db-connection-details
```

There are three key differences between an XR and a claim:

1. Claims are namespaced, while XRs (and Managed Resources) are cluster scoped.
1. Claims are of a different `kind` than the XR - by convention the XR's `kind`
   without the proceeding `X`. For example a `PostgreSQLInstance` claims an
   `XPostgreSQLInstance`.
1. An active claim contains a reference to its corresponding XR, while an XR
   contains both a reference to the claim an array of references to the managed
   resources it composes.

Not all XRs offer a claim - doing so is optional. See the XRD section of the
[Composition reference][xr-ref] to learn how to offer a claim.

![Diagram showing the relationship between claims and XRs][claims-and-xrs]

Claims may seem a little superfluous at first, but they enable some handy
scenarios, including:

- **Private XRs.** Sometimes a platform team might not want a type of XR to be
  directly consumed by their application teams. For example because the XR
  represents 'supporting' infrastructure - consider the above VPC `XNetwork` XR. App
  teams might create `PostgreSQLInstance` claims that _reference_ (i.e. consume)
  an `XNetwork`, but they shouldn't be _creating their own_. Similarly, some
  kinds of XR might be intended only for 'nested' use - intended only to be
  composed by other XRs.

- **Global XRs**. Not all infrastructure is conceptually namespaced. Say your
  organisation uses team scoped namespaces. A `PostgreSQLInstance` that belongs
  to Team A should probably be part of the `team-a` namespace - you'd represent
  this by creating a `PostgreSQLInstance` claim in that namespace. On the other
  hand the `XNetwork` XR we mentioned previously could be referenced (i.e. used)
  by XRs from many different namespaces - it doesn't exist to serve a particular
  team.

- **Pre-provisioned XRs**. Finally, separating claims from XRs allows a platform
  team to pre-provision certain kinds of XR. Typically an XR is created
  on-demand in response to the creation of a claim, but it's also possible for a
  claim to instead request an existing XR. This can allow application teams to
  instantly claim infrastructure like database instances that would otherwise
  take minutes to provision on-demand.

[managed-resources]: managed-resources.md
[xrs-and-mrs]: ../media/composition-xrs-and-mrs.svg
[xr-ref]: ../reference/composition.md
[how-it-works]: ../media/composition-how-it-works.svg
[crd-docs]: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[provider-kubernetes]: https://github.com/crossplane-contrib/provider-kubernetes
[provider-helm]: https://github.com/crossplane-contrib/provider-helm
[claims-and-xrs]: ../media/composition-claims-and-xrs.svg

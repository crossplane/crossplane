---
title: Composition
toc: true
weight: 304
indent: true
---

# Overview

This reference provides detailed examples of defining, configuring, and using
Composite Resources in Crossplane. You can also refer to Crossplane's [API
documentation][api-docs] for more details. If you're looking for a more general
overview of Composite Resources and Composition in Crossplane, try the
[Composite Resources][xr-concepts] page under Concepts.

## Composite Resources and Claims

The type and most of the schema of Composite Resources and claims are largely of
your own choosing, but there is some common 'machinery' injected into them.
Here's a hypothetical XR that doesn't have any user-defined fields and thus only
includes the automatically injected Crossplane machinery:

```yaml
apiVersion: database.example.org/v1alpha1
kind: XPostgreSQLInstance
metadata:
  # This XR was created automatically by a claim, so its name is derived from
  # the claim's name.
  name: my-db-mfd1b
  annotations:
    # The external name annotation has special meaning in Crossplane. When a
    # claim creates an XR its external name will automatically be propagated to
    # the XR. Whether and how the external name is propagated to the resources
    # the XR composes is up to its Composition.
    crossplane.io/external-name: production-db-0
spec:
  # XRs have a reference to the claim that created them (or, if the XR was
  # pre-provisioned, to the claim that later claimed them).
  claimRef:
    apiVersion: database.example.org/v1alpha1
    kind: PostgreSQLInstance
    name: my-db
  # The compositionRef specifies which Composition this XR will use to compose
  # resources when it is created, updated, or deleted. This can be omitted and
  # will be set automatically if the XRD has a default or enforced composition
  # reference, or if the below composition selector is set.
  compositionRef:
    name: production-us-east
  # The compositionSelector allows you to match a Composition by labels rather
  # than naming one explicitly. It is used to set the compositionRef if none is
  # specified explicitly.
  compositionSelector:
    matchLabels:
      environment: production
      region: us-east
      provider: gcp
  # The resourceRefs array contains references to all of the resources of which
  # this XR is composed. Despite being in spec this field isn't intended to be
  # configured by humans - Crossplane will take care of keeping it updated.
  resourceRefs:
  - apiVersion: database.gcp.crossplane.io/v1beta1
    kind: CloudSQLInstance
    name: my-db-mfd1b-md9ab
  # The writeConnectionSecretToRef field specifies a Kubernetes Secret that this
  # XR should write its connection details (if any) to.
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: my-db-connection-details
status:
  # An XR's 'Ready' condition will become True when all of the resources it
  # composes are deemed ready. Refer to the Composition 'readinessChecks' field
  # for more information.
  conditions:
  - type: Ready
    statue: "True"
    reason: Available
    lastTransitionTime: 2021-10-02T07:20:50.52Z
  # The last time the XR published its connection details to a Secret.
  connectionDetails:
    lastPublishedTime: 2021-10-02T07:20:51.24Z
```

Similarly, here's an example of the claim that corresponds to the above XR:

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  # Claims are namespaced, unlike XRs.
  namespace: default
  name: my-db
  annotations:
    # The external name annotation has special meaning in Crossplane. When a
    # claim creates an XR its external name will automatically be propagated to
    # the XR. Whether and how the external name is propagated to the resources
    # the XR composes is up to its Composition.
    crossplane.io/external-name: production-db-0
spec:
  # The resourceRef field references the XR this claim corresponds to. You can
  # either set it to an existing (compatible) XR that you'd like to claim or
  # (the more common approach) leave it blank and let Crossplane automatically
  # create and reference an XR for you.
  resourceRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance
    name: my-db-mfd1b
  # A claim's compositionRef and compositionSelector work the same way as an XR.
  compositionRef:
    name: production-us-east
  compositionSelector:
    matchLabels:
      environment: production
      region: us-east
      provider: gcp
  # A claim's writeConnectionSecretToRef mostly works the same way as an XR's.
  # The one difference is that the Secret is always written to the namespace of
  # the claim.
  writeConnectionSecretToRef:
    name: my-db-connection-details
status:
  # A claim's 'Ready' condition will become True when its XR's 'Ready' condition
  # becomes True.
  conditions:
  - type: Ready
    statue: "True"
    reason: Available
    lastTransitionTime: 2021-10-02T07:20:50.52Z
  # The last time the claim published its connection details to a Secret.
  connectionDetails:
    lastPublishedTime: 2021-10-02T07:20:51.24Z
```

> If your XR or claim isn't working as you'd expect you can try running `kubectl
> describe` against it for details - pay particular attention to any events and
> status conditions. You may need to follow the references from claim to XR to
> composed resources to find out what's happening.

## CompositeResourceDefinitions

Below is an example `CompositeResourceDefinition` that includes all configurable
fields.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  # XRDs must be named '<plural>.<group>', per the plural and group names below.
  name: xpostgresqlinstances.example.org
spec:
  # This XRD defines an XR in the 'example.org' API group.
  group: example.org
  # The kind of this XR will be 'XPostgreSQLInstance`. You may also optionally
  # specify a singular name and a listKind.
  names:
    kind: XPostgreSQLInstance
    plural: xpostgresqlinstances
  # This type of XR offers a claim. Omit claimNames if you don't want to do so.
  # The claimNames must be different from the names above - a common convention
  # is that names are prefixed with 'X' while claim names are not. This lets app
  # team members think of creating a claim as (e.g.) 'creating a
  # PostgreSQLInstance'.
  claimNames:
    kind: PostgreSQLInstance
    plural: postgresqlinstances
  # Each type of XR must declare any keys they write to their connection secret.
  connectionSecretKeys:
  - hostname
  # Each type of XR may specify a default Composition to be used when none is
  # specified (e.g. when the XR has no compositionRef or selector). A similar
  # enforceCompositionRef field also exists to allow XRs to enforce a specific
  # Composition that should always be used.
  defaultCompositionRef:
    name: example
  # Each type of XR may be served at different versions - e.g. v1alpha1, v1beta1
  # and v1 - simultaneously. Currently Crossplane requires that all versions
  # have an identical schema, so this is mostly useful to 'promote' a type of XR
  # from alpha to beta to production ready.
  versions:
  - name: v1alpha1
    # Served specifies that XRs should be served at this version. It can be set
    # to false to temporarily disable a version, for example to test whether
    # doing so breaks anything before a version is removed wholesale.
    served: true
    # Referenceable denotes the version of a type of XR that Compositions may
    # use. Only one version may be referenceable.
    referenceable: true
    # Schema is an OpenAPI schema just like the one used by Kubernetes CRDs. It
    # determines what fields your XR and claim will have. Note that Crossplane
    # will automatically extend with some additional Crossplane machinery.
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
          status:
            type: object
            properties:
              address:
                description: Address of this MySQL server.
                type: string
```

Take a look at the Kubernetes [CRD documentation][crd-docs] for a more detailed
guide to writing OpenAPI schemas. Note that the following fields are reserved
for Crossplane machinery, and will be ignored if your schema includes them:

* `spec.resourceRef`
* `spec.resourceRefs`
* `spec.claimRef`
* `spec.writeConnectionSecretToRef`
* `status.conditions`
* `status.connectionDetails`

> If your `CompositeResourceDefinition` isn't working as you'd expect you can
> try running `kubectl describe xrd` for details - pay particular attention to
> any events and status conditions.

## Compositions

You'll encounter a lot of 'field paths' when reading or writing a `Composition`.
Field paths reference a field within a Kubernetes object via a simple string
'path'. [API conventions][field-paths] describe the syntax as:

> Standard JavaScript syntax for accessing that field, assuming the JSON object
> was transformed into a JavaScript object, without the leading dot, such as
> `metadata.name`.

 Valid field paths include:

* `metadata.name` - The `name` field of the `metadata` object.
* `spec.containers[0].name` - The `name` field of the 0th `containers` element.
* `data[.config.yml]` - The `.config.yml` field of the `data` object.
* `apiVersion` - The `apiVersion` field of the root object.

 While the following are invalid:

* `.metadata.name` - Leading period.
* `metadata..name` - Double period.
* `metadata.name.` - Trailing period.
* `spec.containers[]` - Empty brackets.
* `spec.containers.[0].name` - Period before open bracket.

Below is a detailed example of a `Composition`. While detailed, this example
doesn't include every patch, transform, connection detail, and readiness check
type. Keep reading below to discover those.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
  labels:
    crossplane.io/xrd: xpostgresqlinstances.database.example.org
    provider: gcp
spec:

  # Each Composition must declare that it is compatible with a particular type
  # of Composite Resource using its 'compositeTypeRef' field. The referenced
  # version must be marked 'referenceable' in the XRD that defines the XR.
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: XPostgreSQLInstance

  # When an XR is created in response to a claim Crossplane needs to know where
  # it should create the XR's connection secret. This is configured using the
  # 'writeConnectionSecretsToNamespace' field.
  writeConnectionSecretsToNamespace: crossplane-system

  # Each Composition must specify at least one composed resource template. In
  # this case the Composition tells Crossplane that it should create, update, or
  # delete a CloudSQLInstance whenever someone creates, updates, or deletes an
  # XPostgresSQLInstance.
  resources:

    # It's good practice to provide a unique name for each entry. Note that
    # this identifies the resources entry within the Composition - it's not
    # the name the CloudSQLInstance. The 'name' field will be required in a
    # future version of this API.
  - name: cloudsqlinstance

    # The 'base' template for the CloudSQLInstance Crossplane will create.
    # You can use the base template to specify fields that never change, or
    # default values for fields that may optionally be patched over. Bases must
    # be a valid Crossplane resource - a Managed Resource, Composite Resource,
    # or a ProviderConfig.
    base:
      apiVersion: database.gcp.crossplane.io/v1beta1
      kind: CloudSQLInstance
      spec:
        forProvider:
          databaseVersion: POSTGRES_9_6
          region: us-central1
          settings:
            dataDiskType: PD_SSD
            ipConfiguration:
              ipv4Enabled: true
              authorizedNetworks:
                - value: "0.0.0.0/0"
      
    # Each resource can optionally specify a set of 'patches' that copy fields
    # from (or to) the XR.
    patches:
      # FromCompositeFieldPath is the default when 'type' is omitted, but it's
      # good practice to always include the type for readability.
    - type: FromCompositeFieldPath
      fromFieldPath: spec.parameters.size
      toFieldPath: spec.forProvider.settings.tier

      # Each patch can optionally specify one or more 'transforms', which
      # transform the 'from' field's value before applying it to the 'to' field.
      # Transforms are applied in the order they are specified; each transform's
      # output is passed to the following transform's input.
      transforms:
      - type: map
        map:
          medium: db-custom-1-3840

      policy:
        # By default a patch from a field path that does not exist is simply
        # skipped until it does. Use the 'Required' policy to instead block and
        # return an error when the field path does not exist.
        fromFieldPath: Required

        # You can patch entire objects or arrays from one resource to another.
        # By default the 'to' object or array will be overwritten, not merged.
        # Use the 'mergeOptions' field to override this behaviour. Note that
        # these fields accidentally leak Go terminology - 'slice' means 'array'.
        # 'map' means 'map' in YAML or 'object' in JSON.
        mergeOptions:
          appendSlice: true
          keepMapValues: true
    
    # You can include connection details to propagate from this CloudSQLInstance
    # up to the XPostgreSQLInstance XR (and then on to the PostgreSQLInstance
    # claim). Remember that your XRD must declare which connection secret keys
    # it supports.
    connectionDetails:
    - name: hostname
      fromConnectionSecretKey: hostname
    
    # By default an XR's 'Ready' status condition will become True when the
    # 'Ready' status conditions of all of its composed resources become true.
    # You can optionally specify custom readiness checks to override this.
    readinessChecks:
    - type: None

    
  # If you find yourself repeating patches a lot you can group them as a named
  # 'patch set' then use a PatchSet type patch to reference them.
  patchSets:
  - name: metadata
    patches:
    - type: FromCompositeFieldPath
      # When both field paths are the same you can omit the 'toFieldPath' and it
      # will default to the 'fromFieldPath'.
      fromFieldPath: metadata.labels[some-important-label]
```

### Patch Types

You can use the following types of patch in a `Composition`:

`FromCompositeFieldPath`. The default if the `type` is omitted. This type
patches from a field within the XR to a field within the composed resource. It's
commonly used to expose a composed resource spec field as an XR spec field.

```yaml
# Patch from the XR's spec.parameters.size field to the composed resource's
# spec.forProvider.settings.tier field.
- type: FromCompositeFieldPath
  fromFieldPath: spec.parameters.size
  toFieldPath: spec.forProvider.settings.tier
```

`ToCompositeFieldPath`. The inverse of `FromCompositeFieldPath`. This type
patches from a field within the composed resource to a field within the XR. It's
commonly used to derive an XR status field from a composed resource status
field.

```yaml
# Patch from the composed resource's status.atProvider.zone field to the XR's
# status.zone field.
- type: ToCompositeFieldPath
  fromFieldPath: status.atProvider.zone
  toFieldPath: status.zone
```

`CombineFromComposite`. Combines multiple fields from the XR to produce one
composed resource field.

```yaml
# Patch from the XR's spec.parameters.location field and the
# metadata.annotations[crossplane.io/claim-name] annotation to the composed
# resource's spec.forProvider.administratorLogin field.
- type: CombineFromComposite
  combine:
    # The patch will only be applied when all variables have non-zero values.
    variables:
    - fromFieldPath: spec.parameters.location
    - fromFieldPath: metadata.annotations[crossplane.io/claim-name]
    strategy: string
    string:
      fmt: "%s-%s"
  toFieldPath: spec.forProvider.administratorLogin
  # By default Crossplane will skip the patch until all of the variables to be
  # combined have values. Set the fromFieldPath policy to 'Required' to instead
  # abort composition and return an error if a variable has no value.
  policy:
    fromFieldPath: Required
```

At the time of writing only the `string` combine strategy is supported. It uses
[Go string formatting][pkg/fmt] to combine values, so if the XR's location was
`us-west` and its claim name was `db` the composed resource's administratorLogin
would be set to `us-west-db`.

`CombineToComposite` is the inverse of `CombineFromComposite`.

```yaml
# Patch from the composed resource's spec.parameters.administratorLogin and
# status.atProvider.fullyQualifiedDomainName fields back to the XR's
# status.adminDSN field.
- type: CombineToComposite
  combine:
    variables:
      - fromFieldPath: spec.parameters.administratorLogin
      - fromFieldPath: status.atProvider.fullyQualifiedDomainName
    strategy: string
    # Here, our administratorLogin parameter and fullyQualifiedDomainName
    # status are formatted to a single output string representing a DSN.
    string:
      fmt: "mysql://%s@%s:3306/my-database-name"
  toFieldPath: status.adminDSN
```

`PatchSet`. References a named set of patches defined in the `spec.patchSets`
array of a `Composition`.

```yaml
# This is equivalent to specifying all of the patches included in the 'metadata'
# PatchSet.
- type: PatchSet
  patchSetName: metadata
```

The `patchSets` array may not contain patches of `type: PatchSet`. The
`transforms` and `patchPolicy` fields are ignored by `type: PatchSet`.

### Transform Types

You can use the following types of transform on a value being patched:

`map`. Transforms values using a map.

```yaml
# If the value of the 'from' field is 'us-west', the value of the 'to' field
# will be set to 'West US'.
- type: map
  map:
    us-west: West US
    us-east: East US
    au-east: Australia East
```

`math`. Transforms values using math. The input value must be an integer.
Currently only `multiply` is supported.

```yaml
# If the value of the 'from' field is 2, the value of the 'to' field will be set
# to 4.
- type: math
  math:
    multiply: 2
```

`string`. Transforms string values. Currently only [Go style `fmt`][pkg/fmt] is
supported.

```yaml
# If the value of the 'from' field is 'hello', the value of the 'to' field will
# be set to 'hello-world'.
- type: string
  string:
    fmt: "%s-world"
```

`convert`. Transforms values of one type to another, for example from a string
to an integer. The following values are supported by the `from` and `to` fields:

* `string`
* `bool`
* `int`
* `int64`
* `float64`

The strings 1, t, T, TRUE, true, and True are considered 'true', while 0, f, F,
FALSE, false, and False are considered 'false'. The integer 1 and float 1.0 are
considered true, while all other values are considered false. Similarly, boolean
true converts to integer 1 and float 1.0, while false converts to 0 and 0.0.

```yaml
# If the value to be converted is "1" (a string), the value of the 'toType'
# field will be set to 1 (an integer).
- type: convert
  convert:
   toType: int
```

### Connection Details

You can derive the following types of connection details from a composed
resource:

`FromConnectionSecretKey`. Derives an XR connection detail from a connection
secret key of a composed resource.

```yaml
# Derive the XR's 'user' connection detail from the 'username' key of the
# composed resource's connection secret.
- type: FromConnectionSecretKey
  name: user
  fromConnectionSecretKey: username
```

`FromFieldPath`. Derives an XR connection detail from a field path within the
composed resource.

```yaml
# Derive the XR's 'user' connection detail from the 'adminUser' status field of
# the composed resource.
- type: FromFieldPath
  name: user
  fromFieldPath: status.atProvider.adminUser
```

`FromValue`. Derives an XR connection detail from a fixed value.

```yaml
# Always sets the XR's 'user' connection detail to 'admin'.
- type: FromFieldPath
  name: user
  fromValue: admin
```

### Readiness Checks

Crossplane can use the following types of readiness check to determine whether a
composed resource is ready (and therefore whether the XR and claim should be
considered ready). Specify multiple readiness checks if multiple conditions must
be met for a composed resource to be considered ready.

> Note that if you don't specify any readiness checks Crossplane will consider
> the composed resource to be ready when its 'Ready' status condition becomes
> 'True'.

`MatchString`. Considers the composed resource to be ready when the value of a
field within that resource matches a specified string.

```yaml
# The composed resource will be considered ready when the 'state' status field
# matches the string 'Online'.
- type: MatchString
  fieldPath: status.atProvider.state
  matchString: "Online"
```

`MatchInteger`. Considers the composed resource to be ready when the value of a
field within that resource matches a specified integer.

```yaml
# The composed resource will be considered ready when the 'state' status field
# matches the integer 4.
- type: MatchString
  fieldPath: status.atProvider.state
  matchInteger: 4
```

`NonEmpty`. Considers the composed resource to be ready when a field exists in
the composed resource. The name of this check can be a little confusing in that
a field that exists with a zero value (e.g. an empty string or zero integer) is
not considered to be 'empty', and thus will pass the readiness check.

`None`. Considers the composed resource to be ready as soon as it exists.

```yaml
# The composed resource will be considered ready if and when 'online' status
# field  exists.
- type: NonEmpty
  fieldPath: status.atProvider.online
```

### Missing Functionality

You might find while reading through this reference that Crossplane is missing
some functionality you need to compose resources. If that's the case, please
[raise an issue] with as much detail **about your use case** as possible. Please
understand that the Crossplane maintainers are growing the feature set of the
`Composition` type conservatively. We highly value the input of our users and
community, but we also feel it's critical to avoid bloat and complexity. We
therefore wish to carefully consider each new addition. We feel some features
may be better suited for a real, expressive programming language and intend to
build an alternative to the `Composition` type as it is documented here per
[this proposal][issue-2524].

## Tips, Tricks, and Troubleshooting

In this section we'll cover some common tips, tricks, and troubleshooting steps
for working with Composite Resources. If you're trying to track down why your
Composite Resources aren't working the [Troubleshooting][trouble-ref] page also
has some useful information.

### Troubleshooting Claims and XRs

Crossplane relies heavily on status conditions and events for troubleshooting.
You can see both using `kubectl describe` - for example:

```console
# Describe the PostgreSQLInstance claim named my-db
kubectl describe postgresqlinstance.database.example.org my-db
```

Per Kubernetes convention, Crossplane keeps errors close to the place they
happen. This means that if your claim is not becoming ready due to an issue with
your `Composition` or with a composed resource you'll need to "follow the
references" to find out why. Your claim will only tell you that the XR is not
yet ready.

To follow the references:

1. Find your XR by running `kubectl describe` on your claim and looking for its
   "Resource Ref" (aka `spec.resourceRef`).
1. Run `kubectl describe` on your XR. This is where you'll find out about issues
   with the `Composition` you're using, if any.
1. If there are no issues but your XR doesn't seem to be becoming ready, take a
   look for the "Resource Refs" (or `spec.resourceRefs`) to find your composed
   resources.
1. Run `kubectl describe` on each referenced composed resource to determine
   whether it is ready and what issues, if any, it is encountering.

### Composite Resource Connection Secrets

Claim and Composite Resource connection secrets are often derived from the
connection secrets of the managed resources they compose. This is a common
source of confusion because several things need to align for it to work:

1. The XR/claim's connection secret keys must be declared by the XRD.
1. The `Composition` must specify how to derive connection details from each
   composed resource.
1. If connection details are derived from a composed resource's connection
   secret that composed resource must specify its `writeConnectionSecretToRef`.
1. The claim and XR must both specify a `writeConnectionSecretToRef`.

Finally, you can't currently edit a XRD's supported connection details. The
XRD's `spec.connectionSecretKeys` is effectively immutable. This may change in
future per [this issue][issue-2024]

### Claiming an Existing Composite Resource

Most people create Composite Resources using a claim, but you can actually claim
an existing Composite Resource as long as its a type of XR that offers a claim
and no one else has already claimed it. To do so:

1. Set the `spec.resourceRef` of your claim to reference the existing XR.
1. Make sure the rest of your claim's spec fields match the XR's.

If your claim's spec fields don't match the XR's Crossplane will still claim it
but will then try to update the XR's spec fields to match the claim's.

### Influencing External Names

The `crossplane.io/external-name` annotation has special meaning to Crossplane
managed resources - it specifies the name (or identifier) of the resource in the
external system, for example the actual name of a `CloudSQLInstance` in the GCP
API. Some managed resources don't let you specify an external name - in those
cases Crossplane will set it for you to whatever the external system requires.

If you add the `crossplane.io/external-name` annotation to a claim Crossplane
will automatically propagate it when it creates an XR. It's good practice to
have your `Composition` further propagate the annotation to one or more composed
resources, but it's not required.

### Mixing and Matching Providers

Crossplane has providers for many things in addition to the big clouds. Take a
look at [github.com/crossplane-contrib][crossplane-contrib] to find many of
them. Keep in mind that you can mix and match managed resources from different
providers within a `Composition` to create Composite Resources. For example you
might use provider-aws and provider-sql to create an XR that provisions an
`RDSInstance` then creates an SQL `Database` and `User`, or provider-gcp and
provider-helm to create a `GKECluster` and deploy a Helm Chart `Release` to it.

Often when mixing and matching providers you'll need to compose a
`ProviderConfig` for one provider that loads credentials from the connection
secret of a managed resource from another provider. Sometimes you may need to
use an intermediary XR to mutate the connection details to suit your needs.
[This example][helm-and-gcp] from provider-helm demonstrates using a GKE cluster
connection secret as Helm `ProviderConfig` credentials.

### Patching From One Composed Resource to Another

It's not possible to patch _directly_ from one composed resource to another -
i.e. from one entry in the `spec.resources` array of a `Composition` to another.
It is however possible to achieve this by using the XR as an intermediary. To do
so:

1. Use a `ToCompositeFieldPath` patch to patch from your source composed
   resource to the XR. Typically you'll want to patch to a status field or an
   annotation.
1. Use a `FromCompositeFieldPath` patch to patch from the 'intermediary' field
   you patched to in step 1 to a field on the destination composed resource.

[api-docs]: ../api-docs/crossplane.md
[xr-concepts]: ../concepts/composition.md
[crd-docs]: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
[raise an issue]: https://github.com/crossplane/crossplane/issues/new?assignees=&labels=enhancement&template=feature_request.md
[issue-2524]: https://github.com/crossplane/crossplane/issues/2524
[field-paths]:  https://github.com/kubernetes/community/blob/61f3d0/contributors/devel/sig-architecture/api-conventions.md#selecting-fields
[pkg/fmt]: https://golang.org/pkg/fmt/
[trouble-ref]: troubleshoot.md
[crossplane-contrib]: https://github.com/crossplane-contrib
[helm-and-gcp]: https://github.com/crossplane-contrib/provider-helm/blob/2dcbdd0/examples/in-composition/composition.yaml
[issue-2024]: https://github.com/crossplane/crossplane/issues/2024

---
title: Composing Infrastructure
toc: true
weight: 103
indent: true
---

# Composing Infrastructure

## Composition

Providers extend Crossplane with custom resources that can be used to
declaratively configure a system. The AWS provider for example, adds custom
resources for AWS services like RDS and S3. We call these 'managed resources'.
Managed resources match the APIs of the system they represent as closely as
possible, but they’re also opinionated. Common functionality like status
conditions and references work the same no matter which provider you're using -
all managed resources comply with the Crossplane Resource Model, or XRM. Despite
the name, 'provider' doesn’t necessarily mean 'cloud provider'. The Crossplane
community has built providers that add support for managing databases on a [SQL
server](https://github.com/crossplane-contrib/provider-sql), managing
[Helm releases](https://github.com/crossplane-contrib/provider-helm/),
and [ordering pizza](https://blog.crossplane.io/providers-101-ordering-pizza-with-kubernetes-and-crossplane/).

Composition allows platform builders to define new custom resources that are
composed of managed resources. We call these composite resources, or XRs. An XR
typically groups together a handful of managed resources into one logical
resource, exposing only the settings that the platform builer deems useful and
deferring the rest to an API-server-side template we call a 'Composition'.

Composition can be used to build a catalogue of custom resources and classes of
configuration that fit the needs and opinions of your organisation. A platform
team might define their own `MySQLInstance` XR, for example. This XR would allow
the platform customers they support to self-service their database needs by
ensuring they can configure only the settings that _your_ organisation needs -
perhaps engine version and storage size. All other settings are deferred to a
selectable composition representing a configuration class like "production" or
"staging". Compositions can hide infrastructure complexity and include policy
guardrails so that applications can easily and safely consume the infrastructure
they need, while conforming to your organisational best-practices.

## Concepts

![Infrastructure Composition Concepts]

A _Composite Resource_ (XR) is a special kind of custom resource that is
composed of other resources. Its schema is user-defined. The
`CompositeMySQLInstance` in the above diagram is a composite resource. The kind
of a composite resource is configurable - the `Composite` prefix is not
required.

A `Composition` specifies how Crossplane should reconcile a composite
infrastructure resource - i.e. what infrastructure resources it should compose.
For example the Azure `Composition` configures Crossplane to reconcile a
`CompositeMySQLInstance` by creating and managing the lifecycle of an Azure
`MySQLServer` and `MySQLServerFirewallRule`.

A _Composite Resource Claim_ (XRC) for an resource declares that an application
requires particular kind of infrastructure, as well as specifying how to
configure it. The `MySQLInstance` resources in the above diagram declare that
the application pods each require a `CompositeMySQLInstance`. As with composite
resources, the kind of the claim is configurable. Offering a claim is optional.

A `CompositeResourceDefinition` (XRD) defines a new kind of composite resource,
and optionally the claim it offers. The `CompositeResourceDefinition` in the
above diagram defines the `CompositeMySQLInstance` composite resource, and its
corresponding `MySQLInstance` claim.

> Note that composite resources and compositions are _cluster scoped_ - they
> exist outside of any Kubernetes namespace. A claim is a namespaced proxy for a
> composite resource. This enables Crossplane to model complex relationships
> between XRs that may span namespace boundaries - for example MySQLInstances
> spread across multiple namespaces can all share a VPC that exists above any
> namespace.

## Creating A New Kind of Composite Resource

New kinds of composite resource are defined by a platform builder. There are
two steps to this process:

1. Define your composite resource, and optionally the claim it offers.
1. Specify one or more possible ways your composite resource may be composed.

### Define your Composite Resource

Composite resources are defined by a `CompositeResourceDefinition`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  # XRDs follow the constraints of CRD names. They must be named
  # <plural>.<group>, per the plural and group names configured by the
  # crdSpecTemplate below.
  name: compositemysqlinstances.example.org
spec:
  # Composite resources may optionally expose a connection secret - a Kubernetes
  # Secret containing all of the details a pod might need to connect to the
  # resource. Resources that wish to expose a connection secret must declare
  # what keys they support. These keys form a 'contract' - any composition that
  # intends to be compatible with this resource must compose resources that
  # supply these connection secret keys.
  connectionSecretKeys:
  - username
  - password
  - hostname
  - port
  # You can specify a default Composition resource to be selected if there is
  # no composition selector or reference was supplied on the Custom Resource.
  defaultCompositionRef:
    name: example-azure
  # An enforced composition will be selected for all instances of this type and
  # will override any selectors and references.
  # enforcedCompositionRef:
  #   name: securemysql.acme.org
  group: example.org
  # The defined kind of composite resource.
  names:
    kind: CompositeMySQLInstance
    plural: compositemysqlinstances
  # The kind of claim this composite resource offers. Optional - omit the claim
  # names if you don't wish to offer a claim for this composite resource. Must
  # be different from the composite resource's kind. The established convention
  # is for the claim kind to represent what the resource is, conceptually. e.g.
  # 'MySQLInstance', not `MySQLInstanceClaim`.
  claimNames:
    kind: MySQLInstance
    plural: mysqlinstances
  # A composite resource may be served at multiple versions simultaneously, but
  # all versions must have identical schemas; Crossplane does not yet support
  # conversion between different version schemas.
  versions:
  - name: v1alpha1
    # Served specifies whether this version should be exposed via the API
    # server's REST API.
    served: true
    # Referenceable specifies whether this version may be referenced by a
    # Composition. Exactly one version may be referenceable by Compositions, and
    # that version must be served. The referenceable version will always be the
    # storage version of the underlying CRD.
    referenceable: true
    # This schema defines the configuration fields that the composite resource
    # supports. It uses the same structural OpenAPI schema as a Kubernetes CRD
    # - for example, this resource supports a spec.parameters.version enum.
    # The following fields are reserved for Crossplane's use, and will be
    # overwritten if included in this validation schema:
    #
    # - spec.resourceRef
    # - spec.resourceRefs
    # - spec.claimRef
    # - spec.writeConnectionSecretToRef
    # - status.conditions
    # - status.connectionDetails
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
                  version:
                    description: MySQL engine version
                    type: string
                    enum: ["5.6", "5.7"]
                  storageGB:
                    type: integer
                  location:
                    description: Geographic location of this MySQL server.
                    type: string
                required:
                - version
                - storageGB
                - location
            required:
            - parameters
          # The status subresource can be optionally defined in the XRD
          # schema to allow observed fields from the composed resources
          # to be set in the composite resource and claim.
          status:
            type: object
            properties:
              address:
                description: Address of this MySQL server.
                type: string
              adminDSN:
                description: DSN (Data Source Name) of the MySQL server.
                type: string
```

Refer to the Kubernetes documentation on [structural schemas] for full details
on how to configure the `openAPIV3Schema` for your composite resource.

`kubectl describe` can be used to confirm that a new composite
resource was successfully defined. Note the `Established` condition and events,
which indicate the process was successful.

```console
$ kubectl describe xrd compositemysqlinstances.example.org

Name:         compositemysqlinstances.example.org
Namespace:
Labels:       <none>
Annotations:  <none>
API Version:  apiextensions.crossplane.io/v1
Kind:         CompositeResourceDefinition
Metadata:
  Creation Timestamp:  2020-05-15T05:30:44Z
  Finalizers:
    offered.apiextensions.crossplane.io
    defined.apiextensions.crossplane.io
  Generation:        1
  Resource Version:  1418120
  UID:               f8fedfaf-4dfd-4b8a-8228-6af0f4abd7a0
Spec:
  Connection Secret Keys:
    username
    password
    hostname
    port
  Default Composition Ref:
    Name: example-azure
  Group:  example.org
  Names:
    Kind:       CompositeMySQLInstance
    List Kind:  CompositeMySQLInstanceList
    Plural:     compositemysqlinstances
    Singular:   compositemysqlinstance
  Claim Names:
    Kind:       MySQLInstance
    List Kind:  MySQLInstanceList
    Plural:     mysqlinstances
    Singular:   mysqlinstance
  Versions:
    Name:          v1alpha1
    Served:        true
    Referenceable: true
    Schema:
      openAPIV3Schema:
        Properties:
          Spec:
            Properties:
              Parameters:
                Properties:
                  Location:
                    Description:  Geographic location of this MySQL server.
                    Type:         string
                  Storage GB:
                    Type:  integer
                  Version:
                    Description:  MySQL engine version
                    Enum:
                      5.6
                      5.7
                    Type:  string
                Required:
                  version
                  storageGB
                  location
                Type:  object
            Required:
              parameters
            Type:  object
          Status:
            Properties:
              Address:
                Description:  Address of this MySQL server.
                Type:         string
              Admin DSN:
                Description:  DSN (Data Source Name) of the MySQL server.
                Type:         string
            Type:             object
        Type:                 object
Status:
  Conditions:
    Last Transition Time:  2020-05-15T05:30:45Z
    Reason:                WatchingCompositeResource
    Status:                True
    Type:                  Established
    Last Transition Time:  2020-05-15T05:30:45Z
    Reason:                WatchingCompositeResourceClaim
    Status:                True
    Type:                  Offered
  Controllers:
    Composite Resource Claim Type:
      API Version:  example.org/v1alpha1
      Kind:         MySQLInstance
    Composite Resource Type:
      API Version:  example.org/v1alpha1
      Kind:         CompositeMySQLInstance
Events:
  Type     Reason              Age                   From                                                             Message
  ----     ------              ----                  ----                                                             -------
  Normal   EstablishComposite  4m10s                 defined/compositeresourcedefinition.apiextensions.crossplane.io  waiting for composite resource CustomResourceDefinition to be established
  Normal   OfferClaim          4m10s                 offered/compositeresourcedefinition.apiextensions.crossplane.io  waiting for composite resource claim CustomResourceDefinition to be established
  Normal   ApplyClusterRoles   4m9s (x4 over 4m10s)  rbac/compositeresourcedefinition.apiextensions.crossplane.io     Applied RBAC ClusterRoles
  Normal   RenderCRD           4m7s (x8 over 4m10s)  defined/compositeresourcedefinition.apiextensions.crossplane.io  Rendered composite resource CustomResourceDefinition
  Normal   EstablishComposite  4m7s (x6 over 4m10s)  defined/compositeresourcedefinition.apiextensions.crossplane.io  Applied composite resource CustomResourceDefinition
  Normal   EstablishComposite  4m7s (x5 over 4m10s)  defined/compositeresourcedefinition.apiextensions.crossplane.io  (Re)started composite resource controller
  Normal   RenderCRD           4m7s (x6 over 4m10s)  offered/compositeresourcedefinition.apiextensions.crossplane.io  Rendered composite resource claim CustomResourceDefinition
  Normal   OfferClaim          4m7s (x4 over 4m10s)  offered/compositeresourcedefinition.apiextensions.crossplane.io  Applied composite resource claim CustomResourceDefinition
  Normal   OfferClaim          4m7s (x3 over 4m10s)  offered/compositeresourcedefinition.apiextensions.crossplane.io  (Re)started composite resource claim controller
```

### Specify How Your Resource May Be Composed

Once a new kind of composite resource is defined Crossplane must be instructed
how to reconcile that kind of resource. This is done by authoring a
`Composition`.

A `Composition`:

* Declares one kind of composite resource that it satisfies.
* Specifies a "base" configuration for one or more composed resources.
* Specifies "patches" that overlay configuration values from an instance of the
  composite resource onto each "base".

Multiple compositions may satisfy a particular kind of composite resource, and
the author of a composite resource (or resource claim) may select which
composition will be used. This allows a platform builder to expose a subset of
configuration to their customers in a granular fashion, and defer the rest to
fixed classes of configuration. A platform builder may offer their customers the
choice between an "Azure" and a "GCP" composition, or they may offer a choice
between a "production" and a "staging" composition. They can also offer a
default composition in case their customers do not supply a composition selector
or enforce a specific composition in order to override the composition choice of
users for all instances. In all cases, the customer may configure any value
supported by the composite resource's schema, with all other values being
deferred to the composition.

The below `Composition` satisfies the `CompositeMySQLInstance` defined in the
previous section by composing an Azure SQL server, firewall rule, and resource
group:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example-azure
  labels:
    purpose: example
    provider: azure
spec:
  # This Composition declares that it satisfies the CompositeMySQLInstance
  # resource defined above - i.e. it patches "from" a CompositeMySQLInstance.
  # Note that the version in apiVersion must be the referenceable version of the
  # XRD.
  compositeTypeRef:
    apiVersion: example.org/v1alpha1
    kind: CompositeMySQLInstance

  # This Composition defines a patch set with the name "metadata", which consists
  # of 2 individual patches. Patch sets can be referenced from any of the base
  # resources within the Composition to avoid having to repeat patch definitions.
  # A PatchSet can contain any of the other patch types, except another PatchSet.
  patchSets:
  - name: metadata
    patches:
    # For most patch types, when toFieldPath is omitted it defaults to
    # fromFieldPath. This does not apply to 'Combine' patch types as
    # they can accept multiple input field definitions.
    - fromFieldPath: metadata.labels
    # Exercise caution when patching labels and annotations. Crossplane replaces
    # patched objects - it does not merge them. This means that patching from
    # the 'metadata.annotations' field path will _replace_ all of a composed
    # resource's annotations, including annotations prefixed with crossplane.io/
    # that control Crossplane's behaviour. Patching the entire annotations
    # object can therefore have unexpected consquences and is not recommended.
    # Instead patch specific annotations by specifying their keys.
    - fromFieldPath: metadata.annotations[example.org/app-name]
  - name: external-name
    patches:
    # FromCompositeFieldPath is the default patch type and is thus often
    # omitted for brevity.
    - type: FromCompositeFieldPath
      fromFieldPath: metadata.annotations[crossplane.io/external-name]
      # By default a patch from a field path that does not exist is a no-op. Use
      # the 'Required' policy to instead block and return an error when the
      # field path does not exist.
      policy:
        fromFieldPath: Required

  # This Composition reconciles a CompositeMySQLInstance by patching from
  # the CompositeMySQLInstance "to" new instances of the infrastructure
  # resources below. These resources may be the managed resources of an
  # infrastructure provider such as provider-azure, or other composite
  # resources.
  resources:
    # A CompositeMySQLInstance that uses this Composition will be composed of an
    # Azure ResourceGroup. Note that the 'name' is the name of this entry in the
    # resources array - it does not affect the name of any ResourceGroup that is
    # composed using this Composition. Specifying a name is optional but is
    # *strongly* recommended. When all entries in the resources array are named
    # entries may be added, deleted, and reordered as long as their names do not
    # change. When entries are not named the length and order of the resources
    # array should be treated as immutable. Either all or no entries must be
    # named.
  - name: resourcegroup
    # The "base" for this ResourceGroup specifies the base
    # configuration that may be extended or mutated by the patches below.
    base:
      apiVersion: azure.crossplane.io/v1alpha3
      kind: ResourceGroup
      spec: {}
    # Patches copy or "overlay" the value of a field path within the composite
    # resource (the CompositeMySQLInstance) to a field path within the composed
    # resource (the ResourceGroup). In the below example any labels and
    # annotations will be propagated from the CompositeMySQLInstance to the
    # ResourceGroup (referencing the "metadata" patch set defined on the
    # Composition), as will the location, using the default patch type
    # FromCompositeFieldPath.
    patches:
    - type: PatchSet
      patchSetName: metadata
    - fromFieldPath: "spec.parameters.location"
      toFieldPath: "spec.location"

      # Sometimes it is necessary to "transform" the value from the composite
      # resource into a value suitable for the composed resource, for example an
      # Azure based composition may represent geographical locations differently
      # from a GCP based composition that satisfies the same composite resource.
      # This can be done by providing an optional array of transforms, such as
      # the below that will transform the MySQLInstance spec.parameters.location
      # value "us-west" into the ResourceGroup spec.location value "West US".
      transforms:
      - type: map
        map:
          us-west: West US
          us-east: East US
          au-east: Australia East
    # A MySQLInstance that uses this Composition will also be composed of an
    # Azure MySQLServer.
  - name: mysqlserver
    base:
      apiVersion: database.azure.crossplane.io/v1beta1
      kind: MySQLServer
      spec:
        forProvider:
          # When this MySQLServer is created it must specify a ResourceGroup in
          # which it will exist. The below resourceGroupNameSelector corresponds
          # to the spec.forProvider.resourceGroupName field of the MySQLServer.
          # It selects a ResourceGroup with a matching controller reference.
          # Two resources that are part of the same composite resource will have
          # matching controller references, so this MySQLServer will always
          # select the ResourceGroup above. If this Composition included more
          # than one ResourceGroup they could be differentiated by matchLabels.
          resourceGroupNameSelector:
            matchControllerRef: true
          sslEnforcement: Disabled
          sku:
            tier: GeneralPurpose
            capacity: 8
            family: Gen5
          storageProfile:
            backupRetentionDays: 7
            geoRedundantBackup: Disabled
        writeConnectionSecretToRef:
          namespace: crossplane-system
    patches:
    # This resource also uses the "metadata" patch set defined on the
    # Composition.
    - type: PatchSet
      patchSetName: metadata
    - fromFieldPath: "metadata.uid"
      toFieldPath: "spec.writeConnectionSecretToRef.name"
      transforms:
        # Transform the value from the CompositeMySQLInstance using Go string
        # formatting. This can be used to prefix or suffix a string, or to
        # convert a number to a string. See https://golang.org/pkg/fmt/ for more
        # detail.
      - type: string
        string:
          fmt: "%s-mysqlserver"
    - fromFieldPath: "spec.parameters.version"
      toFieldPath: "spec.forProvider.version"
    - fromFieldPath: "spec.parameters.location"
      toFieldPath: "spec.forProvider.location"
      transforms:
      - type: map
        map:
          us-west: West US
          us-east: East US
          au-east: Australia East
    - fromFieldPath: "spec.parameters.storageGB"
      toFieldPath: "spec.forProvider.storageProfile.storageMB"
      # Transform the value from the CompositeMySQLInstance by multiplying it by
      # 1024 to convert Gigabytes to Megabytes.
      transforms:
        - type: math
          math:
            multiply: 1024

    # For more complex resource definitions, it may be necessary to patch a
    # value based on multiple input values - for example when an underlying
    # resource requires a single field input but the Composite resource
    # exposes this as multiple fields. This can be achieved using the
    # CombineFromComposite patch type. We currently support combining using
    # a string strategy, which formats multiple input values to a single
    # output value using Go string formatting
    # (see https://golang.org/pkg/fmt/). The below patch sets the
    # administratorLogin setting based on an amalgam of the location parameter
    # and name of the resource claim:
    - type: CombineFromComposite
      combine:
        variables:
          # If any of the input variables are unset, the patch will not be
          # applied. Currently, only fromFieldPath is available, and retrieves
          # the value of a source field in the same manner as the
          # FromCompositeFieldPath patch type.
          - fromFieldPath: spec.parameters.location
          - fromFieldPath: metadata.annotations[crossplane.io/claim-name]
        strategy: string
        string:
          # Patch output e.g: us-west-my-sql-server where location = "us-west"
          # and claim-name = "my-sql-server".
          fmt: "%s-%s"
      toFieldPath: spec.forProvider.administratorLogin
      # Combine can also use a patch policy to define whether a missing input
      # variable should return an error, or continue as a no-op.
      policy:
        fromFieldPath: Required

    # Patches can also be applied from the composed resource (MySQLServer)
    # to the composite resource (CompositeMySQLInstance). This MySQLServer
    # will patch the FQDN generated by the provider back to the status
    # subresource of the CompositeMySQLInstance. If a claim is referenced
    # by the composite resource, the claim will also be patched. The
    # "ToCompositeFieldPath" patch may be desirable in cases where a provider
    # generated value is needed by other composed resources. The composite
    # field that is patched back can then be patched forward into other resources.
    - type: ToCompositeFieldPath
      fromFieldPath: "status.atProvider.fullyQualifiedDomainName"
      toFieldPath: "status.address"

    # It is also possible to combine multiple values from the composed resource
    # to the composite resource using the "CombineToComposite" patch type. This
    # type can be configured in the same way as the "CombineFromComposite" patch
    # type above, but its' source and destination resources are switched.
    - type: CombineToComposite
      combine:
        variables:
          # These refer to field paths on the Composed resource. Note that both
          # spec (user input) and status can be combined.
          - fromFieldPath: "spec.parameters.administratorLogin"
          - fromFieldPath: "status.atProvider.fullyQualifiedDomainName"
        strategy: string
        # Here, our administratorLogin parameter and fullyQualifiedDomainName
        # status are formatted to a single output string representing a
        # DSN. This may be useful where other resources need to consume or
        # connect to this resource, but the necessary information is either
        # not exposed in connection details (see below) or the consuming
        # system does not support retrieving connection information from 
        # those details.
        string:
          fmt: "mysql://%s@%s:3306/my-database-name"
      toFieldPath: status.adminDSN
      # Do not report an error when source fields are unset. The
      # fullyQualifiedDomainName status field will not be set until the MySQL
      # server is provisioned, and we do not want to abort rendering of our
      # resources while the field is unset.
      policy:
        fromFieldPath: Optional

    # In addition to a base and patches, this composed MySQLServer declares that
    # it can fulfil the connectionSecretKeys contract required by the definition
    # of the CompositeMySQLInstance. This MySQLServer writes a connection secret
    # with a username, password, and endpoint that may be used to connect to it.
    # These connection details will also be exposed via the composite resource's
    # connection secret. Exactly one composed resource must provide each secret
    # key, but different composed resources may provide different keys.
    connectionDetails:
    - fromConnectionSecretKey: username
    - fromConnectionSecretKey: password
      # The name of the required CompositeMySQLInstance connection secret key
      # can be supplied if it is different from the connection secret key
      # exposed by the MySQLServer.
    - name: hostname
      fromConnectionSecretKey: endpoint
      # In some cases it may be desirable to inject a fixed connection secret
      # value, for example to expose fixed, non-sensitive connection details
      # like standard ports that are not published to the composed resource's
      # connection secret.
    - type: FromValue
      name: port
      value: "3306"
    # Readiness checks allow you to define custom readiness checks. All checks
    # have to return true in order for resource to be considered ready. The
    # default readiness check is to have the "Ready" condition to be "True".
    # Currently Crossplane supports the MatchString, MatchInteger, and None
    # readiness checks.
    readinessChecks:
    - type: MatchString
      fieldPath: "status.atProvider.userVisibleState"
      matchString: "Ready"
    # A CompositeMySQLInstance that uses this Composition will also be composed
    # of an Azure MySQLServerFirewallRule.
  - name: firewallrule
    base:
      apiVersion: database.azure.crossplane.io/v1alpha3
      kind: MySQLServerFirewallRule
      spec:
        forProvider:
          resourceGroupNameSelector:
            matchControllerRef: true
          serverNameSelector:
            matchControllerRef: true
          properties:
            startIpAddress: 10.10.0.0
            endIpAddress: 10.10.255.254
            virtualNetworkSubnetIdSelector:
              name: sample-subnet
    patches:
    - type: PatchSet
      patchSetName: metadata

  # Some composite resources may be "dynamically provisioned" - i.e. provisioned
  # on-demand to satisfy an application's claim for infrastructure. The
  # writeConnectionSecretsToNamespace field configures the default value used
  # when dynamically provisioning a composite resource; it is explained in more
  # detail below.
  writeConnectionSecretsToNamespace: crossplane-system
```

Field paths reference a field within a Kubernetes object via a simple string.
API conventions describe the syntax as "standard JavaScript syntax for accessing
that field, assuming the JSON object was transformed into a JavaScript object,
without the leading dot, such as metadata.name". Array indices are specified via
square braces while object fields may be specified via a period or via square
braces.Kubernetes field paths do not support advanced features of JSON paths,
such as `@`, `$`, or `*`. For example given the below `Pod`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example-pod
  annotations:
    example.org/a: example-annotation
spec:
  containers:
  - name: example-container
    image: example:latest
    command:
    - example
    args:
    - "--debug"
    - "--example"
```

* `metadata.name` would contain "example-pod"
* `metadata.annotations['example.org/a']` would contain "example-annotation"
* `spec.containers[0].name` would contain "example-container"
* `spec.containers[0].args[1]` would contain "--example"

> Note that Compositions provide _intentionally_ limited functionality when
> compared to powerful templating and composition tools like Helm or Kustomize.
> This allows a Composition to be a schemafied Kubernetes-native resource that
> can be stored in and validated by the Kubernetes API server at authoring time
> rather than invocation time.

## Using Composite Resources

![Infrastructure Composition Provisioning]

Crossplane is designed to allow platform builders to expose XRs in several ways:

1. Platform builders can create or manage an XR that does not offer a composite
   resource claim. This XR exists at the cluster scope, which Crossplane
   considers the domain of the platform builder.
1. Platform builders can create an XR of _a kind that offers a claim_ without
   claiming it (i.e. without authoring a claim). This allows their customers to
   claim an existing XR at a future point in time.
1. Platform customers can create a composite resource claim (if the XRD offers
   one), and a composite resource will be provisioned on-demand.

Options one and two are frequently referred to as "static provisioning", while
option three is known as "dynamic provisioning".

> Note that platform builder focused Crossplane concepts are cluster scoped -
> they exist outside any namespace. Crossplane assumes platform builders will
> have similar RBAC permissions to cluster administrators, and will thus be
> permitted to manage cluster scoped resources. Platform customer focused
> Crossplane concepts are namespaced. Crossplane assumes customers will be
> permitted access to the namespace(s) in which their applications run, and not
> to cluster scoped resources.

### Creating and Managing Composite Resources

A platform builder may wish to author a composite resource of a kind that offers
a claim so that a platform customer may later author a claim for that exact
resource. This pattern is useful for resources that may take several minutes to
provision - the platform builder can keep a pool of resources available in
advance in order to ensure claims may be instantly satisfied.

In some cases a platform builder may wish to use Crossplane to model an XR that
they do not wish to allow platform customers to provision. Consider a `VPC` XR
that creates an AWS VPC network with an internet gateway, route table, and
several subnets. Defining this resource as an XR allows the platform builder to
easily reuse their configuration, but it does not make sense to allow platform
customers to create "supporting infrastructure" like a VPC network.

In both of the above scenarios the platform builder may statically provision a
composite resource; i.e. author it directly rather than via its corresponding
resource claim. The `CompositeMySQLInstance` composite resource defined above
could be authored as follows:

```yaml
apiVersion: example.org/v1alpha1
kind: CompositeMySQLInstance
metadata:
  # Composite resources are cluster scoped, so there's no need for a namespace.
  name: example
spec:
  # The schema of the spec.parameters object is defined by the earlier example
  # of an CompositeResourceDefinition. The location, storageGB, and version fields
  # are patched onto the ResourceGroup, MySQLServer, and MySQLServerFirewallRule
  # that this MySQLInstance composes.
  parameters:
    location: au-east
    storageGB: 20
    version: "5.7"
  # Support for a compositionRef is automatically injected into the schema of
  # all defined composite resources. This allows the resource
  # author to explicitly reference a Composition that this composite resource
  # should use - in this case the earlier example-azure Composition. Note that
  # it is also possible to select a composition by labels - see the below
  # MySQLInstance for an example of this approach.
  compositionRef:
    name: example-azure
  # Support for a writeConnectionSecretToRef is automatically injected into the
  # schema of all defined composite resources. This allows the
  # resource to write a connection secret containing any details required to
  # connect to it - in this case the hostname, username, and password. Composite
  # resource authors may omit this reference if they do not need or wish to
  # write these details.
  writeConnectionSecretToRef:
    namespace: infra-secrets
    name: example-mysqlinstance
```

Any updates to the `CompositeMySQLInstance` will be immediately reconciled with
the resources it composes. For example if more storage were needed an update to
the `spec.parameters.storageGB` field would immediately be propagated to the
`spec.forProvider.storageProfile.storageMB` field of the composed `MySQLServer`
due to the relationship established between these two fields by the patches
configured in the `example-azure` `Composition`.

`kubectl describe` may be used to examine a composite resource. Note the `Ready`
condition below. It indicates that all composed resources are indicating they
are 'ready', and therefore the composite resource should be online and ready to
use.

More detail about the health and configuration of the composite resource can be
determined by describing each composed resource. The kinds and names of each
composed resource are exposed as "Resource Refs" - for example `kubectl describe
mysqlserver example-zrpgr` will describe the detailed state of the composed
Azure `MySQLServer`.

```console
$ kubectl describe compositemysqlinstance.example.org

Name:         example
Namespace:
Labels:       crossplane.io/composite=example
Annotations:  <none>
API Version:  example.org/v1alpha1
Kind:         CompositeMySQLInstance
Metadata:
  Creation Timestamp:  2020-05-15T06:53:16Z
  Generation:          4
  Resource Version:    1425809
  UID:                 f654dd52-fe0e-47c8-aa9b-235c77505674
Spec:
  Composition Ref:
    Name:  example-azure
  Parameters:
    Location:      au-east
    Storage GB:    20
    Version:       5.7
  Resource Refs:
    API Version:  azure.crossplane.io/v1alpha3
    Kind:         ResourceGroup
    Name:         example-wspmk
    UID:          4909ab46-95ef-4ba7-8f7a-e1d9ee1a6b23
    API Version:  database.azure.crossplane.io/v1beta1
    Kind:         MySQLServer
    Name:         example-zrpgr
    UID:          3afb903e-32db-4834-a6e7-31249212dca0
    API Version:  database.azure.crossplane.io/v1alpha3
    Kind:         MySQLServerFirewallRule
    Name:         example-h4zjn
    UID:          602c8412-7c33-4338-a3af-78166c17b1a0
  Write Connection Secret To Ref:
    Name:       example-mysqlinstance
    Namespace:  infra-secrets
Status:
  Address:    example.mysql.database.azure.com
  Admin DSN:  mysql://admin@example.mysql.database.azure.com:3306/my-database-name
  Conditions:
    Last Transition Time:  2020-05-15T06:56:46Z
    Reason:                Resource is available for use
    Status:                True
    Type:                  Ready
    Last Transition Time:  2020-05-15T06:53:16Z
    Reason:                Successfully reconciled resource
    Status:                True
    Type:                  Synced
  Connection Details:
    Last Published Time:  2020-05-15T06:53:16Z
Events:
  Type    Reason                   Age                  From                                           Message
  ----    ------                   ----                 ----                                           -------
  Normal  SelectComposition        10s (x7 over 3m40s)  composite/compositemysqlinstances.example.org  Successfully selected composition
  Normal  PublishConnectionSecret  10s (x7 over 3m40s)  composite/compositemysqlinstances.example.org  Successfully published connection details
  Normal  ComposeResources         10s (x7 over 3m40s)  composite/compositemysqlinstances.example.org  Successfully composed resources
```

### Creating a Composite Resource Claim

Composite resource claims represent a need for a particular kind of composite
resource, for example the above `MySQLInstance`. Claims are a proxy for the kind
of resource they claim, allowing platform customers to provision and consume an
XR. An claim may request a pre-existing, statically provisioned XR or it may
dynamically provision one on-demand.

The below claim explicitly requests the `CompositeMySQLInstance` authored in the
previous example:

```yaml
# The MySQLInstance always has the same API group and version as the
# resource it requires. Its kind is always suffixed with .
apiVersion: example.org/v1alpha1
kind: MySQLInstance
metadata:
  # Infrastructure claims are namespaced.
  namespace: default
  name: example
spec:
  # The schema of the spec.parameters object is defined by the earlier example
  # of an CompositeResourceDefinition. The location, storageGB, and version fields
  # are patched onto the ResourceGroup, MySQLServer, and MySQLServerFirewallRule
  # composed by the required MySQLInstance.
  parameters:
    location: au-east
    storageGB: 20
    version: "5.7"
  # Support for a resourceRef is automatically injected into the schema of all
  # resource claims. The resourceRef requests a CompositeMySQLInstance
  # explicitly.
  resourceRef:
    apiVersion: example.org/v1alpha1
    kind: CompositeMySQLInstance
    name: example
  # Support for a writeConnectionSecretToRef is automatically injected into the
  # schema of all published infrastructure claim resources. This allows
  # the resource to write a connection secret containing any details required to
  # connect to it - in this case the hostname, username, and password.
  writeConnectionSecretToRef:
    name: example-mysqlinstance
```

A claim may omit the `resourceRef` and instead include a `compositionRef` (as in
the previous `CompositeMySQLInstance` example) or a `compositionSelector` in
order to trigger dynamic provisioning. A claim that does not include a reference
to an existing composite resource will have a suitable composite resource
provisioned on demand:

```yaml
apiVersion: example.org/v1alpha1
kind: MySQLInstance
metadata:
  namespace: default
  name: example
spec:
  parameters:
    location: au-east
    storageGB: 20
    version: "5.7"
  # Support for a compositionSelector is automatically injected into the schema
  # of all published infrastructure claim resources. This selector selects
  # the example-azure composition by its labels.
  compositionSelector:
    matchLabels:
      purpose: example
      provider: azure
  writeConnectionSecretToRef:
    name: example-mysqlinstance
```

> Note that compositionSelector labels can form a shared language between the
> platform builders who define compositions and their platform customers.
> Compositions could be labelled by zone, size, or purpose in order to allow
> platform customers to request a class of composite resource by describing
> their needs such as "east coast, production".

Like composite resources, claims can be examined using `kubectl describe`. The
`Ready` condition has the same meaning as the `MySQLInstance` above. The
"Resource Ref" indicates the name of the composite resource that was either
explicitly claimed, or in the case of the below claim dynamically provisioned.

```console
$ kubectl describe mysqlinstanceclaim.example.org example

Name:         example
Namespace:    default
Labels:       <none>
Annotations:  crossplane.io/external-name:
API Version:  example.org/v1alpha1
Kind:         MySQLInstance
Metadata:
  Creation Timestamp:  2020-05-15T07:08:11Z
  Finalizers:
    finalizer.apiextensions.crossplane.io
  Generation:        3
  Resource Version:  1428420
  UID:               d87e9580-9d2e-41a7-a198-a39851815840
Spec:
  Composition Selector:
    Match Labels:
      Provider:  azure
      Purpose:   example
  Parameters:
    Location:    au-east
    Storage GB:  20
    Version:     5.7
  Resource Ref:
    API Version:  example.org/v1alpha1
    Kind:         CompositeMySQLInstance
    Name:         default-example-8t4tb
  Write Connection Secret To Ref:
    Name:  example-mysqlinstance
Status:
  Address:    example.mysql.database.azure.com
  Admin DSN:  mysql://admin@example.mysql.database.azure.com:3306/my-database-name
  Conditions:
    Last Transition Time:  2020-05-15T07:26:49Z
    Reason:                Resource is available for use
    Status:                True
    Type:                  Ready
    Last Transition Time:  2020-05-15T07:08:11Z
    Reason:                Successfully reconciled resource
    Status:                True
    Type:                  Synced
  Connection Details:
    Last Published Time:  2020-05-15T07:08:11Z
Events:
  Type    Reason                      Age                    From                                       Message
  ----    ------                      ----                   ----                                       -------
  Normal  ConfigureCompositeResource  8m23s                  claim/compositemysqlinstances.example.org  Successfully configured composite resource
  Normal  BindCompositeResource       8m23s (x7 over 8m23s)  claim/compositemysqlinstances.example.org  Composite resource is not yet ready
  Normal  BindCompositeResource       4m53s (x4 over 23m)    claim/compositemysqlinstances.example.org  Successfully bound composite resource
  Normal  PropagateConnectionSecret   4m53s (x4 over 23m)    claim/compositemysqlinstances.example.org  Successfully propagated connection details from composite resource
```

## Current Limitations

At present the below functionality is planned but not yet implemented:

* Compositions are mutable, and updating a composition causes all composite
  resources that use that composition to be updated accordingly. Revision
  support is planned per issue [#1481].

Refer to the list of [composition related issues] for an up-to-date list of
known issues and proposed improvements.

[Current Limitations]: #current-limitations
[Infrastructure Composition Concepts]: composition-concepts.png
[structural schemas]: https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#specifying-a-structural-schema
[Infrastructure Composition Provisioning]: composition-provisioning.png
[composition related issues]: https://github.com/crossplane/crossplane/labels/composition
[#1481]: https://github.com/crossplane/crossplane/issues/1481

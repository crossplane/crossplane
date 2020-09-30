---
title: Compose Infrastructure
toc: true
weight: 4
indent: true
---

# Compose Infrastructure

In the [last section] we learned that Crossplane can be extended by installing
_providers_, which add support for managed resources. A managed resource is a
Kubernetes custom resource that offers a high fidelity representation of an
infrastructure primitive, like an SQL instance or a firewall rule. Crossplane
goes beyond simply modelling infrastructure primitives as custom resources - it
enables you to define new custom resources with schemas of your choosing. These
resources are composed of infrastructure primitives, allowing you to define and
offer resources that group and abstract infrastructure primitives. We call these
"composite resources" (XRs).

We use two special Crossplane resources to define and configure new XRs:

- A `CompositeResourceDefinition` (XRD) defines a new kind of composite
  resource, including its schema.
- A `Composition` specifies which managed resources a composite resource should
  be composed of, and how they should be configured. You can create multiple
  `Composition` options for each composite resource.

In the examples below, we will define a new `CompositePostgreSQLInstance` XR
that only takes a single `storageGB` parameter, and specifies that it will
create a connection `Secret` with keys for `username`, `password`, and
`endpoint`. We will then create a `Composition` for each provider that can
satisfy a `PostgreSQLInstance`. Let's get started!

## Create CompositeResourceDefinition

The next step is authoring an XRD that defines a `CompositePostgreSQLInstance`:

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances.database.example.org
spec:
  claimNames:
    kind: PostgreSQLInstance
    plural: postgresqlinstances
  connectionSecretKeys:
    - username
    - password
    - endpoint
    - port
  crdSpecTemplate:
    group: database.example.org
    version: v1alpha1
    names:
      kind: CompositePostgreSQLInstance
      plural: compositepostgresqlinstances
    validation:
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

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/definition.yaml
```

You might notice that the above XRD specifies both "names" and "claim names".
This is because the composite resource it defines offers a composite resource
claim (XRC).

Composite resources are always cluster scoped - they exist outside of any
namespace. This allows a composite resource to represent infrastructure that
might be consumed by multiple composite resources across different namespaces.
VPC networks are a common example of this pattern - an infrastructure operator
may wish to define one composite resource that represents a VPC network and
another that represents an SQL instance. The infrastructure operator may wish to
offer the ability to manage SQL instances to their application operators. The
application operators are restricted to their team's namespace, but their SQL
instance should all be attached to the VPC network that the infrastructure
operator manages. Crossplane enables scenarios like this one by allowing the
infrastructure operator to offer their application operators a composite
resource claim.

A composite resource claim is a namespaced proxy for a composite resource. The
schema of the composite resource claim is identical to that of its corresponding
composite resource. In the example above `CompositePostgreSQLInstance` is the
composite resource, while `PostgreSQLInstance` is the composite resource claim.
When an application operator creates a `PostgreSQLinstance` a corresponding
`CompositePostgreSQLInstance` is created. The XRD is said to _define_ the XR,
and to _offer_ the XRC. Offering an XRC is optional; omit `spec.claimNames` to
avoid doing so.

## Create Compositions

Now we'll specify which managed resources our `CompositePostgreSQLInstance` XR
could be composed of, and how they should be configured. For each provider we
will define a `Composition` that can satisfy the XR. In this case, each will
result in the provisioning of a public PostgreSQL instance on the provider.

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-1" data-toggle="tab">AWS (Default VPC)</a></li>
<li><a href="#aws-new-tab-1" data-toggle="tab">AWS (New VPC)</a></li>
<li><a href="#gcp-tab-1" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-1" data-toggle="tab">Azure</a></li>
<li><a href="#alibaba-tab-1" data-toggle="tab">Alibaba</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="aws-tab-1" markdown="1">

> Note that this Composition will create an RDS instance using your default VPC,
> which may or may not allow connections from the internet depending on how it
> is configured. Select the AWS (New VPC) Composition if you wish to create an
> RDS instance that will allow traffic from the internet.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: compositepostgresqlinstances.aws.database.example.org
  labels:
    provider: aws
    guide: quickstart
    vpc: default
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  resources:
    - base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        spec:
          forProvider:
            dbInstanceClass: db.t2.small
            masterUsername: masteruser
            engine: postgres
            engineVersion: "9.6"
            skipFinalSnapshotBeforeDeletion: true
            publiclyAccessible: true
          writeConnectionSecretToRef:
            namespace: crossplane-system
          providerConfigRef:
            name: aws-provider
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-postgresql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.allocatedStorage"
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - fromConnectionSecretKey: port
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/composition-aws.yaml
```

</div>
<div class="tab-pane fade" id="aws-new-tab-1" markdown="1">

> Note: this `Composition` for AWS also includes several networking managed
> resources that are required to provision a publicly available PostgreSQL
> instance. Composition enables scenarios such as this, as well as far more
> complex ones. See the [composition] documentation for more information.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: vpcpostgresqlinstances.aws.database.example.org
  labels:
    provider: aws
    guide: quickstart
    vpc: new
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  resources:
    - base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: VPC
        spec:
          forProvider:
            cidrBlock: 192.168.0.0/16
            enableDnsSupport: true
            enableDnsHostNames: true
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: Subnet
        metadata:
          labels:
            zone: us-west-2a
        spec:
          forProvider:
            cidrBlock: 192.168.64.0/18
            vpcIdSelector:
              matchControllerRef: true
            availabilityZone: us-west-2a
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: Subnet
        metadata:
          labels:
            zone: us-west-2b
        spec:
          forProvider:
            cidrBlock: 192.168.128.0/18
            vpcIdSelector:
              matchControllerRef: true
            availabilityZone: us-west-2b
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: Subnet
        metadata:
          labels:
            zone: us-west-2c
        spec:
          forProvider:
            cidrBlock: 192.168.192.0/18
            vpcIdSelector:
              matchControllerRef: true
            availabilityZone: us-west-2c
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: DBSubnetGroup
        spec:
          forProvider:
            description: An excellent formation of subnetworks.
            subnetIdSelector:
              matchControllerRef: true
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: InternetGateway
        spec:
          forProvider:
            vpcIdSelector:
              matchControllerRef: true
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: ec2.aws.crossplane.io/v1alpha4
        kind: RouteTable
        spec:
          forProvider:
            region: us-west-2
            vpcIdSelector:
              matchControllerRef: true
            routes:
              - destinationCidrBlock: 0.0.0.0/0
                gatewayIdSelector:
                  matchControllerRef: true
            associations:
              - subnetIdSelector:
                  matchLabels:
                    zone: us-west-2a
              - subnetIdSelector:
                  matchLabels:
                    zone: us-west-2b
              - subnetIdSelector:
                  matchLabels:
                    zone: us-west-2c
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: SecurityGroup
        spec:
          forProvider:
            vpcIdSelector:
              matchControllerRef: true
            groupName: crossplane-getting-started
            description: Allow access to PostgreSQL
            ingress:
              - fromPort: 5432
                toPort: 5432
                ipProtocol: tcp
                ipRanges:
                  - cidrIp: 0.0.0.0/0
                    description: Everywhere
          providerConfigRef:
            name: aws-provider
    - base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        spec:
          forProvider:
            dbSubnetGroupNameSelector:
              matchControllerRef: true
            vpcSecurityGroupIDSelector:
              matchControllerRef: true
            dbInstanceClass: db.t2.small
            masterUsername: masteruser
            engine: postgres
            engineVersion: "9.6"
            skipFinalSnapshotBeforeDeletion: true
            publiclyAccessible: true
          writeConnectionSecretToRef:
            namespace: crossplane-system
          providerConfigRef:
            name: aws-provider
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-postgresql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.allocatedStorage"
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - fromConnectionSecretKey: port
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/composition-aws-with-vpc.yaml
```

</div>
<div class="tab-pane fade" id="gcp-tab-1" markdown="1">

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: compositepostgresqlinstances.gcp.database.example.org
  labels:
    provider: gcp
    guide: quickstart
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  resources:
    - base:
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
          writeConnectionSecretToRef:
            namespace: crossplane-system
          providerConfigRef:
            name: gcp-provider
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-postgresql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.settings.dataDiskSizeGb"
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - name: port
          value: "5432"
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/composition-gcp.yaml
```

</div>
<div class="tab-pane fade" id="azure-tab-1" markdown="1">

> Note: the `Composition` for Azure also includes a `ResourceGroup` and
> `PostgreSQLServerFirewallRule` that are required to provision a publicly
> available PostgreSQL instance on Azure. Composition enables scenarios such as
> this, as well as far more complex ones. See the [composition] documentation
> for more information.

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: compositepostgresqlinstances.azure.database.example.org
  labels:
    provider: azure
    guide: quickstart
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  resources:
    - base:
        apiVersion: azure.crossplane.io/v1alpha3
        kind: ResourceGroup
        spec:
          location: West US 2
          providerConfigRef:
            name: azure-provider
    - base:
        apiVersion: database.azure.crossplane.io/v1beta1
        kind: PostgreSQLServer
        spec:
          forProvider:
            administratorLogin: myadmin
            resourceGroupNameSelector:
              matchControllerRef: true
            location: West US 2
            sslEnforcement: Disabled
            version: "9.6"
            sku:
              tier: GeneralPurpose
              capacity: 2
              family: Gen5
          writeConnectionSecretToRef:
            namespace: crossplane-system
          providerConfigRef:
            name: azure-provider
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-postgresql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.storageProfile.storageMB"
          transforms:
            - type: math
              math:
                multiply: 1024
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - name: port
          value: "5432"
    - base:
        apiVersion: database.azure.crossplane.io/v1alpha3
        kind: PostgreSQLServerFirewallRule
        spec:
          forProvider:
            serverNameSelector:
              matchControllerRef: true
            resourceGroupNameSelector:
              matchControllerRef: true
            properties:
              startIpAddress: 0.0.0.0
              endIpAddress: 255.255.255.254
          providerConfigRef:
            name: azure-provider
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/composition-azure.yaml
```

</div>
<div class="tab-pane fade" id="alibaba-tab-1" markdown="1">

```yaml
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: compositepostgresqlinstances.alibaba.database.example.org
  labels:
    provider: alibaba
    guide: quickstart
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  compositeTypeRef:
    apiVersion: database.example.org/v1alpha1
    kind: CompositePostgreSQLInstance
  resources:
    - base:
        apiVersion: database.alibaba.crossplane.io/v1alpha1
        kind: RDSInstance
        spec:
          forProvider:
            engine: PostgreSQL
            engineVersion: "9.4"
            dbInstanceClass: rds.pg.s1.small
            securityIPList: "0.0.0.0/0"
            masterUsername: "myuser"
          writeConnectionSecretToRef:
            namespace: crossplane-system
          providerConfigRef:
            name: alibaba-provider
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-postgresql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.dbInstanceStorageInGB"
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - fromConnectionSecretKey: port
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/composition-alibaba.yaml
```

</div>
</div>

## Create a Claim

We have now:

- Defined a `CompositePostgreSQLInstance` composite resource.
- Offered a `PostgreSQLInstance` composite resource claim.
- Created at least one `Composition` that can satisfy our composite resource.

This means we have everything we need to create a `PostgreSQLInstance` in the
namespace of our choosing. Each `Composition` we created was labelled
`provider: <name-of-provider>` label. This lets us use a `compositionSelector`
to match our `Composition` of choice.

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-2" data-toggle="tab">AWS (Default VPC)</a></li>
<li><a href="#aws-tab-new-2" data-toggle="tab">AWS (New VPC)</a></li>
<li><a href="#gcp-tab-2" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-2" data-toggle="tab">Azure</a></li>
<li><a href="#alibaba-tab-2" data-toggle="tab">Alibaba</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="aws-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: aws
      vpc: default
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/claim-aws.yaml
```

</div>
<div class="tab-pane fade" id="aws-tab-new-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: aws
      vpc: new
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/claim-aws.yaml
```

</div>
<div class="tab-pane fade" id="gcp-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: gcp
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/claim-gcp.yaml
```

</div>
<div class="tab-pane fade" id="azure-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: azure
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/claim-azure.yaml
```

</div>
<div class="tab-pane fade" id="alibaba-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: alibaba
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/claim-alibaba.yaml
```

</div>
</div>

After creating the `PostgreSQLInstance` Crossplane will provision a
database instance on your provider of choice. Once provisioning is complete, you
should see `READY: True` in the output when you run:

```console
kubectl get postgresqlinstances.database.example.org my-db
```

> Note: while waiting for the `PostgreSQLInstance` to become ready, you
> may want to look at other resources in your cluster. The following commands
> will allow you to view groups of Crossplane resources:
>
> - `kubectl get managed`: get all resources that represent a unit of external
>   infrastructure
> - `kubectl get <name-of-provider>`: get all resources related to `<provider>`
> - `kubectl get crossplane`: get all resources related to Crossplane

You should also see a `Secret` in the `default` namespace named `db-conn` that
contains fields for `username`, `password`, and `endpoint`:

```console
kubectl get secrets db-conn
```

## Consume Infrastructure

Because connection secrets are written as a Kubernetes `Secret` they can easily
be consumed by Kubernetes primitives. The most basic building block in
Kubernetes is the `Pod`. Let's define a `Pod` that will show that we are able to
connect to our newly provisioned database.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: see-db
  namespace: default
spec:
  containers:
  - name: see-db
    image: postgres:9.6
    command: ['psql']
    args: ['-c', 'SELECT current_database();']
    env:
    - name: PGDATABASE
      value: postgres
    - name: PGHOST
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: endpoint
    - name: PGUSER
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: username
    - name: PGPASSWORD
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: password
    - name: PGPORT
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: port
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/compose/pod.yaml
```

This `Pod` simply connects to a PostgreSQL database and prints its name, so you
should see the following output (or similar) after creating it if you run
`kubectl logs see-db`:

```SQL
 current_database
------------------
 postgres
(1 row)
```

## Clean Up

To clean up the infrastructure that was provisioned, you can delete the
`PostgreSQLInstance`:

```console
kubectl delete postgresqlinstances.database.example.org my-db
```

To clean up the `Pod`, run:

```console
kubectl delete pod see-db
```

> Don't clean up your `CompositeResourceDefinition` or `Composition` just yet if
> you plan to continue on to the next section of the guide! We'll use them again
> when we deploy an OAM application.

## Next Steps

Now you have seen how to provision and publish more complex infrastructure
setups. In the [next section] you will learn how to consume infrastructure
alongside your [OAM] application manifests.

<!-- Named Links -->

[last section]: provision-infrastructure.md
[composition]: ../introduction/composition.md
[next section]: run-applications.md
[OAM]: https://oam.dev/

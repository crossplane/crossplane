---
title: Create a Configuration
toc: true
weight: 4
indent: true
---

# Create a Configuration

In the [previous section] we were able to create a PostgreSQL database because
we had installed a configuration package that defined the `PostgreSQLInstance`
type and a `Composition` of managed resources that mapped to it. Crossplane
allows you to define your own composite resources (XRs) and compositions, then
package them up to be easily distributed as OCI images. This allows you to
construct a reproducible platform that exposes infrastructure APIs at your
desired level of abstraction, and can be installed into any Crossplane cluster.

## Create a Configuration Directory

We are going to build the same configuration package that we previously
installed. It will consist of three files:

* `crossplane.yaml` - Metadata about the configuration.
* `definition.yaml` - The XRD.
* `composition.yaml` - The Composition.

Crossplane can create a configuration from any directory with a valid
`crossplane.yaml` metadata file at its root, and one or more XRDs or
Compositions. The directory structure does not matter, as long as the
`crossplane.yaml` file is at the root. Note that a configuration need not
contain one XRD and one composition - it could include only an XRD, only a
composition, several compositions, or any combination thereof.

Before we go any further, we must create a directory in which to build our
configuration:

```console
mkdir crossplane-config
cd crossplane-config
```

We'll create the aforementioned three files in this directory, then build them
into a package.

> Note that `definition.yaml` and `composition.yaml` could be created directly
> in the Crossplane cluster without packaging them into a configuration. This
> can be useful for testing compositions before pushing them to a registry.

## Create CompositeResourceDefinition

First we'll create a `CompositeResourceDefinition` (XRD) to define the schema of
our `CompositePostgreSQLInstance` and its `PostgreSQLInstance` resource claim.

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances.database.example.org
spec:
  group: database.example.org
  names:
    kind: CompositePostgreSQLInstance
    plural: compositepostgresqlinstances
  claimNames:
    kind: PostgreSQLInstance
    plural: postgresqlinstances
  connectionSecretKeys:
    - username
    - password
    - endpoint
    - port
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

```console
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/definition.yaml
```

> You might notice that the XRD we created specifies both "names" and "claim
> names". This is because the composite resource it defines offers a composite
> resource claim (XRC).

## Create Compositions

Now we'll specify which managed resources our `CompositePostgreSQLInstance` XR
and its claim could be composed of, and how they should be configured. We do
this by defining a `Composition` that can satisfy the XR we defined above. In
this case, our `Composition` will specify how to provision a public PostgreSQL
instance on the chosen provider.

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-2" data-toggle="tab">AWS (Default VPC)</a></li>
<li><a href="#aws-new-tab-2" data-toggle="tab">AWS (New VPC)</a></li>
<li><a href="#gcp-tab-2" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-2" data-toggle="tab">Azure</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="aws-tab-2" markdown="1">

> Note that this Composition will create an RDS instance using your default VPC,
> which may or may not allow connections from the internet depending on how it
> is configured. Select the AWS (New VPC) Composition if you wish to create an
> RDS instance that will allow traffic from the internet.

```yaml
apiVersion: apiextensions.crossplane.io/v1
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
    - name: rdsinstance
      base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        spec:
          forProvider:
            region: us-east-1
            dbInstanceClass: db.t2.small
            masterUsername: masteruser
            engine: postgres
            engineVersion: "12"
            skipFinalSnapshotBeforeDeletion: true
            publiclyAccessible: true
          writeConnectionSecretToRef:
            namespace: crossplane-system
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
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/aws/composition.yaml
```

</div>
<div class="tab-pane fade" id="aws-new-tab-2" markdown="1">

> Note: this `Composition` for AWS also includes several networking managed
> resources that are required to provision a publicly available PostgreSQL
> instance. Composition enables scenarios such as this, as well as far more
> complex ones. See the [composition] documentation for more information.

```yaml
apiVersion: apiextensions.crossplane.io/v1
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
    - name: vpc
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: VPC
        spec:
          forProvider:
            region: us-east-1
            cidrBlock: 192.168.0.0/16
            enableDnsSupport: true
            enableDnsHostNames: true
    - name: subnet-a
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: Subnet
        metadata:
          labels:
            zone: us-east-1a
        spec:
          forProvider:
            region: us-east-1
            cidrBlock: 192.168.64.0/18
            vpcIdSelector:
              matchControllerRef: true
            availabilityZone: us-east-1a
    - name: subnet-b
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: Subnet
        metadata:
          labels:
            zone: us-east-1b
        spec:
          forProvider:
            region: us-east-1
            cidrBlock: 192.168.128.0/18
            vpcIdSelector:
              matchControllerRef: true
            availabilityZone: us-east-1b
    - name: subnet-c
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: Subnet
        metadata:
          labels:
            zone: us-east-1c
        spec:
          forProvider:
            region: us-east-1
            cidrBlock: 192.168.192.0/18
            vpcIdSelector:
              matchControllerRef: true
            availabilityZone: us-east-1c
    - name: dbsubnetgroup
      base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: DBSubnetGroup
        spec:
          forProvider:
            region: us-east-1
            description: An excellent formation of subnetworks.
            subnetIdSelector:
              matchControllerRef: true
    - name: internetgateway
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: InternetGateway
        spec:
          forProvider:
            region: us-east-1
            vpcIdSelector:
              matchControllerRef: true
    - name: routetable
      base:
        apiVersion: ec2.aws.crossplane.io/v1alpha4
        kind: RouteTable
        spec:
          forProvider:
            region: us-east-1
            vpcIdSelector:
              matchControllerRef: true
            routes:
              - destinationCidrBlock: 0.0.0.0/0
                gatewayIdSelector:
                  matchControllerRef: true
            associations:
              - subnetIdSelector:
                  matchLabels:
                    zone: us-east-1a
              - subnetIdSelector:
                  matchLabels:
                    zone: us-east-1b
              - subnetIdSelector:
                  matchLabels:
                    zone: us-east-1c
    - name: securitygroup
      base:
        apiVersion: ec2.aws.crossplane.io/v1beta1
        kind: SecurityGroup
        spec:
          forProvider:
            region: us-east-1
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
    - name: rdsinstance
      base:
        apiVersion: database.aws.crossplane.io/v1beta1
        kind: RDSInstance
        spec:
          forProvider:
            region: us-east-1
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
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/aws-with-vpc/composition.yaml
```

</div>
<div class="tab-pane fade" id="gcp-tab-2" markdown="1">

```yaml
apiVersion: apiextensions.crossplane.io/v1
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
          writeConnectionSecretToRef:
            namespace: crossplane-system
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
        - type: FromValue
          name: port
          value: "5432"
```

```console
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/gcp/composition.yaml
```

</div>
<div class="tab-pane fade" id="azure-tab-2" markdown="1">

> Note: the `Composition` for Azure also includes a `ResourceGroup` and
> `PostgreSQLServerFirewallRule` that are required to provision a publicly
> available PostgreSQL instance on Azure. Composition enables scenarios such as
> this, as well as far more complex ones. See the [composition] documentation
> for more information.

```yaml
apiVersion: apiextensions.crossplane.io/v1
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
    - name: resourcegroup
      base:
        apiVersion: azure.crossplane.io/v1alpha3
        kind: ResourceGroup
        spec:
          location: West US 2
    - name: postgresqlserver
      base:
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
        - type: FromValue
          name: port
          value: "5432"
    - name: firewallrule
      base:
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
```

```console
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/azure/composition.yaml
```

</div>
<div class="tab-pane fade" id="alibaba-tab-2" markdown="1">

```yaml
apiVersion: apiextensions.crossplane.io/v1
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
    - name: rdsinstance
      base:
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
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/alibaba/composition.yaml
```

</div>
</div>

## Build and Push The Configuration

Finally, we'll author our metadata file then build and push our configuration
so that Crossplane users may install it.

> Note that Crossplane pushes packages to an OCI registry - currently [Docker
> Hub] by default. You may need to run `docker login` before you are able to
> push a package.

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-3" data-toggle="tab">AWS (Default VPC)</a></li>
<li><a href="#aws-new-tab-3" data-toggle="tab">AWS (New VPC)</a></li>
<li><a href="#gcp-tab-3" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-3" data-toggle="tab">Azure</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="aws-tab-3" markdown="1">

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: getting-started-with-aws
  annotations:
    guide: quickstart
    provider: aws
    vpc: default
spec:
  crossplane:
    version: ">=v1.0.0-0"
  dependsOn:
    - provider: crossplane/provider-aws
      version: ">=v0.14.0"
```

```console
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/aws/crossplane.yaml

kubectl crossplane build configuration
```

You should see a file in your working directory with a `.xpkg` extension. The
Crossplane CLI will automatically tag and push it to the registry of your
choosing in the next step if it is the only `.xpkg` in the directory. Otherwise
you may specify a specific package by using the `-f` flag.

```console
# Set this to the Docker Hub username or OCI registry you wish to use.
REG=my-package-repo
kubectl crossplane push configuration ${REG}/getting-started-with-aws:master
```

> Note that the Crossplane CLI will not follow symbolic links for files in the
> root package directory.

</div>
<div class="tab-pane fade" id="aws-new-tab-3" markdown="1">

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: getting-started-with-aws-with-vpc
  annotations:
    guide: quickstart
    provider: aws
    vpc: new
spec:
  crossplane:
    version: ">=v1.0.0-0"
  dependsOn:
    - provider: crossplane/provider-aws
      version: "v0.16.0"
```

```console
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/aws-with-vpc/crossplane.yaml

kubectl crossplane build configuration
```

You should see a file in your working directory with a `.xpkg` extension. The
Crossplane CLI will automatically tag and push it to the registry of your
choosing in the next step if it is the only `.xpkg` in the directory. Otherwise
you may specify a specific package by using the `-f` flag.

```console
# Set this to the Docker Hub username or OCI registry you wish to use.
REG=my-package-repo
kubectl crossplane push configuration ${REG}/getting-started-with-aws-with-vpc:master
```

> Note that the Crossplane CLI will not follow symbolic links for files in the
> root package directory.

</div>
<div class="tab-pane fade" id="gcp-tab-3" markdown="1">

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: getting-started-with-gcp
  annotations:
    guide: quickstart
    provider: gcp
spec:
  crossplane:
    version: ">=v1.0.0-0"
  dependsOn:
    - provider: crossplane/provider-gcp
      version: ">=v0.13.0"
```

```console
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/gcp/crossplane.yaml

kubectl crossplane build configuration
```

You should see a file in your working directory with a `.xpkg` extension. The
Crossplane CLI will automatically tag and push it to the registry of your
choosing in the next step if it is the only `.xpkg` in the directory. Otherwise
you may specify a specific package by using the `-f` flag.

```console
# Set this to the Docker Hub username or OCI registry you wish to use.
REG=my-package-repo
kubectl crossplane push configuration ${REG}/getting-started-with-gcp:master
```

> Note that the Crossplane CLI will not follow symbolic links for files in the
> root package directory.

</div>
<div class="tab-pane fade" id="azure-tab-3" markdown="1">

```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: getting-started-with-azure
  annotations:
    guide: quickstart
    provider: azure
spec:
  crossplane:
    version: ">=v1.0.0-0"
  dependsOn:
    - provider: crossplane/provider-azure
      version: ">=v0.13.0"
```

```console
curl -OL https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/package/azure/crossplane.yaml

kubectl crossplane build configuration
```

You should see a file in your working directory with a `.xpkg` extension. The
Crossplane CLI will automatically tag and push it to the registry of your
choosing in the next step if it is the only `.xpkg` in the directory. Otherwise
you may specify a specific package by using the `-f` flag.

```console
# Set this to the Docker Hub username or OCI registry you wish to use.
REG=my-package-repo
kubectl crossplane push configuration ${REG}/getting-started-with-azure:master
```

> Note that the Crossplane CLI will not follow symbolic links for files in the
> root package directory.

</div>
</div>

That's it! You've now built and pushed your package. Take a look at the
Crossplane [packages] documentation for more information about installing and
working with packages, or read about other Crossplane [concepts].

## Clean Up

To clean up, you can simply delete your package directory:

```console
cd ..
rm -rf crossplane-config
```

<!-- Named Links -->

[previous section]: provision-infrastructure.md
[composed]: ../concepts/composition.md
[composition]: ../concepts/composition.md
[Docker Hub]: https://hub.docker.com/
[packages]: ../concepts/packages.md
[concepts]: ../concepts/overview.md

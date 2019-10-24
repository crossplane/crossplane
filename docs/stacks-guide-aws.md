---
title: "Stacks Guide: AWS Setup"
toc: true
weight: 530
indent: true
---

# Stacks Guide: AWS Setup

## Table of Contents

  1. [Introduction](#introduction)
  1. [Install the AWS stack](#install-the-aws-stack)
  1. [Configure the AWS account](#configure-the-aws-account)
  1. [Configure Crossplane Provider for AWS](#configure-crossplane-provider-for-aws)
  1. [Set Up Network Configuration](#set-up-network-configuration)
  1. [Configure Cloud-Specific Resource Classes](#configure-cloud-specific-resource-classes)
  1. [Recap](#recap)
  1. [Next Steps](#next-steps)

## Introduction

in this guide, we will set up an aws provider in crossplane so that we can
install and use the [wordpress sample stack][sample-wordpress-stack], which
depends on mysql and kubernetes!

Before we begin, you will need:

* Everything from the [Crossplane Stacks Guide][stacks-guide] before the
  cloud provider setup
  - A `kubectl v1.15+` pointing to a Crossplane cluster
  - The [Crossplane CLI][crossplane-cli] installed
* An account on [AWS][aws]
* The [aws cli][installed]

At the end, we will have:

* A Crossplane cluster configured to use AWS
* A typical AWS network configured to support secure connectivity between
  resources
* Support in Crossplane cluster for satisfying MySQL and Kubernetes
  claims
* A slightly better understanding of:
  * The way AWS is configured in Crossplane
  * The way dependencies for cloud-portable workloads are configured in
    Crossplane

## Install the AWS stack

After Crossplane has been installed, it can be extended with more
functionality by installing a [Crossplane Stack][stack-docs]! Let's
install the [stack for Amazon Web Services][stack-aws] (AWS) to add
support for that cloud provider.

The namespace where we install the stack, is also the one that our managed AWS
resources will reside. The name of this namespace is arbitrary, and we are
calling it "`infra-aws`" in this guide. Let's create it:

```bash
# namespace for aws stack and infra resources
kubectl create namespace infra-aws
```

Now we install the AWS stack using Crossplane CLI. Since this is an
infrastructure stack, we need to specify that it's cluster-scoped by
passing the `--cluster` flag.

```bash
kubectl crossplane stack generate-install --cluster 'crossplane/stack-aws:master' stack-aws | kubectl apply --namespace infra-aws -f -
```

The rest of the steps assume that you installed the AWS stack into the
`infra-aws` namespace.

## Configure the AWS account

An [aws user][] with `Administrative` privileges is needed to enable Crossplane to
create the required resources. Once the user is provisioned, an [Access Key][]
needs to be created so the user can have API access.

Using the set of access key credentials for the user with the right
access, we will to have [`aws` command line tool][] [installed][], and
then we will need to [configure it][aws-cli-configure].

When the aws cli is configured, the credentials and configuration will
be in `~/.aws/credentials` and `~/.aws/config` respectively. These will
be consumed in the next step.

When configuring the aws cli, it is recommended that the user credentials are
configured under a specific [aws named profile][], and not under
`default`. In this guide, we assume that the credentials are configured
under the `crossplane-user` profile, but you can use a different profile name (including `default`)
if you want. Let's store the profile name in a variable so we can
use it in later steps:

```bash
aws_profile=crossplane-user
```

## Configure Crossplane Provider for AWS

Crossplane uses the aws user credentials that were configured in the previous
step to create resources in AWS. These credentials will be stored as a
[secret][] in Kubernetes, and will be used by an [aws
provider][aws-provider-docs] instance. The AWS region is also pulled
from the cli configuration, so that the aws provider can target a
specific region.

To store the credentials as a secret, run:

```bash
# retrieve profile's credentials, save it under 'default' profile, and base64 encode it
AWS_CREDS_BASE64=$(echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $aws_profile)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $aws_profile)" | base64  | tr -d "\n")
# retrieve the profile's region from config
AWS_REGION=$(aws configure get region --profile ${aws_profile})
```

At this point, the region and the encoded credentials are stored in respective
 variables. Next, we'll need to create an instance of [`aws provider`][]:

```bash
cat > provider.yaml <<EOF
---
apiVersion: v1
data:
  credentials: ${AWS_CREDS_BASE64}
kind: Secret
metadata:
  name: aws-user-creds
  namespace: infra-aws
type: Opaque
---
apiVersion: aws.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: aws-provider
  namespace: infra-aws
spec:
  credentialsSecretRef:
    key: credentials
    name: aws-user-creds
  region: ${AWS_REGION}
EOF

# apply it to the cluster:
kubectl apply -f "provider.yaml"

# delete the credentials environment variable
unset AWS_CREDS_BASE64
```

The output will look like the following:

```bash
secret/aws-user-creds created
provider.aws.crossplane.io/aws-provider created
```

## Set Up Network Configuration

In this section we build a simple AWS network configuration, by creating
corresponding Crossplane managed resources in the `infra-aws` namespace that
we created earlier. This network configuration enables resources in WordPress
stack to communicate securely.
In this guide, we will use the [sample AWS network configuration] in Crossplane repository.

### TL;DR

Apply the sample network configuration resources:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/gitops/aws-infra?ref=v0.4.0
```

And you're done! You can check the status of the provisioning by running:

```bash
kubectl get -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/gitops/aws-infra?ref=v0.4.0
```

When all resources has the `Ready` condition in `True` state, the provisioning is completed. You can now move to the next section, or keep reading below for more details about the managed resources that we created.

### Behind the scenes

When configured in AWS, WordPress resources map to an EKS cluster and an RDS
database. In order to make the RDS instance accessible from the EKS cluster,
they both need to live within the same VPC. However, a VPC is not the only AWS
resource that needs to be created to enable inter-resource connectivity. In
general, a **Network Configuration** which consists of a set of VPCs, Subnets,
Security Groups, Route Tables, IAM Roles and other resources, is required for
this purpose. For more information, see [AWS resource connectivity] design
document.

To inspect the resources that we created above, let's run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/gitops/aws-infra?ref=v0.4.0 > network-config.yaml
```

This will save the sample network configuration resources locally in
`network-config..yaml`. Please note that AWS parameters that are used in these resources (like `cidrBlock`, `region`, etc...) are arbitrarily chosen in this solution and could be configured to implement other [configurations][eks-user-guide].

Below we inspect each of these resources in more details.
* **`VPC`** Represents an AWS [Virtual Private Network][] (VPC).

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha2
  kind: VPC
  metadata:
    name: sample-vpc
    namespace: infra-aws
  spec:
    cidrBlock: 192.168.0.0/16
    enableDnsSupport: true
    enableDnsHostNames: true
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```

* **`Subnet`** Represents an AWS [Subnet][]. For this configuration we create
  one Subnet per each availability zone in the region.

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha2
  kind: Subnet
  metadata:
    name: sample-subnet1
    namespace: infra-aws
  spec:
    cidrBlock: 192.168.64.0/18
    vpcIdRef:
      name: sample-vpc
    availabilityZone: us-west-2a
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ---
  apiVersion: network.aws.crossplane.io/v1alpha2
  kind: Subnet
  metadata:
    name: sample-subnet2
    namespace: infra-aws
  spec:
    cidrBlock: 192.168.128.0/18
    vpcIdRef:
      name: sample-vpc
    availabilityZone: us-west-2b
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ---
  apiVersion: network.aws.crossplane.io/v1alpha2
  kind: Subnet
  metadata:
    name: sample-subnet3
    namespace: infra-aws
  spec:
    cidrBlock: 192.168.192.0/18
    vpcIdRef:
      name: sample-vpc
    availabilityZone: us-west-2c
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```

* **`InternetGateway`** Represents an AWS [Internet Gateway][] which allows
  the resources in the VPC to have access to the Internet. Since the
  WordPress application will be accessed from the internet, this resource is
  required in the network configuration.

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha2
  kind: InternetGateway
  metadata:
    name: sample-internetgateway
    namespace: infra-aws
  spec:
    vpcIdRef:
      name: sample-vpc
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```

* **`RouteTable`** Represents an AWS [Route Table][], which specifies rules to
  direct traffic in a virtual network. We use a Route Table to redirect internet
  traffic from all Subnets to the Internet Gateway instance.

    ```yaml
    ---
    apiVersion: network.aws.crossplane.io/v1alpha2
    kind: RouteTable
    metadata:
      name: sample-routetable
      namespace: infra-aws
    spec:
      vpcIdRef:
        name: sample-vpc
      routes:
        - destinationCidrBlock: 0.0.0.0/0
          gatewayIdRef:
            name: sample-internetgateway
      associations:
        - subnetIdRef: 
            name: sample-subnet1
        - subnetIdRef:
            name: sample-subnet2
        - subnetIdRef:
            name: sample-subnet3
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: infra-aws
    ```

* **`SecurityGroup`** Represents an AWS [Security Group][], which controls
  inbound and outbound traffic to EC2 instances.

  We need two security groups in this configuration:

  * A security group to assign it later to the EKS cluster workers, so they have
 the right permissions to communicate with each API server

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha2
  kind: SecurityGroup
  metadata:
    name: sample-cluster-sg
    namespace: infra-aws
  spec:
    vpcIdRef:
      name: sample-vpc
    groupName: sample-ekscluster-sg
    description: Cluster communication with worker nodes
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```

* A security group to assign it later to the RDS database instance, which
  allows the instance to accept traffic from worker nodes.

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha2
  kind: SecurityGroup
  metadata:
    name: sample-rds-sg
    namespace: infra-aws
  spec:
    vpcIdRef:
      name: sample-vpc
    groupName: sample-rds-sg
    description: open rds access to crossplane workload
    reclaimPolicy: Delete
    ingress:
      - fromPort: 3306
        toPort: 3306
        protocol: tcp
        cidrBlocks:
          - cidrIp: 0.0.0.0/0
            description: all ips
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```

* **`DBSubnetGroup`** Represents an AWS [Database Subnet Group][], which
  creates a group of Subnets that can communicate with the RDS database
  instance that we will create later.

  ```yaml
  ---
  apiVersion: storage.aws.crossplane.io/v1alpha2
  kind: DBSubnetGroup
  metadata:
    name: sample-dbsubnetgroup
    namespace: infra-aws
  spec:
    groupName: sample_dbsubnetgroup
    description: EKS vpc to rds
    subnetIdRefs:
      - name: sample-subnet1
      - name: sample-subnet2
      - name: sample-subnet3
    tags:
      - key: name
        value: sample-dbsubnetgroup
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```

* **`IAMRole`** Represents An AWS [IAM Role][], which assigns a set of access policies to the
  AWS principal that assumes it. We create a role to later add needed policies and assign it to the
  cluster, granting the permissions it needs to communicate with other resources
  in AWS.

  ```yaml
  ---
  apiVersion: identity.aws.crossplane.io/v1alpha2
  kind: IAMRole
  metadata:
    name: sample-eks-cluster-role
    namespace: infra-aws
  spec:
    roleName: sample-eks-cluster-role
    description: a role that gives a cool power
    assumeRolePolicyDocument: |
      {
        "Version": "2012-10-17",
        "Statement": [
          {
            "Effect": "Allow",
            "Principal": {
              "Service": "eks.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
          }
        ]
      }
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```


* **`IAMRolePolicyAttachment`** Represents an AWS [IAM Role Policy][], which
  defines a certain permission in an IAM Role. We need two policies to create
  and assign it to the IAM Role above, so the cluster to communicate with
  other aws resources.

  ```yaml
  ---
  apiVersion: identity.aws.crossplane.io/v1alpha2
  kind: IAMRolePolicyAttachment
  metadata:
    name: sample-role-servicepolicy
    namespace: infra-aws
  spec:
    roleNameRef:
      name: sample-eks-cluster-role
    # wellknown policy arn
    policyArn: arn:aws:iam::aws:policy/AmazonEKSServicePolicy
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ---
  apiVersion: identity.aws.crossplane.io/v1alpha2
  kind: IAMRolePolicyAttachment
  metadata:
    name: sample-role-clusterpolicy
    namespace: infra-aws
  spec:
    roleNameRef:
      name: sample-eks-cluster-role
    # wellknown policy arn
    policyArn: arn:aws:iam::aws:policy/AmazonEKSClusterPolicy
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
      namespace: infra-aws
  ```

As you probably have noticed, some resources are referencing other resource attributes in their YAML representations. For instance in `Subnet` YAML we have:

```yaml
...
    vpcIdRef:
      name: sample-vpc
...
```

Such cross resource referencing is a Crossplane feature that enables managed resources to retrieve other resources attributes. This creates a *blocking dependency*, avoiding the dependent resource to be created before the referred resource is ready. In the example above, `Subnet` will be blocked until the referred `VPC` is created, and then it retrieves its `vpcId`. For more information, see [Cross Resource Referencing][].

## Configure Cloud-Specific Resource Classes

Once we have the network configuration set up, we need to tell Crossplane how to
satisfy WordPress's claims for a database and a Kubernetes cluster, using AWS
[Resource classes][resource-classes-docs]. These resources serve as templates to
satisfy cloud-agnostic resource claims of WordPress stack. In this guide, we
will use the [sample AWS resource classes] in Crossplane repository.

### TL;DR

Apply the sample sample AWS resource classes:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/gitops/aws-resource-classes?ref=v0.4.0
```

And you're done! Note that these resources do not immediately provision external AWS resourcs.

### More Details

To inspect the resource classes that we created above, run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/gitops/aws-infra?ref=v0.4.0 > resource-classes.yaml
```

This will save the sample resource classes YAML locally in
`resource-classes.yaml`. As mentioned above, these resource classes serve as templates and could be configured depending on the specific needs that are needed from the underlying resources. For instance, in the sample resources the `RDSInstanceClass` has `size: 20`, which will result in RDS databases of size 20 once a claim is submitted for this class. In addition, it's possible to have multiple classes defined for the same claim kind, but our sample has defined only one class for each resource type.

Below we inspect each of these resource classes in more details:

* **`RDSInstanceClass`** Represents a resource that serves as a template to create a [RDS Database Instance][].

  ```yaml
  ---
  apiVersion: database.aws.crossplane.io/v1alpha2
  kind: RDSInstanceClass
  metadata:
    name: standard-mysql
    namespace: infra-aws
  specTemplate:
    class: db.t2.small
    masterUsername: masteruser
    securityGroupRefs:
      - name: sample-rds-sg
    subnetGroupNamRef:
      name: sample-dbsubnetgroup
    size: 20
    engine: mysql
    providerRef:
      name: aws-provider
      namespace: infra-aws
    reclaimPolicy: Delete
  ```

* **`EKSClusterClass`** Represents a resource that serves as a template to create a [EKS Cluster][].

  ```yaml
  ---
  apiVersion: compute.aws.crossplane.io/v1alpha2
  kind: EKSClusterClass
  metadata:
    name: standard-cluster
    namespace: infra-aws
  specTemplate:
    region: us-west-2
    roleARNRef:
      name: sample-eks-cluster-role
    vpcIdRef:
      name: sample-vpc
    subnetIdRefs:
      - name: sample-subnet1
      - name: sample-subnet2
      - name: sample-subnet3
    securityGroupIdRefs:
      - name: sample-cluster-sg
    workerNodes:
      nodeInstanceType: m3.medium
      nodeAutoScalingGroupMinSize: 1
      nodeAutoScalingGroupMaxSize: 1
      nodeGroupName: demo-nodes
      clusterControlPlaneSecurityGroupRef:
        - name: sample-cluster-sg
    providerRef:
      name: aws-provider
      namespace: infra-aws
    reclaimPolicy: Delete
  ```

For more details about resource claims and how they work, see the [documentation
on resource claims][resource-claims-docs].

## Recap

To recap what we've set up now in our environment:

* A Crossplane Provider resource for AWS
* A Network Configuration to have secure connectivity between resources
* An EKSClusterClass and an RDSInstanceClass with the right configuration to use
  the mentioned networking setup.

## Next Steps

Next we'll set up a Crossplane App Stack and use it! Head [back over to the
Stacks Guide document][stacks-guide-continue] so we can pick up where we left
off.

<!-- Links -->
[stacks-guide]: stacks-guide.md

[aws]: https://aws.amazon.com
[stack-aws]: https://github.com/crossplaneio/stack-aws
[sample-wordpress-stack]: https://github.com/crossplaneio/sample-stack-wordpress

[stack-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks

[aws user]: https://docs.aws.amazon.com/mediapackage/latest/ug/setting-up-create-iam-user.html
[Access Key]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html

[`aws provider`]: https://github.com/crossplaneio/stack-aws/blob/master/aws/apis/v1alpha2/types.go#L43
[aws-provider-docs]: https://github.com/crossplaneio/stack-aws/blob/master/aws/apis/v1alpha2/types.go#L43

[`aws` command line tool]: https://aws.amazon.com/cli/
[AWS SDK for GO]: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html

[installed]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html
[aws-cli-configure]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
[AWS security credentials]: https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
[secret]: https://kubernetes.io/docs/concepts/configuration/secret/
[aws named profile]: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html

[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli

[Virtual Private Network]: https://aws.amazon.com/vpc/
[Subnet]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html#vpc-subnet-basics
[AWS resource connectivity]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#amazon-web-services
[Internet Gateway]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Internet_Gateway.html
[Route Table]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Route_Tables.html
[Security Group]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_SecurityGroups.html
[Database Subnet Group]: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_VPC.WorkingWithRDSInstanceinaVPC.html
[IAM Role]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html
[IAM Role Policy]: https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html

[portable-classes-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-default-resource-class.md
[resource-classes-docs]: concepts.md#resource-claims-and-resource-classes

[stacks-guide-continue]: stacks-guide.md#install-support-for-our-application-into-crossplane
[resource-claims-docs]: concepts.md#resource-claims-and-resource-classes
[eks-user-guide]: https://docs.aws.amazon.com/eks/latest/userguide/create-public-private-vpc.html
[Cross Resource Referencing]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
[sample AWS network configuration]: https://github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/gitops/aws-infra
[RDS Database Instance]: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Overview.DBInstance.html
[EKS Cluster]: https://docs.aws.amazon.com/eks/latest/userguide/clusters.html
---
title: "Stacks Guide: AWS Setup"
toc: true
weight: 530
indent: true
---

# Stacks Guide: AWS Setup

## Table of Contents

- [Stacks Guide: AWS Setup](#stacks-guide-aws-setup)
  - [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Install the AWS stack](#install-the-aws-stack)
    - [Validate the installation](#validate-the-installation)
  - [Configure the AWS account](#configure-the-aws-account)
  - [Set Up Network Configuration](#set-up-network-configuration)
    - [TL;DR](#tldr)
    - [Behind the scenes](#behind-the-scenes)
  - [Configure Resource Classes](#configure-resource-classes)
    - [TL;DR](#tldr-1)
    - [More Details](#more-details)
  - [Recap](#recap)
  - [Next Steps](#next-steps)

## Introduction

In this guide, we will set up an AWS provider in Crossplane so that we can
install and use the [WordPress sample stack][sample-WordPress-stack], which
depends on MySQL and Kubernetes!

Before we begin, you will need:

- Everything from the [Crossplane Stacks Guide][stacks-guide] before the cloud
  provider setup
  - The `kubectl` (v1.15+) tool installed and pointing to a Crossplane cluster
  - The [Crossplane CLI][crossplane-cli] installed
- An account on [AWS][aws]
- The [aws cli][aws command line tool] installed

At the end, we will have:

- A Crossplane cluster configured to use AWS
- A typical AWS network configured to support secure connectivity between
  resources
- Support in Crossplane cluster for satisfying MySQL and Kubernetes claims
- A slightly better understanding of:
  - The way AWS is configured in Crossplane
  - The way dependencies for cloud-portable workloads are configured in
    Crossplane

We will **not** be covering the core concepts in this guide, but feel free to
check out the [Crossplane concepts document][crossplane-concepts] for that.

## Install the AWS stack

After Crossplane has been installed, it can be extended with more functionality
by installing a [Crossplane Stack][stack-docs]! Let's install the [stack for
Amazon Web Services][stack-aws] (AWS) to add support for that cloud provider.

The namespace where we install the stack, is also the one in which the provider
secret will reside. The name of this namespace is arbitrary, and we are calling
it `crossplane-system` in this guide. Let's create it:

```bash
# namespace for AWS stack and provider secret
kubectl create namespace crossplane-system
```

Now we install the AWS stack using Crossplane CLI. Since this is an
infrastructure stack, we need to specify that it's cluster-scoped by passing the
`--cluster` flag.

```bash
kubectl crossplane stack generate-install --cluster 'crossplane/stack-aws:master' stack-aws | kubectl apply --namespace crossplane-system -f -
```

The rest of this guide assumes that the AWS stack is installed within
`crossplane-system` namespace.

### Validate the installation

To check to see whether our stack installed correctly, we can look at the status
of our stack:

```bash
kubectl -n crossplane-system get stack
```

It should look something like:

```bash
NAME        READY   VERSION   AGE
stack-aws   True    0.0.2     45s
```

## Configure the AWS account

It is essential to make sure that the AWS user credentials are configured in
Crossplane as a provider. Please follow the steps in the AWS [provider
guide][aws-provider-guide] for more information.

## Set Up Network Configuration

In this section we build a simple AWS network configuration, by creating
corresponding Crossplane managed resources. These resources are cluster scoped,
so don't belong to a specific namespace. This network configuration enables
resources in the WordPress stack to communicate securely. In this guide, we will use
the [sample AWS network configuration][] in the Crossplane repository. You can read
more [here][crossplane-aws-networking-docs] about network secure connectivity
configurations in Crossplane.

### TL;DR

Apply the sample network configuration resources:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/network-config?ref=master
```

And you're done! You can check the status of the provisioning by running:

```bash
kubectl get -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/network-config?ref=master
```

When all resources have the `Ready` condition in `True` state, the provisioning
is complete. You can now move on to the next section, or keep reading below for
more details about the managed resources that we created.

### Behind the scenes

When configured in AWS, WordPress resources map to an EKS cluster and an RDS
database instance. In order to make the RDS instance accessible from the EKS
cluster, they both need to live within the same VPC. However, a VPC is not the
only AWS resource that needs to be created to provide inter-resource
connectivity. In general, a **Network Configuration** which consists of a set of
VPCs, Subnets, Security Groups, Route Tables, IAM Roles and other resources is
required for this purpose. For more information, see [AWS resource
connectivity][aws-resource-connectivity] design document.

To inspect the resources that we created above, let's run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/network-config?ref=master > network-config.yaml
```

This will save the sample network configuration resources locally in
`network-config.yaml`. Please note that the AWS parameters that are used in
these resources (like `cidrBlock`, `region`, etc...) are arbitrarily chosen in
this solution and could be configured to implement other
[configurations][eks-user-guide].

Below we inspect each of these resources in more details.

- **`VPC`** Represents an AWS [Virtual Private Network][] (VPC).

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha3
  kind: VPC
  metadata:
    name: sample-vpc
  spec:
    cidrBlock: 192.168.0.0/16
    enableDnsSupport: true
    enableDnsHostNames: true
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ```

- **`Subnet`** Represents an AWS [Subnet][]. For this configuration we create
  one Subnet per each availability zone in the selected region.

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha3
  kind: Subnet
  metadata:
    name: sample-subnet1
  spec:
    cidrBlock: 192.168.64.0/18
    vpcIdRef:
      name: sample-vpc
    availabilityZone: us-west-2a
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ---
  apiVersion: network.aws.crossplane.io/v1alpha3
  kind: Subnet
  metadata:
    name: sample-subnet2
  spec:
    cidrBlock: 192.168.128.0/18
    vpcIdRef:
      name: sample-vpc
    availabilityZone: us-west-2b
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ---
  apiVersion: network.aws.crossplane.io/v1alpha3
  kind: Subnet
  metadata:
    name: sample-subnet3
  spec:
    cidrBlock: 192.168.192.0/18
    vpcIdRef:
      name: sample-vpc
    availabilityZone: us-west-2c
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ```

- **`InternetGateway`** Represents an AWS [Internet Gateway][] which allows the
  resources in the VPC to have access to the Internet. Since the WordPress
  application will be accessed from the internet, this resource is required in
  the network configuration.

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha3
  kind: InternetGateway
  metadata:
    name: sample-internetgateway
  spec:
    vpcIdRef:
      name: sample-vpc
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ```

- **`RouteTable`** Represents an AWS [Route Table][], which specifies rules to
  direct traffic in a virtual network. We use a Route Table to redirect internet
  traffic from all Subnets to the Internet Gateway instance.

    ```yaml
    ---
    apiVersion: network.aws.crossplane.io/v1alpha3
    kind: RouteTable
    metadata:
      name: sample-routetable
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
    ```

- **`SecurityGroup`** Represents an AWS [Security Group][], which controls
  inbound and outbound traffic to EC2 instances.

  We need two security groups in this configuration:

  - A security group to assign later to the EKS cluster workers, so they have
    the right permissions to communicate with each API server

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha3
  kind: SecurityGroup
  metadata:
    name: sample-cluster-sg
  spec:
    vpcIdRef:
      name: sample-vpc
    groupName: my-cool-ekscluster-sg
    description: Cluster communication with worker nodes
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ```

  - A security group to assign later to the RDS database instance, which
    allows the instance to accept traffic from worker nodes.

  ```yaml
  ---
  apiVersion: network.aws.crossplane.io/v1alpha3
  kind: SecurityGroup
  metadata:
    name: sample-rds-sg
  spec:
    vpcIdRef:
      name: sample-vpc
    groupName: my-cool-rds-sg
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
  ```

- **`DBSubnetGroup`** Represents an AWS [Database Subnet Group][] that stores a
  set of existing Subnets in different availability zones, from which an IP
  address is chosen and assigned to the RDS instance.

  ```yaml
  ---
  apiVersion: database.aws.crossplane.io/v1alpha3
  kind: DBSubnetGroup
  metadata:
    name: sample-dbsubnetgroup
  spec:
    groupName: my-cool-dbsubnetgroup
    description: EKS vpc to rds
    subnetIdRefs:
      - name: sample-subnet1
      - name: sample-subnet2
      - name: sample-subnet3
    tags:
      - key: name
        value: my-cool-dbsubnetgroup
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ```

- **`IAMRole`** Represents An AWS [IAM Role][], which assigns a set of access
  policies to the AWS principal that assumes it. We create a role, and later add
  policies to it and then assign the role to the cluster. This grants the
  permissions the cluster needs to communicate with other resources in AWS.

  ```yaml
  ---
  apiVersion: identity.aws.crossplane.io/v1alpha3
  kind: IAMRole
  metadata:
    name: sample-eks-cluster-role
  spec:
    roleName: my-cool-eks-cluster-role
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
  ```

- **`IAMRolePolicyAttachment`** Represents an AWS [IAM Role Policy][], which
  defines a certain permission in an IAM Role. We need two policies to create
  and assign to the IAM Role above, so the cluster my communicate with other
  AWS resources.

  ```yaml
  ---
  apiVersion: identity.aws.crossplane.io/v1alpha3
  kind: IAMRolePolicyAttachment
  metadata:
    name: sample-role-servicepolicy
  spec:
    roleNameRef:
      name: sample-eks-cluster-role
    # wellknown policy arn
    policyArn: arn:aws:iam::aws:policy/AmazonEKSServicePolicy
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ---
  apiVersion: identity.aws.crossplane.io/v1alpha3
  kind: IAMRolePolicyAttachment
  metadata:
    name: sample-role-clusterpolicy
  spec:
    roleNameRef:
      name: sample-eks-cluster-role
    # wellknown policy arn
    policyArn: arn:aws:iam::aws:policy/AmazonEKSClusterPolicy
    reclaimPolicy: Delete
    providerRef:
      name: aws-provider
  ```

As you probably have noticed, some resources are referencing other resources in
their YAML representations. For instance for `Subnet` resource we have:

```yaml
...
    vpcIdRef:
      name: sample-vpc
...
```

Such cross resource referencing is a Crossplane feature that enables managed
resources to retrieve other resources attributes. This creates a *blocking
dependency*, preventing the dependent resource from being  created before the referred
resource is ready. In the example above, `Subnet` will be blocked until the
referred `VPC` is created, and then it retrieves its `vpcId`. For more
information, see [Cross Resource Referencing][].

## Configure Resource Classes

Once we have the network configuration set up, we need to tell Crossplane how to
satisfy WordPress's claims (that will be created when we later install the
WordPress stack) for a database and a Kubernetes cluster. The [Resource
Classes][resource-claims-docs] serve as templates for the corresponding resource
claims.

In this guide, we will use the [sample AWS resource classes][] in Crossplane
repository.

### TL;DR

Apply the sample AWS resource classes:

```bash
kubectl apply -k github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/resource-classes?ref=master
```

And you're done! Note that these resources do not immediately provision external
AWS resources, as they only serve as as template classes.

### More Details

To inspect the resource classes that we created above, run:

```bash
kubectl kustomize github.com/crossplaneio/crossplane//cluster/examples/workloads/kubernetes/wordpress/aws/resource-classes?ref=master > resource-classes.yaml
```

This will save the sample resource classes YAML locally in
`resource-classes.yaml`. As mentioned above, these resource classes serve as
templates and could be configured depending on the specific needs that are
needed from the underlying resources. For instance, in the sample resources the
`RDSInstanceClass` has `size: 20`, which will result in RDS databases of size 20
once a claim is submitted for this class. In addition, it's possible to have
multiple classes defined for the same claim kind, but our sample has defined
only one class for each resource type.

Below we inspect each of these resource classes in more details:

- **`RDSInstanceClass`** Represents a resource that serves as a template to
  create an [RDS Database Instance][].

  ```yaml
  ---
  apiVersion: database.aws.crossplane.io/v1beta1
  kind: RDSInstanceClass
  metadata:
    name: standard-mysql
    annotations:
      resourceclass.crossplane.io/is-default-class: "true"
  specTemplate:
    writeConnectionSecretsToNamespace: crossplane-system
    forProvider:
      dbInstanceClass: db.t2.small
      masterUsername: cool_user
      vpcSecurityGroupIDRefs:
        - name: sample-rds-sg
      dbSubnetGroupNameRef:
        name: sample-dbsubnetgroup
      allocatedStorage: 20
      engine: mysql
      skipFinalSnapshotBeforeDeletion: true
    providerRef:
      name: aws-provider
    reclaimPolicy: Delete
  ```

- **`EKSClusterClass`** Represents a resource that serves as a template to create an [EKS Cluster][].

  ```yaml
  ---
  apiVersion: compute.aws.crossplane.io/v1alpha3
  kind: EKSClusterClass
  metadata:
    name: standard-cluster
    annotations:
      resourceclass.crossplane.io/is-default-class: "true"
  specTemplate:
    writeConnectionSecretsToNamespace: crossplane-system
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
        name: sample-cluster-sg
    providerRef:
      name: aws-provider
    reclaimPolicy: Delete
  ```

These resources will be the default resource classes for the corresponding
claims (`resourceclass.crossplane.io/is-default-class: "true"` annotation). For
more details about resource claims and how they work, see the documentation on
[resource claims][resource-claims-docs], and [resource class selection].

## Recap

To recap what we've set up now in our environment:

- A Crossplane Provider resource for AWS
- A Network Configuration to have secure connectivity between resources
- An EKSClusterClass and an RDSInstanceClass with the right configuration to use
  the mentioned networking setup.

## Next Steps

Next we'll set up a Crossplane App Stack and use it! Head [back over to the
Stacks Guide document][stacks-guide-continue] so we can pick up where we left
off.

<!-- Links -->
[crossplane-concepts]: concepts.md
[stacks-guide]: stacks-guide.md
[aws]: https://aws.amazon.com
[stack-aws]: https://github.com/crossplaneio/stack-aws
[sample-wordpress-stack]: https://github.com/crossplaneio/sample-stack-wordpress
[stack-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks
[aws command line tool]: https://aws.amazon.com/cli/
[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli/tree/release-0.2
[Virtual Private Network]: https://aws.amazon.com/vpc/
[Subnet]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html#vpc-subnet-basics
[aws-resource-connectivity]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#amazon-web-services
[Internet Gateway]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Internet_Gateway.html
[Route Table]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Route_Tables.html
[Security Group]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_SecurityGroups.html
[Database Subnet Group]: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_VPC.WorkingWithRDSInstanceinaVPC.html
[IAM Role]: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html
[IAM Role Policy]: https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html
[stacks-guide-continue]: stacks-guide.md#install-support-for-our-application-into-crossplane
[resource-claims-docs]: concepts.md#resource-claims-and-resource-classes
[eks-user-guide]: https://docs.aws.amazon.com/eks/latest/userguide/create-public-private-vpc.html
[Cross Resource Referencing]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
[sample AWS network configuration]: https://github.com/crossplaneio/crossplane/tree/master/cluster/examples/workloads/kubernetes/wordpress/aws/network-config?ref=master
[sample AWS resource classes]: https://github.com/crossplaneio/crossplane/tree/master/cluster/examples/workloads/kubernetes/wordpress/aws/resource-classes?ref=master
[RDS Database Instance]: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Overview.DBInstance.html
[EKS Cluster]: https://docs.aws.amazon.com/eks/latest/userguide/clusters.html
[resource-classes-docs]: concepts.md#resource-claims-and-resource-classes
[resource class selection]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-simple-class-selection.md
[crossplane-aws-networking-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-resource-connectivity-mvp.md#amazon-web-services
[aws-provider-guide]: cloud-providers/aws/aws-provider.md

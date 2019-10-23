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
  1. [Configure Crossplane Provider for
    AWS](#configure-crossplane-provider-for-aws)
  1. [Set Up Network Configuration](#set-up-network-configuration)
  1. [Configure Provider Resources](#configure-provider-resources)
  1. [Recap](#recap)
  1. [Next Steps](#next-steps)

## Introduction

In this guide, we will set up an AWS provider in Crossplane so that we
can install and use the [WordPress sample
stack][sample-wordpress-stack], which depends on MySQL and Kubernetes!

Before we begin, you will need:

* Everything from the [Crossplane Stacks Guide][stacks-guide] before the
  cloud provider setup
  - A `kubectl` pointing to a Crossplane control cluster
  - The [Crossplane CLI][crossplane-cli] installed
* An account on [AWS][aws]
* The [aws cli][installed]

At the end, we will have:

* A Crossplane control cluster configured to use AWS
* The boilerplate of an AWS-based project spun up
* Support in the control cluster for managing MySQL and Kubernetes
  cluster dependencies
* A slightly better understanding of:
  - The way cloud providers are configured in Crossplane
  - The way dependencies for cloud-portable workloads are configured in
    Crossplane

## Install the AWS stack

After Crossplane has been installed, it can be extended with more
functionality by installing a [Crossplane Stack][stack-docs]! Let's
install the [stack for Amazon Web Services][stack-aws] (AWS) to add
support for that cloud provider.

The namespace where we install the stack, is also the one that our managed AWS
resources will reside. Let's call this namespace `infra-aws`, and go ahead and
create it:

```bash
# the namespace that the aws infra structure resources will be created
export INFRA_NAMESPACE=infra-aws
# create the namespace in Crossplane
kubectl create namespace ${INFRA_NAMESPACE}
```

Now we can install the AWS stack using Crossplane CLI.  Since this is an
infrastructure stack, we need to specify that it's cluster-scoped by
passing the `--cluster` flag.

```bash
kubectl crossplane stack generate-install --cluster 'crossplane/stack-aws:master' stack-aws | kubectl apply --namespace ${INFRA_NAMESPACE} -f -
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
under the `crossplane-user` profile, but you can use a different profile
name if you want. Let's store the profile name in a variable so we can
use it in later steps:

```bash
export aws_profile=crossplane-user
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
export AWS_CREDS_BASE64=$(echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $aws_profile)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $aws_profile)" | base64  | tr -d "\n")
# retrieve the profile's region from config
export AWS_REGION=$(aws configure get region --profile ${aws_profile})
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
  namespace: ${INFRA_NAMESPACE}
type: Opaque
---
apiVersion: aws.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: aws-provider
  namespace: ${INFRA_NAMESPACE}
spec:
  credentialsSecretRef:
    key: credentials
    name: aws-user-creds
  region: ${AWS_REGION}
EOF

# apply it to the cluster:
kubectl apply -f "provider.yaml"
unset AWS_CREDS_BASE64
```

The output will look like the following:

```bash
secret/aws-user-creds created
provider.aws.crossplane.io/aws-provider created
```

## Set Up Network Configuration

When configured in AWS, WordPress resources map to an EKS cluster and an RDS
database. In order to make the RDS instance accessible from the EKS cluster,
they both need to live within the same VPC. However, a VPC is not the only AWS
resource that needs to be created to enable inter-resource connectivity. In
general, a **Network Configuration** which consists of a set of VPCs, Subnets,
Security Groups, Route Tables, IAM Roles and other resources, is required for
this purpose. For more information, see [AWS resource connectivity] design
document.

Using managed AWS resources in Crossplane, in this section we build a simple
network configuration for WordPress stack.

First let's create a folder `network-config`, in which we put all the YAML
representation of all the required managed resources:

```bash
mkdir -p network-config
```

Then in the following we create each resource, applying the
shell variables we created earlier, and explain each in more details:

  * **`VPC`** Represents an [AWS Virtual Private Network][] (VPC).

    ```bash
    # build vpc yaml
    cat > network-config/vpc.yaml <<EOF
    apiVersion: network.aws.crossplane.io/v1alpha2
    kind: VPC
    metadata:
      name: ${CONFIG_NAME}-vpc
      namespace: ${INFRA_NAMESPACE}
    spec:
      cidrBlock: 192.168.0.0/16
      enableDnsSupport: true
      enableDnsHostNames: true
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    EOF
    ```

  * **`Subnet`** Represents an [AWS Subnet][]. For this configuration we create
    one Subnet per each availability zone in the region.

    ```bash
    # build subnet yaml
    cat > network-config/subnets.yaml <<EOF
    ---
    apiVersion: network.aws.crossplane.io/v1alpha2
    kind: Subnet
    metadata:
      name: ${CONFIG_NAME}-subnet1
      namespace: ${INFRA_NAMESPACE}
    spec:
      cidrBlock: 192.168.64.0/18
      vpcIdRef:
        name: ${CONFIG_NAME}-vpc
      availabilityZone: ${AWS_REGION}a
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    ---
    apiVersion: network.aws.crossplane.io/v1alpha2
    kind: Subnet
    metadata:
      name: ${CONFIG_NAME}-subnet2
      namespace: ${INFRA_NAMESPACE}
    spec:
      cidrBlock: 192.168.128.0/18
      vpcIdRef:
        name: ${CONFIG_NAME}-vpc
      availabilityZone: ${AWS_REGION}b
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    ---
    apiVersion: network.aws.crossplane.io/v1alpha2
    kind: Subnet
    metadata:
      name: ${CONFIG_NAME}-subnet3
      namespace: ${INFRA_NAMESPACE}
    spec:
      cidrBlock: 192.168.192.0/18
      vpcIdRef:
        name: ${CONFIG_NAME}-vpc
      availabilityZone: ${AWS_REGION}c
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    EOF
    ```

  * **`InternetGateway`** Represents an AWS [Internet Gateway][] which allows
    the resources in the VPC to have access to the Internet. Since the
    WordPress application will be accessed from the internet, this resource is
    required in the network configuration.

    ```bash
    # build internet gateway yaml
    cat > network-config/internetgateway.yaml <<EOF
    apiVersion: network.aws.crossplane.io/v1alpha2
    kind: InternetGateway
    metadata:
      name: ${CONFIG_NAME}-internetgateway
      namespace: ${INFRA_NAMESPACE}
    spec:
      vpcIdRef:
        name: ${CONFIG_NAME}-vpc
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    EOF
    ```

  * **`RouteTable`** Represents an AWS [Route Table][], which specifies rules to
    direct traffic in a virtual network. We use a Route Table to redirect internet
    traffic from all Subnets to the Internet Gateway instance.

      ```bash
      # build route table yaml
      cat > network-config/routetable.yaml <<EOF
      apiVersion: network.aws.crossplane.io/v1alpha2
      kind: RouteTable
      metadata:
        name: ${CONFIG_NAME}-routetable
        namespace: ${INFRA_NAMESPACE}
      spec:
        vpcIdRef:
          name: ${CONFIG_NAME}-vpc
        routes:
          - destinationCidrBlock: 0.0.0.0/0
            gatewayIdRef:
              name: ${CONFIG_NAME}-internetgateway
        associations:
          - subnetIdRef: 
              name: ${CONFIG_NAME}-subnet1
          - subnetIdRef:
              name: ${CONFIG_NAME}-subnet2
          - subnetIdRef:
              name: ${CONFIG_NAME}-subnet3
        reclaimPolicy: Delete
        providerRef:
          name: aws-provider
          namespace: ${INFRA_NAMESPACE}
      EOF
      ```

  * **`SecurityGroup`** Represents an AWS [Security Group][], which controls
    inbound and outbound traffic to EC2 instances.

    We need two security groups in this configuration:

    * A security group to assign it later to the EKS cluster workers, so they have
     the right permissions to communicate with each API server

      ```bash
      # build the cluster security group yaml
      cat > network-config/eks_securitygroup.yaml <<EOF
      apiVersion: network.aws.crossplane.io/v1alpha2
      kind: SecurityGroup
      metadata:
        name: ${CONFIG_NAME}-cluster-sg
        namespace: ${INFRA_NAMESPACE}
      spec:
        vpcIdRef:
          name: ${CONFIG_NAME}-vpc
        groupName: ${CONFIG_NAME}-ekscluster-sg
        description: Cluster communication with worker nodes
        reclaimPolicy: Delete
        providerRef:
          name: aws-provider
          namespace: ${INFRA_NAMESPACE}
      EOF
      ```

    * A security group to assign it later to the RDS database instance, which
      allows the instance to accept traffic from worker nodes.

      ```bash
      # build the rds security group yaml
      cat > network-config/rds_securitygroup.yaml <<EOF
      apiVersion: network.aws.crossplane.io/v1alpha2
      kind: SecurityGroup
      metadata:
        name: ${CONFIG_NAME}-rds-sg
        namespace: ${INFRA_NAMESPACE}
      spec:
        vpcIdRef:
          name: ${CONFIG_NAME}-vpc
        groupName: ${CONFIG_NAME}-rds-sg
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
          namespace: ${INFRA_NAMESPACE}
      EOF
      ```

  * **`DBSubnetGroup`** Represents an AWS [Database Subnet Group][], which
    creates a group of Subnets that can communicate with the RDS database
    instance that we will create later.

    ```bash
    # build db subnet group yaml
    cat > network-config/dbsubnetgroup.yaml <<EOF
    apiVersion: storage.aws.crossplane.io/v1alpha2
    kind: DBSubnetGroup
    metadata:
      name: ${CONFIG_NAME}-dbsubnetgroup
      namespace: ${INFRA_NAMESPACE}
    spec:
      groupName: ${CONFIG_NAME}_dbsubnetgroup
      description: EKS vpc to rds
      subnetIdRefs:
        - name: ${CONFIG_NAME}-subnet1
        - name: ${CONFIG_NAME}-subnet2
        - name: ${CONFIG_NAME}-subnet3
      tags:
        - key: name
          value: ${CONFIG_NAME}-dbsubnetgroup
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    EOF
    ```

  * **`IAMRole`** Represents An AWS [IAM Role][], which assigns a set of access policies to the
    AWS principal that assumes it. We create a role to later add needed policies and assign it to the
    cluster, granting the permissions it needs to communicate with other resources
    in AWS.

    ```bash
    cat > network-config/iamrole.yaml <<EOF
    apiVersion: identity.aws.crossplane.io/v1alpha2
    kind: IAMRole
    metadata:
      name: ${CONFIG_NAME}-eks-cluster-role
      namespace: ${INFRA_NAMESPACE}
    spec:
      roleName: ${CONFIG_NAME}-eks-cluster-role
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
        namespace: ${INFRA_NAMESPACE}
    EOF
    ```


  * **`IAMRolePolicyAttachment`** Represents an AWS [IAM Role Policy][], which
    defines a certain permission in an IAM Role. We need two policies to create
    and assign it to the IAM Role above, so the cluster to communicate with
    other aws resources.

    ```bash
    # build policies yaml
    cat > network-config/iamrole_policies.yaml <<EOF
    ---
    apiVersion: identity.aws.crossplane.io/v1alpha2
    kind: IAMRolePolicyAttachment
    metadata:
      name: ${CONFIG_NAME}-role-servicepolicy
      namespace: ${INFRA_NAMESPACE}
    spec:
      roleNameRef:
        name: ${CONFIG_NAME}-eks-cluster-role
      # wellknown policy arn
      policyArn: arn:aws:iam::aws:policy/AmazonEKSServicePolicy
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    ---
    apiVersion: identity.aws.crossplane.io/v1alpha2
    kind: IAMRolePolicyAttachment
    metadata:
      name: ${CONFIG_NAME}-role-clusterpolicy
      namespace: ${INFRA_NAMESPACE}
    spec:
      roleNameRef:
        name: ${CONFIG_NAME}-eks-cluster-role
      # wellknown policy arn
      policyArn: arn:aws:iam::aws:policy/AmazonEKSClusterPolicy
      reclaimPolicy: Delete
      providerRef:
        name: aws-provider
        namespace: ${INFRA_NAMESPACE}
    EOF
    ```

At this point, you should have folder `network-config` with the following content:

```bash
├── network-config
│   ├── dbsubnetgroup.yaml
│   ├── eks_securitygroup.yaml
│   ├── iamrole_policies.yaml
│   ├── iamrole.yaml
│   ├── internetgateway.yaml
│   ├── rds_securitygroup.yaml
│   ├── routetable.yaml
│   ├── subnets.yaml
│   └── vpc.yaml
```

As you probably have noticed, some resources are referencing other resource attributes in their YAML representations. For instance in `Subnet` YAML we have:

```yaml
...
    vpcIdRef:
      name: ${CONFIG_NAME}-vpc
...
```

Such cross resource referencing is a Crossplane feature that enables managed resources to retrieve other resources attributes. This creates a *blocking dependency*, avoiding the dependent resource to be created before the referred resource is ready. In the example above, `Subnet` will be blocked until the referred `VPC` is created, and then it retrieves its `vpcId`.  For more information, see [Cross Resource Referencing][].

Now you can install all these resources by simply running:

```bash
kubectl apply -f network-config
```

This will create all the managed resources, honoring the resource dependencies we mentioned above. This should take a few seconds to complete. You can check the status of provisioning by running:

```bash
kubectl get -f network-config
```

When all resources has the `Ready` condition in `True` state, the provisioning is completed.

In the next section we are going to add more resources to this folder.

## Configure Provider Resources

Once we have the network configuration set up, we also need to tell Crossplane
how to satisfy WordPress's claims for a database and a Kubernetes cluster, using
AWS resources. [Resource classes][resource-classes-docs] serve as
templates for the new claims we make. The following resource classes
allow the claims for the database and Kubernetes cluster to be satisfied
with the network configuration we just set up:

```bash
# build resource classes yaml, by using the configured network resources
cat > network-config/resource_classes.yaml <<EOF
---
apiVersion: database.aws.crossplane.io/v1alpha2
kind: RDSInstanceClass
metadata:
  name: standard-mysql
  namespace: ${INFRA_NAMESPACE}
specTemplate:
  class: db.t2.small
  masterUsername: masteruser
  securityGroupRefs:
    - name: ${CONFIG_NAME}-rds-sg
  subnetGroupNamRef:
    name: ${CONFIG_NAME}-dbsubnetgroup
  size: 20
  engine: mysql
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
  reclaimPolicy: Delete
---
apiVersion: compute.aws.crossplane.io/v1alpha2
kind: EKSClusterClass
metadata:
  name: standard-cluster
  namespace: ${INFRA_NAMESPACE}
specTemplate:
  region: ${AWS_REGION}
  roleARNRef:
    name: ${CONFIG_NAME}-eks-cluster-role
  vpcIdRef:
    name: ${CONFIG_NAME}-vpc
  subnetIdRefs:
    - name: ${CONFIG_NAME}-subnet1
    - name: ${CONFIG_NAME}-subnet2
    - name: ${CONFIG_NAME}-subnet3
  securityGroupIdRefs:
    - name: ${CONFIG_NAME}-cluster-sg
  workerNodes:
    nodeInstanceType: m3.medium
    nodeAutoScalingGroupMinSize: 1
    nodeAutoScalingGroupMaxSize: 1
    nodeGroupName: demo-nodes
    clusterControlPlaneSecurityGroupRef:
      - name: ${CONFIG_NAME}-cluster-sg
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
  reclaimPolicy: Delete
EOF
```

So far we have been creating resources in `$INFRA_NAMESPACE`, where a
bunch of resources live: the ones to configure the AWS stack with the
AWS account; to create a network configuration; and to define resources
classes that will satisfy our claims. Now, we will create an app
namespace and populate it with resources that are used to let Crossplane
know how to satisfy the claims. Let's call this namespace
`app-project1-dev`.

```bash
# the namespace that the app resources will be created
export APP_NAMESPACE=app-project1-dev
# create the namespace in Crossplane
kubectl create namespace ${APP_NAMESPACE}
```

Now that we have a namespace, we need to tell Crossplane which resource classes
should be used to satisfy our claims in that namespace. We will create [portable
classes][portable-classes-docs] that have references to the
cloud-specific classes that we created earlier.

For example, `MySQLInstanceClass` is a portable class. It may refer to AWS's
`RDSInstanceClass`, which is a non-portable class.

To read more about portable classes, how they work, and how to use them in
different ways, including by specifying default classes when no reference is
provided, see the [portable classes and claims
documentation][portable-classes-docs].

```bash
cat > portable_classes.yaml <<EOF
---
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: mysql-standard
  namespace: ${APP_NAMESPACE}
  labels:
    default: "true"
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha2
  name: standard-mysql
  namespace: ${INFRA_NAMESPACE}
---
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesClusterClass
metadata:
  name: k8s-standard
  namespace: ${APP_NAMESPACE}
  labels:
    default: "true"
classRef:
  kind: EKSClusterClass
  apiVersion: compute.aws.crossplane.io/v1alpha2
  name: standard-cluster
  namespace: ${INFRA_NAMESPACE}
---
EOF
```

For more details about resource claims and how they work, see the [documentation
on resource claims][resource-claims-docs].

## Recap

To recap what we've set up now in our environment:

* Our provider account, both on the provider side and on the Crossplane side.
* A Network Configuration for all instances to share.
* An EKSClusterClass and an RDSInstanceClass with the right configuration to use
  the mentioned networking setup.
* A namespace for our app resources, with a default MySQLInstanceClass and
  a default KubernetesClusterClass that refer to our EKSClusterClass and
  RDSInstanceClass.

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

[AWS Virtual Private Network]: https://aws.amazon.com/vpc/
[AWS Subnet]: https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html#vpc-subnet-basics
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
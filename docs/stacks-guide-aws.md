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
general, a **Network Configuration**, which consists of a set of VPCs, Subnets,
Security Groups, Route Tables, IAM Roles and other resources, is required for
this purpose. For more information, see [AWS resource connectivity][] design
document.

In this section, we will build a simple network configuration, by creating AWS
resources that are managed by Crossplane. There are a couple of challenges when
creating these resources:

- Some of these resources depend on other ones. For instance, a Subnet
  depends on a VPC, so creating a Subnet needs to be done after creating
  the VPC.

  To solve this issue, we will need to create the resources in order, so
  resources are provisioned after their dependencies exist. Since
  provisioning a resource might take some time, we need to make sure the
  resource is ready before moving forward to the next step. Let's create the
  following function for this purpose:

  ```bash
  # apply_and_wait_until_ready accepts the yaml file name as an argument
  # and then applies the yaml object. Then waits until the resource status
  # becomes Ready, which indicates that the resource is provisioned and
  # ready to be used
  function apply_and_wait_until_ready {
    kubectl apply -f "$1"
    kubectl wait --for=condition=Ready -f "$1"
  }
  ```

- Some of these resources have identifying attributes that are
  non-deterministic. In other words, they become known after the resource is
  provisioined. For instance, a VPC has an ID (VPC_ID) attribute which is
  consumed by other resources (such as a Subnet), and the ID only
  becomes known after the VPC is created.

  To tackle this challege, we will need to retrieve the non-deterministic
  identifiers of the resources after their creation, and inject them to the
  consumer resources that require those attributes.

The rest of this section creates the resources for a configuration described in
[the EKS user guide][eks-user-guide].
For grouping all these resources together, we will use a `CONFIG_NAME` variable,
which will be prepended to the names of these resources in Crossplane,
and also their corresponding external resources in AWS. Keep in mind
that if you create multiple such configurations in the same Crossplane
cluster or the same AWS account, you will need to use different config
names. Otherwise, there will be naming conflicts.

```bash
# the name of the aws network configuration
export CONFIG_NAME=aws-network-config
```

### VPC

A [Virtual Private Network][] or VPC is a virtual network in AWS.

```bash
# build vpc yaml
cat > vpc.yaml <<EOF
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

# create a vpc object in Crossplane, and wait until the corresponding
# VPC in AWS is created and is Ready to use
apply_and_wait_until_ready "vpc.yaml"
```

Sample output:

```bash
vpc.network.aws.crossplane.io/aws-network-config-vpc created
vpc.network.aws.crossplane.io/aws-network-config-vpc condition met
```

Once the VPC is created, you can see the full object and its status by running:

```bash
kubectl get -f "vpc.yaml" -o yaml
```

The output will look something like:

```yaml
apiVersion: network.aws.crossplane.io/v1alpha2
kind: VPC
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"network.aws.crossplane.io/v1alpha2","kind":"VPC","metadata":{"annotations":{},"name":"aws-network-config-vpc","namespace":"aws"},"spec":{"cidrBlock":"192.168.0.0/16","enableDnsHostNames":true,"enableDnsSupport":true,"providerRef":{"name":"aws-provider","namespace":"infra-aws"},"reclaimPolicy":"Delete"}}
  creationTimestamp: "2019-09-17T04:40:18Z"
  finalizers:
  - finalizer.managedresource.crossplane.io
  generation: 2
  name: aws-network-config-vpc
  namespace: aws
  resourceVersion: "92185"
  selfLink: /apis/network.aws.crossplane.io/v1alpha2/namespaces/aws/vpcs/aws-network-config-vpc
  uid: 052e1934-00e4-43fb-adf8-5a93c45af363
spec:
  cidrBlock: 192.168.0.0/16
  enableDnsHostNames: true
  enableDnsSupport: true
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
  reclaimPolicy: Delete
  writeConnectionSecretToRef: {}
status:
  conditions:
  - lastTransitionTime: "2019-09-17T04:43:24Z"
    reason: Managed resource is available for use
    status: "True"
    type: Ready
  - lastTransitionTime: "2019-09-17T04:43:23Z"
    reason: Successfully reconciled managed resource
    status: "True"
    type: Synced
  vpcId: vpc-0661625a89f410b37
  vpcState: available
```

Now that we have a VPC, we can retrieve the VPCID to use in subsequent
resources and save it to a variable:

```bash
export VPC_ID=$(kubectl get -f "vpc.yaml"  -o jsonpath='{.status.vpcId}')
```

### Subnets

In this configuration we create three public [Subnet][]s.

```bash
# build subnet yaml
cat > subnets.yaml <<EOF
---
apiVersion: network.aws.crossplane.io/v1alpha2
kind: Subnet
metadata:
  name: ${CONFIG_NAME}-subnet1
  namespace: ${INFRA_NAMESPACE}
spec:
  cidrBlock: 192.168.64.0/18
  vpcId: ${VPC_ID}
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
  vpcId: ${VPC_ID}
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
  vpcId: ${VPC_ID}
  availabilityZone: ${AWS_REGION}c
  reclaimPolicy: Delete
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
EOF

# create subnet objects in Crossplane, and wait until the corresponding
# Subnets in AWS are created and Ready to use
apply_and_wait_until_ready "subnets.yaml"
```

Sample output:

```bash
subnet.network.aws.crossplane.io/aws-network-config-subnet1 created
subnet.network.aws.crossplane.io/aws-network-config-subnet2 created
subnet.network.aws.crossplane.io/aws-network-config-subnet3 created
subnet.network.aws.crossplane.io/aws-network-config-subnet1 condition met
subnet.network.aws.crossplane.io/aws-network-config-subnet2 condition met
subnet.network.aws.crossplane.io/aws-network-config-subnet3 condition met
```

We need to retrieve the SubnetIDs for subsequent resources and save them
to variables:

```bash
export SUBNET1_ID=$(kubectl get -f "subnets.yaml" -o=jsonpath='{.items[0].status.subnetId}')
export SUBNET2_ID=$(kubectl get -f "subnets.yaml" -o=jsonpath='{.items[1].status.subnetId}')
export SUBNET3_ID=$(kubectl get -f "subnets.yaml" -o=jsonpath='{.items[2].status.subnetId}')
```

### Internet Gateway

An [Internet Gateway][] allows the resources in the VPC to have access to the
Internet. Since the WordPress application will be accessed from the internet,
this resource is required in the network configuration.

```bash
# build internet gateway yaml
cat > internetgateway.yaml <<EOF
apiVersion: network.aws.crossplane.io/v1alpha2
kind: InternetGateway
metadata:
  name: ${CONFIG_NAME}-internetgateway
  namespace: ${INFRA_NAMESPACE}
spec:
  vpcId: ${VPC_ID}
  reclaimPolicy: Delete
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
EOF

# create subnet objects in Crossplane, and wait until the corresponding
# Subnets in AWS are created and Ready to use
apply_and_wait_until_ready "internetgateway.yaml"
```

Sample output:

```bash
internetgateway.network.aws.crossplane.io/aws-network-config-internetgateway created
internetgateway.network.aws.crossplane.io/aws-network-config-internetgateway condition met
```

Retrieve the internet gateway's ID (IG_ID) and save it in a variable:

```bash
export IG_ID=$(kubectl get -f "internetgateway.yaml" -o=jsonpath='{.status.internetGatewayId}')
```

### Route Table

A [Route Table][] specifies rules to direct traffic in a virtual network. We use a
Route Table to redirect internet traffic from all Subnets to the Internet
Gateway instance that we created in the previous step.

```bash
# build route table yaml
cat > routetable.yaml <<EOF
apiVersion: network.aws.crossplane.io/v1alpha2
kind: RouteTable
metadata:
  name: ${CONFIG_NAME}-routetable
  namespace: ${INFRA_NAMESPACE}
spec:
  vpcId: ${VPC_ID}
  routes:
    - destinationCidrBlock: 0.0.0.0/0
      gatewayId: ${IG_ID}
  associations:
    - subnetId: ${SUBNET1_ID}
    - subnetId: ${SUBNET2_ID}
    - subnetId: ${SUBNET3_ID}
  reclaimPolicy: Delete
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
EOF

# create a routetable object in Crossplane, and wait until the corresponding
# Route Table in AWS is created and Ready to use
apply_and_wait_until_ready "routetable.yaml"
```

Sample output:

```bash
routetable.network.aws.crossplane.io/aws-network-config-routetable created
routetable.network.aws.crossplane.io/aws-network-config-routetable condition met
```

### Cluster Security Group

A [Security Group][] is created so that later we can assign it to the EKS cluster. This
security group allows the cluster to communicate with the worker nodes.

```bash
# build the cluster security group yaml
cat > cluster_sg.yaml <<EOF
apiVersion: network.aws.crossplane.io/v1alpha2
kind: SecurityGroup
metadata:
  name: ${CONFIG_NAME}-cluster-sg
  namespace: ${INFRA_NAMESPACE}
spec:
  vpcId: ${VPC_ID}
  groupName: ${CONFIG_NAME}-cluster-sg
  description: Cluster communication with worker nodes
  reclaimPolicy: Delete
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
EOF

# create a cluster security group object in Crossplane, and wait until the corresponding
# Security Group in AWS is created and Ready to use
apply_and_wait_until_ready "cluster_sg.yaml"
```

Sample output:

```bash
securitygroup.network.aws.crossplane.io/aws-network-config-cluster-sg created
securitygroup.network.aws.crossplane.io/aws-network-config-cluster-sg condition met
```

Retrieve the SecurityGroupID for cluster security group and save it to a
variable:

```bash
export CLUSTER_SECURITY_GROUP_ID=$(kubectl get -f "cluster_sg.yaml" -o=jsonpath='{.status.securityGroupID}')
```

### Database Security Group

A [Security Group][] is created so that later we can assign it to the RDS database
instance. This security group allows the database instance to accept traffic
from the internet on a certain port.

```bash
# build the rds security group yaml
cat > rds_sg.yaml <<EOF
apiVersion: network.aws.crossplane.io/v1alpha2
kind: SecurityGroup
metadata:
  name: ${CONFIG_NAME}-rds-sg
  namespace: ${INFRA_NAMESPACE}
spec:
  vpcId: ${VPC_ID}
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

# create a security group object for the rds instance in Crossplane
# and wait until the corresponding Security Group in AWS is created and Ready to use
apply_and_wait_until_ready "rds_sg.yaml"
```

Sample output:

```bash
securitygroup.network.aws.crossplane.io/aws-network-config-rds-sg created
securitygroup.network.aws.crossplane.io/aws-network-config-rds-sg condition met
```

Retrieve the SecurityGroupID for rds security group and store it in a
variable:

```bash
export RDS_SECURITY_GROUP_ID=$(kubectl get -f "rds_sg.yaml" -o=jsonpath='{.status.securityGroupID}')
```

### Database Subnet Group

A [Database Subnet Group][] creates a group of Subnets which can communicate with
an RDS database instance.

```bash
# build db subnet group yaml
cat > dbsubnetgroup.yaml <<EOF
apiVersion: storage.aws.crossplane.io/v1alpha2
kind: DBSubnetGroup
metadata:
  name: ${CONFIG_NAME}-dbsubnetgroup
  namespace: ${INFRA_NAMESPACE}
spec:
  groupName: ${CONFIG_NAME}_dbsubnetgroup
  description: EKS vpc to rds
  subnetIds:
    - ${SUBNET1_ID}
    - ${SUBNET2_ID}
    - ${SUBNET3_ID}
  tags:
    - key: name
      value: ${CONFIG_NAME}-dbsubnetgroup
  reclaimPolicy: Delete
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
EOF

# create db subnet group object in Crossplane, and wait until the corresponding
# DB Subnet Group in AWS is created and Ready to use
apply_and_wait_until_ready "dbsubnetgroup.yaml"
```

Sample output:

```bash
dbsubnetgroup.storage.aws.crossplane.io/aws-network-config-dbsubnetgroup created
dbsubnetgroup.storage.aws.crossplane.io/aws-network-config-dbsubnetgroup condition met
```

We need to retrieve the SubnetIDs so other resources can use them:

```bash
export RDS_SUBNET_GROUP_NAME=$(kubectl get -f "dbsubnetgroup.yaml" -o=jsonpath='{.spec.groupName}')
```

### Cluster IAM Role

An [IAM Role][] gives permissions to the principal which assumes that role. We
create a role to be assumed by the cluster, which later is granted the
permissions needed to talk to our resources in AWS.

```bash
# build vpc yaml
cat > iamrole.yaml <<EOF
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

# create an IAM Role object in Crossplane, and wait until the corresponding
# IAM Role in AWS is created and Ready to use
apply_and_wait_until_ready "iamrole.yaml"
```

Sample output:

```bash
iamrole.identity.aws.crossplane.io/aws-network-config-eks-cluster-role created
iamrole.identity.aws.crossplane.io/aws-network-config-eks-cluster-role condition met
```

Retrieve the IAM Role Arn and store it in a variable:

```bash
export EKS_ROLE_ARN=$(kubectl get -f "iamrole.yaml" -o=jsonpath='{.status.arn}')
```

### Cluster IAM Role Policies

An [IAM Role Policy][] grants a role a certain permission. We add two policies to
the Cluster IAM Role that we created above. These policies are needed for the
cluster to communicate with other aws resources.

```bash
# build policies yaml
cat > policies.yaml <<EOF
---
apiVersion: identity.aws.crossplane.io/v1alpha2
kind: IAMRolePolicyAttachment
metadata:
  name: ${CONFIG_NAME}-role-servicepolicy
  namespace: ${INFRA_NAMESPACE}
spec:
  roleName: ${CONFIG_NAME}-eks-cluster-role
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
  roleName: ${CONFIG_NAME}-eks-cluster-role
  policyArn: arn:aws:iam::aws:policy/AmazonEKSClusterPolicy
  reclaimPolicy: Delete
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
EOF

# create IAM Role Policy objects in Crossplane, and wait until the corresponding
# IAM Role Policies in AWS are created and Ready to use
apply_and_wait_until_ready "policies.yaml"
```

Sample output:

```bash
iamrolepolicyattachment.identity.aws.crossplane.io/aws-network-config-role-servicepolicy created
iamrolepolicyattachment.identity.aws.crossplane.io/aws-network-config-role-clusterpolicy created
iamrolepolicyattachment.identity.aws.crossplane.io/aws-network-config-role-servicepolicy condition met
iamrolepolicyattachment.identity.aws.crossplane.io/aws-network-config-role-clusterpolicy condition met
```

## Configure Provider Resources

Once we have the network configuration set up, we also need to tell Crossplane
how to satisfy WordPress's claims for a database and a Kubernetes cluster, using
AWS resources. [Resource classes][resource-classes-docs] serve as
templates for the new claims we make. The following resource classes
allow the claims for the database and Kubernetes cluster to be satisfied
with the network configuration we just set up:

```bash
# build resource classes yaml, by using the configured network resources
cat > resource_classes.yaml <<EOF
---
apiVersion: database.aws.crossplane.io/v1alpha2
kind: RDSInstanceClass
metadata:
  name: standard-mysql
  namespace: ${INFRA_NAMESPACE}
specTemplate:
  class: db.t2.small
  masterUsername: masteruser
  securityGroups:
    - ${RDS_SECURITY_GROUP_ID}
  subnetGroupName: ${RDS_SUBNET_GROUP_NAME}
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
  roleARN: ${EKS_ROLE_ARN}
  vpcId: ${VPC_ID}
  subnetIds:
    - ${SUBNET1_ID}
    - ${SUBNET2_ID}
    - ${SUBNET3_ID}
  securityGroupIds:
    - ${CLUSTER_SECURITY_GROUP_ID}
  workerNodes:
    nodeInstanceType: m3.medium
    nodeAutoScalingGroupMinSize: 1
    nodeAutoScalingGroupMaxSize: 1
    nodeGroupName: demo-nodes
    clusterControlPlaneSecurityGroup: ${CLUSTER_SECURITY_GROUP_ID}
  providerRef:
    name: aws-provider
    namespace: ${INFRA_NAMESPACE}
  reclaimPolicy: Delete
EOF

# apply the resource classes yaml to Crossplane
kubectl apply -f "resource_classes.yaml"
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

kubectl apply -f "portable_classes.yaml"
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

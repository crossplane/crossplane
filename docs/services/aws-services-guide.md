---
title: Using AWS Services
toc: true
weight: 430
indent: true
---

# Deploying Wordpress in Amazon Web Services (AWS)

This user guide will walk you through Wordpress application deployment using
Crossplane managed resources and the official Wordpress Docker image.

## Table of Contents

1. [Pre-requisites](#pre-requisites)
1. [Preparation](#preparation)
1. [Set Up an EKS Cluster](#set-up-an-eks-cluster)
1. [Set Up RDS Configurations](#set-up-rds-configurations)
1. [Set Up Crossplane](#set-up-crossplane)
1. [Install Wordpress](#install-wordpress)
1. [Uninstall](#uninstall)
1. [Conclusion and Next Steps](#conclusion-and-next-steps)

## Pre-requisites

These tools are required to complete this guide. They must be installed on your
local machine.

* [AWS CLI][aws-cli-install]
* [kubectl][install-kubectl]
* [Helm][using-helm], minimum version `v2.10.0+`.
* [jq][jq-docs] - command line JSON processor `v1.5+`

## Preparation

This guide assumes that you have already [installed][aws-cli-install] and
[configured][aws-cli-configure].

*Note: the following session variables are used throughout this guide. You may
use the values below or create your own.*

### Set Up an EKS Cluster

We will create an EKS cluster, follwoing the steps provided in [AWS
documentation][aws-create-eks].

#### Create and Configure an EKS compatible VPN

First, we will need to create a VPC, and its related resources. This can be done
by creating a compatible **CloudFormation stack**, provided in [EKS official
documentation][sample-cf-stack]. This stack consumes a few parameters, that we
will provide by the following variables:

```bash
# we give an arbitrary name to the stack.
# this name has to be unique within an aws account and region
VPC_STACK_NAME=crossplane-example

# any aws region that supports eks clusters
REGION=eu-west-1

# a sample stack that can be used to create an EKS-compatible
# VPC and other the related resources
STACK_URL=https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2019-02-11/amazon-eks-vpc-sample.yaml
```

Once all these variables are set, create the stack:

```bash
# generate and run the CloudFormation Stack
aws cloudformation create-stack \
  --template-url="${STACK_URL}" \
  --region "${REGION}" \
  --stack-name "${VPC_STACK_NAME}" \
  --parameters \
    ParameterKey=VpcBlock,ParameterValue=192.168.0.0/16 \
    ParameterKey=Subnet01Block,ParameterValue=192.168.64.0/18 \
    ParameterKey=Subnet02Block,ParameterValue=192.168.128.0/18 \
    ParameterKey=Subnet03Block,ParameterValue=192.168.192.0/18
```

The output of this command will look like:

>```bash
>{
>    "StackId": "arn:aws:cloudformation:eu-west-1:123456789012:stack/crossplane-example-workers/de1d4770-d9f0-11e9-a293-02673541b8d0"
>}
>```

Creating the stack continues in the background and  could take a few minutes to
complete. You can check its status by running:

```bash
aws cloudformation describe-stacks --output json --stack-name ${VPC_STACK_NAME} --region $REGION | jq -r '.Stacks[0].StackStatus'
```

Once the output of the above command is `CREATE_COMPLETE`, the stack creation is
completed, and you can retrieve some of the properties of the created resources.
These properties will later be consumed in other resources.

```bash
VPC_ID=$(aws cloudformation describe-stacks --output json --stack-name ${VPC_STACK_NAME} --region ${REGION} | jq -r '.Stacks[0].Outputs[]|select(.OutputKey=="VpcId").OutputValue')

# comma separated list of Subnet IDs
SUBNET_IDS=$(aws cloudformation describe-stacks --output json --stack-name ${VPC_STACK_NAME} --region ${REGION} | jq -r '.Stacks[0].Outputs[]|select(.OutputKey=="SubnetIds").OutputValue')

# the ID of security group that later will be used for the EKS cluster
EKS_SECURITY_GROUP=$(aws cloudformation describe-stacks --output json --stack-name ${VPC_STACK_NAME} --region ${REGION} | jq -r '.Stacks[0].Outputs[]|select(.OutputKey=="SecurityGroups").OutputValue')
```

#### Create an IAM Role for the EKS cluster

For the EKS cluster to be able to access different resources, it needs to be
given the required permissions through an **IAM Role**. In this section we
create a role and assign the required policies. Later we will make EKS to assume
this role.

```bash
EKS_ROLE_NAME=crossplane-example-eks-role

# the policy that determines what principal can assume this role
ASSUME_POLICY='{
  "Version": "2012-10-17",
  "Statement": {
    "Effect": "Allow",
    "Principal": {
      "Service": "eks.amazonaws.com"
    },
    "Action": "sts:AssumeRole"
  }
}'

# Create a Role that can be assumed by EKS
aws iam create-role \
  --role-name "${EKS_ROLE_NAME}" \
  --region "${REGION}" \
  --assume-role-policy-document "${ASSUME_POLICY}"
```

The output should be the created role in JSON. Next, you'll attach the required
policies to this role:

```bash
# attach the required policies
aws iam attach-role-policy --role-name "${EKS_ROLE_NAME}" --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy > /dev/null
aws iam attach-role-policy --role-name "${EKS_ROLE_NAME}" --policy-arn arn:aws:iam::aws:policy/AmazonEKSServicePolicy > /dev/null
```

Now lets retrieve the **ARN** of this role and store it in a variable. We later
assign this to the EKS.

```bash
EKS_ROLE_ARN=$(aws iam get-role --output json --role-name "${EKS_ROLE_NAME}" | jq -r .Role.Arn)
```

#### Creating the EKS cluster

At this step we create an EKS cluster:

```bash
CLUSTER_NAME=crossplane-example-cluster
K8S_VERSION=1.14

aws eks create-cluster \
  --region "${REGION}" \
  --name "${CLUSTER_NAME}" \
  --kubernetes-version "${K8S_VERSION}" \
  --role-arn "${EKS_ROLE_ARN}" \
  --resources-vpc-config subnetIds="${SUBNET_IDS}",securityGroupIds="${EKS_SECURITY_GROUP}"
```

An EKS cluster should have started provisioning, which could take take up to 15
minutes. You can check the status of the cluster by periodically running:

```bash
aws eks describe-cluster --name "${CLUSTER_NAME}" --region "${REGION}" | jq -r .cluster.status
```

Once the provisioning is completed, the above command will return `ACTIVE`.

#### Configuring `kubectl` to communicate with the EKS cluster

Once the cluster is created and is `ACTIVE`, we configure `kubectl` to target
this cluster:

```bash
# this environment variable tells kubectl what config file to use
# its value is an arbitrary name, which will be used to name
# the eks cluster configuration file
export KUBECONFIG=~/.kube/eks-config

# this command will populate the eks k8s config file
aws eks update-kubeconfig \
  --name  "${CLUSTER_NAME}"\
  --region "${REGION}"\
  --kubeconfig "${KUBECONFIG}"
```

The output will look like:

>```bash
>Added new context arn:aws:eks:eu-west-1:123456789012:cluster/crossplane-example-cluster to /path/to/.kube/eks-config
>```

Now `kubectl` should be configured to talk to the EKS cluster. To verify this
run:

```bash
kubectl cluster-info
```

Which should produce something like:

>```bash
>Kubernetes master is running at https://12E34567898A607F40B3C2FDDF42DC5.sk1.eu-west-1.eks.amazonaws.com
>```

#### Creating Worker Nodes for the EKS cluster

The worker nodes of an EKS cluster are not managed by the cluster itself.
Instead, a set of EC2 instances, along with other resources and configurations
are needed to be provisioned. In this section, we will create another
CloudFormation stack to setup a worker node configuration, which is described in
[EKS official documentation][sample-workernodes-stack].

Before creating the stack, we will need to create a [Key Pair][aws-key-pair].
This key pair will be used to log into the worker nodes. Even though we don't
need to log into the worker nodes for the purpose of this guide, the worker
stack would fail without providing it.

```bash
# an arbitrary name for the keypair
KEY_PAIR=crossplane-example-kp

# an arbitrary name for the workers stack
# this name has to be unique within an aws account and region
WORKERS_STACK_NAME=crossplane-example-workers

# the id for the AMI image that will be used for worker nodes
# there is a specific id for each region and k8s version
NODE_IMAGE_ID="ami-0497f6feb9d494baf"

# a sample stack that can be used to launch worker nodes
WORKERS_STACK_URL=https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2019-02-11/amazon-eks-nodegroup.yaml

# generate a KeyPair. the output will be the RSA private key value
# which we do not need and will ignore
aws ec2 create-key-pair --key-name "${KEY_PAIR}" --region="${REGION}" > /dev/null

# create the workers stack
aws cloudformation create-stack \
  --template-url="${WORKERS_STACK_URL}" \
  --region "${REGION}" \
  --stack-name "${WORKERS_STACK_NAME}" \
   --capabilities CAPABILITY_IAM \
  --parameters \
    ParameterKey=ClusterName,ParameterValue="${CLUSTER_NAME}" \
    ParameterKey=ClusterControlPlaneSecurityGroup,ParameterValue="${EKS_SECURITY_GROUP}" \
    ParameterKey=NodeGroupName,ParameterValue="crossplane-eks-nodes" \
    ParameterKey=NodeAutoScalingGroupMinSize,ParameterValue=1 \
    ParameterKey=NodeAutoScalingGroupDesiredCapacity,ParameterValue=1 \
    ParameterKey=NodeInstanceType,ParameterValue="m3.medium" \
    ParameterKey=NodeImageId,ParameterValue="${NODE_IMAGE_ID}" \
    ParameterKey=KeyName,ParameterValue="${KEY_PAIR}" \
    ParameterKey=VpcId,ParameterValue="${VPC_ID}" \
    ParameterKey=Subnets,ParameterValue="${SUBNET_IDS//,/\,}"
```

The output will look like:

>```bash
>{
>    "StackId": "arn:aws:cloudformation:eu-west-1:123456789012:stack/aws-service-example-stack 5730d720-d9d9-11e9-8662-029e8e947a9c"
>}
>```

Similar to the VPC stack, creating the workers stack continues in the background
and could take a few minutes to complete. You can check its status by running:

```bash
aws cloudformation describe-stacks --output json --stack-name ${WORKERS_STACK_NAME} --region ${REGION} | jq -r '.Stacks[0].StackStatus'
```

Once the output of the above command is `CREATE_COMPLETE`, all worker nodes are
created. Now we need to tell the EKS cluster to let worker nodes with a certain
role join the cluster. First let's retrieve the worker node role from the stack
we just created: and then add that role to the **aws-auth** config map:

```bash
NODE_INSTANCE_ROLE=$(aws cloudformation describe-stacks --output json --stack-name ${WORKERS_STACK_NAME} --region ${REGION} | jq -r '.Stacks[0].Outputs[]|select(.OutputKey=="NodeInstanceRole").OutputValue')

cat > aws-auth-cm.yaml <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth
  namespace: kube-system
data:
  mapRoles: |
    - rolearn: ${NODE_INSTANCE_ROLE}
      username: system:node:{{EC2PrivateDNSName}}
      groups:
        - system:bootstrappers
        - system:nodes
EOF
```

Then, we will apply this config map to the EKS cluster:

```bash
kubectl apply -f aws-auth-cm.yaml
```

This should print the following output:

>```bash
> configmap/aws-auth created
> ```

Now, you can monitor that worker nodes (in this case only a single node) join
the cluster:

```bash
kubectl get nodes
```

>```bash
> NAME                                            STATUS     ROLES    AGE   VERSION
> ip-192-168-104-194.eu-west-1.compute.internal   NotReady   <none>   8s    v1.14.6-eks-5047ed
>```

After a few minutes, the `NotReady` status above should become `Ready`.

Congratulations! You have successfully setup and configured your EKS cluster!

### Set Up RDS Configurations

In AWS an RDS database instance will be provisioned to satisfy WordPress
application's `MySQLInstanceClass` claim. In this section we create the required
configurations in order to make an RDS instance accessible by the EKS cluster.

#### RDS Security Group

A security group should be created and assigned to the RDS, so certain traffic
from the EKS cluster is allowed.

```bash
# an arbitrary name for the security group
RDS_SG_NAME="crossplane-example-rds-sg"

# generate the security group
aws ec2 create-security-group \
  --vpc-id="${VPC_ID}" \
  --region="${REGION}" \
  --group-name="${RDS_SG_NAME}" \
  --description="open mysql access for crossplane-example cluster"

# retrieve the ID for this security group
RDS_SECURITY_GROUP_ID=$(aws ec2 describe-security-groups --filter Name=group-name,Values="${RDS_SG_NAME}" --region="${REGION}" --output=text --query="SecurityGroups[0].GroupId")
```

After creating the security group, we add a rule to allow traffic on `MySQL`
port

```bash
aws ec2 authorize-security-group-ingress \
  --group-id="${RDS_SECURITY_GROUP_ID}" \
  --protocol=tcp \
  --port=3306 \
  --region="${REGION}" \
  --cidr=0.0.0.0/0  > /dev/null
```

#### RDS Subnet Group

A **DB Subnet Group** is needed to associate the RDS instance with different
subnets and availability zones.

```bash
# an arbitrary name for the db subnet group
DB_SUBNET_GROUP_NAME=crossplane-example-dbsubnetgroup

# convert subnets to a white space separated list
# to satisfy the command input format below
SUBNETS_LIST="${SUBNET_IDS//,/ }"

aws rds create-db-subnet-group \
  --region="${REGION}" \
  --db-subnet-group-name="${DB_SUBNET_GROUP_NAME}" \
  --db-subnet-group-description="crossplane-example db subnet group" \
  --subnet-ids $SUBNETS_LIST > /dev/null
```

These resources will later be used to create cloud-specific MySQL resources.

### Set Up Crossplane

Using the newly provisioned cluster:

1. Install Crossplane from alpha channel. (See the [Crossplane Installation
   Guide][crossplane-install] for more information.)

```bash
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

2. Install the AWS stack into Crossplane. (See the [AWS stack
   section][aws-stack-install] of the install guide for more information.)

```yaml
cat > stack-aws.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: aws
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackRequest
metadata:
  name: stack-aws
  namespace: crossplane-system
spec:
  package: "crossplane/stack-aws:master"
EOF

kubectl apply -f stack-aws.yaml
```

3. Obtain AWS credentials. (See the [Cloud Provider Credentials][cloud-creds]
   docs for more information.)

#### Infrastructure Namespaces

Kubernetes namespaces allow for separation of environments within your cluster.
You may choose to use namespaces to group resources by team, application, or any
other logical distinction. For this guide, we will create a namespace called
`app-project1-dev`, which we will use to group our AWS infrastructure
components.

* Define a `Namespace` in `aws-infra-dev-namespace.yaml` and create it:

  ```yaml
  cat > aws-infra-dev-namespace.yaml <<EOF
  ---
  apiVersion: v1
  kind: Namespace
  metadata:
    name: aws-infra-dev
  EOF

  kubectl apply -f aws-infra-dev-namespace.yam
  ```

* You should see the following output:

> namespace/aws-infra-dev created

#### AWS Provider

It is essential to make sure that the AWS user credentials are configured in
Crossplane as a provider. Please follow the steps [provider
guide][aws-provider-guide] for more information.

#### Cloud-Specific Resource Classes

Cloud-specific resource classes are used to define a reusable configuration for
a specific managed resource. Wordpress requires a MySQL database, which can be
satisfied by an [AWS RDS][aws-rds] instance.

* Define an AWS RDS `RDSInstanceClass` in `aws-mysql-standard.yaml` and
  create it:

```yaml
cat > aws-mysql-standard.yaml <<EOF
---
apiVersion: database.aws.crossplane.io/v1alpha2
kind: RDSInstanceClass
metadata:
  name: aws-mysql-standard
  namespace: aws-infra-dev
specTemplate:
  class: db.t2.small
  masterUsername: masteruser
  securityGroups:
    - "${RDS_SECURITY_GROUP_ID}"
  subnetGroupName: "${RDS_SG_NAME}"
  size: 20
  engine: mysql
  providerRef:
    name: aws-provider
    namespace: crossplane-system
  reclaimPolicy: Delete
EOF

kubectl apply -f aws-mysql-standard.yaml
```

Note that we are using `RDS_SECURITY_GROUP_ID` and `RDS_SG_NAME` variables here,
that we configured earlier.

* You should see the following output:

> rdsinstanceclass.database.aws.crossplane.io/aws-mysql-standard created

* You can verify creation with the following command and output:

```bash
$ kubectl get rdsinstanceclass -n aws-infra-dev
NAME                 PROVIDER-REF    RECLAIM-POLICY   AGE
aws-mysql-standard   aws-provider    Delete           11s
```

You are free to create more AWS `RDSInstanceClass` instances to define more
potential configurations. For instance, you may create `large-aws-rds` with
field `size: 100`.

#### Application Namespaces

Earlier, we created a namespace to group our AWS infrastructure resources.
Because our application resources may be satisfied by services from any cloud
provider, we want to separate them into their own namespace. For this demo, we
will create a namespace called `app-project1-dev`, which we will use to group
our Wordpress resources.

* Define a `Namespace` in `app-project1-dev-namespace.yaml` and create it:

  ```yaml
  cat > app-project1-dev-namespace.yaml <<EOF
  ---
  apiVersion: v1
  kind: Namespace
  metadata:
    name: app-project1-dev
  EOF

  kubectl apply -f app-project1-dev-namespace.yaml
  ```

* You should see the following output:

  > namespace/app-project1-dev created

#### Portable Resource Classes

Portable resource classes are used to define a class of service in a single
namespace for an abstract service type. We want to define our AWS
`RDSInstanceClass` as the standard MySQL class of service in the namespace that
our Wordpress resources will live in.

* Define a `MySQLInstanceClass` in `mysql-standard.yaml` for namespace
  `app-project1-dev` and create it:

  ```yaml
  cat > mysql-standard.yaml <<EOF
  ---
  apiVersion: database.crossplane.io/v1alpha1
  kind: MySQLInstanceClass
  metadata:
    name: mysql-standard
    namespace: app-project1-dev
  classRef:
    kind: RDSInstanceClass
    apiVersion: database.aws.crossplane.io/v1alpha2
    name: aws-mysql-standard
    namespace: aws-infra-dev
  EOF

  kubectl apply -f mysql-standard.yaml
  ```

* You should see the following output:

  > mysqlinstanceclass.database.crossplane.io/mysql-standard created

* You can verify creation with the following command and output:

  ```bash
  $ kubectl get mysqlinstanceclasses -n app-project1-dev
  NAME             AGE
  mysql-standard   27s
  ```

Once again, you are free to create more `MySQLInstanceClass` instances in this
namespace to define more classes of service. For instance, if you created
`mysql-aws-large` above, you may want to create a `MySQLInstanceClass` named
`mysql-large` that references it. You may also choose to create MySQL resource
classes for other non-AWS providers, and reference them for a class of service
in the `app-project1-dev` namespace.

You may specify *one* instance of a portable class kind as *default* in each
namespace. This means that the portable resource class instance will be applied
to claims that do not directly reference a portable class. If we wanted to make
our `mysql-standard` instance the default `MySQLInstanceClass` for namespace
`app-project1-dev`, we could do so by adding a label:

```yaml
---
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: mysql-standard
  namespace: app-project1-dev
  labels:
    default: "true"
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha2
  name: aws-mysql-standard
  namespace: aws-infra-dev
```

#### Resource Claims

Resource claims are used to create external resources by referencing a class of
service in the claim's namespace. When a claim is created, Crossplane uses the
referenced portable class to find a cloud-specific resource class to use as the
configuration for the external resource. We need a to create a claim to
provision the RDS database we will use with AWS.

* Define a `MySQLInstance` claim in `mysql-claim.yaml` and create it:

  ```yaml
  cat > mysql-claim.yaml <<EOF
  apiVersion: database.crossplane.io/v1alpha1
  kind: MySQLInstance
  metadata:
    name: mysql-claim
    namespace: app-project1-dev
  spec:
    classRef:
      name: mysql-standard
    writeConnectionSecretToRef:
      name: wordpressmysql
    engineVersion: "5.6"
  EOF

  kubectl apply -f mysql-claim.yaml
  ```

What we are looking for is for the `STATUS` value to become `Bound` which
indicates the managed resource was successfully provisioned and is ready for
consumption. You can see when claim is bound using the following:

```bash
$ kubectl get mysqlinstances -n app-project1-dev
NAME          STATUS   CLASS            VERSION   AGE
mysql-claim   Bound    mysql-standard   5.6       11m
```

If the `STATUS` is blank, we are still waiting for the claim to become bound.
You can observe resource creation progression using the following:

```bash
$ kubectl describe mysqlinstance mysql-claim -n app-project1-dev
Name:         mysql-claim
Namespace:    app-project1-dev
Labels:       <none>
Annotations:  kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"database.crossplane.io/v1alpha1","kind":"MySQLInstance","metadata":{"annotations":{},"name":"mysql-claim","namespace":"team..."}}
API Version:  database.crossplane.io/v1alpha1
Kind:         MySQLInstance
Metadata:
  Creation Timestamp:  2019-09-16T13:46:42Z
  Finalizers:
    finalizer.resourceclaim.crossplane.io
  Generation:        2
  Resource Version:  4256
  Self Link:         /apis/database.crossplane.io/v1alpha1/namespaces/app-project1-dev/mysqlinstances/mysql-claim
  UID:               6a7fe064-d888-11e9-ab90-42b6bb22213a
Spec:
  Class Ref:
    Name:          mysql-standard
  Engine Version:  5.6
  Resource Ref:
    API Version:  database.aws.crossplane.io/v1alpha2
    Kind:         MysqlServer
    Name:         mysqlinstance-6a7fe064-d888-11e9-ab90-42b6bb22213a
    Namespace:    aws-infra-dev
  Write Connection Secret To Ref:
    Name:  wordpressmysql
Status:
  Conditions:
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Managed claim is waiting for managed resource to become bindable
    Status:                False
    Type:                  Ready
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Successfully reconciled managed resource
    Status:                True
    Type:                  Synced
Events:                    <none>
```

*Note: You must wait until the claim becomes bound before continuing with this
guide. It could take up to a few minutes.*

We referenced our portable `MySQLInstanceClass` directly in the claim above, but
if you specified that `mysql-standard` was the default `MySQLInstanceClass` for
namespace `app-project1-dev`, we could have omitted the claim's `classRef` and
it would automatically be assigned:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: app-project1-dev
spec:
  writeConnectionSecretToRef:
    name: wordpressmysql
  engineVersion: "5.6"
```

## Install Wordpress

Installing Wordpress requires creating a Kubernetes `Deployment` and load
balancer `Service`. We will point the deployment to the `wordpressmysql` secret
that we specified in our claim above for the Wordpress container environment
variables. It should have been populated with our MySQL connection details after
the claim became `Bound`.

* Check to make sure `wordpressmysql` exists and is populated:

  ```bash
  $ kubectl describe secret wordpressmysql -n app-project1-dev
  Name:         wordpressmysql
  Namespace:    app-project1-dev
  Labels:       <none>
  Annotations:  <none>

  Type:  Opaque

  Data
  ====
  endpoint:  75 bytes
  password:  27 bytes
  username:  58 bytes
  ```

* Define the `Deployment` and `Service` in `wordpress-app.yaml` and create it:

  ```yaml
  cat > wordpress-app.yaml <<EOF
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    namespace: app-project1-dev
    name: wordpress
    labels:
      app: wordpress
  spec:
    selector:
      matchLabels:
        app: wordpress
    template:
      metadata:
        labels:
          app: wordpress
      spec:
        containers:
          - name: wordpress
            image: wordpress:4.6.1-apache
            env:
              - name: WORDPRESS_DB_HOST
                valueFrom:
                  secretKeyRef:
                    name: wordpressmysql
                    key: endpoint
              - name: WORDPRESS_DB_USER
                valueFrom:
                  secretKeyRef:
                    name: wordpressmysql
                    key: username
              - name: WORDPRESS_DB_PASSWORD
                valueFrom:
                  secretKeyRef:
                    name: wordpressmysql
                    key: password
            ports:
              - containerPort: 80
                name: wordpress
  ---
  apiVersion: v1
  kind: Service
  metadata:
    namespace: app-project1-dev
    name: wordpress
    labels:
      app: wordpress
  spec:
    ports:
      - port: 80
    selector:
      app: wordpress
    type: LoadBalancer
  EOF

  kubectl apply -f wordpress-app.yaml
  ```

* You can verify creation with the following command and output:

  ```bash
  $ kubectl get -f wordpress-app.yaml
  NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
  deployment.apps/wordpress   1/1     1            1           11m

  NAME                TYPE           CLUSTER-IP    EXTERNAL-IP   PORT(S)        AGE
  service/wordpress   LoadBalancer   10.0.128.30   52.168.69.6   80:32587/TCP   11m
  ```

If the `EXTERNAL-IP` field of the `LoadBalancer` is `<pending>`, wait until it
becomes available, then navigate to the address. You should see the following:

![alt wordpress](wordpress-start.png)

## Uninstall

### Wordpress

All Wordpress components that we installed can be deleted with one command:

```bash
kubectl delete -f wordpress-app.yaml
```

### Crossplane Configuration

To delete all created resources, but leave Crossplane and the AWS stack
running, execute the following commands:

```bash
kubectl delete -f mysql-claim.yaml
kubectl delete -f mysql-standard.yaml
kubectl delete -f aws-mysql-standard.yaml
kubectl delete -f app-project1-dev-namespace.yaml
kubectl delete -f aws-infra-dev-namespace.yaml
```

### AWS Resources 
We will also need to delete the resources that we created for the RDS database
and EKS cluster:

```bash
# delete the db subnet group
aws rds delete-db-subnet-group \
  --region "${REGION}" \
  --db-subnet-group-name="${DB_SUBNET_GROUP_NAME}"

# delete the security group for RDS
aws ec2 delete-security-group \
  --region "${REGION}" \
  --group-id="${RDS_SECURITY_GROUP_ID}"

# delete the CloudFormation Stack for worker nodes
aws cloudformation delete-stack \
  --region "${REGION}" \
  --stack-name "${WORKERS_STACK_NAME}"

# delete the key-pair for worker nodes
aws ec2 delete-key-pair --key-name "${KEY_PAIR}"

# delete the EKS cluster
aws eks delete-cluster \
  --region "${REGION}" \
  --name "${CLUSTER_NAME}"

# detach role policies
aws iam detach-role-policy \
 --role-name "${EKS_ROLE_NAME}" \
 --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy
aws iam detach-role-policy \
 --role-name "${EKS_ROLE_NAME}" \
 --policy-arn arn:aws:iam::aws:policy/AmazonEKSServicePolicy

# delete the cluster role
aws iam delete-role --role-name "${EKS_ROLE_NAME}"

# delete the CloudFormation Stack for vpc
# this should be executed once all previous stes are completed
aws cloudformation delete-stack \
  --region "${REGION}" \
  --stack-name "${VPC_STACK_NAME}"

# delete clusters config file
rm "${KUBECONFIG}"
```

## Conclusion and Next Steps

In this guide we:

* Setup an EKS Cluster using the AWS CLI
* Configured RDS to communicate with EKS
* Installed Crossplane from the alpha channel
* Installed the AWS stack
* Created an infrastructure (`aws-infra-dev`) and application
  (`app-project1-dev`) namespace
* Setup an AWS `Provider` with our account
* Created a `RDSInstanceClass` in the ` with configuration using RDS configuration earlier
* Created a `MySQLInstanceClass` that specified the `RDSInstanceClass` as
  `mysql-standard` in the `app-project1-dev` namespace
* Created a `MySQLInstance` claim in the `app-project1-dev1` namespace that
  referenced `mysql-standard`
* Created a `Deployment` and `Service` to run Wordpress on our EKS Cluster and
  assign an external IP address to it

If you would like to try out a similar workflow using a different cloud
provider, take a look at the other [services guides][services]. If you would like
to learn more about stacks, checkout the [stacks guide][stacks]

<!-- Named links -->
[aws-create-eks]: https://docs.aws.amazon.com/eks/latest/userguide/create-cluster.html
[sample-cf-stack]: https://docs.aws.amazon.com/eks/latest/userguide/create-public-private-vpc.html
[sample-workernodes-stack]: https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
[aws-cli-install]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html
[aws-cli-configure]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
[aws-key-pair]: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html
[install-kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[using-helm]: https://docs.helm.sh/using_helm/
[jq-docs]: https://stedolan.github.io/jq/
[crossplane-install]: ../install-crossplane.md#alpha
[cloud-creds]: ../cloud-providers.md
[aws-provider-guide]: ../cloud-providers/aws/aws-provider.md
[aws-rds]: https://aws.amazon.com/rds/
[services]: ../services-guide.md
[stacks]: ../stacks-guide.md
[aws-stack-install]: ../install-crossplane.md#aws-stack

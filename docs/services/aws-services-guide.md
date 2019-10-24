---
title: Using AWS Services toc: true weight: 430 indent: true
---

# Deploying Wordpress in Amazon Web Services (AWS)

This user guide will walk you through Wordpress application deployment using
Crossplane managed resources and the official Wordpress Docker image.

## Table of Contents

1. [Pre-requisites](#pre-requisites)
1. [Preparation](#preparation)
1. [Create Configuration](#create-configuration)
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
[configured][aws-cli-configure]. It also assumes an existing EKS cluster,
configured in a VPC with three public subnets (i.e. exposed to the internet).

In order to utilize these pre-existing resources, set environment variables that
can be used when creating resources necessary to deploy Wordpress.

```bash
export CLUSTER_NAME=yourclustername
export REGION=yourclusterregion
export VPC_ID=yourvpcid
export SUBNET_ONE_ID=yourpublicsubnetoneid
export SUBNET_TWO_ID=yourpublicsubnettwoid
export SUBNET_THREE_ID=yourpublicsubnetthreeid
```

#### Configuring `kubectl` to communicate with the EKS cluster

In order to communicate with with your EKS cluster, you must configure
`kubectl`.

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

### Set Up Crossplane

Using the EKS cluster:

1. Install Crossplane from alpha channel. (See the [Crossplane Installation
   Guide][crossplane-install] for more information.)

```bash
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

2. Install the AWS stack into Crossplane. (See the [AWS stack
   section][aws-stack-install] of the install guide for more information.)

```bash
cat > stack-aws.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: aws
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
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

### Create AWS Provider

It is essential to make sure that the AWS user credentials are configured in
Crossplane as a provider. Please follow the steps [provider
guide][aws-provider-guide] for more information.

## Create Configuration

To keep your resource configuration organized, start by creating a new
directory:

```bash
mkdir wordpress && cd $_
```

### RDS

#### Network Resources

Before you setup an RDS instance, you will need to create a subnet group for it
to be provisioned into, as well as a security group to determine how it can be
accessed

* Define an AWS `DBSubnetGroup` in `aws-dbsubnet.yaml` and create it:

```bash
cat > aws-dbsubnet.yaml <<EOF
apiVersion: storage.aws.crossplane.io/v1alpha2 # change me!
kind: DBSubnetGroup
metadata:
  name: sample-dbsubnetgroup
spec:
  groupName: sample_dbsubnetgroup
  description: EKS vpc to rds
  subnetIds:
    - ${SUBNET_ONE_ID}
    - ${SUBNET_TWO_ID}
    - ${SUBNET_THREE_ID}
  tags:
    - key: name
      value: sample-dbsubnetgroup
  reclaimPolicy: Delete
  providerRef:
    name: aws-provider
EOF

kubectl apply -f aws-dbsubnet.yaml
```

* Define an AWS `SecurityGroup` in `aws-sg.yaml` and create it:

```bash
cat > aws-sg.yaml <<EOF
apiVersion: network.aws.crossplane.io/v1alpha2
kind: SecurityGroup
metadata:
  name: sample-rds-sg
spec:
  vpcId: ${VPC_ID}
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
EOF

kubectl apply -f aws-sg.yaml
```

#### Cloud-Specific Resource Classes

Cloud-specific resource classes are used to define a reusable configuration for
a specific managed resource. Wordpress requires a MySQL database, which can be
satisfied by an [AWS RDS][aws-rds] instance.

* Define an AWS RDS `RDSInstanceClass` in `aws-mysql-standard.yaml` and create
  it:

```yaml
cat > aws-mysql-standard.yaml <<EOF
apiVersion: database.aws.crossplane.io/v1alpha2
kind: RDSInstanceClass
metadata:
  name: standard-mysql
  annotations:
    resourceclass.crossplane.io/is-default-class: "true"
specTemplate:
  class: db.t2.small
  masterUsername: masteruser
  securityGroupRefs:
    - name: sample-rds-sg
  subnetGroupNameRef:
    name: sample-dbsubnetgroup
  size: 20
  engine: mysql
  providerRef:
    name: aws-provider
  reclaimPolicy: Delete
  writeConnectionSecretsToNamespace: crossplane-system
EOF

kubectl apply -f mysql-standard.yaml
```

Note that we are referencing the security group and subnet group we created
earlier.

* You should see the following output:

> rdsinstanceclass.database.aws.crossplane.io/aws-mysql-standard created

* You can verify creation with the following command and output:

```bash
$ kubectl get rdsinstanceclasses.database.aws.crossplane.io
NAME                 PROVIDER-REF    RECLAIM-POLICY   AGE
standard-mysql       aws-provider    Delete           11s
```

You are free to create more AWS `RDSInstanceClass` instances to define more
potential configurations. For instance, you may create `large-aws-rds` with
field `size: 100`.

#### Resource Claims

Resource claims are used to create external resources by binding to a resource
class or a managed resource. This can be accomplished in a variety of ways
including referencing the class or managed resource directly, providing labels
that are used to match to a class, or by defaulting to a class that is annotated
with `resourceclass.crossplane.io/is-default-class: "true"`. In the
`RDSInstanceClass` above, we added the default annotation, so our claim will
bind to it automatically if no other classes exist with annotation. If there are
multiple classes annotated as default, one will be chosen at random.

* Define a `MySQLInstance` claim in `mysql-claim.yaml` and create it:

```bash
cat > mysql-claim.yaml <<EOF
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
spec:
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
$ kubectl get mysqlinstances
NAME          STATUS   CLASS-KIND         CLASS-NAME       RESOURCE-KIND   RESOURCE-NAME               AGE
mysql-claim            RDSInstanceClass   standard-mysql   RDSInstance     default-mysql-claim-5p66w   9s
```

If the `STATUS` is blank, we are still waiting for the claim to become bound.
You can observe resource creation progression using the following:

```bash
$ kubectl describe mysqlinstance mysql-claim
Name:         mysql-claim
Namespace:    default
Labels:       <none>
Annotations:  kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"database.crossplane.io/v1alpha1","kind":"MySQLInstance","metadata":{"annotations":{},"name":"mysql-claim","namespace":"defa...
API Version:  database.crossplane.io/v1alpha1
Kind:         MySQLInstance
Metadata:
  Creation Timestamp:  2019-10-24T19:59:18Z
  Finalizers:
    finalizer.resourceclaim.crossplane.io
  Generation:        3
  Resource Version:  6425
  Self Link:         /apis/database.crossplane.io/v1alpha1/namespaces/default/mysqlinstances/mysql-claim
  UID:               c3aca763-f698-11e9-a957-12a4af141bea
Spec:
  Class Ref:
    API Version:   database.aws.crossplane.io/v1alpha2
    Kind:          RDSInstanceClass
    Name:          standard-mysql
    UID:           6cf90617-f698-11e9-b058-028a0ecde201
  Engine Version:  5.6
  Resource Ref:
    API Version:  database.aws.crossplane.io/v1beta1
    Kind:         RDSInstance
    Name:         app-project1-dev-mysql-claim-8shd2
  Write Connection Secret To Ref:
    Name:  wordpressmysql
Status:
  Conditions:
    Last Transition Time:  2019-10-24T19:59:20Z
    Reason:                Managed claim is waiting for managed resource to become bindable
    Status:                False
    Type:                  Ready
    Last Transition Time:  2019-10-24T19:59:20Z
    Reason:                Successfully reconciled managed resource
    Status:                True
    Type:                  Synced
Events:                    <none>
```

## Install Wordpress

Installing Wordpress requires creating a Kubernetes `Deployment` and load
balancer `Service`. We will point the deployment to the `wordpressmysql` secret
that we specified in our claim above for the Wordpress container environment
variables. It should have been populated with our MySQL connection details after
the claim became `Bound`.

* Check to make sure `wordpressmysql` exists and is populated:

```bash
$ kubectl describe secret wordpressmysql
Name:         wordpressmysql
Namespace:    default
Labels:       <none>
Annotations:  crossplane.io/propagate-from-name: c3aca763-f698-11e9-a957-12a4af141bea
            crossplane.io/propagate-from-namespace: crossplane-system
            crossplane.io/propagate-from-uid: c539fcef-f698-11e9-a957-12a4af141bea

Type:  Opaque

Data
====
endpoint:  83 bytes
password:  27 bytes
username:  10 bytes
```

* Define the `Deployment` and `Service` in `wordpress-app.yaml` and create it:

```bash
cat > wordpress-app.yaml <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
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

```bash
kubectl delete -f wordpress/
```

## Conclusion and Next Steps

In this guide we:

* Setup an EKS Cluster using the AWS CLI
* Configured RDS to communicate with EKS
* Installed Crossplane from the alpha channel
* Installed the AWS stack
* Setup an AWS `Provider` with our account
* Created a `RDSInstanceClass` with configuration for an AWS RDS instance
* Created a `MySQLInstance` claim that defaulted to the `mysql-standard`
  resource class
* Created a `Deployment` and `Service` to run Wordpress on our EKS Cluster and
  assign an external IP address to it

If you would like to try out a similar workflow using a different cloud
provider, take a look at the other [services guides][services]. If you would
like to learn more about stacks, checkout the [stacks guide][stacks].

<!-- Named links -->
[aws-cli-install]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html
[aws-cli-configure]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
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

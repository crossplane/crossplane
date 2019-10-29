---
title: Using Azure Services
toc: true
weight: 440
indent: true
---

# Azure Services Guide

This user guide will walk you through Wordpress application deployment using
Crossplane managed resources and the official Wordpress Docker image.

## Table of Contents

1. [Pre-requisites](#pre-requisites)
1. [Preparation](#preparation)
1. [Set Up Crossplane](#set-up-crossplane)
    1. [Install in Target Cluster](#install-in-target-cluster)
    1. [Cloud Provider](#cloud-provider)
    1. [Resource Classes](#resource-classes)
    1. [Configure Managed Service Access](#configure-managed-service-access)
1. [Provision MySQL](#provision-mysql)
   1. [Resource Claim](#resource-claim)
   1. [Virtual Network Rule](#virtual-network-rule)
1. [Install Wordpress](#install-wordpress)
1. [Clean Up](#clean-up)
1. [Conclusion and Next Steps](#conclusion-and-next-steps)

## Pre-requisites

These tools are required to complete this guide. They must be installed on your
local machine.

* [Azure CLI][azure-cli]
    * Make sure to [login][azure-login] after installation.
* [kubectl][install-kubectl]
    * kubectl also be installed using the Azure CLI: `az aks install-cli`
* [Helm][using-helm], minimum version `v2.10.0+`.
* [jq][jq-docs] - command line JSON processor `v1.5+`


## Preparation

This guide assumes that you have setup the Azure CLI and are logged in to your
desired account. It also assumes that you have an existing AKS cluster in a
Virtual Network. Make sure to populate the environment variables below with the
relevant values for your AKS cluster.

*Note: environment variables are used throughout this guide.*

```bash
export AKS_RESOURCE_GROUP=myAKSResourceGroup
export AKS_VNET=myAKSVnet
export AKS_NAME=myAKSName
export AKS_REGION=myRegion
export SUBSCRIPTION_ID=$(az account list | jq -j '.[0].id')
```

## Set Up Crossplane

### Install in Target Cluster

Assuming you are [connected][aks-kubectl] to your AKS cluster via `kubectl`:

1. Install Crossplane from alpha channel. (See the [Crossplane Installation
   Guide][crossplane-install] for more information.)

```bash
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

2. Install the Azure stack into Crossplane. (See the [Azure stack
   section][azure-stack-install] of the install guide for more information.)

```yaml
cat > stack-azure.yaml <<EOF
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-azure
  namespace: crossplane-system
spec:
  package: "crossplane/stack-azure:v0.2.0"
EOF

kubectl apply -f stack-azure.yaml
```

3. Obtain Azure credentials. (See the [Cloud Provider Credentials][cloud-creds]
   docs for more information.)

### Cloud Provider

It is essential to make sure that the Azure user credentials are configured in
Crossplane as a provider. Please follow the steps in the Azure [provider
guide][azure-provider-guide] for more information.

###  Resource Classes

To keep your resource configuration organized, start by creating a new
directory:

```bash
mkdir wordpress && cd $_
```

Resource classes are used to define a reusable configuration for a specific
managed service. Wordpress requires a MySQL database, which can be satisfied by
an [Azure Database for MySQL][azure-mysql] instance.

* Define an Azure MySQL `SQLServerClass` in `azure-mysql-standard.yaml` and
  create it:

```yaml
cat > azure-mysql-standard.yaml <<EOF
---
apiVersion: database.azure.crossplane.io/v1alpha3
kind: SQLServerClass
metadata:
  name: azure-mysql-standard
  labels:
    size: standard
    demo: "true"
specTemplate:
  adminLoginName: myadmin
  resourceGroupName: $AKS_RESOURCE_GROUP
  location: $AKS_REGION
  sslEnforced: false
  version: "5.6"
  pricingTier:
    tier: GeneralPurpose
    vcores: 2
    family: Gen5
  storageProfile:
    storageGB: 25
    backupRetentionDays: 7
    geoRedundantBackup: false
  writeConnectionSecretsToNamespace: crossplane-system
  providerRef:
    name: azure-provider
  reclaimPolicy: Delete
EOF

kubectl apply -f azure-mysql-standard.yaml
```

* You should see the following output:

> sqlserverclass.database.azure.crossplane.io/azure-mysql-standard created


* You can verify creation with the following command and output:

```bash
$ kubectl get sqlserverclasses
NAME                   PROVIDER-REF     RECLAIM-POLICY   AGE
azure-mysql-standard   azure-provider   Delete           17s
```

You are free to create more Azure `SQLServerClass` instances to define more
potential configurations. For instance, you may create `large-azure-mysql` with
field `storageGB: 100`.

### Configure Managed Service Access

In order for the AKS cluster to talk to the MySQL Database, you must condigure a
`Microsoft.Sql` service endpoint on the AKS Virtual Network for all subnets. If
you do not already have this configured, Azure has a [guide][service endpoint]
on how to set it up.

## Provision MySQL

### Resource Claims

Resource claims are used for dynamic provisioning of a managed resource (like a
MySQL instance) by matching the claim to a resource class. This can be done in
several ways: (a) rely on the default class marked
`resourceclass.crossplane.io/is-default-class: "true"`, (b) use a
`claim.spec.classRef` to a specific class, or (c) match on class labels using a
`claim.spec.classSelector`.

*Note: claims may also be used in [static provisioning] with a reference to an
existing managed resource.*

In the `SQLServerClass` above, we added the labels `size: standard` and `demo:
"true"`, so our claim will be scheduled to that class using the labels are
specified in the `claim.spec.classSelector`. If there are multiple classes which
match the specified label(s) one will be chosen at random.

* Define a `MySQLInstance` claim in `mysql-claim.yaml` and create it:

```yaml
cat > mysql-claim.yaml <<EOF
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
spec:
  classSelector:
    matchLabels:
      size: standard
      demo: "true"
  engineVersion: "5.6"
  writeConnectionSecretToRef:
    name: wordpressmysql
EOF

kubectl apply -f mysql-claim.yaml
```

What we are looking for is for the `STATUS` value to become `Bound` which
indicates the managed resource was successfully provisioned and is ready for
consumption. You can see when claim is bound using the following:

```bash
$ kubectl get mysqlinstances
NAME          STATUS   CLASS-KIND       CLASS-NAME             RESOURCE-KIND   RESOURCE-NAME               AGE
mysql-claim   Bound    SQLServerClass   azure-mysql-standard   MySQLServer     default-mysql-claim-bm4ft   9s
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
  Creation Timestamp:  2019-10-28T15:43:28Z
  Finalizers:
    finalizer.resourceclaim.crossplane.io
  Generation:        3
  Resource Version:  11072
  Self Link:         /apis/database.crossplane.io/v1alpha1/namespaces/default/mysqlinstances/mysql-claim
  UID:               afff42b3-f999-11e9-a2d5-c64d758a651f
Spec:
  Class Ref:
    API Version:  database.azure.crossplane.io/v1alpha3
    Kind:         SQLServerClass
    Name:         azure-mysql-standard
    UID:          5710f3db-f999-11e9-a2d5-c64d758a651f
  Class Selector:
    Match Labels:
      Demo:        true
      Size:        standard
  Engine Version:  5.6
  Resource Ref:
    API Version:  database.azure.crossplane.io/v1alpha3
    Kind:         MySQLServer
    Name:         default-mysql-claim-bm4ft
    UID:          b02c1389-f999-11e9-a2d5-c64d758a651f
  Write Connection Secret To Ref:
    Name:  wordpressmysql
Status:
  Conditions:
    Last Transition Time:  2019-10-28T15:43:29Z
    Reason:                Managed claim is waiting for managed resource to become bindable
    Status:                False
    Type:                  Ready
    Last Transition Time:  2019-10-28T15:43:29Z
    Reason:                Successfully reconciled managed resource
    Status:                True
    Type:                  Synced
Events:                    <none>
```

*Note: You must wait until the claim becomes bound before continuing with this
guide. It could take a few minutes for Azure to complete MySQL creation.*

### Virtual Network Rule

Before we install Wordpress, we need establish connectivity between our MySQL
database and our AKS cluster. We can do this by creating a [Virtual Network
Rule][azure-vnet-rule].

* Set `MYSQL_NAME` environment variable:

```bash
export MYSQL_NAME=$(kubectl get -o json mysqlinstance mysql-claim | jq -j '.spec.resourceRef.name')
```

* Define a `MySQLServerVirtualNetworkRule` in `wordpress-vnet-rule.yaml` and
  create it:

```yaml
cat > wordpress-vnet-rule.yaml <<EOF
---
apiVersion: database.azure.crossplane.io/v1alpha3
kind: MySQLServerVirtualNetworkRule
metadata:
  name: wordpress-vnet-rule
spec:
  name: wordpress-vnet-rule
  serverName: ${MYSQL_NAME}
  resourceGroupName: ${AKS_RESOURCE_GROUP}
  properties:
    virtualNetworkSubnetId: /subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${AKS_RESOURCE_GROUP}/providers/Microsoft.Network/virtualNetworks/${AKS_VNET}/subnets/aks-subnet
  providerRef:
    name: azure-provider
  reclaimPolicy: Delete
EOF

kubectl apply -f wordpress-vnet-rule.yaml
```

* You can verify creation with the following command and output:

```bash
$ kubectl get mysqlservervirtualnetworkrules
NAME                  STATE   AGE
wordpress-vnet-rule   Ready   17s
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
Annotations:  crossplane.io/propagate-from-name: 084b9476-f99e-11e9-a2d5-c64d758a651f
              crossplane.io/propagate-from-namespace: crossplane-system
              crossplane.io/propagate-from-uid: 2e71f6f9-f99e-11e9-a2d5-c64d758a651f

Type:  Opaque

Data
====
endpoint:  50 bytes
password:  27 bytes
username:  33 bytes
```

* Define the `Deployment` and `Service` in `wordpress-app.yaml` and create it:

```yaml
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


## Clean Up

Because we put all of our configuration in a single directory, we can delete it
all with this command:

```bash
kubectl delete -f wordpress/
```

If you would like to also uninstall Crossplane and the AWS stack, run the
following command:

```bash
kubectl delete namespace crossplane-system
```

## Conclusion and Next Steps

In this guide we:

* Installed Crossplane from alpha channel
* Installed the Azure stack
* Setup an Azure `Provider` with our account
* Created a `SQLServerClass` in the ` with configuration for a MySQL database on
  Azure
* Created a `MySQLInstance` claim in the that was scheduled to the
  `mysql-standard` resource class
* Created a `MySQLServerVirtualNetworkRule` to establish secure connectivity
  between our AKS Cluster and MySQL database
* Created a `Deployment` and `Service` to run Wordpress on our AKS Cluster and
  assign an external IP address to it

If you would like to try out a similar workflow using a different cloud
provider, take a look at the other [services guides][services]. If you would
like to learn more about stacks, checkout the [stacks guide][stacks].

<!-- Named links -->
[azure-cli]: https://docs.microsoft.com/en-us/cli/azure/?view=azure-cli-latest
[azure-login]: https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli?view=azure-cli-latest
[install-kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[using-helm]: https://docs.helm.sh/using_helm/
[jq-docs]: https://stedolan.github.io/jq/
[service endpoint]: https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-service-endpoint-policies-overview
[aks-kubectl]: https://docs.microsoft.com/en-us/azure/aks/kubernetes-walkthrough#connect-to-the-cluster

[crossplane-install]: ../install-crossplane.md#alpha
[azure-stack-install]: ../install-crossplane.md#azure-stack
[cloud-creds]: ../cloud-providers.md

[azure-provider-guide]: ../cloud-providers/azure/azure-provider.md

[azure-mysql]: https://azure.microsoft.com/en-us/services/mysql/
[azure-vnet-rule]: https://docs.microsoft.com/en-us/azure/mysql/concepts-data-access-and-security-vnet
[static provisioning]: ../concepts.md#dynamic-and-static-provisioning

[services]: ../services-guide.md
[stacks]: ../stacks-guide.md

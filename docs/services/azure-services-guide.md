---
title: Using Azure Services toc: true weight: 440 indent: true
---
# Deploying Wordpress in Azure

This user guide will walk you through Wordpress application deployment using
Crossplane managed resources and the official Wordpress Docker image.

## Table of Contents

1. [Pre-requisites](#pre-requisites)
2. [Preparation](#preparation)
3. [Set Up Crossplane](#set-up-crossplane)
4. [Install Wordpress](#install-wordpress)
5. [Clean Up](#clean-up)
6. [Conclusion and Next Steps](#conclusion-and-next-steps)

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
Virtual Network with the `Microsoft.Sql` [service endpoint] enabled. Make sure
to populate the environment variables below with the relevant values for your
AKS cluster.

*Note: environment variables are used throughout this guide.*

```bash
export AKS_RESOURCE_GROUP=myAKSResourceGroup
export AKS_VNET=myAKSVnet
export AKS_NAME=myAKSName
export SUBSCRIPTION_ID=$(az account list | jq -j '.[0].id')
```

### Connect to AKS Cluster

You can connect to your AKS cluster with the following command:

```bash
az aks get-credentials --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME
```

Make sure `kubectl` is able to communicate with AKS cluster with the following
command:

```bash
kubectl cluster-info
```

### Set Up Crossplane

Using the newly provisioned cluster:

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
  package: "crossplane/stack-azure:master"
EOF

kubectl apply -f stack-azure.yaml
```

3. Obtain Azure credentials. (See the [Cloud Provider Credentials][cloud-creds]
   docs for more information.)

#### Azure Provider

It is essential to make sure that the Azure Service Principal is configured with
all permissions outlined in the [provider guide][azure-provider-guide].

Using Azure Service Principal `crossplane-azure-provider-key.json`:

* Generate BASE64ENCODED_AZURE_PROVIDER_CREDS encoded value:

```bash
export BASE64ENCODED_AZURE_PROVIDER_CREDS=$(base64 crossplane-azure-provider-key.json | tr -d "\n")
```

* Define an Azure `Provider` and `Secret` in `azure-provider.yaml` and create
  them:

```yaml
cat > azure-provider.yaml <<EOF
---
# Azure Admin service account secret - used by Azure Provider
apiVersion: v1
kind: Secret
metadata:
  name: demo-provider-azure-dev
  namespace: crossplane-system
type: Opaque
data:
  credentials: $BASE64ENCODED_AZURE_PROVIDER_CREDS
---
# Azure Provider with service account secret reference - used to provision resources
apiVersion: azure.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: demo-azure
spec:
  credentialsSecretRef:
    name: demo-provider-azure-dev
    namespace: crossplane-system
    key: credentials
EOF

kubectl apply -f azure-provider.yaml
```

* Verify Azure provider was successfully registered by the crossplane

```bash
kubectl get providers.azure.crossplane.io
kubectl get secrets -n crossplane-system
```

####  Resource Classes

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
apiVersion: database.azure.crossplane.io/v1alpha2
kind: SQLServerClass
metadata:
  name: azure-mysql-standard
  labels:
    app: wordpress
    demo: true
specTemplate:
  adminLoginName: myadmin
  resourceGroupName: $AKS_RESOURCE_GROUP
  location: EAST US
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
  providerRef:
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
EOF

kubectl apply -f azure-mysql-standard.yaml
```

* You should see the following output:

> sqlserverclass.database.azure.crossplane.io/azure-mysql-standard created


* You can verify creation with the following command and output:

```bash
$ kubectl get sqlserverclasses -n azure-infra-dev
NAME                   PROVIDER-REF   RECLAIM-POLICY   AGE
azure-mysql-standard   demo-azure     Delete           11s
```

You are free to create more Azure `SQLServerClass` instances to define more
potential configurations. For instance, you may create `large-azure-mysql` with
field `storageGB: 100`.

#### Resource Claims

Resource claims are used to create external resources by being scheduled to a
resource class and creating new managed resource or binding to an existing
managed resource directly. This can be accomplished in a variety of ways
including referencing the class or managed resource directly, providing labels
that are used to match to a class, or by defaulting to a class that is annotated
with `resourceclass.crossplane.io/is-default-class: "true"`. In the
`SQLServerClass` above, we added the label `app: wordpress`, so our claim will
be scheduled to that class the labels are specified in the `classSelector`. If
there are multiple classes which match the specified label(s) one will be chosen
at random.

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
        app: wordpress
        demo: true
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
NAME          STATUS   CLASS            VERSION   AGE
mysql-claim   Bound    mysql-standard   5.6       11m
```

If the `STATUS` is blank, we are still waiting for the claim to become bound.
You can observe resource creation progression using the following:

```bash
$ kubectl describe mysqlinstance mysql-claim
Name:         mysql-claim
Namespace:    default
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
    API Version:  database.azure.crossplane.io/v1alpha2
    Kind:         MySQLServer
    Name:         mysqlinstance-6a7fe064-d888-11e9-ab90-42b6bb22213a
    Namespace:    azure-infra-dev
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
guide. It could take a few minutes for Azure to complete MySQL creation.*

#### Virtual Network Rule

Before we install Wordpress, we need establish connectivity between our MySQL
database and our AKS cluster. We can do this by creating a [Virtual Network
Rule][azure-vnet-rule].

* Set `MYSQL_NAME` environment variable:

```bash
export MYSQL_NAME=$(kubectl get -o json mysqlinstance mysql-claim -n app-project1-dev | jq -j '.spec.resourceRef.name')
```

* Define a `MySQLServerVirtualNetworkRule` in `wordpress-vnet-rule.yaml` and
  create it:

```yaml
cat > wordpress-vnet-rule.yaml <<EOF
---
apiVersion: database.azure.crossplane.io/v1alpha2
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
    name: demo-azure
    namespace: azure-infra-dev
  reclaimPolicy: Delete
EOF

kubectl apply -f wordpress-vnet-rule.yaml
```

* You can verify creation with the following command and output:

```bash
kubectl get mysqlservervirtualnetworkrules
NAME                  AGE
wordpress-vnet-rule   27s
```

## Install Wordpress

Installing Wordpress requires creating a Kubernetes `Deployment` and load
balancer `Service`. We will point the deployment to the `wordpressmysql` secret
that we specified in our claim above for the Wordpress container environment
variables. It should have been populated with our MySQL connection details after
the claim became `Bound`.

* Check to make sure `wordpressmysql` exists and is populated:

```bash
$ kubectl describe secret wordpressmysql -n default
Name:         wordpressmysql
Namespace:    default
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
* Created a `MysqlServerVirtualNetworkRule` to establish secure connectivity
  between our AKS Cluster and MySQL database
* Created a `Deployment` and `Service` to run Wordpress on our AKS Cluster and
  assign an external IP address to it

If you would like to try out a similar workflow using a different cloud
provider, take a look at the other [services guides][services]. If you would
like to learn more about stacks, checkout the [stacks guide][stacks]

<!-- Named links -->
[azure-cli]: https://docs.microsoft.com/en-us/cli/azure/?view=azure-cli-latest
[azure-login]: https://docs.microsoft.com/en-us/cli/azure/authenticate-azure-cli?view=azure-cli-latest
[install-kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl/
[using-helm]: https://docs.helm.sh/using_helm/
[jq-docs]: https://stedolan.github.io/jq/
[service endpoint]: https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-service-endpoint-policies-overview

[crossplane-install]: ../install-crossplane.md#alpha
[azure-stack-install]: ../install-crossplane.md#azure-stack
[cloud-creds]: ../cloud-providers.md

[azure-provider-guide]: ../cloud-providers/azure/azure-provider.md

[azure-mysql]: https://azure.microsoft.com/en-us/services/mysql/
[azure-vnet-rule]: https://docs.microsoft.com/en-us/azure/mysql/concepts-data-access-and-security-vnet

[services]: ../services-guide.md
[stacks]: ../stacks-guide.md

# Deploying Wordpress in Azure

This user guide will walk you through Wordpress application deployment using Crossplane managed resources and
the official Wordpress Helm chart.


## Pre-requisites

* [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/?view=azure-cli-latest)
    * Make sure to [login](https://docs.microsoft.com/en-us/cli/azure/get-started-with-azure-cli?view=azure-cli-latest) after installation.
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
    * kubectl also be installed using the Azure CLI: `az aks install-cli`
* [Helm](https://docs.helm.sh/using_helm/), minimum version `v2.10.0+`.
* [jq](https://stedolan.github.io/jq/) - commandline JSON processor `v1.5+`


## Preparation
This guide assumes that you have setup the Azure CLI and are logged in to your desired account.

*Note: environment variables are used throughout this guide. You may use the values below or create your own.*

```bash
export RESOURCE_GROUP_NAME=myResourceGroup
export RESOURCE_GROUP_LOCATION=eastus
export AKS_NAME=myAKSCluster
export AKS_NODE_COUNT=1
export AKS_RESOURCE_GROUP=MC_${RESOURCE_GROUP_NAME}_${AKS_NAME}_${RESOURCE_GROUP_LOCATION}
export SUBSCRIPTION_ID=$(az account list | jq -j '.[0].id')
``` 

### AKS Cluster
Azure maintains a succinct [walkthrough](https://docs.microsoft.com/en-us/azure/aks/kubernetes-walkthrough) for setting up an AKS cluster using the Azure CLI. The basic steps are as follows:

1. Create a Resource Group
```bash
az group create --name $RESOURCE_GROUP_NAME --location $RESOURCE_GROUP_LOCATION
```

2. Create AKS Cluster (this may take a few minutes)
```bash
az aks create \
    --resource-group $RESOURCE_GROUP_NAME \
    --name $AKS_NAME \
    --node-count $AKS_NODE_COUNT \
    --enable-addons monitoring \
    --generate-ssh-keys
```

3. Enable SQL Service Endpoint

Get name of AKS node Virtual Network:
```bash
export AKS_VNET=$(az network vnet list -g $AKS_RESOURCE_GROUP | jq -j '.[0].name')
```

Add Service Endpoint to AKS subnet:
```bash
az network vnet subnet update -g $AKS_RESOURCE_GROUP --vnet-name $AKS_VNET -n aks-subnet --service-endpoints Microsoft.Sql
```

4. Connect to AKS Cluster 
```bash
az aks get-credentials --resource-group $RESOURCE_GROUP_NAME --name $AKS_NAME
```

5. Make sure `kubectl` is able to communicate with AKS Cluster
```bash
kubectl get pods --all-namespaces
```

### Crossplane
Using the newly provisioned cluster:

* Install Crossplane from master channel using the [Crossplane Installation Guide](../install-crossplane.md#master)
* Install the Azure stack into Crossplane using the [Azure stack section](../install-crossplane.md#azure-stack) of the install guide.
* Obtain [Cloud Provider Credentials](../cloud-providers.md)

#### Azure Provider
It is essential to make sure that the Azure Service Principal is configured with all permissions outlined in the [provider guide](../cloud-providers/azure/azure-provider.md).

Using Azure Service Principal `crossplane-azure-provider-key.json`:
* Generate BASE64ENCODED_AZURE_PROVIDER_CREDS encoded value:
```bash
export BASE64ENCODED_AZURE_PROVIDER_CREDS=$(base64 crossplane-azure-provider-key.json | tr -d "\n")
```

* Define an Azure `Provider` and `Secret` in `azure-provider.yaml`:
```yaml
---
# Azure Admin service account secret - used by Azure Provider
apiVersion: v1
kind: Secret
metadata:
  name: demo-provider-azure
  namespace: azure
type: Opaque
data:
  credentials: BASE64ENCODED_AZURE_PROVIDER_CREDS
---
# Azure Provider with service account secret reference - used to provision resources
apiVersion: azure.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: demo-azure
  namespace: azure
spec:
  credentialsSecretRef:
    name: demo-provider-azure
    key: credentials
```

#### Create Provider
* Create Azure provider:
```bash
sed "s/BASE64ENCODED_AZURE_PROVIDER_CREDS/$BASE64ENCODED_AZURE_PROVIDER_CREDS/g" azure-provider.yaml | kubectl create -f -
```

* Verify Azure provider was successfully registered by the crossplane
```bash
kubectl get providers.azure.crossplane.io -n azure
kubectl get secrets -n azure
```

#### Cloud-Specific Resource Classes
Cloud-specific resource classes are used to define a reusable configuration for a specific managed service. Wordpress requires a MySQL database, which can be satisfied by an [Azure Database for MySQL](https://azure.microsoft.com/en-us/services/mysql/) instance.

* Define an Azure MySQL `SQLServerClass` in `azure-mysql-class.yaml`:
```yaml
---
apiVersion: database.azure.crossplane.io/v1alpha2
kind: SQLServerClass
metadata:
  name: standard-azure-mysql
  namespace: azure
specTemplate:
  adminLoginName: myadmin
  resourceGroupName: RESOURCE_GROUP_NAME
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
    namespace: azure
  reclaimPolicy: Delete
```

* Create the `SQLServerClass`:
```bash
sed "s/RESOURCE_GROUP_NAME/$RESOURCE_GROUP_NAME/g" azure-mysql-class.yaml | kubectl apply -f -
```

* You should see the following output:
```bash
sqlserverclass.database.azure.crossplane.io/standard-azure-mysql created
```

* You can verify creation with the following command and output:

*Command*
```bash
kubectl get sqlserverclasses -n azure
```
*Output*
```bash
NAME                   PROVIDER-REF   RECLAIM-POLICY   AGE
standard-azure-mysql   demo-azure     Delete           11s
```

You are free to create more Azure `SQLServerClass` instances to define more potential configurations. For instance, you may create `large-azure-mysql` with field `storageGB: 100`.

#### Namespaces
Kubernetes namespaces allow for separation of environments within your cluster. You may choose to use namespaces to group resources by team, application, or any other logical distinction. For this demo, we will create a namespace called `app-project1-prod`, which we will use to group our Wordpress resources.

* Define a `Namespace` in `app-project1-prod-namespace.yaml`:
```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: app-project1-prod
```

* Create the `Namespace`:
```bash
kubectl create -f app-project1-prod-namespace.yaml
```

* You should see the following output:
```bash
namespace/app-project1-prod created
```

#### Portable Resource Classes
Portable resource classes are used to define a class of service in a single namespace for an abstract service type. We want to define our Azure `SQLServerClass` as the standard MySQL class of service in the namespace that our Wordpress resources will live in.

* Define a `MySQLInstanceClass` in `mysql-class.yaml` for namespace `app-project1-prod`:
```yaml
---
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: standard-mysql
  namespace: app-project1-prod
classRef:
  kind: SQLServerClass
  apiVersion: database.azure.crossplane.io/v1alpha2
  name: standard-azure-mysql
  namespace: azure
```

* Create the `MySQLInstanceClass`:
```bash
kubectl apply -f mysql-class.yaml
```

* You should see the following output:
```bash
mysqlinstanceclass.database.crossplane.io/standard-mysql created
```

* You can verify creation with the following command and output:

*Command*
```bash
kubectl get mysqlinstanceclasses -n app-project1-prod
```
*Output*
```bash
NAME             AGE
standard-mysql   27s
```

Once again, you are free to create more `MySQLInstanceClass` instances in this namespace to define more classes of service. For instance, if you created `large-azure-mysql` above, you may want to create a `MySQLInstanceClass` named `large-mysql` that references. You may also choose to create MySQL resource classes for other non-Azure providers, and reference them for a class of service in the `app-project1-prod` namespace.

You may specify *one* instance of a portable class kind as *default* in each namespace. This means that the portable resource class instance will be applied to claims that do not directly reference a portable class. If we wanted to make our `standard-mysql` instance the default `MySQLInstanceClass` for namespace `app-project1-prod`, we could do so by adding a label:

```yaml
---
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: standard-mysql
  namespace: app-project1-prod
  labels:
    default: "true"
classRef:
  kind: SQLServerClass
  apiVersion: database.azure.crossplane.io/v1alpha2
  name: standard-mysql
  namespace: azure
```

#### Resource Claims
Resource claims are used to create external resources by referencing a class of service in the claim's namespace. When a claim is created, Crossplane uses the referenced portable class to find a non-portable class to use as the configuration for the external resource. We need a to create a claim to provision the MySQL database we will use with Azure.

* Define a `MySQLInstance` claim in `mysql-claim.yaml`:
```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: app-project1-prod
spec:
  classRef:
    name: standard-mysql
  writeConnectionSecretToRef:
    name: wordpressmysql
  engineVersion: "5.6"
```

* Create the `MySQLInstance`:
```bash
kubectl apply -f mysql-claim.yaml
```

What we are looking for is for `STATUS` value to become `Bound` which indicates the managed resource was successfully provisioned and is ready for consumption. You can see when claim is bound using the following:

*Command*
```bash
kubectl get mysqlinstances -n app-project1-prod
```

*Output*
```bash
NAME          STATUS   CLASS            VERSION   AGE
mysql-claim   Bound    standard-mysql   5.6       11m
```

If the `STATUS` is blank, we are still waiting for the claim to become bound. You can observe resource creation progression using the following:

*Command*
```bash
kubectl describe mysqlinstance mysql-claim -n app-project1-prod
```

*Output*
```yaml
Name:         mysql-claim
Namespace:    app-project1-prod
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
  Self Link:         /apis/database.crossplane.io/v1alpha1/namespaces/app-project1-prod/mysqlinstances/mysql-claim
  UID:               6a7fe064-d888-11e9-ab90-42b6bb22213a
Spec:
  Class Ref:
    Name:          standard-mysql
  Engine Version:  5.6
  Resource Ref:
    API Version:  database.azure.crossplane.io/v1alpha2
    Kind:         MysqlServer
    Name:         mysqlinstance-6a7fe064-d888-11e9-ab90-42b6bb22213a
    Namespace:    azure
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

*Note: You must wait until the claim becomes bound before continuing with this guide. It could take a few minutes for Azure to complete MySQL creation.*

We referenced our portable `MySQLInstanceClass` directly in the claim above, but if you specified that `standard-mysql` was the default `MySQLInstanceClass` for namespace `app-project1-prod`, we could have omitted the claim's `classRef` and it would automatically be assigned:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: app-project1-prod
spec:
  writeConnectionSecretToRef:
    name: wordpressmysql
  engineVersion: "5.6"
```

#### Virtual Network Rule
Before we install Wordpress, we need establish connectivity between our MySQL database and our AKS cluster. We can do this by creating a [Virtual Network Rule](https://docs.microsoft.com/en-us/azure/mysql/concepts-data-access-and-security-vnet).

* Set `MYSQL_NAME` environment variable:
```bash
export MYSQL_NAME=$(kubectl get -o json mysqlinstance mysql-claim -n app-project1-prod | jq -j '.spec.resourceRef.name')
```

* Define a `MysqlServerVirtualNetworkRule` in `wordpress-vnet-rule.yaml`:
```yaml
---
apiVersion: database.azure.crossplane.io/v1alpha2
kind: MysqlServerVirtualNetworkRule
metadata:
  name: wordpress-vnet-rule
  namespace: app-project1-prod
spec:
  name: wordpress-vnet-rule
  serverName: MYSQL_NAME
  resourceGroupName: AKS_RESOURCE_GROUP
  properties:
    virtualNetworkSubnetId: /subscriptions/SUBSCRIPTION_ID/resourceGroups/AKS_RESOURCE_GROUP/providers/Microsoft.Network/virtualNetworks/AKS_VNET/subnets/aks-subnet
  providerRef:
    name: demo-azure
    namespace: azure
  reclaimPolicy: Delete
```

* Create the `MysqlServerVirtualNetworkRule`:
```bash
sed "s/AKS_RESOURCE_GROUP/$AKS_RESOURCE_GROUP/g; s/SUBSCRIPTION_ID/$SUBSCRIPTION_ID/g; s/AKS_VNET/$AKS_VNET/g; s/MYSQL_NAME/$MYSQL_NAME/g" wordpress-vnet-rule.yaml | kubectl apply -f -
```

* You can verify creation with the following command and output:

*Command*
```bash
kubectl get mysqlservervirtualnetworkrules -n app-project1-prod
```
*Output*
```bash
NAME                  AGE
wordpress-vnet-rule   27s
```

## Install Wordpress
Installing Wordpress requires creating a Kubernetes `Deployment` and load balancer `Service`. We will point the deployment to the `wordpressmysql` secret that we specified in our claim above for the Wordpress container environment variables. It should have been populated with our MySQL connection details after the claim became `Bound`.

* Check to make sure `wordpressmysql` exists and is populated:

*Command*
```bash
kubectl describe secret wordpressmysql -n app-project1-prod
```

*Output*
```bash
Name:         wordpressmysql
Namespace:    app-project1-prod
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
endpoint:  75 bytes
password:  27 bytes
username:  58 bytes
```

* Define the `Deployment` and `Service` in `wordpress-app.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: app-project1-prod
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
  namespace: app-project1-prod
  name: wordpress
  labels:
    app: wordpress
spec:
  ports:
    - port: 80
  selector:
    app: wordpress
  type: LoadBalancer
```

* Create the `Deployment` and `Service`:
```bash
kubectl create -f wordpress-app.yaml
```

* You can verify creation with the following command and output:

*Command*
```bash
kubectl get -f wordpress-deployment.yaml 
```
*Output*
```bash
NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/wordpress   1/1     1            1           11m

NAME                TYPE           CLUSTER-IP    EXTERNAL-IP   PORT(S)        AGE
service/wordpress   LoadBalancer   10.0.128.30   52.168.69.6   80:32587/TCP   11m
```

If the `EXTERNAL-IP` field of the `LoadBalancer` is `<pending>`, wait until it becomes available, then navigate to the address. You should see the following:

![alt azure wordpress](azure-wordpress.png)

## Uninstall

### Wordpress
All Wordpress components that we installed using Helm can be deleted with one command:

```bash
kubectl delete -f wordpress-app.yaml
```

### Crossplane
To delete all created resources, but leave Crossplane and the Azure stack running, execute the following commands:
```bash
kubectl delete -f wordpress-vnet-rule.yaml
kubectl delete -f mysql-claim.yaml
kubectl delete -f mysql-class.yaml
kubectl delete -f azure-mysql-class.yaml
kubectl delete -f app-project1-prod-namespace.yaml
kubectl delete -f azure-provider.yaml
```
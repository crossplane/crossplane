# Deploying a WordPress Workload on Microsoft Azure

This guide will walk you through how to use Crossplane to deploy a stateful workload in a portable way to Azure.
In this environment, the following components will be dynamically provisioned and configured during this guide:

* AKS Kubernetes cluster
* Azure MySQL database
* WordPress application

## Pre-requisites

Before starting this guide, you should have already [configured your Azure account](../../cloud-providers/azure/azure-provider.md) for usage by Crossplane.

- You should have a `crossplane-azure-provider-key.json` file on your local filesystem, preferably at the root of where you cloned the [Crossplane repo](https://github.com/crossplaneio/crossplane).
- You should have a azure resource group with name `group-westus-1`. If not, change the value of `resourceGroupName` to an existing resource group in `cluster/examples/workloads/wordpress/azure/provider.yaml`



## Administrator Tasks

This section covers the tasks performed by the cluster or cloud administrator, which includes:

- Import Azure provider credentials
- Define Resource classes for cluster and database resources
- Create a target Kubernetes cluster (using dynamic provisioning with the cluster resource class)

**Note**: all artifacts created by the administrator are stored/hosted in the `crossplane-system` namespace, which has
restricted access, i.e. `Application Owner(s)` should not have access to them.

For the next steps, make sure your `kubectl` context points to the cluster where `Crossplane` was deployed.

- Create the Azure provider object in your cluster:

  ```console
  sed "s/BASE64ENCODED_AZURE_PROVIDER_CREDS/`base64 crossplane-azure-provider-key.json | tr -d '\n'`/g;" cluster/examples/workloads/wordpress/azure/provider.yaml | kubectl create -f -
  ```

- Next, create the AKS cluster that will eventually be the target cluster for your Workload deployment:

  ```console
  kubectl create -f cluster/examples/workloads/wordpress/azure/cluster.yaml
  ```

  It will take a while (~15 minutes) for the AKS cluster to be deployed and becoming ready. You can keep an eye on its status with the following command:

  ```console
  kubectl -n crossplane-system get akscluster -o custom-columns=NAME:.metadata.name,STATE:.status.state,CLUSTERNAME:.status.clusterName,ENDPOINT:.status.endpoint,LOCATION:.spec.location,CLUSTERCLASS:.spec.classRef.name,RECLAIMPOLICY:.spec.reclaimPolicy
  ```

  Once the cluster is done provisioning, you should see output similar to the following (note the `STATE` field is `Succeeded` and the `ENDPOINT` field has a value):

  ```console
  NAME                                       STATE       CLUSTERNAME                       ENDPOINT                                          LOCATION     CLUSTERCLASS       RECLAIMPOLICY
  aks-587762b3-f72b-11e8-bcbe-0800278fedb1   Succeeded   aks-587762b3-f72b-11e8-bcbe-080   crossplane-aks-653c32ef.hcp.centralus.azmk8s.io   Central US   standard-cluster   Delete
  ```

To recap the operations that we just performed as the administrator:

- Defined a `Provider` with Microsoft Azure service principal credentials
- Defined `ResourceClasses` for `KubernetesCluster` and `MySQLInstance`
- Provisioned (dynamically) an AKS Cluster using the `ResourceClass`

## Application Developer Tasks

This section covers the tasks performed by the application developer, which includes:

- Define Workload in terms of Resources and Payload (Deployment/Service) which will be deployed into the target Kubernetes Cluster
- Define the dependency resource requirements, in this case a `MySQL` database

Let's begin deploying the workload as the application developer:

- Now that the target AKS cluster is ready, we can deploy the Workload that contains all the Wordpress resources, including the SQL database, with the following single command:

  ```console
  kubectl create -f cluster/examples/workloads/wordpress-azure/workload.yaml
  ```

  This will also take awhile to complete, since the MySQL database needs to be deployed before the Wordpress pod can consume it.
  You can follow along with the MySQL database deployment with the following:

  ```console
  kubectl -n crossplane-system get mysqlserver -o custom-columns=NAME:.metadata.name,STATUS:.status.state,CLASS:.spec.classRef.name,VERSION:.spec.version
  ```

  Once the `STATUS` column is `Ready` like below, then the Wordpress pod should be able to connect to it:

  ```console
  NAME                                         STATUS    CLASS            VERSION
  mysql-58425bda-f72d-11e8-bcbe-0800278fedb1   Ready     standard-mysql   5.7
  ```

- Now we can watch the Wordpress pod come online and a public IP address will get assigned to it:

  ```console
  kubectl get workload -o custom-columns=NAME:.metadata.name,CLUSTER:.spec.targetCluster.name,NAMESPACE:.spec.targetNamespace,DEPLOYMENT:.spec.targetDeployment.metadata.name,SERVICE-EXTERNAL-IP:.status.service.loadBalancer.ingress[0].ip
  ```

  When a public IP address has been assigned, you'll see output similar to the following:

  ```console
  NAME            CLUSTER        NAMESPACE   DEPLOYMENT   SERVICE-EXTERNAL-IP
  test-workload   demo-cluster   demo        wordpress    104.43.240.15
  ```

- Once Wordpress is running and has a public IP address through its service, we can get the URL with the following command:

  ```console
  echo "http://$(kubectl get workload test-workload -o jsonpath='{.status.service.loadBalancer.ingress[0].ip}')"
  ```

- Paste that URL into your browser and you should see Wordpress running and ready for you to walk through the setup experience.

## Clean-up

First delete the workload, which will delete Wordpress and the MySQL database:

```console
kubectl delete -f cluster/examples/workloads/wordpress-azure/workload.yaml
```

Then delete the AKS cluster:

```console
kubectl delete -f cluster/examples/workloads/wordpress/azure/cluster.yaml
```

Finally, delete the provider credentials:

```console
kubectl delete -f cluster/examples/workloads/wordpress/azure/provider.yaml
```

# Deploying a WordPress Workload on GCP

This guide will walk you through how to use Crossplane to deploy a stateful workload in a portable way to GCP.
In this environment, the following components will be dynamically provisioned and configured during this guide:

* GKE Kubernetes cluster
* CloudSQL database
* WordPress application

## Pre-requisites

Before starting this guide, you should have already [configured your GCP account](../../cloud-providers/gcp/gcp-provider.md) for usage by Crossplane.

You should have a `crossplane-gcp-provider-key.json` file on your local filesystem, preferably at the root of where you cloned the [Crossplane repo](https://github.com/crossplaneio/crossplane).

## Administrator Tasks

This section covers the tasks performed by the cluster or cloud administrator, which includes:

- Import GCP provider credentials
- Define Resource classes for cluster and database resources
- Create a target Kubernetes cluster (using dynamic provisioning with the cluster resource class)

**Note**: all artifacts created by the administrator are stored/hosted in the `crossplane-system` namespace, which has
restricted access, i.e. `Application Owner(s)` should not have access to them.

For the next steps, make sure your `kubectl` context points to the cluster where `Crossplane` was deployed.

- Export Project ID

  **NOTE** you can skip this step if you generated GCP Service Account using `gcloud`
  ```bash
  export DEMO_PROJECT_ID=[your-demo-project-id]
  ```

- Patch and Apply `provider.yaml`:

  ```bash
  sed "s/BASE64ENCODED_CREDS/`cat key.json|base64 | tr -d '\n'`/g;s/DEMO_PROJECT_ID/$DEMO_PROJECT_ID/g" cluster/examples/workloads/wordpress-gcp/provider.yaml | kubectl create -f -
  ```

  - Verify that GCP Provider is in `Ready` state

    ```bash
    kubectl -n crossplane-system get providers.gcp.crossplane.io -o custom-columns=NAME:.metadata.name,STATUS:'.status.Conditions[?(@.Status=="True")].Type',PROJECT-ID:.spec.projectID
    ```

    Your output should look similar to:
    ```bash
    NAME           STATUS   PROJECT-ID
    gcp-provider   Ready    [your-project-id]
    ```

  - Verify that Resource Classes have been created

    ```bash
    kubectl -n crossplane-system get resourceclass -o custom-columns=NAME:metadata.name,PROVISIONER:.provisioner,PROVIDER:.providerRef.name,RECLAIM-POLICY:.reclaimPolicy
    ```

    Your output should be:
    ```bash
    NAME               PROVISIONER                                            PROVIDER       RECLAIM-POLICY
    standard-cluster   gkecluster.compute.gcp.crossplane.io/v1alpha1          gcp-provider   Delete
    standard-mysql     cloudsqlinstance.database.gcp.crossplane.io/v1alpha1   gcp-provider   Delete
      ```

- Create a target Kubernetes cluster where `Application Owner(s)` will deploy their `WorkLoad(s)`

  As administrator, you will create a Kubernetes cluster leveraging the Kubernetes cluster `ResourceClass` that was created earlier and
  `Crossplane` Kubernetes cluster dynamic provisioning.

  ```bash
  kubectl apply -f cluster/examples/workloads/wordpress-gcp/cluster.yaml
  ```

  - Verify that Kubernetes Cluster resource was created

    ```bash
    kubectl -n crossplane-system get kubernetescluster -o custom-columns=NAME:.metadata.name,CLUSTERCLASS:.spec.classReference.name,CLUSTERREF:.spec.resourceName.name
    ```

    Your output should look similar to:
    ```bash
    NAME               CLUSTERCLASS       CLUSTERREF
    demo-gke-cluster   standard-cluster   gke-67419e79-f5b3-11e8-9cec-9cb6d08bde99
    ```

  - Verify that the target GKE cluster was successfully created

    ```bash
    kubectl -n crossplane-system get gkecluster -o custom-columns=NAME:.metadata.name,STATE:.status.state,CLUSTERNAME:.status.clusterName,ENDPOINT:.status.endpoint,LOCATION:.spec.zone,CLUSTERCLASS:.spec.classRef.name,RECLAIMPOLICY:.spec.reclaimPolicy
    ```

    Your output should look similar to:
    ```bash
    NAME                                       STATE     CLUSTERNAME                                ENDPOINT        LOCATION        CLUSTERCLASS       RECLAIMPOLICY
    gke-67419e79-f5b3-11e8-9cec-9cb6d08bde99   RUNNING   gke-6742fe8d-f5b3-11e8-9cec-9cb6d08bde99   146.148.93.40   us-central1-a   standard-cluster   Delete
    ```

To recap the operations that we just performed as the administrator:

- Defined a `Provider` with Google Service Account credentials
- Defined `ResourceClasses` for `KubernetesCluster` and `MySQLInstance`
- Provisioned (dynamically) a GKE Cluster using the `ResourceClass`

## Application Developer Tasks

This section covers the tasks performed by the application developer, which includes:

- Define Workload in terms of Resources and Payload (Deployment/Service) which will be deployed into the target Kubernetes Cluster
- Define the dependency resource requirements, in this case a `MySQL` database

Let's begin deploying the workload as the application developer:

- Deploy workload

  ```bash
  kubectl apply -f cluster/examples/workloads/wordpress-gcp/workload.yaml
  ```

- Wait for `MySQLInstance` to be in `Bound` State

  You can check the status via:
  ```bash
  kubectl get mysqlinstance -o custom-columns=NAME:.metadata.name,VERSION:.spec.engineVersion,STATE:.status.bindingPhase,CLASS:.spec.classReference.name
  ```

  Your output should look like:
  ```bash
  NAME   VERSION   STATE   CLASS
  demo   5.7       Bound   standard-mysql
  ```

  **Note**: to check on the concrete resource type status as `Administrator` you can run:

  ```bash
  kubectl -n crossplane-system get cloudsqlinstance -o custom-columns=NAME:.metadata.name,STATUS:.status.state,CLASS:.spec.classRef.name,VERSION:.spec.databaseVersion
  ```

  Your output should be similar to:
  ```bash
  NAME                                         STATUS     CLASS            VERSION
  mysql-2fea0d8e-f5bb-11e8-9cec-9cb6d08bde99   RUNNABLE   standard-mysql   MYSQL_5_7
  ```

- Wait for `Workload` External IP Address

  ```bash
  kubectl get workload -o custom-columns=NAME:.metadata.name,CLUSTER:.spec.targetCluster.name,NAMESPACE:.spec.targetNamespace,DEPLOYMENT:.spec.targetDeployment.metadata.name,SERVICE-EXTERNAL-IP:.status.service.loadBalancer.ingress[0].ip
  ```
  **Note** the `Workload` is defined in Application Owner's (`default`) namespace

  Your output should look similar to:
  ```bash
  NAME   CLUSTER            NAMESPACE   DEPLOYMENT   SERVICE-EXTERNAL-IP
  demo   demo-gke-cluster   demo        wordpress    35.193.100.113
  ```

- Verify that `WordPress` service is accessible via `SERVICE-EXTERNAL-IP` by:
    - Navigate in your browser to `SERVICE-EXTERNAL-IP`

At this point, you should see the setup page for WordPress in your web browser.

## Clean Up

Once you are done with this example, you can clean up all its artifacts with the following commands:

- Remove `Workload`

  ```bash
  kubectl delete -f cluster/examples/workloads/wordpress-gcp/workload.yaml
  ```

- Remove `KubernetesCluster`

  ```bash
  kubectl delete -f cluster/examples/workloads/wordpress-gcp/cluster.yaml
  ```

- Remove GCP `Provider` and `ResourceClasses`

  ```bash
  kubectl delete -f cluster/examples/workloads/wordpress-gcp/provider.yaml
  ```

- Delete Google Project

  ```bash
  # list all your projects
  gcloud projects list

  # delete demo project
  gcloud projects delete [demo-project-id]
  ```

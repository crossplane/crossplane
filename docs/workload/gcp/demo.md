# WordPress Crossplane Workload on GCP

Deploy WordPress application as a Workload into a dynamically provisioned Kubernetes cluster on GKE, 
and backed by dynamically provisioned MySQL (CloudSQL) using Crossplane deployed on Minikube cluster 

## Requirements
- Minikube `v0.29.0+`
- A GCP account with administrator (or owner) level privileges.
- (Optional) `gcloud` command-line tool

## Install Crossplane 

Install Crossplane in a Minikube cluster first, for example with the following `helm` command after setting your preferred values for `image.repository` and `image.tag` in the `values.yaml` file:

```bash
helm install --name crossplane --namespace crossplane-system ${GOPATH}/src/github.com/crossplaneio/crossplane/cluster/charts/crossplane
```

## GCP Setup
Per requirements section, for this demo we must have a Google Cloud service account key in `json` format, which
corresponds to an active/valid GCP Service Account and has been granted the following roles:
- `Service Account User`: needed to have access to the service account information
- `Cloud SQL Admin`: needed to access (create/retrieve/connect/delete) CloudSQL instances
- `Kubernetes Engine User`: needed to access (create/connect/delete) GKE instances

Create a GCP demo project which we will use to host our demo GKE cluster, as well as, our demo CloudSQL instance.

- Login into [GCP Console](https://console.cloud.google.com)
- Create a new project (either stand alone or under existing organization)
- Create Demo Service Account
    - Navigate to: [Create Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts)
    - `Service Account Name`: type "demo"
    - `Service Account ID`: leave auto assigned
    - `Service Account Description`: type "Crossplane demo"
    - Hit `Create` button
        - This should advance to the next section `2 Grant this service account to project (optional)`
    - We will assign this account 3 roles:
        - `Service Account User`
        - `Cloud SQL Admin`
        - `Kubernetes Engine Admin`  
    - Hit `Create` button
        - This should advance to the next section `3 Grant users access to this service account (optinoal)`
    - We don't need to assign any user or admin roles to this account for the demo purposes, so you can leave following two fields blank:
        - `Service account users role`
        - `Service account admins role`
    - Next, we will create and export service account key
        - Hit `+ Create Key` button. 
            - This should open a `Create Key` side panel
        - Select `json` for the Key type (should be selected by default) 
        - Hit `Create`
            - This should show `Private key saved to your computer` confirmation dialog
            - You also should see `crossplane-demo-1234-[suffix].json` file in your browser's Download directory
            - Save (copy or move) this file into demo (this) directory, with new name `key.json`
- Enable `Cloud SQL API`
    - Navigate to [Cloud SQL Admin API](https://console.developers.google.com/apis/api/sqladmin.googleapis.com/overview)
    - Hit `Enable`
- Enable `Kubernetes Engine API`
    - Navigate to [Kubernetes Engine API](https://console.developers.google.com/apis/api/container.googleapis.com/overview)
    - Hit `Enable`

If you have `gcloud` utility, you can ran following commands from the demo directory 

```bash
# list your organizations (if applicable)
gcloud organizations list
    
# create a new project
export DEMO_PROJECT_NAME=crossplane-demo-123
gcloud projects create $DEMO_PROJECT_NAME --enable-cloud-apis [--organization ORANIZATION_ID]

# record the PROJECT_ID value of the newly created project
export DEMO_PROJECT_ID=$(gcloud projects list --filter NAME=$DEMO_PROJECT_NAME --format="value(PROJECT_ID)")   

# enable Kubernetes API
gcloud --project $DEMO_PROJECT_ID services enable container.googleapis.com
# enable CloudSQL API
gcloud --project $DEMO_PROJECT_ID services enable sqladmin.googleapis.com 

# create service account
gcloud --project $DEMO_PROJECT_ID iam service-accounts create demo-123 --display-name "Crossplane Demo"
# export service account email
export DEMO_SA="demo-123@$DEMO_PROJECT_ID.iam.gserviceaccount.com"

# create service account key (this will create a `key.json` file in your current working directory)
gcloud --project $DEMO_PROJECT_ID iam service-accounts keys create --iam-account $DEMO_SA key.json 

# assign roles
gcloud projects add-iam-policy-binding $DEMO_PROJECT_ID --member "serviceAccount:$DEMO_SA" --role="roles/iam.serviceAccountUser"
gcloud projects add-iam-policy-binding $DEMO_PROJECT_ID --member "serviceAccount:$DEMO_SA" --role="roles/cloudsql.admin"
gcloud projects add-iam-policy-binding $DEMO_PROJECT_ID --member "serviceAccount:$DEMO_SA" --role="roles/container.admin"
```

### Enable Billing
In order to create GKE clusters you must enable Billing.
- Go to [GCP Console](https://console.cloud.google.com)
    - Select demo project
    - Hit "Enable Billing"
- Go to [Kubernetes Clusters](https://console.cloud.google.com/kubernetes/list)
    - Hit "Enable Billing"

## WordPress Example
In the course of this demonstration we will show how to prepare and provision a sample application: WordPress which
uses MySQL backend database. 

We will use local (`minikube`) Kubernetes cluster to host `Crossplane` (`Crossplane cluster`) 

To demonstrate `Crossplane` concept of `separation of concerns` during this demo we will assume two identities:
1. Administrator (cluster or cloud) - responsible for setting up credentials and defining resource classes
2. Application Owner (developer) - responsible for defining and deploying application and its dependencies

### As Administrator
you will perform following tasks:

- Create Cloud provider credentials
- Define Resource classes
- Create a target Kubernetes cluster (using dynamic provisioning with the cluster resource class)

**Note**: all artifacts created by the administrator are stored/hosted in the `crossplane-system` namespace, which has
a restricted access, i.e. `Application Owner(s)` do not have access to them.

For the next steps, make sure your `kubectl` context points to the `Crossplane` cluster

- Export Project ID

    **NOTE** you can skip this step if you generated GCP Service Account using `gcloud`
    ```bash
    export DEMO_PROJECT_ID=[your-demo-project-id]
    ```

- Patch and Apply `provider.yaml`:
    ```bash
    sed "s/BASE64ENCODED_CREDS/`cat key.json|base64 | tr -d '\n'`/g;s/DEMO_PROJECT_ID/$DEMO_PROJECT_ID/g" cluster/examples/workloads/wordpress-gcp/provider.yaml | kubectl create -f -
    ``` 
 
    - Verify that GCP Provider is in READY state
        ```bash
        kubectl -n crossplane-system get providers.gcp.crossplane.io -o custom-columns=NAME:.metadata.name,STATUS:.status.Conditions[0].Type,PROJECT-ID:.spec.projectID
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
   
    As administrator, you will create a Kubernetes cluster leveraging existing Kubernetes cluster `ResourceClass` and 
    `Crossplane` Kubernetes cluster dynamic provisioning.
    ```bash
    kubectl apply -f cluster/examples/workloads/wordpress-gcp/kubernetes.yaml
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
  
    - Verify that Target GKE cluster was successfully created
        ```bash
        kubectl -n crossplane-system get gkecluster -o custom-columns=NAME:.metadata.name,STATE:.status.state,CLUSTERNAME:.status.clusterName,ENDPOINT:.status.endpoint,LOCATION:.spec.zone,CLUSTERCLASS:.spec.classRef.name,RECLAIMPOLICY:.spec.reclaimPolicy
        ```
        
        Your output should look similar to:
        ```bash
        NAME                                       STATE     CLUSTERNAME                                ENDPOINT        LOCATION        CLUSTERCLASS       RECLAIMPOLICY
        gke-67419e79-f5b3-11e8-9cec-9cb6d08bde99   RUNNING   gke-6742fe8d-f5b3-11e8-9cec-9cb6d08bde99   146.148.93.40   us-central1-a   standard-cluster   Delete
        ```       

To recap, as administrator user, you have:
- Defined a `Provider` with Google Service Account credentials
- Defined `ResourceClasses` for `KubernetesCluster` and `CloudSQLInstance`
- Provisioned (dynamically) a GKE Cluster using the `ResourceClass`

### As Application Owner
you will perform following tasks

- Define Workload in terms of Resources and Payload (Deployment/Service) which will be deployed onto a Target Kubernetes Cluster
- Define dependency resource requirements, in this case `MySQL` database

#### MySQL 
First, let's take a look at the dependency resource
```yaml
## WordPress MySQL Database Instance
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: demo
  namespace: default
spec:
  classReference:
    name: standard-mysql
    namespace: crossplane-system
  engineVersion: "5.7"
```

This will request to create a `MySQLInstance` version 5.7, which will be fulfilled by the `standard-mysql` ResourceClass.

Note, the Application Owner is not aware of any further specifics when it comes down to `MySQLInstance` beyond the engine version.

#### Workload
```yaml
## WordPress Workload
apiVersion: compute.crossplane.io/v1alpha1
kind: Workload
metadata:
  name: demo
  namespace: default
spec:
  resources:
  - name: demo
    secretName: demo
  targetCluster:
    name: demo-gke-cluster
    namespace: crossplane-system
  targetDeployment:
    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: wordpress
      labels:
        app: wordpress
    spec:
      selector:
        app: wordpress
      strategy:
        type: Recreate
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
                      name: demo
                      key: endpoint
                - name: WORDPRESS_DB_USER
                  valueFrom:
                    secretKeyRef:
                      name: demo
                      key: username
                - name: WORDPRESS_DB_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: demo
                      key: password
              ports:
                - containerPort: 80
                  name: wordpress
  targetNamespace: demo
  targetService:
    apiVersion: v1
    kind: Service
    metadata:
      name: wordpress
    spec:
      ports:
        - port: 80
      selector:
        app: wordpress
      type: LoadBalancer
```
   
The `Workload` definition spawns multiple constructs and kubernetes object. 
- Resources: list of the resources required by the payload application.
- TargetCluster: the cluster where the payload application and all its requirements should be deployed.
- TargetNamespace: the namespace on the target cluster
- Workload Payload:
    - TargetDeployment
    - TargetService
    
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
    
## Clean Up

- Remove `Workload` 
```bash
kubectl delete -f cluster/examples/workloads/wordpress-gcp/workload.yaml
```

- Remove `KubernetesCluster`
```bash
kubectl delete -f cluster/examples/workloads/wordpress-gcp/kubernetes.yaml
```

- Remove GCP Provider and ResourceClasses
```bash
kubectl delete -f cluster/examples/workloads/wordpress-gcp/provider.yaml
```

- Delete Google Project
```bash
# list all your projects
gcloud projects list

# delete demo project
gcloud projects delete [demo-project-id
```

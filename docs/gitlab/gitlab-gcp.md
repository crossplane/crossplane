# Deploying GitLab in GCP

This user guide will walk you through GitLab application deployment using Crossplane managed resources and
the official GitLab Helm chart.

## Pre-requisites

* [Kubernetes cluster](https://kubernetes.io/docs/setup/)
  * For example [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), minimum version `v0.28+`
* [Helm](https://docs.helm.sh/using_helm/), minimum version `v2.9.1+`.
* [jq](https://stedolan.github.io/jq/) - commandline JSON processor `v1.5+`


## Preparation

### Crossplane
- Install Crossplane using the [Crossplane Installation Guide](../install-crossplane.md)
- Obtain [Cloud Provider Credentials](../cloud-providers.md) 

#### GCP Provider   
It is essential to make sure that the GCP Service Account used by the Crossplane GCP Provider has the following Roles:

    Cloud SQL Admin
    Kubernetes Engine Admin
    Service Account User
    Cloud Memorystore Redis Admin
    Storage Admin

Using GCP Service Account `gcp-credentials.json`:
- Generate BASE64ENCODED_GCP_PROVIDER_CREDS encrypted value:
```bash
echo gcp-credentials.json | base64 -w0
```

- Update [provider.yaml](../../cluster/examples/gitlab/gcp/provider.yaml) replacing `BASE64ENCODED_GCP_PROVIDER_CREDS`
- Update [provider.yaml](../../cluster/examples/gitlab/gcp/provider.yaml) replacing `PROJECT_ID` with `project_id` from the credentials.json

#### GCS 
It is recommended to create a separate GCP Service Account dedicated to storage operations only, i.e. with a reduced IAM role set, for example: `StorageAdmin` only.

Follow the same step as for GCP credentials to create and obtain `gcs-credentials.json`
- Generate BASE64ENCODED_GCS_PROVIDER_CREDS encrypted value:
```bash
echo gcp-credentials.json | base64 -w0
```

Otherwise, you can use `BASE64ENCODED_GCP_PROVIDER_CREDS` in place of `BASE64ENCODED_GCS_PROVIDER_CREDS`

- Update [provider.yaml](../../cluster/examples/gitlab/gcp/provider.yaml) replacing `BASE64ENCODED_GCS_PROVIDER_CREDS`

##### GCS Interoperability
- Navigate to: https://console.cloud.google.com/storage/settings in your GCP project
- Click on `Interoperability` Tab
- Using  `Interoperable storage access keys` generate `BASE64ENCODED` values
   - `BASE64ENCODED_GCS_INTEROP_ACCESS_KEY`
   - `BASE64ENCODED_GCS_INTEROP_SECRET`
- Update [provider.yaml](../../cluster/examples/gitlab/gcp/provider.yaml) replacing:
   - `BASE64ENCODED_GCS_INTEROP_ACCESS_KEY`
   - `BASE64ENCODED_GCS_INTEROP_SECRET`
   
#### Create
- Create GCP provider:
    ```bash
    kubectl create -f cluster/examples/gitlab/gcp/provider.yaml  
    ```
- Verify GCP provider was successfully registered by the crossplane
    ```bash
    kubectl get providers.gcp.crossplane.io -n crossplane-system
    kubectl get secrets -n crossplane-system
    ```
    
    - You should see output similar to:    
    
    ```bash
    NAME       PROJECT-ID            AGE
    demo-gcp   your-project-123456   11m    
    NAME                  TYPE                                  DATA   AGE
    default-token-974db   kubernetes.io/service-account-token   3      2d16h
    demo-gcp-creds        Opaque                                1      103s
    demo-gcs-creds        Opaque                                3      2d11h
    ```   
        
#### Resource Classes
Create Crossplane Resource Class needed to provision managed resources for GitLab applications

```bash
kubectl create -f cluster/examples/gitlab/gcp/resource-classes/
```
```
resourceclass.core.crossplane.io/standard-gcp-bucket created
resourceclass.core.crossplane.io/standard-gcp-cluster created
resourceclass.core.crossplane.io/standard-gcp-postgres created
resourceclass.core.crossplane.io/standard-gcp-redis created
```    

Verify
```bash
kubectl get resourceclasses -n crossplane-system
```
```
NAME                    PROVISIONER                                                 PROVIDER-REF   RECLAIM-POLICY   AGE
standard-gcp-bucket     bucket.storage.gcp.crossplane.io/v1alpha1                   demo-gcp       Delete           17s
standard-gcp-cluster    gkecluster.compute.gcp.crossplane.io/v1alpha1               demo-gcp       Delete           17s
standard-gcp-postgres   cloudsqlinstance.database.gcp.crossplane.io/v1alpha1        demo-gcp       Delete           17s
standard-gcp-redis      cloudmemorystoreinstance.cache.gcp.crossplane.io/v1alpha1   demo-gcp       Delete           17s
```

#### Resource Claims
Provision Managed Resources required by GitLab application using Crossplane Resource Claims.

Note: you can use a separate command for each claim file, or create all claims in one command, like so:

```bash
kubectl create -Rf cluster/examples/gitlab/gcp/resource-claims/
```
```
bucket.storage.crossplane.io/gitlab-artifacts created
bucket.storage.crossplane.io/gitlab-backups-tmp created
bucket.storage.crossplane.io/gitlab-backups created
bucket.storage.crossplane.io/gitlab-externaldiffs created
bucket.storage.crossplane.io/gitlab-lfs created
bucket.storage.crossplane.io/gitlab-packages created
bucket.storage.crossplane.io/gitlab-pseudonymizer created
bucket.storage.crossplane.io/gitlab-registry created
bucket.storage.crossplane.io/gitlab-uploads created
kubernetescluster.compute.crossplane.io/gitlab-gke created
postgresqlinstance.storage.crossplane.io/gitlab-postgresql created
rediscluster.cache.crossplane.io/gitlab-redis created  
```

Verify that the resource claims were successfully provisioned. 
```bash
# check status of kubernetes cluster
kubectl get -f cluster/examples/gitlab/gcp/resource-claims/kubernetes.yaml
kubectl get -f cluster/examples/gitlab/gcp/resource-claims/postgres.yaml
kubectl get -f cluster/examples/gitlab/gcp/resource-claims/redis.yaml
```

```
NAME         STATUS   CLUSTER-CLASS          CLUSTER-REF                                AGE
gitlab-gke   Bound    standard-gcp-cluster   gke-af012df6-6e2a-11e9-ac37-9cb6d08bde99   4m7s
---
NAME                STATUS   CLASS                   VERSION   AGE
gitlab-postgresql   Bound    standard-gcp-postgres   9.6       5m27s
---
NAME           STATUS   CLASS                VERSION   AGE
gitlab-redis   Bound    standard-gcp-redis   3.2       7m10s
```

```bash
# check all bucket claims
kubectl get -f cluster/examples/gitlab/gcp/resource-claims/buckets/
```
```text
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-artifacts       Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-backups-tmp     Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-backups         Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-externaldiffs   Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-lfs             Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-packages        Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-pseudonymizer   Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-registry        Bound    standard-gcp-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-uploads         Bound    standard-gcp-bucket                                       4m49s
```

What we are looking for is for `STATUS` value to become `Bound` which indicates the managed resource was successfully provisioned and is ready for consumption

##### Resource Claims Connection Secrets
Verify that every resource has created a connection secret 
```bash
kubectl get secrets -n default 
```
```
NAME                   TYPE                                  DATA   AGE
default-token-mzsgg    kubernetes.io/service-account-token   3      5h42m
gitlab-artifacts       Opaque                                4      6m41s
gitlab-backups         Opaque                                4      7m6s
gitlab-backups-tmp     Opaque                                4      7m7s
gitlab-externaldiffs   Opaque                                4      7m5s
gitlab-lfs             Opaque                                4      7m4s
gitlab-packages        Opaque                                4      2m28s
gitlab-postgresql      Opaque                                3      30m
gitlab-pseudonymizer   Opaque                                4      7m2s
gitlab-redis           Opaque                                1      28m
gitlab-registry        Opaque                                4      7m1s
gitlab-uploads         Opaque                                4      7m1s
```

Note: Kubernetes cluster claim is created in "privileged" mode; thus the kubernetes cluster resource secret is located in `crossplane-system` namespace, however, you will not need to use this secret for our GitLab demo deployment.

At this point, all GitLab managed resources should be ready to consume and this completes the Crossplane resource provisioning phase. 

### GKE Cluster
Following the below steps will prepare the GKE Cluster for GitLab installation.

- First, get the GKE Cluster's name by examining the Kubernetes Resource Claim
```bash
kubectl get -f cluster/examples/gitlab/gcp/resource-claims/kubernetes.yaml
```
```
NAME         STATUS   CLUSTER-CLASS          CLUSTER-REF                                AGE
gitlab-gke   Bound    standard-gcp-cluster   gke-af012df6-6e2a-11e9-ac37-9cb6d08bde99   71m
```
- Using `CLUSTER-REF` get GKECluster resource
```bash
kubectl get gkecluster [CLUSTER-REF value] -n crossplane-system
```
```
NAME                                       STATUS   STATE     CLUSTER-NAME                               ENDPOINT          CLUSTER-CLASS          LOCATION        RECLAIM-POLICY   AGE
gke-af012df6-6e2a-11e9-ac37-9cb6d08bde99   Bound    RUNNING   gke-af11dfb1-6e2a-11e9-ac37-9cb6d08bde99   130.211.208.249   standard-gcp-cluster   us-central1-a   Delete           72m
```
- Record the `CLUSTER_NAME` value
- Obtain GKE Cluster credentials
    - Note: the easiest way to get `glcoud` command is via:
        - Go to: https://console.cloud.google.com/kubernetes/list
        - Click `Connect` next to cluster with `CLUSTER-NAME` value 
```bash
gcloud container clusters [CLUSTER-NAME] --zone [CLUSTER-ZONE] --project my-project-123456
``` 

Add your user account to the cluster admin role
```bash
kubectl create clusterrolebinding cluster-admin-binding \
    --clusterrole cluster-admin \
    --user [your-gcp-user-name]
```

#### External DNS
- Fetch the [External-DNS](https://github.com/helm/charts/tree/master/stable/external-dns) helm chart
```bash
helm fetch stable/external-dns
```
If the `helm fetch` command is successful, you should see a new file created in your CWD:
```bash
ls -l external-dns-*
```
```
-rw-r--r-- 1 user user 8913 May  3 23:24 external-dns-1.7.5.tgz
```

- Render the Helm chart into `yaml`, and set values and apply to your GKE cluster
```bash
helm template external-dns-1.7.5.tgz --name gitlab-demo --namespace kube-system  \
    --set provider=google \
    --set txtOwnerId=[gke-cluster-name] \
    --set google.project=[gcp-project-id] \
    --set rbac.create=true | kubectl -n kube-system apply -f -
```
```
service/release-name-external-dns created
deployment.extensions/release-name-external-dns created
```
- Verify `External-DNS` is up and running
```bash
kubectl get deploy,service -l release=gitlab-demo -n kube-system
```
```
NAME                                             DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
deployment.extensions/gitlab-demo-external-dns   1         1         1            1           1m

NAME                               TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
service/gitlab-demo-external-dns   ClusterIP   10.75.14.226   <none>        7979/TCP   1m
```

#### Managed Resource Secrets
Decide on the GKE cluster namespace where GitLab's application artifacts will be deployed.

We will use: `gitlab`, and for convenience we will [set our current context](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/#setting-the-namespace-preference) to this namespace 
```bash
kubectl create ns gitlab
kubectl config set-context $(kubectl config current-context) --namespace=gitlab
```

##### Export and Convert Secrets
GitLab requires to provide connection information in the specific format per cloud provider.
In addition, we need to extract endpoints and additional managed resource properties and add them to helm values.

_There is [current and ongoing effort](https://github.com/crossplaneio/gitlab-controller) to create an alternative experience to deploy GitLab Crossplane application, which alleviates integration difficulties between Crossplane platform and the GitLab Helm chart deployment._ 

We will use a convenience script for this purpose.
Note: your output may be different
```bash
./cluster/examples/gitlab/gcp/secrets.sh [your-local-k8s-cluster-context: default=minikube]
```
```
Source cluster kubectl context: microk8s
Current cluster kubectl context: gke_you-project-123456_us-central1-a_gke-a2345dfb1-asdf-11e9-ac37-9cb6d08bde99
---
Source cluster secrets:
NAME                   TYPE                                  DATA   AGE
default-token-mzsgg    kubernetes.io/service-account-token   3      2d7h
gitlab-artifacts       Opaque                                4      34h
gitlab-backups         Opaque                                4      34h
gitlab-backups-tmp     Opaque                                4      34h
gitlab-externaldiffs   Opaque                                4      34h
gitlab-lfs             Opaque                                4      34h
gitlab-packages        Opaque                                4      34h
gitlab-postgresql      Opaque                                3      2d2h
gitlab-pseudonymizer   Opaque                                4      34h
gitlab-redis           Opaque                                1      2d2h
gitlab-registry        Opaque                                4      34h
gitlab-uploads         Opaque                                4      34h
---
Generate PostgreSQL secret and values file
secret/gitlab-postgresql created
---
Generate Redis values file
---
Generate Buckets secrets
secret/bucket-artifacts created
secret/bucket-backups-tmp created
secret/bucket-backups created
secret/bucket-externaldiffs created
secret/bucket-lfs created
secret/bucket-packages created
secret/bucket-pseudonymizer created
secret/bucket-registry created
secret/bucket-uploads created

``` 

## Install
Render the official GitLab Helm chart with the generated values files, and your settings into a `gitlab-gcp.yaml` file.
See [GitLab Helm Documentation](https://docs.gitlab.com/charts/installation/deployment.html) for the additional details

```bash
helm repo add gitlab https://charts.gitlab.io/
helm repo update
helm fetch gitlab/gitlab --version v1.7.1
helm template gitlab-1.7.1.tgz --name gitlab-demo --namespace gitlab \
    -f cluster/examples/gitlab/gcp/values-buckets.yaml \
    -f cluster/examples/gitlab/gcp/values-redis.yaml \
    -f cluster/examples/gitlab/gcp/values-psql.yaml \
    --set global.hosts.domain=your.domain \
    --set global.hosts.hostSuffix=demo \
    --set certmanager-issuer.email=email@account.io > gitlab-gcp.yaml
```

Examine `gitlab-gcp.yaml` to familiarize yourself with all GitLab components.

Install GitLab
Note: your output may look different:
```bash
kubectl create -f gitlab-gcp.yaml
```

Validate GitLab components:

```bash
kubectl get jobs,deployments,statefulsets
```

It usually takes few minutes for all GitLab components to get initialized and be ready.

Note: During the initialization "wait", some pods could automatically restart, but this should stabilize once all the 
dependent components become available.

Note: There also could be intermittent `ImagePullBackOff`, but those, similar to above should clear up by themselves.

Note: It appears the `gitlab-demo-unicorn-test-runner-*` (job/pod) will Error and will not re-run, unless the pod is resubmitted. 

After few minutes your output for:
```bash
kubectl get pod
```
Should look similar to:
```bash
NAME                                                         READY   STATUS             RESTARTS   AGE
gitlab-demo-certmanager-59f887dc9-jppl7                      1/1     Running            0          9m
gitlab-demo-gitaly-0                                         1/1     Running            0          9m
gitlab-demo-gitlab-runner-fcc9cc7cf-c7pzz                    0/1     Init:0/1           0          9m
gitlab-demo-gitlab-shell-57b887755c-kqm89                    1/1     Running            0          8m
gitlab-demo-gitlab-shell-57b887755c-vzqkf                    1/1     Running            0          9m
gitlab-demo-issuer.0-ddzwp                                   0/1     Completed          0          9m
gitlab-demo-migrations.0-2h5px                               1/1     Running            2          9m
gitlab-demo-nginx-ingress-controller-7bf4f7574d-cznfl        1/1     Running            0          9m
gitlab-demo-nginx-ingress-controller-7bf4f7574d-f5wjz        1/1     Running            0          9m
gitlab-demo-nginx-ingress-controller-7bf4f7574d-mxqpz        1/1     Running            0          9m
gitlab-demo-nginx-ingress-default-backend-5886cb59c7-bjnrt   1/1     Running            0          9m
gitlab-demo-nginx-ingress-default-backend-5886cb59c7-gchhp   1/1     Running            0          9m
gitlab-demo-prometheus-server-64897864cf-p4sd7               2/2     Running            0          9m
gitlab-demo-registry-746bbb488f-xjlhp                        1/1     Running            0          8m
gitlab-demo-registry-746bbb488f-xxpcr                        1/1     Running            0          9m
gitlab-demo-shared-secrets.0-mr7-2v5cf                       0/1     Completed          0          9m
gitlab-demo-sidekiq-all-in-1-5dd8b5b9d-58p72                 1/1     Running            0          9m
gitlab-demo-task-runner-7c477b48dc-d5nf6                     1/1     Running            0          9m
gitlab-demo-unicorn-6dd757db97-4vqgc                         1/2     ImagePullBackOff   0          9m
gitlab-demo-unicorn-6dd757db97-nmglt                         2/2     Running            0          8m
gitlab-demo-unicorn-test-runner-f2ttk                        0/1     Error              0          9m
```

Note: if `ImagePullBackOff` error Pod does not get auto-cleared, consider deleting the pod. 
A new pod should come up with "Running" STATUS.

## Use
Retrieve the DNS name using GitLab ingress componenet:
```bash
kubectl get ingress
```
You should see following ingress configurations:
```
NAME                   HOSTS                       ADDRESS          PORTS     AGE
gitlab-demo-registry   registry-demo.upbound.app   35.222.163.203   80, 443   14m
gitlab-demo-unicorn    gitlab-demo.upbound.app     35.222.163.203   80, 443   14m
```

Navigate your browser to https://gitlab-demo.upbound.app, and if everything ran successfully, you should see:

![alt test](gitlab-login.png)

## Uninstall

### GitLab
To remove the GitLab application from the GKE cluster: run:
```bash
kubectl delete -f gitlab-gcp.yaml
```

### External-DNS
```bash
kubectl delete deploy,service -l app=external-dns -n kube-system
```

### Crossplane
To remove Crossplane managed resources, switch back to local cluster context `minikube`:
```bash
kubectl config use-context minikube
```

Delete all managed resources by running:
```bash
kubectl delete -Rf cluster/examples/gitlab/gcp/resource-claims
```
```
bucket.storage.crossplane.io "gitlab-artifacts" deleted
bucket.storage.crossplane.io "gitlab-backups-tmp" deleted
bucket.storage.crossplane.io "gitlab-backups" deleted
bucket.storage.crossplane.io "gitlab-externaldiffs" deleted
bucket.storage.crossplane.io "gitlab-lfs" deleted
bucket.storage.crossplane.io "gitlab-packages" deleted
bucket.storage.crossplane.io "gitlab-pseudonymizer" deleted
bucket.storage.crossplane.io "gitlab-registry" deleted
bucket.storage.crossplane.io "gitlab-uploads" deleted
kubernetescluster.compute.crossplane.io "gitlab-gke" deleted
postgresqlinstance.storage.crossplane.io "gitlab-postgresql" deleted
rediscluster.cache.crossplane.io "gitlab-redis" deleted
```

Verify that all resource claims have been removed:
```bash
kubectl get -Rf cluster/examples/gitlab/gcp/resource-claims
```
Note: typically it may take few seconds for Crossplane to process the request.
By running resource and provider removal in the same command or back-to-back, we are running the risk of having orphaned resource.
I.E., a resource that could not be cleaned up because the provider is no longer available. 

Delete all resource classes:
```bash
kubectl delete -Rf cluster/examples/gitlab/gcp/resource-classes/
```
```
resourceclass.core.crossplane.io "standard-gcp-bucket" deleted
resourceclass.core.crossplane.io "standard-gcp-cluster" deleted
resourceclass.core.crossplane.io "standard-gcp-postgres" deleted
resourceclass.core.crossplane.io "standard-gcp-redis" deleted
```

Delete gcp-provider and secrets
```bash
kubectl delete -f cluster/examples/gitlab/gcp/provider.yaml
```

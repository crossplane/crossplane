# Deploying GitLab in AWS

This user guide will walk you through GitLab application deployment using Crossplane managed resources and
the official GitLab Helm chart.

The following components are dynamically provisioned and configured during this guide:

* An EKS Kubernetes cluster
* An RDS Postgres database
* A Redis cluster
* A sample Gitlab application

## Pre-requisites

* Before starting this guide, you should have already [configured your AWS account](../../cloud-providers/aws/aws-provider.md) for use with Crossplane.
* You should also have an AWS credentials file at `~/.aws/credentials` already on your local filesystem.
* [Kubernetes cluster](https://kubernetes.io/docs/setup/)
  * For example [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), minimum version `v0.28+`
* [Helm](https://docs.helm.sh/using_helm/), minimum version `v2.9.1+`.
* [jq](https://stedolan.github.io/jq/) - commandline JSON processor `v1.5+`


## Preparation

### Crossplane
- Install Crossplane using the [Crossplane Installation Guide](../install-crossplane.md)
- Obtain [Cloud Provider Credentials](../cloud-providers.md)

## Administrator Tasks

This section covers tasks performed by the cluster or cloud administrator.  These include:

- Importing AWS provider credentials
- Defining resource classes for cluster and database resources
- Creating all EKS pre-requisite artifacts
- Creating a target EKS cluster (using dynamic provisioning with the cluster resource class)

> Note: All artifacts created by the administrator are stored/hosted in the `crossplane-system` namespace, which has
restricted access, i.e. `Application Owner(s)` should not have access to them.

To successfully follow this guide, make sure your `kubectl` context points to the cluster where `Crossplane` was deployed.

### Configuring EKS Cluster Pre-requisites

EKS cluster deployment is somewhat of an arduous process right now.
A number of artifacts and configurations need to be set up within the AWS console prior to provisioning an EKS cluster using Crossplane.
We anticipate that AWS will make improvements on this user experience in the near future.

#### Create a named keypair
1. Use an existing ec2 key pair or create a new key pair by following [these steps](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html)
1. Export the key pair name as the EKS_WORKER_KEY_NAME environment variable
    ```console
    export EKS_WORKER_KEY_NAME=replace-with-key-name
    ```

#### Create your Amazon EKS Service Role
[Original Source Guide](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html)

1. Open the [IAM console](https://console.aws.amazon.com/iam/).
1. Choose Roles, then Create role.
1. Choose EKS from the list of services, then "Allows EKS to manage clusters on your behalf", then Next: Permissions.
1. Choose Next: Tags.
1. Choose Next: Review.
1. For the Role name, enter a unique name for your role, such as eksServiceRole, then choose Create role.
1. Set the EKS_ROLE_ARN environment variable to the name of your role ARN
    ```console
    export EKS_ROLE_ARN=replace-with-full-role-arn
    ```

#### Create your Amazon EKS Cluster VPC
[Original Source Guide](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html)

1. Open the [AWS CloudFormation console](https://console.aws.amazon.com/cloudformation).
1. From the navigation bar, select a Region that supports Amazon EKS.
    > Note: Amazon EKS is available in the following Regions at this time:
     > * US West (Oregon) (us-west-2)
     > * US East (N. Virginia) (us-east-1)
     > * EU (Ireland) (eu-west-1)

1. Set the REGION environment variable to your region
    ```console
    export REGION=replace-with-region
    ```

1. Choose Create stack.
1. For Choose a template, select Specify an Amazon S3 template URL.
1. Paste the following URL into the text area and choose Next.
    ```
    https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-11-07/amazon-eks-vpc-sample.yaml
    ```
1. On the Specify Details page, fill out the parameters accordingly, and choose Next.
    ```
    * Stack name: Choose a stack name for your AWS CloudFormation stack. For example, you can call it eks-vpc.
    * VpcBlock: Choose a CIDR range for your VPC. You may leave the default value.
    * Subnet01Block: Choose a CIDR range for subnet 1. You may leave the default value.
    * Subnet02Block: Choose a CIDR range for subnet 2. You may leave the default value.
    * Subnet03Block: Choose a CIDR range for subnet 3. You may leave the default value.
    ```
1. (Optional) On the Options page, tag your stack resources and choose Next.
1. On the Review page, choose Create.
1. When your stack is created, select it in the console and choose Outputs.
1. Using values from outputs, export the following environment variables.
    ```console
    export EKS_VPC=replace-with-eks-vpcId
    export EKS_SUBNETS=replace-with-eks-subnetIds01,replace-with-eks-subnetIds02,replace-with-eks-subnetIds03
    export EKS_SECURITY_GROUP=replace-with-eks-securityGroupId
    ```

#### Create an RDS subnet group
1. Navigate to the aws console in same region as the EKS cluster
1. Navigate to `RDS` service
1. Navigate to `Subnet groups` in left hand pane
1. Click `Create DB Subnet Group`
1. Name your subnet i.e. `eks-db-subnets`
1. Select the VPC created in the EKS VPC step
1. Click `Add all subnets related to this VPC`
1. Click Create
1. Export the db subnet group name
    ```console
    export RDS_SUBNET_GROUP_NAME=replace-with-DBSubnetgroup-name
    ```
#### Create an RDS Security Group (example only)

> Note: This will make your RDS instance visible from anywhere on the internet.
This is for **EXAMPLE PURPOSES ONLY** and is **NOT RECOMMENDED** for production system.

1. Navigate to ec2 in the same region as the EKS cluster
1. Click: security groups
1. Click `Create Security Group`
1. Name it, ex. `demo-rds-public-visibility`
1. Give it a description
1. Select the same VPC as the EKS cluster.
1. On the Inbound Rules tab, choose Edit.
    - For Type, choose `MYSQL/Aurora`
    - For Port Range, type `3306`
    - For Source, choose `Anywhere` from drop down or type: `0.0.0.0/0`
1. Choose Add another rule if you need to add more IP addresses or different port ranges.
1. Click: Create
1. Export the security group id
    ```console
    export RDS_SECURITY_GROUP=replace-with-security-group-id
    ```

#### Create a Redis security group

1. TODO: add details here about redis, but open everything on tcp 6379 - No security/auth on redis typically, should not leave this running

```console
export REDIS_SECURITY_GROUP=eplace-with-redis-securityGroupId
```

#### Create
- Create AWS provider:

Create provider:
```console
sed -e "s|BASE64ENCODED_AWS_PROVIDER_CREDS|`cat ~/.aws/credentials|base64|tr -d '\n'`|g;" cluster/examples/gitlab/aws/provider.yaml | kubectl create -f -
```

- Verify AWS provider was successfully registered by the crossplane
    ```bash
    kubectl get providers.aws.crossplane.io -n crossplane-system
    kubectl get secrets -n crossplane-system
    ```
    
    - You should see output similar to:    
    
    ```bash
    NAME       PROJECT-ID            AGE
    demo-aws   your-project-123456   11m
    NAME                  TYPE                                  DATA   AGE
    default-token-974db   kubernetes.io/service-account-token   3      2d16h
    demo-aws-creds        Opaque                                1      103s
    ```   
        
#### Resource Classes
Create Crossplane Resource Class needed to provision managed resources for GitLab applications

```bash
```console
sed -e "s|REDIS_SECURITY_GROUP|$REDIS_SECURITY_GROUP|g;s|EKS_WORKER_KEY_NAME|$EKS_WORKER_KEY_NAME|g;s|EKS_ROLE_ARN|$EKS_ROLE_ARN|g;s|REGION|$REGION|g;s|EKS_VPC|$EKS_VPC|g;s|EKS_SUBNETS|$EKS_SUBNETS|g;s|EKS_SECURITY_GROUP|$EKS_SECURITY_GROUP|g;s|RDS_SUBNET_GROUP_NAME|$RDS_SUBNET_GROUP_NAME|g;s|RDS_SECURITY_GROUP|$RDS_SECURITY_GROUP|g" cluster/examples/gitlab/aws/resource-classes/* | kubectl create -f -
```
```
resourceclass.core.crossplane.io/standard-aws-bucket created
resourceclass.core.crossplane.io/standard-aws-cluster created
resourceclass.core.crossplane.io/standard-aws-postgres created
resourceclass.core.crossplane.io/standard-aws-redis created
```    

Verify
```bash
kubectl get resourceclasses -n crossplane-system
```
```
NAME                    PROVISIONER                                                 PROVIDER-REF   RECLAIM-POLICY   AGE
standard-aws-bucket     s3buckets.storage.aws.crossplane.io/v1alpha1                demo-aws       Delete           17s
standard-aws-cluster    ekscluster.compute.aws.crossplane.io/v1alpha1               demo-aws       Delete           17s
standard-aws-postgres   rdsinstance.database.aws.crossplane.io/v1alpha1             demo-aws       Delete           17s
standard-aws-redis      replicationgroup.cache.aws.crossplane.io/v1alpha1           demo-aws       Delete           17s
```

#### Resource Claims
Provision Managed Resources required by GitLab application using Crossplane Resource Claims.

Note: you can use a separate command for each claim file, or create all claims in one command, like so:

```bash
kubectl create -Rf cluster/examples/gitlab/aws/resource-claims/
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
kubectl get -f cluster/examples/gitlab/aws/resource-claims/kubernetes.yaml
kubectl get -f cluster/examples/gitlab/aws/resource-claims/postgres.yaml
kubectl get -f cluster/examples/gitlab/aws/resource-claims/redis.yaml
```

```
NAME         STATUS   CLUSTER-CLASS          CLUSTER-REF                                AGE
gitlab-eks   Bound    standard-aws-cluster   aws-af012df6-6e2a-11e9-ac37-9cb6d08bde99   4m7s
---
NAME                STATUS   CLASS                   VERSION   AGE
gitlab-postgresql   Bound    standard-aws-postgres   9.6       5m27s
---
NAME           STATUS   CLASS                VERSION   AGE
gitlab-redis   Bound    standard-aws-redis   3.2       7m10s
```

```bash
# check all bucket claims
kubectl get -f cluster/examples/gitlab/aws/resource-claims/buckets/
```
```text
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-artifacts       Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-backups-tmp     Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-backups         Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-externaldiffs   Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-lfs             Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-packages        Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-pseudonymizer   Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-registry        Bound    standard-aws-bucket                                       4m49s
NAME                   STATUS   CLASS                 PREDEFINED-ACL   LOCAL-PERMISSION   AGE
gitlab-uploads         Bound    standard-aws-bucket                                       4m49s
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
Following the below steps will prepare the EKS Cluster for GitLab installation.

- First, get the GKE Cluster's name by examining the Kubernetes Resource Claim
```bash
kubectl get -f cluster/examples/gitlab/gcp/resource-claims/kubernetes.yaml
```
```
NAME         STATUS   CLUSTER-CLASS          CLUSTER-REF                                AGE
gitlab-eks   Bound    standard-aws-cluster   gke-af012df6-6e2a-11e9-ac37-9cb6d08bde99   71m
```


====== stopping here for now.

- Using `CLUSTER-REF` get GKECluster resource
```bash
kubectl get ekscluster [CLUSTER-REF value] -n crossplane-system
```
```
NAME                                       STATUS   STATE     CLUSTER-NAME                               ENDPOINT          CLUSTER-CLASS          LOCATION        RECLAIM-POLICY   AGE
eks-af012df6-6e2a-11e9-ac37-9cb6d08bde99   Bound    RUNNING   gke-af11dfb1-6e2a-11e9-ac37-9cb6d08bde99   130.211.208.249   standard-gcp-cluster   us-central1-a   Delete           72m
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

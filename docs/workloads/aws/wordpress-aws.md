# Deploying a WordPress Workload on AWS

This guide walks you through how to use Crossplane to deploy a stateful workload in a portable way on AWS.
The following components are dynamically provisioned and configured during this guide:

* An EKS Kubernetes cluster
* An RDS MySQL database
* A sample WordPress application

## Pre-requisites

Before starting this guide, you should have already [configured your AWS account](../../cloud-providers/aws/aws-provider.md) for use with Crossplane.

You should also have an AWS credentials file at `~/.aws/credentials` already on your local filesystem.

## Administrator Tasks

This section covers tasks performed by the cluster or cloud administrator.  These include:

* Importing AWS provider credentials
* Defining resource classes for cluster and database resources
* Creating all EKS pre-requisite artifacts
* Creating a target EKS cluster (using dynamic provisioning with the cluster resource class)

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

    ```text
    https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-11-07/amazon-eks-vpc-sample.yaml
    ```

1. On the Specify Details page, fill out the parameters accordingly, and choose Next.

    ```console
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

1. Navigate to the aws console in same region as the EKS clsuter
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
    * For Type, choose `MYSQL/Aurora`
    * For Port Range, type `3306`
    * For Source, choose `Anywhere` from drop down or type: `0.0.0.0/0`
1. Choose Add another rule if you need to add more IP addresses or different port ranges.
1. Click: Create
1. Export the security group id

    ```console
    export RDS_SECURITY_GROUP=replace-with-security-group-id
    ```

### Deploy all Workload Resources

Now deploy all the workload resources, including the RDS database and EKS cluster with the following commands:

Create provider:

```console
sed -e "s|BASE64ENCODED_AWS_PROVIDER_CREDS|`base64 ~/.aws/credentials|tr -d '\n'`|g;s|EKS_WORKER_KEY_NAME|$EKS_WORKER_KEY_NAME|g;s|EKS_ROLE_ARN|$EKS_ROLE_ARN|g;s|REGION|$REGION|g;s|EKS_VPC|$EKS_VPC|g;s|EKS_SUBNETS|$EKS_SUBNETS|g;s|EKS_SECURITY_GROUP|$EKS_SECURITY_GROUP|g;s|RDS_SUBNET_GROUP_NAME|$RDS_SUBNET_GROUP_NAME|g;s|RDS_SECURITY_GROUP|$RDS_SECURITY_GROUP|g" cluster/examples/workloads/kubernetes/wordpress/aws/provider.yaml | kubectl create -f -
```

Create cluster:

```console
kubectl create -f cluster/examples/workloads/kubernetes/wordpress/aws/cluster.yaml
```

It will take a while (~15 minutes) for the EKS cluster to be deployed and become available.
You can keep an eye on its status with the following command:

```console
kubectl -n crossplane-system get ekscluster
```

Once the cluster is done provisioning, you should see output similar to the following
> Note:  the `STATE` field is `ACTIVE` and the `ENDPOINT` field has a value):

```console
NAME                                       STATUS   STATE    CLUSTER-NAME   ENDPOINT                                                                   CLUSTER-CLASS      LOCATION   RECLAIM-POLICY   AGE
eks-825c1234-9697-11e9-8b05-080027550c17   Bound    ACTIVE                  https://6A7981620931E720CE162F751C158A78.yl4.eu-west-1.eks.amazonaws.com   standard-cluster              Delete           51m
```

## Application Developer Tasks

This section covers tasks performed by an application developer.  These include:

* Defining a Workload in terms of Resources and Payload (Deployment/Service) which will be deployed into the target Kubernetes Cluster
* Defining the resource's dependency requirements, in this case a `MySQL` database

Now that the EKS cluster is ready, let's begin deploying the workload as the application developer:

```console
kubectl create -f cluster/examples/workloads/kubernetes/wordpress/aws/app.yaml
```

This will also take awhile to complete, since the MySQL database needs to be deployed before the WordPress pod can consume it.
You can follow along with the MySQL database deployment with the following:

```console
kubectl -n crossplane-system get rdsinstance
```

Once the `STATUS` column is `available` as seen below, the WordPress pod should be able to connect to it:

```console
NAME                                         STATUS   STATE       CLASS            VERSION   AGE
mysql-3f902b48-974f-11e9-8b05-080027550c17   Bound    available   standard-mysql   5.7       15m
```

As an administrator, we can examine the cluster directly.

```console
$ CLUSTER=eks-$(kubectl get kubernetesclusters.compute.crossplane.io -n complex -o=jsonpath='{.items[0].spec.resourceName.uid}')
$ KUBECONFIG=/tmp/$CLUSTER aws eks update-kubeconfig --name=$CLUSTER --region=$REGION
$ KUBECONFIG=/tmp/$CLUSTER kubectl get all -lapp=wordpress -A
NAMESPACE   NAME                             READY   STATUS    RESTARTS   AGE
wordpress   pod/wordpress-8545774bcf-8xj8j   1/1     Running   0          13m

NAMESPACE   NAME                TYPE           CLUSTER-IP      EXTERNAL-IP                                                               PORT(S)        AGE
wordpress   service/wordpress   LoadBalancer   10.100.201.94   a4631fbfa974f11e9932a060b5ad3abc-1542130681.eu-west-1.elb.amazonaws.com   80:31832/TCP   13m

NAMESPACE   NAME                        DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
wordpress   deployment.apps/wordpress   1         1         1            1           13m

NAMESPACE   NAME                                   DESIRED   CURRENT   READY   AGE
wordpress   replicaset.apps/wordpress-8545774bcf   1         1         1       13m
$ rm /tmp/$CLUSTER
```

Continuing as the application developer, we can watch the WordPress pod come online and a public IP address will get assigned to it:

```console
kubectl -n complex get kubernetesapplication,kubernetesapplicationresources
```

When a public IP address has been assigned, you'll see output similar to the following:

```console
NAME                                                          CLUSTER                  STATUS               DESIRED   SUBMITTED
kubernetesapplication.workload.crossplane.io/wordpress-demo   wordpress-demo-cluster   PartiallySubmitted   3         2

NAME                                                                             TEMPLATE-KIND   TEMPLATE-NAME   CLUSTER                  STATUS
kubernetesapplicationresource.workload.crossplane.io/wordpress-demo-deployment   Deployment      wordpress       wordpress-demo-cluster   Submitted
kubernetesapplicationresource.workload.crossplane.io/wordpress-demo-namespace    Namespace       wordpress       wordpress-demo-cluster   Submitted
kubernetesapplicationresource.workload.crossplane.io/wordpress-demo-service      Service         wordpress       wordpress-demo-cluster   Failed
```

*Note* A Failed status on the Service may be attributable to issues [#428](https://github.com/crossplaneio/crossplane/issues/428) and [504](https://github.com/crossplaneio/crossplane/issues/504). The service should be running despite this status.

Once WordPress is running and has a public IP address through its service, we can get the URL with the following command:

```console
echo "http://$(kubectl get kubernetesapplicationresource.workload.crossplane.io/wordpress-demo-service -n complex -o yaml  -o jsonpath='{.status.remote.loadBalancer.ingress[0].hostname}')"
```

Paste that URL into your browser and you should see WordPress running and ready for you to walk through its setup experience. You may need to wait a few minutes for this to become accessible via the AWS load balancer.

## Connecting to your EKSCluster (optional)

Requires:

* awscli
* aws-iam-authenticator

Please see [Install instructions](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html) section: `Install and Configure kubectl for Amazon EKS`

When the EKSCluster is up and running, you can update your kubeconfig with:

```console
aws eks update-kubeconfig --name <replace-me-eks-cluster-name>
```

Node pool is created after the master is up, so expect a few more minutes to wait, but eventually you can see that nodes joined with:

```console
kubectl config use-context <context-from-last-command>
kubectl get nodes
```

## Clean-up

First delete the workload, which will delete WordPress and the MySQL database:

```console
kubectl delete -f cluster/examples/workloads/kubernetes/wordpress/aws/app.yaml
```

Then delete the EKS cluster:

```console
kubectl delete -f cluster/examples/workloads/kubernetes/wordpress/aws/cluster.yaml
```

Finally, delete the provider credentials:

```console
kubectl delete -f cluster/examples/workloads/kubernetes/wordpress/aws/provider.yaml
```

> Note: There may still be an ELB that was not properly cleaned up, and you will need
to go to EC2 > ELBs and delete it manually.

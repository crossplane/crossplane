# Wordpress on AWS

## Pre-requisites

### AWS Credentials

AWS Credentials file

Follow the steps in the [AWS SDK for GO](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html) to get your access key ID and secret access key
1. Open the IAM console.
1. On the navigation menu, choose Users.
1. Choose your IAM user name (not the check box).
1. Open the Security credentials tab, and then choose Create access key.
1. To see the new access key, choose Show. Your credentials resemble the following:
    - Access key ID: AKIAIOSFODNN7EXAMPLE
    - Secret access key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
1. To download the key pair, choose Download .csv file. Store the keys

Convert *.csv to `.aws/credentials` format
```
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

**Note** If you have installed and configured `aws cli` you can find your AWS credentials file in  `~/.aws/credentials`

## Setup AWS provider

Next, create a `demo` namespace:

```console
kubectl create namespace demo
```

### Create credentials

1. Get base64 encoded credentials with cat ~/.aws/credentials|base64|tr -d '\n'
1. Replace BASE64ENCODED_AWS_PROVIDER_CREDS in cluster/examples/workloads/wordpress-aws/provider.yaml with value from previous step.

## Deploy EKS Cluster

### Create a named keypair
* If you already have an ec2 keypair you can use your existing key pair https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html
* Replace your keypair name in cluster/examples/workloads/wordpress-aws/provider.yaml in EKS_WORKER_KEY_NAME

### Create your Amazon EKS Service Role
[Original Source](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html)

1. Open the IAM console at https://console.aws.amazon.com/iam/.
1. Choose Roles, then Create role.
1. Choose EKS from the list of services, then Allows Amazon EKS to manage your clusters on your behalf for your use case, then Next: Permissions.
1. Choose Next: Review.
1. For Role name, enter a unique name for your role, such as eksServiceRole, then choose Create role.
1. Replace EKS_ROLE_ARN in cluster/examples/workloads/wordpress-aws/provider.yaml with role arn from previous step.

### Create your Amazon EKS Cluster VPC
[Original Source](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html)

1. Open the AWS CloudFormation console at https://console.aws.amazon.com/cloudformation.
1. From the navigation bar, select a Region that supports Amazon EKS.
    ```> Note
     Amazon EKS is available in the following Regions at this time:
     * US West (Oregon) (us-west-2)
     * US East (N. Virginia) (us-east-1)
     * EU (Ireland) (eu-west-1)
    ```

1. Choose Create stack.
1. For Choose a template, select Specify an Amazon S3 template URL.
1. Paste the following URL into the text area and choose Next:
    ```
    https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-11-07/amazon-eks-vpc-sample.yaml
    ```
1. On the Specify Details page, fill out the parameters accordingly, and then choose Next.
    ```
    * Stack name: Choose a stack name for your AWS CloudFormation stack. For example, you can call it eks-vpc.
    * VpcBlock: Choose a CIDR range for your VPC. You may leave the default value.
    * Subnet01Block: Choose a CIDR range for subnet 1. You may leave the default value.
    * Subnet02Block: Choose a CIDR range for subnet 2. You may leave the default value.
    * Subnet03Block: Choose a CIDR range for subnet 3. You may leave the default value.
    ```
1. (Optional) On the Options page, tag your stack resources. Choose Next.
1. On the Review page, choose Create.
1. When your stack is created, select it in the console and choose Outputs.
1. Replace `EKS_VPC`, `EKS_ROLE_ARN`, `EKS_SUBNETS`, `EKS_SECURITY_GROUP` in cluster/examples/workloads/wordpress-aws/provider.yaml with values from previous step (vpcId, subnetIds, securityGroupIds). Note `EKS_SECURITY_GROUP` needs to be replaced twice in file.
1. Replace `REGION` in cluster/examples/workloads/wordpress-aws/provider.yaml with the region you selected in VPC creation.

### Create an RDS subnet group
1. Navigate to aws console in same region as eks clsuter
1. Navigate to `RDS` service
1. Naviate to `Subnet groups` in left hand pane
1. Click `Create DB Subnet Group`
1. Name your subnet i.e. eks-db-subnets
1. Select the VPC created in the EKS VPC step
1. Click `Add all subnets related to this VPC`
1. Click Create
1. Replace `RDS_SUBNET_GROUP` in cluster/examples/workloads/wordpress-aws/provider.yaml in DBSubnetgroup name you just created.

### Create an RDS Security Group (demo only)

**Note**: This will make your RDS instance visible from Anywhere on the internet. This if for **DEMO PURPOSES ONLY**, and
is **NOT RECOMMENDED** for production system.

1. Navigate to ec2 in the region of the EKS cluster
1. Navigate to security groups
1. Select the same VPC from the EKS cluster.
1. On the Inbound Rules tab, choose Edit.
    - For Type, choose `MYSQL/Aurora`
    - For Port Range, type `3306`
    - For Source, choose `Anywhere` from drop down or type: `0.0.0.0/0`
1. Choose Add another rule if you need to add more IP addresses or different port ranges.
1. Replace `RDS_SECURITY_GROUP` in cluster/examples/workloads/wordpress-aws/provider.yaml with the security group we just created.

## Deploy Wordpress Resources

Now deploy all the Wordpress provider and workload resources, including the RDS database, and EKS cluster with the following single commands:

Create provider
```console
kubectl create -f cluster/examples/workloads/wordpress-aws/provider.yaml
```

Create cluster
```console
kubectl create -f cluster/examples/workloads/wordpress-aws/cluster.yaml
```


Note: It will take about 10 minutes for the EKSCluster to become active, with about another 5 for the nodes to be started and join the cluster.

## Waiting for load balancer and things to be available

It will take a while (~15 minutes) for the EKS cluster to be deployed and becoming ready.
You can keep an eye on its status with the following command:

```console
kubectl -n crossplane-system get ekscluster -o custom-columns=NAME:.metadata.name,STATE:.status.state,CLUSTERNAME:.status.clusterName,ENDPOINT:.status.endpoint,LOCATION:.spec.location,CLUSTERCLASS:.spec.classRef.name,RECLAIMPOLICY:.spec.reclaimPolicy
```

Once the cluster is done provisioning, you should see output similar to the following (note the `STATE` field is `Succeeded` and the `ENDPOINT` field has a value):

```console
NAME                                       STATE      CLUSTERNAME   ENDPOINT                                                                   LOCATION   CLUSTERCLASS       RECLAIMPOLICY
eks-8f1f32c7-f6b4-11e8-844c-025000000001   ACTIVE     <none>        https://B922855C944FC0567E9050FCD75B6AE5.yl4.us-west-2.eks.amazonaws.com   <none>     standard-cluster   Delete
```

Now that the target EKS cluster is ready, we can deploy the Workload that contains all the Wordpress resources, including the SQL database, with the following single command:

```console
kubectl -n demo create -f cluster/examples/workloads/wordpress-${provider}/workload.yaml
```

This will also take awhile to complete, since the MySQL database needs to be deployed before the Wordpress pod can consume it.
You can follow along with the MySQL database deployment with the following:

```console
kubectl -n crossplane-system get rdsinstance -o custom-columns=NAME:.metadata.name,STATUS:.status.state,CLASS:.spec.classRef.name,VERSION:.spec.version
```

Once the `STATUS` column is `Ready` like below, then the Wordpress pod should be able to connect to it:

```console
NAME                                         STATUS      CLASS            VERSION
mysql-2a0be04f-f748-11e8-844c-025000000001   available   standard-mysql   <none>
```

Now we can watch the Wordpress pod come online and a public IP address will get assigned to it:

```console
kubectl -n demo get workload -o custom-columns=NAME:.metadata.name,CLUSTER:.spec.targetCluster.name,NAMESPACE:.spec.targetNamespace,DEPLOYMENT:.spec.targetDeployment.metadata.name,SERVICE-EXTERNAL-IP:.status.service.loadBalancer.ingress[0].ip
```

When a public IP address has been assigned, you'll see output similar to the following:

```console
NAME            CLUSTER        NAMESPACE   DEPLOYMENT   SERVICE-EXTERNAL-IP
demo            demo-cluster   demo        wordpress    104.43.240.15
```

Once Wordpress is running and has a public IP address through its service, we can get the URL with the following command:

```console
echo "http://$(kubectl get workload test-workload -o jsonpath='{.status.service.loadBalancer.ingress[0].ip}')"
```

Paste that URL into your browser and you should see Wordpress running and ready for you to walk through the setup experience. You may need to wait a few minutes for this to become active in the aws elb.

## Connect to your EKSCluster (optional)

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

First delete the workload, which will delete Wordpress and the MySQL database:

```console
kubectl -n demo delete -f cluster/examples/workloads/wordpress-${provider}/workload.yaml
```

Then delete the EKS cluster:

```console
kubectl delete -f cluster/examples/workloads/wordpress-${provider}/cluster.yaml
```

Finally, delete the provider credentials:

```console
kubectl delete -f cluster/examples/workloads/wordpress-${provider}/provider.yaml
```

> Note: There may still be an ELB that was not properly cleaned up, and you will need
to go to EC2 > ELBs and delete it manually.

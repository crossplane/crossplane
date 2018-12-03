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
    > https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-11-07/amazon-eks-vpc-sample.yaml
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
1. Replace EKS_VPC, EKS_ROLE_ARN, EKS_SUBNETS, EKS_SECURITY_GROUP in cluster/examples/workloads/wordpress-aws/provider.yaml with values from previous step (vpcId, subnetIds, securityGroupIds). Note EKS_SECURITY_GROUP needs to be replaced twice in file.
1. Replace REGION in cluster/examples/workloads/wordpress-aws/provider.yaml with the region you selected in VPC creation.

### Create an RDS subnet group
1. Navigate to aws console in same region as eks clsuter
1. Navigate to `RDS` service
1. Naviate to `Subnet groups` in left hand pane
1. Click `Create DB Subnet Group`
1. Name your subnet i.e. eks-db-subnets
1. Select the VPC created in the EKS VPC step
1. Click `Add all subnets related to this VPC`
1. Click Create
1. Replace DBSubnet name in cluster/examples/workloads/wordpress-aws/provider.yaml in RDS_SUBNET_GROUP

### Create an RDS Security Group (demo only)

**Note**: This will make your RDS instance visible from Anywhere on the internet. This if for **DEMO PURPOSES ONLY**, and
is **NOT RECOMMENDED** for production system.

1. Navigate to ec2 in the region of the EKS cluster
1. Navigate to security groups
1. Select the same VPC from the EKS cluster.
1. On the Inbound Rules tab, choose Edit.
    - For Type, choose `MYSQL/Aurora`
    - For Port Range, type `3306`
    - For Source, choose `Anywere` from drop down or type: `0.0.0.0/0`
1. Choose Add another rule if you need to add more IP addresses or different port ranges.
1. Replace RDS_SECURITY_GROUP in cluster/examples/workloads/wordpress-aws/provider.yaml with the security group we just created.

## Deploy Wordpress Resources

Now deploy all the Wordpress provider and workload resources, including the RDS database, and EKS cluster with the following single commands:

```console
kubectl -n demo create -f cluster/examples/workloads/wordpress-aws/provider.yaml
kubectl -n demo create -f cluster/examples/workloads/wordpress-aws/workload.yaml
```

Now you can proceed back to the main quickstart to [wait for the resources to become ready](quickstart.md#waiting-for-completion).

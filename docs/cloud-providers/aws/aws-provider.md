# Adding Amazon Web Services (AWS) to Crossplane

In this guide, we will walk through the steps necessary to configure your AWS account to be ready for integration with Crossplane.

## AWS Credentials

### Option 1: aws Command Line Tool

If you have already installed and configured the [`aws` command line tool](https://aws.amazon.com/cli/), you can simply find your AWS credentials file in `~/.aws/credentials`.

#### Using `aws-credentials.sh`

In the `cluster/examples` directory you will find a helper script, `aws-credentials.sh`.  This script will use the `aws` CLI to create the necessary AWS components for the Crossplane examples.  Running the final output of this command will configure your Crossplane AWS provider, RDS, and EKS resource classes.

```console
$ ./cluster/examples/aws-credentials.sh
Waiting for 'CREATE_COMPLETE' from Cloudformation Stack crossplane-example-stack-25077.......................
#
# Run the following for the variables that are used throughout the AWS example projects
#
export BASE64ENCODED_AWS_PROVIDER_CREDS=$(base64 -w0 < ~/.aws/credentials)
export EKS_WORKER_KEY_NAME=crossplane-example-25077
export EKS_ROLE_ARN=arn:aws:iam::987654321234:role/crossplane-example-role-25077
export REGION=eu-west-1
export EKS_VPC=vpc-085444e4ce26b55e8
export EKS_SUBNETS=subnet-08ad61800a39c537a,subnet-0d05d23815bed79be,subnet-07adcb08485e186fc
export EKS_SECURITY_GROUP=sg-09aaba94fe7050cf8
export RDS_SUBNET_GROUP_NAME=crossplane-example-db-subnet-group-25077
export RDS_SECURITY_GROUP=sg-0b586dbd763fb35ad

#
# For example, to use this environment as an AWS Crossplane provider:
#
sed -e "s|BASE64ENCODED_AWS_PROVIDER_CREDS|$(base64 -w0 < /home/marques/.aws/credentials)|g" \
    -e "s|EKS_WORKER_KEY_NAME|$EKS_WORKER_KEY_NAME|g" \
    -e "s|EKS_ROLE_ARN|$EKS_ROLE_ARN|g" \
    -e "s|REGION|$REGION|g" \
    -e "s|EKS_VPC|$EKS_VPC|g" \
    -e "s|EKS_SUBNETS|$EKS_SUBNETS|g" \
    -e "s|EKS_SECURITY_GROUP|$EKS_SECURITY_GROUP|g" \
    -e "s|RDS_SUBNET_GROUP_NAME|$RDS_SUBNET_GROUP_NAME|g" \
    -e "s|RDS_SECURITY_GROUP|$RDS_SECURITY_GROUP|g" \
    cluster/examples/workloads/kubernetes/wordpress-aws/provider.yaml | kubectl apply -f -

# Clean up after this script by deleting everything it created:
# ./cluster/examples/aws-credentials.sh delete 25077
```

After running `gcp-credentials.sh`, a series of `export` commands will be shown.  Copy and paste the `export` commands that are provided.  These variable names will be referenced throughout the Crossplane examples, generally with a `sed` command.

You will also see a `sed` command.  This command will configure the AWS Crossplane provider using the environment that was created by the `aws-credentials.sh` script.

When you are done with the examples and have deleted all of the AWS artifacts project artifacts you can use the delete command provided by the `aws-credentials.sh` script to remove the CloudFormation Stack, VPC, and other artifacts of the script.

*Note* The AWS artifacts should be removed first using Kubernetes commands (`kubectl delete ...`) as each example will explain.

### Option 2: AWS Console in Web Browser

If you do not have the `aws` tool installed, you can alternatively log into the [AWS console](https://aws.amazon.com/console/) and export the credentials.
The steps to follow below are from the [AWS SDK for GO](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html):

1. Open the IAM console.
1. On the navigation menu, choose Users.
1. Choose your IAM user name (not the check box).
1. Open the Security credentials tab, and then choose Create access key.
1. To see the new access key, choose Show. Your credentials resemble the following:
  - Access key ID: AKIAIOSFODNN7EXAMPLE
  - Secret access key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
1. To download the key pair, choose Download .csv file.

Then convert the `*.csv` file to the below format and save it to `~/.aws/credentials`:

```
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

After the steps above, you should have your AWS credentials stored in `~/.aws/credentials`.

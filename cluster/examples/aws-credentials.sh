#!/usr/bin/env bash
#
# This is a helper script that uses ~/.aws/credentials and the aws CLI to create an
# environment of EKS, RDS, VPC, Security and Subnet Groups for use in Crossplane AWS examples
#
# aws is required and must be configured with privileges to perform these tasks
#
set -e -o pipefail
RAND=$RANDOM

if ! command -v aws > /dev/null; then
	echo "Please install aws: https://aws.amazon.com/cli/)"
	exit 1
fi

if ! command -v jq > /dev/null; then
	echo "Please install jq: https://stedolan.github.io/jq/download/)"
	exit 1
fi

# must be one of us-west-2,us-east-1,eu-west-1 at time of writing for EKS
export REGION=eu-west-1
export KEYFILE=~/.aws/credentials
export EKS_WORKER_KEY_NAME=crossplane-example-$RAND
export RDS_SUBNET_GROUP_NAME=crossplane-example-db-subnet-group-$RAND

# These variables are not used in yaml, just for the cli provisioning
export EKS_ROLE_NAME=crossplane-example-role-$RAND
export EKS_STACK_NAME=crossplane-example-stack-$RAND


# delete everything that was created by this script give the random project id
crossplane_delete_aws() {
    RAND=$1
    EKS_WORKER_KEY_NAME=crossplane-example-$RAND
    RDS_SUBNET_GROUP_NAME=crossplane-example-db-subnet-group-$RAND
    EKS_ROLE_NAME=crossplane-example-role-$RAND
    EKS_STACK_NAME=crossplane-example-stack-$RAND
    RDS_SECURITY_GROUP_NAME=crossplane-example-$RAND
    RDS_SECURITY_GROUP_ID=$(aws ec2 describe-security-groups --region=$REGION --filter Name=group-name,Values=$RDS_SECURITY_GROUP_NAME --output=text --query="SecurityGroups[0].GroupId")

    aws ec2 delete-key-pair --region $REGION --key-name $EKS_WORKER_KEY_NAME
    aws iam detach-role-policy --region $REGION --role-name $EKS_ROLE_NAME --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy
    aws iam detach-role-policy --region $REGION --role-name $EKS_ROLE_NAME --policy-arn arn:aws:iam::aws:policy/AmazonEKSServicePolicy
    aws iam delete-role --region $REGION --role-name $EKS_ROLE_NAME
    aws cloudformation delete-stack --region $REGION --stack-name $EKS_STACK_NAME
    aws rds delete-db-subnet-group --region=$REGION --db-subnet-group-name=$RDS_SUBNET_GROUP_NAME
    aws ec2 revoke-security-group-ingress --region $REGION --group-id $RDS_SECURITY_GROUP_ID --port 3306 --protocol tcp
    aws ec2 delete-security-group --region $REGION --group-id $RDS_SECURITY_GROUP_ID
}

if [[ "$1" == "delete" ]]; then
  if [[ -n "$2" ]]; then
    crossplane_delete_aws $2
    exit $?
  else
    echo "$0 delete [example-id]"
  fi
  exit 1
fi


# Generate a KeyPair 
aws ec2 create-key-pair --key-name $EKS_WORKER_KEY_NAME --region=$REGION > /dev/null

# Generate a Role that can Do everying necessary to provider EKS clusters
aws iam create-role --role-name $EKS_ROLE_NAME --region $REGION --assume-role-policy-document file://<(echo '{"Version": "2012-10-17","Statement": {"Effect": "Allow", "Principal": {"Service": "eks.amazonaws.com"},"Action": "sts:AssumeRole"}}')  > /dev/null

aws iam attach-role-policy --role-name $EKS_ROLE_NAME --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy > /dev/null
aws iam attach-role-policy --role-name $EKS_ROLE_NAME --policy-arn arn:aws:iam::aws:policy/AmazonEKSServicePolicy > /dev/null

export EKS_ROLE_ARN=$(aws iam get-role --role-name $EKS_ROLE_NAME | jq -r .Role.Arn)

# Generate and run a CloudFormation Stack and get the VPC, Subnet, and Security Group associated with it
aws cloudformation create-stack \
    --template-url=https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-11-07/amazon-eks-vpc-sample.yaml \
    --region $REGION \
    --stack-name $EKS_STACK_NAME \
    --parameters ParameterKey=VpcBlock,ParameterValue=192.168.0.0/16 ParameterKey=Subnet01Block,ParameterValue=192.168.64.0/18 ParameterKey=Subnet02Block,ParameterValue=192.168.128.0/18 ParameterKey=Subnet03Block,ParameterValue=192.168.192.0/18  > /dev/null

echo -n "Waiting for 'CREATE_COMPLETE' from Cloudformation Stack $EKS_STACK_NAME"
until [[ "CREATE_COMPLETE" == "$(aws cloudformation describe-stacks --stack-name $EKS_STACK_NAME --region $REGION | jq -r '.Stacks[0].StackStatus')" ]]; do
  echo -n "."
  sleep 2
done;
echo

export EKS_VPC=$(aws cloudformation describe-stacks --stack-name $EKS_STACK_NAME --region $REGION | jq -r '.Stacks[0].Outputs[]|select(.OutputKey=="VpcId").OutputValue')
export EKS_SUBNETS=$(aws cloudformation describe-stacks --stack-name $EKS_STACK_NAME --region $REGION | jq -r '.Stacks[0].Outputs[]|select(.OutputKey=="SubnetIds").OutputValue')
export EKS_SECURITY_GROUP=$(aws cloudformation describe-stacks --stack-name $EKS_STACK_NAME --region $REGION | jq -r '.Stacks[0].Outputs[]|select(.OutputKey=="SecurityGroups").OutputValue')

T="${EKS_SUBNETS//,/ }"
aws rds create-db-subnet-group --region=$REGION --db-subnet-group-name=$RDS_SUBNET_GROUP_NAME --db-subnet-group-description="crossplane-example-$RAND EKS VPC $EKS_VPC to RDS" --subnet-ids $T > /dev/null

# Generate the Security Group and add MySQL ingress to it

aws ec2 create-security-group --vpc-id=$EKS_VPC --region=$REGION --group-name="crossplane-example-$RAND" --description="open mysql access for crossplane-example-$RAND" > /dev/null
export RDS_SECURITY_GROUP=$(aws ec2 describe-security-groups --filter Name=group-name,Values=crossplane-example-$RAND --region=$REGION --output=text --query="SecurityGroups[0].GroupId")

aws ec2 authorize-security-group-ingress --protocol=tcp --port=3306 --region=$REGION --cidr=0.0.0.0/0 --group-id=$RDS_SECURITY_GROUP  > /dev/null


cat <<EOS
#
# Run the following for the variables that are used throughout the AWS example projects
#
export BASE64ENCODED_AWS_PROVIDER_CREDS=\$(base64 $KEYFILE | tr -d "\n")
export EKS_WORKER_KEY_NAME=$EKS_WORKER_KEY_NAME
export EKS_ROLE_ARN=$EKS_ROLE_ARN
export REGION=$REGION
export EKS_VPC=$EKS_VPC
export EKS_SUBNETS=$EKS_SUBNETS
export EKS_SECURITY_GROUP=$EKS_SECURITY_GROUP
export RDS_SUBNET_GROUP_NAME=$RDS_SUBNET_GROUP_NAME
export RDS_SECURITY_GROUP=$RDS_SECURITY_GROUP

#
# Use this environment in an AWS Crossplane provider:
#
sed -e "s|BASE64ENCODED_AWS_PROVIDER_CREDS|\$(base64 $KEYFILE | tr -d "\n")|g" \\
    -e "s|EKS_WORKER_KEY_NAME|\$EKS_WORKER_KEY_NAME|g" \\
    -e "s|EKS_ROLE_ARN|\$EKS_ROLE_ARN|g" \\
    -e "s|REGION|\$REGION|g" \\
    -e "s|EKS_VPC|\$EKS_VPC|g" \\
    -e "s|EKS_SUBNETS|\$EKS_SUBNETS|g" \\
    -e "s|EKS_SECURITY_GROUP|\$EKS_SECURITY_GROUP|g" \\
    -e "s|RDS_SUBNET_GROUP_NAME|\$RDS_SUBNET_GROUP_NAME|g" \\
    -e "s|RDS_SECURITY_GROUP|\$RDS_SECURITY_GROUP|g" \\
    cluster/examples/workloads/kubernetes/wordpress-aws/provider.yaml | kubectl apply -f -

# Clean up after this script by deleting everything it created:
# $0 delete $RAND
EOS
echo

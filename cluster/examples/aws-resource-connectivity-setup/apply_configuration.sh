#!/usr/bin/env bash
#
# This is a helper script that uses ~/.aws/credentials and ~/.aws/config
# to create a set of managed aws resources which provide an environment
# in which other aws managed resources (EKS, RDS, etc...) can connect
#
# aws configuration (credentials and default region) is required to 
# perform these tasks

set -e -o pipefail

# create the namespace 
kubectl apply -f namespace.yaml

kubectl apply -f iamrole.yaml
kubectl -n aws-connectivity wait --for=condition=Ready IAMRole/eks-cluster-role --timeout=1m

kubectl apply -f iamrole_policies.yaml
kubectl -n aws-connectivity wait --for=condition=Ready IAMRolePolicyAttachment --all --timeout=1m


kubectl apply -f vpc.yaml
kubectl -n aws-connectivity wait --for=condition=Ready vpc/rc-vpc --timeout=1m


VPC_ID=$(kubectl -n aws-connectivity get vpcs rc-vpc -o=jsonpath='{.status.vpcId}')
AWS_REGION=$(kubectl -n crossplane-system get provider aws-provider -o=jsonpath='{.spec.region}')


cat subnets.yaml | sed \
  -e "s|((VPC_ID))|"$VPC_ID"|g" \
  -e "s|((AWS_REGION))|"$AWS_REGION"|g" \
| kubectl apply -f -

kubectl -n aws-connectivity wait --for=condition=Ready subnet --all --timeout=1m

SUBNET1_ID=$(kubectl -n aws-connectivity get subnets rc-subnet1 -o=jsonpath='{.status.subnetId}')
SUBNET2_ID=$(kubectl -n aws-connectivity get subnets rc-subnet2 -o=jsonpath='{.status.subnetId}')
SUBNET3_ID=$(kubectl -n aws-connectivity get subnets rc-subnet3 -o=jsonpath='{.status.subnetId}')


cat internetgateway.yaml | sed \
  -e "s|((VPC_ID))|"$VPC_ID"|g" \
| kubectl apply -f -

kubectl -n aws-connectivity wait --for=condition=Ready internetgateway --all --timeout=1m

IG_ID=$(kubectl -n aws-connectivity get internetgateways rc-internetgateway -o=jsonpath='{.status.internetGatewayId}')

cat routetable.yaml | sed \
  -e "s|((VPC_ID))|"$VPC_ID"|g" \
  -e "s|((IG_ID))|"$IG_ID"|g" \
  -e "s|((SUBNET1_ID))|"$SUBNET1_ID"|g" \
  -e "s|((SUBNET2_ID))|"$SUBNET2_ID"|g" \
  -e "s|((SUBNET3_ID))|"$SUBNET3_ID"|g" \
| kubectl apply -f -

kubectl -n aws-connectivity wait --for=condition=Ready routetables --all --timeout=1m

cat cp-securitygroup.yaml | sed \
  -e "s|((VPC_ID))|"$VPC_ID"|g" \
| kubectl apply -f -

kubectl -n aws-connectivity wait --for=condition=Ready securitygroups/rc-controlplane-sg --timeout=1m



cat dbsubnetgroup.yaml | sed \
  -e "s|((SUBNET1_ID))|"$SUBNET1_ID"|g" \
  -e "s|((SUBNET2_ID))|"$SUBNET2_ID"|g" \
  -e "s|((SUBNET3_ID))|"$SUBNET3_ID"|g" \
| kubectl apply -f -

kubectl -n aws-connectivity wait --for=condition=Ready dbsubnetgroups --all --timeout=1m


cat rds-securitygroup.yaml | sed \
  -e "s|((VPC_ID))|"$VPC_ID"|g" \
| kubectl apply -f -

kubectl -n aws-connectivity wait --for=condition=Ready securitygroups/rc-rds-sg --timeout=1m


########################################
# populate resource classes
EKS_ROLE_ARN=$(kubectl -n aws-connectivity get iamroles/eks-cluster-role  -o=jsonpath='{.status.arn}')
EKS_SECURITY_GROUP_ID=$(kubectl -n aws-connectivity get securitygroups/rc-controlplane-sg  -o=jsonpath='{.status.securityGroupID}')
RDS_SECURITY_GROUP_ID=$(kubectl -n aws-connectivity get securitygroups/rc-rds-sg  -o=jsonpath='{.status.securityGroupID}')
RDS_SUBNET_GROUP_NAME=$(kubectl -n aws-connectivity get dbsubnetgroup/rc-dbsubnetgroup -o=jsonpath='{.spec.groupName}')

cat resource-classes.yaml | sed \
  -e "s|((RDS_SECURITY_GROUP_ID))|"$RDS_SECURITY_GROUP_ID"|g" \
  -e "s|((RDS_SUBNET_GROUP_NAME))|"$RDS_SUBNET_GROUP_NAME"|g" \
  -e "s|((AWS_REGION))|"$AWS_REGION"|g" \
  -e "s|((EKS_ROLE_ARN))|"$EKS_ROLE_ARN"|g" \
  -e "s|((VPC_ID))|"$VPC_ID"|g" \
  -e "s|((EKS_SECURITY_GROUP_ID))|"$EKS_SECURITY_GROUP_ID"|g" \
  -e "s|((SUBNET1_ID))|"$SUBNET1_ID"|g" \
  -e "s|((SUBNET2_ID))|"$SUBNET2_ID"|g" \
  -e "s|((SUBNET3_ID))|"$SUBNET3_ID"|g" \
| kubectl apply -f -

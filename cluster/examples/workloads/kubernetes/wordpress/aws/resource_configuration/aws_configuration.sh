#!/usr/bin/env bash
#
# This is a helper script that installs an aws configuration, on top of
# which other aws managed resources (EKS, RDS, etc...) can communicate
# It also could cleanup an existing configuration
# 
# aws provider has to be created and applied to crossplane, prior to
# performing this script

set -e -o pipefail

# change to script directory
cd "$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd )"

CONFIG_NAME=
NAMESPACE=
COMMAND=

while (( "$#" )); do
  if test -z "$2"; then
    echo "invalid value for $1 option"
    exit -1
  fi
  case "$1" in
    -a|--action)
      COMMAND=$2
      shift 2
      ;;
    -c|--config-name)
      CONFIG_NAME=$2
      shift 2
      ;;
    -n|--namespace)
      NAMESPACE=$2
      shift 2
      ;;
    *) 
      echo "unknown option $1"
      exit -1
      shift
      ;;
  esac
done

# make sure the COMMAND is provided
if test -z "$COMMAND"; then
  echo "no COMMAND is given. 'apply' or 'cleanup' is expected"
  exit -1
fi

# make sure the CONFIG_NAME is provided
if test -z "$CONFIG_NAME"; then
  echo "no CONFIG_NAME name is given"
  exit -1
fi

# make sure the provided CONFIG_NAME has valid characters
if ! [[ "$CONFIG_NAME" =~ ^[a-z0-9-]*$ ]]; then
  echo "the provided CONFIG_NAME '${CONFIG_NAME}' must match the pattern [a-z0-9-]*"
  exit -1
fi

# make sure the NAMESPACE is set
if test -z "$NAMESPACE"; then
  echo "no NAMESPACE name is given"
  exit -1
fi

# if NAMESPACE doesn't exit, create it
if ! kubectl get namespace | awk 'NR!=1{print $1}'| grep -q "$NAMESPACE"; then
  echo "namespace $NAMESPACE doesn't exist"
  exit -1
fi

# make sure kubectl is configured
kubectl cluster-info > /dev/null || echo "KUBECONFIG is not configured properly"

AWS_REGION=$(kubectl -n crossplane-system get provider aws-provider -o=jsonpath='{.spec.region}')

# substitue_variables function accepts a file name, substitutes variable names
# with their values, and prints out the result to stdout
# a given file might only have a subset of env variables.
function substitue_variables {
  sed \
    -e "s|((CONFIG_NAME))|"$CONFIG_NAME"|g" \
    -e "s|((NAMESPACE))|"$NAMESPACE"|g" \
    -e "s|((AWS_REGION))|"$AWS_REGION"|g" \
    -e "s|((VPC_ID))|"$VPC_ID"|g" \
    -e "s|((IG_ID))|"$IG_ID"|g" \
    -e "s|((SUBNET1_ID))|"$SUBNET1_ID"|g" \
    -e "s|((SUBNET2_ID))|"$SUBNET2_ID"|g" \
    -e "s|((SUBNET3_ID))|"$SUBNET3_ID"|g" \
    -e "s|((RDS_SECURITY_GROUP_ID))|"$RDS_SECURITY_GROUP_ID"|g" \
    -e "s|((RDS_SUBNET_GROUP_NAME))|"$RDS_SUBNET_GROUP_NAME"|g" \
    -e "s|((EKS_ROLE_ARN))|"$EKS_ROLE_ARN"|g" \
    -e "s|((EKS_SECURITY_GROUP_ID))|"$EKS_SECURITY_GROUP_ID"|g" \
    "$1"
}

# apply_and_wait_until_ready first substitutes the variabels, then
# applies the object, and then waits until the resource is ready
function apply_and_wait_until_ready {
  echo "applying $1..."
  k8s_object=$(substitue_variables "$1")
  echo "$k8s_object" | kubectl apply -f -
  echo "$k8s_object" | kubectl wait --for=condition=Ready -f - > /dev/null
  all_objects="${all_objects}
---
${k8s_object}"
}

# delete_resource substitues the variables and then deletes it
function delete_resource {
  echo "deleting $1..."
  substitue_variables "$1" | kubectl delete -f -
}

function cleanup {
  echo "cleaning up..."

  # ignore errors in deleting resources, as some resources might already been deleted
  set +e

  delete_resource rds-securitygroup.yaml
  delete_resource dbsubnetgroup.yaml
  delete_resource eks-securitygroup.yaml
  delete_resource routetable.yaml
  delete_resource internetgateway.yaml
  delete_resource subnets.yaml
  delete_resource iamrole_policies.yaml
  delete_resource iamrole.yaml

  # retrieve vpc_id to delete orphan dependants. Otherwise it won't get deleted
  VPC_ID=$(substitue_variables vpc.yaml | kubectl get -o=jsonpath='{.items[0].status.vpcId}' -f -) || echo "cannot retrieve vpcID - skipping deleting vpc related orphan resources"
  if ! test -z "$VPC_ID"; then

    echo "deleting vpc orphan resources ..."
    # delete all the load balancers that are externally created in the vpc by k8s
    vpc_loadbalancers=($(aws elb describe-load-balancers | jq -rc --arg VPC_ID "$VPC_ID" '.LoadBalancerDescriptions | .[] | select(.VPCId == $VPC_ID) | .LoadBalancerName' ))  
    for (( i=0; i<${#vpc_loadbalancers[@]}; i++ )); do
      echo "deleting load balancer ${vpc_loadbalancers[i]}"
      aws elb delete-load-balancer --load-balancer-name ${vpc_loadbalancers[i]}
    done

    # delete all the non-default security groups that are externally created in the vpc by k8s
    vpc_securitygroups=($(aws ec2 describe-security-groups | jq -rc --arg VPC_ID "$VPC_ID" '.SecurityGroups | .[] | select(.VpcId == $VPC_ID and .GroupName!="default") | .GroupId'))
    for (( i=0; i<${#vpc_securitygroups[@]}; i++ )); do
      echo "deleting security group ${vpc_securitygroups[i]}"
      aws ec2 delete-security-group --group-id ${vpc_securitygroups[i]}
    done
  fi

  delete_resource vpc.yaml

  echo "Successfully cleaned up all managed configuration resources"
}

function apply {
  echo "applying configuration ${CONFIG_NAME} into namespace ${NAMESPACE}"
  

  all_objects=""

  apply_and_wait_until_ready iamrole.yaml
  EKS_ROLE_ARN=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[0].status.arn}' -f -)

  apply_and_wait_until_ready iamrole_policies.yaml

  apply_and_wait_until_ready vpc.yaml
  VPC_ID=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[0].status.vpcId}' -f -)

  apply_and_wait_until_ready subnets.yaml
  SUBNET1_ID=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[0].status.subnetId}' -f -)
  SUBNET2_ID=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[1].status.subnetId}' -f -)
  SUBNET3_ID=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[2].status.subnetId}' -f -)

  apply_and_wait_until_ready internetgateway.yaml
  IG_ID=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[0].status.internetGatewayId}' -f -)

  apply_and_wait_until_ready routetable.yaml

  apply_and_wait_until_ready eks-securitygroup.yaml
  EKS_SECURITY_GROUP_ID=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[0].status.securityGroupID}' -f -)

  apply_and_wait_until_ready dbsubnetgroup.yaml
  RDS_SUBNET_GROUP_NAME=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[0].spec.groupName}' -f -)

  apply_and_wait_until_ready rds-securitygroup.yaml
  RDS_SECURITY_GROUP_ID=$(echo "$k8s_object" | kubectl get -o=jsonpath='{.items[0].status.securityGroupID}' -f -)


  echo "Successfully applied all configuration resources"
  echo "${all_objects}" | kubectl get -f -

  ########################################
  # populate resource classes

  substitue_variables resource-classes.yaml \
  | kubectl apply -f -
}

case "$COMMAND" in
  apply)
    apply
    ;;
  cleanup)
    cleanup
    ;;
  *)
    echo "command $COMMAND is not a valid command"
    exit 1
esac
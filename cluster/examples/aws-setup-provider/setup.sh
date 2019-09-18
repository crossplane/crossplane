#!/usr/bin/env bash
#
# This is a helper script that uses ~/.aws/credentials and ~/.aws/config
# to build an aws provider object
#
# aws configuration (credentials and default region) is required for this
# script

set -e -o pipefail

# change to script directory
cd "$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd )"

aws_profile=

while (( "$#" )); do
  if test -z "$2"; then
    echo "invalid value for $1 option"
    exit -1
  fi
  case "$1" in
    -p|--profile)
      aws_profile=$2
      shift 2
      ;;
    *) 
      shift
      ;;
  esac
done

# make sure kubectl is configured
kubectl cluster-info > /dev/null || echo "KUBECONFIG is not configured properly"

# if aws_profile is not provided, use default
aws_profile="${aws_profile:-default}"

# if region is not provided, retrieve aws profile region from config
AWS_REGION=$(awk '/["$aws_profile"]/ {getline; print $3}' ${HOME}/.aws/config)

# retrieve aws profile credentials, save it under 'default' profile, and base64 encode it
AWS_CREDS_BASE64=$(cat ${HOME}/.aws/credentials | awk '/["$aws_profile"]/ {getline; print $0}' | awk 'NR==1{print "[default]"}1' | base64 | tr -d "\n")

if test -z "$AWS_REGION"; then
  echo "error retrieving region from aws config. "
  exit -1
fi

if test -z "$AWS_CREDS_BASE64"; then
  echo "error reading credentials from aws config"
  exit -1
fi

# build the secret and provider objects, and then apply it
cat provider.yaml | sed \
  -e "s|((AWS_REGION))|"$AWS_REGION"|g" \
  -e "s|((AWS_CREDS_BASE64))|"$AWS_CREDS_BASE64"|g" \
  | kubectl apply -f -
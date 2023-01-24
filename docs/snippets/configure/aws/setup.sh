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

# retrieve aws profile credentials, save it under 'default' profile, and base64 encode it
AWS_CREDS_BASE64=$(echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $aws_profile)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $aws_profile)" | base64  | tr -d "\n")

if test -z "$AWS_CREDS_BASE64"; then
  echo "error reading credentials from aws config"
  exit 1
fi

echo "apiVersion: v1
data:
  creds: $AWS_CREDS_BASE64
kind: Secret
metadata:
  name: aws-creds
  namespace: crossplane-system
type: Opaque" | kubectl apply -f -

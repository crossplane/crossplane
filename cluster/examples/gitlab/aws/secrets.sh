#!/usr/bin/env bash

SOURCE_CONTEXT=${1:-minikube}
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

DECLARE_OPTS=-A
BASE64_D_OPTS=-d

if [ "$(uname)" == "Darwin" ]; then
    DECLARE_OPTS=-a
    BASE64_D_OPTS=-D
fi

# Generate content of AWS connection.yaml file used in GitLab bucket secrets
#
# $1 - AWS service account credentials data
#
generate_connection_file() {
    local access_key=$1
    local secret_key=$2
    local bucket=$3

cat << EOF
s3:
  bucket: ${bucket}
  accesskey: ${access_key}
  secretkey: ${secret_key}
  region: us-east-1
  v4auth: true

EOF
}

# Generate content of AWS s3cmd.properties file used in GitLab backup bucket secrets
#
# $1 Interoperability Access Key value
# $2 Interoperability Secret value
#
generate_s3cmd_file () {
    local access_key=$1
    local secret_key=$2
cat << EOF
[default]
access_key = ${access_key}
secret_key = ${secret_key}
bucket_location = us-east-1

EOF
}

# Process crossplane bucket connection secrets and create secrets in GitLab expected format, as well as
# GitLab Helm values file with bucket configuration
buckets () {
    declare ${DECLARE_OPTS} buckets

    # use claim file names as bucket name enumerator
    for f in ./cluster/examples/gitlab/aws/resource-claims/buckets/*; do
        bucket=$(basename ${f} .yaml)

        # retrieve interoperability access key and secret
        bucket_name=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-${bucket} -ojson | jq -r '.data.bucketName' | base64 ${BASE64_D_OPTS})
        interop_access_key=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-${bucket} -ojson | jq -r '.data.username' | base64 ${BASE64_D_OPTS})
        interop_secret=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-${bucket} -ojson | jq -r '.data.password' | base64 ${BASE64_D_OPTS})

        # create different secrets based on the bucket "type"
        if [[ ${bucket} == 'backups'* ]]; then
            # for backup buckets we generate secret in `s3cmd.properties` format
            value=$(generate_s3cmd_file ${interop_access_key} ${interop_secret})
            kubectl create secret generic bucket-${bucket} --from-literal=config="${value}"
        else
            # for all other buckets we generate secret in `connection.yaml` format
            value=$(generate_connection_file ${interop_access_key} ${interop_secret} ${bucket_name})
            kubectl create secret generic bucket-${bucket} --from-literal=connection="${value}"
        fi

        buckets[${bucket}]=${bucket_name}
    done

cat > ${DIR}/values-buckets.yaml << EOF
global:
  minio:
    enabled: false
  appConfig:
    lfs:
      bucket: ${buckets[lfs]}
      connection:
        secret: bucket-lfs
    artifacts:
      bucket: ${buckets[artifacts]}
      connection:
        secret: bucket-artifacts
    uploads:
      bucket: ${buckets[uploads]}
      connection:
        secret: bucket-uploads
    packages:
      bucket: ${buckets[packages]}
      connection:
        secret: bucket-packages
    pseudonymizer:
      configMap:
      bucket: ${buckets[pseudonymizer]}
      connection:
        secret: bucket-pseudonymizer
    backups:
      bucket: ${buckets[backups]}
      tmpBucket: bucketname-backups-tmp
gitlab:
  task-runner:
    backups:
      objectStorage:
        config:
          secret: bucket-backups
EOF
}

# Generate the content of Helm values-psql.yaml
#
# $1 postgresql host name (postgresql endpoint ip address)
# $2 postgresql username
#
postgresql_values_file () {
    local host=${1}
    local username=${2}
cat << EOF
global:
  psql:
    password:
      secret: gitlab-postgresql
      key: password
    host: ${host}
    database: postgres
    username: ${username}
postgresql:
  install: false
EOF
}

# Process crossplane postgres connection secret and generate Helm values file for postgresql in addition to creating
# postgres secret for GitLab application
posgtresql () {
    meta='del(.metadata.namespace,.metadata.ownerReferences,.metadata.uid,.metadata.creationTimestamp,.metadata.selfLink,.metadata.resourceVersion)'
    kubectl --context=${SOURCE_CONTEXT} get secret gitlab-postgresql -o json | jq ${meta} | kubectl create -f -

    host=$(kubectl get secret gitlab-postgresql -o json | jq -r '.data.endpoint' | base64 ${BASE64_D_OPTS})
    user=$(kubectl get secret gitlab-postgresql -o json | jq -r '.data.username' | base64 ${BASE64_D_OPTS})
    postgresql_values_file ${host} ${user} > ${DIR}/values-psql.yaml
}

# Generate the content of Helm values-redis.yaml file
#
# $1 host - ip address value for redis instance
#
redis_values_file () {
    local host=${1}
cat << EOF
global:
  redis:
    password:
      enabled: false
    host: ${host}
redis:
  enabled: false
EOF
}

# Process crossplane redis connection secret and generate Helm values file for redis.
redis () {
    host=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-redis -o json | jq -r '.data.endpoint' | base64 ${BASE64_D_OPTS})
    redis_values_file ${host} > ${DIR}/values-redis.yaml
}

#
# Process crossplane claim connection secrets
#
echo "Source cluster kubectl context: ${SOURCE_CONTEXT}"
echo "Current cluster kubectl context: $(kubectl config current-context)"
echo ---
echo "Source cluster secrets:"
kubectl --context=${SOURCE_CONTEXT} get secrets

echo ---
echo "Generate PostgreSQL secret and values file"
posgtresql

echo ---
echo "Generate Redis values file"
redis

echo ---
echo "Generate Buckets secrets"
buckets

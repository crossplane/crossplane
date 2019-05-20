#!/usr/bin/env bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

DECLARE_OPTS=-A
BASE64_D_OPTS=-d

if [ "$(uname)" == "Darwin" ]; then
    DECLARE_OPTS=-a
    BASE64_D_OPTS=-D
fi

# Generate content of GCP connection.yaml file used in GitLab bucket secrets
#
# $1 - GCP service account credentials key.json data
#
generate_connection_file() {
    local json_key=$1

    # retrieve project id value from the credentials json
    project_id=$(echo ${json_key} | jq -r '.project_id')

    # retrieve project id value from the credentials json
    client_email=$(echo ${json_key} | jq -r '.client_email')

cat << EOF
provider: Google
google_project: ${project_id}
google_client_email: ${client_email}
google_json_key_string: |
EOF
    echo ${json_key} | jq '.' | awk '{printf "  %s\n", $0}'
    echo
}

# Generate content of GCP s3cmd.properties file used in GitLab backup bucket secrets
#
# $1 Interoperability Access Key value
# $2 Interoperability Secret value
#
generate_s3cmd_file () {
    local access_key=$1
    local secret_key=$2
cat << EOF
[default]
host_base = storage.googleapis.com
host_bucket = storage.googleapis.com
use_https = True
signature_v2 = True

# Access and secret key can be generated in the interoperability
# https://console.cloud.google.com/storage/settings
# See Docs: https://cloud.google.com/storage/docs/interoperability
access_key = ${access_key}
secret_key = ${secret_key}

# Multipart needs to be disabled for GCS !
enable_multipart = False

EOF
}

# Process crossplane bucket connection secrets and create secrets in GitLab expected format, as well as
# GitLab Helm values file with bucket configuration
buckets () {
    declare ${DECLARE_OPTS} buckets

    # use claim file names as bucket name enumerator
    for f in ./cluster/examples/gitlab/gcp/resource-claims/buckets/*; do
        bucket=$(basename ${f} .yaml)

        # retrieve interoperability access key and secret
        bucket_name=$(kubectl get secret gitlab-${bucket} -ojson | jq -r '.data.endpoint' | base64 ${BASE64_D_OPTS})
        interop_access_key=$(kubectl get secret gitlab-${bucket} -ojson | jq -r '.data.username' | base64 ${BASE64_D_OPTS})
        interop_secret=$(kubectl get secret gitlab-${bucket} -ojson | jq -r '.data.password' | base64 ${BASE64_D_OPTS})

        # retrieve service account key.json
        key_json=$(kubectl get secret gitlab-${bucket} -ojson | jq -r '.data.token' | base64 ${BASE64_D_OPTS})

        # create different secrets based on the bucket "type"
        if [[ ${bucket} == 'backups'* ]]; then
            # for backup buckets we generate secret in `s3cmd.properties` format
            value=$(generate_s3cmd_file ${interop_access_key} ${interop_secret})
            kubectl create secret generic bucket-${bucket} --from-literal=config="${value}"
        else
            # for all other buckets we generate secret in `connection.yaml` format
            value=$(generate_connection_file "${key_json}")
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

# Process crossplane postgres connection secret and generate Helm values file for postgresql
posgtresql () {
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
    host=$(kubectl get secret gitlab-redis -o json | jq -r '.data.endpoint' | base64 ${BASE64_D_OPTS})
    redis_values_file ${host} > ${DIR}/values-redis.yaml
}

#
# Process crossplane claim connection secrets
#
echo "Current cluster kubectl context: $(kubectl config current-context)"
echo ---

echo "Generate PostgreSQL values file"
posgtresql

echo ---
echo "Generate Redis values file"
redis

echo ---
echo "Generate Buckets secrets"
buckets
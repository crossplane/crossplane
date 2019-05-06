#!/usr/bin/env bash

SOURCE_CONTEXT=${1:-minikube}
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

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

# Process crossplane bucket connection secrets and create secrets in GitLab expected format
buckets () {
    # use claim file names as bucket name enumerator
    for f in ./cluster/examples/gitlab/gcp/resource-claims/buckets/*; do
        bucket=$(basename ${f} .yaml)

        # retrieve interoperability access key and secret
        interop_access_key=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-${bucket} -ojson | jq -r '.data.username' | base64 -d)
        interop_secret=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-${bucket} -ojson | jq -r '.data.password' | base64 -d)

        # retrieve service account key.json
        key_json=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-${bucket} -ojson | jq -r '.data.token' | base64 -d)

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
    done
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

    host=$(kubectl get secret gitlab-postgresql -o json | jq -r '.data.endpoint' | base64 -d)
    user=$(kubectl get secret gitlab-postgresql -o json | jq -r '.data.username' | base64 -d)
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
    host=$(kubectl --context=${SOURCE_CONTEXT} get secret gitlab-redis -o json | jq -r '.data.endpoint' | base64 -d)
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
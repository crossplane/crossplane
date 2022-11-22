---
title: Vault Credential Injection
weight: 230
---


> This guide is adapted from the [Vault on Minikube] and [Vault Kubernetes
> Sidecar] guides.

Most Crossplane providers support supplying credentials from at least the
following sources:
- Kubernetes Secret
- Environment Variable
- Filesystem

A provider may optionally support additional credentials sources, but the common
sources cover a wide variety of use cases. One specific use case that is popular
among organizations that use [Vault] for secrets management is using a sidecar
to inject credentials into the filesystem. This guide will demonstrate how to
use the [Vault Kubernetes Sidecar] to provide credentials for [provider-gcp] 
and [provider-aws].

> Note: in this guide we will copy GCP credentials and AWS access keys 
> into Vault's KV secrets engine. This is a simple generic approach to 
> managing secrets with Vault, but is not as robust as using Vault's 
> dedicated cloud provider secrets engines for [AWS], [Azure], and [GCP]. 

## Setup

> Note: this guide walks through setting up Vault running in the same cluster as
> Crossplane. You may also choose to use an existing Vault instance that runs
> outside the cluster but has Kubernetes authentication enabled.

Before getting started, you must ensure that you have installed Crossplane and
Vault and that they are running in your cluster.

1. Install Crossplane

```console
kubectl create namespace crossplane-system

helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update

helm install crossplane --namespace crossplane-system crossplane-stable/crossplane
```

2. Install Vault Helm Chart

```console
helm repo add hashicorp https://helm.releases.hashicorp.com
helm install vault hashicorp/vault
```

3. Unseal Vault Instance

In order for Vault to access encrypted data from physical storage, it must be
[unsealed].

```console
kubectl exec vault-0 -- vault operator init -key-shares=1 -key-threshold=1 -format=json > cluster-keys.json
VAULT_UNSEAL_KEY=$(cat cluster-keys.json | jq -r ".unseal_keys_b64[]")
kubectl exec vault-0 -- vault operator unseal $VAULT_UNSEAL_KEY
```

4. Enable Kubernetes Authentication Method

In order for Vault to be able to authenticate requests based on Kubernetes
service accounts, the [Kubernetes authentication backend] must be enabled. This
requires logging in to Vault and configuring it with a service account token,
API server address, and certificate. Because we are running Vault in Kubernetes,
these values are already available via the container filesystem and environment
variables.

```console
cat cluster-keys.json | jq -r ".root_token" # get root token

kubectl exec -it vault-0 -- /bin/sh
vault login # use root token from above
vault auth enable kubernetes

vault write auth/kubernetes/config \
        token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
        kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443" \
        kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
```

5. Exit Vault Container

The next steps will be executed in your local environment.

```console
exit
```

{{< tabs >}}
{{< tab "GCP" >}}

## Create GCP Service Account

In order to provision infrastructure on GCP, you will need to create a service
account with appropriate permissions. In this guide we will only provision a
CloudSQL instance, so the service account will be bound to the `cloudsql.admin`
role. The following steps will setup a GCP service account, give it the
necessary permissions for Crossplane to be able to manage CloudSQL instances,
and emit the service account credentials in a JSON file.

```console
# replace this with your own gcp project id and the name of the service account
# that will be created.
PROJECT_ID=my-project
NEW_SA_NAME=test-service-account-name

# create service account
SA="${NEW_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
gcloud iam service-accounts create $NEW_SA_NAME --project $PROJECT_ID

# enable cloud API
SERVICE="sqladmin.googleapis.com"
gcloud services enable $SERVICE --project $PROJECT_ID

# grant access to cloud API
ROLE="roles/cloudsql.admin"
gcloud projects add-iam-policy-binding --role="$ROLE" $PROJECT_ID --member "serviceAccount:$SA"

# create service account keyfile
gcloud iam service-accounts keys create creds.json --project $PROJECT_ID --iam-account $SA
```

You should now have valid service account credentials in `creds.json`.

## Store Credentials in Vault

After setting up Vault, you will need to store your credentials in the [kv
secrets engine].

> Note: the steps below involve copying credentials into the container
> filesystem before storing them in Vault. You may also choose to use Vault's
> HTTP API or UI by port-forwarding the container to your local environment
> (`kubectl port-forward vault-0 8200:8200`).

1. Copy Credentials File into Vault Container

Copy your credentials into the container filesystem so that your can store them
in Vault.

```console
kubectl cp creds.json vault-0:/tmp/creds.json
```

2. Enable KV Secrets Engine

Secrets engines must be enabled before they can be used. Enable the `kv-v2`
secrets engine at the `secret` path.

```console
kubectl exec -it vault-0 -- /bin/sh

vault secrets enable -path=secret kv-v2
```

3. Store GCP Credentials in KV Engine

The path of your GCP credentials is how the secret will be referenced when
injecting it into the `provider-gcp` controller `Pod`.

```console
vault kv put secret/provider-creds/gcp-default @tmp/creds.json
```

4. Clean Up Credentials File

You no longer need our GCP credentials file in the container filesystem, so go
ahead and clean it up.

```console
rm tmp/creds.json
```

{{< /tab >}}
{{< tab "AWS" >}}

## Create AWS IAM User

In order to provision infrastructure on AWS, you will need to use an existing or create a new IAM
user with appropriate permissions. The following steps will create an AWS IAM user and give it the necessary
permissions.

> Note: if you have an existing IAM user with appropriate permissions, you can skip this step but you will 
> still need to provide the values for the `ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables.

```console
# create a new IAM user
IAM_USER=test-user
aws iam create-user --user-name $IAM_USER

# grant the IAM user the necessary permissions
aws iam attach-user-policy --user-name $IAM_USER --policy-arn arn:aws:iam::aws:policy/AmazonS3FullAccess

# create a new IAM access key for the user
aws iam create-access-key --user-name $IAM_USER > creds.json
# assign the access key values to environment variables
ACCESS_KEY_ID=$(jq -r .AccessKey.AccessKeyId creds.json)
AWS_SECRET_ACCESS_KEY=$(jq -r .AccessKey.SecretAccessKey creds.json)
```

## Store Credentials in Vault

After setting up Vault, you will need to store your credentials in the [kv
secrets engine].

1. Enable KV Secrets Engine

Secrets engines must be enabled before they can be used. Enable the `kv-v2`
secrets engine at the `secret` path.

```console
kubectl exec -it vault-0 -- env \
  ACCESS_KEY_ID=${ACCESS_KEY_ID} \
  AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
  /bin/sh

vault secrets enable -path=secret kv-v2
```

2. Store AWS Credentials in KV Engine

The path of your AWS credentials is how the secret will be referenced when
injecting it into the `provider-aws` controller `Pod`.

```
vault kv put secret/provider-creds/aws-default access_key="$ACCESS_KEY_ID" secret_key="$AWS_SECRET_ACCESS_KEY"
```

{{< /tab >}}
{{< /tabs >}}

## Create a Vault Policy for Reading Provider Credentials

In order for our controllers to have the Vault sidecar inject the credentials
into their filesystem, you must associate the `Pod` with a [policy]. This policy
will allow for reading and listing all secrets on the `provider-creds` path in
the `kv-v2` secrets engine.

```console
vault policy write provider-creds - <<EOF
path "secret/data/provider-creds/*" {
    capabilities = ["read", "list"]
}
EOF
```

## Create a Role for Crossplane Provider Pods

1. Create Role

The last step is to create a role that is bound to the policy you created and
associate it with a group of Kubernetes service accounts. This role can be
assumed by any (`*`) service account in the `crossplane-system` namespace.

```console
vault write auth/kubernetes/role/crossplane-providers \
        bound_service_account_names="*" \
        bound_service_account_namespaces=crossplane-system \
        policies=provider-creds \
        ttl=24h
```

2. Exit Vault Container

The next steps will be executed in your local environment.

```console
exit
```

{{< tabs >}}
{{< tab "GCP" >}}

## Install provider-gcp

You are now ready to install `provider-gcp`. Crossplane provides a
`ControllerConfig` type that allows you to customize the deployment of a
provider's controller `Pod`. A `ControllerConfig` can be created and referenced
by any number of `Provider` objects that wish to use its configuration. In the
example below, the `Pod` annotations indicate to the Vault mutating webhook that
we want for the secret stored at `secret/provider-creds/gcp-default` to be
injected into the container filesystem by assuming role `crossplane-providers`.
There is also so template formatting added to make sure the secret data is
presented in a form that `provider-gcp` is expecting.

{% raw  %}
```console
echo "apiVersion: pkg.crossplane.io/v1alpha1
kind: ControllerConfig
metadata:
  name: vault-config
spec:
  metadata:
    annotations:
      vault.hashicorp.com/agent-inject: \"true\"
      vault.hashicorp.com/role: "crossplane-providers"
      vault.hashicorp.com/agent-inject-secret-creds.txt: "secret/provider-creds/gcp-default"
      vault.hashicorp.com/agent-inject-template-creds.txt: |
        {{- with secret \"secret/provider-creds/gcp-default\" -}}
         {{ .Data.data | toJSON }}
        {{- end -}}
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-gcp
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0
  controllerConfigRef:
    name: vault-config" | kubectl apply -f -
```
{% endraw %}

## Configure provider-gcp

One `provider-gcp` is installed and running, you will want to create a
`ProviderConfig` that specifies the credentials in the filesystem that should be
used to provision managed resources that reference this `ProviderConfig`.
Because the name of this `ProviderConfig` is `default` it will be used by any
managed resources that do not explicitly reference a `ProviderConfig`.

> Note: make sure that the `PROJECT_ID` environment variable that was defined
> earlier is still set correctly.

```console
echo "apiVersion: gcp.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  projectID: ${PROJECT_ID}
  credentials:
    source: Filesystem
    fs:
      path: /vault/secrets/creds.txt" | kubectl apply -f -
```

To verify that the GCP credentials are being injected into the container run the 
following command:

```console
PROVIDER_CONTROLLER_POD=$(kubectl -n crossplane-system get pod -l pkg.crossplane.io/provider=provider-gcp -o name --no-headers=true)
kubectl -n crossplane-system exec -it $PROVIDER_CONTROLLER_POD -c provider-gcp -- cat /vault/secrets/creds.txt
```

## Provision Infrastructure

The final step is to actually provision a `CloudSQLInstance`. Creating the
object below will result in the creation of a Cloud SQL Postgres database on
GCP.

```console
echo "apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstance
metadata:
  name: postgres-vault-demo
spec:
  forProvider:
    databaseVersion: POSTGRES_12
    region: us-central1
    settings:
      tier: db-custom-1-3840
      dataDiskType: PD_SSD
      dataDiskSizeGb: 10
  writeConnectionSecretToRef:
    namespace: crossplane-system
    name: cloudsqlpostgresql-conn" | kubectl apply -f -
```

You can monitor the progress of the database provisioning with the following
command:

```console
kubectl get cloudsqlinstance -w
```

{{< /tab >}}
{{< tab "AWS" >}}

## Install provider-aws

You are now ready to install `provider-aws`. Crossplane provides a
`ControllerConfig` type that allows you to customize the deployment of a
provider's controller `Pod`. A `ControllerConfig` can be created and referenced
by any number of `Provider` objects that wish to use its configuration. In the
example below, the `Pod` annotations indicate to the Vault mutating webhook that
we want for the secret stored at `secret/provider-creds/aws-default` to be
injected into the container filesystem by assuming role `crossplane-providers`.
There is also some template formatting added to make sure the secret data is
presented in a form that `provider-aws` is expecting.

{% raw  %}
```console
echo "apiVersion: pkg.crossplane.io/v1alpha1
kind: ControllerConfig
metadata:
  name: aws-vault-config
spec:
  args:
    - --debug
  metadata:
    annotations:
      vault.hashicorp.com/agent-inject: \"true\"
      vault.hashicorp.com/role: \"crossplane-providers\"
      vault.hashicorp.com/agent-inject-secret-creds.txt: \"secret/provider-creds/aws-default\"
      vault.hashicorp.com/agent-inject-template-creds.txt: |
        {{- with secret \"secret/provider-creds/aws-default\" -}}
          [default]
          aws_access_key_id="{{ .Data.data.access_key }}"
          aws_secret_access_key="{{ .Data.data.secret_key }}"
        {{- end -}}
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: xpkg.upbound.io/crossplane-contrib/provider-aws:v0.33.0
  controllerConfigRef:
    name: aws-vault-config" | kubectl apply -f -
```
{% endraw %}

## Configure provider-aws

Once `provider-aws` is installed and running, you will want to create a
`ProviderConfig` that specifies the credentials in the filesystem that should be
used to provision managed resources that reference this `ProviderConfig`.
Because the name of this `ProviderConfig` is `default` it will be used by any
managed resources that do not explicitly reference a `ProviderConfig`.

```console
echo "apiVersion: aws.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Filesystem
    fs:
      path: /vault/secrets/creds.txt" | kubectl apply -f -
```

To verify that the AWS credentials are being injected into the container run the 
following command:

```console
PROVIDER_CONTROLLER_POD=$(kubectl -n crossplane-system get pod -l pkg.crossplane.io/provider=provider-aws -o name --no-headers=true)
kubectl -n crossplane-system exec -it $PROVIDER_CONTROLLER_POD -c provider-aws -- cat /vault/secrets/creds.txt
```

## Provision Infrastructure

The final step is to actually provision a `Bucket`. Creating the
object below will result in the creation of a S3 bucket on AWS.

```console
echo "apiVersion: s3.aws.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: s3-vault-demo
spec:
  forProvider:
    acl: private
    locationConstraint: us-east-1
    publicAccessBlockConfiguration:
      blockPublicPolicy: true
    tagging:
      tagSet:
        - key: Name
          value: s3-vault-demo
  providerConfigRef:
    name: default" | kubectl apply -f -
```

You can monitor the progress of the bucket provisioning with the following
command:

```console
kubectl get bucket -w
```

{{< /tab >}}
{{< /tabs >}}

<!-- named links -->

[Vault on Minikube]: https://learn.hashicorp.com/tutorials/vault/kubernetes-minikube
[Vault Kubernetes Sidecar]: https://learn.hashicorp.com/tutorials/vault/kubernetes-sidecar
[Vault]: https://www.vaultproject.io/
[Vault Kubernetes Sidecar]: https://www.vaultproject.io/docs/platform/k8s/injector
[provider-gcp]: https://marketplace.upbound.io/providers/crossplane-contrib/provider-gcp
[provider-aws]: https://marketplace.upbound.io/providers/crossplane-contrib/provider-aws
[AWS]: https://www.vaultproject.io/docs/secrets/aws
[Azure]: https://www.vaultproject.io/docs/secrets/azure
[GCP]: https://www.vaultproject.io/docs/secrets/gcp 
[unsealed]: https://www.vaultproject.io/docs/concepts/seal
[Kubernetes authentication backend]: https://www.vaultproject.io/docs/auth/kubernetes
[kv secrets engine]: https://www.vaultproject.io/docs/secrets/kv/kv-v2
[policy]: https://www.vaultproject.io/docs/concepts/policies

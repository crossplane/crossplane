---
title: Vault as an External Secret Store 
toc: true
weight: 230
indent: true
---

# Using Vault as an External Secret Store

This guide walks through the steps required to configure Crossplane and
its Providers to use [Vault] as an [External Secret Store]. For the sake of
completeness, we will also include steps for Vault installation and setup,
however, you can skip those and use your existing Vault.

> External Secret Stores are an alpha feature. They are not yet recommended for
> production use, and are disabled by default.

Crossplane consumes and also produces sensitive information to operate which
could be categorized as follows:

1. **Provider credentials:** These are the credentials required for Providers
to authenticate against external APIs. For example, AWS Access/Secret keys, GCP
service account json, etc.
2. **Connection Details:** Once an infrastructure provisioned, we usually
need some connection data to consume it. Most of the time, this
information includes sensitive information like usernames, passwords or access
keys. 
3. **Sensitive Inputs to Managed Resources:** There are some Managed resources
which expect input parameters that could be sensitive. Initial password of a
managed database is a good example of this category.

It is already possible to use Vault for the 1st category (i.e. Provider
Credentials) as described in [the previous guide]. 3rd use case is a relatively
rare and being tracked with [this issue].

In this guide we will focus on the 2nd category, which is storing Connection
Details for managed resources in Vault.

## Steps

> Some steps in this guide duplicates [the previous guide] on Vault injection.
> However, for convenience, we put them here as well with minor
> changes/improvements.

At a high level we will run the following steps:

- Install and Unseal Vault.
- Configure Vault with Kubernetes Auth.
- Install and Configure Crossplane by enabling the feature.
- Install and Configure Provider GCP by enabling the feature.
- Deploy a Composition and CompositeResourceDefinition.
- Create a Claim.
- Verify all secrets land in Vault as expected.

For simplicity, we will deploy Vault into the same cluster as Crossplane,
however, this is not a requirement as long as Vault has Kubernetes auth enabled
for the cluster where Crossplane is running.

### Prepare Vault

1. Install Vault Helm Chart

```shell
kubectl create ns vault-system

helm repo add hashicorp https://helm.releases.hashicorp.com --force-update
helm -n vault-system upgrade --install vault hashicorp/vault
```

2. [Unseal] Vault

```shell
kubectl -n vault-system exec vault-0 -- vault operator init -key-shares=1 -key-threshold=1 -format=json > cluster-keys.json
VAULT_UNSEAL_KEY=$(cat cluster-keys.json | jq -r ".unseal_keys_b64[]")
kubectl -n vault-system exec vault-0 -- vault operator unseal $VAULT_UNSEAL_KEY
```

3. Configure Vault with Kubernetes Auth.

In order for Vault to be able to authenticate requests based on Kubernetes 
service accounts, the [Kubernetes auth method] must be enabled.
This requires logging in to Vault and configuring it with a service account
token, API server address, and certificate. Because we are running Vault in
Kubernetes, these values are already available via the container filesystem and
environment variables.

Get Vault Root Token:

```shell
cat cluster-keys.json | jq -r ".root_token"
```

Login as root and enable/configure Kubernetes Auth:

```shell
kubectl -n vault-system exec -it vault-0 -- /bin/sh

vault login # use root token from above

vault auth enable kubernetes
vault write auth/kubernetes/config \
        token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
        kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443" \
        kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        
exit # exit vault container
```

4. Enable Vault Key Value Secret Engine

There are two different versions of [Vault KV Secrets Engine], `v1` and `v2`,
which you can find more details in the linked documentation page. 
We will use `v2` in this guide as an example, however, both versions are
supported as an external secret store.

```shell
kubectl -n vault-system exec -it vault-0 -- vault secrets enable -path=secret kv-v2
```

5. Create a Vault Policy and Role for Crossplane

```shell
kubectl -n vault-system exec -i vault-0 -- vault policy write crossplane - <<EOF
path "secret/data/*" {
    capabilities = ["create", "read", "update", "delete"]
}
path "secret/metadata/*" {
    capabilities = ["create", "read", "update", "delete"]
}
EOF

kubectl -n vault-system exec -it vault-0 -- vault write auth/kubernetes/role/crossplane \
    bound_service_account_names="*" \
    bound_service_account_namespaces=crossplane-system \
    policies=crossplane \
    ttl=24h
```

### Install and Configure Crossplane

1. Install Crossplane by:

- Enabling `External Secret Stores` feature. 
- Annotating for [Vault Agent Sidecar Injection]

```shell
kubectl create ns crossplane-system

helm repo add crossplane-stable https://charts.crossplane.io/stable --force-update

helm upgrade --install crossplane crossplane-stable/crossplane --namespace crossplane-system \
  --set 'args={--enable-external-secret-stores}' \
  --set-string customAnnotations."vault\.hashicorp\.com/agent-inject"=true \
  --set-string customAnnotations."vault\.hashicorp\.com/agent-inject-token"=true \
  --set-string customAnnotations."vault\.hashicorp\.com/role"=crossplane \
  --set-string customAnnotations."vault\.hashicorp\.com/agent-run-as-user"=65532
```

2. Create a Secret `StoreConfig` for Crossplane to be used by
Composition types, i.e. `Composites` and `Claims`:

```shell
echo "apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault
spec:
  type: Vault
  defaultScope: crossplane-system
  vault:
    server: http://vault.vault-system:8200
    mountPath: secret/
    version: v2
    auth:
      method: Token
      token:
        source: Filesystem
        fs:
          path: /vault/secrets/token" | kubectl apply -f -
```

### Install and Configure Provider GCP

1. Similar to Crossplane, install Provider GCP by:

- Enabling `External Secret Stores` feature.
- Annotating for [Vault Agent Sidecar Injection]

```shell
echo "apiVersion: pkg.crossplane.io/v1alpha1
kind: ControllerConfig
metadata:
  name: vault-config
spec:
  args:
    - --enable-external-secret-stores
  metadata:
    annotations:
      vault.hashicorp.com/agent-inject: \"true\"
      vault.hashicorp.com/agent-inject-token: \"true\"
      vault.hashicorp.com/role: crossplane
      vault.hashicorp.com/agent-run-as-user: \"2000\"
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-gcp
spec:
  package: crossplane/provider-gcp:v0.21.0
  controllerConfigRef:
    name: vault-config" | kubectl apply -f -
```

2. Create a Secret `StoreConfig` for Provider GCP to be used by GCP Managed
Resources:

```shell
echo "apiVersion: gcp.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault
spec:
  type: Vault
  defaultScope: crossplane-system
  vault:
    server: http://vault.vault-system:8200
    mountPath: secret/
    version: v2
    auth:
      method: Token
      token:
        source: Filesystem
        fs:
          path: /vault/secrets/token" | kubectl apply -f -
```

### Deploy and Test

> Prerequisite: You should have a working **default** `ProviderConfig` for
> GCP available.

1. Create a `Composition` and a `CompositeResourceDefinition`:

```shell
echo "apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositeessinstances.ess.example.org
  annotations:
    feature: ess
spec:
  group: ess.example.org
  names:
    kind: CompositeESSInstance
    plural: compositeessinstances
  claimNames:
    kind: ESSInstance
    plural: essinstances
  connectionSecretKeys:
    - publicKey
    - publicKeyType
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              parameters:
                type: object
                properties:
                  serviceAccount:
                    type: string
                required:
                  - serviceAccount
            required:
              - parameters" | kubectl apply -f -
              
echo "apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: essinstances.ess.example.org
  labels:
    feature: ess
spec:
  publishConnectionDetailsWithStoreConfigRef: 
    name: vault
  compositeTypeRef:
    apiVersion: ess.example.org/v1alpha1
    kind: CompositeESSInstance
  resources:
    - name: serviceaccount
      base:
        apiVersion: iam.gcp.crossplane.io/v1alpha1
        kind: ServiceAccount
        metadata:
          name: ess-test-sa
        spec:
          forProvider:
            displayName: a service account to test ess
    - name: serviceaccountkey
      base:
        apiVersion: iam.gcp.crossplane.io/v1alpha1
        kind: ServiceAccountKey
        spec:
          forProvider:
            serviceAccountSelector:
              matchControllerRef: true
          publishConnectionDetailsTo:
            name: ess-mr-conn
            metadata:
              labels:
                environment: development
                team: backend
            configRef:
              name: vault
      connectionDetails:
        - fromConnectionSecretKey: publicKey
        - fromConnectionSecretKey: publicKeyType" | kubectl apply -f -
```

2. Create a `Claim`:

```shell
echo "apiVersion: ess.example.org/v1alpha1
kind: ESSInstance
metadata:
  name: my-ess
  namespace: default
spec:
  parameters:
    serviceAccount: ess-test-sa
  compositionSelector:
    matchLabels:
      feature: ess
  publishConnectionDetailsTo:
    name: ess-claim-conn
    metadata:
      labels:
        environment: development
        team: backend
    configRef:
      name: vault" | kubectl apply -f -
```

3. Verify all resources SYNCED and READY:

```shell
kubectl get managed
# Example output:
# NAME                                                      READY   SYNCED   DISPLAYNAME                     EMAIL                                                            DISABLED
# serviceaccount.iam.gcp.crossplane.io/my-ess-zvmkz-vhklg   True    True     a service account to test ess   my-ess-zvmkz-vhklg@testingforbugbounty.iam.gserviceaccount.com

# NAME                                                         READY   SYNCED   KEY_ID                                     CREATED_AT             EXPIRES_AT
# serviceaccountkey.iam.gcp.crossplane.io/my-ess-zvmkz-bq8pz   True    True     5cda49b7c32393254b5abb121b4adc07e140502c   2022-03-23T10:54:50Z

kubectl -n default get claim
# Example output:
# NAME     READY   CONNECTION-SECRET   AGE
# my-ess   True                        19s

kubectl get composite
# Example output:
# NAME           READY   COMPOSITION                    AGE
# my-ess-zvmkz   True    essinstances.ess.example.org   32s
```

### Verify the Connection Secrets landed to Vault

```shell
# Check connection secrets in the "default" scope (namespace). 
kubectl -n vault-system exec -i vault-0 -- vault kv list /secret/default
# Example output:
# Keys
# ----
# ess-claim-conn

# Check connection secrets in the "crossplane-system" scope (namespace).
kubectl -n vault-system exec -i vault-0 -- vault kv list /secret/crossplane-system
# Example output:
# Keys
# ----
# d2408335-eb88-4146-927b-8025f405da86
# ess-mr-conn

# Check contents of claim connection secret
kubectl -n vault-system exec -i vault-0 -- vault kv get /secret/default/ess-claim-conn
# Example output:
# ======= Metadata =======
# Key                Value
# ---                -----
# created_time       2022-03-18T21:24:07.2085726Z
# custom_metadata    map[environment:development secret.crossplane.io/owner-uid:881cd9a0-6cc6-418f-8e1d-b36062c1e108 team:backend]
# deletion_time      n/a
# destroyed          false
# version            1
# 
# ======== Data ========
# Key              Value
# ---              -----
# publicKey        -----BEGIN PUBLIC KEY-----
# MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzsEYCokmYEsZJCc9QN/8
# Fm1M/kTPp7Gat/MXLTP3zFyCTBFVNLN79MbAKdinWi6ePXEb75vzB79IdZcWj8lo
# 8trnS64QjNB9Vs4Xk5UvDALwleFN/bZeperxivDPwVPvT9Aqy/U9kohoS/LHyE8w
# uWQb5AuMeVQ1gtCTnCqQZ4d2MSVhQXYVvAWax1spJ9LT7mHub5j95xDdYIcOV3VJ
# l9CIo4VrWIT8THFN2NnjTrGq9+0TzXY0bV674bjJkfBC6v6yXs5HTetG+Uekq/xf
# FCjrrDi1+2UR9Mu2WTuvl8qn50be+mbwdJO5wE32jewxdYrVVmj19+PkaEeAwGTc
# vwIDAQAB
# -----END PUBLIC KEY-----
# publicKeyType    TYPE_RAW_PUBLIC_KEY

# Check contents of managed resource connection secret
kubectl -n vault-system exec -i vault-0 -- vault kv get /secret/crossplane-system/ess-mr-conn
# Example output:
# ======= Metadata =======
# Key                Value
# ---                -----
# created_time       2022-03-18T21:21:07.9298076Z
# custom_metadata    map[environment:development secret.crossplane.io/owner-uid:4cd973f8-76fc-45d6-ad45-0b27b5e9252a team:backend]
# deletion_time      n/a
# destroyed          false
# version            2
# 
# ========= Data =========
# Key               Value
# ---               -----
# privateKey        {
#   "type": "service_account",
#   "project_id": "REDACTED",
#   "private_key_id": "REDACTED",
#   "private_key": "-----BEGIN PRIVATE KEY-----\nREDACTED\n-----END PRIVATE KEY-----\n",
#   "client_email": "ess-test-sa@REDACTED.iam.gserviceaccount.com",
#   "client_id": "REDACTED",
#   "auth_uri": "https://accounts.google.com/o/oauth2/auth",
#   "token_uri": "https://oauth2.googleapis.com/token",
#   "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
#   "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/ess-test-sa%40REDACTED.iam.gserviceaccount.com"
# }
# privateKeyType    TYPE_GOOGLE_CREDENTIALS_FILE
# publicKey         -----BEGIN PUBLIC KEY-----
# MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzsEYCokmYEsZJCc9QN/8
# Fm1M/kTPp7Gat/MXLTP3zFyCTBFVNLN79MbAKdinWi6ePXEb75vzB79IdZcWj8lo
# 8trnS64QjNB9Vs4Xk5UvDALwleFN/bZeperxivDPwVPvT9Aqy/U9kohoS/LHyE8w
# uWQb5AuMeVQ1gtCTnCqQZ4d2MSVhQXYVvAWax1spJ9LT7mHub5j95xDdYIcOV3VJ
# l9CIo4VrWIT8THFN2NnjTrGq9+0TzXY0bV674bjJkfBC6v6yXs5HTetG+Uekq/xf
# FCjrrDi1+2UR9Mu2WTuvl8qn50be+mbwdJO5wE32jewxdYrVVmj19+PkaEeAwGTc
# vwIDAQAB
# -----END PUBLIC KEY-----
# publicKeyType     TYPE_RAW_PUBLIC_KEY
```

The commands above verifies using the cli, however, you can also connect to the
Vault UI and check secrets there.

```shell
kubectl -n vault-system port-forward vault-0 8200:8200
```

Now, you can open http://127.0.0.1:8200/ui in browser and login with the root token.

### Cleanup

Delete the claim which should clean up all the resources created.

```
kubectl -n default delete claim my-ess
```

<!-- named links -->

[Vault]: https://www.vaultproject.io/
[External Secret Store]: https://github.com/crossplane/crossplane/blob/master/design/design-doc-external-secret-stores.md
[the previous guide]: vault-injection.md
[this issue]: https://github.com/crossplane/crossplane/issues/2985
[Kubernetes Auth Method]: https://www.vaultproject.io/docs/auth/kubernetes
[Unseal]: https://www.vaultproject.io/docs/concepts/seal
[Vault KV Secrets Engine]: https://www.vaultproject.io/docs/secrets/kv
[Vault Agent Sidecar Injection]: https://www.vaultproject.io/docs/platform/k8s/injector

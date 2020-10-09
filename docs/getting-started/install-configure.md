---
title: Install & Configure
toc: true
weight: 2
indent: true
---

# Install & Configure Crossplane

Crossplane can be easily installed into any existing Kubernetes cluster using
the regularly published Helm chart. The Helm chart contains all the custom
resources and controllers needed to deploy and configure Crossplane.

See [Install] and [Configure] docs for installing alternate versions and more
detailed instructions.

## Get a Kubernetes Cluster

<ul class="nav nav-tabs">
<li class="active"><a href="#setup-mac-brew" data-toggle="tab">macOS via Homebrew</a></li>
<li><a href="#setup-mac-linux" data-toggle="tab">macOS / Linux</a></li>
<li><a href="#setup-windows" data-toggle="tab">Windows</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="setup-mac-brew" markdown="1">
For macOS via Homebrew use the following:

```console
brew upgrade
brew install kind
brew install kubectl
brew install helm

kind create cluster --image kindest/node:v1.16.15 --wait 5m
```

</div>
<div class="tab-pane fade" id="setup-mac-linux" markdown="1">
For macOS / Linux use the following:

* [Kubernetes cluster]
  * [Kind]
  * [Minikube], minimum version `v0.28+`
  * etc.

* [Helm], minimum version `v3.0.0+`.

</div>
<div class="tab-pane fade" id="setup-windows" markdown="1">
For Windows use the following:

* [Kubernetes cluster]
  * [Kind]
  * [Minikube], minimum version `v0.28+`
  * etc.

* [Helm], minimum version `v3.0.0+`.

</div>
</div>

## Install Crossplane

<ul class="nav nav-tabs">
<li class="active"><a href="#install-tab-helm3" data-toggle="tab">Helm 3 (alpha)</a></li>
<li><a href="#install-tab-helm3-master" data-toggle="tab">Helm 3 (master)</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="install-tab-helm3" markdown="1">
Use Helm 3 to install the latest official `alpha` release of Crossplane, suitable for community use and testing:

> OAM is available only for 1.16 and later versions of Kubernetes.

```console
kubectl create namespace crossplane-system

helm repo add crossplane-alpha https://charts.crossplane.io/alpha

# Kubernetes 1.16 and later versions
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane --set alpha.oam.enabled=true
```

```console
# Kubernetes 1.15 and earlier versions
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

</div>
<div class="tab-pane fade" id="install-tab-helm3-master" markdown="1">
Use Helm 3 to install the latest `master` pre-release version of Crossplane:

```console
kubectl create namespace crossplane-system

helm repo add crossplane-master https://charts.crossplane.io/master/
helm search repo crossplane-master --devel

# Kubernetes 1.16 and later versions
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --devel --version <version> --set alpha.oam.enabled=true
```

```console
# Kubernetes 1.15 and earlier versions
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --devel --version <version>
```

For example:

```console
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --version 0.11.0-rc.100.gbc5d311 --devel --set alpha.oam.enabled=true
```

</div>
</div>

## Check Crossplane Status

```console
helm list -n crossplane-system

kubectl get all -n crossplane-system
```

## Install Crossplane CLI

The Crossplane CLI extends `kubectl` with functionality to build, push, and install [Crossplane packages]:

```console
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/release-0.13/install.sh | sh
```

## Select Provider

Install and configure a provider for Crossplane to use for infrastructure provisioning:
<ul class="nav nav-tabs">
<li class="active"><a href="#provider-tab-aws" data-toggle="tab">AWS</a></li>
<li><a href="#provider-tab-gcp" data-toggle="tab">GCP</a></li>
<li><a href="#provider-tab-azure" data-toggle="tab">Azure</a></li>
<li><a href="#provider-tab-alibaba" data-toggle="tab">Alibaba</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="provider-tab-aws" markdown="1">

### Install AWS Provider

```console
kubectl crossplane install provider crossplane/provider-aws:v0.12.0
```

### Get AWS Account Keyfile

Using an AWS account with permissions to manage RDS databases:

```console
AWS_PROFILE=default && echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $AWS_PROFILE)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $AWS_PROFILE)" > creds.conf
```

### Create a Provider Secret

```console
kubectl create secret generic aws-creds -n crossplane-system --from-file=key=./creds.conf
```

### Configure the Provider

Create the following `provider.yaml`:

```yaml
apiVersion: aws.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: key
```

Then apply it:

```console
kubectl apply -f provider.yaml
```

</div>
<div class="tab-pane fade" id="provider-tab-gcp" markdown="1">

### Install GCP Provider

```console
kubectl crossplane install provider crossplane/provider-gcp:v0.12.0
```

### Get GCP Account Keyfile

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

### Create a Provider Secret

```console
kubectl create secret generic gcp-creds -n crossplane-system --from-file=key=./creds.json
```

### Configure the Provider

Create the following `provider.yaml`:

```yaml
apiVersion: gcp.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  # replace this with your own gcp project id
  projectID: my-project
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: gcp-creds
      key: key
```

Then apply it:

```console
kubectl apply -f provider.yaml
```

</div>
<div class="tab-pane fade" id="provider-tab-azure" markdown="1">

### Install Azure Provider

```console
kubectl crossplane install provider crossplane/provider-azure:v0.12.0
```

### Get Azure Principal Keyfile

```console
# create service principal with Owner role
az ad sp create-for-rbac --sdk-auth --role Owner > "creds.json"

# we need to get the clientId from the json file to add Azure Active Directory
# permissions.
if which jq > /dev/null 2>&1; then
  AZURE_CLIENT_ID=$(jq -r ".clientId" < "./creds.json")
else
  AZURE_CLIENT_ID=$(cat creds.json | grep clientId | cut -c 16-51)
fi

RW_ALL_APPS=1cda74f2-2616-4834-b122-5cb1b07f8a59
RW_DIR_DATA=78c8a3c8-a07e-4b9e-af1b-b5ccab50a175
AAD_GRAPH_API=00000002-0000-0000-c000-000000000000

az ad app permission add --id "${AZURE_CLIENT_ID}" --api ${AAD_GRAPH_API} --api-permissions ${RW_ALL_APPS}=Role ${RW_DIR_DATA}=Role
az ad app permission grant --id "${AZURE_CLIENT_ID}" --api ${AAD_GRAPH_API} --expires never > /dev/null
az ad app permission admin-consent --id "${AZURE_CLIENT_ID}"
```

### Create a Provider Secret

```console
kubectl create secret generic azure-creds -n crossplane-system --from-file=key=./creds.json
```

### Configure the Provider

Create the following `provider.yaml`:

```yaml
apiVersion: azure.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: azure-creds
      key: key
```

Then apply it:

```console
kubectl apply -f provider.yaml
```

</div>
<div class="tab-pane fade" id="provider-tab-alibaba" markdown="1">

### Install Alibaba Provider

```console
kubectl crossplane install provider crossplane/provider-alibaba:v0.3.0
```

### Create a Provider Secret

```console
kubectl create secret generic alibaba-creds --from-literal=accessKeyId=<your-key> --from-literal=accessKeySecret=<your-secret> -n crossplane-system
```

### Configure the Provider

Create the following `provider.yaml`:

```yaml
apiVersion: alibaba.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  region: cn-beijing
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: alibaba-creds
      key: credentials
```

Then apply it:

```console
kubectl apply -f provider.yaml
```

</div>
</div>

## Next Steps

Now that you have a provider configured, you can [provision infrastructure].

## More Info

See [Install] and [Configure] docs for installing alternate versions and more
detailed instructions.

## Uninstall Provider

Let's check whether there are any managed resources before deleting the provider.

```console
kubectl get managed
```

If there are any, please delete them first so you don't lose track of them.

```console
kubectl delete -f provider.yaml
```

## Uninstall Crossplane

```console
helm delete crossplane --namespace crossplane-system

kubectl delete namespace crossplane-system
```

<!-- Named Links -->

[provision infrastructure]: provision-infrastructure.md
[Install]: ../reference/install.md
[Configure]: ../reference/configure.md
[Kubernetes cluster]: https://kubernetes.io/docs/setup/
[Minikube]: https://kubernetes.io/docs/tasks/tools/install-minikube/
[Helm]: https://docs.helm.sh/using_helm/
[Kind]: https://kind.sigs.k8s.io/docs/user/quick-start/
[Crossplane packages]: ../introduction/packages.md

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

```
brew upgrade
brew install kind
brew install kubectl
brew install helm

kind create cluster --image kindest/node:v1.16.9 --wait 5m
```

</div>
<div class="tab-pane fade" id="setup-mac-linux" markdown="1">
For macOS / Linux use the following:

* [Kubernetes cluster]
  * [Kind]
  * [Minikube], minimum version `v0.28+`
  * etc.

* [Helm], minimum version `v2.12.0+`.
  * For Helm 2, make sure Tiller is initialized with sufficient permissions to
    work on `crossplane-system` namespace.

</div>
<div class="tab-pane fade" id="setup-windows" markdown="1">
For Windows use the following:

* [Kubernetes cluster]
  * [Kind]
  * [Minikube], minimum version `v0.28+`
  * etc.

* [Helm], minimum version `v2.12.0+`.
  * For Helm 2, make sure Tiller is initialized with sufficient permissions to
    work on `crossplane-system` namespace.

</div>
</div>

## Install Crossplane
<ul class="nav nav-tabs">
<li class="active"><a href="#install-tab-helm3" data-toggle="tab">Helm 3 (alpha)</a></li>
<li><a href="#install-tab-helm2" data-toggle="tab">Helm 2 (alpha)</a></li>
<li><a href="#install-tab-helm3-master" data-toggle="tab">Helm 3 (master)</a></li>
<li><a href="#install-tab-helm2-master" data-toggle="tab">Helm 2 (master)</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="install-tab-helm3" markdown="1">
Use Helm 3 to install the latest official `alpha` release of Crossplane, suitable for community use and testing:

```
kubectl create namespace crossplane-system

helm repo add crossplane-alpha https://charts.crossplane.io/alpha

# Kubernetes 1.15 and newer versions
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane

# Kubernetes 1.14 and older versions
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane --disable-openapi-validation
```

</div>
<div class="tab-pane fade" id="install-tab-helm2" markdown="1">
Use Helm 2 to install the latest official `alpha` release of Crossplane, suitable for community use and testing:

```
kubectl create namespace crossplane-system

helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

</div>
<div class="tab-pane fade" id="install-tab-helm3-master" markdown="1">
Use Helm 3 to install the latest `master` pre-release version of Crossplane:

```
kubectl create namespace crossplane-system

helm repo add crossplane-master https://charts.crossplane.io/master/
helm search repo crossplane-master --devel

# Kubernetes 1.15 and newer versions
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --version <version> --devel

# Kubernetes 1.14 and older versions
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane --version <version> --devel --disable-openapi-validation
```

For example:
```
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --version 0.11.0-rc.100.gbc5d311 --devel
```

</div>
<div class="tab-pane fade" id="install-tab-helm2-master" markdown="1">
Use Helm 2 to install the latest `master` pre-release version of Crossplane, which is suitable for testing pre-release versions:

```
kubectl create namespace crossplane-system

helm repo add crossplane-master https://charts.crossplane.io/master/
helm search crossplane-master

helm install --name crossplane --namespace crossplane-system crossplane-master/crossplane --version <version>
```

For example:

```
helm install --name crossplane --namespace crossplane-system crossplane-master/crossplane --version 0.11.0-rc.100.gbc5d311
```

</div>
</div>

## Install Crossplane CLI
The [Crossplane CLI] adds a set of `kubectl crossplane` commands to simplify common tasks:
```
curl -sL https://raw.githubusercontent.com/crossplane/crossplane-cli/master/bootstrap.sh | bash
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

```
PACKAGE=crossplane/provider-aws:master
NAME=provider-aws

kubectl crossplane package install --cluster --namespace crossplane-system ${PACKAGE} ${NAME}
```

### Get AWS Account Keyfile

Using an AWS account with permissions to manage RDS databases:
```
AWS_PROFILE=default && echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $AWS_PROFILE)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $AWS_PROFILE)" > creds.conf
```

### Create a Provider Secret

```
kubectl create secret generic aws-creds -n crossplane-system --from-file=key=./creds.conf
```

### Configure the Provider
Create the following `provider.yaml`:

```
apiVersion: aws.crossplane.io/v1alpha3
kind: Provider
metadata:
  name: aws-provider
spec:
  region: us-west-2
  credentialsSecretRef:
    namespace: crossplane-system
    name: aws-creds
    key: key
```

Then apply it:
```
kubectl apply -f provider.yaml
```

</div>
<div class="tab-pane fade" id="provider-tab-gcp" markdown="1">

### Install GCP Provider

```
PACKAGE=crossplane/provider-gcp:master
NAME=provider-gcp

kubectl crossplane package install --cluster --namespace crossplane-system ${PACKAGE} ${NAME}
```

### Get GCP Account Keyfile

```
# replace this with your own gcp project id and service account name
PROJECT_ID=my-project
SA_NAME=my-service-account-name

# create service account
SA="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" 
gcloud iam service-accounts create $SA_NAME --project $PROJECT_ID

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

```
kubectl create secret generic gcp-creds -n crossplane-system --from-file=key=./creds.json
```

### Configure the Provider
Create the following `provider.yaml`:

```
apiVersion: gcp.crossplane.io/v1alpha3
kind: Provider
metadata:
  name: gcp-provider
spec:
  # replace this with your own gcp project id
  projectID: my-project
  credentialsSecretRef:
    namespace: crossplane-system
    name: gcp-creds
    key: key
```

Then apply it:
```
kubectl apply -f provider.yaml
```

</div>
<div class="tab-pane fade" id="provider-tab-azure" markdown="1">

### Install Azure Provider

```
PACKAGE=crossplane/provider-azure:master
NAME=provider-azure

kubectl crossplane package install --cluster --namespace crossplane-system ${PACKAGE} ${NAME}
```

### Get Azure Principal Keyfile

```
# create service principal with Owner role
az ad sp create-for-rbac --sdk-auth --role Owner > "creds.json"

# add Azure Active Directory permissions
AZURE_CLIENT_ID=$(jq -r ".clientId" < "./creds.json")

RW_ALL_APPS=1cda74f2-2616-4834-b122-5cb1b07f8a59
RW_DIR_DATA=78c8a3c8-a07e-4b9e-af1b-b5ccab50a175
AAD_GRAPH_API=00000002-0000-0000-c000-000000000000

az ad app permission add --id "${AZURE_CLIENT_ID}" --api ${AAD_GRAPH_API} --api-permissions ${RW_ALL_APPS}=Role ${RW_DIR_DATA}=Role
az ad app permission grant --id "${AZURE_CLIENT_ID}" --api ${AAD_GRAPH_API} --expires never > /dev/null
az ad app permission admin-consent --id "${AZURE_CLIENT_ID}"
```

### Create a Provider Secret

```
kubectl create secret generic azure-creds -n crossplane-system --from-file=key=./creds.json
```

### Configure the Provider
Create the following `provider.yaml`:

```
apiVersion: azure.crossplane.io/v1alpha3
kind: Provider
metadata:
  name: azure-provider
spec:
  credentialsSecretRef:
    namespace: crossplane-system
    name: azure-creds
    key: key
```

Then apply it:
```
kubectl apply -f provider.yaml
```

</div>
<div class="tab-pane fade" id="provider-tab-alibaba" markdown="1">

### Install Alibaba Provider

```
PACKAGE=crossplane/provider-alibaba:master
NAME=provider-alibaba

kubectl crossplane package install --cluster --namespace crossplane-system ${PACKAGE} ${NAME}
```

### Create a Provider Secret

```
kubectl create secret generic alibaba-creds --from-literal=accessKeyId=<your-key> --from-literal=accessKeySecret=<your-secret> -n crossplane-system
```

### Configure the Provider
Create the following `provider.yaml`:

```
apiVersion: alibaba.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: alibaba-provider
spec:
  credentialsSecretRef:
    namespace: crossplane-system
    name: alibaba-creds
    key: credentials
  region: cn-beijing
```

Then apply it:
```
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
```
kubectl get managed
```

If there are any, please delete them first so you don't lose track of them.

```
kubectl delete -f provider.yaml
```

## Uninstall Crossplane
```
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
[Crossplane CLI]: https://github.com/crossplane/crossplane-cli

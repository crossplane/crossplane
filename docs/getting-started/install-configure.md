---
title: Install & Configure
toc: true
weight: 2
indent: true
---

# Choosing Your Crossplane Distribution

Users looking to use Crossplane for the first time have two options available to
them today. The first way is to use the version of Crossplane which is 
maintained and released by the community and found on the [Crossplane GitHub].

The second option is to use a vendor supported Crossplane distribution. These 
distributions are [certified by the CNCF] to be conformant with Crossplane, but 
may include additional features or tooling around it that makes it easier to use 
in production environments.

<ul class="nav nav-tabs"> 
<li class="active"><a href="#using-upstream-crossplane" data-toggle="tab">Crossplane (upstream)</a></li>
<li><a href="#using-a-downstream-distro" data-toggle="tab">Downstream Distributions</a></li>
</ul>
<br>
<!-- Begin Distro Tabs -->
<div class="tab-content"> 
<!-- Begin Upstream Tab -->
<div class="tab-pane fade in active" id="using-upstream-crossplane" markdown="1">

## Start with Upstream Crossplane

Installing Crossplane into an existing Kubernetes cluster will require a bit
more setup, but can provide more flexibility for users who need it.

### Get a Kubernetes Cluster

<ul class="nav nav-tabs">
<li class="active"><a href="#setup-mac-brew" data-toggle="tab">macOS via Homebrew</a></li>
<li><a href="#setup-mac-linux" data-toggle="tab">macOS / Linux</a></li>
<li><a href="#setup-windows" data-toggle="tab">Windows</a></li>
</ul>
<br>
<!-- Begin Get Cluster Tabs -->
<div class="tab-content">
<!-- Begin MacOS Tab -->
<div class="tab-pane fade in active" id="setup-mac-brew" markdown="1">
For macOS via Homebrew use the following:

```console
brew upgrade
brew install kind
brew install kubectl
brew install helm

kind create cluster --image kindest/node:v1.23.0 --wait 5m
```
</div>
<!-- End MacOS Tab -->

<!-- Begin Linux Tab -->
<div class="tab-pane fade" id="setup-mac-linux" markdown="1">
For macOS / Linux use the following:

* [Kubernetes cluster]
  * [Kind]
  * [Minikube], minimum version `v0.28+`
  * etc.

* [Helm], minimum version `v3.0.0+`.

</div>
<!-- End Linux Tab -->

<!-- Begin Windows Tab -->
<div class="tab-pane fade" id="setup-windows" markdown="1">
For Windows use the following:

* [Kubernetes cluster]
  * [Kind]
  * [Minikube], minimum version `v0.28+`
  * etc.

* [Helm], minimum version `v3.0.0+`.

</div>
<!-- End Windows Tab -->
</div>
<!-- End Get Cluster Tabs -->

### Install Crossplane

<ul class="nav nav-tabs">
<li class="active"><a href="#install-tab-helm3" data-toggle="tab">Helm 3 (stable)</a></li>
<li><a href="#install-tab-helm3-latest" data-toggle="tab">Helm 3 (latest)</a></li>
</ul>
<br>
<!-- Begin Helm Tabs -->
<div class="tab-content">

<!-- Begin Stable Tab -->
<div class="tab-pane fade in active" id="install-tab-helm3" markdown="1">
Use Helm 3 to install the latest official `stable` release of Crossplane, suitable for community use and testing:

```console
kubectl create namespace crossplane-system

helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update

helm install crossplane --namespace crossplane-system crossplane-stable/crossplane
```

</div>
<!-- End Stable Tab -->

<!-- Begin Latest Tab -->
<div class="tab-pane fade" id="install-tab-helm3-latest" markdown="1">
Use Helm 3 to install the latest pre-release version of Crossplane:

```console
kubectl create namespace crossplane-system

helm repo add crossplane-master https://charts.crossplane.io/master/
helm repo update
helm search repo crossplane-master --devel

helm install crossplane --namespace crossplane-system crossplane-master/crossplane \
  --devel --version <version>
```

For example:

```console
helm install crossplane --namespace crossplane-system crossplane-master/crossplane \
  --version 0.11.0-rc.100.gbc5d311 --devel
```

</div>
<!-- End Latest Tab -->

</div>
<!-- End Helm Tabs -->

### Check Crossplane Status

```console
helm list -n crossplane-system

kubectl get all -n crossplane-system
```

## Install Crossplane CLI

The Crossplane CLI extends `kubectl` with functionality to build, push, and
install [Crossplane packages]:

<ul class="nav nav-tabs">
<li class="active"><a href="#install-tab-cli" data-toggle="tab">Stable</a></li>
<li><a href="#install-tab-cli-latest" data-toggle="tab">Latest</a></li>
</ul>
<br>

<!-- Begin CLI Tabs -->
<div class="tab-content">

<!-- Begin CLI Stable Tab -->
<div class="tab-pane fade in active" id="install-tab-cli" markdown="1">

```console
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | sh
```

</div>
<!-- End CLI Stable Tab -->

<!-- Begin CLI Latest Tab -->
<div class="tab-pane fade" id="install-tab-cli-latest" markdown="1">

```console
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | CHANNEL=master sh
```

You may also specify `VERSION` for download if you would like to select a
specific version from the given release channel. If a version is not specified
the latest version from the release channel will be used.

```console
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | CHANNEL=master VERSION=v1.0.0-rc.0.130.g94f34fd3 sh
```

</div>
<!-- End CLI Latest Tab -->

</div>
<!-- End CLI tabs -->

## Select a Getting Started Configuration

Crossplane goes beyond simply modelling infrastructure primitives as custom
resources - it enables you to define new custom resources with schemas of your
choosing. We call these "composite resources" (XRs). Composite resources compose
managed resources -- Kubernetes custom resources that offer a high fidelity
representation of an infrastructure primitive, like an SQL instance or a
firewall rule.

We use two special Crossplane resources to define and configure these new custom
resources:

- A `CompositeResourceDefinition` (XRD) _defines_ a new kind of composite
  resource, including its schema. An XRD may optionally _offer_ a claim (XRC).
- A `Composition` specifies which resources a composite resource will be
  composed of, and how they should be configured. You can create multiple
  `Composition` options for each composite resource.

XRDs and Compositions may be packaged and installed as a _configuration_. A
configuration is a [package] of composition configuration that can easily be
installed to Crossplane by creating a declarative `Configuration` resource, or
by using `kubectl crossplane install configuration`.

In the examples below we will install a configuration that defines a new
`XPostgreSQLInstance` XR and `PostgreSQLInstance` XRC that takes a
single `storageGB` parameter, and creates a connection `Secret` with keys for
`username`, `password`, and `endpoint`. A `Configuration` exists for each
provider that can satisfy a `PostgreSQLInstance`. Let's get started!

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-1" data-toggle="tab">AWS (Default VPC)</a></li>
<li><a href="#aws-new-tab-1" data-toggle="tab">AWS (New VPC)</a></li>
<li><a href="#gcp-tab-1" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-1" data-toggle="tab">Azure</a></li>
</ul>
<br>

<!-- Begin Cloud Provider Tabs -->
<div class="tab-content">

<!-- Begin AWS Default VPC Tab -->
<div class="tab-pane fade in active" id="aws-tab-1" markdown="1">

### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-aws:latest
```

Wait until all packages become healthy:
```
watch kubectl get pkg
```

### Get AWS Account Keyfile

Using an AWS account with permissions to manage RDS databases:

```console
AWS_PROFILE=default && echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $AWS_PROFILE)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $AWS_PROFILE)" > creds.conf
```

### Create a Provider Secret

```console
kubectl create secret generic aws-creds -n crossplane-system --from-file=creds=./creds.conf
```

### Configure the Provider

We will create the following `ProviderConfig` object to configure credentials
for AWS Provider:

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
      key: creds
```
```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/configure/aws/providerconfig.yaml
```

</div>
<!-- End AWS Default VPC Tab -->

<!-- Begin AWS New VPC Tab -->
<div class="tab-pane fade" id="aws-new-tab-1" markdown="1">

### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-aws-with-vpc:latest
```

Wait until all packages become healthy:
```
watch kubectl get pkg
```

### Get AWS Account Keyfile

Using an AWS account with permissions to manage RDS databases:

```console
AWS_PROFILE=default && echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $AWS_PROFILE)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $AWS_PROFILE)" > creds.conf
```

### Create a Provider Secret

```console
kubectl create secret generic aws-creds -n crossplane-system --from-file=creds=./creds.conf
```

### Configure the Provider

We will create the following `ProviderConfig` object to configure credentials
for AWS Provider:

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
      key: creds
```
```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/configure/aws/providerconfig.yaml
```

</div>
<!-- End AWS New VPC Tab -->

<!-- Begin GCP Tab -->
<div class="tab-pane fade" id="gcp-tab-1" markdown="1">

### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-gcp:latest
```

Wait until all packages become healthy:
```
watch kubectl get pkg
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
kubectl create secret generic gcp-creds -n crossplane-system --from-file=creds=./creds.json
```

### Configure the Provider

We will create the following `ProviderConfig` object to configure credentials
for GCP Provider:

```console
# replace this with your own gcp project id
PROJECT_ID=my-project
echo "apiVersion: gcp.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  projectID: ${PROJECT_ID}
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: gcp-creds
      key: creds" | kubectl apply -f -
```

</div>
<!-- End GCP Tab -->

<!-- Begin Azure Tab -->
<div class="tab-pane fade" id="azure-tab-1" markdown="1">

### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-azure:latest
```

Wait until all packages become healthy:
```
watch kubectl get pkg
```

### Get Azure Principal Keyfile

```console
# create service principal with Owner role
az ad sp create-for-rbac --sdk-auth --role Owner > "creds.json"
```

### Create a Provider Secret

```console
kubectl create secret generic azure-creds -n crossplane-system --from-file=creds=./creds.json
```

### Configure the Provider

We will create the following `ProviderConfig` object to configure credentials
for Azure Provider:

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
      key: creds
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/configure/azure/providerconfig.yaml
```

</div>
<!-- End Azure Tab -->

</div>
<!-- End Cloud Provider Tabs -->

## Next Steps

Now that you have configured Crossplane with support for `PostgreSQLInstance`,
you can [provision infrastructure].

</div>
<!-- End Upstream Tab -->

<!-- Begin Downstream Tab -->
<div class="tab-pane fade" id="using-a-downstream-distro" markdown="1">

## Start with a Downstream Distribution

Upbound, the founders of Crossplane, maintains a free and open source downstream
distribution of Crossplane which makes getting started with Crossplane easy. 
Universal Crossplane, or UXP for short, connects to Upbound's hosted management 
console and Registry to make it easier to develop, debug, and manage Provider
and Configuration packages.

[Get started with Universal Crossplane] on the Upbound Documentation site.

<i>Want see another hosted Crossplane service listed? Please [reach out on
Slack][Slack] and our community will highlight it here!</i>

</div>
<!-- End Downstream Tab -->

</div>
<!-- End Distro Tabs -->

## More Info

* See [Install] and [Configure] docs for installing alternate versions and more
  detailed instructions.

* See [Uninstall] docs for cleaning up resources, packages, and Crossplane
  itself.

* See [Providers] for installing and using different providers beyond AWS, GCP
  and Azure mentionded in this guide.

<!-- Named Links -->

[package]: ../concepts/packages.md
[provision infrastructure]: provision-infrastructure.md
[create a configuration]: create-configuration.md
[Install]: ../reference/install.md
[Configure]: ../reference/configure.md
[Uninstall]: ../reference/uninstall.md
[Kubernetes cluster]: https://kubernetes.io/docs/setup/
[Minikube]: https://kubernetes.io/docs/tasks/tools/install-minikube/
[Helm]:https://docs.helm.sh/using_helm/
[Kind]: https://kind.sigs.k8s.io/docs/user/quick-start/
[Crossplane packages]: ../concepts/packages.md
[Slack]: http://slack.crossplane.io/
[up]: https://github.com/upbound/up
[Upbound documentation]: https://cloud.upbound.io/docs
[Providers]: ../concepts/providers.md
[Universal Crossplane]: https://cloud.upbound.io/docs/uxp
[Get started with Universal Crossplane]: https://cloud.upbound.io/docs/getting-started/install-and-setup
[certified by the CNCF]: https://github.com/cncf/crossplane-conformance
[Crossplane GitHub]: https://github.com/crossplane/crossplane

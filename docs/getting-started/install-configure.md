---
title: Install & Configure
weight: 2
---
## Choosing Your Crossplane Distribution

Users looking to use Crossplane for the first time have two options available to
them today. The first way is to use the version of Crossplane which is
maintained and released by the community and found on the [Crossplane GitHub].

The second option is to use a vendor supported Crossplane distribution. These
distributions are [certified by the CNCF] to be conformant with Crossplane, but
may include additional features or tooling around it that makes it easier to use
in production environments.

{{% tabs "Crossplane Distros" %}}

{{% tab "Crossplane (upstream)" %}}

## Start with Upstream Crossplane

Installing Crossplane into an existing Kubernetes cluster will require a bit
more setup, but can provide more flexibility for users who need it.

### Get a Kubernetes Cluster
<!-- inside Crossplane (upstream) -->
{{% tabs "Kubernetes Clusters" %}}

{{% tab "macOS via Homebrew" %}}

For macOS via Homebrew use the following:

```bash
brew upgrade
brew install kind
brew install kubectl
brew install helm
kind create cluster --image kindest/node:v1.23.0 --wait 5m
```
<!-- close "macOS via Homebrew" -->
{{% /tab  %}}

{{% tab "macOS / Linux" %}}

For macOS / Linux use the following:

* [Kubernetes cluster](https://kubernetes.io/docs/setup/)
* [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
* [Minikube](https://minikube.sigs.k8s.io/docs/start/), minimum version `v0.28+`
* etc.
* [Helm](https://helm.sh/docs/intro/using_helm/), minimum version `v3.0.0+`.

<!-- close "macOS / Linux" -->
{{% /tab %}}

{{% tab "Windows" %}}
For Windows use the following:

* [Kubernetes cluster](https://kubernetes.io/docs/setup/)
* [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
* [Minikube](https://minikube.sigs.k8s.io/docs/start/), minimum version `v0.28+`
* etc.
* [Helm](https://helm.sh/docs/intro/using_helm/), minimum version `v3.0.0+`.

<!-- close "Windows" -->
{{% /tab %}}

<!-- close "Kubernetes Clusters" -->
{{% /tabs %}}

### Install Crossplane

{{% tabs "install with helm" %}}

{{% tab "Helm 3 (stable)" %}}
Use Helm 3 to install the latest official `stable` release of Crossplane, suitable for community use and testing:

```bash
kubectl create namespace crossplane-system
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update

helm install crossplane --namespace crossplane-system crossplane-stable/crossplane
```

<!-- close "Helm 3 (stable)" -->
{{% /tab %}}

{{% tab "Helm 3 (latest)" %}}
<!-- fold start -->
Use Helm 3 to install the latest pre-release version of Crossplane:

```bash
kubectl create namespace crossplane-system

helm repo add crossplane-master https://charts.crossplane.io/master/
helm repo update
helm search repo crossplane-master --devel

helm install crossplane --namespace crossplane-system crossplane-master/crossplane \
  --devel --version <version>
```

For example:

```bash
helm install crossplane --namespace crossplane-system crossplane-master/crossplane \
  --version 0.11.0-rc.100.gbc5d311 --devel
```
<!-- close "Helm 3 (latest)" -->
{{% /tab %}}
<!-- close "install with helm" -->
{{% /tabs %}}

### Check Crossplane Status

```bash
helm list -n crossplane-system

kubectl get all -n crossplane-system
```

## Install Crossplane CLI

The Crossplane CLI extends `kubectl` with functionality to build, push, and
install [Crossplane packages]:

{{% tabs "crossplane CLI" %}}

{{% tab "Stable" %}}
```bash
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | sh
```
<!-- close "Stable" -->
{{% /tab %}}

{{% tab "Latest" %}}

```bash
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | CHANNEL=master sh
```

You may also specify `VERSION` for download if you would like to select a
specific version from the given release channel. If a version is not specified
the latest version from the release channel will be used.

```bash
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | CHANNEL=master VERSION=v1.0.0-rc.0.130.g94f34fd3 sh
```
<!-- close "Latest" -->
{{% /tab %}}

<!-- close "crossplane CLI" -->
{{% /tabs %}}

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

{{% tabs "getting started" %}}

{{% tab "AWS (Default VPC)" %}}
### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```bash
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-aws:latest
```

Wait until all packages become healthy:
```bash
watch kubectl get pkg
```

### Get AWS Account Keyfile

Using an AWS account with permissions to manage RDS databases:

```bash
AWS_PROFILE=default && echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $AWS_PROFILE)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $AWS_PROFILE)" > creds.conf
```

### Create a Provider Secret

```bash
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
```bash
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/configure/aws/providerconfig.yaml
```
<!-- close "AWS (Default VPC)" -->
{{% /tab %}}

{{% tab "AWS (New VPC)" %}}
### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```bash
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-aws-with-vpc:latest
```

Wait until all packages become healthy:
```bash
watch kubectl get pkg
```

### Get AWS Account Keyfile

Using an AWS account with permissions to manage RDS databases:

```bash
AWS_PROFILE=default && echo -e "[default]\naws_access_key_id = $(aws configure get aws_access_key_id --profile $AWS_PROFILE)\naws_secret_access_key = $(aws configure get aws_secret_access_key --profile $AWS_PROFILE)" > creds.conf
```

### Create a Provider Secret

```bash
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

```bash
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/configure/aws/providerconfig.yaml
```
<!-- close "AWS (New VPC)" -->
{{% /tab %}}

{{% tab "GCP" %}}

### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```bash
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-gcp:latest
```

Wait until all packages become healthy:
```
watch kubectl get pkg
```

### Get GCP Account Keyfile

```bash
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

```bash
kubectl create secret generic gcp-creds -n crossplane-system --from-file=creds=./creds.json
```

### Configure the Provider

We will create the following `ProviderConfig` object to configure credentials
for GCP Provider:

```bash
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
<!-- close "GCP" -->
{{% /tab %}}

{{% tab "Azure" %}}

### Install Configuration Package

> If you prefer to see the contents of this configuration package and how it is
> constructed prior to install, skip ahead to the [create a configuration]
> section.

```bash
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-azure:latest
```

Wait until all packages become healthy:
```
watch kubectl get pkg
```

### Get Azure Principal Keyfile

```bash
# create service principal with Owner role
az ad sp create-for-rbac --role Contributor --scopes /subscriptions/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx > "creds.json"
```

### Create a Provider Secret

```bash
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

```bash
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/docs/snippets/configure/azure/providerconfig.yaml
```
<!-- close "Azure" -->
{{% /tab %}}

{{% /tabs %}}

## Next Steps

Now that you have configured Crossplane with support for `PostgreSQLInstance`,
you can [provision infrastructure].
<!-- close "Crossplane (upstream)" -->
{{% /tab %}}

{{% tab "Downstream Distribution" %}}
## Start with a Downstream Distribution

Upbound, the founders of Crossplane, maintains a free and open source downstream
distribution of Crossplane which makes getting started with Crossplane easy.
Universal Crossplane, or UXP for short, connects to Upbound's hosted management
console and Registry to make it easier to develop, debug, and manage Provider
and Configuration packages.

[Get started with Universal Crossplane] on the Upbound Documentation site.

<i>Want see another hosted Crossplane service listed? Please [reach out on
Slack][Slack] and our community will highlight it here!</i>

<!-- close "Downstream Distribution" -->
{{% /tab %}}

<!-- close "Crossplane Distros" -->
{{% /tabs %}}

## More Info

* See [Install] and [Configure] docs for installing alternate versions and more
  detailed instructions.

* See [Uninstall] docs for cleaning up resources, packages, and Crossplane
  itself.

* See [Providers] for installing and using different providers beyond AWS, GCP
  and Azure mentionded in this guide.

<!-- Named Links -->

[package]: {{<ref "../concepts/packages" >}}
[provision infrastructure]: {{<ref "provision-infrastructure" >}}
[create a configuration]: {{<ref "create-configuration" >}}
[Install]: {{<ref "../reference/install" >}}
[Configure]: {{<ref "../reference/configure" >}}
[Uninstall]: {{<ref "../reference/uninstall" >}}
[Kubernetes cluster]: https://kubernetes.io/docs/setup/
[Minikube]: https://minikube.sigs.k8s.io/docs/start/
[Helm]:https://helm.sh/docs/intro/using_helm/
[Kind]: https://kind.sigs.k8s.io/docs/user/quick-start/
[Crossplane packages]: {{<ref "../concepts/packages" >}}
[Slack]: http://slack.crossplane.io/
[up]: https://github.com/upbound/up
[Upbound documentation]: https://https://docs.upbound.io//docs
[Providers]: {{<ref "../concepts/providers" >}}
[Universal Crossplane]: https://https://docs.upbound.io/uxp/
[Get started with Universal Crossplane]: https://docs.upbound.io/uxp/install
[certified by the CNCF]: https://github.com/cncf/crossplane-conformance
[Crossplane GitHub]: https://github.com/crossplane/crossplane

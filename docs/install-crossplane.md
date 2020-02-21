---
title: Install Crossplane
toc: true
weight: 220
indent: true
---
# Install Crossplane

Crossplane can be easily installed into any existing Kubernetes cluster using the regularly published Helm chart.
The Helm chart contains all the custom resources and controllers needed to deploy and configure Crossplane.

## Pre-requisites

* [Kubernetes cluster](https://kubernetes.io/docs/setup/)
  * For example [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), minimum version `v0.28+`
* [Helm](https://docs.helm.sh/using_helm/), minimum version `v2.12.0+`.
  * For Helm 2, make sure Tiller is initialized with sufficient permissions to work on `crossplane-system` namespace.

## Installation

Helm charts for Crossplane are currently published to the `alpha` and `master` channels.
In the future, `beta` and `stable` will also be available.

### Alpha

The alpha channel is the most recent release of Crossplane that is considered ready for testing by the community.

Install with Helm 2:

```console
helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

Install with Helm 3:

If your Kubernetes version is lower than 1.15 and you'd like to install Crossplane via Helm 3, you'll need Helm v3.1.0+ that has the flag `--disable-openapi-validation`.

```console
kubectl create namespace crossplane-system
helm repo add crossplane-alpha https://charts.crossplane.io/alpha

# Kubernetes 1.15 and newer versions
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane

# Kubernetes 1.14 and older versions
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane --disable-openapi-validation
```

### Master

The `master` channel contains the latest commits, with all automated tests passing.
`master` is subject to instability, incompatibility, and features may be added or removed without much prior notice.
It is recommended to use one of the more stable channels, but if you want the absolute newest Crossplane installed, then you can use the `master` channel.

To install the Helm chart from master, you will need to pass the specific version returned by the `search` command:

Install with Helm 2:

```console
helm repo add crossplane-master https://charts.crossplane.io/master/
helm search crossplane --devel
helm install --name crossplane --namespace crossplane-system crossplane-master/crossplane --version <version>
```

For example:

```console
helm install --name crossplane --namespace crossplane-system crossplane-master/crossplane --version 0.0.0-249.637ccf9
```

Install with Helm 3:

If your Kubernetes version is lower than 1.15 and you'd like to install Crossplane via Helm 3, you'll need Helm v3.1.0+.

```console
kubectl create namespace crossplane-system
helm repo add crossplane-master https://charts.crossplane.io/master/
helm search repo crossplane --devel

# Kubernetes 1.15 and newer versions
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --version <version> --devel

# Kubernetes 1.14 and older versions
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --version <version> --devel --disable-openapi-validation
```

## Installing Cloud Provider Stacks

You can add additional functionality to Crossplane's control plane by installing Crossplane Stacks. For example, each 
supported cloud provider has its own corresponding stack that contains all the functionality for that particular cloud.
After a cloud provider's stack is installed, you will be able to provision and manage resources within that cloud 
from Crossplane.

### Installation with Helm

You can include deployment of additional infrastructure stacks into your helm installation by setting `clusterStacks.<stack-name>.deploy` to `true`.

For example, the following will install `master` version of the GCP stack.

Using Helm 2:

```console
helm install --name crossplane --namespace crossplane-system crossplane-master/crossplane --version <version> --set clusterStacks.gcp.deploy=true --set clusterStacks.gcp.version=master
```

Using Helm 3:

```console
kubectl create namespace crossplane-system
helm install crossplane --namespace crossplane-system crossplane-master/crossplane --version <version> --set clusterStacks.gcp.deploy=true --set clusterStacks.gcp.version=master --devel
```

See [helm configuration parameters](#configuration) for supported stacks and parameters.

### Manual Installation

After Crossplane has been installed, it is possible to extend Crossplane's functionality by installing Crossplane stacks.

#### GCP Stack

To get started with Google Cloud Platform (GCP), create a file named `stack-gcp.yaml` with the following content:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: gcp
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-gcp
  namespace: gcp
spec:
  package: "crossplane/stack-gcp:master"
```

Then you can install the GCP stack into Crossplane in the `gcp` namespace with the following command:

```console
kubectl apply -f stack-gcp.yaml
```

#### AWS Stack

To get started with Amazon Web Services (AWS), create a file named `stack-aws.yaml` with the following content:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: aws
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-aws
  namespace: aws
spec:
  package: "crossplane/stack-aws:master"
```

Then you can install the AWS stack into Crossplane in the `aws` namespace with the following command:

```console
kubectl apply -f stack-aws.yaml
```

#### Azure Stack

To get started with Microsoft Azure, create a file named `stack-azure.yaml` with the following content:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: azure
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-azure
  namespace: azure
spec:
  package: "crossplane/stack-azure:master"
```

Then you can install the Azure stack into Crossplane in the `azure` namespace with the following command:

```console
kubectl apply -f stack-azure.yaml
```

#### Rook Stack

To get started with Rook, create a file named `stack-rook.yaml` with the following content:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rook
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-rook
  namespace: rook
spec:
  package: "crossplane/stack-rook:master"
```

Then you can install the Rook stack into Crossplane in the `rook` namespace with the following command:

```console
kubectl apply -f stack-rook.yaml
```

### Uninstalling Cloud Provider Stacks

The cloud provider stacks can be uninstalled simply by deleting the stack resources from the cluster with a command similar to what's shown below.
**Note** that this will also **delete** any resources that Crossplane has provisioned in the cloud provider if their `ReclaimPolicy` is set to `Delete`.

After you have ensured that you are completely done with all your cloud provider resources, you can then run one of the commands below,
depending on which cloud provider you are removing, to remove its stack from Crossplane:

#### Uninstalling GCP

```console
kubectl delete -f stack-gcp.yaml
```

#### Uninstalling AWS

```console
kubectl delete -f stack-aws.yaml
```

#### Uninstalling Azure

```console
kubectl delete -f stack-azure.yaml
```

#### Uninstalling Rook

```console
kubectl delete -f stack-rook.yaml
```

## Uninstalling the Chart

To uninstall/delete the `crossplane` deployment:

```console
helm delete --purge crossplane
```

That command removes all Kubernetes components associated with Crossplane, including all the custom resources and controllers.

## Configuration

The following tables lists the configurable parameters of the Crossplane chart and their default values.

| Parameter                        | Description                                                     | Default                                                |
| -------------------------------- | --------------------------------------------------------------- | ------------------------------------------------------ |
| `image.repository`               | Image                                                           | `crossplane/crossplane`                                |
| `image.tag`                      | Image tag                                                       | `master`                                               |
| `image.pullPolicy`               | Image pull policy                                               | `Always`                                               |
| `imagePullSecrets`               | Names of image pull secrets to use                              | `dockerhub`                                            |
| `replicas`                       | The number of replicas to run for the Crossplane operator       | `1`                                                    |
| `deploymentStrategy`             | The deployment strategy for the Crossplane operator             | `RollingUpdate`                                        |
| `clusterStacks.aws.deploy`       | Deploy AWS stack                                                | `false`    
| `clusterStacks.aws.version`      | AWS stack version to deploy                                     | `<latest released version>`   
| `clusterStacks.gcp.deploy`       | Deploy GCP stack                                                | `false`    
| `clusterStacks.gcp.version`      | GCP stack version to deploy                                     | `<latest released version>`   
| `clusterStacks.azure.deploy`     | Deploy Azure stack                                              | `false`    
| `clusterStacks.azure.version`    | Azure stack version to deploy                                   | `<latest released version>`   
| `clusterStacks.rook.deploy`      | Deploy Rook stack                                               | `false`    
| `clusterStacks.rook.version`     | Rook stack version to deploy                                    | `<latest released version>`   
| `personas.deploy`                | Install roles and bindings for Crossplane user personas         | `true`     
| `templateStacks.enabled`         | Enable experimental template stacks support                     | `true`     
| `templateStacks.controllerImage` | Template Stack controller image                                 | `crossplane/templating-controller:v0.2.1`
 
### Command Line

You can pass the settings with helm command line parameters.
Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.
For example, the following command will install Crossplane with an image pull policy of `IfNotPresent`.

```console
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane --set image.pullPolicy=IfNotPresent
```

### Settings File

Alternatively, a yaml file that specifies the values for the above parameters (`values.yaml`) can be provided while installing the chart.

```console
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane -f values.yaml
```

Here are the sample settings to get you started.

```yaml
replicas: 1

deploymentStrategy: RollingUpdate

image:
  repository: crossplane/crossplane
  tag: master
  pullPolicy: Always

imagePullSecrets:
- dockerhub
```

# Install Crossplane

Crossplane can be easily installed into any existing Kubernetes cluster using
the regularly published Helm chart. The Helm chart contains all the custom
resources and controllers needed to deploy and configure Crossplane.

## Pre-requisites

* [Kubernetes cluster], minimum version `v1.15.0+`
* [Helm], minimum version `v3.0.0+`.

## Installation

Helm charts for Crossplane are currently published to the `alpha` and `master`
channels. In the future, `beta` and `stable` will also be available.

### Alpha

The alpha channel is the most recent release of Crossplane that is considered
ready for testing by the community.

```console
kubectl create namespace crossplane-system
helm repo add crossplane-alpha https://charts.crossplane.io/alpha

helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

### Master

The `master` channel contains the latest commits, with all automated tests
passing. `master` is subject to instability, incompatibility, and features may
be added or removed without much prior notice. It is recommended to use one of
the more stable channels, but if you want the absolute newest Crossplane
installed, then you can use the `master` channel.

To install the Helm chart from master, you will need to pass the specific
version returned by the `search` command:

```console
kubectl create namespace crossplane-system
helm repo add crossplane-master https://charts.crossplane.io/master/
helm search repo crossplane-master --devel

helm install crossplane --namespace crossplane-system crossplane-master/crossplane --version <version> --devel
```

## Uninstalling the Chart

To uninstall/delete the `crossplane` deployment:

```console
helm delete crossplane --namespace crossplane-system
```

That command removes all Kubernetes components associated with Crossplane,
including all the custom resources and controllers.

## Configuration

The following tables lists the configurable parameters of the Crossplane chart
and their default values.

| Parameter                        | Description                                                     | Default                                                |
| -------------------------------- | --------------------------------------------------------------- | ------------------------------------------------------ |
| `image.repository`               | Image                                                           | `crossplane/crossplane`                                |
| `image.tag`                      | Image tag                                                       | `master`                                               |
| `image.pullPolicy`               | Image pull policy                                               | `Always`                                               |
| `imagePullSecrets`               | Names of image pull secrets to use                              | `dockerhub`                                            |
| `replicas`                       | The number of replicas to run for the Crossplane operator       | `1`                                                    |
| `deploymentStrategy`             | The deployment strategy for the Crossplane operator             | `RollingUpdate`                                        |
| `priorityClassName`        | Priority class name for crossplane and package manager pods       | `""`
| `resourcesCrossplane.limits.cpu`        | CPU resource limits for Crossplane                       | `100m`
| `resourcesCrossplane.limits.memory`     | Memory resource limits for Crossplane                    | `512Mi`
| `resourcesCrossplane.requests.cpu`      | CPU resource requests for Crossplane                     | `100m`
| `resourcesCrossplane.requests.memory`   | Memory resource requests for Crossplane                  | `256Mi`
| `forceImagePullPolicy`           | Force the named ImagePullPolicy on Package install and containers | `""`

### Command Line

You can pass the settings with helm command line parameters. Specify each
parameter using the `--set key=value[,key=value]` argument to `helm install`.
For example, the following command will install Crossplane with an image pull
policy of `IfNotPresent`.

```console
helm install --namespace crossplane-system crossplane-alpha/crossplane --set image.pullPolicy=IfNotPresent
```

### Settings File

Alternatively, a yaml file that specifies the values for the above parameters
(`values.yaml`) can be provided while installing the chart.

```console
helm install crossplane --namespace crossplane-system crossplane-alpha/crossplane -f values.yaml
```

Here are the sample settings to get you started.

```yaml
replicas: 1

deploymentStrategy: RollingUpdate

image:
  repository: crossplane/crossplane
  tag: alpha
  pullPolicy: Always

imagePullSecrets:
- dockerhub
```

<!-- Named Links -->

[Kubernetes cluster]: https://kubernetes.io/docs/setup/
[Minikube]: https://kubernetes.io/docs/tasks/tools/install-minikube/
[Helm]: https://docs.helm.sh/using_helm/

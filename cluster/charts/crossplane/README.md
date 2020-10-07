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

| Parameter | Description | Default |
| --- | --- | --- |
| `image.repository` | Image | `crossplane/crossplane` |
| `image.tag` | Image tag | `master` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `imagePullSecrets` | Names of image pull secrets to use | `dockerhub` |
| `replicas` | The number of replicas to run for the Crossplane and RBAC Manager (if enabled) pods | `1` |
| `deploymentStrategy` | The deployment strategy for the Crossplane and RBAC Manager (if enabled) pods | `RollingUpdate` |
| `priorityClassName` | Priority class name for Crossplane and RBAC Manager (if enabled) pods | `""` |
| `resourcesCrossplane.limits.cpu` | CPU resource limits for Crossplane | `100m` |
| `resourcesCrossplane.limits.memory` | Memory resource limits for Crossplane | `512Mi` |
| `resourcesCrossplane.requests.cpu` | CPU resource requests for Crossplane | `100m` |
| `resourcesCrossplane.requests.memory` | Memory resource requests for Crossplane | `256Mi` |
| `packageCache.medium` | Storage medium for package cache. `Memory` means volume will be backed by tmpfs, which can be useful for development. | `""` |
| `packageCache.sizeLimit` | Size limit for package cache. If medium is `Memory` then maximum usage would be the minimum of this value the sum of all memory limits on containers in the Crossplane pod. | `5Mi` |
| `packageCache.pvc` | Name of the PersistentVolumeClaim to be used as the package cache. Providing a value will cause the default emptyDir volume to not be mounted. | `""` |
| `resourcesRBACManager.limits.cpu` | CPU resource limits for RBAC Manager | `100m` |
| `resourcesRBACManager.limits.memory` | Memory resource limits for RBAC Manager | `512Mi` |
| `resourcesRBACManager.requests.cpu` | CPU resource requests for RBAC Manager | `100m` |
| `resourcesRBACManager.requests.memory` | Memory resource requests for RBAC Manager | `256Mi` |
| `rbacManager.deploy` | Deploy RBAC Manager and its required roles | `true` |
| `rbacManager.managementPolicy`| The extent to which the RBAC manager will manage permissions. `All` indicates to manage all Crossplane controller and user roles. `Basic` indicates to only manage Crossplane controller roles and the `crossplane-admin`, `crossplane-edit`, and `crossplane-view` user roles. | `All` |
| `alpha.oam.enabled` | Deploy the `crossplane/oam-kubernetes-runtime` Helm chart | `false` |

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

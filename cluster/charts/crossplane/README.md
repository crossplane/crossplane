
Crossplane can be easily installed into any existing Kubernetes cluster using
the regularly published Helm chart. The Helm chart contains all the custom
resources and controllers needed to deploy and configure Crossplane.

## Pre-requisites

* [Kubernetes cluster], minimum version `v1.16.0+`
* [Helm], minimum version `v3.0.0+`.

## Installation

Helm charts for Crossplane are currently published to the `stable` and `master`
channels.

### Stable

The stable channel is the most recent release of Crossplane that is considered
ready for the community.

```console
kubectl create namespace crossplane-system

helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update

helm install crossplane --namespace crossplane-system crossplane-stable/crossplane
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
helm repo update
helm search repo crossplane-master --devel

helm install crossplane --namespace crossplane-system crossplane-master/crossplane --devel --version <version>
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
| `affinity` | Add `affinities` to the Crossplane pod deployment. | `{}` |
| `args` | Add custom arguments to the Crossplane pod. | `[]` |
| `configuration.packages` | A list of Configuration packages to install. | `[]` |
| `customAnnotations` | Add custom `annotations` to the Crossplane pod deployment. | `{}` |
| `customLabels` | Add custom `labels` to the Crossplane pod deployment. | `{}` |
| `deploymentStrategy` | The deployment strategy for the Crossplane and RBAC Manager pods. | `"RollingUpdate"` |
| `extraEnvVarsCrossplane` | Add custom environmental variables to the Crossplane pod deployment. Replaces any `.` in a variable name with `_`. For example, `SAMPLE.KEY=value1` becomes `SAMPLE_KEY=value1`. | `{}` |
| `extraEnvVarsRBACManager` | Add custom environmental variables to the RBAC Manager pod deployment. Replaces any `.` in a variable name with `_`. For example, `SAMPLE.KEY=value1` becomes `SAMPLE_KEY=value1`. | `{}` |
| `extraVolumeMountsCrossplane` | Add custom `volumeMounts` to the Crossplane pod. | `{}` |
| `extraVolumesCrossplane` | Add custom `volumes` to the Crossplane pod. | `{}` |
| `hostNetwork` | Enable `hostNetwork` for the Crossplane deployment. Caution: enabling `hostNetwork`` grants the Crossplane Pod access to the host network namespace. | `false` |
| `image.pullPolicy` | The image pull policy used for Crossplane and RBAC Manager pods. | `"IfNotPresent"` |
| `image.repository` | Repository for the Crossplane pod image. | `"crossplane/crossplane"` |
| `image.tag` | The Crossplane image tag. Defaults to the value of `appVersion` in Chart.yaml. | `""` |
| `imagePullSecrets` | The imagePullSecret names to add to the Crossplane ServiceAccount. | `{}` |
| `leaderElection` | Enable [leader election](https://docs.crossplane.io/latest/concepts/pods/#leader-election) for the Crossplane pod. | `true` |
| `metrics.enabled` | Enable Prometheus path, port and scrape annotations and expose port 8080 for both the Crossplane and RBAC Manager pods. | `false` |
| `nodeSelector` | Add `nodeSelectors` to the Crossplane pod deployment. | `{}` |
| `packageCache.configMap` | The name of a ConfigMap to use as the package cache. Disables the default package cache `emptyDir` Volume. | `""` |
| `packageCache.medium` | Set to `Memory` to hold the package cache in a RAM-backed file system. Useful for Crossplane development. | `""` |
| `packageCache.pvc` | The name of a PersistentVolumeClaim to use as the package cache. Disables the default package cache `emptyDir` Volume. | `""` |
| `packageCache.sizeLimit` | The size limit for the package cache. If medium is `Memory` the `sizeLimit` can't exceed Node memory. | `"20Mi"` |
| `podSecurityContextCrossplane` | Add a custom `securityContext` to the Crossplane pod. | `{}` |
| `podSecurityContextRBACManager` | Add a custom `securityContext` to the RBAC Manager pod. | `{}` |
| `priorityClassName` | The PriorityClass name to apply to the Crossplane and RBAC Manager pods. | `""` |
| `provider.packages` | A list of Provider packages to install. | `[]` |
| `rbacManager.affinity` | Add `affinities` to the RBAC Manager pod deployment. | `{}` |
| `rbacManager.args` | Add custom arguments to the RBAC Manager pod. | `[]` |
| `rbacManager.deploy` | Deploy the RBAC Manager pod and its required roles. | `true` |
| `rbacManager.leaderElection` | Enable [leader election](https://docs.crossplane.io/latest/concepts/pods/#leader-election) for the RBAC Manager pod. | `true` |
| `rbacManager.managementPolicy` | Defines the Roles and ClusterRoles the RBAC Manager creates and manages. - A policy of `Basic` creates and binds Roles only for the Crossplane ServiceAccount, Provider ServiceAccounts and creates Crossplane ClusterRoles. - A policy of `All` includes all the `Basic` settings and also creates Crossplane Roles in all namespaces. - Read the Crossplane docs for more information on the [RBAC Roles and ClusterRoles](https://docs.crossplane.io/latest/concepts/pods/#crossplane-clusterroles) | `"Basic"` |
| `rbacManager.nodeSelector` | Add `nodeSelectors` to the RBAC Manager pod deployment. | `{}` |
| `rbacManager.replicas` | The number of RBAC Manager pod `replicas` to deploy. | `1` |
| `rbacManager.skipAggregatedClusterRoles` | Don't install aggregated Crossplane ClusterRoles. | `false` |
| `rbacManager.tolerations` | Add `tolerations` to the RBAC Manager pod deployment. | `[]` |
| `registryCaBundleConfig.key` | The ConfigMap key containing a custom CA bundle to enable fetching packages from registries with unknown or untrusted certificates. | `""` |
| `registryCaBundleConfig.name` | The ConfigMap name containing a custom CA bundle to enable fetching packages from registries with unknown or untrusted certificates. | `""` |
| `replicas` | The number of Crossplane pod `replicas` to deploy. | `1` |
| `resourcesCrossplane.limits.cpu` | CPU resource limits for the Crossplane pod. | `"100m"` |
| `resourcesCrossplane.limits.memory` | Memory resource limits for the Crossplane pod. | `"512Mi"` |
| `resourcesCrossplane.requests.cpu` | CPU resource requests for the Crossplane pod. | `"100m"` |
| `resourcesCrossplane.requests.memory` | Memory resource requests for the Crossplane pod. | `"256Mi"` |
| `resourcesRBACManager.limits.cpu` | CPU resource limits for the RBAC Manager pod. | `"100m"` |
| `resourcesRBACManager.limits.memory` | Memory resource limits for the RBAC Manager pod. | `"512Mi"` |
| `resourcesRBACManager.requests.cpu` | CPU resource requests for the RBAC Manager pod. | `"100m"` |
| `resourcesRBACManager.requests.memory` | Memory resource requests for the RBAC Manager pod. | `"256Mi"` |
| `securityContextCrossplane.allowPrivilegeEscalation` | Enable `allowPrivilegeEscalation` for the Crossplane pod. | `false` |
| `securityContextCrossplane.readOnlyRootFilesystem` | Set the Crossplane pod root file system as read-only. | `true` |
| `securityContextCrossplane.runAsGroup` | The group ID used by the Crossplane pod. | `65532` |
| `securityContextCrossplane.runAsUser` | The user ID used by the Crossplane pod. | `65532` |
| `securityContextRBACManager.allowPrivilegeEscalation` | Enable `allowPrivilegeEscalation` for the RBAC Manager pod. | `false` |
| `securityContextRBACManager.readOnlyRootFilesystem` | Set the RBAC Manager pod root file system as read-only. | `true` |
| `securityContextRBACManager.runAsGroup` | The group ID used by the RBAC Manager pod. | `65532` |
| `securityContextRBACManager.runAsUser` | The user ID used by the RBAC Manager pod. | `65532` |
| `serviceAccount.customAnnotations` | Add custom `annotations` to the Crossplane ServiceAccount. | `{}` |
| `tolerations` | Add `tolerations` to the Crossplane pod deployment. | `[]` |
| `webhooks.enabled` | Enable webhooks for Crossplane and installed Provider packages. | `true` |
| `xfn.args` | Add custom arguments to the Composite functions runner container. | `[]` |
| `xfn.cache.configMap` | The name of a ConfigMap to use as the Composite function runner package cache. Disables the default Composite function runner package cache `emptyDir` Volume. | `""` |
| `xfn.cache.medium` | Set to `Memory` to hold the Composite function runner package cache in a RAM-backed file system. Useful for Crossplane development. | `""` |
| `xfn.cache.pvc` | The name of a PersistentVolumeClaim to use as the Composite function runner package cache. Disables the default Composite function runner package cache `emptyDir` Volume. | `""` |
| `xfn.cache.sizeLimit` | The size limit for the Composite function runner package cache. If medium is `Memory` the `sizeLimit` can't exceed Node memory. | `"1Gi"` |
| `xfn.enabled` | Enable the alpha Composition functions (`xfn`) sidecar container. Also requires Crossplane `args` value `--enable-composition-functions` set. | `false` |
| `xfn.extraEnvVars` | Add custom environmental variables to the Composite function runner container. Replaces any `.` in a variable name with `_`. For example, `SAMPLE.KEY=value1` becomes `SAMPLE_KEY=value1`. | `{}` |
| `xfn.image.pullPolicy` | Composite function runner container image pull policy. | `"IfNotPresent"` |
| `xfn.image.repository` | Composite function runner container image. | `"crossplane/xfn"` |
| `xfn.image.tag` | Composite function runner container image tag. Defaults to the value of `appVersion` in Chart.yaml. | `""` |
| `xfn.resources.limits.cpu` | CPU resource limits for the Composite function runner container. | `"2000m"` |
| `xfn.resources.limits.memory` | Memory resource limits for the Composite function runner container. | `"2Gi"` |
| `xfn.resources.requests.cpu` | CPU resource requests for the Composite function runner container. | `"1000m"` |
| `xfn.resources.requests.memory` | Memory resource requests for the Composite function runner container. | `"1Gi"` |
| `xfn.securityContext.allowPrivilegeEscalation` | Enable `allowPrivilegeEscalation` for the Composite function runner container. | `false` |
| `xfn.securityContext.capabilities.add` | Set Linux capabilities for the Composite function runner container. The default values allow the container to create an unprivileged user namespace for running Composite function containers. | `["SETUID","SETGID"]` |
| `xfn.securityContext.readOnlyRootFilesystem` | Set the Composite function runner container root file system as read-only. | `true` |
| `xfn.securityContext.runAsGroup` | The group ID used by the Composite function runner container. | `65532` |
| `xfn.securityContext.runAsUser` | The user ID used by the Composite function runner container. | `65532` |
| `xfn.securityContext.seccompProfile.type` | Apply a `seccompProfile` to the Composite function runner container. The default value allows the Composite function runner container permissions to use the `unshare` syscall. | `"Unconfined"` |

### Command Line

You can pass the settings with helm command line parameters. Specify each
parameter using the `--set key=value[,key=value]` argument to `helm install`.
For example, the following command will install Crossplane with an image pull
policy of `IfNotPresent`.

```console
helm install --namespace crossplane-system crossplane-stable/crossplane --set image.pullPolicy=IfNotPresent
```

### Settings File

Alternatively, a yaml file that specifies the values for the above parameters
(`values.yaml`) can be provided while installing the chart.

```console
helm install crossplane --namespace crossplane-system crossplane-stable/crossplane -f values.yaml
```

Here are the sample settings to get you started.

```yaml
replicas: 1

deploymentStrategy: RollingUpdate

image:
  repository: crossplane/crossplane
  tag: alpha
  pullPolicy: Always
```

<!-- Named Links -->

[Kubernetes cluster]: https://kubernetes.io/docs/setup/
[Minikube]: https://kubernetes.io/docs/tasks/tools/install-minikube/
[Helm]: https://docs.helm.sh/using_helm/


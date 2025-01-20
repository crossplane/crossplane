
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
| `dnsPolicy` | Specify the `dnsPolicy` to be used by the Crossplane pod. | `""` |
| `extraEnvVarsCrossplane` | Add custom environmental variables to the Crossplane pod deployment. Replaces any `.` in a variable name with `_`. For example, `SAMPLE.KEY=value1` becomes `SAMPLE_KEY=value1`. | `{}` |
| `extraEnvVarsRBACManager` | Add custom environmental variables to the RBAC Manager pod deployment. Replaces any `.` in a variable name with `_`. For example, `SAMPLE.KEY=value1` becomes `SAMPLE_KEY=value1`. | `{}` |
| `extraObjects` | To add arbitrary Kubernetes Objects during a Helm Install | `[]` |
| `extraVolumeMountsCrossplane` | Add custom `volumeMounts` to the Crossplane pod. | `{}` |
| `extraVolumesCrossplane` | Add custom `volumes` to the Crossplane pod. | `{}` |
| `function.packages` | A list of Function packages to install | `[]` |
| `hostNetwork` | Enable `hostNetwork` for the Crossplane deployment. Caution: enabling `hostNetwork` grants the Crossplane Pod access to the host network namespace. Consider setting `dnsPolicy` to `ClusterFirstWithHostNet`. | `false` |
| `image.pullPolicy` | The image pull policy used for Crossplane and RBAC Manager pods. | `"IfNotPresent"` |
| `image.repository` | Repository for the Crossplane pod image. | `"xpkg.upbound.io/crossplane/crossplane"` |
| `image.tag` | The Crossplane image tag. Defaults to the value of `appVersion` in `Chart.yaml`. | `""` |
| `imagePullSecrets` | The imagePullSecret names to add to the Crossplane ServiceAccount. | `[]` |
| `leaderElection` | Enable [leader election](https://docs.crossplane.io/latest/concepts/pods/#leader-election) for the Crossplane pod. | `true` |
| `metrics.enabled` | Enable Prometheus path, port and scrape annotations and expose port 8080 for both the Crossplane and RBAC Manager pods. | `false` |
| `nodeSelector` | Add `nodeSelectors` to the Crossplane pod deployment. | `{}` |
| `packageCache.configMap` | The name of a ConfigMap to use as the package cache. Disables the default package cache `emptyDir` Volume. | `""` |
| `packageCache.medium` | Set to `Memory` to hold the package cache in a RAM backed file system. Useful for Crossplane development. | `""` |
| `packageCache.pvc` | The name of a PersistentVolumeClaim to use as the package cache. Disables the default package cache `emptyDir` Volume. | `""` |
| `packageCache.sizeLimit` | The size limit for the package cache. If medium is `Memory` the `sizeLimit` can't exceed Node memory. | `"20Mi"` |
| `packageManager.enableAutomaticDependencyDowngrade` | Enable automatic dependency version downgrades. This configuration is only used when `--enable-dependency-version-upgrades` flag is passed. | `false` |
| `podSecurityContextCrossplane` | Add a custom `securityContext` to the Crossplane pod. | `{}` |
| `podSecurityContextRBACManager` | Add a custom `securityContext` to the RBAC Manager pod. | `{}` |
| `priorityClassName` | The PriorityClass name to apply to the Crossplane and RBAC Manager pods. | `""` |
| `provider.packages` | A list of Provider packages to install. | `[]` |
| `rbacManager.affinity` | Add `affinities` to the RBAC Manager pod deployment. | `{}` |
| `rbacManager.args` | Add custom arguments to the RBAC Manager pod. | `[]` |
| `rbacManager.deploy` | Deploy the RBAC Manager pod and its required roles. | `true` |
| `rbacManager.leaderElection` | Enable [leader election](https://docs.crossplane.io/latest/concepts/pods/#leader-election) for the RBAC Manager pod. | `true` |
| `rbacManager.nodeSelector` | Add `nodeSelectors` to the RBAC Manager pod deployment. | `{}` |
| `rbacManager.replicas` | The number of RBAC Manager pod `replicas` to deploy. | `1` |
| `rbacManager.revisionHistoryLimit` | The number of RBAC Manager ReplicaSets to retain. | `nil` |
| `rbacManager.skipAggregatedClusterRoles` | Don't install aggregated Crossplane ClusterRoles. | `false` |
| `rbacManager.tolerations` | Add `tolerations` to the RBAC Manager pod deployment. | `[]` |
| `rbacManager.topologySpreadConstraints` | Add `topologySpreadConstraints` to the RBAC Manager pod deployment. | `[]` |
| `registryCaBundleConfig.key` | The ConfigMap key containing a custom CA bundle to enable fetching packages from registries with unknown or untrusted certificates. | `""` |
| `registryCaBundleConfig.name` | The ConfigMap name containing a custom CA bundle to enable fetching packages from registries with unknown or untrusted certificates. | `""` |
| `replicas` | The number of Crossplane pod `replicas` to deploy. | `1` |
| `resourcesCrossplane.limits.cpu` | CPU resource limits for the Crossplane pod. | `"500m"` |
| `resourcesCrossplane.limits.memory` | Memory resource limits for the Crossplane pod. | `"1024Mi"` |
| `resourcesCrossplane.requests.cpu` | CPU resource requests for the Crossplane pod. | `"100m"` |
| `resourcesCrossplane.requests.memory` | Memory resource requests for the Crossplane pod. | `"256Mi"` |
| `resourcesRBACManager.limits.cpu` | CPU resource limits for the RBAC Manager pod. | `"100m"` |
| `resourcesRBACManager.limits.memory` | Memory resource limits for the RBAC Manager pod. | `"512Mi"` |
| `resourcesRBACManager.requests.cpu` | CPU resource requests for the RBAC Manager pod. | `"100m"` |
| `resourcesRBACManager.requests.memory` | Memory resource requests for the RBAC Manager pod. | `"256Mi"` |
| `revisionHistoryLimit` | The number of Crossplane ReplicaSets to retain. | `nil` |
| `securityContextCrossplane.allowPrivilegeEscalation` | Enable `allowPrivilegeEscalation` for the Crossplane pod. | `false` |
| `securityContextCrossplane.readOnlyRootFilesystem` | Set the Crossplane pod root file system as read-only. | `true` |
| `securityContextCrossplane.runAsGroup` | The group ID used by the Crossplane pod. | `65532` |
| `securityContextCrossplane.runAsUser` | The user ID used by the Crossplane pod. | `65532` |
| `securityContextRBACManager.allowPrivilegeEscalation` | Enable `allowPrivilegeEscalation` for the RBAC Manager pod. | `false` |
| `securityContextRBACManager.readOnlyRootFilesystem` | Set the RBAC Manager pod root file system as read-only. | `true` |
| `securityContextRBACManager.runAsGroup` | The group ID used by the RBAC Manager pod. | `65532` |
| `securityContextRBACManager.runAsUser` | The user ID used by the RBAC Manager pod. | `65532` |
| `service.customAnnotations` | Configure annotations on the service object. Only enabled when webhooks.enabled = true | `{}` |
| `serviceAccount.create` | Specifies whether Crossplane ServiceAccount should be created | `true` |
| `serviceAccount.customAnnotations` | Add custom `annotations` to the Crossplane ServiceAccount. | `{}` |
| `serviceAccount.name` | Provide the name of an already created Crossplane ServiceAccount. Required when `serviceAccount.create` is `false` | `""` |
| `tolerations` | Add `tolerations` to the Crossplane pod deployment. | `[]` |
| `topologySpreadConstraints` | Add `topologySpreadConstraints` to the Crossplane pod deployment. | `[]` |
| `webhooks.enabled` | Enable webhooks for Crossplane and installed Provider packages. | `true` |

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
  repository: xpkg.upbound.io/crossplane/crossplane
  tag: alpha
  pullPolicy: Always
```

<!-- Named Links -->

[Kubernetes cluster]: https://kubernetes.io/docs/setup/
[Minikube]: https://kubernetes.io/docs/tasks/tools/install-minikube/
[Helm]: https://docs.helm.sh/using_helm/


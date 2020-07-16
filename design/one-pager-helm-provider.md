# Crossplane Helm Provider

* Owner: Hasan Turken (@turkenh)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

As a platform builder, as part of my Composite Resource creation, I would like to make some initial provisioning on my 
infrastructure resources after creating them. This provisioning could have different meanings depending on the type of 
the infrastructure. For example, for a database instance (e.g. `CloudSQLInstance`, `RDSInstance` ...), this could mean
creating additional [databases, users and roles](https://github.com/crossplane/crossplane/issues/29) which would require
a controller using clients of the database. This could be achieved via another crossplane provider similar to 
[Terraform’s MySQL Provider](https://www.terraform.io/docs/providers/mysql/index.html). 

When the infrastructure resource is a Kubernetes cluster (e.g. `GKECluster`, `EKSCluster` ...), by provisioning we usually
mean creating Kubernetes resources on the cluster which could be in the form of raw yaml manifests or in the form of
application packages. Helm is currently the most popular packaging solution for Kubernetes applications and a Crossplane
Helm Provider could enable easy and quick demonstration of Crossplane capabilities. 

This provider will enable deployment of helm charts to (remote) Kubernetes Clusters typically provisioned via
Crossplane. Considering the issues with helm 2 (e.g. security problems regarding tiller and lack of proper go 
clients/libraries for helm), **we will focus and only support Helm 3**. 

## Design

We will implement a Kubernetes controller watching `Release` custom resources and deploying helm charts with the desired
configuration. Since this controller needs to interact with Kubernetes API server, it is a good fit for [Kubernetes
native providers](https://github.com/crossplane/crossplane/blob/master/design/one-pager-k8s-native-providers.md#kubernetes-native-providers)
concept in Crossplane. By using existing [Kubernetes Provider](https://github.com/crossplane/crossplane/blob/master/design/one-pager-k8s-native-providers.md#proposal-kubernetes-provider-kind)
Kind, we will be able to manage helm releases in **Crossplane provisioned external clusters**, **existing external
clusters** and also **Crossplane control cluster** (a.k.a. local cluster).

### `Release` Custom Resource and Controller

```
apiVersion: helm.crossplane.io/v1alpha1
kind: Release
metadata:
  name: wordpress-example
spec:
  forProvider:
    chart:
      name: wordpress
      repository: https://charts.bitnami.com/bitnami
      version: 9.3.19
    namespace: wordpress
    values:
      mariadb:
        enabled: false
      externaldb:
        enabled: true
    set:
    - name: wordpressBlogName
      value: "Hello Crossplane"
    - name: externalDatabase.host
      valueFrom:
        secretKeyRef:
          name: dbconn
          key: host
    - name: externalDatabase.user
      valueFrom:
        secretKeyRef:
          name: dbconn
          key: username
    - name: externalDatabase.password
      valueFrom:
        secretKeyRef:
          name: dbconn
          key: password
  providerRef: 
    name: cluster-1-provider
  reclaimPolicy: Delete
```

We will implement helm controller using the [managed reconciler of crossplane runtime](https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed).

Following logic will be implemented for corresponding functions:

[Connect](https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalConnectorFn.Connect):

- Using provided Kubernetes Provider, create a kubernetes client and helm client as `ExternalClient`. 

[Observe](https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalClientFns.Observe):

1. Using [history action](https://github.com/helm/helm/blob/3d64c6bb5495d4e4426c27b181300fff45f95ff0/pkg/action/history.go#L43)
of helm client, get last [release](https://github.com/helm/helm/blob/3d64c6bb5495d4e4426c27b181300fff45f95ff0/pkg/release/release.go#L22)
information.
    1. If there is no last release, return [`ExternalObservation`](https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalObservation)
    as `ResourceExists = false` which will result in `Create` to be called.
    2. If there is last release, compare last [release](https://github.com/helm/helm/blob/3d64c6bb5495d4e4426c27b181300fff45f95ff0/pkg/release/release.go#L22)
    information with the desired spec (e.g. `spec.forProvider`)
        1. If desired state matches with observed, return `ResourceUpToDate = true`
        2. If desired state differs from observed, return `ResourceUpToDate = false`

[Create](https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalClientFns.Create):

1. Fetch the helm chart [by constructing URL as](https://chartmuseum.com/docs/#helm-chart-repository) `<spec.forProvider.chart.repository>/charts/<spec.forProvider.chart.name>-<spec.forProvider.chart.version>.tgz`
2. Prepare desired config by combining `spec.forProvider.values` and `spec.forProvider.set`s
3. Create the `spec.forProvider.namespace`, if not exists.
4. Using [install action](https://github.com/helm/helm/blob/3d64c6bb5495d4e4426c27b181300fff45f95ff0/pkg/action/install.go#L150)
of helm client, `helm install` with the desired config.

[Update](https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalClientFns.Update):

1. Fetch (if not exists already) the helm chart [by constructing URL as](https://chartmuseum.com/docs/#helm-chart-repository) `<spec.forProvider.chart.repository>/<spec.forProvider.chart.name>-<spec.forProvider.chart.version>.tgz`
2. Prepare desired config by combining `spec.forProvider.values` and `spec.forProvider.set`s
3. Using [upgrade action](https://github.com/helm/helm/blob/3d64c6bb5495d4e4426c27b181300fff45f95ff0/pkg/action/upgrade.go#L71) of
helm client, `helm upgrade` with the desired config.

[Delete](https://godoc.org/github.com/crossplane/crossplane-runtime/pkg/reconciler/managed#ExternalClientFns.Delete):

1. Using [uninstall action](https://github.com/helm/helm/blob/3d64c6bb5495d4e4426c27b181300fff45f95ff0/pkg/action/uninstall.go#L49) of
helm client, `helm uninstall` the release.
2. Once uninstall is successful, finalizer is removed (by crossplane-runtime).

#### Namespaced vs Cluster Scoped

Helm releases are namespaced and with Helm 3, Helm itself also keeping release information in the same namespace where
deployment is made. This was an improvement and actually a better fit to Kubernetes deployment model compared to keeping
all release information in a single namespace (`kube-system` by default) in Helm 2. If we were designing this helm
controller for in cluster deployment only (similar to existing alternatives), it would make super sense to make it
simply `Namespaced` and deploy helm release into that namespace. However, in our case, we want to primarily support
deploying to remote clusters. So we cannot use namespace of the custom resource to decide Helm deployment namespace. 

We have two options here:

1. `Cluster` scoped resource and `spec.namespace` for helm deployment resource.
2. `Namespaced` resource and an optional `spec.remoteNamespace`

Even option 2 makes more sense for local deployment case, we need to design for remote clusters first (e.g. crossplane 
provisioned) and option 1 better fits current patterns in Crossplane for managed resources (i.e. all existing managed
resources are Cluster Scoped).

## Using in Composition

### Use Case Example 1: MonitoredCluster Composite Resource

As a platform builder, I would like to define MonitoredCluster as a Crossplane Composite Resource which is basically 
a managed Kubernetes Cluster of my preferred Cloud + our Monitoring Stack configured properly.

### Use Case Example 2: WordpressCluster Composite Resource

As a platform builder, I would like to define WordpressCluster as a Crossplane Composite Resource which creates an
external database, a Kubernetes cluster, deploys Wordpress application.

We can use `Release` resource in a composition to provision a cluster created by Crossplane. Following example shows
how to define a composition for a Wordpress application deployed on a Kubernetes cluster and consuming a database
both created with Crossplane. 

```
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: wordpressclusters.apps.example.org
spec:
  writeConnectionSecretsToNamespace: crossplane-system
  reclaimPolicy: Delete
  from:
    apiVersion: apps.example.org/v1alpha1
    kind: WordpressCluster
  to:
    - base:
        apiVersion: container.gcp.crossplane.io/v1beta1
        kind: GKECluster
        spec:
          providerRef:
            name: gcp-provider
          forProvider:
            addonsConfig:
              kubernetesDashboard:
                disabled: true
              networkPolicyConfig:
                disabled: true
            databaseEncryption:
              state: DECRYPTED
            defaultMaxPodsConstraint:
              maxPodsPerNode: 110
            description: Wordpress Cluster
            ipAllocationPolicy:
              createSubnetwork: true
              useIpAliases: true
            networkPolicy:
              enabled: false
            legacyAbac:
              enabled: false
            podSecurityPolicyConfig:
              enabled: false
            verticalPodAutoscaling:
              enabled: false
            masterAuth:
              username: admin
            loggingService: logging.googleapis.com/kubernetes
            monitoringService: monitoring.googleapis.com/kubernetes
            networkRef:
              name: gcp-wpclusters-network
            location: us-central1
            locations:
              - us-central1-a
          writeConnectionSecretToRef:
            namespace: crossplane-system
          reclaimPolicy: Delete
      patches:
        - fromFieldPath: "metadata.name"
          toFieldPath: "metadata.name"
        - fromFieldPath: "metadata.name"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
      connectionDetails:
        - fromConnectionSecretKey: kubeconfig
    - base:
        apiVersion: database.gcp.crossplane.io/v1beta1
        kind: CloudSQLInstance
        spec:
          forProvider:
            databaseVersion: MYSQL_5_6
            region: us-central1
            settings:
              tier: db-custom-1-3840
              dataDiskType: PD_SSD
              ipConfiguration:
                ipv4Enabled: true
                authorizedNetworks:
                  - value: "0.0.0.0/0"
          writeConnectionSecretToRef:
            namespace: crossplane-system
          providerRef:
            name: gcp-provider
          reclaimPolicy: Delete
      patches:
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.writeConnectionSecretToRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-mysql"
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.settings.dataDiskSizeGb"
      connectionDetails:
        - fromConnectionSecretKey: username
        - fromConnectionSecretKey: password
        - fromConnectionSecretKey: endpoint
        - name: port
          value: "5432"
    - base:
        apiVersion: kubernetes.crossplane.io/v1alpha1
        kind: Provider
      patches:
        - fromFieldPath: "metadata.name"
          toFieldPath: "spec.credentialsSecretRef.name"       
    - base:
        apiVersion: helm.crossplane.io/v1alpha1
        kind: Release
        spec:
          forProvider:
            chart:
              name: wordpress
              repository: https://charts.bitnami.com/bitnami
            namespace: wordpress
            values: |
              mariadb.enabled: false
              externaldb.enabled: true
            set:
            - name: externalDatabase.host
              valueFrom:
                secretKeyRef:
                  key: host
            - name: externalDatabase.user
              valueFrom:
                secretKeyRef:
                  key: username
            - name: externalDatabase.password
              valueFrom:
                secretKeyRef:
                  key: password
            - name: blogName
          reclaimPolicy: Delete
          providerSelector:
            matchControllerRef: true
      patches:
        - fromFieldPath: "spec.parameters.version"
          toFieldPath: "spec.forProvider.chart.version"
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.forProvider.set[0].valueFrom.secretKeyRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-mysql"
        - fromFieldPath: "metadata.uid"
          toFieldPath: "spec.forProvider.set[1].valueFrom.secretKeyRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-mysql"
          toFieldPath: "spec.forProvider.set[2].valueFrom.secretKeyRef.name"
          transforms:
            - type: string
              string:
                fmt: "%s-mysql"
        - fromFieldPath: "spec.parameters.blogName"
          toFieldPath: "spec.forProvider.set[4].value"
```

Our composition will create:

- a `GKECluster` whose connection secret will be consumed via a `Kubernetes Provider` which is set as `providerRef` in
  helm `Release` resource.
 
- a `CloudSQLInstance` whose connection details will be fed to Helm `Release` via `spec.forProvider.set` list and be
  directly consumed from the connection secret.

- a Helm `Release` resource, using `CloudSQLInstance` and deploying Wordpress helm chart into the `GKECluster` just
  created as part of the composite resource.

Infrastructure definition for above composition:

```
apiVersion: apiextensions.crossplane.io/v1alpha1
kind: InfrastructureDefinition
metadata:
  name: wordpressclusters.apps.example.org
spec:
  crdSpecTemplate:
    group: apps.example.org
    version: v1alpha1
    names:
      kind: WordpressCluster
      listKind: WordpressClusterList
      plural: wordpressclusters
      singular: wordpresscluster
    validation:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              parameters:
                type: object
                properties:
                  version:
                    type: string
                  storageGB:
                    type: integer
                  blogName:
                    type: string
                required:
                  - version
                  - storageGB
                  - blogName
            required:
              - parameters
```

## Alternatives Considered

Deploying Helm charts in a declarative way is a common need especially for GitOps flows. So, there are couple of 
existing helm operators in the community which could help here.

Available Helm Operators as of now:

- https://github.com/fluxcd/helm-operator:
    - Focusing on supporting Flux CD flows
    - Supporting both helm 2 and helm 3
    - Mainly designed to deploy into the same cluster where operator runs - no way to specify cluster per custom resource
- https://github.com/Kubedex/helm-controller
    - Runs Helm binary inside a Kubernetes job for install/upgrade/delete and does not use helm go packages. The idea
     sounds simple but not powerful enough when compared to using helm programmatically. It is a common pattern in 
     Kubernetes controllers to observe existing state before taking an action, which would not be feasible with
     this approach.
    - Allows changing the image used for job, so claims to support helm 3 if you provide an image with helm 3 binary.
-  https://github.com/rancher/helm-controller
    - Focuses on k3s and follows the job approach as kubedex/helm-controller
    - Have couple of important open issues seems related to using jobs
     (e.g. [#25](https://github.com/rancher/helm-controller/issues/25) and [#21](https://github.com/rancher/helm-controller/issues/21))
     and does not seem very active
-  https://github.com/bitnami-labs/helm-crd
    - Experimental and has been archived.

Flux helm operator seems to be the most complete in terms of features, but still missing an important one which is
deploying to remote clusters. There is no way to specify different clusters per `HelmRelease` resource. If we want to build
a Helm provider on top of it, we would need to deploy that operator to the remote cluster as well, which would
complicate the setup compared to just consuming Helm 3 go client and implement our controller with native support of
deploying to remote clusters.

Flux helm operator was mainly designed for helm 2 and later introduced helm 3 support. Designing only for helm 3
would mean a cleaner codebase and by implementing using crossplane-runtime (e.g. managed reconciler, kubernetes native 
providers), we can provide a first class crossplane experience.

## Feature Roadmap

|  | First Version (MVP) | v1.0 | Future Consideration |
|:--- |:---:|:---:|:---:|
| Charts from public Helm repos | ✔ |  |  |
| Charts from private Helm repos | ✔ |  |  |
| Install, upgrade, proper delete | ✔ |  |  |
| Local and remote clusters support | ✔ |  |  |
| Value overrides inline | ✔ |  |  |
| Set values from secrets, configmaps | ✔ |  |  |
| Parallel processing of multiple helm releases |  | ✔ |  |
| Rollback in case of failure |  | ✔ |  |
| Wait (not blocking but checking periodically) |  | ✔ |  |
| Support dependency management |  | ✔ |  |
| Empty version for latest (operator fills at first reconcile) |  | ✔ |  |
| Charts from public/private git repos |  |  | ✔ |
| Auto deploy latest version |  |  | ✔ |

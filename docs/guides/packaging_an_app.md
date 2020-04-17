---
title: Packaging an Application
toc: true
weight: 202
indent: true
---

# Packaging an Application

In the quick start guide, we demonstrated how Wordpress can be installed as a
Crossplane `Application`. Now we want to learn more about how to package any
application in a similar fashion. The good news is that we can use common
Kubernetes configuration tools, such as [Helm] and [Kustomize], which you may
already be familiar with.

## Setting up a Repository

The required components of an application repository are minimal. For example,
the required components of the [Wordpress application] we deployed in the quick
start are the following:

```
├── Dockerfile
├── .registry
│   ├── app.yaml
│   ├── behavior.yaml
│   ├── icon.svg
│   └── resources
│       ├── wordpress.apps.crossplane.io_wordpressinstances.crd.yaml
│       ├── wordpressinstance.icon.svg
│       ├── wordpressinstance.resource.yaml
│       └── wordpressinstance.ui-schema.yaml
├── helm-chart
│   ├── Chart.yaml
│   ├── templates
│   │   ├── app.yaml
│   │   ├── cluster.yaml
│   │   └── database.yaml
│   └── values.yaml
```

Let's take a look at each component in-depth.

### Dockerfile

The Dockerfile is only responsible for copying the configuration directory
(`helm-chart/` in this case) and the `.registry` directory. You can likely use a
very similar Dockerfile across all of your applications:

```Dockerfile
FROM alpine:3.7
WORKDIR /
COPY helm-chart /helm-chart
COPY .registry /.registry
```

### .registry

The `.registry` directory informs Crossplane how to install your application. It
consists of the following:

**app.yaml** `[required]`

The `app.yaml` file is responsible for defining the metadata for an application,
such as name, version, and required permissions. The Wordpress `app.yaml` is a
good reference for available fields:

```yaml
# Human readable title of application.
title: Wordpress

overviewShort: Cloud portable Wordpress deployments behind managed Kubernetes and SQL services are demonstrated in this Crossplane Stack.

overview: |-
 This Wordpress application uses a simple controller that uses Crossplane to orchestrate managed SQL services and managed Kubernetes clusters which are then used to run a Wordpress deployment.
 A simple Custom Resource Definition (CRD) is provided allowing for instances of this Crossplane managed Wordpress Application to be provisioned with a few lines of yaml.
 The Sample Wordpress Application is intended for demonstration purposes and should not be used to deploy production instances of Wordpress.

# Markdown description of this entry
readme: |-
 ### Create wordpresses
 Before wordpresses will provision, the Crossplane control cluster must
 be configured to connect to a provider (e.g. GCP, Azure, AWS).
 Once a provider is configured, starting the process of creating a
 Wordpress Application instance is easy.

 cat <<EOF | kubectl apply -f -
 apiVersion: wordpress.samples.apps.crossplane.io/v1alpha1
 kind: WordpressInstance
 metadata:
   name: wordpressinstance-sample
 EOF

 The stack (and Crossplane) will take care of the rest.

# Maintainer names and emails.
maintainers:
- name: Daniel Suskin
  email: daniel@upbound.io

# Owner names and emails.
owners:
- name: Daniel Suskin
  email: daniel@upbound.io

# Human readable company name.
company: Upbound

# Type of package: Provider, Stack, or Application
packageType: Application

# Keywords that describe this application and help search indexing
keywords:
- "samples"
- "examples"
- "tutorials"
- "wordpress"

# Links to more information about the application (about page, source code, etc.)
website: "https://upbound.io"
source: "https://github.com/crossplane/app-wordpress"

# RBAC Roles will be generated permitting this stack to use all verbs on all
# resources in the groups listed below.
permissionScope: Namespaced
dependsOn:
- crd: "kubernetesclusters.compute.crossplane.io/v1alpha1"
- crd: "mysqlinstances.database.crossplane.io/v1alpha1"
- crd: "kubernetesapplications.workload.crossplane.io/v1alpha1"
- crd: "kubernetesapplicationresources.workload.crossplane.io/v1alpha1"

# License SPDX name: https://spdx.org/licenses/
license: Apache-2.0
```

**behavior.yaml** `[required]`

While the `app.yaml` is responsible for metadata, that `behavior.yaml` is
responsible for operations. It is where you tell Crossplane how to create
resources in the cluster when an instance of the [CustomResourceDefinition] that
represents your application is created. Take a look at the Wordpress
`behavior.yaml` for reference:

```yaml
source:
  path: "helm-chart" # where the configuration data exists in Docker container
crd:
  kind: WordpressInstance # the kind of the CustomResourceDefinition
  apiVersion: wordpress.apps.crossplane.io/v1alpha1 # the apiVersion of the CustomResourceDefinition
engine:
  type: helm3 # the configuration engine to be used (helm3 and kustomize are valid options)
```

**icon.svg**

The `icon.svg` file is a logo for your application.

**resources/** `[required]`

The `resources/` directory contains the CustomResourceDefinition (CRD) that
Crossplane watches to apply the configuration data you supply. For the Wordpress
application, this is `wordpress.apps.crossplane.io_wordpressinstances.crd.yaml`.
CRDs can be generated from `go` code using projects like [controller-tools], or
can be written by hand. 

You can also supply metadata files for your CRD, which can be parsed by a user
interface. The files must match the name of the CRD kind for your application:

- `<your-kind>.icon.svg`: an image to be displayed for your application CRD
- `<your-kind>.resource.yaml`: a description of your application CRD
- `<your-kind>.ui-schema.yaml`: the configurable fields on your CRD that you
  wish to be displayed in a UI

Crossplane will take these files and apply them as [annotations] on the
installed application. They can then be parsed by a user interface.

### Configuration Directory

The configuration directory contains the actual manifests for deploying your
application. In the case of Wordpress, this includes a `KubernetesApplication`
(`helm-chart/templates/app.yaml`), a `KubernetesCluster` claim
(`helm-chart/templates/cluster.yaml`), and a `MySQLInstance` claim
(`helm-chart/templates/database.yaml`). The configuration tool for the manifests
in the directory should match the `engine` field in your
`.registry/behavior.yaml`. The options for engines at this time are `helm3` and
`kustomize`. Crossplane will pass values from the `spec` of the application's
CRD as variables in the manifests. For instance, the `provisionPolicy` field in
the `spec` of the `WordpressInstance` CRD will be passed to the Helm chart
defined in the `helm-chart/` directory.

<!-- Named Links -->

[Helm]: https://helm.sh/
[Kustomize]: https://kustomize.io/
[Wordpress application]: https://github.com/crossplane/app-wordpress
[CustomResourceDefinition]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[controller-tools]: https://github.com/kubernetes-sigs/controller-tools
[annotations]: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/

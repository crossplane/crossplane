# Overview

![Crossplane](media/banner.png)

Crossplane is an open source control plane that allows you to manage
applications and infrastructure the Kubernetes way. It provides the following
features:

- Deployment and management of cloud provider managed services using the
  Kubernetes API.
- Management and scheduling of configuration data across multiple Kubernetes
  clusters.
- Separation of concern between infrastructure owners, application owners, and
  developers.
- Infrastructure agnostic packaging of applications and their dependencies.
- Scheduling applications into different clusters, zones, and regions.

Crossplane does not:

- Require that you run your workloads on Kubernetes.
- Manage the data plane across Kubernetes clusters.
- Manage or provision non-hosted Kubernetes clusters.

Crossplane can be [installed] into any Kubernetes cluster, and is compatible
with any Kubernetes-native project. It manages external services by installing
[Custom Resource Definitions] (CRDs) and [reconciling] instances of those Custom
Resources. Crossplane is built to be extensible, meaning that anyone can add
functionality for an new or existing cloud provider.

Crossplane is comprised of four main components:

1. **Core Crossplane**: the set of Kubernetes CRDs and controllers that manage
   installation of `providers`, `stacks`, and `applications`, as well as the
   scheduling of configuration data to remote Kubernetes clusters.
2. **Providers**: the set of Kubernetes CRDs and controllers that provision and
   manage services on cloud providers. A cloud provider is any service that
   exposes infrastructure via an API.
    - Examples: [Google Cloud Platform], [Amazon Web Services], [Azure],
      [Alibaba], [Github]
3. **Stacks**: a bundled set of custom resources that together represent an
   environment on a cloud provider. The bundle of instances can be created by a
   single custom resource.
   - Examples: [Sample GCP Stack], [Sample AWS Stack], [Sample Azure Stack]
4. **Applications**: a deployable unit of code and configuration, which, when
   created, may involve provisioning new services which are managed by a
   `provider`, or consuming services created by a `stack`.
    - Examples: [Wordpress]

<!-- Named Links -->

[installed]: getting-started/install.md
[Custom Resource Definitions]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[reconciling]: https://kubernetes.io/docs/concepts/architecture/controller/
[Google Cloud Platform]: https://github.com/crossplane/provider-gcp
[Amazon Web Services]: https://github.com/crossplane/provider-aws
[Azure]: https://github.com/crossplane/provider-azure
[Alibaba]: https://github.com/crossplane/provider-alibaba
[Github]: https://github.com/crossplane/provider-github
[Sample GCP Stack]: https://github.com/crossplane/stack-gcp-sample
[Sample AWS Stack]: https://github.com/crossplane/stack-aws-sample
[Sample Azure Stack]: https://github.com/crossplane/stack-azure-sample
[Wordpress]: https://github.com/crossplane/app-wordpress


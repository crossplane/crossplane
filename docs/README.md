# Overview

![Crossplane](media/banner.png)

Crossplane is an open source Kubernetes add-on that extends any cluster with
the ability to provision and manage cloud infrastructure, services, and
applications using kubectl, GitOps, or any tool that works with the Kubernetes
API.

With Crossplane you can:

* **Provision & manage cloud infrastructure with kubectl**
  * [Install Crossplane] to provision and manage cloud infrastructure and
    services from any Kubernetes cluster.
  * Provision infrastructure primitives from any provider ([GCP], [AWS],
    [Azure], [Alibaba], on-prem) and use them alongside existing application
    configurations.
  * Version, manage, and deploy with your favorite tools and workflows that
    you’re using with your clusters today.

* **Publish custom infrastructure resources for your applications to use**
  * Define, compose, and publish your own [infrastructure resources] with
    declarative YAML, resulting in your own infrastructure CRDs being added to
    the Kubernetes API for applications to use.
  * Hide infrastructure complexity and include policy guardrails, so
    applications can easily and safely consume the infrastructure they need,
    using any tool that works with the Kubernetes API.
  * Consume infrastructure resources alongside any Kubernetes application to
    provision and manage the cloud services they need with Crossplane as an
    add-on to any Kubernetes cluster.

* **Deploy applications using a team-centric approach with OAM**
  * Define cloud native applications and the infrastructure they require with
    the Open Application Model ([OAM]).
  * Collaborate with a team-centric approach with a strong separation of
    concerns.
  * Deploy application configurations from app delivery pipelines or GitOps
    workflows, using the proven Kubernetes declarative model.

Separation of concerns is core to Crossplane’s approach to infrastructure and
application management, so team members can deliver value by focusing on what
they know best. Crossplane's team-centric approach reflects individuals often
specializing in the following roles:

* **Infrastructure Operators** - provide infrastructure and services for apps
    to consume
* **Application Developers** - build application components independent of
    infrastructure
* **Application Operators** - compose, deploy, and run application
    configurations

## Getting Started

[Install Crossplane] into any Kubernetes cluster to get started.

<!-- Named Links -->

[Install Crossplane]: getting-started/install-configure.md
[Custom Resource Definitions]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[reconciling]: https://kubernetes.io/docs/concepts/architecture/controller/
[GCP]: https://github.com/crossplane/provider-gcp
[AWS]: https://github.com/crossplane/provider-aws
[Azure]: https://github.com/crossplane/provider-azure
[Alibaba]: https://github.com/crossplane/provider-alibaba
[infrastructure resources]: https://blog.crossplane.io/crossplane-v0-10-compose-and-publish-your-own-infrastructure-crds-velero-backup-restore-compatibility-and-more/
[OAM]: https://github.com/oam-dev/spec/

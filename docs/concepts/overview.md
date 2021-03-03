---
title: Concepts
toc: true
weight: 100
---

# Overview

Crossplane introduces multiple building blocks that enable you to provision,
compose, and consume infrastructure using the Kubernetes API. These individual
concepts work together to allow for powerful separation of concern between
different personas in an organization, meaning that each member of a team
interacts with Crossplane at an appropriate level of abstraction.

## Packages

[Packages] allow Crossplane to be extended to include new functionality. This
typically looks like bundling a set of Kubernetes [CRDs] and [controllers] that
represent and manage external infrastructure (i.e. a provider), then installing
them into a cluster where Crossplane is running. Crossplane handles making sure
any new CRDs do not conflict with existing ones, as well as manages the RBAC and
security of new packages. Packages are not strictly required to be providers,
but it is the most common use-case for packages at this time.

## Providers

Providers are packages that enable Crossplane to provision infrastructure on an
external service. They bring CRDs (i.e. managed resources) that map one-to-one
to external infrastructure resources, as well as controllers to manage the
life-cycle of those resources. You can read more about providers, including how
to install and configure them, in the [providers documentation].

## Managed Resources

Managed resources are Kubernetes custom resources that represent infrastructure
primitives. Managed resources with an API version of `v1beta1` or higher support
every field that the cloud provider does for the given resource. You can find
the Managed Resources and their API specifications for each provider on
[doc.crds.dev] and learn more in the [managed resources documentation].

## Composite Resources

A composite resource (XR) is a special kind of custom resource that is defined
by a `CompositeResourceDefinition`. It composes one or more managed resources
into a higher level infrastructure unit. Composite resources are infrastructure
operator facing, but may optionally offer an application developer facing
composite resource claim that acts as a proxy for a composite resource. You can
learn more about all of these concepts in the [composition documentation].

<!-- Named Links -->

[Packages]: packages.md
[CRDs]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[controllers]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers
[providers documentation]: providers.md
[doc.crds.dev]: https://doc.crds.dev
[managed resources documentation]: managed-resources.md
[composition documentation]: composition.md

---
title: Overview
toc: true
weight: 400
indent: true
---

# Overview

Crossplane introduces multiple building blocks that enable you to provision,
publish, and consume infrastructure using the Kubernetes API. These individual
concepts work together to allow for powerful separation of concern between
different personas in an organization, meaning that each member of a team
interacts with Crossplane at an appropriate level of abstraction.

![Crossplane Concepts]

## Packages

Packages allow Crossplane to be extended to include new functionality. This
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
[doc.crds.dev]. 

## Composition

Composition refers the machinery that allows you to bundle managed resources
into higher-level infrastructure units, using only the Kubernetes API. New
infrastructure units are defined using the `InfrastructureDefinition`,
`InfrastructurePublication`, and `Composition` types, which result in the
creation of new CRDs in a cluster. Creating instances of these new CRDs result
in the creation of one or more managed resources. You can learn more about all
of these concepts in the [composition documentation].

## OAM

Crossplane supports application management as the Kubernetes implementation of
the [Open Application Model].

<!-- Named Links -->

[Crossplane Concepts]: crossplane-concepts.png
[CRDs]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[controllers]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers
[providers documentation]: providers.md
[doc.crds.dev]: https://doc.crds.dev
[composition documentation]: composition.md
[Open Application Model]: https://oam.dev/

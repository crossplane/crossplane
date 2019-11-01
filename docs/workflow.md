---
title: "Workflow"
toc: true
weight: 590
indent: false
---

# CI/CD Using Crossplane

Crossplane enables continuous integration and continuous delivery by integrating
with platforms such as [ArgoCD], [Jenkins], [Gitlab], and many more. The
separation of concern model allows for individuals and organizations to define
their infrastructure universe for different environments, then deploy
applications that make use of that infrastructure, all via a [GitOps] workflow.

These guides serve to demonstrate common scenarios where it is desirable to
manage all infrastructure and applications as code.

<!-- Named links -->
[ArgoCD]: https://argoproj.github.io/argo-cd/
[Jenkins]: https://jenkins.io/
[Gitlab]: https://about.gitlab.com/product/continuous-integration/
[GitOps]: https://www.weave.works/technologies/gitops/
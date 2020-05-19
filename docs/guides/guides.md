---
title: Guides
toc: true
weight: 200
---

# Guides

Because of Crossplane's standardization on the Kubernetes API, it integrates
well with many other projects. Below is a collection of guides and tutorials that
demonstrate how to use Crossplane with a variety tools and projects often used
with Kubernetes plus some deep dive content on Crossplane itself!

- [Argo CD] - use GitOps to provision managed services with Crossplane and Argo CD.
- [Knative] - use managed services provisioned by Crossplane in your Knative services.
- [Okteto] - use managed services in your Okteto development workflow.
- [Open Policy Agent] - set global policy on provisioning cloud resources with Crossplane and OPA.
- [OpenFaaS] - consume managed services with for your serverless functions.
- [Provider Internals] - translate provider APIs into managed resource CRDs and explore managed resource API patterns & best practices.
- [Velero] - backup and restore your Crossplane infrastructure resources.

<!-- Named Links -->

[Velero]: https://www.youtube.com/watch?v=eV_2QoMRqGw&list=PL510POnNVaaYFuK-B_SIUrpIonCtLVOzT&index=18&t=183s
[Argo CD]: https://aws.amazon.com/blogs/opensource/connecting-aws-managed-services-to-your-argo-cd-pipeline-with-open-source-crossplane/
[Open Policy Agent]: https://github.com/crossplane/tbs/tree/master/episodes/14
[Knative]: https://github.com/crossplane/tbs/tree/master/episodes/15
[OpenFaaS]: https://github.com/crossplane/tbs/tree/master/episodes/13
[Okteto]: https://github.com/crossplane/tbs/tree/master/episodes/10
[Provider Internals]: https://github.com/crossplane/tbs/tree/master/episodes/7
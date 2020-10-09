---
title: Experimental
toc: true
weight: 1100
indent: true
---

# Experimental 

## Deprecated: templating-controller and related 
The templating-controller has been deprecated in favor of composite
infrastructure and OAM.

The [templating-controller] allows you to create Crossplane packages without
writing any code using helm3 or kustomize yaml files and simple metadata.

The [Wordpress QuickStart Guide] provides an overview of using packages of
this type.

The namespace-scoped packages using the templating-controller are: 
- [app-wordpress]

The cluster-scoped packages using the templating-controller are: 
- [stack-gcp-sample] 
- [stack-aws-sample] 
- [stack-azure-sample]

Packages that use the [templating-controller] can be installed using
`PackageInstall` and `ClusterPackageInstall`, and CRDs provided by the
`Package` will be reconciled by the `templating-controller` which will apply
behavior (helm3, kustomize) to automatically generate the specified
resources.

See [packaging an app] to learn more.


<!-- Named Link -->
[templating-controller]: https://github.com/crossplane/templating-controller
[stack-gcp-sample]: https://github.com/crossplane/stack-gcp-sample
[stack-aws-sample]: https://github.com/crossplane/stack-aws-sample
[stack-azure-sample]: https://github.com/crossplane/stack-azure-sample
[app-wordpress]: https://github.com/crossplane/app-wordpress
[Wordpress Quickstart Guide]: https://github.com/crossplane/app-wordpress/blob/master/docs/quickstart.md
[packaging an app]: experimental/packaging_an_app.md

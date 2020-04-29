---
title: API Documentation
toc: true
weight: 400
---

# API Documentation

The Crossplane ecosystem contains many CRDs that map to API types represented by
external infrastructure providers. The documentation for these CRDs are
auto-generated on [doc.crds.dev]. To find the CRDs available for providers
maintained by the Crossplane organization, you can search for the Github URL, or
append it in the [doc.crds.dev] URL path.

For instance, to find the CRDs available for [provider-azure], you would go to:

[doc.crds.dev/github.com/crossplane/provider/azure]

By default, you will be served the latest CRDs on the `master` branch for the
repository. If you prefer to see the CRDs for a specific version, you can append
the git tag for the release:

[doc.crds.dev/github.com/crossplane/provider-azure@v0.8.0]

Crossplane repositories that are not providers but do publish CRDs are also
served on [doc.crds.dev]. For instance, the [crossplane/crossplane] repository.

Bugs and feature requests for API documentation should be [opened as issues] on
the open source [doc.crds.dev repo].

<!-- Named Links -->

[doc.crds.dev]: https://doc.crds.dev/
[provider-azure]: https://github.com/crossplane/provider-azure
[doc.crds.dev/github.com/crossplane/provider/azure]: https://doc.crds.dev/github.com/crossplane/provider-azure
[doc.crds.dev/github.com/crossplane/provider-azure@v0.8.0]: https://doc.crds.dev/github.com/crossplane/provider-azure@v0.8.0
[crossplane/crossplane]: https://doc.crds.dev/github.com/crossplane/crossplane
[opened as issues]: https://github.com/crdsdev/doc/issues/new
[doc.crds.dev repo]: https://github.com/crdsdev/doc

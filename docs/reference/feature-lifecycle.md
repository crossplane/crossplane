---
title: Feature Lifecycle
toc: true
weight: 309
indent: true
---

# Feature Lifecycle

Crossplane follows a similar feature lifecycle to [upstream
Kubernetes][kube-features]. All major new features must be added in alpha. Alpha
features are expected to eventually graduate to beta, and then to general
availability (GA). Features that languish at alpha or beta may be subject to
deprecation.

## Alpha Features

Alpha are off by default, and must be enabled by a feature flag, for example
`--enable-composition-revisions`. API types pertaining to alpha features use a
`vNalphaN` style API version, like `v1alpha`. **Alpha features are subject to
removal or breaking changes without notice**, and generally not considered ready
for use in production. 

In some cases alpha features require fields be added to existing beta or GA
API types. In these cases fields must clearly be marked (i.e in their OpenAPI
schema) as alpha and subject to alpha API constraints (or lack thereof).

All alpha features should have an issue tracking their graduation to beta.

## Beta Features

Beta features are on by default, but may be disabled by a feature flag. API
types pertaining to beta features use a `vNbetaN` style API version, like
`v1beta1`. Beta features are considered to be well tested, and will not be
removed completely without being marked deprecated for at least two releases.

The schema and/or semantics of objects may change in incompatible ways in a
subsequent beta or stable release. When this happens, we will provide
instructions for migrating to the next version. This may require deleting,
editing, and re-creating API objects. The editing process may require some
thought. This may require downtime for applications that rely on the feature.

In some cases beta features require fields be added to existing GA API types. In
these cases fields must clearly be marked (i.e in their OpenAPI schema) as beta
and subject to beta API constraints (or lack thereof).

All beta features should have an issue tracking their graduation to GA.

## GA Features

GA features are always enabled - they cannot be disabled. API types pertaining
to GA features use `vN` style API versions, like `v1`. GA features are widely
used and thoroughly tested. They guarantee API stability - only backward
compatible changes are allowed.

[kube-features]: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-stages
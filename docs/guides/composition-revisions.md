---
title: Composition Revisions
toc: true
weight: 260
indent: true
---

# Composition Revisions

This guide discusses the use of "Composition Revisions" to safely make and roll
back changes to a Crossplane [`Composition`][composition-type]. It assumes
familiarity with Crossplane, and particularly with
[Composition][composition-term].

> Composition Revisions are an __alpha feature__. They are not yet recommended
> for production use, and are disabled by default.

A `Composition` configures how Crossplane should reconcile a Composite Resource
(XR). Put otherwise, when you create an XR the selected `Composition` determines
what managed resources Crossplane will create in response. Let's say for example
that you define a `PlatformDB` XR, which represents your organisation's common
database configuration of an Azure MySQL Server and a few firewall rules. The
`Composition` contains the 'base' configuration for the MySQL server and the
firewall rules that is extended by the configuration for the `PlatformDB`.

There is a one-to-many relationship between a `Composition` and the XRs that use
it. You might define a `Composition` named `big-platform-db` that is used by ten
different `PlatformDB` XRs. Usually, in the interest of self-service, the
`Composition` is managed by a different team from the actual `PlatformDB` XRs.
For example the `Composition` may be written and maintained by a platform team
member, while individual application teams create `PlatformDB` XRs that use said
`Composition`.

Each `Composition` is mutable - you can update it as your organisation's needs
change. However, without Composition Revisions updating a `Composition` can be a
risky process. Crossplane constantly uses the `Composition` to ensure that your
actual infrastructure - your MySQL Servers and firewall rules - match your
desired state. If you have 10 `PlatformDB` XRs all using the `big-platform-db`
`Composition`, all 10 of those XRs will be instantly updated in accordance with
any updates you make to the `big-platform-db` `Composition`.

Composition Revisions allow XRs to opt out of automatic updates. Instead you can
update your XRs to leverage the latest `Composition` settings at your own pace.
This enables you to [canary] changes to your infrastructure, or to roll back
some XRs to previous `Composition` settings without rolling back all XRs.

## Enabling Composition Revisions

Composition Revisions are an alpha feature. They are not yet recommended for
production use, and are disabled by default. Start Crossplane with the
`--enable-composition-revisions` flag to enable Composition Revision support.

```console
kubectl create namespace crossplane-system
helm install crossplane --namespace crossplane-system crossplane-stable/crossplane --set args='{--enable-composition-revisions}'
```

See the [getting started guide][install-guide] for more information on
installing Crossplane.

## Using Composition Revisions

When you enable Composition Revisions three things happen:

1. Crossplane creates a `CompositionRevision` for each `Composition` update.
1. Composite Resources gain a `spec.compositionRevisionRef` field that specifies
   which `CompositionRevision` they use.
1. Composite Resources gain a `spec.compositionUpdatePolicy` field that
   specifies how they should be updated to new Composition Revisions.

Each time you edit a `Composition` Crossplane will automatically create a
`CompositionRevision` that represents that 'revision' of the `Composition` -
that unique state. Each revision is allocated an increasing revision number.
This `CompositionRevision` consumers an idea about which revision is 'newest'.

Crossplane distinguishes between the 'newest' and the 'current' revision of a
`Composition`. That is, if you revert a `Composition` to a previous state that
corresponds to an existing `CompositionRevision` that revision will become
'current' even if it is not the 'newest' revision (i.e. the most latest _unique_
`Composition` configuration).

You can discover which revisions exist using `kubectl`:

```console
# Find all revisions of the Composition named 'example'
kubectl get compositionrevision -l crossplane.io/composition-name=example
```

This should produce output something like:

```console
NAME            REVISION   CURRENT   AGE
example-18pdg   1          False     4m36s
example-2bgdr   2          True      73s
example-xjrdm   3          False     61s
```

> A `Composition` is a mutable resource that you can update as your needs
> change over time. Each `CompositionRevision` is an immutable snapshot of those
> needs at a particular point in time.

Crossplane behaves the same way by default whether Composition Revisions are
enabled or not. This is because when you enable Composition Revisions all XRs
default to the `Automatic` `compositionUpdatePolicy`. XRs support two update
policies:

* `Automatic`: Automatically use the current `CompositionRevision`. (Default)
* `Manual`: Require manual intervention to change `CompositionRevision`.

The below XR uses the `Manual` policy. When this policy is used the XR will
select the current `CompositionRevision` when it is first created, but must
manually be updated when you wish it to use another `CompositionRevision`.

```yaml
apiVersion: example.org/v1alpha1
kind: PlatformDB
metadata:
  name: example
spec:
  parameters:
    storageGB: 20
  # The Manual policy specifies that you do not want this XR to update to the
  # current CompositionRevision automatically.
  compositionUpdatePolicy: Manual
  compositionRef:
    name: example
  writeConnectionSecretToRef:
    name: db-conn
```

Crossplane sets an XR's `compositionRevisionRef` automatically at creation time
regardless of your chosen `compositionUpdatePolicy`. If you choose the `Manual`
policy you must edit the `compositionRevisionRef` field when you want your XR to
use a different `CompositionRevision`.

```yaml
apiVersion: example.org/v1alpha1
kind: PlatformDB
metadata:
  name: example
spec:
  parameters:
    storageGB: 20
  compositionUpdatePolicy: Manual
  compositionRef:
    name: example
  # Update the referenced CompositionRevision if and when you are ready.
  compositionRevisionRef:
    name: example-18pdg
  writeConnectionSecretToRef:
    name: db-conn
```

[composition-type]: ../concepts/composition.md
[composition-term]: ../concepts/terminology.md#composition
[canary]: https://martinfowler.com/bliki/CanaryRelease.html
[install-guide]: ../getting-started/install-configure.md
---
title: Composition Revisions
weight: 100
---

This guide discusses the use of "Composition Revisions" to safely make and roll
back changes to a Crossplane [`Composition`][composition-type]. It assumes
familiarity with Crossplane, and particularly with
[Composition][composition-term].

> Composition Revisions are a __beta feature__.

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


## Using Composition Revisions

When Composition Revisions are enabled three things happen:

1. Crossplane creates a `CompositionRevision` for each `Composition` update.
1. Composite Resources gain a `spec.compositionRevisionRef` field that specifies
   which `CompositionRevision` they use.
1. Composite Resources gain a `spec.compositionUpdatePolicy` field that
   specifies how they should be updated to new Composition Revisions.

Each time you edit a `Composition` Crossplane will automatically create a
`CompositionRevision` that represents that 'revision' of the `Composition` -
that unique state. Each revision is allocated an increasing revision number.
This gives `CompositionRevision` consumers an idea about which revision is
'newest'.

You can discover which revisions exist using `kubectl`:

```console
# Find all revisions of the Composition named 'example'
kubectl get compositionrevision -l crossplane.io/composition-name=example
```

This should produce output something like:

```console
NAME            REVISION   AGE
example-18pdgs2   1          4m36s
example-2bgdr31   2          73s
example-xjrdmzz   3          61s
```

> A `Composition` is a mutable resource that you can update as your needs
> change over time. Each `CompositionRevision` is an immutable snapshot of those
> needs at a particular point in time.

Crossplane behaves the same way by default whether Composition Revisions are
enabled or not. This is because when you enable Composition Revisions all XRs
default to the `Automatic` `compositionUpdatePolicy`. XRs support two update
policies:

* `Automatic`: Automatically use the latest `CompositionRevision`. (Default)
* `Manual`: Require manual intervention to change `CompositionRevision`.

The below XR uses the `Manual` policy. When this policy is used the XR will
select the latest `CompositionRevision` when it is first created, but must
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
  # latest CompositionRevision automatically.
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

To implement channel-based deployments, you can use `compositionUpdatePolicy: Automatic`
and `compositionRevisionSelector` fields. Each time you create or update a `Composition`, its labels
will be propagated to the `CompositionRevision` that is created. You can use `matchLabels` in the
composites to select the channel you want to listen and always use the latest `CompositionRevision` 
that matches the selector.

```yaml
apiVersion: example.org/v1alpha1
kind: PlatformDB
metadata:
   name: example
spec:
  parameters:
    storageGB: 20
  compositionUpdatePolicy: Automatic
  compositionRevisionSelector:
     matchLabels:
       channel: prod
   writeConnectionSecretToRef:
     name: db-conn
```

[composition-type]: {{<ref "../concepts/composition" >}}
[composition-term]: {{<ref "../concepts/terminology" >}}#composition
[canary]: https://martinfowler.com/bliki/CanaryRelease.html
[install-guide]: {{<ref "../getting-started/install-configure" >}}

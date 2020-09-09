# Crossplane RBAC Manager

* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Crossplane, as a project, consists of three key building blocks:

1. A provider is a controller manager that can orchestrate an external system
   (like a cloud provider). Each external system primitive is represented as a
   'managed resource' - a kind of custom resource.
1. The API extensions controllers allow Crossplane to compose managed resources
   into higher level 'composite resources'.
1. The package manager controllers allow Crossplane to fetch and install a
   provider or a configuration of composition from a package - an OCI image
   containing metadata and custom resources.

Each provider is a distinct process that is typically deployed as a pod. The API
extensions and package manager controllers are part of the 'core' Crossplane
controller manager process. The core controller manager is therefore responsible
for _extending Crossplane_. Its controllers add and and remove Custom Resource
Definitions (CRDs) to and from the API server. The core Crossplane controllers
define custom resources (CRs) that represent:

* Provider configuration.
* The managed resources a provider may orchestrate within an external system.
* Composite resources and composite resource claims.

These controllers are novel in that they define custom resources and start
controllers in response to the creation of other custom resources. Creating a
`Provider` custom resource, for example, would cause the package manager to
deploy a new provider controller manager, and to define the custom resources
that it can reconcile.

When Crossplane defines new custom resources some entities (or subjects, in
Kubernetes RBAC terminology) must be granted access to those resources in order
for them to be useful. These subjects can be categorised as software, or humans.

Software, in this context, corresponds to the service accounts that Crossplane
or a provider run as. When the package manager reconciles a `Provider`, it runs
the provider pod as a certain service account. The provider cannot reconcile its
managed resources unless its service account is granted access to do so.
Furthermore, the core Crossplane controllers cannot create the provider's
managed resources, and thus cannot compose them into a higher level resource.

Humans are the people who use Crossplane to orchestrate external systems.
Crossplane typically categorises these people into three roles, which may
overlap:

* Platform builders define composite resources and compositions. They may
  package these composite resources and compositions as configurations.
* Platform operators install providers, install configurations, and manage
  composite resources. They may also interact with the managed resources of
  which a composite resource is composed.
* Platform consumers claim composite resources. The platform builder specifies
  which composite resources are offered to platform consumers.

These people are unable to configure and use Crossplane to orchestrate the
systems they wish to orchestrate unless they (i.e. their RBAC user or group) are
granted access to perform the tasks they need to perform. Furthermore, no one is
able to grant this access because the API server requires that a subject be
granted a particular RBAC role before they are able to grant that role to
another subject.

In either case - software or human - it is possible for a superuser to manually
author the appropriate RBAC roles and bind them to the appropriate subjects to
make Crossplane work. A superuser in this context is either a subject bound to
the built in `cluster-admin` RBAC `ClusterRole`, or a subject bound to a role
that grants the `escalate` verb on RBAC `Roles` and/or `ClusterRoles`. Manually
authoring and binding RBAC roles is the most flexible and secure approach, but
it is onerous and often boilerplate. Kubernetes addresses this by offering
`admin`, `edit`, and `view` RBAC `ClusterRoles` that when bound grant varying
levels of broad access to the majority of the resources an API server supports.

## Goals

The goal of this document is to allow Crossplane to dynamically manage (and in
some cases bind) RBAC roles that grant subjects access to use Crossplane as it
is extended by installing providers and defining composite resources.

Crossplane should be able to create and bind RBAC roles that grant:

* A provider's service account the access it needs to reconcile the managed
  resources it defines.
* Crossplane's service account access to compose the managed resources a
  provider defines.
* Crossplane's service account access to reconcile any defined composite
  resources and composite resource claims.

Crossplane should also create (but not bind) RBAC roles that grant:

* Platform operators access to interact with the managed resources a provider
  defines.
* Platform operators access to interact with the composite resources and
  composite resource claims they define.
* Platform consumers access to interact with the set of composite resource
  claims their platform operator(s) deem appropriate.

Finally, it should be possible to opt-out of RBAC role management. This allows
Crossplane to be run without any special privileges (i.e. without the ability to
escalate its own privileges) at the expense of deferring RBAC role management to
platform operators.

## Proposal

This document proposes that Crossplane introduce an 'RBAC manager'. The RBAC
manager would run as a distinct pod, and would be responsible for creating and
binding the RBAC roles Crossplane and its users need. This allows Crossplane and
its providers run as service accounts that are limited to a specific set of
resources and verbs (e.g. create, watch, etc). The RBAC manager would also run
as a service account limited to a specific set of resources and verbs, but would
be granted the special `escalate` and `bind` on the `Role` and `ClusterRole`
resources, thus allowing it to grant access that it does not have.

Privilege escalation is an unfortunate by-product of a controller manager that
may dynamically define new custom resources. Centralising the ability to
escalate privileges into the RBAC manager has two nice properties:

* The platform operator can choose not to deploy the RBAC manager, and can
  instead manually curate RBAC roles - e.g. to manually create and bind RBAC
  roles for each provider and composite resource.
* The attack surface required to exploit Crossplane is reduced. Fewer Crossplane
  controllers run with the ability to escalate privileges.

### Managed RBAC ClusterRoles

The RBAC manager will be responsible for the management of several cluster
roles, using [`ClusterRole` aggregation][Aggregated ClusterRoles] to form a set
of cluster roles that may change over time as providers and composite resources
are added to and removed from the API server. The RBAC manager would manage the
following cluster roles. Note that these roles are aggregated - they consist of
several more granular roles that are not covered here. **The following examples
are 'flattened' aggregations represented as a single fixed `ClusterRole` to
demonstrate specifically what permissions each role will have.**

The Crossplane `ClusterRole` is bound to the service account the Crossplane pod
runs as. It grants the following rules:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane
rules:
# Crossplane creates events to report its status.
- apiGroups: [""]
  resources: [events]
  verbs: [create]
# Crossplane manages the secrets of its composite resources. It never explicitly
# deletes secrets, but instead relies on garbage collection.
- apiGroups: [""]
  resources: [secrets]
  verbs: [get, create, update]
# Crossplane creates CRDs in response to the creation of XRDs.
- apiGroups: [apiextensions.k8s.io]
  resources: [customresourcedefinitions]
  verbs: [get, create, update, delete]
# Crossplane has full access to the types it defines.
- apiGroups:
  - apiextensions.crossplane.io
  - pkg.crossplane.io
  resources: ["*"]
  verbs: ["*"]
# Crossplane has full access to the composite resources and claims it defines.
- apiGroups: [xr.example.org]
  resources:
  - examplecomposites
  - examplecomposites/status
  - exampleclaims
  - exampleclaims/status
  verbs: ["*"]
# Crossplane has access to the provider resource that it may need to compose.
- apiGroups: [provider.example.org]
  resources:
  - examplemanageds
  - exampleproviderconfigs
  verbs: ["*"]
# Crossplane uses jobs to unpack packages.
- apiGroups: [batch, extensions]
  resources: [jobs]
  verbs: [get, create, update, delete]
# Crossplane reads pod logs to unpack packages.
- apiGroups: [""]
  resources: [pods]
  verbs: [get, list]
- apiGroups: [""]
  resources: [pods/log]
  verbs: [get]
# Crossplane uses deployments to run providers.
- apiGroups: [extensions, apps]
  resources: [deployments]
  verbs: [get, create, update, delete]
```

A Crossplane provider's `ClusterRole` is bound to the service account the
provider pod runs as. It grants the following rules:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: example-provider
rules:
# Providers create events to report their status.
- apiGroups: [""]
  resources: [events]
  verbs: [create]
# Providers read their credentials from secrets. They may also create and update
# secrets containing managed resource connection details.
- apiGroups: [""]
  resources: [secrets]
  verbs: [get, create, update]
# Providers have access to the resources they define.
- apiGroups: [provider.example.org]
  resources:
  - examplemanageds
  - examplemanageds/status
  - exampleproviderconfigs
  - exampleproviderconfigs/status
  verbs: [get, list, watch, update, patch]
```

Crossplane's human-facing cluster roles are inspired by the [user-facing roles]
of Kubernetes - `admin`, `edit`, and `view`. Note that Kubernetes distinguishes
the `cluster-admin` cluster role, which is intended to be granted at cluster
scope via a cluster role binding, from the `admin`, `edit`, and `view` cluster
roles, which are intended to be granted within a particular namespace using a
role binding. Crossplane does not make this distinction. The `crossplane-admin`,
`crossplane-edit`, and `crossplane-view` roles are intended to be granted at the
cluster scope, and thus grant admin, edit, or view access at the cluster scope.
Crossplane provides distinct edit and view roles for each namespace.

The `crossplane-admin` role is automatically bound to the `crossplane:masters`
group for convenience. It grants the following rules:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-admin
rules:
# Crossplane administrators have access to view events.
- apiGroups: [""]
  resources: [events]
  verbs: [get, list, watch]
# Crossplane administrators must create provider credential secrets, and may
# need to read or otherwise interact with connection secrets. They may also need
# to create or annotate namespaces.
- apiGroups: [""]
  resources: [secrets, namespaces]
  verbs: ["*"]
# Crossplane administrators have access to view the cluster roles that they may
# be able to grant to other subjects.
- apiGroups: [rbac.authorization.k8s.io]
  resources: [clusterroles]
  verbs: [get, list, watch]
# Crossplane administrators have access to grant the access they have to other
# subjects.
- apiGroups: [rbac.authorization.k8s.io]
  resources: [clusterrolebindings, rolebindings]
  verbs: [*]
# Crossplane administrators have full access to built in Crossplane types.
- apiGroups:
  - apiextensions.crossplane.io
  - pkg.crossplane.io
  resources: ["*"]
  verbs: ["*"]
# Crossplane administrators have full access to all of the composite resources
# and claims it defines.
- apiGroups: [xr.example.org]
  resources:
  - examplecomposites
  - examplecomposites/status
  - exampleclaims
  - exampleclaims/status
  verbs: ["*"]
# Crossplane administrators have full access to all resources defined by
# installed providers.
- apiGroups: [provider.example.org]
  resources:
  - examplemanageds
  - exampleproviderconfigs
  verbs: ["*"]
```

The `crossplane-edit` role is identical to `crossplane-admin`, sans the ability
to grant others access by managing role bindings.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-edit
rules:
# Crossplane editors have access to view events.
- apiGroups: [""]
  resources: [events]
  verbs: [get, list, watch]
# Crossplane editors must create provider credential secrets, and may need to
# read or otherwise interact with connection secrets.
- apiGroups: [""]
  resources: [secrets]
  verbs: ["*"]
# Crossplane editors may see which namespaces exist, but not edit them.
- apiGroups: [""]
  resources: [namespaces]
  verbs: [get, list, watch]
# Crossplane editors have full access to built in Crossplane types.
- apiGroups:
  - apiextensions.crossplane.io
  - pkg.crossplane.io
  resources: ["*"]
  verbs: ["*"]
# Crossplane editors have full access to all of the composite resources and
# claims it defines.
- apiGroups: [xr.example.org]
  resources:
  - examplecomposites
  - examplecomposites/status
  - exampleclaims
  - exampleclaims/status
  verbs: ["*"]
# Crossplane viewers have full access to all resources defined by installed
# providers.
- apiGroups: [provider.example.org]
  resources:
  - examplemanageds
  - exampleproviderconfigs
  verbs: ["*"]
```

The `crossplane-view` role allows read-only access to all Crossplane resources.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-view
rules:
# Crossplane viewers have access to view events.
- apiGroups: [""]
  resources: [events]
  verbs: [get, list, watch]
# Crossplane viewers may see which namespaces exist.
- apiGroups: [""]
  resources: [namespaces]
  verbs: [get, list, watch]
# Crossplane viewers have read-only access to built in Crossplane types.
- apiGroups:
  - apiextensions.crossplane.io
  - pkg.crossplane.io
  resources: ["*"]
  verbs: [get, list, watch]
# Crossplane viewers have read-only access to all of the composite resources and
# claims it defines.
- apiGroups: [xr.example.org]
  resources:
  - examplecomposites
  - examplecomposites/status
  - exampleclaims
  - exampleclaims/status
  verbs: [get, list, watch]
# Crossplane viewers have read-only access to all resources defined by installed
# providers.
- apiGroups: [provider.example.org]
  resources:
  - examplemanageds
  - exampleproviderconfigs
  verbs: [get, list, watch]
```

The RBAC manager creates `edit` and `view` cluster roles for each namespace.
These cluster roles are 'namespace aligned' - they are intended to grant access
to a particular namespace via a `RoleBinding` - but not namespace scoped. This
is because (while these examples are flattened into single roles) they use role
aggregation, which is not supported by namespaced RBAC roles.

Maintaining an `edit` and a `view` role for each namespace allows the specific
access that these roles grant to vary from namespace to namespace. A platform
operator who has been granted the `crossplane-admin` role may influence which
resource claims are available to a particular namespaces by annotating the
namespace (see [Composite Resource ClusterRole Mechanics] for details).

> Note that there is no namespace aligned `admin` role. It may be desirable for
> a subject bound to the `admin` role in a particular namespace to be able to
> bind other subjects to that role in that namespace, but it is not possible to
> enforce this. Kubernetes RBAC allows a subject with access to create role
> bindings to bind any role that grants less or equal access than they have, and
> thus a subject bound to the `crossplane-ns-example-admin` cluster role in the
> `example` namespace could bind another subject the `crossplane-ns-other-admin`
> cluster role in the `example` namespace as long as binding the latter role was
> not a privilege escalation.

The `crossplane-ns-${namespace}-edit` role grants full access to the resource
claims that are available to said namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  # This role is intended for use with the 'example' namespace.
  name: crossplane-ns-example-edit
rules:
# Crossplane namespace editors have access to view events.
- apiGroups: [""]
  resources: [events]
  verbs: [get, list, watch]
# Crossplane namespace editors may need to read or otherwise interact with
# resource claim connection secrets.
- apiGroups: [""]
  resources: [secrets]
  verbs: ["*"]
# Crossplane editors have read-only access to composite resource definitions and
# compositions. This allows them to discover and select an appropriate
# composition when creating a resource claim.
- apiGroups: [apiextensions.crossplane.io]
  resources: ["*"]
  verbs: [get, list, watch]
# Crossplane editors have full access to all of the composite resource claims
# that an admin has chosen to enable in their namespace.
- apiGroups: [xr.example.org]
  resources:
  - exampleclaims
  - exampleclaims/status
  verbs: ["*"]
```

The `crossplane-ns-${namespace}-view` role grants read-only access to the
resource claims that are available to said namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  # This role is intended for use with the 'example' namespace.
  name: crossplane-ns-example-view
rules:
# Crossplane namespace viewers have access to view events.
- apiGroups: [""]
  resources: [events]
  verbs: [get, list, watch]
# Crossplane namespace viewers have read-only access to all of the composite
# resource claims that an admin has chosen to enable in their namespace.
- apiGroups: [xr.example.org]
  resources:
  - exampleclaims
  - exampleclaims/status
  verbs: [get, list, watch]
```

### ClusterRole Aggregation Mechanics

Each of [the RBAC roles that Crossplane manages][Managed RBAC ClusterRoles] is
in fact an aggregation of several cluster roles. Typically each role will be an
aggregation of a 'base' role - a fixed set of rules - and zero or more roles
that are created by the RBAC manager in response to the installation of a
provider, or the definition of a composite resource. The `crossplane-admin` role
might be an aggregation of the following roles:

* `crossplane:aggregate-to-admin`
* `crossplane:provider:7f63e3661:aggregate-to-edit`
* `crossplane:composite:examplecomposites.xr.example.org:aggregate-to-edit`

For example the below `crossplane:aggregate-to-admin` role aggregates to the
user-facing `crossplane-admin` role.

```yaml
---
# The 'user-facing' crossplane-admin role.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-admin
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.crossplane.io/aggregate-to-crossplane-admin: "true"
---
# The 'base' role containing the fixed rules that always aggregate to the above
# crossplane-admin cluster role.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane:aggregate-to-admin
  labels:
    rbac.crossplane.io/aggregate-to-crossplane-admin: "true"
rules:
- apiGroups: [""]
  resources: [events]
  verbs: [get, list, watch]
- apiGroups: [""]
  resources: [secrets, namespaces]
  verbs: ["*"]
- apiGroups: [rbac.authorization.k8s.io]
  resources: [clusterroles]
  verbs: [get, list, watch]
- apiGroups: [rbac.authorization.k8s.io]
  resources: [clusterrolebindings, rolebindings]
  verbs: [*]
- apiGroups:
  - apiextensions.crossplane.io
  - pkg.crossplane.io
  resources: ["*"]
  verbs: ["*"]
```

The cluster roles intended to be bound at the cluster scope (e.g.
`crossplane-admin`) and their base roles (e.g. `crossplane:aggregate-to-admin`)
can be created outside of Crossplane - typically by Crossplane's Helm chart. All
other roles that aggregate to `crossplane-admin` are created as required by the
RBAC manager. Cluster roles intended to be bound at the namespace scope (e.g.
`crossplane-ns-example-admin`) on the other hand must be created by the RBAC
manager, though their base roles can be created outside of Crossplane.

The naming convention established by this design is that user-facing roles use
hyphen separated names, while system facing and aggregated roles use colon
user-facing role. There is only one `crossplane:aggregate-to-admin` base role
and only separated names. In the above example the base role is one-to-one with
the one `crossplane-admin` user-facing role. In other cases one base role is
shared by many user-facing roles - for example many user-facing namespace roles
share one base role:

```yaml
---
# The 'user-facing' edit role for the 'example' namespace.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-ns-example-edit
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.crossplane.io/aggregate-to-ns-edit: "true"
      rbac.crossplane.io/base-of-ns-edit: "true"
---
# The 'base' role containing the fixed rules that always aggregate to all
# namespace aligned edit cluster roles.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane:aggregate-to-ns-edit
  labels:
    rbac.crossplane.io/aggregate-to-ns-edit: "true"
    rbac.crossplane.io/base-of-ns-edit: "true"
rules:
- apiGroups: [""]
  resources: [events]
  verbs: [get, list, watch]
- apiGroups: [""]
  resources: [secrets]
  verbs: ["*"]
- apiGroups: [apiextensions.crossplane.io]
  resources: ["*"]
  verbs: [get, list, watch]
```

Note that aggregation may be transitive. For example the `crossplane-admin` role
is identical to the `crossplane-edit` role except for a few extra rules in its
base role. Provider and composite resource roles can therefore aggregate only to
`crossplane-edit`, which in turn aggregates to `crossplane-admin`.

### Composite Resource ClusterRole Mechanics

When a composite resource is defined the RBAC manager will create two cluster
roles. For example creating an XRD named `composites.example.org` would trigger
the creation of the following roles:

* `crossplane:composite:composites.example.org:aggregate-to-edit`
* `crossplane:composite:composites.example.org:aggregate-to-view`

The `aggregate-to-edit` role aggregates to the `crossplane`, `crossplane-admin`,
and `crossplane-edit` roles, as well as any namespace-aligned `edit` role for a
namespace in which the composite resource may claimed. The `aggregate-to-view`
cluster role aggregates to the `crossplane-view` cluster role, and any `view`
role for a namespace in which the composite resource may be claimed.

Composite resources may be claimed in any namespace that has an annotation with
the key `rbac.crossplane.io/composites.example.org`, and the value `xrd-enabled`
(where `composites.example.org` is the name of the XRD). When the RBAC manager
encounters a namespace with one or more such annotations, it creates `edit` and
`view` cluster roles for that namespace.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: example
  annotations:
    rbac.crossplane.io/composites.example.org: xrd-enabled
    rbac.crossplane.io/examplecomposites.xr.example.org: xrd-enabled
```

The above namespace, for example, will result in the creation of the following
cluster role. Note that the latter two role selectors are derived from the
annotations of the namespace.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane-ns-example-edit
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.crossplane.io/aggregate-to-ns-edit: "true"
      rbac.crossplane.io/base-of-ns-edit: "true"
  - matchLabels:
      rbac.crossplane.io/aggregate-to-ns-edit: "true"
      rbac.crossplane.io/xrd: composites.example.org
  - matchLabels:
      rbac.crossplane.io/aggregate-to-ns-edit: "true"
      rbac.crossplane.io/xrd: examplecomposites.xr.example.org
```

Below is an example of an `edit` composite resource cluster role.

```yaml
# This composite resource role aggregates up to any namespace aligned cluster
# role that declares compatibility by including the XRD label. The same role
# also aggregates to the crossplane-edit role (and thus transitively to the
# crossplane-admin role).
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane:composite:examplecomposites.xr.example.org:aggregate-to-edit
  labels:
    rbac.crossplane.io/aggregate-to-edit: "true"
    rbac.crossplane.io/aggregate-to-ns-edit: "true"
    rbac.crossplane.io/xrd: examplecomposites.xr.example.org
rules:
- apiGroups: [xr.example.org]
  resources:
  - exampleclaims
  - exampleclaims/status
  # Composite resources are cluster scoped. Binding this cluster role at the
  # namespace scope will not actually grant access to these composite resources.
  # Including them in this cluster role allows one crossplane:composite role to
  # aggregate to both the crossplane-ns-example-edit and crossplane-edit roles.
  - examplecomposites
  - examplecomposites/status
  verbs: ["*"]
```

### Provider ClusterRole Mechanics

When a provider is installed (via the package manager) the RBAC manager will
define three cluster roles:

* `crossplane:provider:7f63e3661:aggregate-to-edit`
* `crossplane:provider:7f63e3661:aggregate-to-view`
* `crossplane:provider:7f63e3661:system`

Note that these role names are derived from the active `ProviderRevision`. In
this example `7f63e3661` is an abbreviated hash. The `aggregate-to-edit` role
aggregates to the `crossplane`, `crossplane-admin` and `crossplane-edit` roles,
while the `aggregate-to-view` role aggregates to the `crossplane-view` role. The
`system` role does not aggregate; it is granted by the RBAC manager to the
service account under which the provider pod runs.

To create a cluster role for a `Provider` (package), the RBAC manager:

1. Finds its active `ProviderRevision`.
1. Lists all CRDs that are controlled by that `ProviderRevision`.
1. Creates a `ClusterRole` that grants access to the defined CRs.

In the case of the `system` role the RBAC manager creates a `ClusterRoleBinding`
to the `ServiceAccount` under which the provider runs. This service account is
also controlled by the aforementioned `ProviderRevision`, and can be discovered
similarly.

Below is an example of the `system` provider role and its role binding.

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane:provider:7f63e3661:system
rules:
- apiGroups: [provider.example.org]
  resources:
  - examplemanageds
  - examplemanageds/status
  - exampleproviderconfigs
  - exampleproviderconfigs/status
  verbs: [get, list, watch, update, patch]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: crossplane:provider:7f63e3661:system
roleRef:
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRole
  name: crossplane:provider:7f63e3661:system
subjects:
- kind: ServiceAccount
  name: crossplane-provider-7f63e3661
  namespace: crossplane-system
```

Below is an example of an `edit` provider role. Note that in practice the role
would likely have many more rules - one for each custom resource defined by the
provider.

```yaml
---
# This role aggregates to the crossplane and crossplane-edit roles. It thus
# aggregates transitively to the crossplane-admin role via crossplane-edit.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: crossplane:provider:7f63e3661:aggregate-to-edit
  labels:
    rbac.crossplane.io/aggregate-to-crossplane: "true"
    rbac.crossplane.io/aggregate-to-edit: "true"
rules:
- apiGroups: [provider.example.org]
  resources:
  - examplemanageds
  - exampleproviderconfigs
  verbs: [*]
```

### Opting out of RBAC Management

Building and deploying the RBAC manager as a distinct process allows Crossplane
users to completely opt-out of RBAC management. Users may choose not to deploy
the RBAC manager without compromising any core Crossplane functionality. In
addition to completely disabling the RBAC manager it will be possible to limit
the scope of the RBAC manager via a flag. The following levels will exist:

* `--manage=all` - Include all functionality described in this design.
* `--manage=serviceaccounts` - Manage only the 'software' roles - i.e. the
  `crossplane` role and the roles for each provider.
* `--manage=basic` - Manage the software roles plus the `crossplane-admin`,
  `crossplane-edit`, and `crossplane-view` roles.

## Alternatives Considered

The following alternatives were considered before arriving at this design.

### Package Driven RBAC

Crossplane currently relies on the package manager to manage its RBAC roles.
This has become less desirable primarily because it's now possible to extend
Crossplane (i.e. define new APIs) using the API extensions controllers. While
many composite resources will be defined via a `Configuration` package it's
equally possible that they will be defined directly.

### Fixed Admin, Edit, and View Roles

Kubernetes provides three cluster roles; `admin`, `edit`, and `view` that are
intended to be bound to a subject at the namespace scope using a `RoleBinding`.
This allows a subject to be "an administrator of a namespace" or "a viewer of a
namespace" similar to the design put forward by this namespace. The limitation
of this approach is that "administrator" or "viewer" must mean the same thing to
every namespace; it is not possible for the administrator role for a namespace
to grant different access to that namespace from the administrator role of
another namespace.

### Configurable RBAC Controller

This design proposes that the RBAC manager react to the definition of composite
resources and the installation of providers by creating and aggregating
opinionated RBAC roles. It may be possible to extend this design by making the
RBAC manager rule driven; allowing it to be configured by a new set of custom
resources to create and aggregate arbitrary RBAC rules when new Crossplane types
(or even arbitrary types) are created. Crossplane is choosing to be opinionated
about its RBAC roles at this time, but may explore this path in the future if
opinionated RBAC roles prove to be insufficient.

[Aggregated ClusterRoles]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles
[user-facing roles]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles
[Composite Resource ClusterRole Mechanics]: #composite-resource-clusterrole-mechanics
[Managed RBAC ClusterRoles]: #managed-rbac-clusterroles

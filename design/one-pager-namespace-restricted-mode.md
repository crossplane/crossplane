# Namespace-restricted mode for Crossplane Core

* Owner: François Rigaut (@frigaut-orange)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

The core Crossplane pod currently requires rights on Services, ServiceAccounts, 
Deployments, and Secrets. If the pod does not have these permissions, it will
not start properly. Either it will crash, or it will run without being able to
reconcile any resource. This is due to the core's manager's cache watching
resources in all namespaces by default.

Additionnaly, Crossplane will attempt to create events on cluster-scoped resources
in the `default` namespace. If it does not have the rights to do so, logs can be
polluted by error messages.

The crossplane ServiceAccount having this level of permissions may represent a 
security issue. It may cause trouble for teams who manage a Crossplane instance
on a cluster, without having full admin rights on it. This is why this proposal 
introduces a new option to enable compatibility between Crossplane and permissions
restricted only to its own namespace.

## Goals

The objective is for Crossplane to work properly with permissions on:

- Cluster-scoped resources **only** for Crossplane custom resources
- Namespaced-scoped kubernetes-native resources **only** in Crossplane's own 
  namespace
- Namespaced-scoped Crossplane custom resources in all namespaces (crossplane v2)

If Crossplane is able to run with these minimal permissions, it enables user-side
(i.e. outside of Crossplane) fine-tuning of the ClusterRole, in order to better 
control which resources Crossplane can manipulate or not, and in which namespaces.

## Minimal set of cluster permissions

Here is the minimal desired set of cluster-wide permissions for crossplane in
namespace-restricted mode:

```yaml
# ClusterRole
rules:
- apiGroups:
  - apiextensions.crossplane.io
  - pkg.crossplane.io
  - secrets.crossplane.io
  resources:
  - "*"
  verbs:
  - "*"
# This could even be optional depending on the desired level of control
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  - customresourcedefinitions/status
  verbs:
  - "*"
```

Assuming crossplane has admin rights on all resources in its own namespace (or at
least rights that are currently defined in the ClusterRole, except in a Role instead)

Note: Additional permissions would be required **after startup**: Managed Resources, XRs...
This proposal only focuses on making Crossplane launch (and run) properly, so
assumptions will not be made on how these ulterior permissions are managed by the user
(or by Crossplane).

## Proposal

To enable compatibility between Crossplane and these minimal permissions, this
document proposes a new `--namespace-restricted` option for the core crossplane
command. This option would, for now, have 2 consequences :

1. Make the previously mentionned cache watch resources in Crossplane's namespace 
   instead of the whole cluster.
1. Disable the creation of events on cluster-wide resources, since it requires
   permissions on the `default` namespace.

The Helm Chart should be adapted accordingly. A `namespace_restricted` variable
should, when set to `true`:

1. Start the core controller with the `--namespace-restricted` arg.
1. Disable the RBAC manager pod, since the objective is to manage permissions 
   specific to an ecosystem (i.e. outside of Crossplane).
1. Make the `crossplane` ClusterRole have minimal rights, and create a Role instead
   with everything needed for Crossplane to work.

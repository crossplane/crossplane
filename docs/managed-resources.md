---
title: Managed Resources
toc: true
weight: 400
indent: true
---

# Managed Resources

## Overview

Managed resources are Crossplane representation of the external provider resources
and they are considered primitive low level custom resources that are used as
building blocks in other abstractions such as your application or a layer of composition.

For example, `RDSInstance` in AWS Provider corresponds to an actual RDS Instance
in AWS. There is a one-to-one relationship and the changes on managed resources are
reflected directly on the corresponding resource in the provider.

## Syntax

Crossplane has some API conventions that are built on top of Kubernetes API
conventions for how the schema of the managed resources should look like. Following
is an example of `RDSInstance`:

```yaml
apiVersion: database.aws.crossplane.io/v1beta1
kind: RDSInstance
metadata:
  name: foodb
spec:
  forProvider:
    dbInstanceClass: db.t2.small
    masterUsername: root
    allocatedStorage: 20
    engine: mysql
  writeConnectionSecretToRef:
      name: mysql-secret
      namespace: crossplane-system
  providerRef:
    name: mycreds
  reclaimPolicy: Delete
```

In Kubernetes, `spec` top field represents the desired state of the user.
Crossplane adheres to that and has its own conventions about how the fields under
`spec` should look like.

* `writeConnectionSecretToRef`: A reference to the secret that you want this
  managed resource to write its connection secret that you'd be able to mount
  to your pods in the same namespace. For `RDSInstance`, this secret would contain
  `endpoint`, `username` and `password`.
  
* `providerRef`: Reference to the `Provider` resource that will provide information
  regarding authentication of Crossplane to the provider. `Provider` resources
  refer to `Secret` and potentially contain other information regarding authentication.

* `reclaimPolicy`: Enum to specify whether the actual cloud resource should be
  deleted when this managed resource is deleted in Kubernetes API server. Possible
  values are `Delete` and `Retain`.
  
* `forProvider`: While the rest of the fields relate to how Crossplane should
  behave, the fields under `forProvider` are solely used to configure the actual
  external resource. In most of the cases, the field names correspond to the what
  exists in provider's API Reference.
  
  The objects under `forProvider` field can get huge depending on the provider
  API. For example, GCP `ServiceAccount` has only a few fields while GCP `CloudSQLInstance`
  has over 100 fields that you can configure.

## Behavior

As a general rule, managed resource controllers try not to make any decision
that is not specified by the user in the desired state since managed resources
are the lowest level components of your infrastructure that you can build other
abstractions for doing magic.

### Continuous Reconciliation

Crossplane providers continuously reconcile the managed resource to achieve the
desired state. The parameters under `spec` is considered one and only source of
truth for the external resource. This means that if someone changed a configuration
in the UI of the provider, Crossplane will change it back to what's given under
`spec`.

#### Immutable Properties

There are configuration parameters in external resources that provider does not
allow to be changed. If the corresponding field in the managed resource is changed
by the user, Crossplane submits the new desired state to the provider and returns
the error, if any. For example, in AWS, you cannot change the region of
an `RDSInstance`.

Some infrastructure tools such as Terraform deletes and recreates the resource
to accommodate those changes but Crossplane does not take that route. Unless
the managed resource is deleted and its `reclaimPolicy` is `Delete`, its controller
never deletes the external resource in the provider.

> Immutable fields are marked as `immutable` in Crossplane codebase
but Kubernetes does not yet have immutable field notation in CRDs.

### External Name

Crossplane has a special annotation to represent the name of the external resource
as it appears in the provider. Users might want to opt in to use that external
name annotation if they would like their external resource to be named other than
the name of the managed resource. For example, I would like to have an `CloudSQLInstance`
with an external name that is different than its managed resource name:

```yaml
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstance
metadata:
  name: foodb
  annotations:
    crossplane.io/external-name: my-special-db
spec:
  ...
```

When you create this managed resource, you will see that the name of `CloudSQLInstance`
in GCP console will be `my-special-db`.

If the annotation is not give, Crossplane will fill it with the name of the managed
resource by default. In cases where provider doesn't allow you to name the resource,
like AWS VPC, the controller creates the resource and sets external annotation
to be the name that provider chose. So, you would see something like `vpc-28dsnh3`
in the annotation of your AWS VPC no matter you added your own custom name or not.


### Late Initialization

For some of the optional fields, users rely on the default that provider chooses
for them. Since Crossplane treats the managed resource as the source of the truth,
values of those fields need to exist in `spec` of the managed resource. So, in each
reconciliation, Crossplane will fill the value of a field that is left empty by
the user but is assigned a value by the provider. For example, there could be
two fields like `region` and `availabilityZone` and you might only want to give
`region`. In that case, if provider assigns an availability zone, Crossplane gets
that value and fills `availabilityZone` if it's not given already. Note that if
the field is already filled, the controller won't override its value.

### Deletion

When a deletion request is made for a managed resource, its controller starts the
deletion process immediately. However, it blocks the managed resource disappearence
via a finalizer so that the managed resource disappears only when the controller
confirms that the external resource is gone. This way, controller ensures that
if any error happens during deletion and resource is still there, you can still
see and manage it via its managed resource.

## Dependencies

In many cases, an external resource refers to another one for a specific configuration.
For example, you could want your Azure Kubernetes cluster in a specific
Virtual Network. External resources have specific fields for these relations, however,
they usually require the information to be supplied in different formats. In Azure
MySQL, you might be required to enter only the name of the Virtual Network while in
Azure Kubernetes, it could be required to enter a string in a specific format
that includes other information such as resource group name.

In Crossplane, users have 3 fields to refer to another resource. Here is an example
from Azure MySQL managed resource referring to a Azure Resource Group:

```yaml
spec:
  forProvider:
    resourceGroupName: foo-res-group
    resourceGroupNameRef:
      name: resourcegroup
    resourceGroupNameSelector:
      matchLabels:
        app: prod
```

In this example, the user provided only a set of labels to select a `ResourceGroup`
managed resource that already exists in the cluster via `resourceGroupNameSelector`.
Then after a specific `ResourceGroup` is selected, `resourceGroupNameRef` is filled
with the name of that `ResourceGroup` managed resource. Then in the last step,
Crossplane fills the actual `resourceGroupName` field with whatever format Azure
accepts it.

Users are able to give the input in all three levels: selector to select via labels,
reference to point to a determined managed resource and actual value that will
be submitted to the provider.

It's important to note that in case a reference exists, the managed resource does
not create the external resource until the referenced object is ready. In this
example, creation call of Azure MySQL Server will not be made until referenced
`ResourceGroup` has its condition named `Ready` to be true.

## Importing Existing Resources

If you have some resources that are already provisioned in the provider, you can
import them as managed resources and let Crossplane manage them. What you need to
do is to enter the name of the external resource as well as the required fields
on the managed resource. For example, let's say I have a GCP Network provisioned
from GCP console and I would like to migrate it to Crossplane. Here is the YAML
that I need to create:

```yaml
apiVersion: compute.gcp.crossplane.io/v1beta1
kind: Network
metadata:
  name: foo-network
  annotation:
    crossplane.io/external-name: existing-network
spec:
  providerRef:
    name: gcp-creds
```

Crossplane will check whether a GCP Network called `existing-network` exists, and
if it does, then the optional fields under `forProvider` will be filled with the
values that are fetched from the provider.

Note that if a resource has required fields, you will need to fill those fields, too,
because otherwise creation of the managed resource will be rejected. So, in those cases,
you will need to enter the name of the resource as well as values of those required
fields.

## Backup and Restore

Crossplane adheres to Kubernetes conventions as much as possible and one of the
advantages we gain is backup & restore ability with tools that work with native
Kubernetes types.

If you'd like to backup and restore manually, you can simply export them and save YAMLs
in your file system. When you reload them, as we've discovered in import section,
their external nem annotation and requried fields are there and those are enough to
import a resource. The tool you're using needs to store `annotations` and `spec`
fields, which most tools do including Velero.
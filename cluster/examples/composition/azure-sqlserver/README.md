# azure-sqlserver

This example demonstrates Crossplane's nascent support for [composite
infrastructure resources]. It defines two new kind of resource, both in the
`azure.example.org` API group:

* `NetworkGroup` - An Azure virtual network, subnet, and resource group in which
  other resources may be created.
* `PrivatePostgreSQLServer` - An Azure PostgreSQL server that is accessible only
  from a particular `NetworkGroup` (e.g. the subnet of that `NetworkGroup`.).

The `NetworkGroup` is defined but not published. Defined resources are cluster
scoped and intended for use only by infrastructure operators. Any defined
resource may subsequently be published as available for application operators to
request. An application operator may request a `PrivatePostgreSQLServer` to use
with their application by authoring a `PrivatePostgreSQLServerRequirement` in
the namespace where their application is running.

To try this example, first install provider-azure and create an Azure `Provider`
named `example` per the [Crossplane documentation], then:

1. Define and publish the new resources by running `kubectl apply -f
   definitions/`.
1. Create a `NetworkGroup` by running `kubectl apply -f networkgroup.yaml`.
1. Create a `PrivatePostgreSQLServerRequirement` by running `kubectl apply -f
   privatepostgresqlserver.yaml`.

Steps one and two would typically be performed by an infrastructure operator who
wanted to allow their application operators to create private PostgreSQL
servers, while step three would be performed by an application operator who
wanted to request a private PostgreSQL server for their application.

Use `kubectl describe` to view the status of your newly created resources:

```shell
# Make sure the new resources were correctly defined.
$ kubectl describe infrastructuredefinitions

# Make sure the PrivatePostgreSQLServerRequirement was correctly published.
$ kubectl describe infrastructurepublications

# Make sure the NetworkGroup is working:
$ kubectl describe networkgroups

# Make sure the PrivatePostgreSQLServerRequirement is working
$ kubectl describe privatepostgresqlserverrequirementes
```

[composite infrastructure resources]: https://github.com/crossplane/crossplane/blob/f0263cd/design/design-doc-composition.md
[Crossplane documentation]: https://crossplane.io/docs/

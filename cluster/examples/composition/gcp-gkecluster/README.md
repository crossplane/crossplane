# gcp-gkecluster

This example demonstrates Crossplane's nascent support for [composite
infrastructure resources]. It defines two new kind of resource, both in the
`gke.example.org` API group:

* `ServicedNetwork` - A GCP VPC network with service networking enabled,
  allowing instances in this network to privately access GCP services such as
  CloudSQL.
* `Cluster` - A GKE cluster with two node pools.

Both resources are defined but not published. Defined resources are cluster
scoped and intended for use only by infrastructure operators. Any defined
resource may subsequently be published as available for application operators to
request - see the neighboring gcp-sqlserver for an example of a published
resource.

To try this example, first install provider-gcp and create an GCP `Provider`
named `example` per the [Crossplane documentation], then:

1. Define and publish the new resources by running `kubectl apply -f
   definitions/`.
1. Create a `ServicedNetwork` by running `kubectl apply -f
   servicednetwork.yaml`.
1. Create a `Cluster` by running `kubectl apply -f cluster.yaml`.

Use `kubectl describe` to view the status of your newly created resources:

```shell
# Make sure the new resources were correctly defined.
$ kubectl describe infrastructuredefinitions

# Make sure the ServicedNetwork is working:
$ kubectl describe servicednetworks

# Make sure the Cluster is working
$ kubectl describe clusters
```

[composite infrastructure resources]: https://github.com/crossplane/crossplane/blob/f0263cd/design/design-doc-composition.md
[Crossplane documentation]: https://crossplane.io/docs/

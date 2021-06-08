---
title: Compose Infrastructure
toc: true
weight: 4
indent: true
---

# Compose Infrastructure

In the [last section] we learned that Crossplane can be extended by installing
_providers_, which add support for managed resources. A managed resource is a
Kubernetes custom resource that offers a high fidelity representation of an
infrastructure primitive, like an SQL instance or a firewall rule. Crossplane
goes beyond simply modelling infrastructure primitives as custom resources - it
enables you to define new custom resources with schemas of your choosing. These
resources are composed of managed resources, allowing you to define and offer
resources that group and abstract infrastructure primitives. We call these
"composite resources" (XRs).

XRs are always cluster scoped - they exist outside of any namespace. This allows
an XR to represent infrastructure that might be consumed from several different
namespaces. This is often true for VPC networks - an infrastructure operator may
wish to define a VPC network XR and an SQL instance XR, only the latter of which
may be managed by application operators. The application operators are
restricted to their team's namespace, but their SQL instances should all be
attached to the VPC network that the infrastructure operator manages. Crossplane
enables scenarios like this  by allowing the infrastructure operator to offer
their application operators a _composite resource claim_ (XRC). An XRC is a
namespaced proxy for an XR; the schema of an XRC is identical to that of its
corresponding XR. When an application operator creates an XRC, a corresponding
backing XR is created automatically.

We use two special Crossplane resources to define and configure new XRs and
XRCs:

- A `CompositeResourceDefinition` (XRD) _defines_ a new kind of composite
  resource, including its schema. An XRD may optionally _offer_ a claim.
- A `Composition` specifies which resources a composite resource will be
  composed of, and how they should be configured. You can create multiple
  `Composition` options for each composite resource.

XRDs and Compositions may be packaged and installed as a _configuration_. A
configuration is a [package] of composition configuration that can easily be
installed to Crossplane by creating a declarative `Configuration` resource, or
by using `kubectl crossplane install configuration`. In the examples below we
will install a configuration that defines a new `CompositePostgreSQLInstance` XR
that takes a single `storageGB` parameter, and creates a connection `Secret`
with keys for `username`, `password`, and `endpoint`. A `Configuration` exists
for each provider that can satisfy a `PostgreSQLInstance`. Let's get started!

## Configure and Claim Your Infrastructure

We will now install a `Configuration` that:

- Defines a `CompositePostgreSQLInstance` XR.
- Offers a `PostgreSQLInstance` claim (XRC) for said XR.
- Creates a `Composition` that can satisfy our XR.

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-1" data-toggle="tab">AWS (Default VPC)</a></li>
<li><a href="#aws-new-tab-1" data-toggle="tab">AWS (New VPC)</a></li>
<li><a href="#gcp-tab-1" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-1" data-toggle="tab">Azure</a></li>
<li><a href="#alibaba-tab-1" data-toggle="tab">Alibaba</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="aws-tab-1" markdown="1">

> Note that this configuration will create an RDS instance using your default
> VPC, which may or may not allow connections from the internet depending on how
> it is configured. Select the AWS (New VPC) configuration if you wish to create
> an RDS instance that will allow traffic from the internet.

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-aws:v1.0.1
```

</div>
<div class="tab-pane fade" id="aws-new-tab-1" markdown="1">

> Note that this configuration for AWS also includes several networking managed
> resources that are required to provision a publicly available PostgreSQL
> instance. Composition enables scenarios such as this, as well as far more
> complex ones. See the [composition] documentation for more information.

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-aws-with-vpc:v1.0.1
```

</div>
<div class="tab-pane fade" id="gcp-tab-1" markdown="1">

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-gcp:v1.0.1
```

</div>
<div class="tab-pane fade" id="azure-tab-1" markdown="1">

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-azure:v1.0.1
```

</div>
<div class="tab-pane fade" id="alibaba-tab-1" markdown="1">

```console
kubectl crossplane install configuration registry.upbound.io/xp/getting-started-with-alibaba:v1.0.1
```

</div>
</div>

Crossplane should now be configured to allow us to create a `PostgreSQLInstance`
claim! You can watch the installation progress of the `Configuration` object by
running:

```console
kubectl get configuration --watch
```

Make sure you've [setup] a `ProviderConfig` named `default` for your chosen
provider, then create your XRC to provision a PostgreSQL instance and all the
supporting infrastructure (VPCs, firewall rules, resource groups, etc) that it
may need!

<ul class="nav nav-tabs">
<li class="active"><a href="#aws-tab-2" data-toggle="tab">AWS (Default VPC)</a></li>
<li><a href="#aws-new-tab-2" data-toggle="tab">AWS (New VPC)</a></li>
<li><a href="#gcp-tab-2" data-toggle="tab">GCP</a></li>
<li><a href="#azure-tab-2" data-toggle="tab">Azure</a></li>
<li><a href="#alibaba-tab-2" data-toggle="tab">Alibaba</a></li>
</ul>
<br>
<div class="tab-content">
<div class="tab-pane fade in active" id="aws-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: aws
      vpc: default
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.0/docs/snippets/compose/claim-aws.yaml
```

</div>
<div class="tab-pane fade" id="aws-new-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: aws
      vpc: new
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.0/docs/snippets/compose/claim-aws.yaml
```

</div>
<div class="tab-pane fade" id="gcp-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: gcp
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.0/docs/snippets/compose/claim-gcp.yaml
```

</div>
<div class="tab-pane fade" id="azure-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: azure
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.0/docs/snippets/compose/claim-azure.yaml
```

</div>
<div class="tab-pane fade" id="alibaba-tab-2" markdown="1">

```yaml
apiVersion: database.example.org/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: my-db
  namespace: default
spec:
  parameters:
    storageGB: 20
  compositionSelector:
    matchLabels:
      provider: alibaba
  writeConnectionSecretToRef:
    name: db-conn
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.0/docs/snippets/compose/claim-alibaba.yaml
```

</div>
</div>

After creating the `PostgreSQLInstance` Crossplane will begin provisioning a
database instance on your provider of choice. Once provisioning is complete, you
should see `READY: True` in the output when you run:

```console
kubectl get postgresqlinstance my-db
```

> Note: while waiting for the `PostgreSQLInstance` to become ready, you
> may want to look at other resources in your cluster. The following commands
> will allow you to view groups of Crossplane resources:
>
> - `kubectl get claim`: get all resources of all claim kinds, like `PostgreSQLInstance`.
> - `kubectl get composite`: get all resources that are of composite kind, like `CompositePostgreSQLInstance`.
> - `kubectl get managed`: get all resources that represent a unit of external
>   infrastructure.
> - `kubectl get <name-of-provider>`: get all resources related to `<provider>`.
> - `kubectl get crossplane`: get all resources related to Crossplane.

Try the following command to watch your provisioned resources become ready:

```console
kubectl get crossplane -l crossplane.io/claim-name=my-db
```

Once your `PostgreSQLInstance` is ready, you should see a `Secret` in the `default`
namespace named `db-conn` that contains keys that we defined in XRD. If they were
filled by the composition, then they should appear:

```console
$ kubectl describe secrets db-conn
Name:         db-conn
Namespace:    default
...

Type:  connection.crossplane.io/v1alpha1

Data
====
password:  27 bytes
port:      4 bytes
username:  25 bytes
endpoint:  45 bytes
```

## Consume Your Infrastructure

Because connection secrets are written as a Kubernetes `Secret` they can easily
be consumed by Kubernetes primitives. The most basic building block in
Kubernetes is the `Pod`. Let's define a `Pod` that will show that we are able to
connect to our newly provisioned database.

> Note that if you're using a hosted Crossplane you'll need to copy the db-conn
> connection secret over to your own Kubernetes cluster and run this pod there.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: see-db
  namespace: default
spec:
  containers:
  - name: see-db
    image: postgres:9.6
    command: ['psql']
    args: ['-c', 'SELECT current_database();']
    env:
    - name: PGDATABASE
      value: postgres
    - name: PGHOST
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: endpoint
    - name: PGUSER
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: username
    - name: PGPASSWORD
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: password
    - name: PGPORT
      valueFrom:
        secretKeyRef:
          name: db-conn
          key: port
```

```console
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/release-1.0/docs/snippets/compose/pod.yaml
```

This `Pod` simply connects to a PostgreSQL database and prints its name, so you
should see the following output (or similar) after creating it if you run
`kubectl logs see-db`:

```SQL
 current_database
------------------
 postgres
(1 row)
```

## Clean Up

To clean up the `Pod`, run:

```console
kubectl delete pod see-db
```

To clean up the infrastructure that was provisioned, you can delete the
`PostgreSQLInstance` XRC:

```console
kubectl delete postgresqlinstance my-db
```

> Don't clean up your `Configuration` just yet if you plan to continue on to the
> next section of the guide! We'll use them again when we deploy an OAM
> application.

## Next Steps

Now you have seen how to provision and publish more complex infrastructure
setups. In the [next section] you will learn how to consume infrastructure
alongside your [OAM] application manifests.

<!-- Named Links -->

[last section]: provision-infrastructure.md
[composition]: ../introduction/composition.md
[package]: ../introduction/packages.md
[setup]: install-configure.md
[next section]: run-applications.md
[OAM]: https://oam.dev/

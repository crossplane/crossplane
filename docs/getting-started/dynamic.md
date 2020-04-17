---
title: Dynamic Provisioning
toc: true
weight: 4
indent: true
---

# Dynamic Provisioning

While someone in your organization needs to be familiar with the lowest level of
infrastructure, many people just want a simple workflow for acquiring and using
infrastructure. Crossplane provides the ability for organizations to define a
catalog of infrastructure and applications, then for teams and individuals to
consume from the catalog with application-focused requests. This is made
possible by *dynamic provisioning*.

## Dynamically Provision a Redis Cluster on GCP

In the [previous example], we created a `CloudMemorystoreInstance`
then claimed it directly with a `RedisCluster` claim. With dynamic provisioning,
we can instead create a *resource class* then request request creation of a
managed resource (and subsequent creation of its external implementation) using
a claim.

> **Resource Class**: an instance of a cluster-scoped CRD that represents
> configuration for an external unit of infrastructure. The fields of a resource
> class CRD map 1-to-1 with the fields exposed by the provider's API, but
> creating a resource class instance *does not* result in immediate creation of
> the external unit. The CRDs that represent resource classes on a provider are
> installed with it.

Create a file named `cloud-memorystore-class.yaml` with the following content:

```yaml
apiVersion: cache.gcp.crossplane.io/v1beta1
kind: CloudMemorystoreInstanceClass
metadata:
  name: cms-class
  labels:
    guide: quickstart
specTemplate:
  providerRef:
    name: gcp-provider
  writeConnectionSecretsToNamespace: crossplane-system
  reclaimPolicy: Delete
  forProvider:
    tier: STANDARD_HA
    region: us-west2
    memorySizeGb: 1
```

> *Note: similar to a managed resource, there is no namespace defined on our
> configuration for the `CloudMemorystoreInstanceClass` above because resource
> classes are
> [cluster-scoped].*

You will notice that this looks very similar to the
`CloudMemorystoreInstanceClass` we created in the previous example. It has the
same configuration for the external Cloud Memorystore instance, but you will
notice that it contains a `specTemplate` instead of a `spec`. The `specTemplate`
will be used later to create a `CloudMemorystoreInstance` that has the same
configuration as the one in the static provisioning example.

Create the `CloudMemorystoreInstanceClass`:

```
kubectl apply -f cloud-memorystore-class.yaml
```

There is nothing to observe yet, we have published this configuration for a
Cloud Memorystore instance for later use. To actually create our
`CloudMemorystoreInstance`, we must create a `RedisCluster` claim that
references the `CloudMemorystoreInstanceClass`. A claim can reference a class in
two ways:

1. **Class Reference**: this is most similar to the way we referenced the
   managed resource in the previous example. Instead of a `resourceRef` we can
   provide a `classRef`:

```yaml
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  name: redis-claim-dynamic
  namespace: cp-quickstart
spec:
  classRef:
    apiVersion: cache.gcp.crossplane.io/v1beta1
    kind: CloudMemorystoreInstanceClass
    name: cms-class
  writeConnectionSecretToRef:
    name: redis-connection-details-dynamic
```

2. **Class Selector**: this is a more general way to reference a class, and also
   requires less knowledge of the underlying implementation by the claim
   creator. You will notice that the `CloudMemorystoreInstanceClass` we created
   above includes the `guide: quickstart` label. If we include that label in a
   selector on the `RedisCluster` claim, the claim will be scheduled to a class
   that has that label.

> *Note: if multiple classes have a label that is included in the claim's
> selector, one will be chosen at random.*

```yaml
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  name: redis-claim-dynamic
  namespace: cp-quickstart
spec:
  classSelector:
    matchLabels:
      guide: quickstart
  writeConnectionSecretToRef:
    name: redis-connection-details-dynamic
```

Using a label selector means that the `RedisCluster` claim creator is not
concerned whether the claim is satisfied by a GCP
`CloudMemorystoreInstanceClass`, an AWS `ReplicationGroupClass`, an Azure
`RedisClass`, or other. It allows them to select an implementation based on its
traits, rather than its provider. Selecting by label is frequently used to
choose between a set of classes from the same provider, each with with different
configuration. For instance, we could create three different
`CloudMemorystoreInstanceClass` with `memorySizeGb: 1`, `memorySizeGb: 5`,
`memorySizeGb: 10`, and label them `storage: small`, `storage: medium`, and
`storage: large`. Depending on our application requirements, we can then provide
a label selector that will find an appropriate implementation. Each of these
implementations will result in a Redis cluster being created and sufficient
connection details being propagated to the namespace of the claim.

Create a file name `redis-cluster-dynamic.yaml` and add the content the label
selector example above. Then, create the claim:

```
kubectl apply -f redis-cluster-dynamic.yaml
```

Because the `CloudMemorystoreInstanceClass` is the only Redis-compatible class
with the `guide: quickstart` label in our cluster, it is guaranteed to be used.
If you take a look at the `RedisCluster` claim, you should see that the
`example-cloudmemorystore-class` is being used and a `CloudMemorystoreInstance`
has been created with the same configuration:

```
kubectl get -f redis-cluster-dynamic.yaml
```

```
NAME                  STATUS   CLASS-KIND                      CLASS-NAME   RESOURCE-KIND              RESOURCE-NAME                             AGE
redis-claim-dynamic            CloudMemorystoreInstanceClass   cms-class    CloudMemorystoreInstance   cp-quickstart-redis-claim-dynamic-vp4dv   4s
```

You can also view the status of the `CloudMemorystoreInstance` as its external
resource is being created:

```
kubectl get cloudmemorystoreinstances
```

```
NAME                                      STATUS   STATE      CLASS       VERSION     AGE
cp-quickstart-redis-claim-dynamic-vp4dv            CREATING   cms-class   REDIS_4_0   25s
```

Once the `CloudMemorystoreInstance` becomes ready and bound, we have found
ourselves in the same situation as the conclusion of our static provisioning.
However, we got here with separating the responsibilities of defining and
consuming infrastructure, and we can create as many `RedisCluster` claims as we
want with the reusable configuration defined in our
`CloudMemorystoreInstanceClass`.

Continue to the [next section] to learn how Crossplane can
schedule workloads to remote Kubernetes clusters!

## Clean Up

If you would like to clean up the resources created in this section, run the
following command:

```
kubectl delete -f redis-cluster-dynamic.yaml
```

<!-- Named Links -->

[previous example]: static.md
[cluster-scoped]: https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#create-a-customresourcedefinition
[next section]: workload.md

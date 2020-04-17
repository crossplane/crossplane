---
title: Static Provisioning
toc: true
weight: 3
indent: true
---

# Static Provisioning

Crossplane supports provisioning resources *statically* and *dynamically*.
Static provisioning is the traditional manner of creating new infrastructure
using an infrastructure as code tool. To statically provision a resource, you
provide all necessary configuration and Crossplane simply takes your
configuration and submits it to the cloud provider.

Where Crossplane differs from an infrastructure as code tool is that it
continues to manage your resources after they are created. Let's take a look at
a simple example. We will use GCP for this quick start, but you can achieve the
same functionality of any of the providers mentioned in the [installation] and
[configuration] sections. You should have your provider of choice installed and
should have created a `Provider` resource with the necessary credentials. We
will use a [GCP `Provider`] resource with name `gcp-provider` below.

## Statically Provision a Redis Cluster on GCP

GCP provides Redis clusters using [Cloud Memorystore]. The GCP Crossplane
provider installs a `CloudMemorystoreInstance` [CustomResourceDefinition] (CRD)
which makes the API type available in your Kubernetes cluster. Creating an
instance of this CRD will result in the creation of a corresponding Cloud
Memorystore instance on GCP. CRDs like `CloudMemorystoreInstance` are referred
to as **Managed Resources** in Crossplane.

> **Managed Resource**: a cluster-scoped custom resource that represents an
> external unit of infrastructure. The fields of a managed resource CRD map
> 1-to-1 with the fields exposed by the provider's API, and creation of a
> managed resource result in immediate creation of external unit. The CRDs that
> represent managed resources on a provider are installed with it.

The fields available on the `CloudMemorystoreInstance` CRD match the ones
exposed by GCP, so you can configure it however you see fit.

Create a file named `cloud-memorystore.yaml` with the following content:

```yaml
apiVersion: cache.gcp.crossplane.io/v1beta1
kind: CloudMemorystoreInstance
metadata:
  name: example-cloudmemorystore-instance
spec:
  providerRef:
    name: gcp-provider
  writeConnectionSecretToRef:
    name: example-cloudmemorystore-connection-details
    namespace: crossplane-system
  reclaimPolicy: Delete
  forProvider:
    tier: STANDARD_HA
    region: us-west2
    memorySizeGb: 1
```

> *Note: there is no namespace defined on our configuration for the
> `CloudMemorystoreInstance` above because it is [cluster-scoped].*

Now, create a `CloudMemorystoreInstance` in your cluster with the following
command:

```
kubectl apply -f cloud-memorystore.yaml
```

The GCP provider controllers will see the creation of this
`CloudMemorystoreInstance` and subsequently create it on GCP. You can log in to
the GCP console to view the the status of the resource, but Crossplane will also
propagate the status back to the `CloudMemorystore` object itself. This allows
you to manage your infrastructure without ever leaving `kubectl`.

```
kubectl describe -f cloud-memorystore.yaml
```

```
Name:         example-cloudmemorystore-instance
Namespace:    
Labels:       <none>
Annotations:  crossplane.io/external-name: example-cloudmemorystore-instance
              kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"cache.gcp.crossplane.io/v1beta1","kind":"CloudMemorystoreInstance","metadata":{"annotations":{},"name":"example-cloudmemory...
API Version:  cache.gcp.crossplane.io/v1beta1
Kind:         CloudMemorystoreInstance
Metadata:
  Creation Timestamp:  2020-03-23T19:28:14Z
  Finalizers:
    finalizer.managedresource.crossplane.io
  Generation:        2
  Resource Version:  1476
  Self Link:         /apis/cache.gcp.crossplane.io/v1beta1/cloudmemorystoreinstances/example-cloudmemorystore-instance
  UID:               68be2036-4716-4c82-be5c-7923f1f8d6b1
Spec:
  For Provider:
    Alternative Location Id:  us-west2-a
    Authorized Network:       projects/crossplane-playground/global/networks/default
    Location Id:              us-west2-b
    Memory Size Gb:           1
    Redis Version:            REDIS_4_0
    Region:                   us-west2
    Tier:                     STANDARD_HA
  Provider Ref:
    Name:          gcp-provider
  Reclaim Policy:  Delete
  Write Connection Secret To Ref:
    Name:       example-cloudmemorystore-connection-details
    Namespace:  crossplane-system
Status:
  At Provider:
    Create Time:               2020-03-23T19:28:16Z
    Name:                      projects/crossplane-playground/locations/us-west2/instances/example-cloudmemorystore-instance
    Persistence Iam Identity:  serviceAccount:651413264395-compute@developer.gserviceaccount.com
    Port:                      6379
    State:                     CREATING
  Conditions:
    Last Transition Time:  2020-03-23T19:28:14Z
    Reason:                Successfully resolved resource references to other resources
    Status:                True
    Type:                  ReferencesResolved
    Last Transition Time:  2020-03-23T19:28:14Z
    Reason:                Resource is being created
    Status:                False
    Type:                  Ready
    Last Transition Time:  2020-03-23T19:28:17Z
    Reason:                Successfully reconciled resource
    Status:                True
    Type:                  Synced
Events:
  Type    Reason                   Age   From                                                      Message
  ----    ------                   ----  ----                                                      -------
  Normal  CreatedExternalResource  14s   managed/cloudmemorystoreinstance.cache.gcp.crossplane.io  Successfully requested creation of external resource
```

When the resource is done provisioning on GCP, you should see the `State` turn
from `CREATING` to `READY`, and a corresponding event will be emitted. At this
point, Crossplane will create a `Secret` that contains any connection
information for the external resource. The `Secret` is created in the location
we specified in our configuration:

```yaml
writeConnectionSecretToRef:
    name: example-cloudmemorystore-connection-details
    namespace: crossplane-system
```

It will take some time to provision, but once the `CloudMemorystoreInstance` is
ready, take a look at the contents of the `Secret`:

```
kubectl -n crossplane-system describe secret example-cloudmemorystore-connection-details
```

```
Name:         example-cloudmemorystore-connection-details
Namespace:    crossplane-system
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
endpoint:  14 bytes
port:      4 bytes
```

You will also see that the `CloudMemorystoreInstance` resource is still
reporting `Status: Unbound`. This is because we have not *claimed* it for usage
yet.

Crossplane follows a similar pattern to [Kubernetes persistent volumes]. When
you statically provision a resource in Crossplane, the external resource is also
created. However, when you want to use a resource, you create an
application-focused **claim** for it. In this case, we will create a
`RedisCluster` claim for the `CloudMemorystoreInstance`. Because we know exactly
which `CloudMemorystoreInstance` we want to use, we reference it directly from
the claim.

> **Claim**: a namespace-scoped custom resources that represents a claim for
> usage of a managed resource. Claims are abstract types, like `RedisCluster`,
> that have support across multiple providers. A claim may be satisfied by many
> different managed resource types. For instance, a `RedisCluster` can be
> satisfied by an instance of a GCP `CloudMemorystoreInstance`, an AWS
> `ReplicationGroup`, or an Azure `Redis`. It could also be satisfied by
> different configurations of a single resource type. For instance, you may have
> a large, medium, and small storage `CloudMemorystoreInstance`. When a claim
> becomes *bound* to a managed resource, any connection information from the
> managed resource (i.e. usernames, password, IP addresses, etc.) is propagated
> to the namespace of the claim.

First, let's create a `Namespace` for our claim:

```
kubectl create namespace cp-quickstart
```

Next, create a file named `redis-cluster-static.yaml` with the following
content:

```yaml
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  name: redis-claim-static
  namespace: cp-quickstart
spec:
  resourceRef:
    apiVersion: cache.gcp.crossplane.io/v1beta1
    kind: CloudMemorystoreInstance
    name: example-cloudmemorystore-instance
  writeConnectionSecretToRef:
    name: redis-connection-details-static
```

Now, create `RedisCluster` claim in your cluster with the following command:

```
kubectl apply -f redis-cluster-static.yaml
```

You should see the the claim was created, and is now bound:

```
kubectl get -f redis-cluster-static.yaml
```

```
NAME                 STATUS   CLASS-KIND   CLASS-NAME   RESOURCE-KIND              RESOURCE-NAME                       AGE
redis-claim-static   Bound                              CloudMemorystoreInstance   example-cloudmemorystore-instance   12s
```

You should also notice that the connection `Secret` we looked at earlier has now
been propagated to the namespace of our claim:

```
kubectl -n cp-quickstart get secrets
```

```
NAME                              TYPE                                  DATA   AGE
default-token-cnhfn               kubernetes.io/service-account-token   3      74s
redis-connection-details-static   Opaque                                2      36s
```

We have now created and prepared an external managed service for usage using
only `kubectl`, but it was a fairly manual process that required familiarity
with the underlying Redis implementation (Cloud Memorystore). This can be made
easier with *[dynamic provisioning]*.

## Clean Up

If you would like to clean up the resources created in this section, run the
following command:

```
kubectl delete -f redis-cluster-static.yaml
```

<!-- Named Links -->

[installation]: install.md
[configuration]: configure.md
[GCP `Provider`]: cloud-providers/gcp/gcp-provider.md
[Cloud Memorystore]: https://cloud.google.com/memorystore
[CustomResourceDefinition]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[cluster-scoped]: https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#create-a-customresourcedefinition
[Kubernetes persistent volumes]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
[dynamic provisioning]: dynamic.md

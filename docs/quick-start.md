---
title: Getting Started
toc: true
weight: 210
---
# Getting Started

This guide will demonstrate using Crossplane to deploy a portable Redis cluster
on the Google Cloud Platform (GCP). It serves as an initial introduction to
Crossplane, but only displays a small set of its features.

In this guide we will:

1. [Install Crossplane](#install-crossplane)
1. [Add your GCP project to Crossplane](#add-your-gcp-project-to-crossplane)
1. [Provision a Redis Cluster using Cloud
   Memorystore](#provision-a-redis-cluster)
1. [Define a class of Cloud Memorystore for dynamic
   provisioning](#define-a-class-of-cloud-memorystore)

## Install Crossplane

We'll start by installing Crossplane using [Helm]. You'll need a working
Kubernetes cluster ([minikube] or [kind] will do just fine). Crossplane is
currently in alpha, so we'll use the `alpha` channel:

```bash
# Crossplane lives in the crossplane-system namespace by convention.
kubectl create namespace crossplane-system

helm repo add crossplane-alpha https://charts.crossplane.io/alpha
helm install --name crossplane --namespace crossplane-system crossplane-alpha/crossplane
```

Once Crossplane is installed we'll need to install the a [stack] for our cloud
provider - in this case GCP. Installing the GCP stack teaches Crossplane how to
provision and maanage things in GCP. You install it by creating a
`ClusterStackInstall`:

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: stack-gcp
  namespace: crossplane-system
spec:
  package: "crossplane/stack-gcp:master"
```

Save the above as `stack.yaml`, and apply it by running:

```bash
kubectl apply -f stack.yaml
```

We've now installed Crossplane with GCP support! Take a look at the [Crossplane
installation guide] for more installation options, and to learn how to install
support for other cloud providers such as Amazon Web Services and Microsoft
Azure.

## Add Your GCP Project to Crossplane

We've taught Crossplane how to work with GCP - now we must tell it how to
connect to your GCP project. We'll do this by creating a Crossplane `Provider`
that specifies the project name and some GCP service account credentials to use:

```yaml
apiVersion: gcp.crossplane.io/v1alpha3
kind: Provider
metadata:
  name: example-provider
spec:
  # Make sure to update your project's name here.
  projectID: my-cool-gcp-project
  credentialsSecretRef:
    name: example-gcp-credentials
    namespace: crossplane-system
    key: credentials.json
```

Save the above `Provider` as `provider.yaml`, save your Google Application
Credentials as `credentials.json`, then run:

```bash
kubectl -n crossplane-system create secret generic example-gcp-credentials --from-file=credentials.json
kubectl apply -f provider.yaml
```

Crossplane can now manage your GCP project! Your service account will need the
Redis Admin role for this guide. Check out GCP's [Getting Started With
Authentication] guide if you need help creating a service account and
downloading its `credentials.json` file, and Crossplane's [GCP provider
documentation] for detailed instructions on setting up your project and service
account permissions.

## Provision a Redis Cluster

GCP provides Redis clusters using [Cloud Memorystore]. Crossplane uses a
resource and claim pattern to provision and manage cloud resources like Cloud
Memorystore - if you've ever used [persistent volumes in Kubernetes] you've seen
this pattern before. The simplest way to start using a new Redis cluster on GCP
is to provision a `CloudMemorystoreInstance`, then claim it via a
`RedisCluster`. We call this process _static provisioning_.


```yaml
apiVersion: cache.gcp.crossplane.io/v1beta1
kind: CloudMemorystoreInstance
metadata:
  name: example-cloudmemorystore-instance
spec:
  providerRef:
    name: example-provider
  writeConnectionSecretToRef:
    name: example-cloudmemorystore-connection-details
    namespace: crossplane-system
  reclaimPolicy: Delete
  forProvider:
    tier: STANDARD_HA
    region: us-west2
    memorySizeGb: 1
```

First we create a Cloud Memorystore instance. Save the above as
`cloudmemorystore.yaml`, then apply it:

```bash
kubectl apply -f cloudmemorystore.yaml
```

Crossplane is now creating the `CloudMemorystoreInstance`! Before we can use it,
we need to claim it.

```yaml
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  name: example-redis-claim
spec:
  resourceRef:
    apiVersion: cache.gcp.crossplane.io/v1beta1
    kind: CloudMemorystoreInstance
    name: example-cloudmemorystore-instance
  writeConnectionSecretToRef:
    name: example-redis-connection-details
```

Save the above as `redis.yaml`, and once again apply it:

```bash
kubectl --namespace default apply -f redis.yaml
```

In Crossplane cloud provider specific resources like the
`CloudMemorystoreInstance` we created above are called _managed resources_.
They're considered infrastructure, like a Kubernetes `Node` or
`PersistentVolume`. Managed resources exist at the cluster scope (they're not
namespaced) and let you specify nitty-gritty provider specific configuration
details. Managed resources that have reached `v1beta1` are a high fidelity
representation of their underlying cloud provider resource, and can be updated
to change their configuration after provisioning. We _claim_ these resources by
submitting a _resource claim_ like the `RedisCluster` above. Resource claims are
namespaced, and indicate that the managed resource they claim is in use by
_binding_ to it. You can also use resource claims to _dynamically provision_
managed resources on-demand - we'll discuss that in the next section of this
guide.

Soon your new `RedisCluster` should be online. You can use `kubectl` to inspect
its status. If you see `Bound` under the `STATUS` column, it's ready to use!

```bash
$ kubectl --namespace default get rediscluster example-redis-claim
NAME                  STATUS   CLASS-KIND   CLASS-NAME   RESOURCE-KIND              RESOURCE-NAME                       AGE
example-redis-claim   Bound                              CloudMemorystoreInstance   example-cloudmemorystore-instance   8m39s
```

You'll find all the details you need to connect to your new Redis cluster
instance saved in the Kubernetes `Secret` you specified via
`writeConnectionSecretToRef`, ready to [use with your Kubernetes pods].

```bash
$ kubectl --namespace default describe secret example-redis-connection-details
Name:         example-redis-connection-details
Namespace:    default
Labels:       <none>
Annotations:  crossplane.io/propagate-from-name: example-cloudmemorystore-connection-details
              crossplane.io/propagate-from-namespace: crossplane-system
              crossplane.io/propagate-from-uid: 7cd8666f-0bb9-11ea-8195-42010a800088

Type:  Opaque

Data
====
endpoint:  12 bytes
port:      4 bytes
```

That's all there is to static provisioning with Crossplane! We've created a
`CloudMemorystoreInstance` as cluster scoped infrastructure, then claimed it as
a `RedisCluster`. You can use `kubectl describe` to view the detailed
configuration and status of your `CloudMemorystoreInstance`.

```bash
$ kubectl describe cloudmemorystoreinstance example-cloudmemorystore-instance
Name:         example-cloudmemorystore-instance
Namespace:    
Labels:       <none>
Annotations:  crossplane.io/external-name: example-cloudmemorystore-instance
              kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"cache.gcp.crossplane.io/v1beta1","kind":"CloudMemorystoreInstance","metadata":{"annotations":{},"name":"example-cloudmemory...
API Version:  cache.gcp.crossplane.io/v1beta1
Kind:         CloudMemorystoreInstance
Metadata:
  Creation Timestamp:  2019-11-20T17:16:27Z
  Finalizers:
    finalizer.managedresource.crossplane.io
  Generation:        4
  Resource Version:  284706
  Self Link:         /apis/cache.gcp.crossplane.io/v1beta1/cloudmemorystoreinstances/example-cloudmemorystore-instance
  UID:               7c9cb407-0bb9-11ea-8195-42010a800088
Spec:
  Claim Ref:
    API Version:  cache.crossplane.io/v1alpha1
    Kind:         RedisCluster
    Name:         example-redis-claim
    Namespace:    default
    UID:          9cd9105b-0bb9-11ea-8195-42010a800088
  For Provider:
    Alternative Location Id:  us-west2-b
    Authorized Network:       projects/my-project/global/networks/default
    Location Id:              us-west2-a
    Memory Size Gb:           1
    Redis Version:            REDIS_4_0
    Region:                   us-west2
    Reserved Ip Range:        10.77.247.64/29
    Tier:                     STANDARD_HA
  Provider Ref:
    Name:  example-provider
  Write Connection Secret To Ref:
    Name:       example-cloudmemorystore-connection-details
    Namespace:  crossplane-system
Status:
  At Provider:
    Create Time:               2019-11-20T17:16:29Z
    Current Location Id:       us-west2-a
    Host:                      10.77.247.68
    Name:                      projects/my-project/locations/us-west2/instances/example-cloudmemorystore-instance
    Persistence Iam Identity:  serviceAccount:651413264395-compute@developer.gserviceaccount.com
    Port:                      6379
    State:                     READY
  Binding Phase:               Bound
  Conditions:
    Last Transition Time:  2019-11-20T17:16:27Z
    Reason:                Successfully resolved managed resource references to other resources
    Status:                True
    Type:                  ReferencesResolved
    Last Transition Time:  2019-11-20T17:20:00Z
    Reason:                Managed resource is available for use
    Status:                True
    Type:                  Ready
    Last Transition Time:  2019-11-20T17:16:29Z
    Reason:                Successfully reconciled managed resource
    Status:                True
    Type:                  Synced
```

Pay attention to the `Ready` and `Synced` conditions above. `Ready` represents
the availability of the Cloud Memorystore instance while `Synced` reflects
whether Crossplane is successfully applying your specified Cloud Memorystore
configuration.

## Define a Class of Cloud Memorystore

Now that we've learned how to statically provision and claim managed resources
it's time to try out _dynamic provisioning_. Dynamic provisioning allows us to
define a class of managed resource - a _resource class_ - that will be used to
automatically satisfy resource claims when they are created.

Here's a resource class that will dynamically provision Cloud Memorystore with
the same settings as the `CloudMemorystoreInstance` we provisioned earlier in
the guide:

```yaml
apiVersion: cache.gcp.crossplane.io/v1beta1
kind: CloudMemorystoreInstanceClass
metadata:
  name: example-cloudmemorystore-class
  annotations:
    resourceclass.crossplane.io/is-default-class: "true"
  labels:
    guide: getting-started
specTemplate:
  providerRef:
    name: example-provider
  writeConnectionSecretsToNamespace: crossplane-system
  reclaimPolicy: Delete
  forProvider:
    tier: STANDARD_HA
    region: us-west2
    memorySizeGb: 1
```

Save the above as `cloudmemorystore-class.yaml` and apply it to enable dynamic
provisioning of `CloudMemorystoreInstance` managed resources:

```bash
kubectl apply -f cloudmemorystore-class.yaml
```

Now you can omit the `resourceRef` when you create resource claims. Save the
below resource claim as `redis-dynamic-claim.yaml`:

```yaml
apiVersion: cache.crossplane.io/v1alpha1
kind: RedisCluster
metadata:
  name: redis-dynamic-claim
spec:
  classSelector:
    matchLabels:
      guide: getting-started
  writeConnectionSecretToRef:
    name: example-redis-dynamic-connection-details
```

When you apply this `RedisCluster` claim you'll see that it dynamically
provisions a new `CloudMemorystoreInstance` to satisfy the resource claim:

```bash
$ kubectl --namespace default apply -f redis-dynamic-claim.yaml
rediscluster.cache.crossplane.io/redis-dynamic-claim created

$ kubectl get rediscluster redis-dynamic-claim
NAME                  STATUS   CLASS-KIND                      CLASS-NAME                       RESOURCE-KIND              RESOURCE-NAME                       AGE
redis-dynamic-claim            CloudMemorystoreInstanceClass   example-cloudmemorystore-class   CloudMemorystoreInstance   default-redis-dynamic-claim-hvwwd   33s

```

You just dynamically provisioned a `CloudMemorystoreInstance`! You can find the
name of your new `CloudMemorystoreInstance` under the `RESOURCE-NAME` column
when you run `kubectl describe rediscluster`. Reuse the resource class as many
times as you like; simply submit more `RedisCluster` resource claims to create
more Cloud Memorystore instances.

You may have noticed that your resource claim included a `classSelector`. The
class selector lets you select which resource class to use by [matching its
labels]. Resource claims like `RedisCluster` can match different kinds of
resource class using label selectors, so you could just as easily use the exact
same `RedisCluster` to create an Amazon Replication Group instance by creating a
`ReplicationGroupClass` labelled as `guide: getting-started`. When multiple
resource classes match the class selector, a matching class is chosen at random.
Claims can be matched to classes by either:

* Specifying a `classRef` to a specific resource class.
* Specifying a `classSelector` that matches one or more resource classes.
* Omitting both of the above and defaulting to a resource class [annotated] as
  `resourceclass.crossplane.io/is-default-class: "true"`.

## Next Steps

* Add additional [cloud provider stacks](cloud-providers.md) to Crossplane.
* Explore the [Services Guide](services-guide.md) and the [Stacks
  Guide](stacks-guide.md).
* Learn more about [Crossplane concepts](concepts.md).
* See what managed resources are [currently supported](api.md) for each
  provider.
* Build [your own stacks](developer-guide.md)!

<!-- Named Links -->

[Helm]: https://helm.sh
[minikube]: https://kubernetes.io/docs/tasks/tools/install-minikube/
[kind]: https://github.com/kubernetes-sigs/kind
[stack]: concepts.md#stacks
[Crossplane installation guide]: install-crossplane.md
[Getting Started With Authentication]: https://cloud.google.com/docs/authentication/getting-started
[GCP provider documentation]: gcp-provider.md
[Cloud Memorystore]: https://cloud.google.com/memorystore/
[Persistent volumes in Kubernetes]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
[use with your Kubernetes pods]: https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets
[matching its labels]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
[annotated]: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/

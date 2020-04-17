---
title: Scheduling Workloads
toc: true
weight: 5
indent: true
---

# Scheduling Workloads to Remote Clusters

In the previous two examples, we provisioned infrastructure that is consumed by
some form of application. However, many providers expose services that you run
your application on. The most obvious example of this type of service would be a
managed Kubernetes service, such as [GKE], [EKS], or [AKS]. Crossplane not only
provisions and manages these types of infrastructure, but also allows you to
schedule workloads to them.

In the case of a Kubernetes cluster, Crossplane lets you schedule to remote
Kubernetes clusters from a single *control cluster*. The remote cluster may have
been, but does not have to been *provisioned* by Crossplane. Importantly, each
remote cluster maintains its own *control plane*. Crossplane is only responsible
for sending configuration data to the remote cluster.

> **Control Cluster**: the Kubernetes cluster where Crossplane is installed. It
> may also be used to run workloads besides Crossplane controllers, but it is
> not required to do so.

> **Remote Cluster**: a Kubernetes cluster that Crossplane has access to and may
> schedule workloads to. A remote cluster may have been created from the
> Crossplane control cluster using a provider's managed Kubernetes service, or
> it may be an existing cluster whose connection information was imported into
> the control cluster.

## Provisioning a GKE Cluster and Scheduling a Workload to it

By this point, you are familiar with both dynamic and static provisioning. In
this example, we will dynamically provision a `GKECluster`, but will focus on
what happens after it is ready and bound.

Create a file named `gke-cluster-class.yaml` with the following content:

```yaml
apiVersion: compute.gcp.crossplane.io/v1alpha3
kind: GKEClusterClass
metadata:
  name: gkecluster-standard
  labels:
    guide: quickstart
specTemplate:
  writeConnectionSecretsToNamespace: crossplane-system
  machineType: n1-standard-1
  numNodes: 1
  zone: us-central1-b
  providerRef:
    name: gcp-provider
  reclaimPolicy: Delete
```

Create the `GKEClusterClass` resource in your cluster:

```
kubectl apply -f gke-cluster-class.yaml
```

Now create a file named `k8s-cluster.yaml` with the following content:

```yaml
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesCluster
metadata:
  name: k8scluster
  namespace: cp-quickstart
  labels:
    cluster: hello-world
spec:
  classSelector:
    matchLabels:
      example: "true"
  writeConnectionSecretToRef:
    name: k8scluster
```

Then create the `KubernetesCluster` claim in your cluster:

```
kubectl apply -f k8s-cluster.yaml
```

As before, a `GKECluster` managed resource should be created and its connection
information will be propagated to the `cp-quickstart` namespace when it is ready
and bound:

```
kubectl get -f k8scluster.yaml
```

```
NAME         STATUS   CLASS-KIND        CLASS-NAME            RESOURCE-KIND   RESOURCE-NAME                    AGE
k8scluster            GKEClusterClass   gkecluster-standard   GKECluster      cp-quickstart-k8scluster-88426   36s
```

As you may have guessed, the connection information for a `KubernetesCluster`
claim contains [kubeconfig] information. Once the `KubernetesCluster` claim is
bound, you can view the contents of the `Secret` in the `cp-quickstart`
namespace:

```
kubectl -n cp-quickstart describe secret k8scluster
```

The `KubernetesCluster` claim is also unique from other claim types in that when
it becomes bound, Crossplane automatically creates a `KubernetesTarget` that
references the connection secret in the same namespace. You can see the
`KubernetesTarget` that Crossplane created for this `KubernetesCluster` claim:

```
kubectl -n cp-quickstart get kubernetestargets
```

> *Note: a `KubernetesTarget` that is automatically created by Crossplane for a
> bound `KubernetesCluster` claim will have the same labels as the
> `KubernetesCluster` claim.*

To schedule workloads to remote clusters, Crossplane requires those resource to
be wrapped in a `KubernetesApplication`.

> **Kubernetes Application**: a custom resource that bundles other resources
> that are intended to be run on a remote Kubernetes cluster. Creating a
> `KubernetesApplication` will cause Crossplane to find a suitable
> `KubernetesTarget` and attempt to create the bundled resources on the
> referenced `KubernetesCluster` using its connection `Secret`.

We can start by bundling a simple hello world app with a `Namespace`,
`Deployment`, and `Service` for scheduling to our GKE cluster.

Create a file named `helloworld.yaml` with the following content:

```yaml
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplication
metadata:
  name: helloworld
  namespace: cp-quickstart
  labels:
    app: helloworld
spec:
  resourceSelector:
    matchLabels:
      app: helloworld
  targetSelector:
    matchLabels:
      cluster: hello-world
  resourceTemplates:
    - metadata:
        name: helloworld-namespace
        labels:
          app: helloworld
      spec:
        template:
          apiVersion: v1
          kind: Namespace
          metadata:
            name: helloworld
            labels:
              app: helloworld
    - metadata:
        name: helloworld-deployment
        labels:
          app: helloworld
      spec:
        template:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: helloworld-deployment
            namespace: helloworld
          spec:
            selector:
              matchLabels:
                app: helloworld
            replicas: 1
            template:
              metadata:
                labels:
                  app: helloworld
              spec:
                containers:
                - name: hello-world
                  image: gcr.io/google-samples/node-hello:1.0
                  ports:
                  - containerPort: 8080
                    protocol: TCP
    - metadata:
        name: helloworld-service
        labels:
          app: helloworld
      spec:
        template:
          kind: Service
          metadata:
            name: helloworld-service
            namespace: helloworld
          spec:
            selector:
              app: helloworld
            ports:
            - port: 80
              targetPort: 8080
            type: LoadBalancer
```

Create the `KubernetesApplication`:

```
kubectl apply -f helloworld.yaml
```

Crossplane will immediately attempt to find a compatible `KubernetesTarget` with
matching labels to the stanza we included on our `KubernetesApplication`:

```
targetSelector:
  matchLabels:
    cluster: hello-world
```

Because we only have one `KubernetesTarget` with these labels in the
`cp-quickstart` namespace, the `KubernetesApplication` will be scheduled to the
GKE cluster we created earlier. You can view the progress of creating the
resources on the remote cluster by looking at the `KubernetesApplication` and
the resulting `KubernetesApplicationResources`:

```
kubectl -n cp-quickstart get kubernetesapplications
```

```
NAME         CLUSTER                                STATUS      DESIRED   SUBMITTED
helloworld   92184b85-4db3-48d2-99a2-36b3cf81226e   Scheduled   3         
```

```
kubectl -n cp-quickstart get kubernetesapplicationresources
```

```
NAME                    TEMPLATE-KIND   TEMPLATE-NAME           CLUSTER                                STATUS
helloworld-deployment   Deployment      helloworld-deployment   c1c435a3-8673-46d5-95bb-55cc5040a6fd   Submitted
helloworld-namespace    Namespace       helloworld              c1c435a3-8673-46d5-95bb-55cc5040a6fd   Submitted
helloworld-service      Service         helloworld-service      c1c435a3-8673-46d5-95bb-55cc5040a6fd   Submitted
```

> *Note: each in-line template in a `KubernetesApplication` results in the
> creation of a corresponding `KubernetesApplicationResource`. Crossplane keeps
> the resources on the remote cluster in sync with their
> `KubernetesApplicationResource`, and keeps each respective
> `KubernetesApplicationResource` in sync with its template on the
> `KubernetesApplication`.*

When all three resources have been provisioned, the `KubernetesApplication` will
show a `3` in the `SUBMITTED` column. If you inspect the
`KubernetesApplication`, you should see the IP address of the `LoadBalancer`
`Service` in the remote cluster. If you navigate your browser to that address,
you should be greeted by the hello world application.

```
kubectl -n cp-quickstart describe kubernetesapplicationresource helloworld-service
```

```
Name:         helloworld-service
Namespace:    cp-quickstart
Labels:       app=helloworld
Annotations:  <none>
API Version:  workload.crossplane.io/v1alpha1
Kind:         KubernetesApplicationResource
Metadata:
  Creation Timestamp:  2020-03-23T22:29:16Z
  Finalizers:
    finalizer.kubernetesapplicationresource.workload.crossplane.io
  Generation:  2
  Owner References:
    API Version:           workload.crossplane.io/v1alpha1
    Block Owner Deletion:  true
    Controller:            true
    Kind:                  KubernetesApplication
    Name:                  helloworld
    UID:                   1f1808ad-2b82-47df-8e8f-40255511c20a
  Resource Version:        31969
  Self Link:               /apis/workload.crossplane.io/v1alpha1/namespaces/cp-quickstart/kubernetesapplicationresources/helloworld-service
  UID:                     508c07d5-e3c8-4df2-9927-c320448db437
Spec:
  Target Ref:
    Name:  c1c435a3-8673-46d5-95bb-55cc5040a6fd
  Template:
    Kind:  Service
    Metadata:
      Name:       helloworld-service
      Namespace:  helloworld
    Spec:
      Ports:
        Port:         80
        Target Port:  8080
      Selector:
        App:  helloworld
      Type:   LoadBalancer
Status:
  Conditioned Status:
    Conditions:
      Last Transition Time:  2020-03-23T22:29:54Z
      Reason:                Successfully reconciled resource
      Status:                True
      Type:                  Synced
  Remote:
    Load Balancer:
      Ingress:
        Ip:  34.67.121.186 # the application is running at this IP address
  State:     Submitted
Events:      <none>
```

> *Note: Creating a cluster and scheduling resources to it is a nice workflow,
> but it is likely that you may want to schedule resources to an existing
> cluster or one that is not a managed service that Crossplane supports. This is
> made possible by storing the base64 encoded `kubeconfig` data in a `Secret`,
> then manually creating a `KubernetesTarget` to point at it. This is an
> advanced workflow, and additional information can be found in the
> [guide on manually adding clusters].*

## Clean Up

If you would like to clean up the resources created in this section, run the
following commands:

```
kubectl delete -f helloworld.yaml
kubectl delete -f k8s-cluster.yaml
```

<!-- Named Links -->

[GKE]: https://cloud.google.com/kubernetes-engine
[EKS]: https://aws.amazon.com/eks/
[AKS]: https://azure.microsoft.com/en-us/services/kubernetes-service/
[kubeconfig]: https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/
[guide on manually adding clusters]: ../guides/manually_adding_clusters.md

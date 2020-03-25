---
title: Applications
toc: true
weight: 7
---

# From Workloads to Apps

Crossplane *Applications* allow you to define your application and its managed
service dependencies as a single installable unit. They serve as an abstraction
above the claims, classes, and managed resources we explored in previous
sections. They are portable in that they create claims for infrastructure that
are satisfied by different managed service implementations depending on how your
Crossplane control cluster is configured.

## Deploying the Wordpress Application on GCP

[Wordpress](https://wordpress.org/) is a relatively simple monolithic
application that only requires compute to run its containerized binary and a
connection to a MySQL database. Wordpress is typically installed in a Kubernetes
cluster using its official [Helm
chart](https://github.com/bitnami/charts/tree/master/bitnami/wordpress).
Crossplane applications let you define your application using common
configuration tools such as [Helm](https://helm.sh/) and
[Kustomize](https://kustomize.io/), but represent them as a
[CustomResourceDefinition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
in your cluster.

The steps for using a Crossplane application involve defining your
infrastructure, installing the application, then creating an instance of that
application. In the [previous section](stack.md), we completed the first step by
creating our `MinimalGCP` instance. In contrast to the GCP provider and Minimal
GCP stack, the Wordpress application will be installed with a `StackInstall`
instead of a `ClusterStackInstall`. This means that the installation will only
be available in the namespace that we specify. You can read more about the
difference between the two in the [infrastructure
operators](infra_operators/packaging_a_stack.md) and [application
operators](app_operators/packaging_an_app.md) guides.

Create a file named `wordpress-install.yaml` with the following content:

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackInstall
metadata:
  name: app-wordpress
  namespace: cp-quickstart
spec:
  package: crossplane/app-wordpress:v0.2.0
```

Then create it in your cluster:

```
kubectl apply -f wordpress-install.yaml
```

We can now create Wordpress instances in the `crossplane-quickstart` namespace
using a single
[CustomResourceDefinition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
When we do, a `KubernetesCluster` claim and a `MySQLInstance` claim will be
created in the namespace, as well as a `KubernetesApplication` that contains the
Wordpress application components. The claims will be satisfied by the
`GKEClusterClass` and `CloudSQLInstanceClass` we created in the [previous
section](stack.md). Let's create a `WordpressInstance` and see what happens.

Create a file named `my-wordpress.yaml` with the following content:

```yaml
apiVersion: wordpress.apps.crossplane.io/v1alpha1
kind: WordpressInstance
metadata:
  name: my-wordpress
  namespace: cp-quickstart
spec:
  provisionPolicy: ProvisionNewCluster 
```

Then create it in your cluster:

```
kubectl apply -f my-wordpress.yaml
```

You can use the following commands to look at the resources being provisioned:

```
kubectl -n cp-quickstart get kubernetesclusters
```

```
NAME                   STATUS   CLASS-KIND        CLASS-NAME                   RESOURCE-KIND   RESOURCE-NAME                              AGE
my-wordpress-cluster            GKEClusterClass   my-min-gcp-gkeclusterclass   GKECluster      cp-quickstart-my-wordpress-cluster-jxftn   19s
```

```
kubectl -n cp-quickstart get mysqlinstances
```

```
NAME               STATUS   CLASS-KIND              CLASS-NAME                               RESOURCE-KIND      RESOURCE-NAME                          AGE
my-wordpress-sql            CloudSQLInstanceClass   my-min-gcp-cloudsqlinstanceclass-mysql   CloudSQLInstance   cp-quickstart-my-wordpress-sql-vz9r7   30s
```

```
kubectl -n cp-quickstart get kubernetesapplications
```

```
NAME               CLUSTER   STATUS    DESIRED   SUBMITTED
my-wordpress-app             Pending             
```

It will take some time for the `GKECluster` and `CloudSQLInstance` to be
provisioned and ready, but when they are, Crossplane will schedule the Wordpress
`KubernetesApplication` to the remote `GKECluster`, as well as send the
`CloudSQLInstance` connection information to the remote cluster in the form of a
`Secret`. Because Wordpress is running in the `GKECluster` that we created in
the same network as the `CloudSQLInstance`, it will be able to communicate with
it freely.

When the `KubernetesApplication` has submitted all of its resources to the
cluster, you should be able to view the IP address of the Wordpress `Service`:

```
kubectl -n cp-quickstart describe kubernetesapplicationresources my-wordpress-service
```

```
Name:         my-wordpress-service
Namespace:    cp-quickstart
Labels:       app=my-wordpress
Annotations:  <none>
API Version:  workload.crossplane.io/v1alpha1
Kind:         KubernetesApplicationResource
Metadata:
  Creation Timestamp:  2020-03-23T23:07:07Z
  Finalizers:
    finalizer.kubernetesapplicationresource.workload.crossplane.io
  Generation:  1
  Owner References:
    API Version:           workload.crossplane.io/v1alpha1
    Block Owner Deletion:  true
    Controller:            true
    Kind:                  KubernetesApplication
    Name:                  my-wordpress-app
    UID:                   c4baec14-c8ac-4c75-94f9-0a1cd3638ea6
  Resource Version:        3509
  Self Link:               /apis/workload.crossplane.io/v1alpha1/namespaces/cp-quickstart/kubernetesapplicationresources/my-wordpress-service
  UID:                     80f5513a-704c-41f9-b5e9-d681dda85feb
Spec:
  Target Ref:
    Name:  c568d44c-c882-42ca-ab4b-217cd101b269
  Template:
    API Version:  v1
    Kind:         Service
    Metadata:
      Labels:
        App:      wordpress
      Name:       wordpress
      Namespace:  my-wordpress
    Spec:
      Ports:
        Port:  80
      Selector:
        App:  wordpress
      Type:   LoadBalancer
Status:
  Conditioned Status:
    Conditions:
      Last Transition Time:  2020-03-23T23:07:11Z
      Reason:                Successfully reconciled resource
      Status:                True
      Type:                  Synced
  Remote:
    Load Balancer:
      Ingress:
        Ip:  34.94.54.204 # the application is running at this IP address
  State:     Submitted
Events:      <none>
```

Navigating to the address in your browser should take you to the Wordpress
welcome page.

Now you are familiar with **Providers**, **Stacks**, and **Applications**. The
next step is to build and deploy your own. Take a look at the following guides
to learn more:

- [Infrastructure Operators](infra_operators/installing_a_stack.md)
- [Application Operators](app_operators/packaging_an_app.md)
- [Developers](developers/requesting_infrastructure.md)

## Clean Up

If you would like to clean up the resources created in this section, run the
following commands:

```
kubectl delete -f my-wordpress.yaml
kubectl delete -f my-min-gcp.yaml
```

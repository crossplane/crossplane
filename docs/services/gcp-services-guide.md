---
title: Using GCP Services
toc: true
weight: 420
indent: true
---

# GCP Services Guide

This user guide will walk you through Wordpress application deployment using
your existing Kubernetes cluster and Crossplane managed resources. We will:
* Install Crossplane to your cluster.
* Create necessary resource classes for MySQL database instance.
* Create network resources to get GKE cluster to connect to MySQL instance.
* Deploy Wordpress.

## Table of Contents

1. [Pre-requisites](#pre-requisites)
1. [Preparation](#preparation)
1. [Set Up Crossplane](#set-up-crossplane)
    1. [Install in Target Cluster](#install-in-target-cluster)
    1. [Cloud Provider](#cloud-provider)
    1. [Resource Classes](#resource-classes)
    1. [Configure Managed Service Access](#configure-managed-service-access)
1. [Provision MySQL](#provision-mysql)
   1. [Resource Claim](#resource-claim)
1. [Install Wordpress](#install-wordpress)
1. [Clean Up](#clean-up)
1. [Conclusion and Next Steps](#conclusion-and-next-steps)

## Pre-requisites

* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* A GKE cluster.

## Preparation

This guide assumes that you have setup the gcloud CLI and are logged in to your
desired account.

*Note: environment variables are used throughout this guide. You should use your
own values.*

Run the following:
```bash
export PROJECT_ID=crossplane-playground # the project that all resources reside.
export NETWORK_NAME=default # the network that your GKE cluster lives in.
export SUBNETWORK_NAME=default # the subnetwork that your GKE cluster lives in.
```

## Set Up Crossplane

### Installation

Assuming you are
[connected](https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl)
to your GKE cluster via `kubectl`:

* Install Crossplane from alpha channel using the [Crossplane Installation
  Guide](../install-crossplane.md#alpha)
* Install the GCP stack into Crossplane using the [GCP stack
  section](../install-crossplane.md#gcp-stack) of the install guide.

To keep your resource configuration organized, start by creating a new
directory:

```bash
mkdir wordpress && cd $_
```

### Cloud Provider

It is essential to make sure that the GCP user credentials are configured in
Crossplane as a provider. Please follow the steps [provider
guide](../cloud-providers/gco/gcp-provider.md) for more information.

### Resource Classes

Cloud-specific resource classes are used to define a reusable configuration for
a specific managed service. Wordpress requires a MySQL database, which can be
satisfied by a [Google Cloud SQL
Instance](https://cloud.google.com/sql/docs/mysql/).

* Define a GCP CloudSQL class `CloudSQLInstanceClass`:

```bash
cat > gcp-mysql-standard.yaml <<EOF
---
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstanceClass
metadata:
  name: standard-cloudsql
  labels:
    size: standard
specTemplate:
  writeConnectionSecretsToNamespace: crossplane-system
  forProvider:
    databaseVersion: MYSQL_5_7
    region: us-central1
    settings:
      tier: db-n1-standard-1
      dataDiskType: PD_SSD
      dataDiskSizeGb: 10
      # Note from GCP Docs: Your Cloud SQL instances are not created in your VPC network.
      # They are created in the service producer network (a VPC network internal to Google) that is then connected (peered) to your VPC network.
      ipConfiguration:
        privateNetwork: projects/$PROJECT_ID/global/networks/$NETWORK_NAME
  providerRef:
    name: example
  reclaimPolicy: Delete
EOF

kubectl apply -f gcp-mysql-standard.yaml
```

* You can verify creation with the following command and output:

*Command*
```bash
kubectl get cloudsqlinstanceclasses
```
*Output*
```bash
NAME                   PROVIDER-REF   RECLAIM-POLICY   AGE
standard-cloudsql      gcp-provider   Delete           11s
```

You are free to create more GCP `CloudSQLInstanceClass` instances to define more
potential configurations. For instance, you may create `large-gcp-mysql` with
field `storageGB: 100`.

### Configure Managed Service Access

Before we install Wordpress, we need to establish connectivity between the the
MySQL database and the GKE cluster. We can do this by creating a [Private
Service
Connection](https://cloud.google.com/vpc/docs/configure-private-services-access).

You can create it by following the instructions at the link above, or you could
use Crossplane to do it:

* Create a `GlobalAddress` and `Connection` resources:

  ```bash
  cat > network.yaml <<EOF
  ---
  # example-globaladdress defines the IP range that will be allocated for cloud services connecting
  # to the instances in the given Network.
  apiVersion: compute.gcp.crossplane.io/v1alpha2
  kind: GlobalAddress
  metadata:
    name: example-globaladdress
  spec:
    providerRef:
      name: gcp-provider
    reclaimPolicy: Delete
    name: example-globaladdress
    purpose: VPC_PEERING
    addressType: INTERNAL
    prefixLength: 16
    network: projects/$PROJECT_ID/global/networks/$NETWORK_NAME
  ---
  # example-connection is what allows cloud services to use the allocated GlobalAddress for communication. Behind
  # the scenes, it creates a VPC peering to the network that those service instances actually live.
  apiVersion: servicenetworking.gcp.crossplane.io/v1alpha2
  kind: Connection
  metadata:
    name: example-connection
  spec:
    providerRef:
      name: gcp-provider
    reclaimPolicy: Delete
    parent: services/servicenetworking.googleapis.com
    network: projects/$PROJECT_ID/global/networks/$NETWORK_NAME
    reservedPeeringRanges:
      - example-globaladdress
  EOF

  kubectl apply -f network.yaml
  ```

* You can verify creation with the following command and output:

  *Command*

  ```bash
  kubectl get connection example-connection -o custom-columns='NAME:.metadata.name,FIRST_CONDITION:.status.conditions[0].status,SECOND_CONDITION:.status.conditions[1].status'
  ```

  *Output*

  ```bash
  NAME                 FIRST_CONDITION   SECOND_CONDITION
  example-connection   True              True
  ```

  Wait for both conditions to be true to continue. The conditions we're checking
  for are `Ready` and `Synced`. The reason we are using `FIRST_CONDITION` and
  `SECOND_CONDITION` is because we don't know what order they'll be in when we
  run the command.

## Provision 

### Resource Claim

Resource claims are used for dynamic provisioning of a managed resource (like a
MySQL instance) by matching the claim to a resource class. This can be done in
several ways: (a) rely on the default class marked
`resourceclass.crossplane.io/is-default-class: "true"`, (b) use a
`claim.spec.classRef` to a specific class, or (c) match on class labels using a
`claim.spec.classSelector`.

*Note: claims may also be used in [static
provisioning](../concepts.md#dynamic-and-static-provisioning) with a reference
to an existing managed resource.*

In the `CloudsqlInstanceClass` above, we added the label `size: standard`, so
our claim will be scheduled to that class using the label is specified in the
`claim.spec.classSelector`. If there are multiple classes which match the
specified label(s) one will be chosen at random.

* Define a `MySQLInstance` claim in `mysql-claim.yaml`:

  ```bash
  cat > mysql-claim.yaml <<EOF
  ---
  apiVersion: database.crossplane.io/v1alpha1
  kind: MySQLInstance
  metadata:
    name: mysql-claim
  spec:
    classSelector:
      matchLabels:
        size: standard
    engineVersion: "5.7"
    # A secret is exported by providing the secret name
    # to export it under. This is the name of the secret
    # in the crossplane cluster, and it's scoped to this claim's namespace.
    writeConnectionSecretToRef:
      name: wordpressmysql
  EOF

  kubectl apply -f mysql-claim.yaml
  ```

What we are looking for is for the claim's `STATUS` value to become `Bound`
which indicates the managed resource was successfully provisioned and is ready
for consumption. You can see when claim is bound using the following:

*Command*
```bash
kubectl get mysqlinstances
```

*Output*
```bash
NAME          STATUS   CLASS-KIND         CLASS-NAME          RESOURCE-KIND      RESOURCE-NAME                                         AGE
mysql-claim   Bound    mysql-standard     standard-cloudsql   CloudsqlInstance   mysqlinstance-6a7fe064-d888-11e9-ab90-42b6bb22213a    11m
```

If the `STATUS` is blank, we are still waiting for the claim to become bound.
You can observe resource creation progression using the following:

*Command*
```bash
kubectl describe mysqlinstance mysql-claim --watch
```

*Output*
```
Name:         mysql-claim
Namespace:    default
Labels:       <none>
Annotations:  kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"database.crossplane.io/v1alpha1","kind":"MySQLInstance","metadata":{"annotations":{},"name":"mysql-claim","namespace":"team..."}}
API Version:  database.crossplane.io/v1alpha1
Kind:         MySQLInstance
Metadata:
  Creation Timestamp:  2019-09-16T13:46:42Z
  Finalizers:
    finalizer.resourceclaim.crossplane.io
  Generation:        2
  Resource Version:  4256
  Self Link:         /apis/database.crossplane.io/v1alpha1/namespaces/app-project1-dev/mysqlinstances/mysql-claim
  UID:               6a7fe064-d888-11e9-ab90-42b6bb22213a
Spec:
  Class Ref:
    Name:          mysql-standard
  Engine Version:  5.6
  Resource Ref:
    API Version:  database.gcp.crossplane.io/v1beta1
    Kind:         CloudSQLInstance
    Name:         mysqlinstance-6a7fe064-d888-11e9-ab90-42b6bb22213a
    Namespace:    gcp-infra-dev
  Write Connection Secret To Ref:
    Name:  wordpressmysql
Status:
  Conditions:
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Managed claim is waiting for managed resource to become bindable
    Status:                False
    Type:                  Ready
    Last Transition Time:  2019-09-16T13:46:42Z
    Reason:                Successfully reconciled managed resource
    Status:                True
    Type:                  Synced
Events:                    <none>
```

*Note: You must wait until the claim becomes bound before continuing with this
guide. It could take a few minutes for GCP to complete CloudSQL creation.*

## Install Wordpress

Installing Wordpress requires creating a Kubernetes `Deployment` and load
balancer `Service`. We will point the deployment to the `wordpressmysql` secret
that we specified in our claim above for the Wordpress container environment
variables. It should have been populated with our MySQL connection details after
the claim became `Bound`.

> Binding status tells you whether your resource has been provisioned and ready
to use. Crossplane binds the actual resource to the claim via changing the
readiness condition to `Bound`. This happens only when the resource is ready to
be consumed.

* Check to make sure `wordpressmysql` exists and is populated:

*Command*
```bash
kubectl describe secret wordpressmysql
```

*Output*
```bash
Name:         wordpressmysql
Namespace:    default
Labels:       <none>
Annotations:  crossplane.io/propagate-from-name: c3aca763-f698-11e9-a957-12a4af141bea
            crossplane.io/propagate-from-namespace: crossplane-system
            crossplane.io/propagate-from-uid: c539fcef-f698-11e9-a957-12a4af141bea

Type:  Opaque

Data
====
endpoint:  75 bytes
password:  27 bytes
username:  58 bytes
```

* Define the `Deployment` and `Service` in `wordpress.yaml`:

  ```bash
  cat > wordpress.yaml <<EOF
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: wordpress
    labels:
      app: wordpress
  spec:
    selector:
      matchLabels:
        app: wordpress
    template:
      metadata:
        labels:
          app: wordpress
      spec:
        containers:
          - name: wordpress
            image: wordpress:4.6.1-apache
            env:
              - name: WORDPRESS_DB_HOST
                valueFrom:
                  secretKeyRef:
                    name: wordpressmysql
                    key: endpoint
              - name: WORDPRESS_DB_USER
                valueFrom:
                  secretKeyRef:
                    name: wordpressmysql
                    key: username
              - name: WORDPRESS_DB_PASSWORD
                valueFrom:
                  secretKeyRef:
                    name: wordpressmysql
                    key: password
            ports:
              - containerPort: 80
                name: wordpress
  ---
  apiVersion: v1
  kind: Service
  metadata:
    name: wordpress
    labels:
      app: wordpress
  spec:
    ports:
      - port: 80
    selector:
      app: wordpress
    type: LoadBalancer
  EOF

  kubectl apply -f wordpress.yaml
  ```

* You can verify creation with the following command and output:

  *Command*

  ```bash
  kubectl get -f wordpress.yaml
  ```

  *Output*

  ```bash
  NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
  deployment.apps/wordpress   1/1     1            1           11m

  NAME                TYPE           CLUSTER-IP    EXTERNAL-IP   PORT(S)        AGE
  service/wordpress   LoadBalancer   10.0.128.30   52.168.69.6   80:32587/TCP   11m
  ```

If the `EXTERNAL-IP` field of the `LoadBalancer` is `<pending>`, wait until it
becomes available, then navigate to the address. You should see the following:

![alt wordpress](wordpress-start.png)

## Clean Up

Because we put all of our configuration in a single directory, we can delete it
all with this command:

```bash
kubectl delete -f wordpress/
```

If you would like to also uninstall Crossplane and the AWS stack, run the
following command:

```bash
kubectl delete namespace crossplane-system
```

## Conclusion and Next Steps

We're done!

In this guide, we:

* Set up Crossplane on our GKE Cluster.
* Installed Crossplane GCP Stack.
* Created resource classes for MySQL database.
* Provisioned a MySQL database on GCP using Crossplane.
* Connected our GKE cluster to our MySQL database.
* Installed Wordpress to our GKE cluster.

In this guide, we used an existing GKE cluster but actually Crossplane can
provision a Kubernetes cluster from GCP just like it provisions a MySQL
database.

We deployed Wordpress using bare `Deployment` and `Service` resources but there
is actually a Wordpress App stack that creates these resources for us!

Check out the [stacks guides](../stacks-guide.md)!

## References

* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* [Crossplane Installation Guide](../install-crossplane.md#alpha)
* [GCP Stack Installation](../install-crossplane.md#gcp-stack)
* [GCP Provider Guide](../cloud-providers/gcp/gcp-provider.md)
* [Google Cloud SQL Instance](https://cloud.google.com/sql/docs/mysql/)
* [Default Resource Classes One-Pager](https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-default-resource-class.md)
* [Google Private Service Connection](https://cloud.google.com/vpc/docs/configure-private-services-access)

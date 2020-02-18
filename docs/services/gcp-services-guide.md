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

* [kubectl]
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
```

## Set Up Crossplane

### Installation

Assuming you are [connected] to your GKE cluster via `kubectl`:

* Install Crossplane from alpha channel using the [Crossplane Installation Guide]
* Install the GCP stack into Crossplane using the [GCP stack section] of the install guide.

To keep your resource configuration organized, start by creating a new
directory:

```bash
mkdir wordpress && cd $_
```

### Cloud Provider

It is essential to make sure that the GCP user credentials are configured in
Crossplane as a provider. Please follow the steps in the GCP [provider guide] for more information.

### Resource Classes

Resource classes are used to define a reusable configuration for a specific
managed service. Wordpress requires a MySQL database, which can be satisfied by
a [Google Cloud SQL Instance].

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
    name: gcp-provider
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
MySQL database and the GKE cluster. We can do this by creating a [Private Service Connection].

You can create it by following the instructions at the link above, or you could
use Crossplane to do it:

* Create a `GlobalAddress` and `Connection` resources:

  ```bash
  cat > network.yaml <<EOF
  ---
  # example-globaladdress defines the IP range that will be allocated for cloud services connecting
  # to the instances in the given Network.
  apiVersion: compute.gcp.crossplane.io/v1beta1
  kind: GlobalAddress
  metadata:
    name: example-globaladdress
  spec:
    forProvider:
      purpose: VPC_PEERING
      addressType: INTERNAL
      prefixLength: 16
      network: projects/$PROJECT_ID/global/networks/$NETWORK_NAME
    providerRef:
      name: gcp-provider
    reclaimPolicy: Delete
  ---
  # example-connection is what allows cloud services to use the allocated GlobalAddress for communication. Behind
  # the scenes, it creates a VPC peering to the network that those service instances actually live.
  apiVersion: servicenetworking.gcp.crossplane.io/v1beta1
  kind: Connection
  metadata:
    name: example-connection
  spec:
    forProvider:
      parent: services/servicenetworking.googleapis.com
      network: projects/$PROJECT_ID/global/networks/$NETWORK_NAME
      reservedPeeringRangeRefs:
        - name: example-globaladdress
    providerRef:
      name: gcp-provider
    reclaimPolicy: Delete
  EOF

  kubectl apply -f network.yaml
  ```

* You can verify creation with the following command and output:

  *Command*

  ```bash
  kubectl describe connection.servicenetworking.gcp.crossplane.io example-connection
  ```

  *Output*

  ```
  Name:         example-connection
  Namespace:    
  Labels:       <none>
  Annotations:  crossplane.io/external-name: example-connection
                kubectl.kubernetes.io/last-applied-configuration:
                  {"apiVersion":"servicenetworking.gcp.crossplane.io/v1beta1","kind":"Connection","metadata":{"annotations":{},"name":"example-connection"}...
  API Version:  servicenetworking.gcp.crossplane.io/v1beta1
  Kind:         Connection
  Metadata:
    Creation Timestamp:  2019-10-28T14:10:23Z
    Finalizers:
      finalizer.managedresource.crossplane.io
    Generation:        1
    Resource Version:  7245
    Self Link:         /apis/servicenetworking.gcp.crossplane.io/v1beta1/connections/example-connection
    UID:               aeae7e4d-f98c-11e9-8275-42010a800122
  Spec:
    Network:  projects/crossplane-playground/global/networks/default
    Parent:   services/servicenetworking.googleapis.com
    Provider Ref:
      Name:          gcp-provider
    Reclaim Policy:  Delete
    Reserved Peering Ranges:
      example-globaladdress
  Status:
    Conditions:
      Last Transition Time:  2019-10-28T14:10:23Z
      Reason:                Successfully resolved managed resource references to other resources
      Status:                True
      Type:                  ReferencesResolved
      Last Transition Time:  2019-10-28T14:10:23Z
      Reason:                Managed resource is being created
      Status:                False
      Type:                  Ready
      Last Transition Time:  2019-10-28T14:10:23Z
      Reason:                Successfully reconciled managed resource
      Status:                True
      Type:                  Synced
  Events:                    <none>
  ```

  We are looking for the `Connection` resource to report `Type: Ready` `Status:
  True` in its `status.conditions`.

## Provision 

### Resource Claim

Resource claims are used for dynamic provisioning of a managed resource (like a
MySQL instance) by matching the claim to a resource class. This can be done in
several ways: (a) rely on the default class marked
`resourceclass.crossplane.io/is-default-class: "true"`, (b) use a
`claim.spec.classRef` to a specific class, or (c) match on class labels using a
`claim.spec.classSelector`.

*Note: claims may also be used in [static provisioning] with a reference
to an existing managed resource.*

In the `CloudSQLInstanceClass` above, we added the label `size: standard`, so
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
NAME          STATUS   CLASS-KIND              CLASS-NAME          RESOURCE-KIND      RESOURCE-NAME               AGE
mysql-claim   Bound    CloudSQLInstanceClass   standard-cloudsql   CloudSQLInstance   default-mysql-claim-vtnf7   3m
```

If the `STATUS` is blank, we are still waiting for the claim to become bound.
You can observe resource creation progression using the following:

*Command*
```bash
kubectl describe mysqlinstance mysql-claim
```

*Output*
```
Name:         mysql-claim
Namespace:    default
Labels:       <none>
Annotations:  kubectl.kubernetes.io/last-applied-configuration:
                {"apiVersion":"database.crossplane.io/v1alpha1","kind":"MySQLInstance","metadata":{"annotations":{},"name":"mysql-claim","namespace":"defa...
API Version:  database.crossplane.io/v1alpha1
Kind:         MySQLInstance
Metadata:
  Creation Timestamp:  2019-10-28T14:18:55Z
  Finalizers:
    finalizer.resourceclaim.crossplane.io
  Generation:        3
  Resource Version:  9011
  Self Link:         /apis/database.crossplane.io/v1alpha1/namespaces/default/mysqlinstances/mysql-claim
  UID:               e0329d69-f98d-11e9-8275-42010a800122
Spec:
  Class Ref:
    API Version:  database.gcp.crossplane.io/v1beta1
    Kind:         CloudSQLInstanceClass
    Name:         standard-cloudsql
    UID:          431580bd-f989-11e9-8275-42010a800122
  Class Selector:
    Match Labels:
      Size:        standard
  Engine Version:  5.7
  Resource Ref:
    API Version:  database.gcp.crossplane.io/v1beta1
    Kind:         CloudSQLInstance
    Name:         default-mysql-claim-vtnf7
    UID:          e07c42c5-f98d-11e9-8275-42010a800122
  Write Connection Secret To Ref:
    Name:  wordpressmysql
Status:
  Conditions:
    Last Transition Time:  2019-10-28T14:18:56Z
    Reason:                Managed claim is waiting for managed resource to become bindable
    Status:                False
    Type:                  Ready
    Last Transition Time:  2019-10-28T14:18:56Z
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
Annotations:  crossplane.io/propagate-from-name: 330cccf5-f991-11e9-8275-42010a800122
              crossplane.io/propagate-from-namespace: crossplane-system
              crossplane.io/propagate-from-uid: 33581ec7-f991-11e9-8275-42010a800122

Type:  Opaque

Data
====
endpoint:                             10 bytes
password:                             27 bytes
publicIP:                             13 bytes
serverCACertificateCert:              1272 bytes
serverCACertificateCommonName:        98 bytes
serverCACertificateCreateTime:        24 bytes
serverCACertificateExpirationTime:    24 bytes
privateIP:                            10 bytes
serverCACertificateCertSerialNumber:  1 bytes
serverCACertificateInstance:          25 bytes
serverCACertificateSha1Fingerprint:   40 bytes
username:                             4 bytes
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
  deployment.apps/wordpress   1/1     1            1           77s

  NAME                TYPE           CLUSTER-IP    EXTERNAL-IP      PORT(S)        AGE
  service/wordpress   LoadBalancer   10.12.3.121   35.223.147.148   80:30287/TCP   77s
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

Check out the [stacks guides]!

[kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl
[connected]: https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl
[Crossplane Installation Guide]: ../install-crossplane.md#alpha
[GCP stack section]: ../install-crossplane.md#gcp-stack
[provider guide]: ../cloud-providers/gcp/gcp-provider.md
[Google Cloud SQL Instance]: https://cloud.google.com/sql/docs/mysql/
[Private Service Connection]: https://cloud.google.com/vpc/docs/configure-private-services-access
[static provisioning]: ../concepts.md#dynamic-and-static-provisioning
[stacks guides]: ../stacks-guide.md


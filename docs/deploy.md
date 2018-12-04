---
title: Deploying Workloads
toc: true
weight: 340
indent: true
---
# Deploying Workloads

## Guides

This section will walk you through how to deploy workloads to various cloud provider environments in a highly portable way.
For detailed instructions on how to deploy workloads to your cloud provider of choice, please visit the following guides:

* [Deploying a Workload on Google Cloud Platform (GCP)](workloads/gcp/wordpress-gcp.md)
* [Deploying a Workload on Microsoft Azure](workloads/azure/wordpress-azure.md)
* [Deploying a Workload on Amazon Web Services](workloads/aws/wordpress-aws.md)

## Workload Overview

A workload is a schedulable unit of work and contains a payload as well as defines its requirements for how the workload should run and what resources it will consume.
This helps Crossplane setup connectivity between the workload and resources, and make intelligent decisions about where and how to provision and manage the resources in their entirety.
Crossplane's scheduler is responsible for deploying the workload to a target cluster, which in this guide we will also be using Crossplane to deploy within your chosen cloud provider.

This walkthrough also demonstrates Crossplane's concept of a clean "separation of concerns" between developers and administrators.
Developers define workloads without having to worry about implementation details, environment constraints, and policies.
Administrators can define environment specifics, and policies.
The separation of concern leads to a higher degree of reusability and reduces complexity.

During this walkthrough, we will assume two separate identities:

1. Administrator (cluster or cloud) - responsible for setting up credentials and defining resource classes
2. Application Owner (developer) - responsible for defining and deploying the application and its dependencies

## Workload Example

### Dependency Resource

Let's take a closer look at a dependency resource that a workload will declare:

```yaml
## WordPress MySQL Database Instance
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: demo
  namespace: default
spec:
  classReference:
    name: standard-mysql
    namespace: crossplane-system
  engineVersion: "5.7"
```

This will request to create a `MySQLInstance` version 5.7, which will be fulfilled by the `standard-mysql` `ResourceClass`.
Note that the application developer is not aware of any further specifics when it comes to the `MySQLInstance` beyond their requested engine version.
This enables highly portable workloads, since the environment specific details of the database are defined by the administrator in a `ResourceClass`.

### Workload

Now let's look at the workload itself, which will reference the dependency resource from above, as well as other information such as the target cluster to deploy to.

```yaml
## WordPress Workload
apiVersion: compute.crossplane.io/v1alpha1
kind: Workload
metadata:
  name: demo
  namespace: default
spec:
  resources:
  - name: demo
    secretName: demo
  targetCluster:
    name: demo-gke-cluster
    namespace: crossplane-system
  targetDeployment:
    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: wordpress
      labels:
        app: wordpress
    spec:
      selector:
        app: wordpress
      strategy:
        type: Recreate
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
                      name: demo
                      key: endpoint
                - name: WORDPRESS_DB_USER
                  valueFrom:
                    secretKeyRef:
                      name: demo
                      key: username
                - name: WORDPRESS_DB_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: demo
                      key: password
              ports:
                - containerPort: 80
                  name: wordpress
  targetNamespace: demo
  targetService:
    apiVersion: v1
    kind: Service
    metadata:
      name: wordpress
    spec:
      ports:
        - port: 80
      selector:
        app: wordpress
      type: LoadBalancer
```
   
This `Workload` definition contains multiple components that informs Crossplane on how to deploy the workload and its resources:

- Resources: list of the resources required by the payload application
- TargetCluster: the cluster where the payload application and all its requirements should be deployed
- TargetNamespace: the namespace on the target cluster
- Workload Payload:
    - TargetDeployment
    - TargetService

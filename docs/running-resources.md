---
title: Running Resources
toc: true
weight: 350
indent: true
---
# Running Resources

Crossplane enables you to run a number of different resources in a portable and cloud agnostic way, allowing you to author an application that runs without modifications on multiple environments and cloud providers.
A single Crossplane enables the provisioning and full-lifecycle management of infrastructure across a wide range of providers, vendors, regions, and offerings.

## Running Databases

Database managed services can be statically or dynamically provisioned by Crossplane in AWS, GCP, and Azure.
An application developer simply has to specify their general need for a database such as MySQL, without any specific knowledge of what environment that database will run in or even what specific type of database it will be at runtime.
The following sample is all the application developer needs to specify in order to get the correct MySQL database (CloudSQL, RDS, Azure MySQL) provisioned and configured for their application:

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: demo-mysql
spec:
  classReference:
    name: standard-mysql
    namespace: crossplane-system
  engineVersion: "5.7"
```

The cluster administrator specifies a resource class that acts as a template with the implementation details and policy specific to the environment that the generic MySQL resource is being deployed to.
This enables the database to be dynamically provisioned at deployment time without the application developer needing to know any of the details, which promotes portability and reusability.
An example resource class that will provision a CloudSQL instance in GCP in order to fulfill the applications general MySQL requirement would look like this:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: standard-mysql
  namespace: crossplane-system
parameters:
  tier: db-n1-standard-1
  region: us-west2
  storageType: PD_SSD
provisioner: cloudsqlinstance.database.gcp.crossplane.io/v1alpha1
providerRef:
  name: gcp-provider
reclaimPolicy: Delete
```

## Running Kubernetes Clusters

Kubernetes clusters are another type of resource that can be dynamically provisioned using a generic resource claim by the application developer and an environment specific resource class by the cluster administrator.

Generic Kubernetes cluster resource claim created by the application developer:

```yaml
apiVersion: compute.crossplane.io/v1alpha1
kind: KubernetesCluster
metadata:
  name: demo-cluster
  namespace: crossplane-system
spec:
  classReference:
    name: standard-cluster
    namespace: crossplane-system
```

Environment specific GKE cluster resource class created by the admin:

```yaml
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  name: standard-cluster
  namespace: crossplane-system
parameters:
  machineType: n1-standard-1
  numNodes: "1"
  zone: us-central1-a
provisioner: gkecluster.compute.gcp.crossplane.io/v1alpha1
providerRef:
  name: gcp-provider
reclaimPolicy: Delete
```

## Future support

As the project continues to grow with support from the community, support for more resources will be added.
This includes all of the essential managed services from cloud providers as well as local or in-cluster services that deploy using the operator pattern.
Crossplane will provide support for serverless, databases, object storage (buckets), analytics, big data, AI, ML, message queues, key-value stores, and more.
---
title: Services Guide
toc: true
weight: 410
---
# Services Guide
Welcome to the Crossplane Services Guide! 

Crossplane Services enables managed service provisioning from `kubectl`
including for databases, caches, buckets and more, including secure usage with
Kubernetes `Secrets`.

Crossplane Service follows established Kubernetes patterns like Persistent
Volume Claims (PVC) to support dynamic provisioning of managed services and a
clean separation of concerns between app teams and cluster administrators.

In this document, we will:
 * Manually provision a new managed Kubernetes cluster and install Crossplane.
 * Learn how to provision managed services from `kubectl`.
 * Introduce cloud-specific guides with step-by-step instructions:
   * [GCP Services Guide][gcp-services-guide]
   * [AWS Services Guide][aws-services-guide]
   * [Azure Services Guide][azure-services-guide]
 * Explore how workload portability is achieved and how to configure shared clusters for multiple teams using namespaces.
 * Provide next steps for learning more about Crossplane!

We will **not**:
 * Learn first principles (see the concepts document for that level of detail)
 * Deploy Crossplane as a dedicated control plane, it will run embedded in a single Kuberetes cluster.
 * Use advanced workload scheduling or multi-cluster management.

If you have any questions, please drop us a note on [Crossplane Slack][join-crossplane-slack] or [contact us][contact-us]!

Let's go!

# Concepts
There are a bunch of things you might want to know to fully understand what's
happening in this document. This guide won't cover them, but there are other
ones that do. Here are some links!
 * [Crossplane concepts][crossplane-concepts]
 * [Kubernetes concepts][kubernetes-concepts]

# Before you get started
This guide assumes you are using a *nix-like environment. It also assumes you have a basic working familiarity with the following:
 * The terminal environment
 * Setting up cloud provider accounts for the cloud provider you want to use

You will need:
 * A *nix-like environment
 * A cloud provider account, for the cloud provider of your choice (out of the supported providers)

# Provisioning managed services from kubectl
Crossplane can be added to existing Kubernetes clusters and cleanly layers on
top of clusters provisioned by GKE, EKS, AKS, and more. Cluster administrators
install Crossplane, set cloud credentials, and offer classes of service for
self-service provisioning using `kubectl`. Application teams can provision
managed services with `Resource Claims` without having to worry about
cloud-specific infrastructure details or manage credentials.


# Overview

This guide shows how to provision a managed `MySQLInstance` and securely consume it from a Wordpress `Deployment`.

To provision a portable `MySQLInstance` for the Wordpress app we'd like to enable app teams to:

```sh
kubectl create -f mysql-claim.yaml
```
with mysql-claim.yaml:
```yaml
apiVersion: database.crossplane.io/v1alpha2
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: app-project1-dev
spec:
  classRef:
    name: mysql-standard
  writeConnectionSecretToRef:
    name: mysql-claim-secret
  engineVersion: "5.6"
```

Note there are no references in this `Resource Claim` to anything
cloud-specific. As such any environment can be configured to satisfy this claim,
using different configurations for different environments (dev, staging, prod),
or different managed service providers such as CloudSQL, RDS, or Azure DB.

This portable experience is typically accomplished by:
1. Defining **cloud-specific** `Resource Classes` in an infrastructure namespace.
1. Offering **portable** `Resource Classes` in an app project namespace for provisioning with `kubectl`.
1. Creating **portable** `Resource Claims` using `kubectl` to provision a managed service.

This enables the following usage: app -> portable claim -> portable class -> cloud-specific class -> provider.

## Steps
### A) One-time cluster setup
 1. Manually provision a managed Kubernetes target cluster: GKE, EKA, AKS.
 1. Install Crossplane into the target cluster.
 1. Install a cloud provider Stack: GCP, AWS, Azure.
 1. Connect a cloud provider account to a shared infrastructure namespace.
 1. Create cloud-specific classes of service with best-practice configurations.

### B) Onboard app projects in a shared cluster
 1. Create an app project namespace `app-project1-dev`.
 1. Add portable classes of service for managed service provisioning using `kubectl`.
 1. Set default classes of service.

### C) Deploy Wordpress with a managed MySQLInstance
 1. Provision a `MySQLInstance` using `kubectl`.
 1. Securely connect to the database using a generated Kubernetes `Secret`.
 1. Verify Wordpress is working correctly.
 1. Delete all resources.
 1. Verify everything was cleanly deleted.

## Resulting Kubernetes objects

In an AWS envionment offering multiple classes of service, the following Kubernetes objects would result:
```text
namespaces
└── aws-infra-dev
      └── provider                 # AWS provider configuration
      └── provider-creds           # AWS provider account credentials
      └── rds-mysql-standard       # RDS-specific class, non-portable config
      └── rds-mysql-replicated     # RDS-specific class, non-portable config
      └── rds-postgres-standard    # RDS-specific class, non-portable config
      └── rds-postgres-replicated  # RDS-specific class, non-portable config
└── app-project1-dev
      └── mysql-standard           # portable MySQL class of service
      └── mysql-replicated         # portable MySQL class of service
      └── postgres-standard        # portable PostgreSQL class of service
      └── postgres-ha              # portable PostgreSQL class of service
      └── mysql-claim              # portable MySQL claim for mysql-standard class of service
      └── mysql-claim-secret       # generated secret to access database
      └── wordpress-deployment     # standard Kubernetes deployment
      └── wordpress-service        # standard Kubernetes service
```

# Cloud-specific Guides
Use these step-by-step guides to provision a managed `MySQLInstance` and
securely consume it from a Wordpress `Deployment`:
 * [GCP Services Guide][gcp-services-guide]
 * [AWS Services Guide][aws-services-guide]
 * [Azure Services Guide][azure-services-guide]

# Reviewing what happened across providers
This section reviews the general flow of the cloud-specific guides, how workload
portability is achieved using resource claims and classes, and techniques to
organize a shared cluster using namespaces. 

## A) One-time cluster setup 
### Managed Kubernetes Cluster
Provision a new managed Kubernetes cluster, following the cloud-specific guides
for [GCP][gcp-services-guide], [AWS][aws-services-guide], or [Azure][azure-services-guide]

### Install Crossplane
 1. [Install Crossplane from the alpha channel][install-crossplane-alpha].
 1. [Install a cloud provider Stack][install-provider-stacks]
from the [Stacks registry][stack-registry] from one of: 
[stack-gcp][stack-gcp], [stack-aws][stack-aws], or [stack-azure][stack-azure].

### Connect Crossplane to a Cloud Provider
Crossplane supports connecting multiple cloud provider accounts from a single
cluster, so different environments (dev, staging, prod) can use separate
accounts, projects, and/or credentials.

While the guides use a single infrastructure namespace (gcp-infra-dev,
aws-infra-dev, or azure-infra-dev), you can create as many as you like using
whatever naming works best for your organization.

To connect an infrastructure namespace to a cloud provider:
 1. Create an infrastructure namespace in the Kubernetes cluster.
 1. [Obtain Cloud Provider Credentials][cloud-provider-creds]
and export to `BASE64ENCODED_PROVIDER_CREDS`.
 1. Add a Crossplane `Provider`.

For example, based on your cloud provider, add a `Provider` to your infrastructure namespace:

gcp-provider.yaml
```yaml
---
apiVersion: v1
data:
  credentials.json: $BASE64ENCODED_PROVIDER_CREDS
kind: Secret
metadata:
  name: provider-creds
  namespace: gcp-infra-dev
type: Opaque
---
## Crossplane GCP Provider
apiVersion: gcp.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: provider
  namespace: gcp-infra-dev
spec:
  credentialsSecretRef:
    name: provider-creds
    key: credentials.json
  projectID: $PROJECT_ID
```

aws-provider.yaml
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: provider-creds
  namespace: aws-infra-dev
type: Opaque
data:
  credentials: $BASE64ENCODED_PROVIDER_CREDS
---
## Crossplane AWS Provider
apiVersion: aws.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: provider
  namespace: aws-infra-dev
spec:
  credentialsSecretRef:
    key: credentials
    name: provider-creds
  region: $REGION
```

azure-provider.yaml
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: provider-creds
  namespace: azure-infra-dev
type: Opaque
data:
  credentials: $BASE64ENCODED_PROVIDER_CREDS
---
## Crossplane Azure Provider
apiVersion: azure.crossplane.io/v1alpha2
kind: Provider
metadata:
  name: provider
  namespace: azure-infra-dev
spec:
  credentialsSecretRef:
    name: provider-creds
    key: credentials
```

The `Provider` defined in the infrastructure namespace will be referenced by cloud-specific `Resource Classes` in the next step.

### Create classes of service with best-practice configurations
**Cloud-specific** `Resource Classes` capture reusable, best-practice configurations for a specific managed service.

For example, Wordpress requires a MySQL database which can be satisfied by CloudSQL, RDS, or Azure DB.

Based on your cloud provider, add a **cloud-specific** `Resource Class` to your infrastructure namespace:

rds-mysql-standard.yaml
```yaml
---
apiVersion: database.aws.crossplane.io/v1alpha2
kind: RDSInstanceClass
metadata:
  name: rds-mysql-standard
  namespace: aws-infra-dev
specTemplate:
  class: db.t2.small
  masterUsername: masteruser
  securityGroups:
   - # sg-ab1cdefg
   - # sg-05adsfkaj1ksdjak
  size: 20
  engine: mysql
  providerRef:
    name: demo
    namespace: aws-infra-dev
  reclaimPolicy: Delete
```

cloudsql--mysql-standard.yaml
```yaml
---
apiVersion: database.gcp.crossplane.io/v1alpha2
kind: CloudsqlInstanceClass
metadata:
  name: cloudsql-mysql-standard
  namespace: gcp-infra-dev
specTemplate:
  databaseVersion: MYSQL_5_6
  tier: db-custom-1-3840
  region: us-west2
  storageType: PD_SSD
  storageGB: 10
  providerRef:
    name: demo
    namespace: gcp-infra-dev
  reclaimPolicy: Delete
```

azuredb-mysql-standard.yaml
```yaml
---
apiVersion: database.azure.crossplane.io/v1alpha2
kind: SQLServerClass
metadata:
  name: azuredb-mysql-standard
  namespace: azure-infra-dev
specTemplate:
  adminLoginName: myadmin
  resourceGroupName: group-westus-1
  location: West US
  sslEnforced: false
  version: "5.6"
  pricingTier:
    tier: Basic
    vcores: 1
    family: Gen5
  storageProfile:
    storageGB: 25
    backupRetentionDays: 7
    geoRedundantBackup: false
  providerRef:
    name: demo
    namespace: azure-infra-dev
  reclaimPolicy: Delete
```

Creating multiple classes of service in an AWS environment results in these Kubernetes objects:

```text
namespaces
└── aws-infra-dev
      └── provider                 # AWS provider configuration
      └── provider-creds           # AWS provider account credentials
      └── rds-mysql-standard       # RDS-specific class, non-portable config
      └── rds-mysql-replicated     # RDS-specific class, non-portable config
      └── rds-postgres-standard    # RDS-specific class, non-portable config
      └── rds-postgres-replicated  # RDS-specific class, non-portable config
```

However, cloud-specific `Resource Classes` are not portable across providers so
we need something to represent a portable class of service for use in a portable
`Resource Claim`.

The next section covers how to offer a cloud-specific `Resource Class` as a
portable class of service, so an app team can provision managed services using
`kubectl` in a portable way.

## B) Onboard app projects in a shared cluster
### Offer Portable Classes of Service in App Project Namespaces
[Portable Resource Classes][concept-portable-class]
define a named class of service that can be used by portable `Resource Claims`
in the same namespace. When used in a project namespace, this enables the
project to provision portable managed services using `kubectl`.

```sh
kubectl create -f mysql-claim.yaml
```
with mysql-claim.yaml:
```yaml
apiVersion: database.crossplane.io/v1alpha2
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: app-project1-dev
spec:
  classRef:
    name: mysql-standard
  writeConnectionSecretToRef:
    name: mysql-claim-secret
  engineVersion: "5.6"
```
Note the portable `Resource Claim` below uses a `spec.classRef.name` of
`mysql-standard` to reference a portable `Resource Class` in the same namespace.
It has no knowledge of which cloud provider will satisfy this claim or how a
suitable cloud-specific `Resource Class` will be selected.

Adding portable classes of service to the `app-project1-dev` namespace, results in these Kubernetes objects:
```text
└── app-project1-dev
      └── mysql-standard           # portable MySQL class of service
      └── mysql-replicated         # portable MySQL class of service
      └── postgres-standard        # portable PostgreSQL class of service
      └── postgres-ha              # portable PostgreSQL class of service
```

These portable `Resource Classes` could be defined as follows for an AWS dev
environment, but alternate configurations could be provided for different
environments (staging, prod) or different cloud provider like GCP or Azure, to
satisfy the named classes of service:

mysql-standard.yaml
```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: mysql-standard
  namespace: app-project1-dev
  labels:
    default: true
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: rds-mysql-standard
  namespace: aws-infra-dev
```

mysql-replicated.yaml
```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstanceClass
metadata:
  name: mysql-replicated
  namespace: app-project1-dev
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: rds-mysql-replicated
  namespace: aws-infra-dev
```

postgres-standard.yaml
```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstanceClass
metadata:
  name: postgres-standard
  namespace: app-project1-dev
  labels:
    default: true
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: rds-postgres-standard
  namespace: aws-infra-prod
```

postgres-ha.yaml
```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: PostgreSQLInstanceClass
metadata:
  name: postgres-ha
  namespace: app-project1-dev
classRef:
  kind: RDSInstanceClass
  apiVersion: database.aws.crossplane.io/v1alpha1
  name: rds-postgres-ha
  namespace: aws-infra-prod
```

Note that some portable `Resource Classes` are marked with 
[`label.default: true`][concept-default-class]
to indicate it's the default class of service for a given claim kind in the
`app-project1-dev` namespace. 

`Resource Claims` can rely on the default class of service in the same namespace for a given claim kind by omitting `spec.classRef`.

Claim-based provisioning and use of default `Resource Classes` will be covered in the next section.

With multiple classes of service available in the `app-project1-dev` namespace, these Kuberntes objects would be present:
```text
namespaces
└── aws-infra-dev
      └── provider                 # AWS provider configuration
      └── provider-creds           # AWS provider account credentials
      └── rds-mysql-standard       # RDS-specific class, non-portable config
      └── rds-mysql-replicated     # RDS-specific class, non-portable config
      └── rds-postgres-standard    # RDS-specific class, non-portable config
      └── rds-postgres-replicated  # RDS-specific class, non-portable config
└── app-project1-dev
      └── mysql-standard           # portable MySQL class of service
      └── mysql-replicated         # portable MySQL class of service
      └── postgres-standard        # portable PostgreSQL class of service
      └── postgres-ha              # portable PostgreSQL class of service
```


## C) Deploy Wordpress with a managed MySQLInstance
### Provision a MySQLInstance from kubectl
Managed services can be provisioned in a portable way using `kubectl`, with the
`app-project1-dev` namespace populated with available classes of service.

```sh
kubectl create -f mysql-claim.yaml
```
with mysql-claim.yaml:
```yaml
apiVersion: database.crossplane.io/v1alpha2
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: app-project1-dev
spec:
  classRef:
    name: mysql-standard
  writeConnectionSecretToRef:
    name: mysql-claim-secret
  engineVersion: "5.6"
```

The `spec.classRef` can be omitted from a `Resource Claim` to rely on the
default class of service in the same namespace.
```yaml
apiVersion: database.crossplane.io/v1alpha2
kind: MySQLInstance
metadata:
  name: mysql-claim
  namespace: app-project1-dev
spec:
  writeConnectionSecretToRef:
    name: mysql-claim-secret
  engineVersion: "5.6"
```

The `Binding Status` of a `Resource Claim` will indicate `Bound` when the
underlying managed service has been provisioned and the connection secret is
available for use.

```sh
kubectl get mysqlinstances -n app-project1-dev
```
Output:
```sh
NAME          STATUS   CLASS            VERSION   AGE
mysql-claim   Bound    mysql-standard   5.6       11
```

### Securely consume the MySQLInstance from a Wordpress Deployment
```sh
kubectl create -f wordpress-app.yaml
```
with wordpress-app.yaml:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wordpress-deployment
  namespace: app-project1-dev
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
                  name: mysql-claim-secret
                  key: endpoint
            - name: WORDPRESS_DB_USER
              valueFrom:
                secretKeyRef:
                  name: mysql-claim-secret
                  key: username
            - name: WORDPRESS_DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: mysql-claim-secret
                  key: password
          ports:
            - containerPort: 80
              name: wordpress
---
apiVersion: v1
kind: Service
metadata:
  name: wordpress-service
  namespace: app-project1-dev
  labels:
    app: wordpress
spec:
  ports:
    - port: 80
  selector:
    app: wordpress
  type: LoadBalancer
```

### Cleanly Delete Wordpress and the MySQLInstance
```sh
kubectl delete -f wordpress-app.yaml
kubectl delete -f mysql-claim.yaml
```

# Summary
In this example we saw how to:
 * Add Crossplane to a managed Kubernetes cluster.
 * Install a cloud provider Stack for GCP, AWS, or Azure to add managed service provisoining.
 * Define cloud-specific classes of service in an infrastructure namespace.
 * Offer portable classes of service in an app project namespace.
 * Provision a managed MySQLInstance using kubectl.
 * Securely connect to the MySQLInstance from a Wordpress Deployment.
 * Cleanly delete all resources.

After one-time setup was done and app projects were onboarded into the shared
cluster, managed services could be provisioned using `kubectl` with portable
claims in a project namespace.

Resources were configured in infrastructure and app project namespaces:
```text
namespaces
└── aws-infra-dev
      └── provider                 # AWS provider configuration
      └── provider-creds           # AWS provider account credentials
      └── rds-mysql-standard       # RDS-specific class, non-portable config
      └── rds-mysql-replicated     # RDS-specific class, non-portable config
      └── rds-postgres-standard    # RDS-specific class, non-portable config
      └── rds-postgres-replicated  # RDS-specific class, non-portable config
└── app-project1-dev
      └── mysql-standard           # portable MySQL class of service
      └── mysql-replicated         # portable MySQL class of service
      └── postgres-standard        # portable PostgreSQL class of service
      └── postgres-ha              # portable PostgreSQL class of service
      └── mysql-claim              # portable MySQL claim for mysql-standard class of service
      └── mysql-claim-secret       # generated secret to access database
      └── wordpress-deployment     # standard Kubernetes deployment
      └── wordpress-service        # standard Kubernetes service
```

Crossplane Services brings managed service provisioning to `kubectl` and enables
cluster admins to offer multiple classes of service to accelerate app delivery
while ensuring best-practices and security in your cloud of choice. 

Claim-based provisioning supports portability into different cloud environments
since the app only depends on named or default classes of service that can
provide wire-compatible managed services (MySQL, PostgreSQL, Redis, and more)
independent of how a given cloud provider satisfies the claim. Claim-based
provisioning also supports differentiated cloud services, so all managed
services can work with Crossplane.

If you have any questions, please drop us a note on [Crossplane Slack][join-crossplane-slack] or [contact us][contact-us]!

# Learn More
This guide covered deploying Crossplane into a single managed Kubernetes
cluster, and using cloud provider Stacks to provision a managed MySQL instance for 
use with a Wordpress Deployment. However, this involved configuring multiple
Kubernetes objects to get a fully functioning Wordpress instance securely
deployed.

Stacks can also be used to simplify app management and automate operations. Our
next guide shows how an App Stack can automate most of the steps covered in this
guide and be run from a dedicated control plane that: (a) dynamically provisions
the target cluster, (b) provisions the managed services, and (c) deploys the app
itself with secure connectivity.

App Stacks simplify operations for an app by moving the steps covered in this guide into a Kubernetes controller that owns an app CRD (custom resource definition) with a handful of settings required to deploy a new app instance, complete with the managed services it depends on.

## Next Steps
* [Crossplane Stacks Guide][stack-user-guide] to deploy the same Wordpress instance with a
  single yaml file, using the [portable Wordpress App Stack][stack-wordpress].
* [Extend a Stack][stack-developer-guide] to add more cloud services to:
  [stack-gcp][stack-gcp], [stack-aws][stack-aws], or [stack-azure][stack-azure].
* [Build a new Stack][stack-developer-guide] to add more cloud providers or
  independent cloud services.

If you have any questions, please drop us a note on [Crossplane Slack][join-crossplane-slack] or [contact us][contact-us]!

## References
### Concepts
* [Crossplane Concepts][crossplane-concepts]
* [Claims][concept-claim]
* [Classes][concept-class]
* [Portable Classes][concept-portable-class]
* [Default Classes][concept-default-class]
* [Workloads][concept-workload]
* [Stacks][concept-stack]
* [Stacks Design][stack-design]
* [Stacks Manager][stack-manager]
* [Stacks Registry][stack-registry]
* [Stack Install Flow][stack-install-docs]
* [Stack Package Format][stack-format-docs]

### Getting Started
* [Install Crossplane][install-crossplane]
* [Install Provider Stacks][install-provider-stacks]
* [Cloud Provider Credentials][cloud-provider-creds]
* [Crossplane CLI][crossplane-cli]
* [Crossplane CLI Docs][crossplane-cli-docs]

**GCP**
* [GCP Services Guide][gcp-services-guide]
* [GCP Stack][stack-gcp]
* [GCP Docs][gcp-docs]

**AWS**
* [AWS Services Guide][aws-services-guide]
* [AWS Stack][stack-aws]
* [AWS Docs][aws-docs]

**AWS**
* [Azure Services Guide][azure-services-guide]
* [Azure Stack][stack-azure]
* [Azure Docs][azure-docs]

### Using and Building Stacks
* [Stacks Guide][stack-user-guide]
* [Stacks Developer Quick Start][stack-quick-start]
* [Stacks Developer Guide][stack-developer-guide]

### Kubernetes
* [Kubernetes Concepts][kubernetes-concepts]
* [Kubernetes Docs][kubernetes-docs]
* [kubectl docs][kubectl-docs]

### Learn More
* [Join Crossplane Slack][join-crossplane-slack]
* [Contact Us][contact-us]
* [Learn More][learn-more]

<!-- Named links -->
[crossplane-concepts]: concepts.md
[concept-claim]: concepts.md#resource-claims-and-resource-classes
[concept-class]: concepts.md#resource-claims-and-resource-classes
[concept-workload]: concepts.md#resources-and-workloads
[concept-stack]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks
[concept-portable-class]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-default-resource-class.md#proposal-default-class-reference-v2--claim-portability
[concept-default-class]: https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-default-resource-class.md#denote-default-via-label

[kubernetes-concepts]: https://kubernetes.io/docs/concepts/
[kubernetes-docs]: https://kubernetes.io/docs/home/
[kubectl-docs]: https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands

[install-crossplane]: install-crossplane.md
[install-crossplane-alpha]: install-crossplane.html#alpha
[install-provider-stacks]: install-crossplane.md#installing-cloud-provider-stacks
[cloud-provider-creds]: cloud-providers.md

[crossplane-cli]: https://github.com/crossplaneio/crossplane-cli
[crossplane-cli-docs]: https://github.com/crossplaneio/crossplane-cli/blob/master/README.md

[stack-quick-start]: https://github.com/crossplaneio/crossplane-cli#quick-start-stacks
[stack-registry]: https://hub.docker.com/search?q=crossplane&type=image
[stack-design]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#crossplane-stacks
[stack-manager]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#terminology
[stack-install-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#installation-flow
[stack-format-docs]: https://github.com/crossplaneio/crossplane/blob/master/design/design-doc-stacks.md#stack-package-format
[stack-user-guide]: stacks-guide.md
[stack-developer-guide]: developer-guide.md
[contact-us]: https://github.com/crossplaneio/crossplane#contact
[join-crossplane-slack]: https://slack.crossplane.io

[stack-gcp]: https://github.com/crossplaneio/stack-gcp
[stack-aws]: https://github.com/crossplaneio/stack-aws
[stack-azure]: https://github.com/crossplaneio/stack-azure
[stack-wordpress]: https://github.com/crossplaneio/sample-stack-wordpress

[gcp-services-guide]: services/gcp-services-guide.md
[aws-services-guide]: services/aws-services-guide.md
[azure-services-guide]: services/azure-services-guide.md

[aws-docs]: https://docs.aws.amazon.com/
[gcp-docs]: https://cloud.google.com/docs/
[azure-docs]: https://docs.microsoft.com/azure/

[learn-more]: learn-more.md

---
title: Getting Started
toc: true
weight: 210
---
# Getting Started

This guide will demonstrate using Crossplane to deploy a portable MySQL database
on the Google Cloud Platform (GCP). It serves as an initial introduction to
Crossplane, but only displays a small set of its features.

In this guide we will:

1. [Install Crossplane](#install-crossplane)
1. [Add your GCP project to Crossplane](#add-your-gcp-project-to-crossplane)
1. [Provision a MySQL instance using CloudSQL](#provision-a-mysql-instance)
1. [Define a class of CloudSQL instance for dynamic provisioning](#define-a-class-of-cloudsql-instance)

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

We've now installed Crossplane with GCP support! Take a look at the guide to
[Crossplane installation guide] for more installation options, and to learn how
to install support for other cloud providers such as Amazon Web Services and
Microsoft Azure.

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
kubectl -n crossplane-system create secret example-gcp-credentials --from-file=credentials.json
kubectl apply -f provider.yaml
```

Crossplane can now manage your GCP project! Your service account will need the
CloudSQL Admin role for this guide. Check out GCP's [Getting Started With
Authentication] guide if you need help creating a service account and
downloading its `credentials.json` file, and Crossplane's [GCP provider
documentation] for detailed instructions on setting up your project and service
account permissions.

## Provision a MySQL Instance

GCP provides MySQL databases using [CloudSQL] instances. Crossplane uses a
resource and claim pattern to provision and manage cloud resources like CloudSQL
instances - if you've ever used [persistent volumes in Kubernetes] you've seen
this pattern before. The simplest way to start using a new MySQL instance on GCP
is to provision a `CloudSQLInstance`, then claim it via a `MySQLInstance`. We
call this process _static provisioning_.


```yaml
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstance
metadata:
  name: example-cloudsql-instance
spec:
  providerRef:
    name: example-provider
  writeConnectionSecretToRef:
    name: example-cloudsql-connection-details
    namespace: crossplane-system
  forProvider:
    databaseVersion: MYSQL_5_6
    region: us-west2
    settings:
      tier: db-n1-standard-1
      dataDiskType: PD_SSD
      dataDiskSizeGb: 10
      ipConfiguration:
        ipv4Enabled: true
```

First we create a CloudSQL instance. Save the above as `cloudsql.yaml`, then
apply it:

```bash
kubectl apply -f cloudsql.yaml
```

Crossplane is now creating the `CloudSQLInstance`! Before we can use it, we need
to claim it.

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: example-mysql-claim
spec:
  resourceRef:
    apiVersion: database.gcp.crossplane.io/v1beta1
    kind: CloudSQLInstance
    name: example-cloudsql-instance
  writeConnectionSecretToRef:
    name: example-mysql-connection-details
```

Save the above as `mysql.yaml`, and once again apply it:

```bash
kubectl --namespace default apply -f mysql.yaml
```

In Crossplane cloud provider specific resources like the `CloudSQLInstance` we
created above are called _managed resources_. They're considered infrastructure,
like a Kubernetes `Node` or `PersistentVolume`. Managed resources exist at the
cluster scope (they're not namespaced) and let you specify nitty-gritty provider
specific configuration details. Managed resources that have reached `v1beta1`
are a high fidelity representation of their underlying cloud provider resource,
and can be updated to change their configuration after provisioning. We _claim_
these resources by submitting a _resource claim_ like the `MySQLInstance` above.
Resource claims are namespaced, and indicate that the managed resource they
claim is in use by _binding_ to it. You can also use resource claims to
_dynamically provision_ managed resources on-demand - we'll discuss that in the
next section of this guide.

Soon your new `MySQLInstance` should be online. You can use `kubectl` to
inspect its status. If you see `Bound` under the `STATUS` column, it's ready to
use!

```bash
$ kubectl --namespace default get mysqlinstance example-mysql-claim
NAME                  STATUS   CLASS-KIND   CLASS-NAME   RESOURCE-KIND      RESOURCE-NAME               AGE
example-mysql-claim   Bound                              CloudSQLInstance   example-cloudsql-instance   4m
```

You'll find all the details you need to connect to your new MySQL instance saved
in the Kubernetes `Secret` you specified via `writeConnectionSecretToRef`, ready
to [use with your Kubernetes pods].

```bash
$ kubectl --namespace default describe secret example-mysql-connection-details
Name:         example-mysql-connection-details
Namespace:    default
Type:  Opaque

Data
====
serverCACertificateCommonName:        98 bytes
serverCACertificateInstance:          25 bytes
username:                             4 bytes
password:                             27 bytes
publicIP:                             13 bytes
serverCACertificateCertSerialNumber:  1 bytes
serverCACertificateCreateTime:        24 bytes
serverCACertificateExpirationTime:    24 bytes
serverCACertificateSha1Fingerprint:   40 bytes
endpoint:                             13 bytes
serverCACertificateCert:              1272 bytes
```

That's all there is to static provisioning with Crossplane! We've created a
`CloudSQLInstance` as cluster scoped infrastructure, then claimed it as a
`MySQLInstance`. You can use `kubectl describe` to view the detailed
configuration and status of your `CloudSqlInstance`.

```bash
$ kubectl describe example-cloudsql-instance
Name:         example-cloudsql-instance
Annotations:  crossplane.io/external-name: example-cloudsql-instance
API Version:  database.gcp.crossplane.io/v1beta1
Kind:         CloudSQLInstance
Spec:
  For Provider:
    Database Version:  MYSQL_5_6
    Gce Zone:          us-west2-b
    Instance Type:     CLOUD_SQL_INSTANCE
    Region:            us-west2
    Settings:
      Activation Policy:  ALWAYS
      Backup Configuration:
        Start Time:       17:00
      Data Disk Size Gb:  10
      Data Disk Type:     PD_SSD
      Ip Configuration:
        ipv4Enabled:  true
      Location Preference:
        Zone:               us-west2-b
      Pricing Plan:         PER_USE
      Replication Type:     SYNCHRONOUS
      Storage Auto Resize:  true
      Tier:                 db-n1-standard-1
  Provider Ref:
    Name:  example-provider
  Write Connection Secret To Ref:
    Name:       example-cloudsql-connection-details
    Namespace:  crossplane-system
Status:
  At Provider:
    Backend Type:     SECOND_GEN
    Connection Name:  my-cool-gcp-project:us-west2:example-cloudsql-instance
    Gce Zone:         us-west2-b
    Ip Addresses:
      Ip Address:                   8.8.8.8
      Type:                         PRIMARY
    Project:                        my-cool-gcp-project
    Self Link:                      https://www.googleapis.com/sql/v1beta4/projects/my-cool-gcp-project/instances/example-cloudsql-instance
    Service Account Email Address:  REDACTED@gcp-sa-cloud-sql.iam.gserviceaccount.com
    State:                          RUNNABLE
  Binding Phase:                    Bound
  Conditions:
    Last Transition Time:  2019-10-25T08:09:16Z
    Reason:                Successfully reconciled managed resource
    Status:                True
    Type:                  Synced
    Last Transition Time:  2019-10-25T08:09:12Z
    Reason:                Successfully resolved managed resource references to other resources
    Status:                True
    Type:                  ReferencesResolved
    Last Transition Time:  2019-10-25T08:09:16Z
    Reason:                Managed resource is available for use
    Status:                True
    Type:                  Ready
```

Pay attention to the `Ready` and `Synced` conditions above. `Ready` represents
the availability of the CloudSQL instance while `Synced` reflects whether
Crossplane is successfully applying your specified CloudSQL configuration.

## Define a Class of CloudSQL Instance

Now that we've learned how to statically provision and claim managed resources
it's time to try out _dynamic provisioning_. Dynamic provisioning allows us to
define a class of managed resource - a _resource class_ - that will be used to
automatically satisfy resource claims when they are created.

Here's a resource class that will dynamically provision Cloud SQL instances with
the same settings as the `CloudSqlInstance` we provisioned earlier in the guide:

```yaml
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstanceClass
metadata:
  name: example-cloudsql-class
  annotations:
    resourceclass.crossplane.io/is-default-class: "true"
  labels:
    guide: getting-started
specTemplate:
  providerRef:
    name: example
  writeConnectionSecretsToNamespace: crossplane-system
  forProvider:
    databaseVersion: MYSQL_5_6
    region: us-west2
    settings:
      tier: db-n1-standard-1
      dataDiskType: PD_SSD
      dataDiskSizeGb: 10
      ipConfiguration:
        ipv4Enabled: true
```

Save the above as `cloudsql-class.yaml` and apply it to enable dynamic
provisioning of `CloudSqlInstance` managed resources:

```bash
kubectl apply -f cloudsql-class.yaml
```

Now you can omit the `resourceRef` when you create resource claims. Save the
below resource claim as `mysql-dynamic-claim.yaml`:

```yaml
apiVersion: database.crossplane.io/v1alpha1
kind: MySQLInstance
metadata:
  name: example-mysql-dynamic-claim
spec:
  classSelector:
    matchLabels:
      guide: getting-started
  writeConnectionSecretToRef:
    name: example-mysql-dynamic-connection-details
```

When you apply this `MySQLInstance` claim you'll see that it dynamically
provisions a new `CloudSQLInstance` to satisfy the resource claim:

```bash
$ kubectl --namespace default apply -f mysql-dynamic-claim.yaml
mysqlinstance.database.crossplane.io/example-mysql-dynamic-claim created

$ kubectl get mysqlinstance example-mysql-dynamic-claim
NAME                          STATUS   CLASS-KIND              CLASS-NAME               RESOURCE-KIND      RESOURCE-NAME                               AGE
example-mysql-dynamic-claim            CloudSQLInstanceClass   example-cloudsql-class   CloudSQLInstance   default-example-mysql-dynamic-claim-bwpzd   47s
```

You just dynamically provisioned a `CloudSQLInstance`! You can find the name of
your new `CloudSQLInstance` under the `RESOURCE-NAME` column when you run
`kubectl describe mysqlinstance`. Reuse the resource class as many times as you
like; simply submit more `MySQLInstance` resource claims to create more CloudSQL
instances.

You may have noticed that your resource claim included a `classSelector`. The
class selector lets you select which resource class to use by [matching its
labels]. Resource claims like `MySQLInstance` can match different kinds of
resource class using label selectors, so you could just as easily use the
exact same `MySQLInstance` to create an Amazon Relational Database Service (RDS)
instance by creating an `RDSInstanceClass` labelled as `guide: getting-started`.
When multiple resource classes match the class selector, a matching class is
chosen at random. Claims can be matched to classes by either:

* Specifying a `classRef` to a specific resource class.
* Specifying a `classSelector` that matches one or more resource classes.
* Omitting both of the above and defaulting to a resource class [annotated] as
  `resourceclass.crossplane.io/is-default-class: "true"`.

## Next Steps

* Add additional [cloud provider stacks](cloud-providers.md) to Crossplane.
* Explore the [Services Guide](services-guide.md) and the [Stacks Guide](stacks-guide.md).
* Learn more about [Crossplane concepts](concepts.md).
* See what managed resources are [currently supported](api.md) for each provider.
* Build [your own stacks](developer-guide.md)!

<!-- Named Links -->

[Helm]: https://helm.sh
[minikube]: https://kubernetes.io/docs/tasks/tools/install-minikube/
[kind]: https://github.com/kubernetes-sigs/kind
[stack]: concepts.md#stacks
[Crossplane installation guide]: install-crossplane.md
[Getting Started With Authentication]: https://cloud.google.com/docs/authentication/getting-started
[GCP provider documentation]: gcp-provider.md
[CloudSQL]: https://cloud.google.com/sql/docs/mysql/
[Persistent volumes in Kubernetes]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
[use with your Kubernetes pods]: https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets
[matching its labels]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
[annotated]: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/

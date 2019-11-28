# Configuration Stacks

* Owner: Muvaffak Onus (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Terminology

* _Custom Resource Definition (CRD)_. A Kubernetes type that defines a new type
  of resource that can be managed declaratively. This serves as the unit of
  management in Crossplane.
* _Custom Resource (CR)_. A Kubernetes resource that is an instance of the type
  that is introduced via a specific CRD.
* _Stack_. A unit of extension of capabilities of Crossplane. A stack can consist
  of new CRDs, controllers to manage them and related metadata.
* _Resource Claim_. The Crossplane representation of a request for the allocation of a managed resource.
  Resource claims typically represent the need for a managed resource that implements a particular protocol.
  MySQLInstance and RedisCluster are examples of resource claims.
* _Resource Class_. The Crossplane representation of the desired configuration of a managed resource.
  Claims reference a resource class in order to specify how they should be satisfied by a managed resource.
* _External Resource_. An actual resource that exists outside Kubernetes, typically in the cloud.
  AWS RDS and GCP Cloud Memorystore instances are external resources.
* _Managed Resource_. The Crossplane representation of an external resource.
  The RDSInstance and CloudMemorystoreInstance Kubernetes kinds are managed resources.

## Background

Crossplane uses a [class and claim] model to provision and manage resources in
an external system, such as a cloud provider. _External resources_ in the
provider's API are modelled as _managed resources_ in the Kubernetes API server.
Managed resources are considered the domain of _infrastructure operators_;
they're cluster scoped infrastructure like a `Node` or `PersistentVolume`.
_Application operators_ may claim a managed resource for a particular purpose by
creating a namespaced _resource claim_. Managed resources may be provisioned
explicitly before claim time (static provisioning), or automatically at claim
time (dynamic provisioning). The initial configuration of dynamically
provisioned managed resources is specified by a _resource class_.

In most of the cases, the resources have to refer to each other in order to be
connected at the cloud provider level. For example, let's say you have an application
like Wordpress that you want to deploy to a Kubernetes cluster and have its database
to be managed by the cloud provider. While you can connect from your cluster to
the database over the internet, it's usually desirable to have both of them in a
private network to connect to each other and expose only your application's IP
to the world. There are usually a few resources that you need to create to achieve
this like VPC, sub-network, internet gateway etc. After creating these resources,
you need to refer to them in your resource classes that you author for database
and cluster claims to use.

Today, it's possible to do all this by manually configuring each resource YAML
files and run `kubectl apply -f <directory of YAMLs>`. However, there are two areas
that we can improve during this process:
* The preparation phase of these YAMLs requires to be very careful and changing
  something usually means touching a few different files. While this may not seem
  like a big hurdle, changing some high level parameters of that environment is
  cumbersome _after_ it's deployed.
* A user who is familiar with AWS may not be familiar with Azure or GCP; so,
  ready-made configuration stacks that promise the exact same setup would be
  useful for them to see how they can achieve similar setups and run various
  benchmarks according to their business needs.
* After you run `kubectl apply -f <directory of YAMLs>`, a dependency graph of
  resources starts to resolve. However, it's not always clear for user to know _when_
  everything is actually ready to use unless they know readiness of which resources
  signal the readiness of the whole environment.

## Goals

The main goal of this design is to make it easier for people to write sets of
configurations and be able to manage those configuration sets on a higher level.

It is important that the design puts forward:

* A base boilerplate package where creating a new configuration set that has a
  controller is as easy as changing the YAML files.
* A way to declare pre-requisites that needs to be satisfied by the cluster
  for the configuration set to work.
* A way to change a high level configuration by changing 1 value that is then
  translated to the YAMLs that are already deployed.

## Proposal

Crossplane has the notion of stacks that can be used as extension to Crossplane's
abilites, to deploy an application and all other things that you can do with
Kubernetes controllers since main components of stacks are CRDs and controllers
watching them.

To achieve the mentioned goals, we can publish the YAML files in a stack with a
controller that has its own CRD with high level parameters. Controller deploys
and updates the YAMLs that are packaged in the stack image according to user's
input on the CR instance of that stack that user creates.

As sky is the limit when it comes to stacks, we need to put down some general
rules to be adhered by initial configuration stacks.

* CRDs of the resources that will be deployed by the stack are declared as
  dependency so that user knows what CR instances will be deployed by the given
  configuration stack.
* It should be possible to have different sets of configurations in one stack
  and user should be able to choose from them via the CR that is an instance
  of stack's CRD.
  * Note that it's highly preferred to have 1 custom resource definition that
    the stack uses and expose the high level variations of the parameters through
    `spec` instead of having different custom resource definitions for each class
    of configuration.
* A minimal high level set of configurations are exposed to the user through
  the stack's CR, such as `Provider` reference to refer in the deployed resource
  or `region`.
* Controller should signal readiness of all resources in CR's status.
* All resources should be labelled referring to the CR instance of the stack.
* In YAMLs, resources should refer to each other using cross-resource references
  wherever possible.
* **TBD** Controller should update the resources continuously and treat the given CR as
  the source of truth even though user manually changes the resources that are
  deployed by the stack.
* **TBD** CRD of the configuration stack should be namespaced so that different
  teams/users can create their own configured environment.
* Stack type will be `ClusterStack` as most of the resources it deploys are
  cluster-scoped.

Name of the concept is **TBD**; _EasyStack_, _ConfigurationStack_,
_EnvironmentStack_ are current candidates.

## User Experience

1. Install AWS stack and create your `Provider` object named `aws-provider`.
2. Install a AWS configuration stack via:
```yaml
# exact apiVersion is TBD.
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: easy-aws
  namespace: crossplane-system
spec:
  package: "crossplane/easy-aws:latest"
```

Now, I want an environment where all my database instances and kubernetes clusters
are connected to each other in a private VPC, which what this specific configuration
stack does. Create the following:

```yaml
# exact apiVersion is TBD.
apiVersion: aws.configurationstacks.crossplane.io/v1alpha1
kind: EasyAWS
metadata:
  name: project-future
  namespace: dev
spec:
  providerRef:
    name: aws-provider
```

Then I wait for status to become ready. After it's done, all resources are deployed.
```yaml
# exact apiVersion is TBD.
apiVersion: aws.configurationstacks.crossplane.io/v1alpha1
kind: EasyAWS
metadata:
  name: project-future
  namespace: dev
spec:
  providerRef:
    name: aws-provider
status:
  state: ready
```

An example deployed resource would be:
```yaml
apiVersion: database.aws.crossplane.io/v1beta1
kind: RDSInstanceClass
metadata:
  name: dev-project-future-mysql
  labels:
    "aws.configurationstacks.crossplane.io/name": project-future
    "aws.configurationstacks.crossplane.io/namespace": dev
    "aws.configurationstacks.crossplane.io/uid": 1ca16960-973b-4d58-a6fa-5696638ec631
specTemplate:
  writeConnectionSecretsToNamespace: crossplane-system
  forProvider:
    dbInstanceClass: db.t2.small
    masterUsername: root
    vpcSecurityGroupIDRefs:
      - name: dev-project-future-rds-security-group
    dbSubnetGroupNameRef:
      name: dev-project-future-dbsubnetgroup
    allocatedStorage: 20
    engine: mysql
    skipFinalSnapshotBeforeDeletion: true
  providerRef:
    name: aws-provider
  reclaimPolicy: Delete
```

Other people in different namespaces can create their own instances of `EasyAWS`
custom resource and have their own similar environment with different names.

## Technical Implementation

### Metadata of The Stack

[Dependency list] has to be filled with each kind of the resource that the stack
deploys, i.e. f the stack only deploys `RDSInstanceClass` resource, it should
not require all AWS stack to be present.

[UI annotations] should be present to make it easy for frontend software to process
the stack.

### Folder Structure

All directories containing base YAML files should be in a top-level directory
of the repository named `resources` so that it's easier for others to fork & change.

Controller will scan this folder recursively to get all YAML files. So, the folder
structure inside that `resources` directory won't matter to the controller but
it's good to separate the folders cleanly so that it's easier to fork & change.

There are mainly 2 types of resources that configuration stacks will usually
need to deploy; resource classes and managed resources. Under `resources` folder
we can have 2 separate folders for each of those resource types; `classes` and
`services`. Under these folders, resources should be in directories recursively
named after their group and kind. An example folder structure:

```
├── resources
│   ├── classes
│   │   ├── cache
│   │   │   └── replicationgroup.yaml
│   │   ├── compute
│   │   │   └── ekscluster.yaml
│   │   └── database
│   │       ├── rdsinstancemysql.yaml
│   │       └── rdsinstancepostgresql.yaml
│   └── services
│       ├── database
│       │   └── dbsubnetgroup.yaml
│       ├── identity
│       │   ├── iamrole.yaml
│       │   └── iamrolepolicyattachment.yaml
│       └── network
│           ├── internetgateway.yaml
│           ├── routetable.yaml
│           ├── securitygroup.yaml
│           ├── subnet.yaml
│           └── vpc.yaml
```

Example above is a suggestion for stacks with one tier of configuration.
There could also be cases where one configuration stack has the YAML files for
different tiers and a `spec` allows to deploy one of them, in that case it'd make
sense for developer to have a folder for each tier under `resources` folder.

### Reconciliation

There are mainly 3 responsibility of the controller:
* When all deployed resources are ready, `status.state` should be `ready`.
  * For resource classes; if the resource exists in the api-server it's considered
    to be ready.
  * For managed resources; if the `Ready` condition status is `true`, then it's
    considered to be ready.
  * For other possible resources, the developer chooses what to look for as
    readiness.
* Apply; continuously reproduce the resources from YAMLs and apply it.
* Deletion: should be done via label selector when the CR is deleted.

In the `Apply` phase, there will be some processing phasse before calling `Create`
for the resources:

1. Names of the resources should be converted to the following format
  `<CR namespace>-<CR name>-<Name given in YAML>` to prevent name collisions
  since most of the resources stack deploys are cluster-scoped.
  So, the name that appears on YAML files are valid for referencing each other
  while building the stack but in actual operation, do not expect exact names
  to appear.
  * This will require cross-resource references to be scanned and replaced with
    the new name by the stack controller.
2. All resources will be labelled with `name`, `namespace` and `uid` of the stack's CR instance.
3. If there is a high level configuration in `spec` to be applied, it will be. The
  mapping of that configuration field to each resource is coded in the controller.

## Alternatives Considered

* Helm chart or kustomize.
  * No reconciliation to change a top level parameter.
  * No go-to place to see the readiness of the whole environment deployed.
  * Stacks provide a better interface to users by allowing to have the same
    environment by just creating a CR instance instead of having everything in
    local.
  * Stacks do a better job for declaring CRD dependencies and UI annotation metadata.

[Dependency list]: https://github.com/crossplaneio/crossplane/blob/98e8520e2a2285cd6944fcd67fbef427299891e8/design/design-doc-stacks.md#stack-crd
[UI annotations]: https://github.com/crossplaneio/crossplane/blob/5758662818fc1e840adbfbf1a9fb37b87c3d5a5c/design/one-pager-stack-ui-metadata.md
[class and claim]: https://static.sched.com/hosted_files/kccncna19/2d/kcconna19-eric-tune.pdf
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
files and run `kubectl apply -f <directory of YAMLs>`. However, there are a few
areas that we can improve during this process:
* The preparation phase of these YAMLs requires a user to be very careful and
  changing something usually means touching a few different files. While this
  may not seem like a big hurdle, changing some high level parameters of that
  environment is cumbersome _after_ it's deployed.
* A user who is familiar with AWS may not be familiar with Azure or GCP. So,
  ready-made configuration stacks that promise a very similar setup would be
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

* A base boilerplate tooling where creating a new configuration set that has a
  controller is as easy as changing the YAML files.
* An easy way for users to deploy a set of pre-defined configurations that has
  a controller which reconciles the set of the resources deployed.

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

* It should be possible to have different sets of configurations in one stack
  and user should be able to choose from them via the CR that is an instance
  of stack's CRD.
  * Note that it's highly preferred to have only one custom resource definition that
    the stack uses. You can expose the high level variations of the parameters
    through `spec` instead of having different custom resource definitions in the
    same stack or you can write another stack with its own CRD for that purpose.
* A minimal high level set of configurations are exposed to the user through
  the stack's CR, such as reference to the cloud credential secret or `region`.
* All resources should be labelled referring to the CR instance of the stack.
* In YAMLs, resources should refer to each other using cross-resource references
  wherever possible.
* Controller should update the resources continuously and treat the given CR as
  the source of truth even though user manually changes the resources that are
  deployed by the stack.
* CRD of the configuration stack should be cluster-scoped since the author of
  the CR instances of the configuration stack is assumed to be the infrastructure
  owner.
* Stack type will be `ClusterStack` as most of the resources it deploys are
  cluster-scoped.
* CRD of the configuration stack should have a boolean spec field called
  `keepDefaultingLabels` which indicates what happens to the resource classes that
  are marked default.
  * By default, the zero-value of the boolean is _false_. Meaning, if user is
    indifferent we remove that special default label from all resource classes
    to avoid conflict and unexpected randomization between resource classes.
  * Users who deploy the CR will need to mark this field as _true_ if they'd like
    to keep the defaults, which is expected to appear on only one configuration
    stack CR in the cluster.
  * See details about [defaulting mechanism here].
* All the labels in the configuration stack CR is propagated down to all resources
  that it deploys.

## User Experience

1. Install GCP stack and create a secret that contains GCP credentials.
2. Install a GCP configuration stack via:
```yaml
# exact apiVersion is TBD.
apiVersion: stacks.crossplane.io/v1alpha1
kind: ClusterStackInstall
metadata:
  name: minimal-gcp
  namespace: crossplane-system
spec:
  package: "crossplane/minimal-gcp:latest"
```

Now, I want an environment where all my database instances and kubernetes clusters
are connected to each other in a private VPC, which what this specific configuration
stack does. Create the following:

```yaml
apiVersion: gcp.configurationstacks.crossplane.io/v1alpha1
kind: MinimalGCP
metadata:
  name: small-infra
  labels:
      "foo-key": bar-value
spec:
  region: us-west2
  projectID: foo-project
  keepDefaultingAnnotations: true
  credentialsSecretRef:
    name: gcp-credentials
    namespace: crossplane-system
    key: credentials
```

Then I wait for `Synced` condition to become `true`. After it's done, all resources
are deployed.
```yaml
apiVersion: gcp.configurationstacks.crossplane.io/v1alpha1
kind: MinimalGCP
metadata:
  name: small-infra
  labels:
    "foo-key": bar-value
spec:
  region: us-west2
  projectID: foo-project
  keepDefaultingAnnotations: true
  credentialsSecretRef:
    name: gcp-credentials
    namespace: crossplane-system
    key: credentials
status:
  conditions:
  - lastTransitionTime: "2019-12-03T23:16:58Z"
    reason: Successfully reconciled
    status: "True"
    type: Synced
```

An example deployed resource would be:
```yaml
apiVersion: database.gcp.crossplane.io/v1beta1
kind: CloudSQLInstanceClass
metadata:
  labels:
    # All labels on MinimalGCP are propagated down to all resources.
    "foo-key": bar-value
    # Default labels for all deployed resources.
    gcp.resourcepacks.crossplane.io/name: minimal-setup
    gcp.resourcepacks.crossplane.io/uid: 34646233-f58e-4c99-b0a8-0d766533b12c
  annotations:
    # The defaulting annotation is kept since keepDefaultingAnnotations was true.
    # Otherwise, it'd have been removed.
    resourceclass.crossplane.io/is-default-class: "true"
  name: minimal-setup-cloudsqlinstance-mysql
  ownerReferences:
  # Once the referred MinimalGCP instance is deleted, this resource will be
  # deleted by Kubernetes api-server.
  - apiVersion: gcp.resourcepacks.crossplane.io/v1alpha1
    kind: MinimalGCP
    name: minimal-setup
    uid: 34646233-f58e-4c99-b0a8-0d766533b12c
specTemplate:
  forProvider:
    databaseVersion: MYSQL_5_7
    # Propagated from MinimalGCP instance.
    region: us-west2
    settings:
      dataDiskSizeGb: 10
      dataDiskType: PD_SSD
      ipConfiguration:
        ipv4Enabled: false
        privateNetworkRef:
          # Hard-coded value was "network" but since Network with name "network"
          # has a new name, the ref here is also updated.
          name: minimal-setup-network
      tier: db-n1-standard-1
  providerRef:
    name: minimal-setup-gcp-provider
  reclaimPolicy: Delete
  writeConnectionSecretsToNamespace: crossplane-system

```

There could be several instances of `MinimalGCP` custom resource and each would 
have their own similar environment with resources that have different names.

## Technical Implementation

Reconciliation will mainly consist of the following steps:
1. A `Kustomization` overlay object will be generated that looks like the following:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namePrefix: <CRNAME>-
commonLabels:
  "gcp.configurationstacks.crossplane.io/name": <CRNAME>
  "gcp.configurationstacks.crossplane.io/uid": <CRUID>
```

2. Call custom patch functions that consumer of the reconciler provided in order
   to make changes on `Kustomization` object.
3. Call Kustomize and generate the resources.
4. Read the stream of YAMLs from kustomize output.
5. Call custom patch functions that consumer of the reconciler provided in order
   to make changes on the generated resources before deployment.
6. _Apply_ all resources. Set reconciliation condition to success if no error is
   present.

Note that if the CR has deletion timestamp, we do not reconcile at all, letting
Kubernetes garbage collection take care of the deletion of the resources.

All resource YAMLs will exist in the top directory called `resources`, which looks
like the following:

```
├── resources
│   ├── gcp
│   │   ├── cache
│   │   │   ├── cloudmemorystoreinstance.yaml
│   │   │   ├── kustomization.yaml
│   │   │   └── kustomizeconfig.yaml
│   │   ├── compute
│   │   │   ├── gkeclusterclass.yaml
│   │   │   ├── globaladdress.yaml
│   │   │   ├── kustomization.yaml
│   │   │   ├── kustomizeconfig.yaml
│   │   │   ├── network.yaml
│   │   │   └── subnetwork.yaml
│   │   ├── database
│   │   │   ├── cloudsqlinstanceclass.yaml
│   │   │   ├── kustomization.yaml
│   │   │   └── kustomizeconfig.yaml
│   │   ├── kustomization.yaml
│   │   ├── provider.yaml
│   │   └── servicenetworking
│   │       ├── connection.yaml
│   │       ├── kustomization.yaml
│   │       └── kustomizeconfig.yaml
│   └── kustomization.yaml
```

As you see, we follow `resources/{cloud provider}/{group}/{kind}.yaml` where all
resources of same kind are present in the same YAML.

Example above is a suggestion for stacks with one tier of configuration.
There could also be cases where one configuration stack has the YAML files for
different tiers and a `spec` allows to deploy one of them, in that case it'd make
sense for developer to have a folder for each tier under `resources` folder. It's
basically up to you to try various structures as long as kustomize is able to
work through your structure.

Note that nothing in `resources` folder is changed. The reconciler generates a
new overlay with its own `kustomization.yaml` in a temporary directory and refers
to the `resources` folder.

[UI annotations] should be present to make it easy for frontend software to process
the stack.

### Custom Patchers

The configuration stack reconciler that has two types of patcher functions where
developer can intercept the reconciler flow and provide their own logic:
* `KustomizationPatcher`: Its signature includes the generic `ParentResource` object
  that represents the stack CR and `Kustomization` object that represents the
  overlay `kustomization.yaml` file.
  * Developers who want to make additions to the default `kustomization.yaml` file
    with data from runtime can provide their own patchers to the pipeline.
* `ChildResourcePatcher`: Its signature includes the generic `ParentResource` object
  as well as the list of the generated `ChildResource`s that will be deployed.
  * Developers who'd like to make changes to the resources generated via `kustomize`
    will provide their own functions.

Configuration stack reconciler will have default patchers for the functionality
that is expected to be common for all configuration stacks such as label propagation
from stack CR to deployed resources.

### Referencing

We will use Kustomize [custom transformer configurations] to achieve the referencing
behaviors.

It's developer's responsibility to declare the reference dependencies between
the resources. Kustomize supports the following referencers as of writing:
* Name Referencers: If a resource references to another one, you need to declare
  this dependency as a kustomize config so that when a different name is generated
  for the referred resource, related references under the referrer are also
  changed.
* Variant Referencers: If a resource needs a value from another resource, you can
  use `$(VALUE)` and then in `Kustomization` object, you can declare where to fetch
  that `VALUE`. However, Kustomize requires you to explicitly declare which fields
  of the CRD you expect to have a variant like `$(VALUE)`. So, developer needs to
  declare this in the kustomize config file.
  
An example kustomize config file looks like following:

```yaml
nameReference:
  - kind: Provider
    fieldSpecs:
      - path: specTemplate/providerRef/name
        kind: CloudMemoryInstanceClass
varReference:
  - path: specTemplate/forProvider/region
    kind: CloudMemorystoreInstanceClass
```

What the `nameReference` in the snippet above says is that the kind
`CloudMemoryInstanceClass`'s field path `specTemplate/providerRef/name` refers
to the name of the kind `Provider`. So, during transformations, if the `Provider`
resource with name in `specTemplate.providerRef.name` of the resources with kind
`CloudMemoryInstanceClass` ends up with a different name, go ahead and update
the value in `specTemplate.providerRef.name`.

What the `varReference` declares is that during variant calculations, the path
`specTemplate/forProvider/region` of resources of kind `CloudMemorystoreInstanceClass`
should be taken into consideration. If the value is bare string, nothing will be
done. But if it's like `$(REGION)` and you did add a `Var` with name `REGION` to
the list `Vars` of the `Kustomization` object, then the calculated value will be
written to `specTemplate.forProvider.region` of the said resource. A `Vars` array
looks like the following:
```yaml
vars:
- name: REGION
  objref:
    kind: MinimalGCP
    apiVersion: gcp.configurationstacks.crossplane.io/v1alpha1
    name: <CR NAME>
  fieldref:
    fieldpath: spec.region
```

There is a tricky part when you'd like to refer a value in your CR since the CR
instance doesn't exist in the `resources` folder. Because of this reason,
the generic reconciler will dump the CR instance YAML file to the kustomization
folder so that Kustomize takes it into the calculation but it will remove it
from the resource list that Kustomize returned after the generation is completed.

Here is an example flow of adding a new field, say `region`, to the stack's CR
and use it in the child resources:
* In the `resources` folder, go to resources that you'd like the change its `region`
  property, put `$(REGION)` string.
* In `kustomizeconfig.yaml`, declare the field like (create that file if it doesn't
  exist):
```yaml
varReference:
  - path: specTemplate/forProvider/region
    kind: CloudMemorystoreInstanceClass
```
* In `kustomization.yaml` file of the same folder, make sure the `kustomizeconfig.yaml`
  is declared as Kustomize configuration like:
```yaml
configurations:
  - kustomizeconfig.yaml
```

* In your controller code, add a `KustomizationPatcher` that adds the necessary
  `Var` object to the `Vars` array of `Kustomization` file that refers to your
  CR instance and the field that you want `region` value to be taken. An example
  `Var` object would look like:
```go
{
  Name:   "REGION",
  ObjRef: ref,
  FieldRef: types.FieldSelector{
    FieldPath: "spec.region",
  },
},
```

## Alternatives Considered

* Helm chart or kustomize directly.
  * No reconciliation to change a top level parameter.
  * No go-to place to see the readiness of the whole environment deployed.
  * Stacks provide a better interface to users by allowing to have the same
    environment by just creating a CR instance instead of having everything in
    local.
  * Stacks do a better job for declaring CRD dependencies and UI annotation metadata.

[Dependency list]: https://github.com/crossplaneio/crossplane/blob/98e8520e2a2285cd6944fcd67fbef427299891e8/design/design-doc-stacks.md#stack-crd
[UI annotations]: https://github.com/crossplaneio/crossplane/blob/5758662818fc1e840adbfbf1a9fb37b87c3d5a5c/design/one-pager-stack-ui-metadata.md
[class and claim]: https://static.sched.com/hosted_files/kccncna19/2d/kcconna19-eric-tune.pdf
[defaulting mechanism here]: https://github.com/crossplaneio/crossplane/blob/c38561d/design/one-pager-simple-class-selection.md#unopinionated-resource-claims
[custom transformer configurations]: https://github.com/kubernetes-sigs/kustomize/blob/master/examples/transformerconfigs/crd/README.md
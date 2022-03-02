# Crossplane Agent for Consumption

* Owner: Muvaffak Onuş (@muvaf)
* Reviewers: Crossplane Maintainers
* Status: Defunct

## Background

Crossplane allows users to provision and manage cloud services from your
Kubernetes cluster. It has managed resources that map to the services in the
provider 1-to-1 as the lowest level resource. Then users can build & publish
their own APIs that are abstractions over these managed resources. The actual
connection and consumption of these resources by applications are handled with
namespaced types called requirements whose CRDs are created via
`InfrastructurePublication`s and have `*Requirement` suffix in their kind.

The consumption model is such that applications should create requirements to
request certain services and supply `Secret` name in the requirements which will
be used by Crossplane to populate the necessary credentials for application to
consume. As a simple example, an application bundle would have a
`MySQLInstanceRequirement` custom resource, a `Pod` and they would share the
same name for the secret so that one fills that `Secret` with credentials and
the other one mounts it for the containers to consume.

> For brevity, application will be assumed to have only one `Pod`.

This consumption model works well in cases where you use a single cluster for
both Crossplane and all of your applications. However, there could be many cases
that you'd like to have multiple Kubernetes clusters for different purposes
like:

* Private Networking.
  * You may want to deploy different applications into different VPCs but manage
    all of your infrastructure from one place. This isn't possible since you are
    deploying all applications into the same cluster to have them use Crossplane
    and being in the same cluster necessitates usage of the same VPC.
* Cluster Configuration.
  * Because you have to run applications in the same central cluster with
    others, you'll have to share the same Kubernetes resources like nodes and
    your needs in terms of instance types could differ greatly depending on your
    workloads, like some need GPU-powered machines and others memory-heavy ones.
* Security.
  * All applications are subject to the same user management domain, i.e. same
    api-server. This could be managed to be safe, but it's not physically
    impossible to have a `ServiceAccount` in another namespace to have access to
    resources in your namespace. So, you wouldn't really trust to have
    production in one namespace and dev in the other.

When you use multiple clusters and deploy Crossplane to each one of them gives
you more flexibility but you'd lose the ability to see all the infrastructure
from one place as the clusters are physically isolated. An example case that
you'd like to have centralized infrastructure management is that as a platform
team in an organization, you might want to publish a set of APIs that are
audited and blessed for developers in the organization to use in order to
request infrastructure. Besides from that, there are other benefits like cost
overview from one place, tracking lost/forgotten resources etc. But you would
also want to enable application teams to self-serve and have certain level of
freedom to choose the infrastructure architecture they'd like using the building
blocks you've provided.

What we need to do is to enable a platform team to have this central
infrastructure management ability while not imposing hard restrictions on
application teams. In the end, the goal of the platform teams is to increase the
velocity of development while keeping everything manageable.

### Current Approach

Crossplane has several features built to address this use case and the main
driver is the workload API which consists of `KubernetesApplication` and
`KubernetesApplicationResource` CRDs. The gist of how it works is that users
would need to provide the Kubernetes resource YAML as template to a
`KubernetesApplication` instance along with the list of `Secret`s and tell it to
which Kubernetes cluster to schedule that YAML and to propagate the given list
of `Secret`s that will be consumed by the resource in the template. This way,
everyone would still keep their infrastructure in the central cluster but if
they wanted their workloads to run in a separate cluster, then they'd wrap them
into `KubernetesApplication` and submit to that remote cluster. For reference,
here is a short version of how `KubernetesApplicationResource` looks like:

```yaml
apiVersion: common.crossplane.io/v1alpha1
kind: MySQLInstanceRequirement
metadata:
  name: sqldb
  namespace: default
spec:
  version: "5.7"
  storageGB: 20
  writeConnectionSecretToRef:
    name: sql-creds
---
apiVersion: workload.crossplane.io/v1alpha1
kind: KubernetesApplicationResource
metadata:
  name: wp-deployment
spec:
  # Select a KubernetesTarget which points to a secret that contains kubeconfig
  # of remote cluster.
  targetSelector:
    matchLabels:
      app: wp
  # The list of secrets that should be copied from central cluster to the
  # remote cluster.
  secrets:
    - name: sql-creds
  # The template of the actual resource to be created in the remote cluster.
  template:
    apiVersion: v1
    kind: Pod
    metadata:
    ...
    spec:
    containers:
    - name: wordpress
        image: "wordpress:4.6.1-apache"
        env:
        - name: WORDPRESS_DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: wp-deployment-sql-creds
              key: password
```

This resource is created in the central cluster and Crossplane itself would
manage your workload. It'd also propagate the status of the remote resource back
into status of `KubernetesApplicationResource`. In its essence, it pushes the
resources and pulls their status. Over time, we have identified several issues
with this approach:

* You cannot interact with what you deploy directly, i.e. always have to use
  `KubernetesApplicationResource` as a proxy and that has its own set of
  challenges:
  * It's a template and gets deployed after you make the edit, so, you loose the
    admission check rejections in case something went wrong. Instead, you'll see
    them in the status, but you won't be prevented from making the change as
    opposed to directly interacting.
  * Late initialization.
    * Let's say you deployed a `Pod` and `spec.node` is late-initialized. You
      will not see that because we only propagate the status back, not spec
      because the template is not strong-typed and it's hard to differentiate
      between user's actual desired spec and what's only a late-inited value.
    * If you have an element in an array that is late-inited or some elements
      are added after the creation, `PATCH` command will replace the whole array
      with what you got in your template. If the type is well-constructed to
      provide its own merge mechanics, this could be avoidable but that is
      usually not the case. For example, in some cases an element of an array in
      spec is late-inited for bookkeeping the IP and removing this causes its
      controller to provision new ones each time.
* Making existing application bundles like Helm charts are harder.
  * You actually need to change each and every element to be in a
    `KubernetesApplicationResource` in order to deploy them to a cluster that's
    different than where Crossplane itself runs.
  * For example, you'd like change only the `StatefulSet` in the Helm chart with
    `MySQLInstanceRequirement` to use Crossplane for DB provisioning but you
    have to change each resource to be a template in a
    `KubernetesApplicationResource` if you'd like to use the `Secret` of
    `MySQLInstanceRequirement` in the remote cluster.
* Operation experience. This is related to the first point. If you have an app
  using OAM, or some other app model, then you always have that intermediate
  proxy of workload CRs. You're losing on some value that these models provide
  because of that proxy and in some cases it could be functionally detrimental.

Surely, it has its own advantages as well. For example, you can manage all of
your apps from single point via `KubernetesApplication`s targeting the right
clusters. But as we see more usage patterns, we're convinced that the current
mechanics do not provide the experience users would like to have.

## Proposal

In order to preserve the central infrastructure management ability while
alleviating the issues above, we will change our approach from push-based one to
a pull-based one where applications, and their requirements are deployed into
the remote cluster, and they request the infrastructure from a central cluster
and pull the necessary credentials.

Since having this logic in the applications themselves wouldn't be a good UX, we
will have an agent that you will need to deploy into your remote cluster for
doing the heavy-lifting for you. There are several technical problems to be
solved in order to make the experience smooth. Overall, the goal is that we want
to keep the UX of local mode for application operators while keeping the power
of centralized infrastructure management for platform operators. For reference,
here is an example local mode experience we'd like to have for the remote mode
as well:

```yaml
apiVersion: common.crossplane.io/v1alpha1
kind: MySQLInstanceRequirement
metadata:
  name: sqldb
  namespace: default
spec:
  version: "5.7"
  storageGB: 20
  writeConnectionSecretToRef:
    name: sql-creds
---
apiVersion: v1
kind: Pod
metadata:
  name: wp
  namespace: default
spec:
containers:
- name: wordpress
    image: "wordpress:4.6.1-apache"
    env:
    - name: WORDPRESS_DB_PASSWORD
    valueFrom:
        secretKeyRef:
          name: sql-creds
          key: password
```

The agent will be a Kubernetes controller running in the remote cluster and
watching all `*Requirement` types. Next sections will talk about the
implementation and user experience we'd like to have.

### Synchronization

In local mode, users directly interact with what's published by the platform
team, which is `*Requirement` types and consume the infrastructure by mounting
the secret whose name they specified on the `*Requirement` custom resource. To
keep this experience, we need to have a synchronization loop for the following
resources:

* Pull
  * `CustomResourceDefinition`s of all types that we want the applications to be
    able to manage and view:
    * Requirements that are published via `InfrastructurePublication`s. The
      source of truth will be the remote cluster.
    * `InfrastructureDefinition`, `InfrastructurePublication` and `Composition`.
  * `Composition`s to discover what's available to choose. CRs of this type will
    be read-only, and the source of truth will be the central cluster.
  * `InfrastructurePublication`s to discover what's available as published API.
    Read-only.
  * `InfrastructureDefinition`s to discover how the secret keys are shaped.
    Read-only.
  * `Secret`s that are result of the infrastructure that is provisioned so that
    it can be mounted to `Pod`s. Read-only.
* Push
  * `*Requirement` custom resources so that infrastructure can be requested.
    Read and write permissions in a specific namespace in the central cluster
    will be needed.

Note that there will be no controller reconciling `InfrastructurePublication`
and `InfrastructureDefinition` types to generate their corresponding CRDs; the
agent will blindly pull the resulting CRDs from the central cluster so that in
case of a version mismatch between the agent(s) and the Crossplane in the
central cluster, there won't be any schema difference. The source of truth for
all these listed resources except the `*Requirement`s is the central cluster.

Here is an illustration of how synchronization will work:

![Synchronization Flows][sync-diagram]

### RBAC

As we have two different Kubernetes API servers, there will be two separate
security domains and because of that, the `ServiceAccount`s in the remote
cluster will not be able to do any operation in the central cluster. Since the
entity that will execute the operations in the central cluster is the agent, we
need to define how we can deliver the necessary credentials to the agent so that
it can connect to the central cluster. Additionally, it will need some
permissions to execute operations in the remote cluster like `Secret` and
`CustomResourceDefinition` creation. We will look at how the agent will be
granted permissions to do its job in two separate domains with different
mechanisms.

#### Authenticating to The Central Cluster

In order to execute any operation, a `ServiceAccount` needs to exist in the
central cluster with appropriate permissions to read during pull and write
during push operations while synchronizing with central cluster. Since the agent
is running in the remote cluster, the credentials of this `ServiceAccount` will
be stored in a `Secret` in the remote cluster. Alongside the credentials, the
agent needs to know the namespace that it should sync to in the central cluster.

The easiest configuration would be the one where we specify the `Secret` and a
target namespace via installation commands of the agent. Both of these inputs
will act as default and they can be overridden for each `Namespace`
independently. For example, multiple namespaces can have different requirements
with the same name which could cause conficts in the central cluster because all
namespaces are rolled up into one namespace. In order to prevent conflicts, the
agent will annotate the requirements in the central cluster with the UID of the
namespace in the remote cluster and do the necessary checks to avoid conflicts.

An illustrative installation command:
```bash
helm install crossplane/agent \
  --set default-credentials-secret-name=my-sa-creds \
  --set default-target-namespace=ns-in-central
```

While this installation time configuration provides a simple setup, it comes
with the restriction that you cannot have the requirements in different
namespaces with the same name. You can either try using different names or you
can specify which namespace in the remote cluster should be synced to which
namespace in the central cluster. In the remote cluster, `Namespace` can be
annotated as such:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: foo
  annotations:
    "agent.crossplane.io/target-namespace": bar
```

The agent then will try to sync the requirements in `foo` namespace of the
remote cluster into `bar` namespace of the central cluster instead of the
default target namespace which is `ns-in-central` in this example. But it will
keep using the default credentials `Secret`. In case you don't want different
namespaces to share the same credentials `Secret`, then you can override this
setting, too:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: foo
  annotations:
    "agent.crossplane.io/target-namespace": bar
    "agent.crossplane.io/credentials-secret-name": my-other-sa-creds
---
apiVersion: v1
kind: Secret
metadata:
  name: my-sa-creds
  namespace: foo
type: Opaque
data:
  kubeconfig: MWYyZDFlMmU2N2Rm...
```

Now, all the requirements in the `foo` namespace of the remote cluster will be
synced to `bar` namespace of the central cluster and all of the operations will
be done using the credentials in `my-other-sa-creds`.

There is also the option to automate the namespace pairings in a way that if
requirement `A` is deployed in namespace `foo` of the remote cluster, then it
gets synced to namespace `foo` of the central cluster. You can enable this
automation using a flag. Helm command would look like:
```bash
helm install crossplane/agent \
  --set default-credentials-secret-name=my-sa-creds \
  --set match-namespaces=true
```

As with all cases, you can override specific pairings via annotations on the
namespaces of the remote cluster.

#### Conflicts

The agent does not allow any conflicts to happen, meaning if two requirements
created with the same name by different agents in some namespace in the pool of
clusters, then it will reject syncing it to the central cluster. In order to do
that, there needs to be unique identifier of the source of the requirement.

Let's go over the different setups and consider conflict cases:
* Each namespace is annotated with the target namespace.
  * No conflict for single remote cluster.
  * Could conflict if more than one remote cluster is connected and its namespaces have
    common annotations with other namespaces of other remote clusters.
* Each namespace targets a namespace with the same name in the central cluster.
  * No conflict for single remote cluster.
  * Could conflict if more than one cluster is connected and its namespaces have
    the same names.
* All namespaces roll up to a default namespace.
  * Could conflict for single remote cluster if different namespaces have
    requirements with same name.
  * Could conflict if more than one cluster is connected to the same namespace
    but it's less likely to be in that situation since the default namespace
    selection is done explicitly during agent setup, probably by an admin.

In order to prevent conflicts, the agent will add two annotations to the
requirements it syncs:
* `agent.crossplane.io/source-namespace`: The name of the namespace in the
  remote cluster.
* `agent.crossplane.io/source-uid`: The unique identifier of the remote
  cluster.

By default, the value of `agent.crossplane.io/source-uid` will be the UID of the
`kube-system` namespace. However, you can override this with an installation
parameter like:
```bash
helm install crossplane/agent \
  --set default-credentials-secret-name=my-sa-creds \
  --set cluster-identifier=my-funny-little-cluster
```

This override behavior is especially useful in case you need to replace the
remote cluster with another one for various reasons like cluster simply not
responding, datacenter catching fire etc. In such cases, you can use the same
cluster identifier when you install the agent to the new remote cluster so that
it claims the ownership of the requirements that the old remote cluster created.

#### Authorization

##### Remote Cluster

Since there will be one agent for the whole cluster, its own mounted
`ServiceAccount` in that remote cluster needs to have read & write permissions
for all of the following kinds in the remote cluster listed below:

* `CustomResourceDefinition`
* `Composition`
* `InfrastructureDefinition`
* `InfrastructurePublication`
* `Secret`
* All `*Requirement` types

The last one is a bit tricky because the exact list of `*Requirement` types on
kind level is not known during installation and it's not static; new published
APIs should be available in the remote cluster dynamically. One option is to
allow agent to grant `Role` and `RoleBinding`s to itself as it creates the
necessary `CustomResourceDefinition`s in the remote cluster. However, an entity
that is able to grant permissions to itself could greatly impact the security
posture.

When you zoom out and think about how the `Role` will look like, in most of the
cases, it's something like the following:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: crossplane-agent
  namespace: default
rules:
  # all kinds could be under one company/organization group
- apiGroups: ["acmeco.org"] 
  resources: ["*"]
  verbs: ["*"]
  # or there could be logical groupings for different sets of requirements
- apiGroups: ["database.acmeco.org"]
  resources: ["*"]
  verbs: ["*"]
```

As you can see, it's either one group for all the new APIs or a logical group
for each set of APIs. In both cases, the frequency of the need to add a new
`apiGroup` is less than one would imagine thanks to the ability of allowing a
whole group; most frequently, the platform operators will be adding new kinds to
the existing groups.

In the light of this assumption, the initial approach will be that the `Role`
bound to the agent will be populated by a static list of the groups of the
requirement types during the installation like shown above and if a new group is
introduced, then an addition to this `Role` will be needed. A separate
controller to dynamically manage the `Role` is mentioned in the [Future
Considerations](#future-considerations) section.

##### Central Cluster

The `ServiceAccount` that will be created in the central cluster needs to have
the following permissions for agent to do its operations:

* Read
  * `CustomResourceDefinition`s
  * `InfrastructureDefinition`s
  * `InfrastructurePublication`s
  * `Composition`s
  * `Secret`s in the given namespace.
* Write
  * `*Requirement` types that you'd like to allow in given namespace.

### User Experience

In this section, a walkthrough from only a central cluster to a working
application will be shown step by step to show how the user experience will
shape.

Setup:
  * The Central Cluster: A Kubernetes cluster with Crossplane deployed &
    configured with providers and some published APIs.

Steps in the central cluster:
1. A new `Namespace` called `bar` is created.
1. A `ServiceAccount` called `agent1` in that namespace are created and
   necessary RBAC resources are created.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: bar
---
# The ServiceAccount whose credentials will be copied over to remote cluster
# for agent to use to connect to the central cluster.
apiVersion: v1
kind: ServiceAccount
metadata:
  name: agent1
  namespace: bar
---
# To be able to create & delete requirements in the designated namespace of
# the central cluster.
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: agent-requirement
  namespace: bar
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["*"]
- apiGroups: ["database.acmeco.org"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["network.acmeco.org"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: agent-requirement
  namespace: bar
subjects:
- kind: ServiceAccount
  name: agent1
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: agent-requirement
  apiGroup: rbac.authorization.k8s.io
```

The YAML above includes what's necessary to sync a specific namespace. The YAML
below is for cluster-scoped resources that should be read by the agent and it's
generic to be used by all agents except the `subjects` list of
`ClusterRoleBinding`:

```yaml
# To be able to read the cluster-scoped resources.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: read-for-agents
rules:
- apiGroups: ["apiextensions.kubernetes.io"]
  resources: ["customresourcedefinitions"]
  verbs: ["get", "watch", "list"]
- apiGroups: ["apiextensions.crossplane.io"]
  resources:
  - infrastructuredefinitions
  - infrastructurepublications
  - compositions
  verbs: ["get", "watch", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: read-for-agents
subjects:
- kind: ServiceAccount
  name: agent1
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: read-for-agents
  apiGroup: rbac.authorization.k8s.io
```

At this point, we have a `ServiceAccount` with all necessary permissions in our
central cluster. You can think of it like IAM user in the public cloud
providers; we have created it and allowed it to access a certain set of APIs.
Later on, its key will be used by the agent; just like provider-aws using the
key of an IAM User.

1. User provisions a network stack and a Kubernetes cluster in that private
   network through Crossplane (or via other methods, even kind cluster would
   work). This cluster will be used as remote cluster. We'll run the rest of the
   steps in that remote cluster.
1. The `Secret` that contains the kubeconfig of the `ServiceAccount` we created
   in the central cluster is replicated in the remote cluster with name
   `agent1-creds`.
1. The agent is installed into the remote cluster. The Helm package will have
   all the necessary RBAC resources but user will need to enter the API groups
   of the published types so that it can reconcile them in the remote cluster.
   ```bash
   helm install crossplane/agent \
   --set apiGroups=database.acmeco.org,network.acmeco.org \
   --set default-credentials-secret-name=agent1-creds \
   --set default-target-namespace=bar
   ```

At this point, the setup has been completed. Now, users can use the APIs in the
remote cluster just as if they are in the local mode. An example application to
deploy:

```yaml
apiVersion: common.crossplane.io/v1alpha1
kind: MySQLInstanceRequirement
metadata:
  name: sqldb
  namespace: default
spec:
  version: "5.7"
  storageGB: 20
  writeConnectionSecretToRef:
    name: sql-creds
---
apiVersion: v1
kind: Pod
metadata:
  name: wp
  namespace: default
spec:
containers:
- name: wordpress
    image: "wordpress:4.6.1-apache"
    env:
    - name: WORDPRESS_DB_PASSWORD
    valueFrom:
        secretKeyRef:
          name: sql-creds
          key: password
```

The `MySQLInstanceRequirement` will be synced to `bar` namespace in the central
cluster. In case there are other `MySQLInstanceRequirement` custom resources in
the remote cluster with same name but in different namespaces, then the agent
will reject syncing that in order to prevent the conflict.


Note that these steps show the bare-bones setup. Most of the steps can be made
easier with simple commands in Crossplane CLI and YAML templates you can edit &
use.

Users can discover what resources available to them and how they can consume
them from their remote cluster.

List the published APIs:
```bash
kubectl get infrastructurepublications
```

See what keys are included in the connection `Secret` of a specific API so that
they know what keys to use in mounting process:
```
kubectl get infrastructuredefinition mysqlinstance.database.acmeco.org -o yaml
```

See what `Composition`s are available to select from:
```
kubectl get compositions
```

## Future Considerations

### Migration From Local Mode

Let's say you're running Crossplane in a big cluster together with your apps and
decided that you're at a point where you'd like to use the same Crossplane
environment from multiple clusters. You can either make the current cluster a
central cluster and have additional clusters connect to it via agent, which is
possible with this current design, or you'd like to migrate all things related
to Crossplane to a separate cluster and make your current one a remote cluster.
The agent can make some smart operations to enable the migration of the current
composite and managed resources to the central cluster for a smooth migration.

There will likely some changes needed in Crossplane's deployment model as well
but as an overarching goal, this migration path should be smooth.

### Read-only Mode for Federation

Crossplane agent could have a read-only mode where it replicates `Secret`s and
requirements in the remote cluster to let the `Pod`s use them. There would be
only pull operation and the same requirements could be used by N cluster at the
same time. For example, you might want to use the same database cluster from
different clusters spread across the globe.

### Additional Crossplane CLI Commands

Crossplane CLI can have simple commands to do most of the setup. For example,
with one command it should be possible to create the `ServiceAccount` in the
central cluster together with all of its RBAC resources. Also, agent
installation could be a very smooth process if we use Crossplane CLI instead of
Helm.

### RBAC Controller

A controller with its own types to manage the `ServiceAccount`s in the central
cluster could be a boost to UX. You'd create a custom resource that will result
in all RBAC resources that are needed for the agent to work with all the APIs in
the central cluster and write the credentials to a secret. Then the user can
copy this secret into their remote cluster and refer to it.

### Dynamic Updates to Agent's Role

In the remote cluster, we provide the `Role` that has the static set of
`apiGroup`s we'd like agent to be able to manage in the remote cluster. There
could be a controller that is separately installed and it could add new
`apiGroup`s as they appear as `InfrastructurePublication`s in the remote
cluster.

### Agent as an Executable for VMs

Instead of remote cluster, a VM can also use the infrastructure services that
the central cluster exposes. A different version of the agent could sync the
secrets to a file in the VM that can be used as credential file for VM
workloads. Maybe a small api-server shipped with that agent binary for
requirement requests?

### Admission Webhook to Reject Conflicts

In the current design, if you configured the default namespace with the agent,
then it's possible to have conflicts; `foo` requirement in `default` namespace
can conflict with `foo` requirement in `special` namespace since both of them
will be rolled up to a single default namespace in the central cluster. In such
cases, the agent rejects syncing the second one and writes about the conflict to
the status of the resource. However, an admission webhook could check the
conflict and reject the creation altogether immediately.

## Alternatives Considered

### Improving KubernetesApplication

We could make it a lower level component with no scheduling and have the
Crossplane CLI to do some smart conversions of the input YAML. Though that
approach is still subject to the problems of `KubernetesApplication` that are
inherently caused by template propagation.

### Remove Pod Controller

We could disable pod controller (or have a custom api-server with no pod
controller, shipped via Crossplane) and sync the `Pod`s in the local cluster to
the configured remote cluster. This option seems possible but the indirection
could result in increased complexity and also propagation of native types could
result in similar problems we had with templates in `KubernetesApplication`

### Webhook to Convert to KubernetesApplication

A webhook to convert the native types to `KubernetesApplication` during creation
could be possible but it's risky to enter one thing but end up with another
thing in the cluster. It's essentially putting an abstraction on top of all
api-server calls and it has to ignore some types that need to be in local
cluster like requirements.



[sync-diagram]: ../images/agent-diagram.png
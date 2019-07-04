# Modelling Resource Usage in Crossplane

* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Terminology

* _External resource_. An actual resource that exists outside Kubernetes,
  typically in the cloud. AWS RDS and GCP Cloud Memorystore instaces are
  external resources.
* _Managed resource_. The Crossplane representation of an external resource.
  The `RDSInstance` and `CloudMemorystoreInstance` Kubernetes kinds are managed
  resources. A managed resource models the satisfaction of a need; i.e. the need
  for a Redis Cluster is satisfied by the allocation (aka binding) of a
  `CloudMemoryStoreInstance`.
* _Resource claim_. The Crossplane representation of a request for the
  allocation of a managed resource. Resource claims typically represent the need
  for a managed resource that implements a particular protocol. `MySQLInstance`
  and `RedisCluster` are examples of resource claims.
* _Resource class_. The Crossplane representation of the desired configuration
  of a managed resource. Resource claims reference a resource class in order to
  specify how they should be satisfied by a managed resource.
* _Connection secret_. A Kubernetes `Secret` encoding all data required to
  connect to (or consume) an external resource.
* _Claimant_ or _consumer_. The Kubernetes representation of a process wishing
  to connect to a managed resource, typically a `Pod` or some abstraction
  thereupon such as a `Deployment` or `KubernetesApplication`.

## Background

Crossplane allows developers to manage and consume external resources via the
Kubernetes API. Typically a few things must be known about an external resource
in order to consume it: its URL, IP address, ports, credentials, etc. Managed
resources expose the non-sensitive subset of this connection data via their
`.status` object. All connection data, both sensitive and non-sensitive, is also
exposed via a connection secret.

Connection secret data has no schema; each managed resource controller specifies
the keys and values that will be encoded in the `.data` object of its `Secret`.
Constants such as `endpoint` and `password` serve to ensure there is some
consistency in secret data keys across managed resources. Connection secrets are
created at managed resource creation time in the same namespace as their
managed resource (typically `crossplane-system`). Managed resources, which
represent infrastructure external to Kubernetes, would not be namespaced at all
if it were not for their need to persist connection data to a namespaced
`Secret`. The `crossplane-system` namespace was originally envisioned to be the
closest possible facsimile of cluster scoped resources given this dependency on
namespaced `Secrets` per Crossplane issue [#92].

Crossplane intends for consumers to declare their intent to consume a managed
resource via a resource claim. Because resource claims typically exist in the
same namespace as their claimant (for example a `Pod`) the resource claim
controller copies the connection secret written by the managed resource into the
resource claim's namespace. Note that despite this intent only RBAC prevents a
managed resource from being consumed directly. A user with the appropriate RBAC
role could create a `Pod` in `crossplane-system` that loaded said managed
resource's connection secret without ever making a claim to it.

Recall that each managed resource controller emits connection secrets in an
opinionated format. This format is maintained as the connection secret is
propagated to a resource claim. It's possible - even likely - that the
consumer of a managed resource is equally opinionated about how it should be
provided its connection data and that the consumer's opinions are incompatible
with Crossplane's. GitLab is one example of this pattern. The below example
highlights the differences in how Crossplane exposes secrets and how GitLab
expects to consume them. Note that all base64 values have been decoded for
readability:

```yaml
---
# The format in which Crossplane emits S3 bucket connection secrets.
apiVersion: v1
kind: Secret
metadata:
  name: crossplane-s3-bucket-connection-secret
type: Opaque
data:
  # Notice that Crossplane attempts to use its standard set of secret keys
  # to represent this data, particularly how the bucket region is stored in
  # the 'endpoint' key that would typically represent a resource's URL.
  endpoint: aws_region
  username: aws_access_key_id
  password: aws_secret_access_key
---
# The format in which GitLab expects backup S3 bucket connection secrets.
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-s3-bucket-connection-secret-backups
type: Opaque
data:
  config: |
    [default]
    access_key = aws_access_key_id
    secret_key = aws_secret_access_key
    bucket_location = aws_region
---
# The format in which GitLab expects other S3 bucket connection secrets.
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-s3-bucket-connection-secret-other
type: Opaque
data:
  config: |
    provider: AWS
    region: aws_region
    aws_access_key_id: aws_access_key_id
    aws_secret_access_key: aws_secret_access_key
```

Inflexibility is not the only limitation of the contemporary connection secret
pattern. Many external resources either generate or expect to be provided with a
set of superuser credentials at creation time. Crossplane passes these superuser
credentials on to any claimant, going against typical security best practices.
`KubernetesCluster` claims, for example, will be provided with the cluster
administrator's credentials whether they need them or not. Azure storage
containers are another example of this mispattern. Crossplane distributes the
storage account key to all claimants, contrary to the recommendation of the
[Azure storage documentation]:

> Your storage account key is similar to the root password for your storage
> account. Always be careful to protect your account key. Avoid distributing
> it to other users, hard-coding it, or saving it anywhere in plaintext that
> is accessible to others.

Azure recommends instead granting access via [Azure service principal], a
distinct resource with its own credentials that may be granted access to storage
containers and many other Azure resources. [GCP service accounts] and [AWS IAM
users] are analogous to service principals in their respective clouds. Creating
an AWS bucket transparently creates an IAM role that is granted access to the
bucket. This existence of this IAM role is not modelled in the Kubernetes API.
GCP buckets do not create a service account, but require one to be created
manually outside of Crossplane in order to grant bucket access. Storage buckets
happen to be the only class of resource currently supported by Crossplane that
use such 'robot' accounts to grant access, but this is the prevalent pattern
amongst most resources provided by the big three clouds. Hypothetical
`DynamoDB`, `CloudDatastore`, `CloudSpannerInstance`, etc managed resources
would also require such accounts.

## Goals

The design proposed by this document intends to:

* Allow resource claimants to specify the format in which they require
  connection details to be presented.
* Avoid distributing superuser credentials where consumer scoped credentials
  would be more appropriate.
* Establish foundations for modelling the consumption of a managed resource in
  order to eventually ensure network level connectivity between.

Note that while this document may lay the foundation for network level
connectivity constructs, it defers discussing that problem space in detail.

## Proposed Design

This document proposes:

* That the existing resource class and managed resource kinds become cluster
  scoped, rather than namespaced.
* The introduction of namespaced 'resource usages' aligned with existing
  resource claim kinds, for example `PostgreInstanceSQLUsage`, or
  `RedisClusterUsage`.

### Managed Resource Connection Secrets

Managed resources, while cluster scoped, will simply specify a name and
namespace to which to write their connection secret. Connection secrets will be
used strictly for _secret_ connection details such as credentials, not for less
sensitive details like endpoints and ports. Managed resources will not be
required to write each credential kind to any particular key in the connection
secret. For example:

```yaml
apiVersion: database.aws.crossplane.io/v1alpha1
kind: RDSInstance
metadata:
  name: cool-sock-product-db
spec:
  writeConnectionSecretTo:
    namespace: kube-system
    name: rds-some-uuid
```

If the namespace is omitted the `Secret` will be created in the `default`
namespace. A connection secret might look like the following:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: rds-some-uuid
  namespace: kube-system
data:
  # Values are base64 encoded, presented decoded here for readability.
  # Note the lack of non-sensitive data such as username, URI, etc.
  masterPassword: crossplane_generated_password
```

### Resource Usages

A resource usage builds on the resource claim concept, specifying how a claim's
underlying managed resource's connection data should be exposed to a specific
consumer, as well as 'solving' the usage, for example by ensuring the consumer
has network connectivity to the managed resource, or provisioning usage scoped
credentials with access to the managed resource. Resource usages, like resource
claims, are namespaced. They must exist in the same namespace as their claim and
consumer.

```yaml
apiVersion: storage.crossplane.io/v1alpha1
kind: PostgreSQLInstanceUsage
metadata:
  namespace: cool-team
  name: cool-sock-product-db
spec:
  # Note that claimRef and consumerRef would likely be *corev1.ObjectReference
  # behind the scenes, but would only be allowed to reference claims and
  # consumers in the same namespace as the usage.
  claimRef:
    apiVersion: storage.crossplane.io/v1alpha1
    kind: PostgreSQLInstance
    name: cool-sock-db
  consumerRef:
    apiVersion: apps/v1
    kind: Deployment
    name: cool-sock-product
  publish:
    service:
      name: cool-sock-product-db
      targetTemplate: "{{ .instance.status.endpoint }}"
    secret:
      name: cool-sock-product-db
      dataTemplate:
        config.json: |
          {
              "host": "cool-sock-product-db.cool-team.svc.cluster.local",
              "port": "{{ .instance.status.port }}",
              "user": "{{ .user.spec.name }}
              "password": "{{ .user.secret.password }}"
          }
    configMap:
      name: cool-sock-product-db
      dataTemplate:
        url: "postgres://{{ .instance.status.endpoint }}:{{ .instance.status.port }}"
        username: "{{ .user.spec.name }}"
status:
  solutionRefs:
  - apiVersion: database.gcp.crossplane.io/v1alpha1
    kind: CloudSQLDatabase
    name: cool-sock-product
  - apiVersion: database.gcp.crossplane.io/v1alpha1
    kind: CloudSQLUser
    name: cool-sock-product
  publishedRefs:
  - apiVersion: v1
    kind: Service
    namespace: cool-team
    name: cool-sock-product
  - apiVersion: v1
    kind: Secret
    namespace: cool-team
    name: cool-sock-product
  - apiVersion: v1
    kind: ConfigMap
    namespace: cool-team
    name: cool-sock-product
  conditions:
  - type: Solved
    status: "True"
    lastTransitionTime: "1970-01-01 00:00:01"
    reason: All solutions created.
  - type: Published
    status: "True"
    lastTransitionTime: "1970-01-01 00:00:01"
    reason: All connections published.
```

A one-to-one relationship exists between resource claim kinds and resource usage
kinds; a `PostgreSQLInstanceUsage` must always reference a `PostgreSQLInstance`
kind claim. Resource usages thus have a one-to-many transitive relationship to
managed resource kinds due to the one-to-many relationship between resource
claim kinds and managed resource kinds. A separate controller must reconcile
each `(resource-usage, managed-resource)` tuple in order to tailor solutions to
a particular external resource. In the above example the usage controller
determines the `PostgreSQLInstance` claim is satisfied by a `CloudSQLInstance`
managed resource and thus creates a `CloudSQLDatabase` and `CloudSQLUser` (both
of which are modelled in the external CloudSQL API) scoped to the usage.

In the above example:

* `.spec.claimRef` is a reference to the `PostgreSQLInstance` resource claim
  being consumed.
* `.spec.consumerRef` is a reference to a consuming resource of an arbitrary
  kind.
* `.spec.publish` specifies how the underlying managed resource should be
  exposed to its consumer. `.spec.publish` and each of its immediate sub-keys
  are optional.
* `.spec.publish.service` specifies that an `ExternalName` type `Service` (i.e.
  a cluster internal DNS CNAME) named `name` should be created pointing to
  `targetTemplate`, where `targetTemplate` is a Go template expected to render
  to a target DNS name.
* `.spec.publish.secret` specifies that a `Secret` named `name` should be
  created with data built from `dataTemplate`, an object mapping string keys
  to templated values.
* `.spec.publish.configMap` specifies that a `ConfigMap` named `name` should be
  created with data built from `dataTemplate`, an object mapping string keys
  to templated values.
* `.status.solutionRefs` represents the array of Kubernetes resources created by
  the usage controller in order to solve the usage.
* `.status.publishRefs` represents the array of Kubernetes resources published
  by the usage controller.
* `.status.conditions` represents the status of the solving and publishing
  processes.

Usages are solved before they are published. Each usage controller
implementation may choose what data is available to the templated fields of its
usage resource. This allows resource controllers to determine whether usages
may access any superuser credentials associated with the underlying managed
resource as appropriate, but puts the onus on controller implementations to
ensure resource usages are furnished with a consistent data keys regardless of
the underlying managed resource kind.

## Open Questions

This design is an early draft, and thus has many areas for improvement and
raises some questions, including:

* Does the resource usage concept hold up given the subtle implementation
  differences between the multiple managed resources that may satisfy a resource
  claim?
* Should "sub-managed-resources" such as databases and users be automatically
  "solved" into existence by a resource usage controller, or should they be
  instantiated explicitly by their own claims and classes?

[#92]: https://github.com/crossplaneio/crossplane/issues/92
[Azure storage documentation]: https://docs.microsoft.com/en-us/azure/storage/common/storage-configure-connection-string
[Azure service principal]: https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli
[GCP service accounts]: https://cloud.google.com/iam/docs/understanding-service-accounts
[AWS IAM users]: https://docs.aws.amazon.com/IAM/latest/UserGuide/introduction_identity-management.html
[encryption at rest]: https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/

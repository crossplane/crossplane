# External Secret Stores

* Owner: Hasan Türken (@turkenh)
* Reviewers: Crossplane Maintainers
* Status: Accepted

## Background

Kubernetes suggests using `Secret` resource to store sensitive information and
when requested from the API Server, one gets a base64 encoded value of sensitive
data rather than encrypted. RBAC is used as a security measure for access
control, instead of dealing with a decryption process whenever this data needed
to be consumed. However, in practice it is not easy to ensure proper access to
sensitive data with RBAC. For example, when an application requires read/write
access to some secrets in a namespace, it is common pitfall to deploy with
access to any secret in the namespace which could result in unintended access to
another secret in the same namespace. Hence, Kubernetes secrets usually
considered as **not so secure**.

Kubernetes has a solution to [encrypt secrets at rest], however, this is only to
keep secret data as encrypted in etcd which changes nothing at Kubernetes API
level. Requests for supporting additional external secret stores at API level
were [rejected for various reasons].

Another point is, Kubernetes clusters often considered as ephemeral resources
as opposed to being a **reliable** data store not only for sensitive data but
also non-sensitive data like application manifests and this increased popularity
of GitOps tools.

Hence, _storing sensitive information in external secret stores is a common
practice_. Since applications running on K8S need this information as
well, it is also quite common to sync data from external secret stores to K8S.
There are [quite a few tools] out there that are trying to resolve this exact
same problem. However, Crossplane, as a **producer** of infrastructure
credentials, needs the opposite, which is storing sensitive information
**to external secret stores**.

Today, Crossplane only supports storing connection details for managed resources
as **Kubernetes secrets**. This is configured via the
`spec.writeConnectionSecretToRef` field. However, there is an increasing demand
to store infrastructure credentials on external secret stores rather than
relying on Kubernetes secrets.

This document aims to propose a design for supporting external secret stores in
Crossplane. The solution should apply to both Crossplane itself (for composite
connection details) and providers (for managed resource connection details).
Throughout the document, we will mostly focus on [Vault] as the most popular
secret store today.

### Goals

We would like to come up with a design that:

- Will not break the existing APIs.
- Will support adding new secret stores without any breaking API changes.
- Will support switching to an **out-of-tree plugin model** without breaking the
  API for existing in-tree secret stores.

We would like Crossplane to be able to store connection details to external
secret stores and still satisfy the following user stories:

- As a platform operator, I would like to configure how/where to store
  connection details for managed and composite resources.
- As a platform consumer, I would like to configure how/where to store
  connection details for my composite resource claims.

To achieve this, it should still be possible to pass partial inputs for
connection details secret for dynamically provisioned resources:

- In Composition spec, configuring where/how to store connection details for
composite resources (i.e. `writeConnectionSecretsToNamespace`)
- Assigning a [generated name] to connection secrets of composite resources.

## Out of Scope

The following is out of scope for this design:

- Reading **provider credentials** from external secret stores.

  This design focuses on writing/publishing connection details to external
  stores and not about reading them as **provider credentials**. There are
  already ways to consume secrets in external stores from Kubernetes and is out
  of the scope for this document. Check the [Vault credential injection guide] to
  see how one can configure Vault and Crossplane to consume provider credentials
  from Vault.

- Reading managed resource **input secrets** from external secret stores.

  There are some resources which requires sensitive information like initial
  password as input. Typically, this type of input is received from a Kubernetes
  secret. This design focuses on storing credentials produced by Crossplane
  hence input secrets are out of scope.

## Design

A secret is a resource keeping sensitive information typically as a set of key
value pairs. To access a secret instance in a secret store we would need the
following information:

- An identifier which uniquely identifies the secret instance within the
store (a.k.a. `external-name`). This identifier could be split into two
different parts:
  - **A name**: A unique name within a scope/group.
  - **A scope/group identifier**: Specific to secret store. For example,
  `namespace` in _Kubernetes_, a parent `path` in _Vault_ and a `region` for
  _AWS Secret Manager_.
- **Additional configuration** to reach to the store like its endpoint and
credentials to authenticate/authorize. For example, a `kubeconfig` for
_Kubernetes_, a `server` endpoint + auth config for _Vault_ and access/secret
keys for _AWS Secret Manager_.

### API

#### Secret Configuration: PublishConnectionDetailsTo

We will deprecate the existing `writeConnectionSecretToRef` field in favor of
`publishConnectionDetailsTo` field which would support publishing connection
details to the local Kubernetes cluster **or** to an external secret store.

We will take the **name** of secret and some per secret metadata like tags in
our secret configuration API (i.e. `publishConnectionDetailsTo`). The rest of
the configuration will go to a separate store specific config (`StoreConfig`).
This classification enables building a flexible API that satisfies the
separation of concerns between platform operators and consumers which Crossplane
already enables today for Kubernetes Secrets.

We will end up having a unified configuration spec for all external secret store
types which contains the name field (`name`) and a reference to any additional
store specific configuration (`configRef`). This would be enough to uniquely
identify and access any secret instance, however there could still be some
additional metadata specific to store type that might be desired
to be set per secret instance. For example, `labels`, `annotations` and `type`
of the secret in _Kubernetes_; `Tags` and `EncryptionKey` in _AWS_.

A `StoreConfig` named `default` will be created during installation which is
configured to write secret to a Kubernetes cluster in the Crossplane
installation namespace.`publishConnectionDetailsTo.configRef` will be optional
and **in its absence**, it will be late initialized as `configRef.name=default`.

**Examples:**

Publish Connection details to a _Kubernetes_ secret (already existing case):

```yaml
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    configRef:
      name: default
```

Publish Connection details to a _Vault_ secret:

```yaml
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    configRef:
      name: vault-dev
```

Publish Connection details to a _Kubernetes_ secret with some labels/annotations:

```yaml
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    metadata:
      labels:
        environment: production
      annotations:
        acme.example.io/secret-type: infrastructure
    configRef:
      name: default
```

Publish Connection details to an _AWS Secret Manager_ secret with some tags:

```yaml
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    metadata:
      tags:
        environment: production
    configRef:
      name: aws-secret-manager-prod
```

#### Separation of Concerns: Compositions, Composites and Claims

Currently, platform operators could specify where should composite resource
secrets land using the partial input `writeConnectionSecretsToNamespace`. We
will similarly deprecate this field in favor of
`publishConnectionDetailsToStoreConfigRef`.

**Examples:**

**Composition** configuring to publish to default Store which is created during
installation time, i.e. publish to a Kubernetes secret in the namespace where
crossplane installed.

```yaml
spec:
  publishConnectionDetailsToStoreConfigRef: default
```

**Composition** configuring to publish to another namespace, e.g.
`infrastructure-staging`, where a `StoreConfig` named
`store-infrastructure-staging` created with `defaultScope` parameter as
`infrastructure-staging`.

```yaml
spec:
  publishConnectionDetailsToStoreConfigRef: store-infrastructure-staging
```

**Composition** configuring to publish to vault.

```yaml
spec:
  publishConnectionDetailsToStoreConfigRef: vault-production
```

For **Claim** resources, claim namespace will be used as a scope instead of the
`defaultScope` in `StoreConfig`.

```yaml
spec:
  publishConnectionDetailsTo:
    name: database-creds
```

#### External Secret Store Configuration: StoreConfig

External secret store configuration will contain the required information other
than the name (and optional metadata) of secret. Thanks to
[the standardization efforts] on a declarative API for syncing secrets from
external stores into Kubernetes, there is already an [existing schema] that we
can follow here. However, there will be slight differences since we want to
receive the `name` of secret as input in our API whereas, they expect a full
identifier of the secret instance within the secret store
(in [ExternalSecret spec] at `spec.dataFrom.key`). Another reason for
differences is the direction of operation; Crossplane needs to _publish to_
external stores whereas those tools targets the opposite, that is
_fetching from_ external stores.

Since we are mapping namespaced secret resources to a cluster scoped
StoreConfig, we need to handle same secret names coming from different
namespaces with some store specific scoping. This would be `namespace` for
Kubernetes, a parent directory for Vault or simply prefixing the name of the
secret if the secret store does not have a concept for scoping. We will have
a `spec.defaultScope` field in `StoreConfig` to be used for cluster scoped
resources.

**Examples:**

Publish to Vault under parent path `secret/my-cloud/dev/` using Kubernetes auth:

```yaml
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault-default
spec:
  type: Vault
  # defaultScope used for scoping secrets for cluster scoped resources.
  # For example, secrets for MRs will land under
  # "secret/my-cloud/dev/crossplane-system" path in Vault with this StoreConfig.
  # However, secret claims in `team-a` namespace will go to
  # "secret/my-cloud/dev/team-a".
  defaultScope: crossplane-system
  vault:
    server: "https://vault.acme.org"
    # parentPath is the parent path that will be prepended to the secrets
    # created with this store config.
    parentPath: "secret/my-cloud/dev/"
    version: "v2"
    caBundle: "..."

    auth:
      # Kubernetes auth: https://www.vaultproject.io/docs/auth/kubernetes
      kubernetes:
        mountPath: "kubernetes"
        role: "demo"
```

Publish with an _out-of-tree Secret Plugin_ (for future support, if needed):

```yaml
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: acme-secretstore
spec:
  type: Plugin
  defaultScope: crossplane-system
  plugin:
    name: plugin-x
    endpoint: unix:///tmp/plugin-x.sock
    config:
      host: secretstore.acme.org
      port: 9999
      caBundle: "..."
      some:
        other:
          arbitrary-config: true
```

### User Experience

With the API definition above, it is mostly clear how the user interaction would
be. But one important point that worth mentioning is, both Crossplane core pod
and provider pods would need to access to the external secret stores. The
credentials that is made available needs to be authorized to read, write and
delete the secrets living at the configured scope.

Here is an example flow for configuring Vault as an external secret store with
[Kubernetes Auth]:

1. [Configure] Vault for Kubernetes Auth for Crossplane and provider service
accounts in Crossplane namespace (e.g. `crossplane-system`) with permissions
to read, write and delete the secrets under a parent path
(e.g. `secret/my-cloud/dev/`).
2. Deploy [Vault sidecar injector] into the same cluster as Crossplane.
3. Add necessary annotations to get the Vault sidecar injected to
Crossplane/provider pods and make token available for the configured Vault role
(e.g. `crossplane`).
   1. Add annotations to the Crossplane core pod (needs to be exposed as a helm
parameter).
   2. Add annotations to the Provider pods (using `ControllerConfig`)
4. Create `StoreConfig` CRs as follows:

For core Crossplane:

```yaml
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault-default
spec:
  type: Vault
  defaultScope: crossplane-system
  vault:
    server: "https://vault.acme.org"
    parentPath: "secret/my-cloud/dev/"
    version: "v2"
    caBundle: "..."
    auth:
      kubernetes:
        mountPath: "kubernetes"
        role: "crossplane"
```

For the provider pod:

```yaml
apiVersion: aws.secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault-default
spec:
  type: Vault
  defaultScope: crossplane-system
  vault:
    server: "https://vault.acme.org"
    parentPath: "secret/my-cloud/dev/"
    version: "v2"
    caBundle: "..."
    auth:
      kubernetes:
        mountPath: "kubernetes"
        role: "crossplane"
```

5. Use the following `publishConnectionDetailsTo` for resources:

```yaml
spec:
  publishConnectionDetailsTo:
    name: <secret-name>
    configRef:
      name: vault-default
```

### Implementation

We will define a new interface, namely `ConnectionSecretStore`, which satisfies
slightly modified versions of the existing [ConnectionPublisher] and
[ConnectionDetailsFetcher] interfaces. This interface will be satisfied by any
secret store including the local Kubernetes. We will need this interface to be
defined in [crossplane-runtime repository] since both managed and Crossplane
composite reconcilers would use this interface. This will require some
refactoring since the existing interfaces defined in different
packages/repositories today.

```go
type ConnectionSecretStore interface {
	ConnectionDetailsPublisher
	ConnectionDetailsFetcher
}

type ConnectionDetailsPublisher interface {
	PublishConnection(ctx context.Context, p ConnectionSecretPublisher, c managed.ConnectionDetails) error
	UnpublishConnection(ctx context.Context, p ConnectionSecretPublisher, c managed.ConnectionDetails) error
}

type ConnectionDetailsFetcher interface {
	FetchConnectionDetails(ctx context.Context, p ConnectionSecretPublisher) (managed.ConnectionDetails, error)
}
```

Implementations of any function in this interface will first fetch `StoreConfig`
and configure its client before any read/write/delete. This is required to
ensure any changes in the `StoreConfig` resources to be reflected in the next
reconcile. Local Kubernetes store is an exception here since it already uses
in cluster config which does not depend on a `StoreConfig` and would use the
same client as it is doing today.

Other types to complete the picture:

```go
type ConnectionSecretPublisher interface {
	Object

	ConnectionSecretPublisherTo
}

type ConnectionSecretPublisherTo interface {
	SetPublishConnectionSecretTo(c *xpv1.ConnectionSecretConfig)
	GetPublishConnectionSecretTo() *xpv1.ConnectionSecretConfig
}

type ConnectionSecretConfig struct {
	Name string `json:"name"`
	Metadata map[string]any `json:"metadata"`
	ConfigRef *SecretStoreConfig `json:"configRef"`
}
```

External secret store support will be introduced in a phased fashion, with
initially being off by default behind a feature flag like
`--enable-alpha-external-secret-stores`.

### Bonus Use Case: Publish Connection Details to Another Kubernetes Cluster

By adding support for `Kubernetes` as an external secret store in `StoreConfig`,
we could enable publishing connection secrets to another Kubernetes cluster.
For example, platform consumers could publish database connection details they
were provisioned on control plane cluster to their existing application clusters
or even to a new Kubernetes cluster provisioned together with the database.

Example claim spec:

```yaml
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    configRef:
      name: kubernetes-cluster-1
```

Example StoreConfig:

```yaml
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: kubernetes-cluster-1
spec:
  type: Kubernetes
  defaultScope: crossplane-system
  kubernetes:
    namespace: backend-dev
    auth:
      kubeconfig:
        secretRef:
          name: "db-claim-cluster-conn"
          key: "kubeconfig"
```

## Future Considerations

### Reading Input Secrets from External Secret Stores

There are two types of input secrets in Crossplane, provider credentials and
sensitive fields in managed resource spec. These are provided with Kubernetes
secrets today, and intentionally left out of scope for this design to limit
the scope. However, the types and interfaces defined here would be leveraged
to satisfy these two cases which will result in a unified secret management with
external stores no matter it is input or output.

### Templating Support for Custom Secret Values

Not directly related to supporting external secret stores but thanks to the
extensible API proposed in this design, we might consider adding an interface
that supports adding new keys to connection secret content from existing
connection detail keys according to a given template (similar to
[Vault agent inject template]). This would be helpful to prepare a secret
content in an expected format like SQL connection strings.

Another possible use case is, combined with support for adding annotations to
Kubernetes secrets, creating a [ArgoCD cluster] could be enabled with a spec
as follows:

```yaml
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    metadata:
      annotations:
        acme.example.io/secret-type: infrastructure
    template: |
      name: my-crossplane-managed-cluster
      server: {{ .ConnectionDetails.Endpoint }}
      config: |
        {
          "bearerToken": "{{ .ConnectionDetails.Bearer }}",
          "tlsClientConfig": {
            "insecure": false,
            "caData": "{{ .ConnectionDetails.CABundle }}"
          }
        }
```

## Alternatives Considered

### Propagate from K8S secrets

Crossplane enables managing external resources from Kubernetes API via custom
resources. Considering each secret living in an external secret store, is indeed
an external resource which could be managed by Crossplane just like any other
managed resources, we could leverage Crossplane providers to create connection
details secrets in external secret stores.

For example, to store a connection detail secret in [AWS secret manager], all we
need to do is to create a [provider-aws Secret resource] with proper configuration
and reference to the Kubernetes secret with connection details. Similarly, to
store them in Vault, we would need to implement a _provider-vault_ and create a
[GenericSecret] resource.

This approach would allow us to avoid reimplementing ways to connect and
authenticate different providers at different layers and rely on ProviderConfigs
which is exactly responsible for this.

_Despite this being architecturally the cleanest solution, it has a major
limitation which is the need for storing sensitive information in a Kubernetes
secret which violates one of the main motivations of this design._

### Out-of-tree Support with a Plugin API

This option proposes a pluggable secret backend in upstream Crossplane which
would allow out-of-tree secret store plugins. When configured, Crossplane and
providers will communicate with the secret store plugin over gRPC. The plugin
would then be responsible for communicating with the secret store. We will
follow a similar approach as [KMS plugin support in Kubernetes API server].

In this option, we will only implement support for a plugin API in Crossplane
Core. Actual plugins and utilities to deploy and configure the environment
properly could be build independently, i.e. as separate components.

This approach has the advantage of not introducing hard dependencies to
Crossplane and also would be more scalable if/once we want to add support for
more secret stores. _However, this option introduces an upfront complexity
especially around deployment of Crossplane and providers mostly related to
securing the communication between plugins._

### Intercepting Secret Requests with an Admission Webhook or Proxy

The main motivation behind this approach is hiding the complexity of interacting
with external secret stores from Crossplane by putting an intermediate layer
between Crossplane and Kubernetes intercepting Secret operations. In this
option there will be no (or very minimal) changes in Crossplane, but the
heavy lifting will be handled by the Intermediate Layer.

There are two different approaches that we have considered in the scope of this
option:

- Intercept secret requests with _a Mutating Admission Webhook_: This approach
proposes deploying an admission webhook which intercepts secret requests,
writes the data to an external secret store and removes sensitive data from the
Kubernetes secret. Compared to the proxy approach below, this has the advantage
of using native Kubernetes mechanisms to intercept the request but requires some
workaround and minor changes in Crossplane since it is not possible to intercept
read requests.
- Intercept secret requests with a _Transparent Kubernetes API Server proxy_: This
approach proposes deploying a proxy in front of Kubernetes API Server which
would transparently proxy all incoming requests except Secrets. Similar to the
webhook approach, Secret requests would be processed by storing sensitive
information in an external secret store and removing sensitive data from the
Kubernetes Secret. Compared to the webhook approach, this has the advantage of
intercepting all secret requests including reads that would require less changes
in Crossplane codebase, however, intercepting all requests, even proxies
transparently, has the caveat of causing performance bottlenecks or connectivity
problems (especially considering long-running watch requests).

_While this option would require minimal changes in Crossplane and sounds
more attractive technically, in addition to performance concerns, we prefer to
be clear at Crossplane API level instead of hiding the information of where the
connection information actually landed._

### CSI Drivers

The idea of Crossplane writing secrets to the filesystem in the pod and a proper
CSI driver syncs these secrets to an external secret store sounds fancy. I have
investigated this possibility and came across
[kubernetes-sigs/secrets-store-csi-driver] repository which already supports
different secret backends like aws, gcp, azure and vault. _However, all the work
is around making a secret available in an external secret store in the
filesystem of the pod and not the opposite direction that we need here. It is
also debatable whether this would be possible at all and could not find any
related discussion or issue._

[encrypt secrets at rest]: https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/
[rejected for various reasons]: https://github.com/kubernetes/kubernetes/issues/75899#issuecomment-484731795
[quite a few tools]: https://github.com/external-secrets/kubernetes-external-secrets/issues/47
[the standardization efforts]: https://github.com/external-secrets/external-secrets/blob/9e3914b944955bf07c2700a00b85091de560995e/design/design-crd-spec.md#summary
[existing schema]: https://external-secrets.io/api-secretstore/
[ExternalSecret spec]: https://external-secrets.io/api-externalsecret/
[that design]: https://github.com/external-secrets/external-secrets/blob/9e3914b944955bf07c2700a00b85091de560995e/apis/externalsecrets/v1alpha1/externalsecret_types.go#L120
[Vault]: https://www.vaultproject.io
[generated name]: https://github.com/crossplane/crossplane/blob/8204b777edf7a60cff08fd99399671e91f6c00d2/internal/controller/apiextensions/composite/api.go#L403
[Vault credential injection guide]: https://crossplane.io/docs/v1.3/guides/vault-injection.html
[injecting via Vault agent]: https://learn.hashicorp.com/tutorials/vault/kubernetes-sidecar
[External Secrets]: https://github.com/external-secrets/external-secrets/
[secret-store-runtime]: images/secret-store-runtime.png
[secret-store-runtime-example]: images/secret-store-runtime-example.png
[KMS plugin support in Kubernetes API server]: https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/#implementing-a-kms-plugin
[Vault sidecar injector]: https://www.vaultproject.io/docs/platform/k8s/injector
[Kubernetes Auth]: https://www.vaultproject.io/docs/auth/kubernetes
[Configure]: https://www.vaultproject.io/docs/auth/kubernetes#configuration
[ConnectionPublisher]: https://github.com/crossplane/crossplane-runtime/blob/bf5d5512c2f236535c7758f3eaf59c5414c6cf78/pkg/reconciler/managed/reconciler.go#L108
[ConnectionDetailsFetcher]: https://github.com/crossplane/crossplane/blob/ed06be3612b4993a977e3846bfeb9f1930032617/internal/controller/apiextensions/composite/reconciler.go#L162
[crossplane-runtime repository]: https://github.com/crossplane/crossplane-runtime
[Vault agent inject template]: https://learn.hashicorp.com/tutorials/vault/kubernetes-sidecar#apply-a-template-to-the-injected-secrets
[ArgoCD cluster]: https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#clusters
[AWS secret manager]: https://aws.amazon.com/secrets-manager/
[provider-aws Secret resource]: https://github.com/crossplane/provider-aws/blob/master/examples/secretsmanager/secret.yaml
[GenericSecret]: https://registry.terraform.io/providers/hashicorp/vault/latest/docs/resources/generic_secret
[kubernetes-sigs/secrets-store-csi-driver]: https://github.com/kubernetes-sigs/secrets-store-csi-driver

# External Secret Stores

* Owner: Hasan Türken (@turkenh)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Kubernetes suggests using `Secret` resource to store sensitive information and
one gets a base64 encoded value of sensitive data rather than encrypted. RBAC is
used as a security measure to access control, instead of dealing with a
decryption process whenever this data needed to be consumed. However, in 
practice it is not easy to ensure proper access to sensitive data with RBAC. 
For example, when an application requires read/write access to secrets in a
namespace, it is usually deployed with access to any secret in the namespace
which could result in unexpected access to another secret in the same namespace.

Kubernetes has a solution to [encrypt secrets at rest], however, this is only to
keep secret data as encrypted in etcd which changes nothing at Kubernetes API
level. Requests for supporting additional external secret stores at API level
were [rejected for various reasons]. Another point is, Kubernetes clusters 
usually considered as ephemeral resources as opposed to being a reliable data
store not only for sensitive data but also non-sensitive data like application
manifests which helps popularity of GitOps tools.

Hence, storing sensitive information in external secret stores is a common
practice. Since applications running on K8S need this information as
well, it is also quite common to sync data from external secrets store to K8S.
There are [quite a few tools] out there that are trying to resolve this exact same
problem. However, Crossplane, as a producer of infrastructure credentials,
needs the opposite, which is storing sensitive information **to external secret
stores**.

Today, Crossplane only supports storing connection details for managed resources
as Kubernetes secrets. This is configured via `spec.writeConnectionSecretToRef`
field. However, there is an increasing demand to store infrastructure
credentials on external secret stores rather than relying on Kubernetes secrets.

This document aims to propose a design for supporting external secret stores in
Crossplane. The solution should apply to both Crossplane itself (for composite
connection details) and providers (for managed resource connection details).
Throughout the document, we will mostly focus on [Vault] as the most popular
secret store today.

### Goals

We would like to come up with a design that:

- Will not break the existing APIs.
- Will support adding new Secret Stores without any API changes.
- Will support switching to an out-of-tree plugin model without breaking the
  API for existing in-tree Secret Stores.

We would like Crossplane to be able to store connection details to external
secret stores and still satisfy the following user stories:

- As a platform operator, I would like to configure how/where to store 
  connection details for managed and composite resources.
- As a platform consumer, I would like to configure how/where to store
  connection details for my composite resource claims.

For dynamically provisioned resources, it should still be possible to pass
partial inputs for connection details secret:

- In Composition spec, configuring where/how to store connection details for
composite resources (i.e. `writeConnectionSecretsToNamespace`)
- Assigning a [generated name] to connection secrets of composite resources.


## Out of Scope

This design focuses on writing/publishing connection details to external stores
and not about reading/consuming them in the form of **provider credentials**
or **to consume via the workloads**. There are already ways to consume secrets
in external stores from Kubernetes and is out of the scope for this document.
See [Vault credential injection guide] to see how one can configure Vault and
Crossplane to consume provider credentials from Vault. To consume secrets living
inside Vault from workloads running in Kubernetes, one can consider
[injecting via Vault agent] or use solutions like [External Secrets].

One exception for reading secrets back is, Crossplane itself needs to read
connection details of managed resources to build secrets for composite
resources and this will be covered in the design.

## Design

A secret is a resource storing a set of sensitive information typically as key
value pairs. To access a secret instance we would need the following
information:

- An identifier which uniquely identifies the secret instance within the
Store (a.k.a. `external-name`). This identifier could be split into two
different parts:
  - **A name**: A unique name within logical group.
  - **A scope/group identifier**: Specific to Secret Store. For example,
  `namespace` in _Kubernetes_, a parent `path` in _Vault_ and `account`+`region`
  for _AWS Secret Manager_.
- **Additional configuration** to reach to the Store like its endpoint and 
credentials to authenticate/authorize. For example, a `kubeconfig` for 
_Kubernetes_, a `server` endpoint + auth config for _Vault_ and access/secret
keys for _AWS Secret Manager_.

### API

#### PublishConnectionDetailsTo

We will deprecate existing `writeConnectionSecretToRef` field in favor of
`publishConnectionDetailsTo` field which would support publishing connection
details to the local Kubernetes cluster **and/or** to an external Secret Store.
The API would only allow publishing to a single external store but would
still allow publishing to an external store while still writing into a local
Kubernetes secret. This could be helpful when the motivation behind storing in
external store is _reliability_ rather than _security_. As an example use case,
a platform consumer, might want to consume database creds directly in the team
namespace but still want it to be saved in a more reliable store for disaster
recovery or backup purposes.

We will only take the **name** of secret in our secret specification API
(i.e. `publishConnectionDetailsTo`) and the rest will go to a separate config
(`StoreConfig`). This classification enables building a flexible API that
satisfies the separation of concerns between platform operators and consumers
which Crossplane already enables today for Kubernetes Secrets. We will end up
having a unified spec for all Secret Store types which contains the name field
(`name`) and a reference to any additional configuration (`configRef`). 

This would be enough to uniquely identify and access any secret instance,
however there are still some additional attributes or metadata specific to
Secret Store type that might be desired to be set per secret instance. For
example, `labels`, `annotations` and `type` of the secret in _Kubernetes_ and;
`Tags` and `EncryptionKey` in _AWS_. For local Kubernetes, this could be
supported by a strong typed spec, however, for external stores, to stick with a
unified interface, we will receive this configuration as arbitrary key value
pairs with `attributes`.

**Examples:**

Publish Connection details to a Kubernetes secret (already existing case):

```
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    kubernetes:
      namespace: crossplane-system
```

Publish Connection details to a _Vault_ secret:

```
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    externalStore:
      configRef:
        name: vault-default
        namespace: crossplane-system
```

Publish Connection details to a Kubernetes secret with some labels:

```
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    kubernetes:
      namespace: crossplane-system
      labels:
        environment: production
      annotations:
        acme.example.io/secret-type: infrastructure
```

Publish Connection details to a AWS Secret Manager with some tags:

```
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    externalStore:
      configRef:
        name: aws-secret-manager-platform
        namespace: crossplane-system
      attributes:
        tags:
          environment: production
```

Publish Connection details to Vault but still store it as a Kubernetes secret:

```
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    kubernetes:
      namespace: crossplane-system
    externalStore:
      configRef:
        name: vault-platform
        namespace: crossplane-system
```

#### Separation of Concerns: Compositions, Composites and Claims

`StoreConfig` resource will be a _namespaced_ resource to allow platform
consumers to configure their own secret store of choice. This is aligned with
the existing approach since Kubernetes secrets are namespaced resources and
platform operators could specify where should composite resource secrets land
using the partial input `writeConnectionSecretsToNamespace`. We will similarly
deprecate this field in favor of `publishConnectionDetailsToStore`.

**Examples:**

Composition configuring the Kubernetes namespace that connection secrets
for Composite resources will be stored.

```
spec:
  publishConnectionDetailsToStore:
    kubernetes:
      namespace: crossplane-system
```

Composition configuring the secret store that connection secrets for
Composite resources will be stored.

```
spec:
  publishConnectionDetailsToStoreConfigRef:
    name: vault-platform
    namespace: crossplane-system
```

Claim spec for writing connection secret in the same Kubernetes namespace:

```
spec:
  publishConnectionDetailsTo:
    name: database-creds # when neither `kubernetes` nor `externalStore` configs
                         # provided, it defaults to "kubernetes in the same
                         # namespace", same as "kubernetes: {}"
```

Claim spec for writing connection secret to Vault:

```
spec:
  publishConnectionDetailsTo:
    name: database-creds
    externalStore:
      configRef:
        name: vault-team-a
```


Claim spec for writing connection secret to both Kubernetes and Vault:

```
spec:
  publishConnectionDetailsTo:
    name: database-creds
    externalStore:
      configRef:
        name: vault-team-a
        # please note, there is no namespace option here, platform consumers
        # only have access to their team namespaces, so the StoreConfig in the
        # same namespace as Claim will be used.
```

#### Secret StoreConfig

Secret store config will contain the required information other than the name
of secret. Thanks to [the standardization efforts] on a declarative API for
syncing secrets from external stores into Kubernetes, there is already an
[existing schema] that we can follow here. However, there will be slight 
differences since we want to receive the `name` of secret as input in our API
whereas, they expect a full identifier of the secret instance within the secret
store (in [ExternalSecret spec] at `spec.dataFrom.key`). Another reason for
differences is the direction of operation, Crossplane needs to store to external
Store whereas those tools targets the opposite.

**Examples:**

Publish to Vault under parent path `secret/my-cloud/dev/` using Kubernetes auth:

```
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault-platform
  namespace: crossplane-system
spec:
  type: Vault
  vault:
    server: "https://vault.acme.org"
    # parentPath is the parent path that will be prepended to the secrets
    # created with this store config.
    parentPath: "secret/my-cloud/dev/"
    # Version is the Vault KV secret engine version.
    # This can be either "v1" or "v2", defaults to "v2"
    version: "v2"
    caBundle: "..."
    
    auth:
      # Kubernetes auth: https://www.vaultproject.io/docs/auth/kubernetes
      kubernetes:
        mountPath: "kubernetes"
        role: "demo"
```

Publish with an out-of-tree Secret Plugin (for future support, if needed):

```
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault-platform
  namespace: crossplane-system
spec:
  type: Plugin
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
4. Create a `StoreConfig` CR as follows:

```
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault-default
  namespace: crossplane-system
spec:
  type: Vault
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

```
spec:
  publishConnectionDetailsTo:
    name: <secret-name>
    externalStore:
      configRef:
        name: vault-default
        namespace: crossplane-system
```

### Implementation



### Bonus Use Case: Publish Connection Details to Another Kubernetes Cluster

By adding support for `kubernetes` as an external secret store in `StoreConfig`,
we could enable publishing connection secrets to another Kubernetes cluster.
For example, platform consumers could publish database connection details they
were provisioned on control plane cluster to their existing application clusters
or even to a new Kubernetes cluster provisioned together with the database.

Example claim spec:

```
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    externalStore:
      configRef:
        name: kubernetes-cluster-1
```

Example StoreConfig:

```
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: kubernetes-cluster-1
  namespace: team-a
spec:
  type: Kubernetes
  kubernetes:
    namespace: backend-dev
    auth:
      kubeconfig:
        secretRef:
          name: "db-claim-cluster-conn"
          key: "kubeconfig"
```

## Future Considerations

### Templating support for custom secret values 

Not directly related to supporting external secret stores but thanks to
extensible API proposed in this design, we might consider adding an interface
that supports adding new keys to connection secret content from existing
connection detail keys according to a given template (similar to
[Vault agent inject template]). This would be helpful to prepare a secret
content in an expected format like SQL connection strings. 

Another possible use case is, combined with support for adding annotations to
Kubernetes secrets, creating a [ArgoCD cluster] could be enabled with a spec
as follows:

```
spec:
  publishConnectionDetailsTo:
    name: my-db-connection
    kubernetes:
      namespace: crossplane-system
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
would then be responsible for communicating with the Secret Store. We will
follow a similar approach as [KMS plugin support in Kubernetes API server].

In this option, we will only implement support for a plugin API in Crossplane
Core. Actual plugins and utilities to deploy and configure the environment
properly could be build independently, i.e. as separate components.

This approach has the advantage of not introducing hard dependencies to 
Crossplane and also would be more scalable if/once we want to add support for
more Secret Stores. _However, this option introduces an upfront complexity
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
Kubernetes secret. Compared to proxy approach, this has the advantage of using
Kubernetes mechanisms to intercept the request but requires some workaround and
minor changes in Crossplane since it is not possible to intercept read requests.
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

The idea of crossplane writing secrets to the filesystem in the pod and a proper
CSI driver syncs these secrets to an external secret store sounds fancy. I have
investigated this possibility and came across
[kubernetes-sigs/secrets-store-csi-driver] repository which already supports
different secret backends like aws, gcp, azure and vault. _However, all the work
is around making a secret available in an external secret store in the
filesystem of the pod and not the opposite direction that we need here. It is
also debatable whether this would be possible at all and could not find any
related discussion or issue._

[encrypt secrets at rest]: (https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)
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
[Vault agent inject template]: https://learn.hashicorp.com/tutorials/vault/kubernetes-sidecar#apply-a-template-to-the-injected-secrets
[ArgoCD cluster]: https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#clusters
[AWS secret manager]: https://aws.amazon.com/secrets-manager/
[provider-aws Secret resource]: https://github.com/crossplane/provider-aws/blob/master/examples/secretsmanager/secret.yaml
[GenericSecret]: https://registry.terraform.io/providers/hashicorp/vault/latest/docs/resources/generic_secret
[kubernetes-sigs/secrets-store-csi-driver]: https://github.com/kubernetes-sigs/secrets-store-csi-driver
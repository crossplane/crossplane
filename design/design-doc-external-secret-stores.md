# External Secret Stores

* Owner: Hasan Turken (@turkenh)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Today, Crossplane only supports storing connection details for managed resources
as Kubernetes secrets. This is configured via spec.writeConnectionSecretToRef
field. However, there is an increasing demand to store infrastructure
credentials on external secret stores rather than relying on Kubernetes secrets.

Main motivations behind this demand are:
- Security: Kubernetes secrets are known to be insecure since they store data
as an unencrypted base64-encoded string. Proper access control via RBAC is
usually too hard if not impossible.
- Reliability: Most users consider Kubernetes clusters as ephemeral resources
which could be replaced with a new one (e.g. via GitOps) for some reason.
Using on Kubernetes API to store infrastructure credentials is not a reliable
approach in this sense.

In this document we will investigate alternative options to support additional
secret stores in Crossplane. Throughout the document, we will mostly focus on
[Vault] as the most popular secret store today.

## Goals

The goal of this document is to investigate alternative approaches to support
external secret stores in Crossplane to store connection details. The solution
should apply to both Crossplane itself (for composite connection details) and
providers (for managed resource connection details).

## Out of Scope

This document focuses on writing/publishing connection details to external
stores and not about reading/consuming them in the form of provider credentials
or to consume via the workloads. There are already ways to consume secrets in
external stores from Kubernetes and is out of the scope for this document.

See [Vault credential injection guide] to see how one can configure Vault and
Crossplane to consume provider credentials from Vault. To consume secrets living
inside Vault from workloads running in Kubernetes, one can consider
[injecting via Vault agent] or use solutions like [Vault secrets operator].

## Design

There are a couple of alternative approaches that we would affect the design.
We would like to list them all here and finally agree on one of them.

### Option 1: Supporting Additional Stores in Crossplane-runtime

This is the most straightforward approach. We will implement support for writing
to external stores in _crossplane-runtime_ via importing and using their 
clients. With this solution, Crossplane and providers would immediately get
this feature when configured properly.

![Secret Store Runtime][secret-store-runtime]

#### API

We will deprecate existing `writeConnectionSecretToRef` field in favor of a
`publishConnectionDetailsTo` field which would support configuring multiple
secret backends. This will apply to not only managed resources but also to
_Composite Resources_ and _Composite Resource Claims_.

```
apiVersion: widgets.crossplane.io/v1beta1
kind: Example
metadata:
  name: my-cool-managed-resource
spec:
  forProvider:
    coolness: 24
  publishConnectionDetailsTo:
  - store: KubernetesSecret
    kubernetesSecret:
      secretRef:
        name: my-cool-managed-resource-connection
        namespace: secrets
  - store: Vault
    vault:
      path: /path/to/secret/my-cool-managed-resource-connection
      configRef:
        name: vault-primary
```

For compositions, we will similarly deprecate existing 
`writeConnectionSecretsToNamespace` in favor of the same 
`publishConnectionDetailsTo` field. This would be used for any partial input
that needs to be enforced/provided by the composition authors.

```
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: compositeclusters.aws.platformref.crossplane.io
spec:
  publishConnectionDetailsTo:
  - kubernetes:
      secretRef:
        namespace: secrets
  - vault:
      configName: vault-primary
      path: /path/to/secret/my-cool-composite-resource-connection
  compositeTypeRef:
    apiVersion: aws.platformref.crossplane.io/v1alpha1
    kind: CompositeCluster
  resources:
    - base:
        apiVersion: aws.platformref.crossplane.io/v1alpha1
        kind: EKS
      connectionDetails:
        - fromConnectionSecretKey: kubeconfig
      patches:
	...
```

#### Configuring External Secret Stores

To configure an external secret store, we would need to know how to connect 
and authenticate to it. This is indeed an already solved problem in Crossplane.
Providers use ProviderConfig resources to connect and authenticate to external
providers. For any external secret store, we will need a similar configuration.
For example, if the secret store is an AWS secret manager, we will need to
communicate with AWS API similar to provider-aws. Since this time we would need
to connect at crossplane-runtime level (from any controller), we might consider
migrating connection logic for providers to the crossplane-runtime repository.
However, in the beginning, we will start by making some duplication and consider
finding ways to reuse the connection logic in providers once we better figure
out what it really means.

To enable/configure an external secret store, user would need to create the
following custom resource:

```
apiVersion: secrets.crossplane.io/v1alpha1
kind: StoreConfig
metadata:
  name: vault-primary
Spec:
  type: Vault
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: vault-creds
      key: config
```

#### Example Deployment for Using Vault as Secret Store

1. User deploys [Vault sidecar injector] to the cluster.
2. User configures proper Vault authentication, e.g. Kubernetes auth
3. User deploys Crossplane and provider controllers with the following
annotations:
   ```
   vault.hashicorp.com/agent-inject: "true"
   vault.hashicorp.com/agent-inject-token: "true"
   ```
4. User creates a Kubernetes secret with the following config:
   ```
   {
    "vault-address": "https://vault-cluster.vault-system:8200",
    "vault-token-file": "/vault/secrets/token"
   }
   ```
5. User creates a Secret `StoreConfig` with type `Vault` referring to the above
secret.
6. Crossplane and provider controllers would publish connection details to Vault
if requested in the managed resource API (`publishConnectionDetailsTo` includes
`Vault` and `configRef.name` is set as `vault-primary`).

![Secret Store Runtime Example][secret-store-runtime]

### Option 2: Out-of-tree Support with a Plugin API

We will implement a pluggable secret backend in upstream Crossplane which would
allow out-of-tree secret store plugins. When configured, crossplane and
providers will communicate with the secret store plugin over gRPC. The plugin
would then be responsible for communicating with the Secret Store. We will
follow a similar approach as [KMS plugin support in Kubernetes API server].

![Secret Store Plugin][secret-store-plugin]

In this option, we will only implement support for a plugin API in Crossplane
Core. Actual plugins and utilities to deploy and configure the environment
properly could be build independently, i.e. as separate components.

To avoid implementation of authentication and authorization with the plugins,
communication would be over _unix domain sockets_. This would require the plugin
to be included in each controller pod as a sidecar.

We will need to implement the following 3 pieces:

**Plugin API in upstream Crossplane (in Crossplane Runtime)**

It should be possible to configure Crossplane and providers with a secret store
configuration. This would register the secret store during initialization and
will be available to publish connection details if requested via resource APIs.

**Plugins for different secret stores**

These plugins would implement a well known API and serve with gRPC.
Communication and authentication with the Secret Store would be the
responsibility of this component. Plugins would authenticate to external secret
stores using a configuration file or environment variables.

**Mutating webhook injecting plugin sidecars with proper configuration**

When enabled/configured, this component would be responsible for injecting
plugin sidecars with proper configuration.

#### API

API will be similar to option 1 but not the same since we would no longer be
able to have strong types per secret store option. In the API, users would
specify the name of the secret store which should match the registered store
name and arbitrary key value pairs which would be required by the selected
secret store. Store `kubernetes` would be available by default which would
provide previous behaviour to keep connection details in Kubernetes secrets.

```
apiVersion: widgets.crossplane.io/v1beta1
kind: Example
metadata:
  name: my-cool-managed-resource
spec:
  forProvider:
    coolness: 24
  publishConnectionDetailsTo:
  - store: kubernetes
      secretName: my-cool-managed-resource-connection
      secretNamespace: secrets
  - store: vault
      path: /path/to/secret/my-cool-managed-resource-connection
```

#### Example Deployment for Using Vault as Secret Store

1. User deploys [Vault sidecar injector] to the cluster.
2. User configures proper Vault authentication, e.g. Kubernetes auth
3. User deploys **Crossplane Secret Store Plugin Webhook** with proper
configuration:
   1. Annotate with:
      ```
      vault.hashicorp.com/agent-inject: "true"
      vault.hashicorp.com/agent-inject-token: "true"
      ```
   2. Enable Vault plugin with:
      ```
      VAULT_ADDRESS=https://vault-cluster.vault-system:8200
      VAULT_TOKEN_FILE=/vault/secrets/token
      ```
4. User deploys Crossplane and provider controllers.
5. **Crossplane Secret Store Plugin Webhook** injects plugin sidecar and also
registers plugin by passing plugin config flags to crossplane and provider
controllers.
6. Crossplane and provider controllers registers Vault as a secret store at
startup.
7. Crossplane and provider controllers would publish connection details to Vault
if requested in the managed resource API (`publishConnectionDetailsTo` includes
`Vault` as store).

![Secret Store Plugin Example][secret-store-plugin-example]

### Option 3: Propagate from K8S secrets

_In this option we assume that it is ok to keep storing connection details as
Kubernetes secrets (temporarily or permanently) as long as they will also be
propagated to external secret stores if configured/requested._

Crossplane enables managing external resources from Kubernetes API via custom
resources. Considering each secret living in an external secret store is indeed
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

![Secret Store Propagate][secret-store-propagate]

#### API

This will be controlled via annotations on managed or composite resources and no
API change would be required. To propagate a connection secret to a supported
secret store, users would need to add the following annotation on the resource:

```
runtime.crossplane.io/propagate-connection-secret-to: <supported-secret-store>
```

For example:

```
runtime.crossplane.io/propagate-connection-secret-to: vault
runtime.crossplane.io/propagate-connection-secret-path: /path/prefix
```

In Crossplane runtime we will implement the following interface per secret store
we want to support:

```
type ConnectionPropagator interface {
	PropagateConnection(mg resource.Managed) *unstructured.Unstructured
}
```

A Vault implementation of `ConnectionPropagator` would return a corresponding
`GenericSecret` custom resource for a given managed resource in the 
`PropagateConnection` function. When the annotation is found for a supported
secret store in a managed resource, a custom resource of the secret store would
be created in addition to the Kubernetes secret.

We might consider adding support for deleting kubernetes secret after
propagation with another annotation which would mean we will only use Kubernetes
secrets as a temporary medium to propagate secret to the actual store:

```
runtime.crossplane.io/delete-secret-after-propagate: true
```

#### Example Deployment for Using Vault as Secret Store

1. User deploys crossplane and providers including _provider-vault_.
2. User configures _provider-vault_ with a `ProviderConfig`.
3. User annotates a managed resource to propagate its connection details to 
Vault.
4. Once the managed resource is ready, a Vault `GenericSecret` custom resource
created in addition to the connection details Kubernetes secret.
5. _provider-vault_ stores secret in Vault.

## Conclusion

The problem of publishing connection details to external secret stores is a
similar problem that Crossplane already solves but at a different layer.
**Option 3** leverages most of the existing mechanisms but requires secrets to
be stored in Kubernetes, at least for some time with a possible
_delete-after-propagation_ option.

When we want to go with an out-of-tree plugin based solution, we cannot use
Crossplane’s existing extensibility solution, which is providers, if 
communication over k8s api (via secrets) is not acceptable. **Option 2** tries
to find a solution by introducing a new plugin solution while also trying to
keep things simple by not introducing an additional authentication and
authorization problem by suggesting a communication over unix domain sockets.
However, since all providers and Crossplane itself require this functionality,
deploying plugins as sidecars introduces another complexity which could be
solved with a sidecar injecting mutating webhook.

**Option 1**, as the most straightforward solution, suffers from the need for
solving the same configuration problem that providers already solve with
ProviderConfigs to authenticate to external stores. This option would also
require most changes in the runtime and also introduce dependencies to third
party clients like vault or aws to the crossplane-runtime repository.

## Alternatives Considered

### Go Plugins

To support out-of-tree secret store plugins, we might use the
[go plugin package]. This way, we could enable plugins by putting them in
provider images at a predefined location. However, go plugin package seems to
have a lot of constraints which would make working with plugins an unpleasant
experience.

See [this reddit thread] and [this blog post]:

> That’s all about plugins; make sure to not use them, and discourage your
> co-workers from doing so. They are against Go’s philosophy as a
> statically-linked language, they are clunky, need special environment for
> building and maintaining, play a lot with interface{} type casts, and can panic
> unexpectedly.


### CSI Drivers

The idea of crossplane writing secrets to the filesystem in the pod and a proper
CSI driver syncs these secrets to an external secret store sounds fancy. I have
investigated this possibility and came across
[kubernetes-sigs/secrets-store-csi-driver] repository which already supports
different secret backends like aws, gcp, azure and vault. However, all the work
is around making a secret available in an external secret store in the
filesystem of the pod and not the opposite direction that we need here. It is
also debatable whether this would be possible at all and could not find any
related discussion or issue.


[Vault]: https://www.vaultproject.io
[Vault credential injection guide]: https://crossplane.io/docs/v1.3/guides/vault-injection.html
[injecting via Vault agent]: https://learn.hashicorp.com/tutorials/vault/kubernetes-sidecar
[Vault secrets operator]: https://github.com/ricoberger/vault-secrets-operator
[secret-store-runtime]: images/secret-store-runtime.png
[secret-store-runtime-example]: images/secret-store-runtime-example.png
[KMS plugin support in Kubernetes API server]: https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/#implementing-a-kms-plugin
[secret-store-plugin]: images/secret-store-plugin.png
[Vault sidecar injector]: https://www.vaultproject.io/docs/platform/k8s/injector
[secret-store-plugin-example]: images/secret-store-plugin-example.png
[AWS secret manager]: https://aws.amazon.com/secrets-manager/
[provider-aws Secret resource]: https://github.com/crossplane/provider-aws/blob/master/examples/secretsmanager/secret.yaml
[GenericSecret]: https://registry.terraform.io/providers/hashicorp/vault/latest/docs/resources/generic_secret
[secret-store-propagate]: images/secret-store-propagate.png
[go plugin package]: https://pkg.go.dev/plugin
[this reddit thread]: https://www.reddit.com/r/golang/comments/b6h8qq/is_anyone_actually_using_go_plugins/
[this blog post]: https://tpaschalis.github.io/golang-plugins/
[kubernetes-sigs/secrets-store-csi-driver]: https://github.com/kubernetes-sigs/secrets-store-csi-driver
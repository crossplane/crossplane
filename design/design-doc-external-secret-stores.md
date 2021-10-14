# External Secret Stores

* Owner: Hasan Turken (@turkenh)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Today, Crossplane only supports storing connection details for managed resources
as Kubernetes secrets. This is configured via `spec.writeConnectionSecretToRef`
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
[injecting via Vault agent] or use solutions like [External Secrets] and
[Vault secrets operator].

## Design

We will implement support for writing to external stores in _crossplane-runtime_
via importing and using their clients. With this solution, Crossplane and
providers would immediately get this feature when configured properly.

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

## Alternatives Considered

### Propagate from K8S secrets

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


[Vault]: https://www.vaultproject.io
[Vault credential injection guide]: https://crossplane.io/docs/v1.3/guides/vault-injection.html
[injecting via Vault agent]: https://learn.hashicorp.com/tutorials/vault/kubernetes-sidecar
[External Secrets]: https://github.com/external-secrets/external-secrets/
[Vault secrets operator]: https://github.com/ricoberger/vault-secrets-operator
[secret-store-runtime]: images/secret-store-runtime.png
[secret-store-runtime-example]: images/secret-store-runtime-example.png
[KMS plugin support in Kubernetes API server]: https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/#implementing-a-kms-plugin
[Vault sidecar injector]: https://www.vaultproject.io/docs/platform/k8s/injector
[AWS secret manager]: https://aws.amazon.com/secrets-manager/
[provider-aws Secret resource]: https://github.com/crossplane/provider-aws/blob/master/examples/secretsmanager/secret.yaml
[GenericSecret]: https://registry.terraform.io/providers/hashicorp/vault/latest/docs/resources/generic_secret
[kubernetes-sigs/secrets-store-csi-driver]: https://github.com/kubernetes-sigs/secrets-store-csi-driver
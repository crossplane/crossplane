# Package `ImageConfig` API for Crossplane Packages

* Owner: Hasan Türken (@turkenh)
* Reviewers: @negz, @phisco
* Status: Draft

## Background

Crossplane packages are distributed as OCI images and hosted in container
registries. The Crossplane package manager fetches these images from the
registries and deploys them into the cluster. A Crossplane package may also have
dependencies on other packages, which are resolved by the package manager and
deployed alongside the primary package. This process works seamlessly when the
images are hosted in public registries. However, when images are stored in
private repositories, the package manager requires credentials to pull the
images. While the package installation APIs provide a way to specify pull
secrets, this only applies to the primary package, not its dependencies.

Currently, there is no proper mechanism to configure credentials for
dependencies at runtime. The [existing workaround] involves assigning pull
secrets to the Crossplane Service Account via Helm API during installation. This
approach is far from ideal, as it requires the user to have control over
Crossplane’s installation, which is often impractical, especially in managed
environments.

Beyond authentication, ensuring the integrity and authenticity of the images is
a key consideration. In modern software delivery, it’s common for images to be
signed, providing a way to verify their source and integrity. The package
manager should have the capability to verify these signatures when the images
are signed and when signature verification is configured. Similarly, such
verification policies need to be applied not only to the primary package but
also to any dependencies, ensuring a consistent level of security across all
images being deployed to the cluster.

These requirements highlight the need for an image-centric configuration API
that allows users to define settings for package images, regardless of how or
when the packages are installed. We anticipate additional use cases where users
may want to configure the package manager with more advanced settings around
image management. This API should centralize such configurations rather than
passing them individually through the installation APIs.

### Prior Art on Image Verification

Verification of image signatures is a common practice in the container ecosystem
and doing so in a Kubernetes environment is not unique to Crossplane. The two
outstanding solutions in this regard are [Policy Controller] and [Kyverno].
Both projects provide a way to enforce policies at the admission level with the
APIs they expose.

Policy Controller is a project from [Sigstore], the same organization that
maintains the [cosign] tool for signing and verifying container images. Through
its `ClusterImagePolicy` API, Policy Controller allows users to define policies
for verifying images with a wide range of options. As of today, Policy
Controller only operates on Pods or the native Kubernetes resources resulting in
a Pod, e.g. Deployments, StatefulSets, etc. There is no way to configure it to
work with Crossplane packages.

Kyverno, on the other hand, is a general-purpose policy engine for Kubernetes
that allows users to define policies for any Kubernetes resource. Using its
`ClusterPolicy` API, one can define [rules to verify images] and by leveraging
the `imageExtractors` field, it is possible to configure it to work with
Crossplane packages. See the [using Kyverno for package image verification]
section for more details.

Another example in this space is the project [Flux]. Compared to the previous
APIs, this project provides a [minimalistic API to verify images] with the
`OCIRepository` API, marking some parts of the API (e.g. keyless verification)
as experimental.

In Crossplane itself, we had [a previous proposal] aiming to solve signature
verification for packages but mostly focusing on the implementation details,
primarily for verification using a shared public key. The follow-up
[implementation PR] extended the solution also to support keyless verification
by mostly following the same solution / similar API as Flux by extending the
Package installation APIs for verification settings, leaving the dependencies
out of the scope. This proposal aims to provide a more comprehensive solution by
introducing an API that can be extended to support various verification settings
and also covering the ones coming as a dependency.

## Goals

- Allow configuration of credentials for package images and their dependencies
  at runtime.
- Enable users to set policies for verifying image signatures.
- Define an image-centric configuration API that centralizes settings for
  package images.

## Proposal

This document proposes a new API, `ImageConfig`, under the `pkg.crossplane.io`
API group, that allows users to configure settings for package images. The API
enables users to define rules for matching images and configuring how to
interact with the registries hosting the images, including authentication and
TLS settings. It also provides a way to define policies for verifying image
signatures.

### API

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: ImageConfig
metadata:
  name: acme-packages
spec:
  matchImages:
    - type: Prefix
      prefix: registry1.com/acme-co/configuration-foo
    - type: Prefix
      prefix: registry1.com/acme-co/configuration-bar
    - type: Prefix
      prefix: registry1.com/acme-co/function-baz
  registry:
    authentication:
      pullSecretRef:
        name: acme-registry-credentials
    tls:
      mode: Strict # Defaults to Strict, other values are Insecure, or Disabled
      caBundleConfigMapRef:
        name: registry1-ca
        key: ca.crt
  verification:
    provider: Cosign
    cosign:
      authorities:
        - name: verify acme builds
          keyless:
            url: https://fulcio.sigstore.dev
            identities:
              - issuer: https://token.actions.githubusercontent.com
                subjectRegExp: https://github.com/acme-co/crossplane-packages/*
          attestations:
            - name: verify attestations
              predicateType: spdxjson
```

The `ImageConfig` API `spec` has the following fields:

- `matchImages`: A list of rules to match images. The package manager will
  apply the settings defined in the `ImageConfig` object to the images that
  match the rule. We will start with a single match type, `Prefix`, which
  matches the image prefix. In the future, we may introduce more match types
  like `Glob` or `Regex`.
- `registry`: Configuration for interacting with the registry hosting the images.
  - `authentication`: Authentication settings for the registry.
    - `pullSecretRef`: Reference to the Kubernetes Secret containing the
      credentials to pull images from the registry. This secret must be of type
      [`kubernetes.io/dockerconfigjson`].
  - `tls`: TLS settings for the registry.
    - `mode`: The TLS mode to use for connecting to the registry. The default
      value is `Strict`. Other possible values are `Insecure` and `Disabled`.
    - `caBundleConfigMapRef`: Reference to the ConfigMap containing the CA
      certificate bundle to use for verifying the server's certificate.
- `verification`: Configuration for verifying image signatures.
  - `provider`: The provider to use for verifying image signatures. In the
    beginning, only `Cosign` will be supported.
  - `cosign`: Configuration for verifying images using cosign.
    - `authorities`: List of authorities to use for verifying images.
      - `name`: The name of the authority.
      - `keyless`: Keyless verification settings.
        - `url`: The URL of the keyless authority.
        - `identities`: List of identities to use for verification.
          - `issuer`: The issuer of the identity.
          - `subjectRegExp`: The regular expression to match the subject of the
            identity.
      - `attestations`: List of attestations to use for verification.
        - `name`: The name of the attestation.
        - `predicateType`: The type of the predicate to use for verification.

The `ImageConfig` object will be a global object in the cluster, configuring
the package manager behavior without being referenced by any other object. There
could be multiple `ImageConfig` objects in the cluster, each defining settings
for different sets of images.

When a package image needs to be pulled, either as a primary package or as a
dependency, the package manager will look for `ImageConfig` objects with a pull
secret defined and use the one matching the image (see 
[selecting from multiple matches] for further details). The pull secret from the
selected `ImageConfig` object will be appended to the list of pull secrets that
might have been provided by other means, e.g., the package installation APIs.
Similarly, after the image is pulled, the package manager will query the
`ImageConfig` objects to find a matching verification settings for the image.
The selected `ImageConfig` may be different for authentication and verification
settings if there are separate objects defined for these settings. 

Careful readers might have noticed that the `spec.verification.cosign` field
closely follows the schema used in the _Policy Controller's_ `ClusterImagePolicy`
API. This is a deliberate design choice to ensure the API is flexible enough to
handle various image verification setups while also providing a consistent user
experience for those familiar with the Policy Controller. Since both _Policy
Controller_ and _Cosign_ are developed by the same organization, we believe
there's no better source of expertise for verifying Cosign-signed images. We
plan to leverage this expertise, along with existing libraries from the Policy
Controller project, to implement reliable image verification in the package
manager.

### Selecting from Multiple Matches

For a given image and configuration, there may be multiple matching
`ImageConfig` objects. We have the following options:

- **Option A:** Error out if there is more than one match.
- **Option B:** Choose the best match (e.g., "longest match" for prefix,
  "undefined" for other match types).
- **Option C:** Random selection.
- **Option D:** Advanced API with weights/precedence.
- **Option E:** Stack them together with best effort (e.g., append pull
  secrets, validate with all verifications, error out if related to registry
  TLS/proxy).

We believe the best match option is the most intuitive and offers the best
user experience by enabling users to define more specific settings
for a subset of images while maintaining a fallback/default for the rest.
However, we cannot define a clear "best match" for all possible match types
we may introduce in the future.

We propose a mixed approach between options (B) and (A). We will start with the
best match option for the `Prefix` match type. If we introduce more match types
in the future, we will error out if we cannot determine the best match
(e.g., multiple matches where at least one is a non-prefix match). Even for the
`Prefix` match type, we will error out if there is more than one best match,
i.e. multiple `ImageConfig` objects with the same prefix and same configuration.

Note that the best match will always be evaluated between `ImageConfig`
objects with the configuration of interest. For example, when the package
manager needs to pull the image, it will select the best match among those
with a pull secret. So, there cloud be two `ImageConfig` objects with the same
prefix—one with a pull secret, the other with verification, and this is fine.

### User Experience

We anticipate that the following would be important for users of this API:

- To understand which `ImageConfig` object would be selected for a given package.
- To understand whether image verification is skipped, succeeded, or failed for
  a given package image.

Considering this API primarily impacts the package revision APIs, namely
`ProviderRevision`, `ConfigurationRevision`, and `FunctionRevision`, we believe
that it is a good idea to communicate these details on those objects. The selected
`ImageConfig` object will be communicated as an event on the revision object.
For image verification, we plan to introduce a new condition on the revision
object indicating the status of the verification.

For example, a `ProviderRevision` object status could look like this:

```yaml
Status:
  Conditions:
    Last Transition Time:  2024-10-07T07:18:33Z
    Reason:                VerificationSucceeded # or VerificationFailed, VerificationSkipped
    Status:                True
    Type:                  SignatureVerified
    Last Transition Time:  2024-10-07T07:18:33Z
    Reason:                HealthyPackageRevision
    Status:                True
    Type:                  Healthy    
Events:
  Type     Reason                 Age                    From                                              Message
  ----     ------                 ----                   ----                                              -------
  Normal   SelectedImageConfig    2m19s                  packages/providerrevision.pkg.crossplane.io       Selected ImageConfig "acme-packages" for registry authentication
  Normal   SelectedImageConfig    2m40s                  packages/providerrevision.pkg.crossplane.io       Selected ImageConfig "acme-packages" for signature verification
```

### Implementation

For authentication, we will extend the [`xpkg.K8sFetcher`] implementation to
query and inject the matching pull secret from the `ImageConfig` objects into
the [`k8schain.New`] function. By doing so, we will ensure that the package
manager will use the configured pull secret when fetching the image, getting the
package descriptor, and querying the available tags of the image for dependency
resolution. Just like any other pull secrets, the configured pull secret will
be provided to the package runtime deployment so that the runtime can pull the
image as well. Other registry settings like TLS configuration will most likely
be handled in the `K8sFetcher` as well.

For image verification, we will introduce a new controller that watches the
package revision objects and triggers the verification process when a new
revision is created. The controller will query the `ImageConfig` objects to find
the best matching verification settings for the image and verify the image
signature accordingly. If there is no matching `ImageConfig` object for the
image, the verification will be skipped. The verification status will be
communicated back to the package revision object as a condition. The existing
package revision controller responsible for fetching the package images will be
changed to wait for the verification to complete before proceeding with the
installation. We need to be careful about finding the right balance between
relying on the previous verification results and re-verifying the image when
needed. This is left as a detail to be worked out during the implementation.

## Alternatives Considered

### Flowing pull secrets from parent to dependencies

This feels like the most intuitive solution, but there are caveats. We would be
passing credentials to public dependencies as well. Or, if a package is a
dependency for multiple parents, would it get secrets from all etc. It is
typical that the parent package is hosted in a different repository than the
dependencies, e.g. `xpkg.upbound.io/acmecorp/config-foo` depending on
`xpkg.upbound.io/upbound/provider-aws-etc`. It is not convenient to pass the
credentials per package compared to having a single place to configure them.

### Extending the Package installation APIs for dependencies

This may work for simple scenarios where there are minimal dependencies and
known at the time of package installation. However, it is not scalable for
packages with many dependencies and/or dependencies having their own
dependencies. Similar to the previous alternative, it is not convenient to pass
the credentials or signature verification configuration per package compared to
having a single place to configure them.

### Using Kyverno for package image verification

As mentioned in the background section, Kyverno can be used to enforce policies
for image verification for custom resources as well. One can define the
following Kyverno policy to verify Crossplane Provider images with Kyverno.
However, Kyverno may not be available in all Crossplane clusters, and
introducing it as a dependency to Crossplane just for image verification feels
like overkill.

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: signed-acme-providers
spec:
  validationFailureAction: Enforce
  rules:
  - name: check-signature
    match:
      any:
      - resources:
          kinds:
          - Provider
    imageExtractors:
      Provider:
        - name: "providers"
          path: /spec/package
    verifyImages:
    - imageReferences:
      - "xpkg.upbound.io/acme-co/*"
      attestors:
      - entries:
        - keyless:
            subject: "https://github.com/acme-co/crossplane-packages/.github/workflows/supplychain.yml@refs/heads/main"
            issuer: "https://token.actions.githubusercontent.com"
            rekor:
              url: https://rekor.sigstore.dev
```

### A singleton API with multiple rules

Instead of having multiple `ImageConfig` objects, we could have a single one
with multiple rules and credentials. This would be simpler to process by the
package manager and provide ordering guarantees. However, it would be harder to
manage the same single object by multiple users trying to configure different
credentials for different repositories, especially in multi-tenant environments
following GitOps practices. Also, as we extend the API to support more settings,
a single object may become even more complex to manage.

```yaml
apiVersion: pkg.crossplane.io/v1alpha1
kind: ImageConfig
metadata:
  name: default  # or whatever "singleton" name
spec:
  rules:
    - matchImages:
        - prefix: registry1.com/acme-co/configuration-foo
        - prefix: registry1.com/acme-co/configuration-bar
        - prefix: registry1.com/acme-co/function-baz
      verification:
        provider: cosign
        cosign:
          authorities:
            - name: verify acme builds
              keyless:
                url: https://fulcio.sigstore.dev
                identities:
                  - issuer: https://token.actions.githubusercontent.com
                    subjectRegExp: https://github.com/acme-co/crossplane-packages/*
    - matchImages:
        - prefix: registry2.com/org-foo/
      verification:
        provider: cosign
        cosign:
          authorities:
            - name: verify org-foo builds
              keyless:
                url: https://fulcio.sigstore.dev
                identities:
                  - issuer: https://token.actions.githubusercontent.com
                    subjectRegExp: https://github.com/org-foo/crossplane-packages/*
```

[existing workaround]: https://github.com/crossplane/docs/issues/789
[Policy Controller]: https://docs.sigstore.dev/policy-controller/overview/
[Kyverno]: https://kyverno.io/
[Sigstore]: https://sigstore.dev/
[cosign]: https://github.com/sigstore/cosign
[rules to verify images]: https://release-1-9-0.kyverno.io/docs/writing-policies/verify-images/#verifying-image-signatures
[Flux]: https://fluxcd.io/
[minimalistic API to verify images]: https://fluxcd.io/flux/components/source/ocirepositories/#verification
[using Kyverno for package image verification]: #using-kyverno-for-package-image-verification
[`kubernetes.io/dockerconfigjson`]: https://kubernetes.io/docs/concepts/configuration/secret/#docker-config-secrets
[`xpkg.K8sFetcher`]: https://github.com/crossplane/crossplane/blob/ed4e659c5c217fb69958eeb75ce8daa65b63823c/internal/xpkg/fetch.go#L54C6-L54C16
[`k8schain.New`]: https://github.com/crossplane/crossplane/blob/ed4e659c5c217fb69958eeb75ce8daa65b63823c/internal/xpkg/fetch.go#L131C15-L131C28
[a previous proposal]: https://github.com/crossplane/crossplane/pull/3297
[implementation PR]: https://github.com/crossplane/crossplane/pull/3552
[selecting from multiple matches]: #selecting-from-multiple-matches


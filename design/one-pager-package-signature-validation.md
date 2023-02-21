# Package Signature Validation

* Owners: Jesse Sanford (@jessesanford), Dan Mangum (@hasheddan)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Background

Crossplane packages (xpkgs) are the unit of extension in any Crossplane
installation. Currently two package types are supported: Providers and
Configurations. Providers may install Custom Resource Definitions (CRDs),
Mutating Webhook Configurations, and Validating Webhook Configurations.
They also may provide a (potentially self-referential) controller image
reference, which Crossplane will install as a Deployment to reconcile the CRDs
included in the package, which are commonly referred to as Managed Resources
(MRs). Configurations may install Composite Resource Definitions (XRDs) and
Compositions, which allow users to define abstractions that may compose any
number of MRs or other Composite Resources (XRs).

Crossplane packages are defined as OCI images, which allows Crossplane users to
take advantage of existing tooling and distribution channels, as well as the
myriad of benefits offered by a content-addressable API. Standardizing on this
ecosystem also allows Crossplane to easily integrate with other projects that
are built on top of the OCI specification.

An important distinction between running an image on Kubernetes and installing a
package via Crossplane is that the former invokes the container runtime on a
Node in the cluster, while the latter fetches the image directly from the
Crossplane process. Additionally, Crossplane maintains a separate cache of
package content, allowing it to minimize the number of operations required when
reconciling a package. Because package content is fetched directly, the
Crossplane package manager is a convenient location to inject additional logic
and safeguards. One such feature is image signature verification.

## Goals

- Enable provenance validation of crossplane packages
- Allow the specification of a trusted public key to use for validation
- Provide feedback through the status of the package validation through the\
PackageRevision that triggered the fetch

## Non-Goals

- How to sign the packages
- Where to store the signatures
- How to validate policies beyond simple provenance

## Proposal
Let's start by implementing the minimal viable provenance checking Sigstore
cosign offers. By making use of a pre-shared public key, we can validate the
signatures of crossplane packages that were signed by cosign using the paired
private key.

### Assumptions:
- The signatures will have to be computed after the crossplane .xpkg is created
by using the cosign cli. The signatures can be stored alongside the crossplane
packages in an OCI compliant registry.

- We should perform the validation before the package revision reports a healthy
status so that it cannot be activated. 

### Implementation

#### Point of Validation
If we implement validation into the fetcher to evaluate package signature we can
fail as early as possible in the lifecycle of a package revision.
```go
// Fetch fetches a package image.
func (i *K8sFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	auth, err := k8schain.New(ctx, i.client, k8schain.Options{
		Namespace:        i.namespace,
		ImagePullSecrets: secrets,
	})
	if err != nil {
		return nil, err
	}
	return remote.Image(ref, remote.WithAuthFromKeychain(auth), remote.WithTransport(i.transport), remote.WithContext(ctx))
}
```
[Link]: https://github.com/crossplane/crossplane/blob/b01b17353198a8de28664cf1eec601aaaf2fd95a/internal/xpkg/fetch.go#L119

#### Prior Art For Validation:
Cosign's operating mode using asymetric encryption with x509 certs can be patterened off of the
Sigstore policy-controller admission controller implementation. Validating against a single public key is
the equivalent of the “traditional” mode in the sigstore policy-controller. You can see it as the fallback
here: 
```go
if passedPolicyChecks {
	logging.FromContext(ctx).Debugf("Found at least one matching policy and it was validated for %s", ref.Name())
	continue
}
logging.FromContext(ctx).Errorf("ref: for %v", ref)
logging.FromContext(ctx).Errorf("container Keys: for %v", containerKeys)
if _, err := valid(ctx, ref, nil, containerKeys, ociremote.WithRemoteOptions(remote.WithAuthFromKeychain(kc))); err != nil {
	errorField := apis.ErrGeneric(err.Error(), "image").ViaFieldIndex(field, i)
	errorField.Details = c.Image
	errs = errs.Also(errorField)
	continue
}
```
[Link]: https://github.com/sigstore/policy-controller/blob/9ed1f43631a04c17c3f6e65982ba7c9dca3fff99/pkg/webhook/validator.go#L286-L298

In that implementation, the public key to be used comes from:
```go
	kc, err := k8schain.New(ctx, v.client, opt)
	if err != nil {
		logging.FromContext(ctx).Warnf("Unable to build k8schain: %v", err)
		return apis.ErrGeneric(err.Error(), apis.CurrentField)
	}

	s, err := v.lister.Secrets(system.Namespace()).Get(v.secretName)
	if err != nil && !apierrs.IsNotFound(err) {
		return apis.ErrGeneric(err.Error(), apis.CurrentField)
	}
	// If the secret is not found, we verify against the fulcio root.
	keys := make([]crypto.PublicKey, 0)
	if err == nil {
		var kerr *apis.FieldError
		keys, kerr = getKeys(ctx, s.Data)
		if kerr != nil {
			return kerr
		}
	}
```
[Link]: https://github.com/sigstore/policy-controller/blob/9ed1f43631a04c17c3f6e65982ba7c9dca3fff99/pkg/webhook/validator.go#L167-L185

You can see that using the public key for the fulcio root is the fallback:

```go
// If the secret is not found, we verify against the fulcio root.
keys := make([]crypto.PublicKey, 0)
if err == nil {
	var kerr *apis.FieldError
	keys, kerr = getKeys(ctx, s.Data)
	if kerr != nil {
		return kerr
	}
}
```
[Link]: https://github.com/sigstore/policy-controller/blob/9ed1f43631a04c17c3f6e65982ba7c9dca3fff99/pkg/webhook/validator.go#L181

However we are explicitly going to perform the validation if the secret is
found. If the secret is not found then we assume validation is disabled.
Perhaps we should warn users if a image is pulled that has a signature, but
validation is not "enabled".

The secret to store the pub key is stored as a k8s secret and passed in from:
```go
var secretName = flag.String("secret-name", "", "Flag -secret-name has been deprecated and will be removed in the future. The name of the secret in the webhook's namespace that holds the public key for verification.")
```
[Link]: https://github.com/sigstore/policy-controller/blob/9ed1f43631a04c17c3f6e65982ba7c9dca3fff99/cmd/webhook/main.go#L51

Here is where the key is actually used for verification of the signature:
```go
// We return nil if ANY key matches
var lastErr error
for _, k := range keys {
	verifier, err := signature.LoadVerifier(k, crypto.SHA256)
	if err != nil {
		logging.FromContext(ctx).Errorf("error creating verifier: %v", err)
		lastErr = err
		continue
	}
	sps, err := validSignatures(ctx, ref, verifier, rekorClient, opts...)
	if err != nil {
		logging.FromContext(ctx).Errorf("error validating signatures: %v", err)
		lastErr = err
		continue
	}
	return sps, nil
}
```
[Link]: https://github.com/sigstore/policy-controller/blob/9ed1f43631a04c17c3f6e65982ba7c9dca3fff99/pkg/webhook/validation.go#L47-L64

While the use of a single pub key is deprecated in policy-controller,
implementations exist elsewhere in the sigstore cosign codebase:

- https://github.com/sigstore/cosign/tree/1e055235dc5bdab5f9d446ad5fd31dbc72225312/copasetic#cosignverifyref-pubkey-cosignsignedpayload

- https://github.com/sigstore/cosign/blob/1e055235dc5bdab5f9d446ad5fd31dbc72225312/copasetic/main.go#L190-L201

- https://github.com/sigstore/cosign/blob/8ffcd1228c463e1ad26ccce68ae16deeca2960b4/pkg/cosign/kubernetes/webhook/validation.go#L60-L85

Also in tekton-chains:
- https://github.com/tektoncd/chains/blob/main/docs/tutorials/signed-provenance-tutorial.md

- https://github.com/tektoncd/chains/blob/main/docs/signing.md

#### Wiring Up Package Fetcher:
As it turns out there is a great example PR for how we can pass the public key
into the fetcher. The PR for adding an alternate CA Bundle is very similar:
https://github.com/crossplane/crossplane/pull/2525/files

Adding a new optional fetcher parameter using the builder pattern for the
fetcherOptions struct seems pretty straightforward:
```go
// FetcherOpt can be used to add optional parameters to NewK8sFetcher
type FetcherOpt func(k *K8sFetcher) error
```
[Link]: https://github.com/crossplane/crossplane/blob/b01b17353198a8de28664cf1eec601aaaf2fd95a/internal/xpkg/fetch.go#L63

```go
// WithCustomCA is a FetcherOpt that can be used to add a custom CA bundle to a K8sFetcher
func WithCustomCA(rootCAs *x509.CertPool) FetcherOpt {
	return func(k *K8sFetcher) error {
		t, ok := k.transport.(*http.Transport)
		if !ok {
			return errors.New("Fetcher transport is not an HTTP transport")
		}

		t.TLSClientConfig = &tls.Config{RootCAs: rootCAs, MinVersion: tls.VersionTLS12}
		return nil
	}
}
```
[Link]: https://github.com/crossplane/crossplane/blob/b01b17353198a8de28664cf1eec601aaaf2fd95a/internal/xpkg/fetch.go#L89-L99

These can be wired up using the plumbing for options passing to controller @negz
introduced in the following commit, which feeds it from Kong startCommand:
```go
if c.CABundlePath != "" {
	rootCAs, err := xpkg.ParseCertificatesFromPath(c.CABundlePath)
	if err != nil {
		return errors.Wrap(err, "Cannot parse CA bundle")
	}
	po.FetcherOptions = []xpkg.FetcherOpt{xpkg.WithCustomCA(rootCAs)}
}
```
[Link]: https://github.com/crossplane/crossplane/blob/fc4619685d1564fafee70a8456fcc124133570e0/cmd/crossplane/core/core.go#L111-L11

We should probably add a test case. Seems like there is a fetch failure test
here:
```go
"ErrBadFetch": {
	reason: "Should return an error if we fail to fetch package image.",
	args: args{
		f: &fake.MockFetcher{
			MockHead: fake.NewMockHeadFn(nil, errBoom),
		},
		pkg: &v1.Provider{
			Spec: v1.ProviderSpec{
				PackageSpec: v1.PackageSpec{
					Package: "test/test:test",
				},
			},
		},
	},
	want: want{
		err: errors.Wrap(errBoom, errFetchPackage),
	},
},
```
[Link]: https://github.com/crossplane/crossplane/blob/master/internal/controller/pkg/manager/revisioner_test.go#L113-L131

## Alternatives Considered

## Future Work
We should consider supporting the full suite of validation mechanisms for
sigstore cosign signatures. You can see the implementation from the
policy-controller here:
```go
func valid(ctx context.Context, ref name.Reference, rekorClient *client.Rekor, keys []crypto.PublicKey, opts ...ociremote.Option) ([]oci.Signature, error) {
	if len(keys) == 0 {
		// If there are no keys, then verify against the fulcio root.
		fulcioRoots, err := fulcioroots.Get()
		if err != nil {
			return nil, err
		}
		return validSignaturesWithFulcio(ctx, ref, fulcioRoots, nil /* rekor */, nil /* no identities */, opts...)
	}
	// We return nil if ANY key matches
	var lastErr error
	for _, k := range keys {
		verifier, err := signature.LoadVerifier(k, crypto.SHA256)
		if err != nil {
			logging.FromContext(ctx).Errorf("error creating verifier: %v", err)
			lastErr = err
			continue
		}

		sps, err := validSignatures(ctx, ref, verifier, rekorClient, opts...)
		if err != nil {
			logging.FromContext(ctx).Errorf("error validating signatures: %v", err)
			lastErr = err
			continue
		}
		return sps, nil
	}
	logging.FromContext(ctx).Debug("No valid signatures were found.")
	return nil, lastErr
}
```
[Link]: https://github.com/sigstore/policy-controller/blob/9ed1f43631a04c17c3f6e65982ba7c9dca3fff99/pkg/webhook/validation.go#L38-L67

SLSA Verifier has a full implementation using cosign as a library here.
```go
package container

import (
	"context"

	crname "github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/oci"
)

var RunCosignImageVerification = func(ctx context.Context,
	image string, co *cosign.CheckOpts) ([]oci.Signature, bool, error) {
	signedImgRef, err := crname.ParseReference(image)
	if err != nil {
		return nil, false, err
	}
	return cosign.VerifyImageAttestations(ctx, signedImgRef, co)
}
```
[Link]: https://github.com/slsa-framework/slsa-verifier/blob/26155fe9a35dc2f3a9ef3740272e92b91fa14d75/verifiers/container/cosign.go

However that might be very heavy to include all of cosign.
We should probably wait to make use of any work done by sigstore to implement a
lib just for validation: https://github.com/sigstore/cosign/issues/532

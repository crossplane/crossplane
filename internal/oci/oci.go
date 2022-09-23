/*
Copyright 2022 The Flux authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package oci provides utility functionality to deal with OCI registry
// objects and their sigstore cosign based signatures
package oci

import (
	"context"
	"crypto"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/cmd/cosign/cli/rekor"
	ociremote "github.com/sigstore/cosign/pkg/oci/remote"

	"github.com/google/go-containerregistry/pkg/name"
	coptions "github.com/sigstore/cosign/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/oci"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

// options is a struct that holds options for verifier.
type options struct {
	PublicKey []byte
	Keychain  authn.Keychain
}

// Options is a function that configures the options applied to a Verifier.
type Options func(opts *options)

// WithPublicKey sets the public key.
func WithPublicKey(publicKey []byte) Options {
	return func(opts *options) {
		opts.PublicKey = publicKey
	}
}

// WithAuthnKeychain sets the keychain to use by the verifier.
func WithAuthnKeychain(keychain authn.Keychain) Options {
	return func(opts *options) {
		opts.Keychain = keychain
	}
}

// Verifier is a struct which is responsible for executing verification logic.
type Verifier struct {
	opts *cosign.CheckOpts
}

// NewVerifier initializes a new Verifier.
func NewVerifier(opts ...Options) (*Verifier, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	checkOpts := &cosign.CheckOpts{}

	ro := coptions.RegistryOptions{}
	co, err := ro.ClientOpts(context.Background())
	if err != nil {
		return nil, err
	}

	if o.Keychain != nil {
		co = append(co, ociremote.WithRemoteOptions(remote.WithAuthFromKeychain(o.Keychain)))
	}

	checkOpts.RegistryClientOpts = co

	// If a public key is provided, it will use it to verify the signature.
	// If there is no public key provided, it will try keyless verification.
	// https://github.com/sigstore/cosign/blob/main/KEYLESS.md.
	if len(o.PublicKey) > 0 {
		pubKeyRaw, err := cryptoutils.UnmarshalPEMToPublicKey(o.PublicKey)
		if err != nil {
			return nil, err
		}

		checkOpts.SigVerifier, err = signature.LoadVerifier(pubKeyRaw, crypto.SHA256)
		if err != nil {
			return nil, err
		}
	} else {
		rcerts, err := fulcio.GetRoots()
		if err != nil {
			return nil, fmt.Errorf("unable to get Fulcio root certs: %w", err)
		}
		checkOpts.RootCerts = rcerts

		icerts, err := fulcio.GetIntermediates()
		if err != nil {
			return nil, fmt.Errorf("unable to get Fulcio intermediate certs: %w", err)
		}
		checkOpts.IntermediateCerts = icerts

		rc, err := rekor.NewClient(coptions.DefaultRekorURL)
		if err != nil {
			return nil, fmt.Errorf("unable to create Rekor client: %w", err)
		}
		checkOpts.RekorClient = rc
	}

	return &Verifier{
		opts: checkOpts,
	}, nil
}

// VerifyImageSignatures verify the authenticity of the given ref OCI image.
func (v *Verifier) VerifyImageSignatures(ctx context.Context, ref name.Reference) ([]oci.Signature, bool, error) {
	return cosign.VerifyImageSignatures(ctx, ref, v.opts)
}

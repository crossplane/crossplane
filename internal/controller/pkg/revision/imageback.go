/*
Copyright 2020 The Crossiane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in comiiance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by apiicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or imiied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package revision

import (
	"archive/tar"
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/validate"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errBadReference            = "package tag is not a valid reference"
	errResolveDigest           = "failed to resolve package reference to a digest"
	errGetVerificationConfig   = "failed to get image verification config"
	errVerifyPackage           = "package signature verification failed"
	errFetchPackage            = "failed to fetch package from remote"
	errGetManifest             = "failed to get package image manifest from remote"
	errFetchLayer              = "failed to fetch annotated base layer from remote"
	errGetUncompressed         = "failed to get uncompressed contents from layer"
	errMultipleAnnotatedLayers = "package is invalid due to multiple annotated base layers"
	errFmtNoPackageFileFound   = "couldn't find \"" + xpkg.StreamFile + "\" file after checking %d files in the archive (annotated layer: %v)"
	errFmtMaxManifestLayers    = "package has %d layers, but only %d are allowed"
	errValidateLayer           = "invalid package layer"
	errValidateImage           = "invalid package image"
)

const (
	layerAnnotation     = "io.crossplane.xpkg"
	baseAnnotationValue = "base"
	// maxLayers is the maximum number of layers an image can have.
	maxLayers = 256
)

// A Validator validates the signature of a package image.
type Validator interface {
	Validate(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, pullSecrets ...string) error
}

// ImageBackend is a backend for parser.
type ImageBackend struct {
	registry  string
	fetcher   xpkg.Fetcher
	config    xpkg.ConfigStore
	validator Validator
}

// An ImageBackendOption sets configuration for an image backend.
type ImageBackendOption func(i *ImageBackend)

// WithDefaultRegistry sets the default registry that an image backend will use.
func WithDefaultRegistry(registry string) ImageBackendOption {
	return func(i *ImageBackend) {
		i.registry = registry
	}
}

// WithVerification makes the backend verify the signature of the image it is
// about to install. The signature controller reports verification status on the
// package revision, but it resolves the package reference independently of this
// backend. Verifying here as well means the bytes we check are the bytes we
// install, whatever the tag resolves to in between.
func WithVerification(c xpkg.ConfigStore, v Validator) ImageBackendOption {
	return func(i *ImageBackend) {
		i.config = c
		i.validator = v
	}
}

// NewImageBackend creates a new image backend.
func NewImageBackend(fetcher xpkg.Fetcher, opts ...ImageBackendOption) *ImageBackend {
	i := &ImageBackend{
		fetcher: fetcher,
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// resolveAndVerify resolves ref to a digest and, if signature verification is
// enabled, verifies the signature of the image that digest points to. Resolving
// once and returning the digest is what makes the bytes we verify the bytes the
// caller installs: verifying a tag and then pulling it again would let a
// registry serve different content to each call.
//
// The signature controller reports verification status on the package revision,
// but it resolves the reference independently of this backend, so it cannot
// give us that guarantee on its own.
func (i *ImageBackend) resolveAndVerify(ctx context.Context, source string, ref name.Reference, secrets ...string) (name.Digest, error) {
	digest, ok := ref.(name.Digest)
	if !ok {
		desc, err := i.fetcher.Head(ctx, ref, secrets...)
		if err != nil {
			return name.Digest{}, errors.Wrap(err, errResolveDigest)
		}

		digest = ref.Context().Digest(desc.Digest.String())
	}

	if i.validator == nil {
		return digest, nil
	}

	// Match the image config on the source rather than on the digest reference,
	// so prefixes keep matching the image path the way they do everywhere else,
	// including prefixes that carry a tag.
	_, vc, err := i.config.ImageVerificationConfigFor(ctx, source)
	if err != nil {
		return name.Digest{}, errors.Wrap(err, errGetVerificationConfig)
	}

	if vc == nil || vc.Cosign == nil {
		return digest, nil
	}

	if err := i.validator.Validate(ctx, digest, vc, secrets...); err != nil {
		return name.Digest{}, errors.Wrap(err, errVerifyPackage)
	}

	return digest, nil
}

// Init initializes an ImageBackend.
func (i *ImageBackend) Init(ctx context.Context, bo ...parser.BackendOption) (io.ReadCloser, error) {
	// NOTE(hasheddan): we use nestedBackend here because simultaneous
	// reconciles of providers or configurations can lead to the package
	// revision being overwritten mid-execution in the shared image backend when
	// it is a member of the image backend struct. We could introduce a lock
	// here, but there is no reason why a given reconcile should require
	// exclusive access to the image backend other than its poor design. We
	// should consider restructuring the parser backend interface to better
	// accommodate for shared, thread-safe backends.
	n := &nestedBackend{}
	for _, o := range bo {
		o(n)
	}
	// Use the package recorded in the status rather than the one from the spec,
	// since it may have been rewritten by an image config.
	source := n.pr.GetResolvedSource()
	ref, err := name.ParseReference(source, name.WithDefaultRegistry(i.registry))
	if err != nil {
		return nil, errors.Wrap(err, errBadReference)
	}
	// Fetch image from registry.
	ps := v1.RefNames(n.pr.GetPackagePullSecrets())
	if n.pullSecretFromConfig != "" {
		ps = append(ps, n.pullSecretFromConfig)
	}
	// Pin all subsequent registry interactions to the digest we verified.
	// Pulling by the original tag would let a registry that serves different
	// content between verification and the pull slip an unverified image into
	// the package.
	digest, err := i.resolveAndVerify(ctx, source, ref, ps...)
	if err != nil {
		return nil, err
	}

	img, err := i.fetcher.Fetch(ctx, digest, ps...)
	if err != nil {
		return nil, errors.Wrap(err, errFetchPackage)
	}
	// Get image manifest.
	manifest, err := img.Manifest()
	if err != nil {
		return nil, errors.Wrap(err, errGetManifest)
	}

	// Check that the image has less than the maximum allowed number of layers.
	if nLayers := len(manifest.Layers); nLayers > maxLayers {
		return nil, errors.Errorf(errFmtMaxManifestLayers, nLayers, maxLayers)
	}

	// Determine if the image is using annotated layers.
	var tarc io.ReadCloser
	foundAnnotated := false
	for _, l := range manifest.Layers {
		if a, ok := l.Annotations[layerAnnotation]; !ok || a != baseAnnotationValue {
			continue
		}
		// NOTE(hasheddan): the xpkg specification dictates that only one layer
		// descriptor may be annotated as xpkg base. Since iterating through all
		// descriptors is relatively inexpensive, we opt to do so in order to
		// verify that we aren't just using the first layer annotated as xpkg
		// base.
		if foundAnnotated {
			return nil, errors.New(errMultipleAnnotatedLayers)
		}
		foundAnnotated = true
		layer, err := img.LayerByDigest(l.Digest)
		if err != nil {
			return nil, errors.Wrap(err, errFetchLayer)
		}
		if err := validate.Layer(layer); err != nil {
			return nil, errors.Wrap(err, errValidateLayer)
		}
		tarc, err = layer.Uncompressed()
		if err != nil {
			return nil, errors.Wrap(err, errGetUncompressed)
		}
	}

	// If we still don't have content then we need to flatten image filesystem.
	if !foundAnnotated {
		if err := validate.Image(img); err != nil {
			return nil, errors.Wrap(err, errValidateImage)
		}
		tarc = mutate.Extract(img)
	}

	// The ReadCloser is an uncompressed tarball, either consisting of annotated
	// layer contents or flattened filesystem content. Either way, we only want
	// the package YAML stream.
	t := tar.NewReader(tarc)
	var read int
	for {
		h, err := t.Next()
		if err != nil {
			return nil, errors.Wrapf(err, errFmtNoPackageFileFound, read, foundAnnotated)
		}
		if h.Name == xpkg.StreamFile {
			break
		}
		read++
	}

	// NOTE(hasheddan): we return a JoinedReadCloser such that closing will free
	// resources allocated to the underlying ReadCloser. See
	// https://github.com/google/go-containerregistry/blob/329563766ce8131011c25fd8758a25d94d9ad81b/pkg/v1/mutate/mutate.go#L222
	// for more info.
	return xpkg.JoinedReadCloser(t, tarc), nil
}

// nestedBackend is a nop parser backend that conforms to the parser backend
// interface to allow holding intermediate data passed via parser backend
// options.
// NOTE(hasheddan): see usage in ImageBackend Init() for reasoning.
type nestedBackend struct {
	pr                   v1.PackageRevision
	pullSecretFromConfig string
}

// Init is a nop because nestedBackend does not actually meant to act as a
// parser backend.
func (n *nestedBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return nil, nil
}

// PackageRevision sets the package revision for ImageBackend.
func PackageRevision(pr v1.PackageRevision) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*nestedBackend)
		if !ok {
			return
		}
		i.pr = pr
	}
}

// PullSecretFromConfig sets the image config pull secret for ImageBackend.
func PullSecretFromConfig(secret string) parser.BackendOption {
	return func(p parser.Backend) {
		i, ok := p.(*nestedBackend)
		if !ok {
			return
		}
		i.pullSecretFromConfig = secret
	}
}

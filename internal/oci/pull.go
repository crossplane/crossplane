/*
Copyright 2022 The Crossplane Authors.

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

package oci

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ociv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errPullNever      = "refusing to pull from remote with image pull policy " + string(ImagePullPolicyNever)
	errNewDigestStore = "cannot create new image digest store"
	errPullImage      = "cannot pull image from remote"
	errStoreImage     = "cannot cache image"
	errImageDigest    = "cannot get image digest"
	errStoreDigest    = "cannot cache image digest"
	errLoadImage      = "cannot load image from cache"
	errLoadHash       = "cannot load image digest"
)

// An ImagePullPolicy dictates when an image may be pulled from a remote.
type ImagePullPolicy string

// Image pull policies
const (
	// ImagePullPolicyIfNotPresent only pulls from a remote if the image is not
	// in the local cache. It is equivalent to ImagePullPolicyNever with a
	// fall-back to ImagePullPolicyAlways.
	ImagePullPolicyIfNotPresent ImagePullPolicy = "IfNotPresent"

	// ImagePullPolicyAlways always pulls at least the image manifest from the
	// remote. Layers are pulled if they are not in cache.
	ImagePullPolicyAlways ImagePullPolicy = "Always"

	// ImagePullPolicyNever never pulls anything from the remote. It resolves
	// OCI references to digests (i.e. SHAs) using a local cache of known
	// mappings.
	ImagePullPolicyNever ImagePullPolicy = "Never"
)

// ImagePullAuth configures authentication to a remote registry.
type ImagePullAuth struct {
	Username string
	Password string
	Auth     string

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string

	// RegistryToken is a bearer token to be sent to a registry.
	RegistryToken string
}

// Authorization builds a go-containerregistry compatible AuthConfig.
func (a ImagePullAuth) Authorization() (*authn.AuthConfig, error) {
	return &authn.AuthConfig{
		Username:      a.Username,
		Password:      a.Password,
		Auth:          a.Auth,
		IdentityToken: a.IdentityToken,
		RegistryToken: a.RegistryToken,
	}, nil
}

// ImageClientOptions configure an ImageClient.
type ImageClientOptions struct {
	pull ImagePullPolicy
	auth *ImagePullAuth
}

func parse(o ...ImageClientOption) ImageClientOptions {
	opt := &ImageClientOptions{
		pull: ImagePullPolicyIfNotPresent, // The default.
	}
	for _, fn := range o {
		fn(opt)
	}
	return *opt
}

// An ImageClientOption configures an ImageClient.
type ImageClientOption func(c *ImageClientOptions)

// WithPullPolicy specifies whether a client may pull from a remote.
func WithPullPolicy(p ImagePullPolicy) ImageClientOption {
	return func(c *ImageClientOptions) {
		c.pull = p
	}
}

// WithPullAuth specifies how a client should authenticate to a remote.
func WithPullAuth(a *ImagePullAuth) ImageClientOption {
	return func(c *ImageClientOptions) {
		c.auth = a
	}
}

// An ImageClient is an OCI registry client.
type ImageClient interface {
	// Image pulls an OCI image.
	Image(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error)
}

// An ImageCache caches OCI images.
type ImageCache interface {
	Image(h ociv1.Hash) (ociv1.Image, error)
	WriteImage(img ociv1.Image) error
}

// A HashCache maps OCI references to hashes.
type HashCache interface {
	Hash(r name.Reference) (ociv1.Hash, error)
	WriteHash(r name.Reference, h ociv1.Hash) error
}

// A RemoteClient fetches OCI image manifests.
type RemoteClient struct{}

// Image fetches an image manifest. The returned image lazily pulls its layers.
func (i *RemoteClient) Image(ctx context.Context, ref name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
	opts := parse(o...)
	if opts.auth != nil {
		return remote.Image(ref, remote.WithContext(ctx), remote.WithAuth(opts.auth))
	}
	if opts.pull == ImagePullPolicyNever {
		return nil, errors.New(errPullNever)
	}
	return remote.Image(ref, remote.WithContext(ctx))
}

// A CachingPuller pulls OCI images. Images are pulled either from a local cache
// or a remote depending on whether they are available locally and a supplied
// ImagePullPolicy.
type CachingPuller struct {
	remote  ImageClient
	local   ImageCache
	mapping HashCache
}

// NewCachingPuller returns an OCI image puller with a local cache.
func NewCachingPuller(h HashCache, i ImageCache, r ImageClient) *CachingPuller {
	return &CachingPuller{remote: r, local: i, mapping: h}
}

// Image pulls the supplied image and all of its layers. The supplied config
// determines where the image may be pulled from - i.e. the local store or a
// remote. Images that are pulled from a remote are cached in the local store.
func (f *CachingPuller) Image(ctx context.Context, r name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
	opts := parse(o...)

	switch opts.pull {
	case ImagePullPolicyNever:
		return f.never(r)
	case ImagePullPolicyAlways:
		return f.always(ctx, r, o...)
	case ImagePullPolicyIfNotPresent:
		fallthrough
	default:
		img, err := f.never(r)
		if err == nil {
			return img, nil
		}
		return f.always(ctx, r, o...)
	}
}
func (f *CachingPuller) never(r name.Reference) (ociv1.Image, error) {
	var h ociv1.Hash
	var err error

	// Avoid a cache lookup if the digest was specified explicitly.
	switch d := r.(type) {
	case name.Digest:
		h, err = ociv1.NewHash(d.DigestStr())
	default:
		h, err = f.mapping.Hash(r)
	}

	if err != nil {
		return nil, errors.Wrap(err, errLoadHash)
	}

	i, err := f.local.Image(h)
	return i, errors.Wrap(err, errLoadImage)
}

func (f *CachingPuller) always(ctx context.Context, r name.Reference, o ...ImageClientOption) (ociv1.Image, error) {
	// This will only pull the image's manifest and config, not layers.
	img, err := f.remote.Image(ctx, r, o...)
	if err != nil {
		return nil, errors.Wrap(err, errPullImage)
	}

	// This will fetch any layers that aren't already in the store.
	if err := f.local.WriteImage(img); err != nil {
		return nil, errors.Wrap(err, errStoreImage)
	}

	d, err := img.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errImageDigest)
	}

	// Store a mapping from this reference to its digest.
	if err := f.mapping.WriteHash(r, d); err != nil {
		return nil, errors.Wrap(err, errStoreDigest)
	}

	// Return the stored image to ensure future reads are from disk, not
	// from remote.
	img, err = f.local.Image(d)
	return img, errors.Wrap(err, errLoadImage)
}

/*
Copyright 2025 The Crossplane Authors.

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

package xpkg

import (
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errGetManifestList = "error retrieving manifest list"
	errManifestDigest  = "error getting manifest digest"

	// ManifestAnnotation is the annotation value for an xpkg extensions manifest.
	ManifestAnnotation = "xpkg-extensions"
)

// Appender defines an xpkg extensions appender.
type Appender struct {
	keychain  remote.Option
	remoteImg name.Reference
}

type appendOpts struct {
	keychain remote.Option
}

// An AppendOpt configures how the remote xpkg is mutated.
type AppendOpt func(*appendOpts)

// WithAuth sets the registry authentication to use for the operation.
func WithAuth(keychain remote.Option) AppendOpt {
	return func(a *appendOpts) {
		a.keychain = keychain
	}
}

// NewAppender returns a new Appender.
func NewAppender(keychain remote.Option, remoteImg name.Reference) *Appender {
	return &Appender{
		keychain:  keychain,
		remoteImg: remoteImg,
	}
}

// Append mutates a remote xpkg to add a manifest referencing a layer of optional package extensions.
func (a *Appender) Append(index v1.ImageIndex, extImg v1.Image, opts ...AppendOpt) (v1.ImageIndex, error) {
	config := &appendOpts{}
	for _, o := range opts {
		o(config)
	}

	// Create the extensions manifest
	extManifestDigest, err := extImg.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errManifestDigest)
	}

	// No-op if there already exists a manifest with the same digest in the index.
	manifestList, err := index.IndexManifest()
	if err != nil {
		return nil, errors.Wrap(err, errGetManifestList)
	}

	for _, manifest := range manifestList.Manifests {
		if manifest.Digest.String() == extManifestDigest.String() {
			return index, nil
		}
	}

	// Create the new index to replace
	newIndex := mutate.AppendManifests(index, mutate.IndexAddendum{
		Add: extImg,
		Descriptor: v1.Descriptor{
			MediaType: types.DockerManifestSchema2,
			Digest:    extManifestDigest,
			Size:      0,
			Annotations: map[string]string{
				AnnotationKey: ManifestAnnotation,
			},
		},
	})

	return newIndex, nil
}

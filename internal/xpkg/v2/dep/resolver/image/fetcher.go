/*
Copyright 2023 The Crossplane Authors.

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

package image

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// LocalFetcher --
type LocalFetcher struct{}

// NewLocalFetcher --
func NewLocalFetcher() *LocalFetcher {
	return &LocalFetcher{}
}

// Fetch fetches a package image.
func (r *LocalFetcher) Fetch(ctx context.Context, ref name.Reference, _ ...string) (v1.Image, error) {
	return remote.Image(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

// Head fetches a package descriptor.
func (r *LocalFetcher) Head(ctx context.Context, ref name.Reference, _ ...string) (*v1.Descriptor, error) {
	return remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

// Tags fetches a package's tags.
func (r *LocalFetcher) Tags(ctx context.Context, ref name.Reference, _ ...string) ([]string, error) {
	return remote.List(ref.Context(), remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

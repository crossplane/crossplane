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

package xpkg

import (
	"context"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// fetchFn fetches a package from a source.
type fetchFn func(context.Context, name.Reference) (v1.Image, error)

// registryFetch fetches a package from the registry.
func registryFetch(ctx context.Context, r name.Reference) (v1.Image, error) {
	return remote.Image(r, remote.WithContext(ctx))
}

// daemonFetch fetches a package from the Docker daemon.
func daemonFetch(ctx context.Context, r name.Reference) (v1.Image, error) {
	return daemon.Image(r, daemon.WithContext(ctx))
}

func xpkgFetch(path string) fetchFn {
	return func(ctx context.Context, r name.Reference) (v1.Image, error) {
		return tarball.ImageFromPath(filepath.Clean(path), nil)
	}
}

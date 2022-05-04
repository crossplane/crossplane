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

package fn

import (
	"context"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Command runs a Composition function.
type Command struct {
	Container containerCommand `cmd:"" help:"Run a containerized Composition function."`
}

// Run is the no-op method required for kong call tree. Kong requires each node
// in the calling path to have an associated Run method.
func (c *Command) Run() error { return nil }

type containerCommand struct {
	CacheDir string        `short:"c" help:"Directory used for caching function images." default:"/cache/fn"`
	Timeout  time.Duration `help:"Maximum time for which the function may run before being killed." default:"10s"`

	Image        string   `arg:"" help:"OCI image to run."`
	ResourceList *os.File `arg:"" help:"YAML encoded ResourceList to pass to the function."`
}

// A BasicFetcher fetches OCI images. It doesn't support private registries.
type BasicFetcher struct{}

// Fetch fetches a package image.
func (i *BasicFetcher) Fetch(ctx context.Context, ref name.Reference, _ ...string) (v1.Image, error) {
	return remote.Image(ref, remote.WithContext(ctx))
}

// Head fetches a package descriptor.
func (i *BasicFetcher) Head(ctx context.Context, ref name.Reference, _ ...string) (*v1.Descriptor, error) {
	return remote.Head(ref, remote.WithContext(ctx))
}

// Tags fetches a package's tags.
func (i *BasicFetcher) Tags(ctx context.Context, ref name.Reference, _ ...string) ([]string, error) {
	return remote.List(ref.Context(), remote.WithContext(ctx))
}

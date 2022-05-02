//go:build linux

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

package xfn

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
	fnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/v1alpha1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Error strings.
const (
	errInvalidInput  = "invalid function input"
	errInvalidOutput = "invalid function output"
	errBadReference  = "OCI tag is not a valid reference"
	errHeadImg       = "cannot fetch OCI image descriptor"
	errExecFn        = "cannot execute function"
	errFetchFn       = "cannot fetch function from registry"
	errLookupFn      = "cannot lookup function in store"
	errWriteFn       = "cannot write function to store"
	errDeleteBundle  = "cannot delete OCI bundle"
)

const spark = "/usr/local/bin/spark"

// An ContainerRunner runs an XRM function packaged as an OCI image by
// extracting it and running it as a 'rootless' container.
type ContainerRunner struct {
	image string

	// TODO(negz): Break fetch-ey bits out of xpkg.
	defaultRegistry string
	registry        xpkg.Fetcher
	store           OCIStore
}

// A ContainerRunnerOption configures a new ContainerRunner.
type ContainerRunnerOption func(*ContainerRunner)

// WithOCIStore configures the OCI fetcher a container runner should use.
func WithOCIFetcher(f xpkg.Fetcher) ContainerRunnerOption {
	return func(r *ContainerRunner) {
		r.registry = f
	}
}

// WithOCIStore configures the OCI store a container runner should use.
func WithOCIStore(s OCIStore) ContainerRunnerOption {
	return func(r *ContainerRunner) {
		r.store = s
	}
}

// NewContainerRunner returns a new Runner that runs functions as rootless
// containers.
func NewContainerRunner(image string, o ...ContainerRunnerOption) *ContainerRunner {
	r := &ContainerRunner{
		image:           image,
		defaultRegistry: name.DefaultRegistry,
		registry:        xpkg.NewNopFetcher(),
		store:           &ForgetfulStore{},
	}
	for _, fn := range o {
		fn(r)
	}
	return r
}

// Run a function as a rootless OCI container. Functions that return non-zero,
// or that cannot be executed in the first place (e.g. because they cannot be
// fetched from the registry) will return an error.
func (r *ContainerRunner) Run(ctx context.Context, in *fnv1alpha1.ResourceList) (*fnv1alpha1.ResourceList, error) {
	// Parse the input early, before we potentially pull and write the image.
	y, err := yaml.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, errInvalidInput)
	}
	stdin := bytes.NewReader(y)

	ref, err := name.ParseReference(r.image, name.WithDefaultRegistry(r.defaultRegistry))
	if err != nil {
		return nil, errors.Wrap(err, errBadReference)
	}

	d, err := r.registry.Head(ctx, ref)
	if err != nil {
		return nil, errors.Wrap(err, errHeadImg)
	}

	imgID := d.Digest.Hex
	runID := uuid.NewString()

	// TODO(negz): Respect the ImagePullPolicy.
	b, err := r.store.Bundle(ctx, imgID, runID)
	if IsNotCached(err) {
		// If the function isn't found in the store we fetch it, write it, and
		// try to look it up again.
		img, ferr := r.registry.Fetch(ctx, ref)
		if ferr != nil {
			return nil, errors.Wrap(ferr, errFetchFn)
		}

		if err := r.store.Cache(ctx, d.Digest.Hex, img); err != nil {
			return nil, errors.Wrap(err, errWriteFn)
		}

		// Note we're setting the outer err that satisfied IsNotFound.
		b, err = r.store.Bundle(ctx, imgID, runID)
	}
	if err != nil {
		return nil, errors.Wrap(err, errLookupFn)
	}

	/*
		We want to create an overlayfs with the cached rootfs as the lower layer
		and the bundle's rootfs as the upper layer, if possible. Kernel 5.11 and
		later supports using overlayfs inside a user (and mount) namespace. The
		best (only?) way to reliably run code in a user namespace in Go is to
		execute a separate binary; the unix.Unshare syscall affects only one OS
		thread, and the Go scheduler might move the goroutine to another.

		Therefore we execute a small shim - spark - in a new user and mount
		namespace. spark sets up the overlayfs if the Kernel supports it.
		Otherwise it falls back to making a copy of the cached rootfs. spark
		then executes an OCI runtime which creates another layer of namespaces
		in order to actually execute the function.

		We don't need to cleanup the mounts spark creates. They will be removed
		automatically along with their mount namespace when spark exits.
	*/
	cmd := exec.CommandContext(ctx, spark, b.CachedRootFS, b.Path) //nolint:gosec // We're intentionally executing with variable input.
	cmd.Stdin = stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags = syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS}

	stdout, err := cmd.Output()
	if err != nil {
		// TODO(negz): Don't swallow stderr if this is an *exec.ExitError?
		return nil, errors.Wrap(err, errExecFn)
	}

	if err := b.Delete(); err != nil {
		return nil, errors.Wrap(err, errDeleteBundle)
	}

	out := &v1alpha1.ResourceList{}
	return out, errors.Wrap(yaml.Unmarshal(stdout, out), errInvalidOutput)
}

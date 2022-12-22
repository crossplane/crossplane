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

// Package spark runs a Composition Function. It is designed to be run as root
// inside an unprivileged user namespace.
package spark

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	"github.com/crossplane/crossplane/internal/oci"
	"github.com/crossplane/crossplane/internal/oci/store"
	"github.com/crossplane/crossplane/internal/oci/store/overlay"
	"github.com/crossplane/crossplane/internal/oci/store/uncompressed"
)

// Error strings.
const (
	errReadRequest      = "cannot read request from stdin"
	errUnmarshalRequest = "cannot unmarshal request data from stdin"
	errMarshalResponse  = "cannot marshal response data to stdout"
	errWriteResponse    = "cannot write response data to stdout"
	errBadReference     = "OCI tag is not a valid reference"
	errMkRuntimeRootdir = "cannot make OCI runtime root directory"
	errRuntime          = "OCI runtime error"
	errFetchFn          = "cannot fetch OCI image from registry"
	errBundleFn         = "cannot create OCI bundle"
	errCleanupBundle    = "cannot cleanup OCI bundle"
)

// The path within the cache dir that the OCI runtime should use for its
// '--root' cache.
const ociRuntimeRoot = "runtime"

// The time after which the OCI runtime will be killed if none is specified in
// the RunFunctionRequest.
const defaultTimeout = 25 * time.Second

// Command runs a containerized Composition Function.
type Command struct {
	CacheDir string `short:"c" help:"Directory used for caching function images and containers." default:"/xfn"`
	Runtime  string `help:"OCI runtime binary to invoke." default:"crun"`
}

// Run a Composition Function inside an unprivileged user namespace. Reads a
// protocol buffer serialized RunFunctionRequest from stdin, and writes a
// protocol buffer serialized RunFunctionResponse to stdout.
func (c *Command) Run() error { //nolint:gocyclo // TODO(negz): Refactor some of this out into functions.
	pb, err := io.ReadAll(os.Stdin)
	if err != nil {
		return errors.Wrap(err, errReadRequest)
	}

	req := &v1alpha1.RunFunctionRequest{}
	if err := proto.Unmarshal(pb, req); err != nil {
		return errors.Wrap(err, errUnmarshalRequest)
	}

	t := req.GetRunFunctionConfig().GetTimeout().AsDuration()
	if t == 0 {
		t = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	ref, err := name.ParseReference(req.GetImage())
	if err != nil {
		return errors.Wrap(err, errBadReference)
	}

	r := &oci.BasicFetcher{}
	runID := uuid.NewString()

	// We prefer to use an overlayfs based store where possible. Both stores use
	// approximately the same amount of disk to store images, but the overlay
	// store uses less space (and disk I/O) when creating a filesystem for a
	// container. This is because the overlay store can create a container
	// filesystem by creating an overlay atop its image layers, while the
	// uncompressed store must extract said layers to create a new filesystem.
	var s store.Bundler
	s, err = uncompressed.NewCachingBundler(c.CacheDir)
	if overlay.Supported(c.CacheDir) {
		s, err = overlay.NewCachingBundler(c.CacheDir)
	}
	if err != nil {
		return errors.Wrap(err, "cannot setup overlay store")
	}

	// TODO(negz): Is it worth using r.Head to determine whether the image is
	// already in the store? This would be cheaper than using r.Fetch, but
	// r.Fetch is only grabbing the manifest (not the layers) and is thus
	// presumably not super expensive.
	// TODO(negz): Respect the ImagePullPolicy.
	img, ferr := r.Fetch(ctx, ref)
	if ferr != nil {
		return errors.Wrap(ferr, errFetchFn)
	}
	b, err := s.Bundle(ctx, img, runID)
	if err != nil {
		return errors.Wrap(err, errBundleFn)
	}

	root := filepath.Join(c.CacheDir, ociRuntimeRoot)
	if err := os.MkdirAll(root, 0700); err != nil {
		_ = b.Cleanup()
		return errors.Wrap(err, errMkRuntimeRootdir)
	}

	// TODO(negz): Consider using the OCI runtime's lifecycle management commands
	// (i.e create, start, and delete) rather than run. This would allow spark
	// to return without sitting in-between xfn and crun.

	//nolint:gosec // Executing with user-supplied input is intentional.
	cmd := exec.CommandContext(ctx, c.Runtime, "--root="+root, "run", "--bundle="+b.Path(), runID)
	cmd.Stdin = bytes.NewReader(req.GetInput())

	out, err := cmd.Output()
	if err != nil {
		_ = b.Cleanup()
		return errors.Wrap(err, errRuntime)
	}
	if err := b.Cleanup(); err != nil {
		return errors.Wrap(err, errCleanupBundle)
	}

	rsp := &v1alpha1.RunFunctionResponse{Output: out}
	pb, err = proto.Marshal(rsp)
	if err != nil {
		return errors.Wrap(err, errMarshalResponse)
	}
	_, err = os.Stdout.Write(pb)
	return errors.Wrap(err, errWriteResponse)
}

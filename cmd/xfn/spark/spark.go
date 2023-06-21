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
	runtime "github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/protobuf/proto"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	"github.com/crossplane/crossplane/internal/oci"
	"github.com/crossplane/crossplane/internal/oci/spec"
	"github.com/crossplane/crossplane/internal/oci/store"
	"github.com/crossplane/crossplane/internal/oci/store/overlay"
	"github.com/crossplane/crossplane/internal/oci/store/uncompressed"
)

// Error strings.
const (
	errReadRequest      = "cannot read request from stdin"
	errUnmarshalRequest = "cannot unmarshal request data from stdin"
	errNewBundleStore   = "cannot create OCI runtime bundle store"
	errNewDigestStore   = "cannot create OCI image digest store"
	errParseRef         = "cannot parse OCI image reference"
	errPull             = "cannot pull OCI image"
	errBundleFn         = "cannot create OCI runtime bundle"
	errMkRuntimeRootdir = "cannot make OCI runtime cache"
	errRuntime          = "OCI runtime error"
	errCleanupBundle    = "cannot cleanup OCI runtime bundle"
	errMarshalResponse  = "cannot marshal response data to stdout"
	errWriteResponse    = "cannot write response data to stdout"
	errCPULimit         = "cannot limit container CPU"
	errMemoryLimit      = "cannot limit container memory"
	errHostNetwork      = "cannot configure container to run in host network namespace"
)

// The path within the cache dir that the OCI runtime should use for its
// '--root' cache.
const ociRuntimeRoot = "runtime"

// The time after which the OCI runtime will be killed if none is specified in
// the RunFunctionRequest.
const defaultTimeout = 25 * time.Second

// Command runs a containerized Composition Function.
type Command struct {
	CacheDir      string `short:"c" help:"Directory used for caching function images and containers." default:"/xfn"`
	Runtime       string `help:"OCI runtime binary to invoke." default:"crun"`
	MaxStdioBytes int64  `help:"Maximum size of stdout and stderr for functions." default:"0"`
}

// Run a Composition Function inside an unprivileged user namespace. Reads a
// protocol buffer serialized RunFunctionRequest from stdin, and writes a
// protocol buffer serialized RunFunctionResponse to stdout.
func (c *Command) Run() error { //nolint:gocyclo // TODO(negz): Refactor some of this out into functions, add tests.
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

	runID := uuid.NewString()

	// We prefer to use an overlayfs bundler where possible. It roughly doubles
	// the disk space per image because it caches layers as overlay compatible
	// directories in addition to the CachingImagePuller's cache of uncompressed
	// layer tarballs. The advantage is faster start times for containers with
	// cached image, because it creates an overlay rootfs. The uncompressed
	// bundler on the other hand must untar all of a containers layers to create
	// a new rootfs each time it runs a container.
	var s store.Bundler = uncompressed.NewBundler(c.CacheDir)
	if overlay.Supported(c.CacheDir) {
		s, err = overlay.NewCachingBundler(c.CacheDir)
	}
	if err != nil {
		return errors.Wrap(err, errNewBundleStore)
	}

	// This store maps OCI references to their last known digests. We use it to
	// resolve references when the imagePullPolicy is Never or IfNotPresent.
	h, err := store.NewDigest(c.CacheDir)
	if err != nil {
		return errors.Wrap(err, errNewDigestStore)
	}

	r, err := name.ParseReference(req.GetImage())
	if err != nil {
		return errors.Wrap(err, errParseRef)
	}

	// We cache every image we pull to the filesystem. Layers are cached as
	// uncompressed tarballs. This allows them to be extracted quickly when
	// using the uncompressed.Bundler, which extracts a new root filesystem for
	// every container run.
	p := oci.NewCachingPuller(h, store.NewImage(c.CacheDir), &oci.RemoteClient{})
	img, err := p.Image(ctx, r, FromImagePullConfig(req.GetImagePullConfig()))
	if err != nil {
		return errors.Wrap(err, errPull)
	}

	// Create an OCI runtime bundle for this container run.
	b, err := s.Bundle(ctx, img, runID, FromRunFunctionConfig(req.GetRunFunctionConfig()))
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
	// to return without sitting in-between xfn and crun. It's also generally
	// recommended; 'run' is more for testing. In practice though run seems to
	// work just fine for our use case.

	//nolint:gosec // Executing with user-supplied input is intentional.
	cmd := exec.CommandContext(ctx, c.Runtime, "--root="+root, "run", "--bundle="+b.Path(), runID)
	cmd.Stdin = bytes.NewReader(req.GetInput())

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = b.Cleanup()
		return errors.Wrap(err, errRuntime)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = b.Cleanup()
		return errors.Wrap(err, errRuntime)
	}

	if err := cmd.Start(); err != nil {
		_ = b.Cleanup()
		return errors.Wrap(err, errRuntime)
	}

	stdout, err := io.ReadAll(limitReaderIfNonZero(stdoutPipe, c.MaxStdioBytes))
	if err != nil {
		_ = b.Cleanup()
		return errors.Wrap(err, errRuntime)
	}
	stderr, err := io.ReadAll(limitReaderIfNonZero(stderrPipe, c.MaxStdioBytes))
	if err != nil {
		_ = b.Cleanup()
		return errors.Wrap(err, errRuntime)
	}

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitErr.Stderr = stderr
		}
		_ = b.Cleanup()
		return errors.Wrap(err, errRuntime)
	}

	if err := b.Cleanup(); err != nil {
		return errors.Wrap(err, errCleanupBundle)
	}

	rsp := &v1alpha1.RunFunctionResponse{Output: stdout}
	pb, err = proto.Marshal(rsp)
	if err != nil {
		return errors.Wrap(err, errMarshalResponse)
	}
	_, err = os.Stdout.Write(pb)
	return errors.Wrap(err, errWriteResponse)
}

func limitReaderIfNonZero(r io.Reader, limit int64) io.Reader {
	if limit == 0 {
		return r
	}
	return io.LimitReader(r, limit)
}

// FromImagePullConfig configures an image client with options derived from the
// supplied ImagePullConfig.
func FromImagePullConfig(cfg *v1alpha1.ImagePullConfig) oci.ImageClientOption {
	return func(o *oci.ImageClientOptions) {
		switch cfg.GetPullPolicy() {
		case v1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_ALWAYS:
			oci.WithPullPolicy(oci.ImagePullPolicyAlways)(o)
		case v1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_NEVER:
			oci.WithPullPolicy(oci.ImagePullPolicyNever)(o)
		case v1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_IF_NOT_PRESENT, v1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_UNSPECIFIED:
			oci.WithPullPolicy(oci.ImagePullPolicyIfNotPresent)(o)
		}
		if a := cfg.GetAuth(); a != nil {
			oci.WithPullAuth(&oci.ImagePullAuth{
				Username:      a.GetUsername(),
				Password:      a.GetPassword(),
				Auth:          a.GetAuth(),
				IdentityToken: a.GetIdentityToken(),
				RegistryToken: a.GetRegistryToken(),
			})(o)
		}
	}
}

// FromRunFunctionConfig extends a runtime spec with configuration derived from
// the supplied RunFunctionConfig.
func FromRunFunctionConfig(cfg *v1alpha1.RunFunctionConfig) spec.Option {
	return func(s *runtime.Spec) error {
		if l := cfg.GetResources().GetLimits().GetCpu(); l != "" {
			if err := spec.WithCPULimit(l)(s); err != nil {
				return errors.Wrap(err, errCPULimit)
			}
		}

		if l := cfg.GetResources().GetLimits().GetMemory(); l != "" {
			if err := spec.WithMemoryLimit(l)(s); err != nil {
				return errors.Wrap(err, errMemoryLimit)
			}
		}

		if cfg.GetNetwork().GetPolicy() == v1alpha1.NetworkPolicy_NETWORK_POLICY_RUNNER {
			if err := spec.WithHostNetwork()(s); err != nil {
				return errors.Wrap(err, errHostNetwork)
			}
		}

		return nil
	}
}

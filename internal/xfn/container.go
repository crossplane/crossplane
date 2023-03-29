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
	"io"
	"net"

	"google.golang.org/grpc"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
)

// Error strings.
const (
	errListen = "cannot listen for gRPC connections"
	errServe  = "cannot serve gRPC API"
)

const defaultCacheDir = "/xfn"

// An ContainerRunner runs a Composition Function packaged as an OCI image by
// extracting it and running it as a 'rootless' container.
type ContainerRunner struct {
	v1alpha1.UnimplementedContainerizedFunctionRunnerServiceServer

	log logging.Logger

	rootUID int
	rootGID int
	setuid  bool // Specifically, CAP_SETUID and CAP_SETGID.
	cache   string
}

// A ContainerRunnerOption configures a new ContainerRunner.
type ContainerRunnerOption func(*ContainerRunner)

// MapToRoot configures what UID and GID should map to root (UID/GID 0) in the
// user namespace in which the function will be run.
func MapToRoot(uid, gid int) ContainerRunnerOption {
	return func(r *ContainerRunner) {
		r.rootUID = uid
		r.rootGID = gid
	}
}

// SetUID indicates that the container runner should attempt operations that
// require CAP_SETUID and CAP_SETGID, for example creating a user namespace that
// maps arbitrary UIDs and GIDs to the parent namespace.
func SetUID(s bool) ContainerRunnerOption {
	return func(r *ContainerRunner) {
		r.setuid = s
	}
}

// WithCacheDir specifies the directory used for caching function images and
// containers.
func WithCacheDir(d string) ContainerRunnerOption {
	return func(r *ContainerRunner) {
		r.cache = d
	}
}

// WithLogger configures which logger the container runner should use. Logging
// is disabled by default.
func WithLogger(l logging.Logger) ContainerRunnerOption {
	return func(cr *ContainerRunner) {
		cr.log = l
	}
}

// NewContainerRunner returns a new Runner that runs functions as rootless
// containers.
func NewContainerRunner(o ...ContainerRunnerOption) *ContainerRunner {
	r := &ContainerRunner{cache: defaultCacheDir, log: logging.NewNopLogger()}
	for _, fn := range o {
		fn(r)
	}

	return r
}

// ListenAndServe gRPC connections at the supplied address.
func (r *ContainerRunner) ListenAndServe(network, address string) error {
	r.log.Debug("Listening", "network", network, "address", address)
	lis, err := net.Listen(network, address)
	if err != nil {
		return errors.Wrap(err, errListen)
	}

	// TODO(negz): Limit concurrent function runs?
	srv := grpc.NewServer()
	v1alpha1.RegisterContainerizedFunctionRunnerServiceServer(srv, r)
	return errors.Wrap(srv.Serve(lis), errServe)
}

// Stdio can be used to read and write a command's standard I/O.
type Stdio struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

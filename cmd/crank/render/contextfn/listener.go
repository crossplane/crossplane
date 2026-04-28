/*
Copyright 2026 The Crossplane Authors.

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

package contextfn

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// Handle is the owner of a running in-process context function.
type Handle struct {
	// Target is the gRPC target that dials the function. Set this as the
	// FunctionInput address passed to the render engine.
	Target string

	srv          *grpc.Server
	fn           *server
	socketPath   string
	dir          string
	stop         sync.Once
	log          logging.Logger
	seedInput    *runtime.RawExtension
	captureInput *runtime.RawExtension
}

// Captured returns the context observed by the capture step, or nil if
// capture did not run.
func (h *Handle) Captured() *structpb.Struct {
	return h.fn.capturedContext()
}

// Start starts an in-process gRPC server that implements the composition
// function RunFunction RPC for context seeding and capture. The server
// listens on a unix-domain socket inside a fresh temp directory. Callers
// must call Handle.Stop when done.
func Start(ctx context.Context, log logging.Logger, contextData map[string]any) (*Handle, error) {
	si, err := json.Marshal(input{Mode: modeSeed})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create seed context function input")
	}
	ci, err := json.Marshal(input{Mode: modeCapture})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create capture context function input")
	}

	dir, err := os.MkdirTemp("", "render-ctx-*")
	if err != nil {
		return nil, errors.Wrap(err, "cannot create temp dir for context function socket")
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	sockPath := filepath.Join(dir, "s")
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "unix", sockPath)
	if err != nil {
		cleanup()
		return nil, errors.Wrapf(err, "cannot listen on %q", sockPath)
	}

	cleanup = func() {
		_ = lis.Close()
		_ = os.RemoveAll(dir)
	}

	// In order for processes in Docker containers to connect to the socket, the
	// socket must be world-writeable and its containing directory must be
	// world-readable.
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // Necessary.
		cleanup()
		return nil, errors.Wrapf(err, "cannot make socket directory world-readable")
	}
	if err := os.Chmod(sockPath, 0o777); err != nil { //nolint:gosec // Necessary.
		cleanup()
		return nil, errors.Wrapf(err, "cannot make socket file writeable")
	}

	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	fn := newServer(contextData)
	fnv1.RegisterFunctionRunnerServiceServer(srv, fn)

	h := &Handle{
		Target:       "unix://" + sockPath,
		srv:          srv,
		fn:           fn,
		socketPath:   sockPath,
		dir:          dir,
		log:          log,
		seedInput:    &runtime.RawExtension{Raw: si},
		captureInput: &runtime.RawExtension{Raw: ci},
	}

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Debug("Context function gRPC server stopped", "error", err)
		}
	}()

	return h, nil
}

// Stop gracefully stops the function server and removes the socket directory.
// Safe to call multiple times.
func (h *Handle) Stop() {
	h.stop.Do(func() {
		done := make(chan struct{})
		go func() {
			h.srv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			h.srv.Stop()
		}
		if err := os.RemoveAll(h.dir); err != nil {
			h.log.Debug("Cannot remove context function socket directory", "dir", h.dir, "error", err)
		}
	})
}

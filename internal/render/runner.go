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

package render

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/crossplane/v2/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// Wait for the server to be ready before sending RPCs.
const waitForReady = `{
	"methodConfig":[{
		"name": [{}],
		"waitForReady": true
	}]
}`

// A FunctionRunner runs composition functions by name via gRPC. It maps
// function names to pre-established gRPC connections.
type FunctionRunner struct {
	conns map[string]*grpc.ClientConn
}

// NewFunctionRunner returns a FunctionRunner that connects to the supplied
// functions. Each FunctionInput maps a function name to a gRPC address. The
// caller is responsible for starting the function runtimes; this constructor
// only establishes gRPC connections.
func NewFunctionRunner(fns []*renderv1alpha1.FunctionInput) (*FunctionRunner, error) {
	conns := make(map[string]*grpc.ClientConn, len(fns))

	for _, fn := range fns {
		if _, ok := conns[fn.GetName()]; ok {
			continue
		}
		conn, err := grpc.NewClient(fn.GetAddress(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(waitForReady))
		if err != nil {
			// Clean up any connections we already opened.
			for _, c := range conns {
				_ = c.Close()
			}
			return nil, errors.Wrapf(err, "cannot connect to function %q at %q", fn.GetName(), fn.GetAddress())
		}
		conns[fn.GetName()] = conn
	}

	return &FunctionRunner{conns: conns}, nil
}

// RunFunction calls the named function via its gRPC connection.
func (r *FunctionRunner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	conn, ok := r.conns[name]
	if !ok {
		return nil, errors.Errorf("unknown function %q - is it listed in the render input?", name)
	}
	return xfn.NewBetaFallBackFunctionRunnerServiceClient(conn).RunFunction(ctx, req)
}

// Close closes all gRPC connections.
func (r *FunctionRunner) Close() error {
	for _, c := range r.conns {
		_ = c.Close()
	}
	return nil
}

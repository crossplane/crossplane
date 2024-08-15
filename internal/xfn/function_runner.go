/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

// Package xfn contains functionality for running Composition Functions.
package xfn

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	fnv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// Error strings.
const (
	errListFunctionRevisions = "cannot list FunctionRevisions"
	errNoActiveRevisions     = "cannot find an active FunctionRevision (a FunctionRevision with spec.desiredState: Active)"
	errListFunctions         = "cannot List Functions to determine which gRPC client connections to garbage collect."

	errFmtGetClientConn = "cannot get gRPC client connection for Function %q"
	errFmtRunFunction   = "cannot run Function %q"
	errFmtEmptyEndpoint = "cannot determine gRPC target: active FunctionRevision %q has an empty status.endpoint"
	errFmtDialFunction  = "cannot gRPC dial target %q from status.endpoint of active FunctionRevision %q"
)

// TODO(negz): Should any of these be configurable?
const (
	// This configures a gRPC client to use round robin load balancing. This
	// means that if the Function Deployment has more than one Pod, and the
	// Function Service is headless, requests will be spread across each Pod.
	// See https://github.com/grpc/grpc/blob/v1.58.0/doc/load-balancing.md#load-balancing-policies
	lbRoundRobin = `{"loadBalancingConfig":[{"round_robin":{}}]}`

	dialFunctionTimeout = 10 * time.Second
	runFunctionTimeout  = 10 * time.Second
)

// A PackagedFunctionRunner runs a Function by making a gRPC call to a Function
// package's runtime. It creates a gRPC client connection for each Function. The
// Function's endpoint is determined by reading the status.endpoint of the
// active FunctionRevision. You must call GarbageCollectClientConnections in
// order to ensure connections are properly closed.
type PackagedFunctionRunner struct {
	client       client.Reader
	creds        credentials.TransportCredentials
	interceptors []InterceptorCreator

	connsMx sync.RWMutex
	conns   map[string]*grpc.ClientConn

	log logging.Logger
}

// An InterceptorCreator creates gRPC UnaryClientInterceptors for functions.
type InterceptorCreator interface {
	// CreateInterceptor creates an interceptor for the named function. It also
	// accepts the function's package OCI reference, which may be used by the
	// interceptor (e.g. to label metrics).
	CreateInterceptor(name, pkg string) grpc.UnaryClientInterceptor
}

// A PackagedFunctionRunnerOption configures a PackagedFunctionRunner.
type PackagedFunctionRunnerOption func(r *PackagedFunctionRunner)

// WithLogger configures the logger the PackagedFunctionRunner should use.
func WithLogger(l logging.Logger) PackagedFunctionRunnerOption {
	return func(r *PackagedFunctionRunner) {
		r.log = l
	}
}

// WithTLSConfig configures the client TLS the PackagedFunctionRunner should use.
func WithTLSConfig(cfg *tls.Config) PackagedFunctionRunnerOption {
	return func(r *PackagedFunctionRunner) {
		r.creds = credentials.NewTLS(cfg)
	}
}

// WithInterceptorCreators configures the interceptors the
// PackagedFunctionRunner should create for each function.
func WithInterceptorCreators(ics ...InterceptorCreator) PackagedFunctionRunnerOption {
	return func(r *PackagedFunctionRunner) {
		r.interceptors = ics
	}
}

// NewPackagedFunctionRunner returns a FunctionRunner that runs a Function by
// making a gRPC call to a Function package's runtime.
func NewPackagedFunctionRunner(c client.Reader, o ...PackagedFunctionRunnerOption) *PackagedFunctionRunner {
	r := &PackagedFunctionRunner{
		client: c,
		creds:  insecure.NewCredentials(),
		conns:  make(map[string]*grpc.ClientConn),
		log:    logging.NewNopLogger(),
	}

	for _, fn := range o {
		fn(r)
	}

	return r
}

// RunFunction sends the supplied RunFunctionRequest to the named Function. The
// function is expected to be an installed Function.pkg.crossplane.io package.
func (r *PackagedFunctionRunner) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	conn, err := r.getClientConn(ctx, name)
	if err != nil {
		return nil, errors.Wrapf(err, errFmtGetClientConn, name)
	}

	// This context is used for actually making the request.
	ctx, cancel := context.WithTimeout(ctx, runFunctionTimeout)
	defer cancel()

	rsp, err := NewBetaFallBackFunctionRunnerServiceClient(conn).RunFunction(ctx, req)
	return rsp, errors.Wrapf(err, errFmtRunFunction, name)
}

// In most cases our gRPC target will be a Kubernetes Service. The package
// manager creates this service for each active FunctionRevision, but the
// Service is aligned with the Function. It's name is derived from the Function
// (not the FunctionRevision). This means the target won't change just because a
// new FunctionRevision was created.
//
// However, once the runtime config design is implemented it's possible that
// something other than the package manager will reconcile FunctionRevisions.
// There's no guarantee it will create a Service, or that the endpoint will
// remain stable across FunctionRevisions.
//
// https://github.com/crossplane/crossplane/blob/226b81f/design/one-pager-package-runtime-config.md
//
// With this in mind, we attempt to:
//
// * Create a connection the first time someone runs a Function.
// * Cache it so we don't pay the setup cost every time the Function is called.
// * Verify that it has the correct target every time the Function is called.
//
// In the happy path, where a client already exists, this means we'll pay the
// cost of listing and iterating over FunctionRevisions from cache. The default
// RevisionHistoryLimit is 1, so for most Functions we'd expect there to be two
// revisions in the cache (one active, and one previously active).
func (r *PackagedFunctionRunner) getClientConn(ctx context.Context, name string) (*grpc.ClientConn, error) {
	log := r.log.WithValues("function", name)

	l := &pkgv1.FunctionRevisionList{}
	if err := r.client.List(ctx, l, client.MatchingLabels{pkgv1.LabelParentPackage: name}); err != nil {
		return nil, errors.Wrapf(err, errListFunctionRevisions)
	}

	var active *pkgv1.FunctionRevision
	for i := range l.Items {
		if l.Items[i].GetDesiredState() == pkgv1.PackageRevisionActive {
			active = &l.Items[i]
			break
		}
	}
	if active == nil {
		return nil, errors.New(errNoActiveRevisions)
	}

	if active.Status.Endpoint == "" {
		return nil, errors.Errorf(errFmtEmptyEndpoint, active.GetName())
	}

	// If we have a connection for the up-to-date endpoint, return it.
	r.connsMx.RLock()
	conn, ok := r.conns[name]
	if ok && conn.Target() == active.Status.Endpoint {
		defer r.connsMx.RUnlock()
		return conn, nil
	}
	r.connsMx.RUnlock()

	// Either we didn't have a connection, or it wasn't up-to-date.
	r.connsMx.Lock()
	defer r.connsMx.Unlock()

	// Another Goroutine might have updated the connections between when we
	// released the read lock and took the write lock, so check again.
	conn, ok = r.conns[name]
	if ok {
		// We now have a connection for the up-to-date endpoint.
		if conn.Target() == active.Status.Endpoint {
			return conn, nil
		}

		// This connection is to an old endpoint. We need to close it and create
		// a new connection. Close only returns an error is if the connection is
		// already closed or in the process of closing.
		log.Debug("Closing gRPC client connection with stale target", "old-target", conn.Target(), "new-target", active.Status.Endpoint)
		_ = conn.Close()
		delete(r.conns, name)
	}

	// This context is only used for setting up the connection.
	ctx, cancel := context.WithTimeout(ctx, dialFunctionTimeout)
	defer cancel()

	is := make([]grpc.UnaryClientInterceptor, len(r.interceptors))
	for i := range r.interceptors {
		is[i] = r.interceptors[i].CreateInterceptor(name, active.Spec.Package)
	}

	conn, err := grpc.DialContext(ctx, active.Status.Endpoint,
		grpc.WithTransportCredentials(r.creds),
		grpc.WithDefaultServiceConfig(lbRoundRobin),
		grpc.WithChainUnaryInterceptor(is...))
	if err != nil {
		return nil, errors.Wrapf(err, errFmtDialFunction, active.Status.Endpoint, active.GetName())
	}

	r.conns[name] = conn

	log.Debug("Created new gRPC client connection", "target", active.Status.Endpoint)
	return conn, nil
}

// GarbageCollectConnections runs every interval until the supplied context is
// cancelled. It garbage collects gRPC client connections to Functions that are
// no longer installed.
func (r *PackagedFunctionRunner) GarbageCollectConnections(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Debug("Stopping gRPC client connection garbage collector", "error", ctx.Err())
			return
		case <-t.C:
			if _, err := r.GarbageCollectConnectionsNow(ctx); err != nil {
				r.log.Info("Cannot garbage collect gRPC client connections", "error", err)
			}
		}
	}
}

// GarbageCollectConnectionsNow immediately garbage collects any gRPC client
// connections to Functions that are no longer installed. It returns the number
// of connections garbage collected.
func (r *PackagedFunctionRunner) GarbageCollectConnectionsNow(ctx context.Context) (int, error) {
	// We try to take the write lock for as little time as possible,
	// because while we have it RunFunction will block. In the happy
	// path where no connections need garbage collecting we shouldn't
	// take it at all.

	// No need to take a write lock or list Functions if there's no work to do.
	r.connsMx.RLock()
	if len(r.conns) == 0 {
		defer r.connsMx.RUnlock()
		return 0, nil
	}
	r.connsMx.RUnlock()

	r.connsMx.Lock()
	defer r.connsMx.Unlock()

	l := &pkgv1.FunctionList{}
	if err := r.client.List(ctx, l); err != nil {
		return 0, errors.Wrap(err, errListFunctions)
	}

	functionExists := map[string]bool{}
	for _, f := range l.Items {
		functionExists[f.GetName()] = true
	}

	// Garbage collect connections.
	closed := 0
	for name := range r.conns {
		if functionExists[name] {
			continue
		}

		// Close only returns an error is if the connection is already
		// closed or in the process of closing.
		_ = r.conns[name].Close()
		delete(r.conns, name)
		closed++
		r.log.Debug("Closed gRPC client connection to Function that is no longer installed", "function", name)
	}

	return closed, nil
}

// A BetaFallBackFunctionRunnerServiceClient tries to send a v1 RPC. If the
// server reports that v1 is unimplemented, it falls back to sending a v1beta1
// RPC. It translates the v1 RunFunctionRequest to v1beta1 by round-tripping it
// through protobuf encoding. This works because the two messages are guaranteed
// to be identical - the v1beta1 proto is replicated from the v1 proto.
type BetaFallBackFunctionRunnerServiceClient struct {
	cc *grpc.ClientConn
}

// NewBetaFallBackFunctionRunnerServiceClient returns a client that falls back
// to v1beta1 when v1 is unimplemented.
func NewBetaFallBackFunctionRunnerServiceClient(cc *grpc.ClientConn) *BetaFallBackFunctionRunnerServiceClient {
	return &BetaFallBackFunctionRunnerServiceClient{cc: cc}
}

// RunFunction tries to send a v1 RunFunctionRequest. It falls back to v1beta1
// if the v1 service is unimplemented.
func (c *BetaFallBackFunctionRunnerServiceClient) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest, opts ...grpc.CallOption) (*fnv1.RunFunctionResponse, error) {
	rsp, err := fnv1.NewFunctionRunnerServiceClient(c.cc).RunFunction(ctx, req, opts...)

	// If the v1 RPC worked, just return the response.
	if err == nil {
		return rsp, nil
	}

	// If we hit an error other than Unimplemented, return it.
	if status.Code(err) != codes.Unimplemented {
		return nil, err
	}

	// The v1 RPC is unimplemented. Try the v1beta1 equivalent. The messages
	// should be identical in Go and on the wire.
	breq, err := toBeta(req)
	if err != nil {
		return nil, err
	}
	brsp, err := fnv1beta1.NewFunctionRunnerServiceClient(c.cc).RunFunction(ctx, breq, opts...)
	if err != nil {
		return nil, err
	}

	rsp, err = fromBeta(brsp)
	return rsp, err
}

func toBeta(req *fnv1.RunFunctionRequest) (*fnv1beta1.RunFunctionRequest, error) {
	out := &fnv1beta1.RunFunctionRequest{}
	b, err := proto.Marshal(req)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal %T to protobuf bytes", req)
	}
	err = proto.Unmarshal(b, out)
	return out, errors.Wrapf(err, "cannot unmarshal %T protobuf bytes into %T", req, out)
}

func fromBeta(rsp *fnv1beta1.RunFunctionResponse) (*fnv1.RunFunctionResponse, error) {
	out := &fnv1.RunFunctionResponse{}
	b, err := proto.Marshal(rsp)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal %T to protobuf bytes", rsp)
	}
	err = proto.Unmarshal(b, out)
	return out, errors.Wrapf(err, "cannot unmarshal %T protobuf bytes into %T", rsp, out)
}

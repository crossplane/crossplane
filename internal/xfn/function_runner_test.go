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

package xfn

import (
	"context"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
	fnv1beta1 "github.com/crossplane/crossplane/v2/proto/fn/v1beta1"
)

var _ fnv1.FunctionRunnerServiceClient = &BetaFallBackFunctionRunnerServiceClient{}

func TestRunFunction(t *testing.T) {
	errBoom := errors.New("boom")

	// Make sure to add servers listeners here, for us to later close.
	listeners := make([]net.Listener, 0)

	type params struct {
		c client.Client
		o []PackagedFunctionRunnerOption
	}

	type args struct {
		ctx  context.Context
		name string
		req  *fnv1.RunFunctionRequest
	}

	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"ListFunctionRevisionError": {
			reason: "We should return an error if we can't get (or verify) a client connection because we can't list FunctionRevisions",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errBoom, errListFunctionRevisions), errFmtGetClientConn, "cool-fn"),
			},
		},
		"NoActiveRevisions": {
			reason: "We should return an error if we can't get (or verify) a client connection because no FunctionRevision is active",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
							{
								Spec: pkgv1.FunctionRevisionSpec{
									PackageRevisionSpec: pkgv1.PackageRevisionSpec{
										DesiredState: pkgv1.PackageRevisionInactive, // This revision is not active.
									},
								},
							},
						}
						return nil
					}),
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
			},
			want: want{
				err: errors.Wrapf(errors.New(errNoActiveRevisions), errFmtGetClientConn, "cool-fn"),
			},
		},
		"ActiveRevisionHasNoEndpoint": {
			reason: "We should return an error if we can't get (or verify) a client connection because the active FunctionRevision has an empty status.endpoint",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-fn-revision-a",
								},
								Spec: pkgv1.FunctionRevisionSpec{
									PackageRevisionSpec: pkgv1.PackageRevisionSpec{
										DesiredState: pkgv1.PackageRevisionActive,
									},
								},
								Status: pkgv1.FunctionRevisionStatus{
									Endpoint: "", // An empty endpoint.
								},
							},
						}
						return nil
					}),
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtEmptyEndpoint, "cool-fn-revision-a"), errFmtGetClientConn, "cool-fn"),
			},
		},
		"SuccessfulRequest": {
			reason: "We should create a new client connection and successfully make a request if no client already exists",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						// Start a gRPC server.
						lis := NewGRPCServer(t, &MockFunctionServer{rsp: &fnv1.RunFunctionResponse{
							Meta: &fnv1.ResponseMeta{Tag: "hi!"},
						}})
						listeners = append(listeners, lis)

						l, ok := obj.(*pkgv1.FunctionRevisionList)
						if !ok {
							// If we're called to list Functions we want to
							// return none, to make sure we GC everything.
							return nil
						}
						l.Items = []pkgv1.FunctionRevision{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-fn-revision-a",
								},
								Spec: pkgv1.FunctionRevisionSpec{
									PackageRevisionSpec: pkgv1.PackageRevisionSpec{
										DesiredState: pkgv1.PackageRevisionActive,
									},
								},
								Status: pkgv1.FunctionRevisionStatus{
									Endpoint: strings.Replace(lis.Addr().String(), "127.0.0.1", "dns:///localhost", 1),
								},
							},
						}
						return nil
					}),
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hi!"},
				},
			},
		},
		"SuccessfulFallbackToBeta": {
			reason: "We should create a new client connection and successfully make a v1beta1 request if the server doesn't yet implement v1",
			params: params{
				c: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						// Start a gRPC server.
						lis := NewBetaGRPCServer(t, &MockBetaFunctionServer{rsp: &fnv1beta1.RunFunctionResponse{
							Meta: &fnv1beta1.ResponseMeta{Tag: "hi!"},
						}})
						listeners = append(listeners, lis)

						l, ok := obj.(*pkgv1.FunctionRevisionList)
						if !ok {
							// If we're called to list Functions we want to
							// return none, to make sure we GC everything.
							return nil
						}
						l.Items = []pkgv1.FunctionRevision{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-fn-revision-a",
								},
								Spec: pkgv1.FunctionRevisionSpec{
									PackageRevisionSpec: pkgv1.PackageRevisionSpec{
										DesiredState: pkgv1.PackageRevisionActive,
									},
								},
								Status: pkgv1.FunctionRevisionStatus{
									Endpoint: strings.Replace(lis.Addr().String(), "127.0.0.1", "dns:///localhost", 1),
								},
							},
						}
						return nil
					}),
				},
			},
			args: args{
				ctx:  context.Background(),
				name: "cool-fn",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hi!"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewPackagedFunctionRunner(tc.params.c, tc.params.o...)
			rsp, err := r.RunFunction(tc.args.ctx, tc.args.name, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Close any gRPC clients.
			if _, err := r.GarbageCollectConnectionsNow(context.Background()); err != nil {
				t.Logf("Error closing client connections: %s", err)
			}
		})
	}

	// Closing these listeners will close any gRPC servers.
	for _, lis := range listeners {
		if err := lis.Close(); err != nil {
			t.Logf("Error closing server listener: %s", err)
		}
	}
}

func TestGetClientConn(t *testing.T) {
	t.Helper()

	// TestRunFunction exercises most of the getClientConn code. Here we just
	// test some cases that don't fit well in our usual table-driven format.

	// Start a gRPC server.
	lis := NewGRPCServer(t, &MockFunctionServer{rsp: &fnv1.RunFunctionResponse{
		Meta: &fnv1.ResponseMeta{Tag: "hi!"},
	}})
	defer lis.Close()

	target := strings.Replace(lis.Addr().String(), "127.0.0.1", "dns:///localhost", 1)

	c := &test.MockClient{
		MockList: NewListFn(target),
	}

	r := NewPackagedFunctionRunner(c)

	// We should be able to create a new connection.
	t.Run("CreateNewConnection", func(t *testing.T) {
		conn, err := r.getClientConn(context.Background(), "cool-fn")

		if diff := cmp.Diff(target, conn.Target()); diff != "" {
			t.Errorf("\nr.getClientConn(...): -want, +got:\n%s", diff)
		}

		if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
			t.Errorf("\nr.getClientConn(...): -want error, +got error:\n%s", diff)
		}
	})

	// If we're called again and our FunctionRevision's endpoint hasn't changed,
	// we should return our cached connection.
	t.Run("ReuseExistingConnection", func(t *testing.T) {
		conn, err := r.getClientConn(context.Background(), "cool-fn")

		if diff := cmp.Diff(target, conn.Target()); diff != "" {
			t.Errorf("\nr.getClientConn(...): -want, +got:\n%s", diff)
		}

		if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
			t.Errorf("\nr.getClientConn(...): -want error, +got error:\n%s", diff)
		}
	})

	// Start another gRPC server.
	lis2 := NewGRPCServer(t, &MockFunctionServer{rsp: &fnv1.RunFunctionResponse{
		Meta: &fnv1.ResponseMeta{Tag: "hi!"},
	}})
	defer lis2.Close()

	target = strings.Replace(lis2.Addr().String(), "127.0.0.1", "dns:///localhost", 1)
	c.MockList = NewListFn(target)

	// If we're called again and our FunctionRevision's endpoint _has_ changed,
	// we should close our cached connection and create a new one.
	t.Run("ReplaceExistingConnection", func(t *testing.T) {
		conn, err := r.getClientConn(context.Background(), "cool-fn")

		if diff := cmp.Diff(target, conn.Target()); diff != "" {
			t.Errorf("\nr.getClientConn(...): -want, +got:\n%s", diff)
		}

		if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
			t.Errorf("\nr.getClientConn(...): -want error, +got error:\n%s", diff)
		}
	})

	// Close any gRPC clients.
	if _, err := r.GarbageCollectConnectionsNow(context.Background()); err != nil {
		t.Logf("Error closing client connections: %s", err)
	}
}

func TestRunFunctionDetectsDeadConnection(t *testing.T) {
	// Start a gRPC server, with a TCP proxy in front of it that lets us
	// simulate a Function pod that goes away without closing its connection.
	lis := NewGRPCServer(t, &MockFunctionServer{rsp: &fnv1.RunFunctionResponse{
		Meta: &fnv1.ResponseMeta{Tag: "hi!"},
	}})
	defer lis.Close()

	proxy := NewBlackholeProxy(t, lis.Addr().String())
	defer proxy.Close()

	target := strings.Replace(proxy.Addr().String(), "127.0.0.1", "dns:///localhost", 1)

	c := &test.MockClient{
		MockList: NewListFn(target),
	}

	// Use the most aggressive keepalive gRPC allows, so the test runs quickly.
	r := NewPackagedFunctionRunner(c, WithKeepaliveParameters(keepalive.ClientParameters{
		Time:    10 * time.Second,
		Timeout: 2 * time.Second,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Establish a connection to the Function.
	if _, err := r.RunFunction(ctx, "cool-fn", &fnv1.RunFunctionRequest{}); err != nil {
		t.Fatalf("r.RunFunction(...): unexpected error: %s", err)
	}

	// The Function pod goes away without closing its connection. Traffic to
	// it is silently dropped.
	proxy.Blackhole()

	// Keepalive should detect the dead connection and fail this RPC. Without
	// keepalive it would block until the context deadline.
	start := time.Now()
	_, err := r.RunFunction(ctx, "cool-fn", &fnv1.RunFunctionRequest{})

	if diff := cmp.Diff(codes.Unavailable, status.Code(err)); diff != "" {
		t.Errorf("\nr.RunFunction(...) on dead connection returned after %s: -want code, +got code:\n%s", time.Since(start), diff)
	}

	// Detecting the dead connection should have dropped it, so this RPC must
	// be served by a new connection.
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := r.RunFunction(ctx, "cool-fn", &fnv1.RunFunctionRequest{}); err != nil {
		t.Errorf("r.RunFunction(...) after dead connection was dropped: unexpected error: %s", err)
	}

	// Close any gRPC clients.
	if _, err := r.GarbageCollectConnectionsNow(context.Background()); err != nil {
		t.Logf("Error closing client connections: %s", err)
	}
}

func TestGarbageCollectConnectionsNow(t *testing.T) {
	// TestRunFunction exercises most of the GarbageCollectConnectionsNow code.
	// Here we just test some cases that don't fit well in our usual
	// table-driven format.

	// Start a gRPC server.
	lis := NewGRPCServer(t, &MockFunctionServer{rsp: &fnv1.RunFunctionResponse{
		Meta: &fnv1.ResponseMeta{Tag: "hi!"},
	}})
	defer lis.Close()

	target := strings.Replace(lis.Addr().String(), "127.0.0.1", "dns:///localhost", 1)

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("gRPC dial failed: %s", err)
	}

	c := &test.MockClient{}
	r := NewPackagedFunctionRunner(c)

	// Add our connection to our pool.
	r.connsMx.Lock()
	r.conns["cool-fn"] = conn
	r.connsMx.Unlock()

	ctx := context.Background()

	t.Run("FunctionStillExistsDoNotGarbageCollect", func(t *testing.T) {
		c.MockList = test.NewMockListFn(nil, func(obj client.ObjectList) error {
			obj.(*pkgv1.FunctionList).Items = []pkgv1.Function{
				{
					// This Function exists!
					ObjectMeta: metav1.ObjectMeta{Name: "cool-fn"},
				},
			}

			return nil
		})

		i, err := r.GarbageCollectConnectionsNow(ctx)

		if diff := cmp.Diff(0, i); diff != "" {
			t.Errorf("\nr.GarbageCollectConnectionsNow(...): -want, +got:\n%s", diff)
		}

		if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
			t.Errorf("\nr.GarbageCollectConnectionsNow(...): -want error, +got error:\n%s", diff)
		}
	})

	t.Run("FunctionDoesNotExistsGarbageCollect", func(t *testing.T) {
		// No Functions exist
		c.MockList = test.NewMockListFn(nil)

		i, err := r.GarbageCollectConnectionsNow(ctx)

		if diff := cmp.Diff(1, i); diff != "" {
			t.Errorf("\nr.GarbageCollectConnectionsNow(...): -want, +got:\n%s", diff)
		}

		if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
			t.Errorf("\nr.GarbageCollectConnectionsNow(...): -want error, +got error:\n%s", diff)
		}
	})
}

func NewListFn(target string) test.MockListFn {
	return test.NewMockListFn(nil, func(obj client.ObjectList) error {
		l, ok := obj.(*pkgv1.FunctionRevisionList)
		if !ok {
			// If we're called to list Functions we want to
			// return none, to make sure we GC everything.
			return nil
		}

		l.Items = []pkgv1.FunctionRevision{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool-fn-revision-a",
				},
				Spec: pkgv1.FunctionRevisionSpec{
					PackageRevisionSpec: pkgv1.PackageRevisionSpec{
						DesiredState: pkgv1.PackageRevisionActive,
					},
				},
				Status: pkgv1.FunctionRevisionStatus{
					Endpoint: target,
				},
			},
		}

		return nil
	})
}

func NewGRPCServer(t *testing.T, ss fnv1.FunctionRunnerServiceServer) net.Listener {
	t.Helper()

	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Listening for gRPC connections on %q", lis.Addr().String())

	// TODO(negz): Is it worth using a WaitGroup for these?
	go func() {
		s := grpc.NewServer()
		fnv1.RegisterFunctionRunnerServiceServer(s, ss)
		_ = s.Serve(lis)
	}()

	// The caller must close this listener to terminate the server.
	return lis
}

func NewBetaGRPCServer(t *testing.T, ss fnv1beta1.FunctionRunnerServiceServer) net.Listener {
	t.Helper()

	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Listening for gRPC connections on %q", lis.Addr().String())

	// TODO(negz): Is it worth using a WaitGroup for these?
	go func() {
		s := grpc.NewServer()
		fnv1beta1.RegisterFunctionRunnerServiceServer(s, ss)
		_ = s.Serve(lis)
	}()

	// The caller must close this listener to terminate the server.
	return lis
}

type MockFunctionServer struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	rsp *fnv1.RunFunctionResponse
	err error
}

func (s *MockFunctionServer) RunFunction(context.Context, *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	return s.rsp, s.err
}

type MockBetaFunctionServer struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	rsp *fnv1beta1.RunFunctionResponse
	err error
}

func (s *MockBetaFunctionServer) RunFunction(context.Context, *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	return s.rsp, s.err
}

// A BlackholeProxy forwards TCP connections to an upstream address. Calling
// Blackhole causes it to silently drop all traffic on the connections that
// exist at that time, while keeping them open. This simulates a peer that
// went away without closing its connections, e.g. a Function pod whose node
// failed. New connections are forwarded normally, like connections to a
// replacement pod would be.
type BlackholeProxy struct {
	lis      net.Listener
	upstream string
	gen      atomic.Int64
}

func NewBlackholeProxy(t *testing.T, upstream string) *BlackholeProxy {
	t.Helper()

	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Proxying TCP connections from %q to %q", lis.Addr().String(), upstream)

	p := &BlackholeProxy{lis: lis, upstream: upstream}

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}

			go p.forward(conn, p.gen.Load())
		}
	}()

	return p
}

// Addr returns the address the proxy listens on.
func (p *BlackholeProxy) Addr() net.Addr {
	return p.lis.Addr()
}

// Blackhole silently drops all traffic on existing connections.
func (p *BlackholeProxy) Blackhole() {
	p.gen.Add(1)
}

// Close stops the proxy accepting new connections.
func (p *BlackholeProxy) Close() error {
	return p.lis.Close()
}

func (p *BlackholeProxy) forward(client net.Conn, gen int64) {
	defer client.Close()

	server, err := net.Dial("tcp", p.upstream)
	if err != nil {
		return
	}
	defer server.Close()

	done := make(chan struct{}, 2)

	pipe := func(dst io.Writer, src io.Reader) {
		buf := make([]byte, 32*1024)

		for {
			n, err := src.Read(buf)
			if err != nil {
				break
			}

			// This connection is black-holed. Drop the data.
			if p.gen.Load() != gen {
				continue
			}

			if _, err := dst.Write(buf[:n]); err != nil {
				break
			}
		}

		done <- struct{}{}
	}

	go pipe(server, client)
	go pipe(client, server)

	<-done
}

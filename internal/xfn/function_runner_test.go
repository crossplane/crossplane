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
	"net"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
		ctx context.Context
		rev string
		req *fnv1.RunFunctionRequest
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
		"GetFunctionRevisionError": {
			reason: "We should return an error if we can't get (or verify) a client connection because we can't get the FunctionRevision",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			args: args{
				ctx: context.Background(),
				rev: "cool-fn-revision-a",
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errBoom, errGetFunctionRevision), errFmtGetClientConn, "cool-fn-revision-a"),
			},
		},
		"InactiveRevision": {
			reason: "We should return an error if we can't get (or verify) a client connection because the FunctionRevision is not active",
			params: params{
				c: &test.MockClient{
					MockGet: NewGetFn(pkgv1.PackageRevisionInactive, "dns:///localhost:1234"), // This revision is not active.
				},
			},
			args: args{
				ctx: context.Background(),
				rev: "cool-fn-revision-a",
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtInactiveRevision, "cool-fn-revision-a"), errFmtGetClientConn, "cool-fn-revision-a"),
			},
		},
		"ActiveRevisionHasNoEndpoint": {
			reason: "We should return an error if we can't get (or verify) a client connection because the active FunctionRevision has an empty status.endpoint",
			params: params{
				c: &test.MockClient{
					MockGet: NewGetFn(pkgv1.PackageRevisionActive, ""), // An empty endpoint.
				},
			},
			args: args{
				ctx: context.Background(),
				rev: "cool-fn-revision-a",
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtEmptyEndpoint, "cool-fn-revision-a"), errFmtGetClientConn, "cool-fn-revision-a"),
			},
		},
		"SuccessfulRequest": {
			reason: "We should create a new client connection and successfully make a request if no client already exists",
			params: params{
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						// Start a gRPC server.
						lis := NewGRPCServer(t, &MockFunctionServer{rsp: &fnv1.RunFunctionResponse{
							Meta: &fnv1.ResponseMeta{Tag: "hi!"},
						}})
						listeners = append(listeners, lis)

						return NewGetFn(pkgv1.PackageRevisionActive, strings.Replace(lis.Addr().String(), "127.0.0.1", "dns:///localhost", 1))(ctx, key, obj)
					},
					// Return no FunctionRevisions, to make sure we GC everything.
					MockList: test.NewMockListFn(nil),
				},
			},
			args: args{
				ctx: context.Background(),
				rev: "cool-fn-revision-a",
				req: &fnv1.RunFunctionRequest{},
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
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						// Start a gRPC server.
						lis := NewBetaGRPCServer(t, &MockBetaFunctionServer{rsp: &fnv1beta1.RunFunctionResponse{
							Meta: &fnv1beta1.ResponseMeta{Tag: "hi!"},
						}})
						listeners = append(listeners, lis)

						return NewGetFn(pkgv1.PackageRevisionActive, strings.Replace(lis.Addr().String(), "127.0.0.1", "dns:///localhost", 1))(ctx, key, obj)
					},
					// Return no FunctionRevisions, to make sure we GC everything.
					MockList: test.NewMockListFn(nil),
				},
			},
			args: args{
				ctx: context.Background(),
				rev: "cool-fn-revision-a",
				req: &fnv1.RunFunctionRequest{},
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
			rsp, err := r.RunFunction(tc.args.ctx, tc.args.rev, tc.args.req)

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
		MockGet:  NewGetFn(pkgv1.PackageRevisionActive, target),
		MockList: test.NewMockListFn(nil),
	}

	r := NewPackagedFunctionRunner(c)

	// We should be able to create a new connection.
	t.Run("CreateNewConnection", func(t *testing.T) {
		conn, err := r.getClientConn(context.Background(), "cool-fn-revision-a")

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
		conn, err := r.getClientConn(context.Background(), "cool-fn-revision-a")

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
	c.MockGet = NewGetFn(pkgv1.PackageRevisionActive, target)

	// If we're called again and our FunctionRevision's endpoint _has_ changed,
	// we should close our cached connection and create a new one.
	t.Run("ReplaceExistingConnection", func(t *testing.T) {
		conn, err := r.getClientConn(context.Background(), "cool-fn-revision-a")

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

type MockInterceptorCreator struct {
	names []string
	pkgs  []string
}

func (m *MockInterceptorCreator) CreateInterceptor(name, pkg string) grpc.UnaryClientInterceptor {
	m.names = append(m.names, name)
	m.pkgs = append(m.pkgs, pkg)

	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func TestGetClientConnInterceptors(t *testing.T) {
	ic := &MockInterceptorCreator{}
	c := &test.MockClient{
		MockGet:  NewGetFn(pkgv1.PackageRevisionActive, "dns:///localhost:1234"),
		MockList: test.NewMockListFn(nil),
	}
	r := NewPackagedFunctionRunner(c, WithInterceptorCreators(ic))

	if _, err := r.getClientConn(context.Background(), "cool-fn-revision-a"); err != nil {
		t.Fatalf("r.getClientConn(...): %s", err)
	}

	// Interceptors should be labelled with the parent Function's name, not the
	// FunctionRevision's, so that labels (e.g. metrics) are stable across
	// revisions.
	if diff := cmp.Diff([]string{"cool-fn"}, ic.names); diff != "" {
		t.Errorf("\nCreateInterceptor(name, ...): -want, +got:\n%s", diff)
	}

	if diff := cmp.Diff([]string{"xpkg.crossplane.io/crossplane-contrib/cool-fn:v0.1.0"}, ic.pkgs); diff != "" {
		t.Errorf("\nCreateInterceptor(..., pkg): -want, +got:\n%s", diff)
	}

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
	r.conns["cool-fn-abc"] = conn
	r.connsMx.Unlock()

	ctx := context.Background()

	t.Run("FunctionStillExistsDoNotGarbageCollect", func(t *testing.T) {
		c.MockList = test.NewMockListFn(nil, func(obj client.ObjectList) error {
			obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cool-fn-abc"},
					Spec: pkgv1.FunctionRevisionSpec{
						PackageRevisionSpec: pkgv1.PackageRevisionSpec{
							Package:      "xpkg.crossplane.io/example/cool-function:v1.0.0",
							DesiredState: pkgv1.PackageRevisionActive,
						},
					},
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

// NewGetFn returns a MockGetFn that gets a FunctionRevision named
// cool-fn-revision-a with the supplied desired state and endpoint.
func NewGetFn(state pkgv1.PackageRevisionDesiredState, endpoint string) test.MockGetFn {
	return test.NewMockGetFn(nil, func(obj client.Object) error {
		rev, ok := obj.(*pkgv1.FunctionRevision)
		if !ok {
			return errors.Errorf("unexpected object type %T", obj)
		}

		*rev = pkgv1.FunctionRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cool-fn-revision-a",
				Labels: map[string]string{
					pkgv1.LabelParentPackage: "cool-fn",
				},
			},
			Spec: pkgv1.FunctionRevisionSpec{
				PackageRevisionSpec: pkgv1.PackageRevisionSpec{
					Package:      "xpkg.crossplane.io/crossplane-contrib/cool-fn:v0.1.0",
					DesiredState: state,
				},
			},
			Status: pkgv1.FunctionRevisionStatus{
				Endpoint: endpoint,
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

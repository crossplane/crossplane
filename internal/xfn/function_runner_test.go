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
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/testing/protocmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

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
		req  *v1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *v1beta1.RunFunctionResponse
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
						obj.(*pkgv1beta1.FunctionRevisionList).Items = []pkgv1beta1.FunctionRevision{
							{
								Spec: pkgv1beta1.FunctionRevisionSpec{
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
						obj.(*pkgv1beta1.FunctionRevisionList).Items = []pkgv1beta1.FunctionRevision{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-fn-revision-a",
								},
								Spec: pkgv1beta1.FunctionRevisionSpec{
									PackageRevisionSpec: pkgv1.PackageRevisionSpec{
										DesiredState: pkgv1.PackageRevisionActive,
									},
								},
								Status: pkgv1beta1.FunctionRevisionStatus{
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
						lis := NewGRPCServer(t, &MockFunctionServer{rsp: &v1beta1.RunFunctionResponse{
							Meta: &v1beta1.ResponseMeta{Tag: "hi!"},
						}})
						listeners = append(listeners, lis)

						l, ok := obj.(*pkgv1beta1.FunctionRevisionList)
						if !ok {
							// If we're called to list Functions we want to
							// return none, to make sure we GC everything.
							return nil
						}
						l.Items = []pkgv1beta1.FunctionRevision{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-fn-revision-a",
								},
								Spec: pkgv1beta1.FunctionRevisionSpec{
									PackageRevisionSpec: pkgv1.PackageRevisionSpec{
										DesiredState: pkgv1.PackageRevisionActive,
									},
								},
								Status: pkgv1beta1.FunctionRevisionStatus{
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
				req:  &v1beta1.RunFunctionRequest{},
			},
			want: want{
				rsp: &v1beta1.RunFunctionResponse{
					Meta: &v1beta1.ResponseMeta{Tag: "hi!"},
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
	// TestRunFunction exercises most of the getClientConn code. Here we just
	// test some cases that don't fit well in our usual table-driven format.

	// Start a gRPC server.
	lis := NewGRPCServer(t, &MockFunctionServer{rsp: &v1beta1.RunFunctionResponse{
		Meta: &v1beta1.ResponseMeta{Tag: "hi!"},
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
	lis2 := NewGRPCServer(t, &MockFunctionServer{rsp: &v1beta1.RunFunctionResponse{
		Meta: &v1beta1.ResponseMeta{Tag: "hi!"},
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

func TestGarbageCollectConnectionsNow(t *testing.T) {
	// TestRunFunction exercises most of the GarbageCollectConnectionsNow code.
	// Here we just test some cases that don't fit well in our usual
	// table-driven format.

	// Start a gRPC server.
	lis := NewGRPCServer(t, &MockFunctionServer{rsp: &v1beta1.RunFunctionResponse{
		Meta: &v1beta1.ResponseMeta{Tag: "hi!"},
	}})
	defer lis.Close()

	target := strings.Replace(lis.Addr().String(), "127.0.0.1", "dns:///localhost", 1)

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("gRPC dial failed: %s", err)
	}

	c := &test.MockClient{}
	r := NewPackagedFunctionRunner(c)

	// Add our connection to our pool.
	r.connsMx.Lock()
	r.conns["cool-fn"] = conn
	r.connsMx.Unlock()

	t.Run("FunctionStillExistsDoNotGarbageCollect", func(t *testing.T) {
		c.MockList = test.NewMockListFn(nil, func(obj client.ObjectList) error {
			obj.(*pkgv1beta1.FunctionList).Items = []pkgv1beta1.Function{
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
		l, ok := obj.(*pkgv1beta1.FunctionRevisionList)
		if !ok {
			// If we're called to list Functions we want to
			// return none, to make sure we GC everything.
			return nil
		}
		l.Items = []pkgv1beta1.FunctionRevision{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cool-fn-revision-a",
				},
				Spec: pkgv1beta1.FunctionRevisionSpec{
					PackageRevisionSpec: pkgv1.PackageRevisionSpec{
						DesiredState: pkgv1.PackageRevisionActive,
					},
				},
				Status: pkgv1beta1.FunctionRevisionStatus{
					Endpoint: target,
				},
			},
		}
		return nil
	})
}

func NewGRPCServer(t *testing.T, ss v1beta1.FunctionRunnerServiceServer) net.Listener {
	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Listening for gRPC connections on %q", lis.Addr().String())

	// TODO(negz): Is it worth using a WaitGroup for these?
	go func() {
		s := grpc.NewServer()
		v1beta1.RegisterFunctionRunnerServiceServer(s, ss)
		_ = s.Serve(lis)
	}()

	// The caller must close this listener to terminate the server.
	return lis
}

type MockFunctionServer struct {
	v1beta1.UnimplementedFunctionRunnerServiceServer

	rsp *v1beta1.RunFunctionResponse
	err error
}

func (s *MockFunctionServer) RunFunction(context.Context, *v1beta1.RunFunctionRequest) (*v1beta1.RunFunctionResponse, error) {
	return s.rsp, s.err
}

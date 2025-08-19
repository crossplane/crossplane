/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package cached

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/xfn/cached/proto/v1alpha1"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

type FunctionRunnerFn func(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)

func (fn FunctionRunnerFn) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	return fn(ctx, name, req)
}

var MockDir = []byte("DIR")

func MockFs(files map[string][]byte) afero.Fs {
	fs := afero.NewMemMapFs()

	for path, data := range files {
		// Special value for making a directory.
		if bytes.Equal(data, MockDir) {
			if err := fs.MkdirAll(path, 0o700); err != nil {
				panic(err)
			}

			continue
		}

		if err := afero.WriteFile(fs, path, data, 0o600); err != nil {
			panic(err)
		}
	}

	return fs
}

var _ logging.Logger = &TestLogger{}

type TestLogger struct {
	t   *testing.T
	kvs []any
}

func (l *TestLogger) Info(msg string, keysAndValues ...any) {
	l.t.Logf("INFO: %s (%s)", msg, append(l.kvs, keysAndValues...))
}

func (l *TestLogger) Debug(msg string, keysAndValues ...any) {
	l.t.Logf("DEBUG: %s (%s)", msg, append(l.kvs, keysAndValues...))
}

func (l *TestLogger) WithValues(keysAndValues ...any) logging.Logger {
	l.kvs = append(l.kvs, keysAndValues...)
	return l
}

func TestRunFunction(t *testing.T) {
	type params struct {
		wrap FunctionRunner
		path string
		o    []FileBackedRunnerOption
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
		"MissingTag": {
			reason: "If the request is missing a tag we should call the wrapped runner without caching.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{Tag: "hi"},
					}
					return rsp, nil
				}),
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					WithFilesystem(afero.NewMemMapFs()),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hi"},
				},
			},
		},
		"NotCached": {
			reason: "If the response isn't cached we should call the wrapped runner and cache it.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{
							Tag: "hello",
							Ttl: durationpb.New(10 * time.Minute),
						},
					}
					return rsp, nil
				}),
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					WithFilesystem(afero.NewMemMapFs()),
				},
			},
			args: args{
				name: "coolfn",
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{
						Tag: "hello",
						Ttl: durationpb.New(10 * time.Minute),
					},
				},
			},
		},
		"WrappedError": {
			reason: "If the response isn't cached and the wrapped runner returns an error we should return it.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return nil, errors.New("boom")
				}),
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					WithFilesystem(afero.NewMemMapFs()),
				},
			},
			args: args{
				name: "coolfn",
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"EmptyFile": {
			reason: "If the cached response is empty should call the wrapped runner and cache it.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{
							Tag: "hello",
							Ttl: durationpb.New(10 * time.Minute),
						},
					}
					return rsp, nil
				}),
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					WithFilesystem(MockFs(map[string][]byte{
						"coolfn/hello": {},
					})),
				},
			},
			args: args{
				name: "coolfn",
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{
						Tag: "hello",
						Ttl: durationpb.New(10 * time.Minute),
					},
				},
			},
		},
		"UnexpectedFile": {
			reason: "If the cached response contains unexpected data we should call the wrapped runner and cache it.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{
							Tag: "hello",
							Ttl: durationpb.New(10 * time.Minute),
						},
					}
					return rsp, nil
				}),
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					WithFilesystem(MockFs(map[string][]byte{
						"coolfn/hello": []byte("Hello dearest caller. I'm not a deadline followed by an encoded RunFunctionResponse."),
					})),
				},
			},
			args: args{
				name: "coolfn",
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{
						Tag: "hello",
						Ttl: durationpb.New(10 * time.Minute),
					},
				},
			},
		},
		"DeadlineExceeded": {
			reason: "If the cached response's deadline has passed we should call the wrapped runner and cache it.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{
							Tag: "hello",
							Ttl: durationpb.New(10 * time.Minute),
						},
					}
					return rsp, nil
				}),
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					WithFilesystem(MockFs(map[string][]byte{
						"coolfn/hello": func() []byte {
							msg, _ := proto.Marshal(&v1alpha1.CachedRunFunctionResponse{
								// In the past.
								Deadline: timestamppb.New(time.Now().Add(-1 * time.Minute)),
								Response: &fnv1.RunFunctionResponse{
									Meta: &fnv1.ResponseMeta{
										Tag: "exceeded",
										Ttl: durationpb.New(10 * time.Minute),
									},
								},
							})

							return msg
						}(),
					})),
				},
			},
			args: args{
				name: "coolfn",
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{
						Tag: "hello",
						Ttl: durationpb.New(10 * time.Minute),
					},
				},
			},
		},
		"CacheHit": {
			reason: "If the cached response is still valid, return it without calling the wrapped runner.",
			params: params{
				// Wrap is nil. It'd panic if called.
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					WithFilesystem(MockFs(map[string][]byte{
						"coolfn/hello": func() []byte {
							msg, _ := proto.Marshal(&v1alpha1.CachedRunFunctionResponse{
								Deadline: timestamppb.New(time.Now().Add(1 * time.Minute)),
								Response: &fnv1.RunFunctionResponse{
									Meta: &fnv1.ResponseMeta{
										Tag: "hello",
										Ttl: durationpb.New(10 * time.Minute),
									},
								},
							})

							return msg
						}(),
					})),
				},
			},
			args: args{
				name: "coolfn",
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{
						Tag: "hello",
						Ttl: durationpb.New(10 * time.Minute),
					},
				},
			},
		},
		"MaxTTLClamping": {
			reason: "If the response TTL exceeds the maximum TTL, it should be clamped to the max.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					rsp := &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{
							Tag: "hello",
							// Request 60 minutes, but maxTTL will clamp to 30.
							Ttl: durationpb.New(60 * time.Minute),
						},
					}
					return rsp, nil
				}),
				o: []FileBackedRunnerOption{
					WithLogger(&TestLogger{t: t}),
					// WithFilesystem(afero.NewMemMapFs()),
					WithFilesystem(MockFs(map[string][]byte{
						"coolfn/hello": func() []byte {
							msg, _ := proto.Marshal(&v1alpha1.CachedRunFunctionResponse{
								Deadline: timestamppb.New(time.Now().Add(45 * time.Minute)),
								Response: &fnv1.RunFunctionResponse{
									Meta: &fnv1.ResponseMeta{
										Tag: "clamped",
										Ttl: durationpb.New(30 * time.Minute),
									},
								},
							})

							return msg
						}(),
					})),
					WithMaxTTL(7 * time.Minute),
				},
			},
			args: args{
				name: "coolfn",
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{
						Tag: "clamped",
						Ttl: durationpb.New(30 * time.Minute),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewFileBackedRunner(tc.params.wrap, tc.params.path, tc.params.o...)
			rsp, err := r.RunFunction(tc.args.ctx, tc.args.name, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

// We test through to CacheFunction a lot in TestRunFunction. This is just a
// little sanity check to make sure we can actually read back what we write.
func TestCacheFunction(t *testing.T) {
	rsp := &fnv1.RunFunctionResponse{
		Meta: &fnv1.ResponseMeta{
			Tag: "wrapped",
			Ttl: durationpb.New(1 * time.Minute),
		},
	}

	wrapped := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		return rsp, nil
	})

	fs := afero.NewMemMapFs()

	r := NewFileBackedRunner(wrapped, "/cache",
		WithLogger(&TestLogger{t: t}),
		WithFilesystem(fs))

	// Populate the cache.
	got, err := r.CacheFunction(context.TODO(), "coolfn", &fnv1.RunFunctionRequest{Meta: &fnv1.RequestMeta{Tag: "req"}})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(rsp, got, protocmp.Transform()); diff != "" {
		t.Errorf("\nr.RunFunction(...): -want rsp, +got rsp:\n%s", diff)
	}

	// Create a new runner backed by the same populated cache, but with a
	// nil wrapped runner that would panic if called.
	r = NewFileBackedRunner(nil, "/cache",
		WithLogger(&TestLogger{t: t}),
		WithFilesystem(fs))

	// Make sure we can read back what we just wrote.
	got, err = r.RunFunction(context.TODO(), "coolfn", &fnv1.RunFunctionRequest{Meta: &fnv1.RequestMeta{Tag: "req"}})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(rsp, got, protocmp.Transform()); diff != "" {
		t.Errorf("\nr.RunFunction(...): -want rsp, +got rsp:\n%s", diff)
	}
}

// Make sure the Global TTL is used when a Function Response TTL is greater.
func TestCacheFunctionWithMaxTTL(t *testing.T) {
	maxTTL := 5 * time.Minute
	requestedTTL := 30 * time.Minute

	rsp := &fnv1.RunFunctionResponse{
		Meta: &fnv1.ResponseMeta{
			Tag: "wrapped",
			Ttl: durationpb.New(requestedTTL),
		},
	}

	wrapped := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		return rsp, nil
	})

	fs := afero.NewMemMapFs()

	r := NewFileBackedRunner(wrapped, "/cache",
		WithLogger(&TestLogger{t: t}),
		WithFilesystem(fs),
		WithMaxTTL(maxTTL))

	startTime := time.Now().UTC()

	// Populate the cache with a response that has a TTL greater than maxTTL.
	got, err := r.CacheFunction(context.TODO(), "coolfn", &fnv1.RunFunctionRequest{Meta: &fnv1.RequestMeta{Tag: "req"}})
	if err != nil {
		t.Fatal(err)
	}

	// The response should be returned unchanged.
	if diff := cmp.Diff(rsp, got, protocmp.Transform()); diff != "" {
		t.Errorf("\nr.CacheFunction(...): -want rsp, +got rsp:\n%s", diff)
	}

	// Read the cached file directly to verify the deadline was clamped.
	b, err := r.fs.ReadFile("coolfn/req")
	if err != nil {
		t.Fatal(err)
	}

	crsp := &v1alpha1.CachedRunFunctionResponse{}
	if err := proto.Unmarshal(b, crsp); err != nil {
		t.Fatal(err)
	}

	deadline := crsp.GetDeadline().AsTime()
	expectedDeadline := startTime.Add(maxTTL)

	// Allow 1 second tolerance for timing differences.
	if deadline.After(expectedDeadline.Add(1*time.Second)) || deadline.Before(expectedDeadline.Add(-1*time.Second)) {
		t.Errorf("Expected deadline to be clamped to maxTTL. Got %v, expected around %v", deadline, expectedDeadline)
	}
}

func TestGarbageCollectFilesNow(t *testing.T) {
	// Deadline in the past.
	past, _ := proto.Marshal(&v1alpha1.CachedRunFunctionResponse{Deadline: timestamppb.New(time.Now().Add(-1 * time.Minute))})

	// Deadline in the future.
	future, _ := proto.Marshal(&v1alpha1.CachedRunFunctionResponse{Deadline: timestamppb.New(time.Now().Add(1 * time.Minute))})

	fs := MockFs(map[string][]byte{
		"/":                         MockDir,
		"/coolfn":                   MockDir,
		"/coolfn/expired":           past,
		"/coolfn/valid":             future,
		"/coolfn/alsovalid":         future,
		"/prettycoolfn":             MockDir,
		"/prettycoolfn/expired":     past,
		"/prettycoolfn/alsoexpired": past,
	})

	r := NewFileBackedRunner(nil, "/cache",
		WithLogger(&TestLogger{t: t}),
		WithFilesystem(fs))

	collected, err := r.GarbageCollectFilesNow(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	want := 3
	if diff := cmp.Diff(want, collected); diff != "" {
		t.Errorf("\nr.GarbageCollectFilesNow(...): -want collected, +got collected:\n%s", diff)
	}
}

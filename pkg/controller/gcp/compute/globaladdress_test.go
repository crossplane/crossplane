/*
Copyright 2019 The Crossplane Authors.

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

package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"

	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
)

var (
	unexpected resource.Managed
)

var (
	errBoom           = errors.New("boom")
	errGoogleNotFound = &googleapi.Error{Code: http.StatusNotFound, Message: "boom"}
	errGoogleConflict = &googleapi.Error{Code: http.StatusConflict, Message: "boom"}
	errGoogleOther    = &googleapi.Error{Code: http.StatusInternalServerError, Message: "boom"}
)

func globalAddress() *v1alpha1.GlobalAddress {
	return &v1alpha1.GlobalAddress{
		Spec: v1alpha1.GlobalAddressSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{ProviderReference: &corev1.ObjectReference{}},
		},
	}
}

func TestGlobalAddressConnect(t *testing.T) {
	var service *compute.Service

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	cases := map[string]struct {
		ec   resource.ExternalConnecter
		args args
		want error
	}{
		"NotGlobalAddressError": {
			ec: &gaConnector{},
			args: args{
				ctx: context.Background(),
				mg:  unexpected,
			},
			want: errors.New(errNotGlobalAddress),
		},
		"GetProviderError": {
			ec: &gaConnector{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						switch obj.(type) {
						case *gcpv1alpha1.Provider:
							return errBoom
						case *corev1.Secret:
						default:
							return errors.New("unexpected resource kind")
						}
						return nil
					}),
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: errors.Wrap(errBoom, errProviderNotRetrieved),
		},
		"GetProviderSecretError": {
			ec: &gaConnector{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						switch obj.(type) {
						case *gcpv1alpha1.Provider:
						case *corev1.Secret:
							return errBoom
						default:
							return errors.New("unexpected resource kind")
						}
						return nil
					}),
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: errors.Wrap(errBoom, errProviderSecretNotRetrieved),
		},
		"GetComputeServiceError": {
			ec: &gaConnector{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						switch obj.(type) {
						case *gcpv1alpha1.Provider:
						case *corev1.Secret:
						default:
							return errors.New("unexpected resource kind")
						}
						return nil
					}),
				},
				newCompute: func(_ context.Context, _ ...option.ClientOption) (*compute.Service, error) { return nil, errBoom },
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: errors.Wrap(errBoom, errNewClient),
		},
		"Successful": {
			ec: &gaConnector{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
						switch obj.(type) {
						case *gcpv1alpha1.Provider:
						case *corev1.Secret:
						default:
							return errors.Errorf("unexpected resource kind %T", obj)
						}
						return nil
					}),
				},
				newCompute: func(_ context.Context, _ ...option.ClientOption) (*compute.Service, error) { return service, nil },
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.ec.Connect(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("Connect(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestGlobalAddressObserve(t *testing.T) {

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		eo  resource.ExternalObservation
		err error
	}

	cases := map[string]struct {
		e    resource.ExternalClient
		args args
		want want
	}{
		"NotGlobalAddressError": {
			e: &gaExternal{},
			args: args{
				ctx: context.Background(),
				mg:  unexpected,
			},
			want: want{
				err: errors.New(errNotGlobalAddress),
			},
		},
		"ErrorGetAddress": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodGet, ReturnError: errGoogleOther}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: want{
				err: errors.Wrap(errGoogleOther, errGetAddress),
			},
		},
		"ErrorUpdateGlobalAddress": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodGet}.Serve(),
				client:  &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: want{
				eo: resource.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
				err: errors.Wrap(errBoom, errUpdateManaged),
			},
		},
		"AddressDoesNotExist": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodGet, ReturnError: errGoogleNotFound}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: want{
				eo: resource.ExternalObservation{
					ResourceExists: false,
				},
			},
		},
		"AddressExists": {
			e: &gaExternal{
				client: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
						if _, ok := obj.(*v1alpha1.GlobalAddress); !ok {
							return errors.Errorf("unexpected resource kind %T", obj)
						}
						return nil
					}),
				},
				compute: FakeComputeService{WantMethod: http.MethodGet}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: want{
				eo: resource.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.e.Observe(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.eo, got); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Observe(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestGlobalAddressCreate(t *testing.T) {

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		ec  resource.ExternalCreation
		err error
	}

	cases := map[string]struct {
		e    resource.ExternalClient
		args args
		want want
	}{
		"NotGlobalAddressError": {
			e: &gaExternal{},
			args: args{
				ctx: context.Background(),
				mg:  unexpected,
			},
			want: want{
				err: errors.New(errNotGlobalAddress),
			},
		},
		"ErrorInsertAddress": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodPost, ReturnError: errGoogleOther}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: want{
				err: errors.Wrap(errGoogleOther, errCreateAddress),
			},
		},
		"AddressAlreadyExists": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodPost, ReturnError: errGoogleConflict}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
		},
		"AddressInserted": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodPost}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.e.Create(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.ec, got); diff != "" {
				t.Errorf("Create(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Create(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestGlobalAddressDelete(t *testing.T) {

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	cases := map[string]struct {
		e    resource.ExternalClient
		args args
		want error
	}{
		"NotGlobalAddressError": {
			e: &gaExternal{},
			args: args{
				ctx: context.Background(),
				mg:  unexpected,
			},
			want: errors.New(errNotGlobalAddress),
		},
		"ErrorDeleteAddress": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodDelete, ReturnError: errGoogleOther}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
			want: errors.Wrap(errGoogleOther, errDeleteAddress),
		},
		"AddressDoesNotExist": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodDelete, ReturnError: errGoogleNotFound}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
		},
		"AddressDeleted": {
			e: &gaExternal{
				compute: FakeComputeService{WantMethod: http.MethodDelete}.Serve(),
			},
			args: args{
				ctx: context.Background(),
				mg:  globalAddress(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.e.Delete(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("Delete(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

type FakeComputeService struct {
	WantMethod  string
	ReturnError error
}

func (s FakeComputeService) Serve() *compute.Service {
	// NOTE(negz): We never close this httptest.Server because returning only a
	// compute.Service makes for a simpler test fake API. We create one server
	// per test case, but they only live for the invocation of the test run.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Body.Close()

		if r.Method != s.WantMethod {
			http.Error(w, fmt.Sprintf("want HTTP method %s, got %s", s.WantMethod, r.Method), http.StatusBadRequest)
			return
		}

		if gae, ok := s.ReturnError.(*googleapi.Error); ok {
			w.WriteHeader(gae.Code)
			_ = json.NewEncoder(w).Encode(struct {
				Error *googleapi.Error `json:"error"`
			}{Error: gae})
			return
		}

		if s.ReturnError != nil {
			http.Error(w, s.ReturnError.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(&compute.Operation{})
	}))

	c, _ := compute.NewService(context.Background(), option.WithEndpoint(srv.URL))
	return c
}

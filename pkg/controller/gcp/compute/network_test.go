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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
)

const (
	testNetworkName        = "test-network"
	testNetworkDescription = "this is my test network!"
	testGoogleProjectID    = "test-project-id"
	testProviderName       = "test-provider"
	testNamespace          = "test-namespace"
)

const testGCPCredentialsJSON = `
{
  "type": "service_account",
  "project_id": "fake-project",
  "private_key_id": "fake-id",
  "private_key": "-----BEGIN PRIVATE KEY-----\nIAMAFAKEPRIVATEKEY-----END PRIVATE KEY-----\n",
  "client_email": "crossplane-test@fake-project.iam.gserviceaccount.com",
  "client_id": "123456789",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/crossplane-test%40crossplane-test.iam.gserviceaccount.com"
}

`

func TestNetworkConnector_Connect(t *testing.T) {
	type args struct {
		cr resource.Managed
		c  *networkConnector
		ns func(ctx context.Context, opts ...option.ClientOption) (*compute.Service, error)
	}

	fakeErr := errors.New("i reject to work")
	testProvider := &gcpv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
		},
		Spec: gcpv1alpha1.ProviderSpec{
			Secret: v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "test-secret-name",
				},
				Key: "test-key",
			},
		},
	}
	testSecret := &v1.Secret{
		Data: map[string][]byte{
			testProvider.Spec.Secret.Key: []byte(testGCPCredentialsJSON),
		},
	}

	cases := map[string]struct {
		args args
		err  error
	}{
		"Successful": {
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ProviderReference: &v1.ObjectReference{
								Name:      testProviderName,
								Namespace: testNamespace,
							},
						},
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name: testNetworkName,
						},
					},
				},
				c: &networkConnector{
					kube: &test.MockClient{
						MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
							switch o := obj.(type) {
							case *gcpv1alpha1.Provider:
								if diff := cmp.Diff(
									meta.NamespacedNameOf(&v1.ObjectReference{Name: testProviderName, Namespace: testNamespace}),
									key); diff != "" {
									t.Errorf("r: -want, +got:\n%s", diff)
								}
								testProvider.DeepCopyInto(o)
								return nil
							case *v1.Secret:
								testSecret.DeepCopyInto(o)
								return nil
							}
							return nil
						},
					},
				},
				ns: func(_ context.Context, _ ...option.ClientOption) (*compute.Service, error) {
					return nil, nil
				},
			},
		},
		"UnnamedNetworkResource": {
			args: args{
				cr: &v1alpha1.Network{},
				c:  &networkConnector{},
			},
			err: errors.New(errInsufficientNetworkSpec),
		},
		"ProviderRetrievalFailed": {
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name: testNetworkName,
						},
						ResourceSpec: corev1alpha1.ResourceSpec{
							ProviderReference: &v1.ObjectReference{
								Name:      testProviderName,
								Namespace: testNamespace,
							},
						},
					},
				},
				c: &networkConnector{
					kube: &test.MockClient{
						MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
							return fakeErr
						},
					},
				},
			},
			err: errors.Wrap(fakeErr, errProviderNotRetrieved),
		},
		"CredFromSecretRetrievalFailed": {
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ProviderReference: &v1.ObjectReference{
								Name:      testProviderName,
								Namespace: testNamespace,
							},
						},
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name: testNetworkName,
						},
					},
				},
				c: &networkConnector{
					kube: &test.MockClient{
						MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
							switch o := obj.(type) {
							case *gcpv1alpha1.Provider:
								if diff := cmp.Diff(
									meta.NamespacedNameOf(&v1.ObjectReference{Name: testProviderName, Namespace: testNamespace}),
									key); diff != "" {
									t.Errorf("r: -want, +got:\n%s", diff)
								}
								testProvider.DeepCopyInto(o)
								return nil
							case *v1.Secret:
								return fakeErr
							}
							return nil
						},
					},
				},
			},
			err: errors.Wrap(fakeErr, errProviderSecretNotRetrieved),
		},
		"NewServiceFailed": {
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						ResourceSpec: corev1alpha1.ResourceSpec{
							ProviderReference: &v1.ObjectReference{
								Name:      testProviderName,
								Namespace: testNamespace,
							},
						},
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name: testNetworkName,
						},
					},
				},
				c: &networkConnector{
					kube: &test.MockClient{
						MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
							switch o := obj.(type) {
							case *gcpv1alpha1.Provider:
								if diff := cmp.Diff(
									meta.NamespacedNameOf(&v1.ObjectReference{Name: testProviderName, Namespace: testNamespace}),
									key); diff != "" {
									t.Errorf("r: -want, +got:\n%s", diff)
								}
								testProvider.DeepCopyInto(o)
								return nil
							case *v1.Secret:
								testSecret.DeepCopyInto(o)
								return nil
							}
							return nil
						},
					},
				},
				ns: func(_ context.Context, _ ...option.ClientOption) (*compute.Service, error) {
					return nil, fakeErr
				},
			},
			err: errors.Wrap(fakeErr, errNewClient),
		},
		"DifferentType": {
			args: args{
				cr: &v1alpha1.Subnetwork{},
				c:  &networkConnector{},
			},
			err: errors.New(errNotNetwork),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.args.c.newServiceFn = tc.args.ns
			_, err := tc.args.c.Connect(context.Background(), tc.args.cr)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNetworkExternal_Observe(t *testing.T) {
	type args struct {
		cr resource.Managed
	}

	cases := map[string]struct {
		handler http.Handler
		args    args
		error   bool
		obs     resource.ExternalObservation
	}{
		"SuccessfulExists": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("GET", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name: testNetworkName,
						},
					},
				},
			},
			obs: resource.ExternalObservation{ResourceExists: true},
		},
		"SuccessfulDoesNotExist": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("GET", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name: testNetworkName,
						},
					},
				},
			},
		},
		"Failed": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("GET", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{},
			},
			error: true,
		},
		"DifferentType": {
			args: args{
				cr: &v1alpha1.Subnetwork{},
			},
			error: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			s, _ := compute.NewService(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
			e := networkExternal{
				projectID: testGoogleProjectID,
				Service:   s,
			}
			obs, err := e.Observe(context.Background(), tc.args.cr)
			if diff := cmp.Diff(tc.error, err != nil); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.obs, obs); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNetworkExternal_Create(t *testing.T) {
	type args struct {
		cr resource.Managed
	}

	trueVal := true
	cases := map[string]struct {
		handler http.Handler
		args    args
		error   bool
	}{
		"Successful": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("POST", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name:                  testNetworkName,
							AutoCreateSubnetworks: &trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "REGIONAL",
							},
						},
					},
				},
			},
		},
		"Failed": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("POST", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{},
			},
			error: true,
		},
		"AlreadyExists": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("POST", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{},
			},
		},
		"DifferentType": {
			args: args{
				cr: &v1alpha1.Subnetwork{},
			},
			error: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			s, _ := compute.NewService(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
			e := networkExternal{
				projectID: testGoogleProjectID,
				Service:   s,
			}
			_, err := e.Create(context.Background(), tc.args.cr)
			if diff := cmp.Diff(tc.error, err != nil); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNetworkExternal_Update(t *testing.T) {
	type args struct {
		cr resource.Managed
	}

	trueVal := true
	cases := map[string]struct {
		handler http.Handler
		args    args
		error   bool
	}{
		"Successful": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("PATCH", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name:                  testNetworkName,
							AutoCreateSubnetworks: &trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "REGIONAL",
							},
						},
					},
					Status: v1alpha1.NetworkStatus{
						GCPNetworkStatus: v1alpha1.GCPNetworkStatus{
							AutoCreateSubnetworks: trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "GLOBAL",
							},
						},
					},
				},
			},
		},
		"Failed": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("PATCH", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name:                  testNetworkName,
							AutoCreateSubnetworks: &trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "REGIONAL",
							},
						},
					},
					Status: v1alpha1.NetworkStatus{
						GCPNetworkStatus: v1alpha1.GCPNetworkStatus{
							AutoCreateSubnetworks: trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "GLOBAL",
							},
						},
					},
				},
			},
			error: true,
		},
		"Unnecessary": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				t.Errorf("patch request should have been skipped")
			}),
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name:                  testNetworkName,
							AutoCreateSubnetworks: &trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "REGIONAL",
							},
						},
					},
					Status: v1alpha1.NetworkStatus{
						GCPNetworkStatus: v1alpha1.GCPNetworkStatus{
							AutoCreateSubnetworks: trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "REGIONAL",
							},
						},
					},
				},
			},
		},
		"DifferentType": {
			args: args{
				cr: &v1alpha1.Subnetwork{},
			},
			error: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			s, _ := compute.NewService(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
			e := networkExternal{
				projectID: testGoogleProjectID,
				Service:   s,
			}
			_, err := e.Update(context.Background(), tc.args.cr)
			if diff := cmp.Diff(tc.error, err != nil); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNetworkExternal_Delete(t *testing.T) {
	type args struct {
		cr resource.Managed
	}

	trueVal := true
	cases := map[string]struct {
		handler http.Handler
		args    args
		error   bool
	}{
		"Successful": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("DELETE", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{
					Spec: v1alpha1.NetworkSpec{
						GCPNetworkSpec: v1alpha1.GCPNetworkSpec{
							Name:                  testNetworkName,
							AutoCreateSubnetworks: &trueVal,
							Description:           testNetworkDescription,
							RoutingConfig: &v1alpha1.GCPNetworkRoutingConfig{
								RoutingMode: "REGIONAL",
							},
						},
					},
				},
			},
		},
		"Failed": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("DELETE", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{},
			},
			error: true,
		},
		"NotFound": {
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.Body.Close()
				if diff := cmp.Diff("DELETE", r.Method); diff != "" {
					t.Errorf("r: -want, +got:\n%s", diff)
				}
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(&compute.Operation{})
			}),
			args: args{
				cr: &v1alpha1.Network{},
			},
		},
		"DifferentType": {
			args: args{
				cr: &v1alpha1.Subnetwork{},
			},
			error: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			s, _ := compute.NewService(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
			e := networkExternal{
				projectID: testGoogleProjectID,
				Service:   s,
			}
			err := e.Delete(context.Background(), tc.args.cr)
			if diff := cmp.Diff(tc.error, err != nil); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

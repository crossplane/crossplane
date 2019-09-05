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

package virtualnetwork

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane/azure/apis/network/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	networkclient "github.com/crossplaneio/crossplane/pkg/clients/azure/network"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/network/fake"
)

const (
	namespace         = "coolNamespace"
	name              = "coolSubnet"
	uid               = types.UID("definitely-a-uuid")
	addressPrefix     = "10.0.0.0/16"
	resourceGroupName = "coolRG"
	location          = "coolplace"

	providerName       = "cool-aws"
	providerSecretName = "cool-aws-secret"
	providerSecretKey  = "credentials"
	providerSecretData = "definitelyini"
)

var (
	ctx       = context.Background()
	errorBoom = errors.New("boom")
	tags      = map[string]string{"one": "test", "two": "test"}

	provider = azurev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
		Spec: azurev1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
				Key:                  providerSecretKey,
			},
		},
	}

	providerSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}
)

type testCase struct {
	name    string
	e       resource.ExternalClient
	r       resource.Managed
	want    resource.Managed
	wantErr error
}

type virtualNetworkModifier func(*v1alpha1.VirtualNetwork)

func withConditions(c ...runtimev1alpha1.Condition) virtualNetworkModifier {
	return func(r *v1alpha1.VirtualNetwork) { r.Status.ConditionedStatus.Conditions = c }
}

func withState(s string) virtualNetworkModifier {
	return func(r *v1alpha1.VirtualNetwork) { r.Status.State = s }
}

func virtualNetwork(vm ...virtualNetworkModifier) *v1alpha1.VirtualNetwork {
	r := &v1alpha1.VirtualNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.VirtualNetworkSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference: &corev1.ObjectReference{Namespace: namespace, Name: providerName},
			},
			Name:              name,
			ResourceGroupName: resourceGroupName,
			VirtualNetworkPropertiesFormat: v1alpha1.VirtualNetworkPropertiesFormat{
				AddressSpace: v1alpha1.AddressSpace{
					AddressPrefixes: []string{addressPrefix},
				},
				EnableDDOSProtection: true,
				EnableVMProtection:   true,
			},
			Location: location,
			Tags:     tags,
		},
		Status: v1alpha1.VirtualNetworkStatus{},
	}

	for _, m := range vm {
		m(r)
	}

	return r
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ resource.ExternalClient = &external{}
var _ resource.ExternalConnecter = &connecter{}

func TestCreate(t *testing.T) {
	cases := []testCase{
		{
			name:    "NotVirtualNetwok",
			e:       &external{client: &fake.MockVirtualNetworksClient{}},
			r:       &v1alpha1.Subnet{},
			want:    &v1alpha1.Subnet{},
			wantErr: errors.New(errNotVirtualNetwork),
		},
		{
			name: "SuccessfulCreate",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ network.VirtualNetwork) (result network.VirtualNetworksCreateOrUpdateFuture, err error) {
					return network.VirtualNetworksCreateOrUpdateFuture{}, nil
				},
			}},
			r: virtualNetwork(),
			want: virtualNetwork(
				withConditions(runtimev1alpha1.Creating()),
			),
		},
		{
			name: "FailedCreate",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ network.VirtualNetwork) (result network.VirtualNetworksCreateOrUpdateFuture, err error) {
					return network.VirtualNetworksCreateOrUpdateFuture{}, errorBoom
				},
			}},
			r: virtualNetwork(),
			want: virtualNetwork(
				withConditions(runtimev1alpha1.Creating()),
			),
			wantErr: errors.Wrap(errorBoom, errCreateVirtualNetwork),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.e.Create(ctx, tc.r)

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.e.Create(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestObserve(t *testing.T) {
	cases := []testCase{
		{
			name:    "NotVirtualNetwok",
			e:       &external{client: &fake.MockVirtualNetworksClient{}},
			r:       &v1alpha1.Subnet{},
			want:    &v1alpha1.Subnet{},
			wantErr: errors.New(errNotVirtualNetwork),
		},
		{
			name: "SuccessfulObserveNotExist",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string) (result network.VirtualNetwork, err error) {
					return network.VirtualNetwork{
							Tags: azure.ToStringPtrMap(tags),
							VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
								AddressSpace: &network.AddressSpace{
									AddressPrefixes: &[]string{addressPrefix},
								},
								EnableDdosProtection: azure.ToBoolPtr(true),
								EnableVMProtection:   azure.ToBoolPtr(true),
							},
						}, autorest.DetailedError{
							StatusCode: http.StatusNotFound,
						}
				},
			}},
			r:    virtualNetwork(),
			want: virtualNetwork(),
		},
		{
			name: "SuccessfulObserveExists",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string) (result network.VirtualNetwork, err error) {
					return network.VirtualNetwork{
						Tags: azure.ToStringPtrMap(tags),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: &[]string{addressPrefix},
							},
							EnableDdosProtection: azure.ToBoolPtr(true),
							EnableVMProtection:   azure.ToBoolPtr(true),
							ProvisioningState:    azure.ToStringPtr(string(network.Available)),
						},
					}, nil
				},
			}},
			r: virtualNetwork(),
			want: virtualNetwork(
				withConditions(runtimev1alpha1.Available()),
				withState(string(network.Available)),
			),
		},
		{
			name: "FailedObserve",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string) (result network.VirtualNetwork, err error) {
					return network.VirtualNetwork{}, errorBoom
				},
			}},
			r:       virtualNetwork(),
			want:    virtualNetwork(),
			wantErr: errors.Wrap(errorBoom, errGetVirtualNetwork),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.e.Observe(ctx, tc.r)

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.e.Observe(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	cases := []testCase{
		{
			name:    "NotVirtualNetwok",
			e:       &external{client: &fake.MockVirtualNetworksClient{}},
			r:       &v1alpha1.Subnet{},
			want:    &v1alpha1.Subnet{},
			wantErr: errors.New(errNotVirtualNetwork),
		},
		{
			name: "SuccessfulDoesNotNeedUpdate",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string) (result network.VirtualNetwork, err error) {
					return network.VirtualNetwork{
						Tags: azure.ToStringPtrMap(tags),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: &[]string{addressPrefix},
							},
							EnableDdosProtection: azure.ToBoolPtr(true),
							EnableVMProtection:   azure.ToBoolPtr(true),
						},
					}, nil
				},
			}},
			r:    virtualNetwork(),
			want: virtualNetwork(),
		},
		{
			name: "SuccessfulNeedsUpdate",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string) (result network.VirtualNetwork, err error) {
					return network.VirtualNetwork{
						Tags: azure.ToStringPtrMap(tags),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: &[]string{"10.1.0.0/16"},
							},
							EnableDdosProtection: azure.ToBoolPtr(true),
							EnableVMProtection:   azure.ToBoolPtr(true),
						},
					}, nil
				},
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ network.VirtualNetwork) (result network.VirtualNetworksCreateOrUpdateFuture, err error) {
					return network.VirtualNetworksCreateOrUpdateFuture{}, nil
				},
			}},
			r:    virtualNetwork(),
			want: virtualNetwork(),
		},
		{
			name: "UnsuccessfulGet",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string) (result network.VirtualNetwork, err error) {
					return network.VirtualNetwork{
						Tags: azure.ToStringPtrMap(tags),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: &[]string{"10.1.0.0/16"},
							},
							EnableDdosProtection: azure.ToBoolPtr(true),
							EnableVMProtection:   azure.ToBoolPtr(true),
						},
					}, errorBoom
				},
			}},
			r:       virtualNetwork(),
			want:    virtualNetwork(),
			wantErr: errors.Wrap(errorBoom, errGetVirtualNetwork),
		},
		{
			name: "UnsuccessfulUpdate",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string) (result network.VirtualNetwork, err error) {
					return network.VirtualNetwork{
						Tags: azure.ToStringPtrMap(tags),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: &[]string{"10.1.0.0/16"},
							},
							EnableDdosProtection: azure.ToBoolPtr(true),
							EnableVMProtection:   azure.ToBoolPtr(true),
						},
					}, nil
				},
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ network.VirtualNetwork) (result network.VirtualNetworksCreateOrUpdateFuture, err error) {
					return network.VirtualNetworksCreateOrUpdateFuture{}, errorBoom
				},
			}},
			r:       virtualNetwork(),
			want:    virtualNetwork(),
			wantErr: errors.Wrap(errorBoom, errUpdateVirtualNetwork),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.e.Update(ctx, tc.r)

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.e.Update(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []testCase{
		{
			name:    "NotVirtualNetwok",
			e:       &external{client: &fake.MockVirtualNetworksClient{}},
			r:       &v1alpha1.Subnet{},
			want:    &v1alpha1.Subnet{},
			wantErr: errors.New(errNotVirtualNetwork),
		},
		{
			name: "Successful",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockDelete: func(_ context.Context, _ string, _ string) (result network.VirtualNetworksDeleteFuture, err error) {
					return network.VirtualNetworksDeleteFuture{}, nil
				},
			}},
			r: virtualNetwork(),
			want: virtualNetwork(
				withConditions(runtimev1alpha1.Deleting()),
			),
		},
		{
			name: "SuccessfulNotFound",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockDelete: func(_ context.Context, _ string, _ string) (result network.VirtualNetworksDeleteFuture, err error) {
					return network.VirtualNetworksDeleteFuture{}, autorest.DetailedError{
						StatusCode: http.StatusNotFound,
					}
				},
			}},
			r: virtualNetwork(),
			want: virtualNetwork(
				withConditions(runtimev1alpha1.Deleting()),
			),
		},
		{
			name: "Failed",
			e: &external{client: &fake.MockVirtualNetworksClient{
				MockDelete: func(_ context.Context, _ string, _ string) (result network.VirtualNetworksDeleteFuture, err error) {
					return network.VirtualNetworksDeleteFuture{}, errorBoom
				},
			}},
			r: virtualNetwork(),
			want: virtualNetwork(
				withConditions(runtimev1alpha1.Deleting()),
			),
			wantErr: errors.Wrap(errorBoom, errDeleteVirtualNetwork),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.e.Delete(ctx, tc.r)

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.e.Delete(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnect(t *testing.T) {
	cases := []struct {
		name    string
		conn    *connecter
		i       resource.Managed
		want    resource.ExternalClient
		wantErr error
	}{
		{
			name:    "NotVirtualNetwork",
			conn:    &connecter{client: &test.MockClient{}},
			i:       &v1alpha1.Subnet{},
			want:    nil,
			wantErr: errors.New(errNotVirtualNetwork),
		},
		{
			name: "SuccessfulConnect",
			conn: &connecter{
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: providerName}:
							*obj.(*azurev1alpha1.Provider) = provider
						case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
							*obj.(*corev1.Secret) = providerSecret
						}
						return nil
					},
				},
				newClientFn: func(_ context.Context, _ []byte) (networkclient.VirtualNetworksClient, error) {
					return &fake.MockVirtualNetworksClient{}, nil
				},
			},
			i:    virtualNetwork(),
			want: &external{client: &fake.MockVirtualNetworksClient{}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := tc.conn.Connect(ctx, tc.i)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, got, test.EquateConditions(), cmp.AllowUnexported(external{})); diff != "" {
				t.Errorf("tc.conn.Connect(...): -want, +got:\n%s", diff)
			}
		})
	}
}

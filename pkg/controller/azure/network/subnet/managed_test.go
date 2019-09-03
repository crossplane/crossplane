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

package subnet

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
	namespace          = "coolNamespace"
	name               = "coolSubnet"
	uid                = types.UID("definitely-a-uuid")
	addressPrefix      = "10.0.0.0/16"
	virtualNetworkName = "coolVnet"
	resourceGroupName  = "coolRG"

	providerName       = "cool-aws"
	providerSecretName = "cool-aws-secret"
	providerSecretKey  = "credentials"
	providerSecretData = "definitelyini"
)

var (
	ctx       = context.Background()
	errorBoom = errors.New("boom")

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
	name       string
	e          resource.ExternalClient
	r          *v1alpha1.Subnet
	want       *v1alpha1.Subnet
	returnsErr bool
}

type subnetModifier func(*v1alpha1.Subnet)

func withConditions(c ...runtimev1alpha1.Condition) subnetModifier {
	return func(r *v1alpha1.Subnet) { r.Status.ConditionedStatus.Conditions = c }
}

func withState(s string) subnetModifier {
	return func(r *v1alpha1.Subnet) { r.Status.State = s }
}

func subnet(sm ...subnetModifier) *v1alpha1.Subnet {
	r := &v1alpha1.Subnet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.SubnetSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference: &corev1.ObjectReference{Namespace: namespace, Name: providerName},
			},
			Name:               name,
			VirtualNetworkName: virtualNetworkName,
			ResourceGroupName:  resourceGroupName,
			SubnetPropertiesFormat: v1alpha1.SubnetPropertiesFormat{
				AddressPrefix: addressPrefix,
			},
		},
		Status: v1alpha1.SubnetStatus{},
	}

	for _, m := range sm {
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
			name: "SuccessfulCreate",
			e: &external{client: &fake.MockSubnetsClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ string, _ network.Subnet) (network.SubnetsCreateOrUpdateFuture, error) {
					return network.SubnetsCreateOrUpdateFuture{}, nil
				},
			}},
			r: subnet(),
			want: subnet(
				withConditions(runtimev1alpha1.Creating()),
			),
		},
		{
			name: "FailedCreate",
			e: &external{client: &fake.MockSubnetsClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ string, _ network.Subnet) (network.SubnetsCreateOrUpdateFuture, error) {
					return network.SubnetsCreateOrUpdateFuture{}, errorBoom
				},
			}},
			r: subnet(),
			want: subnet(
				withConditions(runtimev1alpha1.Creating()),
			),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.e.Create(ctx, tc.r)
			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.e.Create(...) error: want: %t got: %t", tc.returnsErr, err != nil)
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
			name: "SuccessfulObserveNotExist",
			e: &external{client: &fake.MockSubnetsClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string, _ string) (result network.Subnet, err error) {
					return network.Subnet{
							SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
								AddressPrefix: azure.ToStringPtr(addressPrefix),
							},
						}, autorest.DetailedError{
							StatusCode: http.StatusNotFound,
						}
				},
			}},
			r:    subnet(),
			want: subnet(),
		},
		{
			name: "SuccessfulObserveExists",
			e: &external{client: &fake.MockSubnetsClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string, _ string) (result network.Subnet, err error) {
					return network.Subnet{
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix:     azure.ToStringPtr(addressPrefix),
							ProvisioningState: azure.ToStringPtr(string(network.Available)),
						},
					}, nil
				},
			}},
			r: subnet(),
			want: subnet(
				withConditions(runtimev1alpha1.Available()),
				withState(string(network.Available)),
			),
		},
		{
			name: "FailedObserve",
			e: &external{client: &fake.MockSubnetsClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string, _ string) (result network.Subnet, err error) {
					return network.Subnet{}, errorBoom
				},
			}},
			r:          subnet(),
			want:       subnet(),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.e.Observe(ctx, tc.r)
			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.e.Observe(...) error: want: %t got: %t", tc.returnsErr, err != nil)
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
			name: "SuccessfulDoesNotNeedUpdate",
			e: &external{client: &fake.MockSubnetsClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string, _ string) (result network.Subnet, err error) {
					return network.Subnet{
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: azure.ToStringPtr(addressPrefix),
						},
					}, nil
				},
			}},
			r:    subnet(),
			want: subnet(),
		},
		{
			name: "SuccessfulNeedsUpdate",
			e: &external{client: &fake.MockSubnetsClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string, _ string) (result network.Subnet, err error) {
					return network.Subnet{
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: azure.ToStringPtr("10.1.0.0/16"),
						},
					}, nil
				},
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ string, _ network.Subnet) (network.SubnetsCreateOrUpdateFuture, error) {
					return network.SubnetsCreateOrUpdateFuture{}, nil
				},
			}},
			r:    subnet(),
			want: subnet(),
		},
		{
			name: "UnsuccessfulGet",
			e: &external{client: &fake.MockSubnetsClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string, _ string) (result network.Subnet, err error) {
					return network.Subnet{
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: azure.ToStringPtr(addressPrefix),
						},
					}, errorBoom
				},
			}},
			r:          subnet(),
			want:       subnet(),
			returnsErr: true,
		},
		{
			name: "UnsuccessfulUpdate",
			e: &external{client: &fake.MockSubnetsClient{
				MockGet: func(_ context.Context, _ string, _ string, _ string, _ string) (result network.Subnet, err error) {
					return network.Subnet{
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: azure.ToStringPtr("10.1.0.0/16"),
						},
					}, nil
				},
				MockCreateOrUpdate: func(_ context.Context, _ string, _ string, _ string, _ network.Subnet) (network.SubnetsCreateOrUpdateFuture, error) {
					return network.SubnetsCreateOrUpdateFuture{}, errorBoom
				},
			}},
			r:          subnet(),
			want:       subnet(),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.e.Update(ctx, tc.r)
			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.e.Update(...) error: want: %t got: %t", tc.returnsErr, err != nil)
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
			name: "Successful",
			e: &external{client: &fake.MockSubnetsClient{
				MockDelete: func(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string) (result network.SubnetsDeleteFuture, err error) {
					return network.SubnetsDeleteFuture{}, nil
				},
			}},
			r: subnet(),
			want: subnet(
				withConditions(runtimev1alpha1.Deleting()),
			),
		},
		{
			name: "SuccessfulNotFound",
			e: &external{client: &fake.MockSubnetsClient{
				MockDelete: func(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string) (result network.SubnetsDeleteFuture, err error) {
					return network.SubnetsDeleteFuture{}, autorest.DetailedError{
						StatusCode: http.StatusNotFound,
					}
				},
			}},
			r: subnet(),
			want: subnet(
				withConditions(runtimev1alpha1.Deleting()),
			),
		},
		{
			name: "Failed",
			e: &external{client: &fake.MockSubnetsClient{
				MockDelete: func(ctx context.Context, resourceGroupName string, virtualNetworkName string, subnetName string) (result network.SubnetsDeleteFuture, err error) {
					return network.SubnetsDeleteFuture{}, errorBoom
				},
			}},
			r: subnet(),
			want: subnet(
				withConditions(runtimev1alpha1.Deleting()),
			),
			returnsErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.e.Delete(ctx, tc.r)

			if tc.returnsErr != (err != nil) {
				t.Errorf("tc.csd.Delete(...) error: want: %t got: %t", tc.returnsErr, err != nil)
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
		i       *v1alpha1.Subnet
		want    resource.ExternalClient
		wantErr error
	}{
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
				newClientFn: func(_ context.Context, _ []byte) (networkclient.SubnetsClient, error) {
					return &fake.MockSubnetsClient{}, nil
				},
			},
			i:    subnet(),
			want: &external{client: &fake.MockSubnetsClient{}},
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

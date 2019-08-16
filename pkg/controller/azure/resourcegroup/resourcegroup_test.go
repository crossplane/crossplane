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

package resourcegroup

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/resourcegroup"
	fakerg "github.com/crossplaneio/crossplane/pkg/clients/azure/resourcegroup/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "cool-namespace"
	uid       = types.UID("definitely-a-uuid")
	name      = "cool-rg"
	location  = "coolplace"

	providerName       = "cool-azure"
	providerSecretName = "cool-azure-secret"
	providerSecretKey  = "credentials"
	providerSecretData = "definitelyjson"
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

type resourceModifier func(*azurev1alpha1.ResourceGroup)

func withConditions(c ...corev1alpha1.Condition) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.Status.ConditionedStatus.Conditions = c }
}

func withFinalizers(f ...string) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.ObjectMeta.Finalizers = f }
}

func withReclaimPolicy(p corev1alpha1.ReclaimPolicy) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.Spec.ReclaimPolicy = p }
}

func withName(n string) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.Status.Name = n }
}

func withSpecName(n string) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.Spec.Name = n }
}

func withDeletionTimestamp(t time.Time) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t} }
}

// func withDeletionTimestamp(t time.Time) resourceModifier {
// 	return func(r *azurev1alpha1.ResourceGroup) { r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t} }
// }

func resource(rm ...resourceModifier) *azurev1alpha1.ResourceGroup {
	r := &azurev1alpha1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: azurev1alpha1.ResourceGroupSpec{
			Name:     name,
			Location: location,
			ResourceSpec: corev1alpha1.ResourceSpec{
				ProviderReference: &corev1.ObjectReference{Namespace: namespace, Name: providerName},
			},
		},
		Status: azurev1alpha1.ResourceGroupStatus{},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

func TestCreate(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		r           *azurev1alpha1.ResourceGroup
		want        *azurev1alpha1.ResourceGroup
		wantRequeue bool
	}{
		{
			name: "SuccessfulCreate",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ resources.Group) (resources.Group, error) {
					return resources.Group{}, nil
				},
			}},
			r: resource(),
			want: resource(
				withConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileSuccess()),
				withFinalizers(finalizer),
				withName(name),
			),
			wantRequeue: true,
		},
		{
			name: "FailedCreate",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ resources.Group) (resources.Group, error) {
					return resources.Group{}, errorBoom
				},
			}},
			r: resource(),
			want: resource(
				withConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileError(errorBoom)),
			),
			wantRequeue: true,
		},
		{
			name: "FailedCreateDueToName",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ resources.Group) (resources.Group, error) {
					return resources.Group{}, errorBoom
				},
			}},
			r: resource(
				withSpecName("foo."),
			),
			want: resource(
				withSpecName("foo."),
				withConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileError(errorBoom)),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Create(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.CreateOrUpdate(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSync(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		r           *azurev1alpha1.ResourceGroup
		want        *azurev1alpha1.ResourceGroup
		wantRequeue bool
	}{
		{
			name: "SuccessfulSyncWhileResourceReady",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCheckExistence: func(_ context.Context, _ string) (result autorest.Response, err error) {
					return autorest.Response{Response: &http.Response{StatusCode: 204}}, nil
				},
			}},
			r: resource(
				withFinalizers(finalizer),
				withName(name),
			),
			want: resource(
				withFinalizers(finalizer),
				withName(name),
				withConditions(corev1alpha1.Available(), corev1alpha1.ReconcileSuccess()),
			),
			wantRequeue: false,
		},
		{
			name: "FailedSyncResourceNotExist",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCheckExistence: func(_ context.Context, _ string) (result autorest.Response, err error) {
					return autorest.Response{Response: &http.Response{StatusCode: 404}}, nil
				},
			}},
			r: resource(
				withFinalizers(finalizer),
				withName(name),
			),
			want: resource(
				withFinalizers(finalizer),
				withName(name),
				withConditions(corev1alpha1.ReconcileError(errDeleted)),
			),
			wantRequeue: true,
		},
		{
			name: "FailedSyncUnrecognizedResponse",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCheckExistence: func(_ context.Context, _ string) (result autorest.Response, err error) {
					return autorest.Response{Response: &http.Response{StatusCode: 400}}, nil
				},
			}},
			r: resource(
				withFinalizers(finalizer),
				withName(name),
			),
			want: resource(
				withFinalizers(finalizer),
				withName(name),
				withConditions(corev1alpha1.ReconcileSuccess()),
			),
			wantRequeue: true,
		},
		{
			name: "FailedCheck",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCheckExistence: func(_ context.Context, _ string) (result autorest.Response, err error) {
					return autorest.Response{}, errorBoom
				},
			}},
			r: resource(
				withFinalizers(finalizer),
				withName(name),
			),
			want: resource(
				withFinalizers(finalizer),
				withName(name),
				withConditions(corev1alpha1.ReconcileError(errorBoom)),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Sync(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.CheckExistence(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := cmp.Diff(tc.want, tc.r, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		r           *azurev1alpha1.ResourceGroup
		want        *azurev1alpha1.ResourceGroup
		wantRequeue bool
	}{
		{
			name: "ReclaimRetainSuccessfulDelete",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockDelete: func(_ context.Context, _ string) (result resources.GroupsDeleteFuture, err error) {
					return resources.GroupsDeleteFuture{}, nil
				},
			}},
			r: resource(withFinalizers(finalizer), withReclaimPolicy(corev1alpha1.ReclaimRetain)),
			want: resource(
				withReclaimPolicy(corev1alpha1.ReclaimRetain),
				withConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileSuccess()),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteSuccessfulDelete",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockDelete: func(_ context.Context, _ string) (result resources.GroupsDeleteFuture, err error) {
					return resources.GroupsDeleteFuture{}, nil
				},
			}},
			r: resource(withFinalizers(finalizer), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: resource(
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileSuccess()),
			),
			wantRequeue: false,
		},
		{
			name: "ReclaimDeleteFailedDelete",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockDelete: func(_ context.Context, _ string) (result resources.GroupsDeleteFuture, err error) {
					return resources.GroupsDeleteFuture{}, errorBoom
				},
			}},
			r: resource(withFinalizers(finalizer), withReclaimPolicy(corev1alpha1.ReclaimDelete)),
			want: resource(
				withFinalizers(finalizer),
				withReclaimPolicy(corev1alpha1.ReclaimDelete),
				withConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileError(errorBoom)),
			),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Delete(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.csd.Delete(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
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
		conn    connecter
		i       *azurev1alpha1.ResourceGroup
		want    createsyncdeleter
		wantErr error
	}{
		{
			name: "SuccessfulConnect",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*azurev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ []byte) (resourcegroup.GroupsClient, error) {
					return &fakerg.MockClient{}, nil
				},
			},
			i:    resource(),
			want: &azureResourceGroup{client: &fakerg.MockClient{}},
		},
		{
			name: "FailedToGetProvider",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return kerrors.NewNotFound(schema.GroupResource{}, providerName)
				}},
				newClient: func(_ []byte) (resourcegroup.GroupsClient, error) {
					return &fakerg.MockClient{}, nil
				},
			},
			i:       resource(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider %s/%s:  \"%s\" not found", namespace, providerName, providerName)),
		},
		{
			name: "FailedToGetProviderSecret",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*azurev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return kerrors.NewNotFound(schema.GroupResource{}, providerSecretName)
					}
					return nil
				}},
				newClient: func(_ []byte) (resourcegroup.GroupsClient, error) {
					return &fakerg.MockClient{}, nil
				},
			},
			i:       resource(),
			wantErr: errors.WithStack(errors.Errorf("cannot get provider secret %s/%s:  \"%s\" not found", namespace, providerSecretName, providerSecretName)),
		},
		{
			name: "FailedToCreateAzureCacheClient",
			conn: &providerConnecter{
				kube: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*azurev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = providerSecret
					}
					return nil
				}},
				newClient: func(_ []byte) (resourcegroup.GroupsClient, error) { return nil, errorBoom },
			},
			i:       resource(),
			want:    &azureResourceGroup{},
			wantErr: errors.Wrap(errorBoom, "cannot create new Azure Resource Group client"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := tc.conn.Connect(ctx, tc.i)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(azureResourceGroup{})); diff != "" {
				t.Errorf("tc.conn.Connect(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type mockConnector struct {
	MockConnect func(ctx context.Context, i *azurev1alpha1.ResourceGroup) (createsyncdeleter, error)
}

func (c *mockConnector) Connect(ctx context.Context, i *azurev1alpha1.ResourceGroup) (createsyncdeleter, error) {
	return c.MockConnect(ctx, i)
}

type mockCSD struct {
	MockCreate func(ctx context.Context, i *azurev1alpha1.ResourceGroup) bool
	MockSync   func(ctx context.Context, i *azurev1alpha1.ResourceGroup) bool
	MockDelete func(ctx context.Context, i *azurev1alpha1.ResourceGroup) bool
}

func (csd *mockCSD) Create(ctx context.Context, i *azurev1alpha1.ResourceGroup) bool {
	return csd.MockCreate(ctx, i)
}

func (csd *mockCSD) Sync(ctx context.Context, i *azurev1alpha1.ResourceGroup) bool {
	return csd.MockSync(ctx, i)
}

func (csd *mockCSD) Delete(ctx context.Context, i *azurev1alpha1.ResourceGroup) bool {
	return csd.MockDelete(ctx, i)
}

func TestReconcile(t *testing.T) {
	cases := []struct {
		name    string
		rec     *Reconciler
		req     reconcile.Request
		want    reconcile.Result
		wantErr error
	}{
		{
			name: "SuccessfulDelete",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *azurev1alpha1.ResourceGroup) (createsyncdeleter, error) {
					return &mockCSD{MockDelete: func(_ context.Context, _ *azurev1alpha1.ResourceGroup) bool { return false }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*azurev1alpha1.ResourceGroup) = *(resource(withName(name), withDeletionTimestamp(time.Now())))
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "SuccessfulCreate",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *azurev1alpha1.ResourceGroup) (createsyncdeleter, error) {
					return &mockCSD{MockCreate: func(_ context.Context, _ *azurev1alpha1.ResourceGroup) bool { return true }}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*azurev1alpha1.ResourceGroup) = *(resource())
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
		{
			name: "SuccessfulSync",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *azurev1alpha1.ResourceGroup) (createsyncdeleter, error) {
					return &mockCSD{
						MockSync: func(_ context.Context, _ *azurev1alpha1.ResourceGroup) bool { return false },
					}, nil
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*azurev1alpha1.ResourceGroup) = *(resource(withName(name)))
						return nil
					},
					MockUpdate: func(_ context.Context, _ runtime.Object, _ ...client.UpdateOption) error { return nil },
					MockCreate: func(_ context.Context, _ runtime.Object, _ ...client.CreateOption) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetNonexistentResource",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, name)
					},
					MockUpdate: func(_ context.Context, _ runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: nil,
		},
		{
			name: "FailedToGetExtantResource",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errorBoom
					},
					MockUpdate: func(_ context.Context, _ runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: false},
			wantErr: errors.Wrapf(errorBoom, "cannot get resource %s/%s", namespace, name),
		},
		{
			name: "FailedToConnect",
			rec: &Reconciler{
				connecter: &mockConnector{MockConnect: func(_ context.Context, _ *azurev1alpha1.ResourceGroup) (createsyncdeleter, error) {
					return nil, errorBoom
				}},
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*azurev1alpha1.ResourceGroup) = *(resource())
						return nil
					},
					MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						want := resource(withConditions(corev1alpha1.ReconcileError(errorBoom)))
						got := obj.(*azurev1alpha1.ResourceGroup)
						if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
							t.Errorf("kube.Update(...): -want, +got:\n%s", diff)
						}
						return nil
					},
				},
			},
			req:     reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			want:    reconcile.Result{Requeue: true},
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, gotErr := tc.rec.Reconcile(tc.req)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want, gotResult); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}

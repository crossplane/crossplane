/*
Copyright 2018 The Crossplane Authors.

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

package bucket

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplaneio/crossplane/pkg/apis/azure"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1"
	v1alpha1test "github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1/test"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	buckettest "github.com/crossplaneio/crossplane/pkg/controller/storage/bucket/test"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	testNS        = "default"
	testName      = "testName"
	testBucketUID = "test-bucket-uid"
)

func init() {
	_ = azure.AddToScheme(scheme.Scheme)
}

type mockAccountResolver struct {
	mockResolve func(account *v1alpha1.Account, claim corev1alpha1.ResourceClaim) error
}

func (m *mockAccountResolver) resolve(account *v1alpha1.Account, claim corev1alpha1.ResourceClaim) error {
	return m.mockResolve(account, claim)
}

var _ accountResolver = &mockAccountResolver{}

func TestAzureAccountHandler_Find(t *testing.T) {
	type args struct {
		n types.NamespacedName
		c client.Client
	}
	type want struct {
		err error
		res corev1alpha1.Resource
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Failed",
			args: args{
				n: types.NamespacedName{Namespace: testNS, Name: testName},
				c: fake.NewFakeClient(),
			},
			want: want{
				err: errors.Wrapf(errors.New("accounts.storage.azure.crossplane.io \"testName\" not found"),
					"cannot find Azure account instance %s/%s", testNS, testName),
			},
		},
		{
			name: "Successful",
			args: args{
				n: types.NamespacedName{Namespace: testNS, Name: testName},
				c: test.NewMockClient(),
			},
			want: want{
				res: &v1alpha1.Account{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AzureAccountHandler{}
			got, err := h.Find(tt.args.n, tt.args.c)
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("AzureAccountHandler.Find() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("AzureAccountHandler.Find() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestAzureAccountHandler_Provision(t *testing.T) {
	type args struct {
		class *corev1alpha1.ResourceClass
		claim corev1alpha1.ResourceClaim
		c     client.Client
	}
	type want struct {
		err error
		res corev1alpha1.Resource
	}
	tests := []struct {
		name     string
		args     args
		resolver accountResolver
		want     want
	}{
		{
			name: "FailedValuesResolver",
			args: args{
				class: &corev1alpha1.ResourceClass{},
				claim: &storagev1alpha1.Bucket{},
			},
			resolver: &mockAccountResolver{
				mockResolve: func(account *v1alpha1.Account, claim corev1alpha1.ResourceClaim) error {
					return errors.New("test-data-resolver-error")
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("test-data-resolver-error"),
					"failed to resolve account claim values"),
			},
		},
		{
			name: "FailedToCreate",
			args: args{
				class: &corev1alpha1.ResourceClass{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				},
				claim: &storagev1alpha1.Bucket{},
				c: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-create-error")
					},
				},
			},
			resolver: &mockAccountResolver{
				mockResolve: func(account *v1alpha1.Account, claim corev1alpha1.ResourceClaim) error {
					account.Name = testName
					return nil
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("test-create-error"),
					"cannot create instance %s/%s", testNS, testName),
			},
		},
		{
			name: "Successful",
			args: args{
				class: &corev1alpha1.ResourceClass{
					ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				},
				claim: &storagev1alpha1.Bucket{},
				c:     test.NewMockClient(),
			},
			resolver: &mockAccountResolver{
				mockResolve: func(account *v1alpha1.Account, claim corev1alpha1.ResourceClaim) error {
					account.Name = testName
					return nil
				},
			},
			want: want{
				res: v1alpha1test.NewMockAccount(testNS, testName).
					WithTypeMeta(metav1.TypeMeta{
						APIVersion: v1alpha1.APIVersion,
						Kind:       v1alpha1.AccountKind,
					}).
					WithObjectMeta(metav1.ObjectMeta{
						Namespace: testNS,
						Name:      testName,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(&storagev1alpha1.Bucket{
								TypeMeta: metav1.TypeMeta{
									APIVersion: storagev1alpha1.APIVersion,
									Kind:       storagev1alpha1.BucketKind,
								},
							})),
						},
					}).
					WithSpecClassRef(&corev1.ObjectReference{
						APIVersion: corev1alpha1.APIVersion,
						Kind:       corev1alpha1.ResourceClassKind,
						Namespace:  testNS,
					}).
					WithSpecClaimRef(&corev1.ObjectReference{
						APIVersion: storagev1alpha1.APIVersion,
						Kind:       storagev1alpha1.BucketKind,
					}).
					WithSpecStorageAccountSpec(&v1alpha1.StorageAccountSpec{}).
					Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AzureAccountHandler{accountResolver: tt.resolver}
			got, err := h.Provision(tt.args.class, tt.args.claim, tt.args.c)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("AzureAccountHandler.Provision(): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("AzureAccountHandler.Provision(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAzureContainerHandler_Find(t *testing.T) {
	type args struct {
		n types.NamespacedName
		c client.Client
	}
	type want struct {
		err error
		res corev1alpha1.Resource
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Failed",
			args: args{
				n: types.NamespacedName{Namespace: testNS, Name: testName},
				c: fake.NewFakeClient(),
			},
			want: want{
				err: errors.Wrapf(errors.New("containers.storage.azure.crossplane.io \"testName\" not found"),
					"cannot find Azure container instance %s/%s", testNS, testName),
			},
		},
		{
			name: "Successful",
			args: args{
				n: types.NamespacedName{Namespace: testNS, Name: testName},
				c: test.NewMockClient(),
			},
			want: want{
				res: &v1alpha1.Container{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AzureContainerHandler{}
			got, err := h.Find(tt.args.n, tt.args.c)
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("AzureContainerHandler.Find() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("AzureContainerHandler.Find() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestAzureContainerHandler_Provision(t *testing.T) {
	type args struct {
		class *corev1alpha1.ResourceClass
		claim corev1alpha1.ResourceClaim
		c     client.Client
	}
	type want struct {
		err error
		res corev1alpha1.Resource
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "FailedToRetrieveAccount",
			args: args{
				class: &corev1alpha1.ResourceClass{
					ObjectMeta:  metav1.ObjectMeta{Namespace: testNS},
					ProviderRef: corev1.LocalObjectReference{Name: testName},
				},
				claim: &storagev1alpha1.Bucket{},
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key types.NamespacedName, obj runtime.Object) error {
						return errors.New("test-get-account-error")
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("test-get-account-error"),
					"failed to retrieve account reference object: %s/%s", testNS, testName),
			},
		},
		{
			name: "FailedNotABucketClaim",
			args: args{
				class: &corev1alpha1.ResourceClass{
					ObjectMeta:  metav1.ObjectMeta{Namespace: testNS},
					ProviderRef: corev1.LocalObjectReference{Name: testName},
				},
				claim: &storagev1alpha1.MySQLInstance{},
				c:     test.NewMockClient(),
			},
			want: want{
				err: errors.New("unexpected claim type: *v1alpha1.MySQLInstance"),
			},
		},
		{
			name: "FailedToUpdateBucket",
			args: args{
				class: &corev1alpha1.ResourceClass{
					ObjectMeta:  metav1.ObjectMeta{Namespace: testNS},
					ProviderRef: corev1.LocalObjectReference{Name: testName},
				},
				claim: buckettest.NewBucket(testNS, testName).Bucket,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return nil
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-bucket-update-error")
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("test-bucket-update-error"),
					"failed to update bucket claim with account owner reference: %s", testName),
			},
		},
		{
			name: "FailedToCreateContainer",
			args: args{
				class: &corev1alpha1.ResourceClass{
					ObjectMeta:  metav1.ObjectMeta{Namespace: testNS},
					ProviderRef: corev1.LocalObjectReference{Name: testName},
				},
				claim: buckettest.NewBucket(testNS, testName).WithObjectMetaUID("test-bucket-uid").Bucket,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return nil
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-create-container-error")
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("test-create-container-error"),
					"cannot create container instance %s/%s", testNS, "test-bucket-uid"),
			},
		},
		{
			name: "Successful",
			args: args{
				class: &corev1alpha1.ResourceClass{
					ObjectMeta:  metav1.ObjectMeta{Namespace: testNS, Name: testName},
					ProviderRef: corev1.LocalObjectReference{Name: testName},
				},
				claim: buckettest.NewBucket(testNS, testName).WithObjectMetaUID("test-bucket-uid").Bucket,
				c:     test.NewMockClient(),
			},
			want: want{
				res: v1alpha1test.NewMockContainer(testNS, testName).
					WithTypeMeta(metav1.TypeMeta{
						APIVersion: v1alpha1.APIVersion,
						Kind:       v1alpha1.ContainerKind,
					}).
					WithObjectMeta(metav1.ObjectMeta{
						Namespace: testNS,
						Name:      testBucketUID,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(buckettest.NewBucket(testNS, testName).
								WithTypeMeta(metav1.TypeMeta{
									APIVersion: storagev1alpha1.APIVersion,
									Kind:       storagev1alpha1.BucketKind,
								}).
								WithObjectMetaUID(testBucketUID).
								Bucket)),
						},
					}).
					WithSpecClassRef(buckettest.NewBucketClassReference(testNS, testName)).
					WithSpecClaimRef(buckettest.NewBucketClaimReference(testNS, testName, testBucketUID)).
					WithSpecMetadata(map[string]string{}).
					WithSpecAccountRef(testName).
					Container,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AzureContainerHandler{}
			got, err := h.Provision(tt.args.class, tt.args.claim, tt.args.c)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("AzureAccountHandler.Provision(): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("AzureAccountHandler.Provision(): -want, +got\n%s", diff)
			}
		})
	}
}

func TestAzureAccountHandler_SetBindStatus(t *testing.T) {
	nn := types.NamespacedName{Namespace: testNS, Name: testName}
	type args struct {
		n     types.NamespacedName
		c     client.Client
		bound bool
	}
	type want struct {
		err error
		act *v1alpha1.Account
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "FailedToGetAccount",
			args: args{
				n: nn,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errors.New("test-get-account-error")
					},
				},
				bound: true,
			},
			want: want{
				err: errors.Wrapf(errors.New("test-get-account-error"),
					"cannot get account %s/%s", testNS, testName),
			},
		},
		{
			name: "FailedToGetAccountNotFoundAndNotBound",
			args: args{
				n:     nn,
				c:     fake.NewFakeClient(),
				bound: false,
			},
			want: want{},
		},
		{
			name: "FailedToGetAccountNotFoundAndBound",
			args: args{
				n:     nn,
				c:     fake.NewFakeClient(),
				bound: true,
			},
			want: want{
				err: errors.Wrapf(errors.New("accounts.storage.azure.crossplane.io \"testName\" not found"),
					"cannot get account %s/%s", testNS, testName),
			},
		},
		{
			name: "FailedToUpdateAccount",
			args: args{
				n: nn,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return nil
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-update-account-error")
					},
				},
				bound: true,
			},
			want: want{
				err: errors.Wrapf(errors.New("test-update-account-error"),
					"cannot update account %s/%s", testNS, testName),
			},
		},
		{
			name: "Success",
			args: args{
				n:     nn,
				c:     fake.NewFakeClient(v1alpha1test.NewMockAccount(testNS, testName).Account),
				bound: true,
			},
			want: want{
				act: v1alpha1test.NewMockAccount(testNS, testName).WithStatusSetBound(true).Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AzureAccountHandler{}
			err := h.SetBindStatus(tt.args.n, tt.args.c, tt.args.bound)
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("AzureAccountHandler.SetBindStatus() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if tt.want.act != nil {
				act := &v1alpha1.Account{}
				if err := tt.args.c.Get(context.TODO(), nn, act); err != nil {
					t.Errorf("AzureAccountHandler.SetBindStatus() unexected test error getting account: %s", nn)
				}
				if diff := cmp.Diff(act, tt.want.act); diff != "" {
					t.Errorf("AzureAccountHandler.SetBindStatus() = %v, want %v\n%s", act, tt.want.act, diff)
				}
			}
		})
	}
}

func TestAzureContainerHandler_SetBindStatus(t *testing.T) {
	nn := types.NamespacedName{Namespace: testNS, Name: testName}
	type args struct {
		n     types.NamespacedName
		c     client.Client
		bound bool
	}
	type want struct {
		err error
		con *v1alpha1.Container
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "FailedToGetContainer",
			args: args{
				n: nn,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errors.New("test-get-container-error")
					},
				},
				bound: true,
			},
			want: want{
				err: errors.Wrapf(errors.New("test-get-container-error"),
					"cannot get container %s/%s", testNS, testName),
			},
		},
		{
			name: "FailedToGetContainerNotFoundAndNotBound",
			args: args{
				n:     nn,
				c:     fake.NewFakeClient(),
				bound: false,
			},
			want: want{},
		},
		{
			name: "FailedToGetContainerNotFoundAndBound",
			args: args{
				n:     nn,
				c:     fake.NewFakeClient(),
				bound: true,
			},
			want: want{
				err: errors.Wrapf(errors.New("containers.storage.azure.crossplane.io \"testName\" not found"),
					"cannot get container %s/%s", testNS, testName),
			},
		},
		{
			name: "FailedToUpdateContainer",
			args: args{
				n: nn,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return nil
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-update-container-error")
					},
				},
				bound: true,
			},
			want: want{
				err: errors.Wrapf(errors.New("test-update-container-error"),
					"cannot update container %s/%s", testNS, testName),
			},
		},
		{
			name: "Success",
			args: args{
				n:     nn,
				c:     fake.NewFakeClient(v1alpha1test.NewMockContainer(testNS, testName).Container),
				bound: true,
			},
			want: want{
				con: v1alpha1test.NewMockContainer(testNS, testName).
					WithStatusSetBound(true).Container,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &AzureContainerHandler{}
			err := h.SetBindStatus(tt.args.n, tt.args.c, tt.args.bound)
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("AzureContainerHandler.SetBindStatus() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if tt.want.con != nil {
				act := &v1alpha1.Container{}
				if err := tt.args.c.Get(context.TODO(), nn, act); err != nil {
					t.Errorf("AzureContainerHandler.SetBindStatus() unexected test error getting container: %s", nn)
				}
				if diff := cmp.Diff(act, tt.want.con); diff != "" {
					t.Errorf("AzureContainerHandler.SetBindStatus() = %v, want %v\n%s", act, tt.want.con, diff)
				}
			}
		})
	}
}

func Test_azureAccountResolver_resolve(t *testing.T) {
	type args struct {
		account *v1alpha1.Account
		claim   corev1alpha1.ResourceClaim
	}
	type want struct {
		err error
		act *v1alpha1.Account
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "FailedNotABucket",
			args: args{
				claim: &storagev1alpha1.MySQLInstance{},
			},
			want: want{err: errors.New("unexpected claim type: *v1alpha1.MySQLInstance")},
		},
		{
			name: "InvalidSpecMissingName",
			args: args{
				claim: &storagev1alpha1.Bucket{},
			},
			want: want{err: errors.New("invalid account claim:  spec, name property is required")},
		},
		{
			name: "Successful",
			args: args{
				account: v1alpha1test.NewMockAccount(testNS, testName).Account,
				claim:   buckettest.NewBucket(testNS, testName).WithSpecName("account-name").Bucket,
			},
			want: want{
				act: v1alpha1test.NewMockAccount(testNS, "account-name").
					WithSpecStorageAccountName("account-name").Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &azureAccountResolver{}
			err := a.resolve(tt.args.account, tt.args.claim)
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("azureAccountResolver.resolve() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if tt.want.act != nil {
				if diff := cmp.Diff(tt.args.account, tt.want.act); diff != "" {
					t.Errorf("azureAccountResolver.resolve() account = %v, wantErr %v\n%s",
						tt.args.account, tt.want.act, diff)
				}
			}
		})
	}
}

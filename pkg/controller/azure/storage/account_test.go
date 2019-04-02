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

package storage

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-test/deep"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/azure"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	azurestorage "github.com/crossplaneio/crossplane/pkg/clients/azure/storage"
	azurestoragefake "github.com/crossplaneio/crossplane/pkg/clients/azure/storage/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

func init() {
	_ = azure.AddToScheme(scheme.Scheme)
}

type MockAccountSecretupdater struct {
	MockUpdateSecret func(context.Context, *storage.Account) error
}

func (m *MockAccountSecretupdater) updatesecret(ctx context.Context, a *storage.Account) error {
	return m.MockUpdateSecret(ctx, a)
}

var _ secretupdater = &MockAccountSecretupdater{}

type MockAccountSyncbacker struct {
	MockSyncback func(context.Context, *storage.Account) (reconcile.Result, error)
}

func (m *MockAccountSyncbacker) syncback(ctx context.Context, a *storage.Account) (reconcile.Result, error) {
	return m.MockSyncback(ctx, a)
}

var _ syncbacker = &MockAccountSyncbacker{}

type MockAccountCreateUpdater struct {
	MockCreate func(context.Context) (reconcile.Result, error)
	MockUpdate func(context.Context, *storage.Account) (reconcile.Result, error)
}

func newMockAccountCreateUpdater() *MockAccountCreateUpdater {
	return &MockAccountCreateUpdater{
		MockUpdate: func(i context.Context, acct *storage.Account) (result reconcile.Result, e error) {
			return requeueOnSuccess, nil
		},
		MockCreate: func(i context.Context) (result reconcile.Result, e error) {
			return requeueOnSuccess, nil
		},
	}
}

func (m *MockAccountCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	return m.MockCreate(ctx)
}

func (m *MockAccountCreateUpdater) update(ctx context.Context, a *storage.Account) (reconcile.Result, error) {
	return m.MockUpdate(ctx, a)
}

var _ createupdater = &MockAccountCreateUpdater{}

type MockAccountSyncDeleter struct {
	MockDelete func(context.Context) (reconcile.Result, error)
	MockSync   func(context.Context) (reconcile.Result, error)
}

func newMockAccountSyncDeleter() *MockAccountSyncDeleter {
	return &MockAccountSyncDeleter{
		MockSync: func(i context.Context) (result reconcile.Result, e error) {
			return requeueOnSuccess, nil
		},
		MockDelete: func(i context.Context) (result reconcile.Result, e error) {
			return result, nil
		},
	}
}

func (m *MockAccountSyncDeleter) delete(ctx context.Context) (reconcile.Result, error) {
	return m.MockDelete(ctx)
}

func (m *MockAccountSyncDeleter) sync(ctx context.Context) (reconcile.Result, error) {
	return m.MockSync(ctx)
}

var _ syncdeleter = &MockAccountSyncDeleter{}

type MockAccountHandleMaker struct {
	MockNew func(context.Context, *v1alpha1.Account) (syncdeleter, error)
}

func newMockAccountHandleMaker(rh syncdeleter, err error) *MockAccountHandleMaker {
	return &MockAccountHandleMaker{
		MockNew: func(i context.Context, bucket *v1alpha1.Account) (handler syncdeleter, e error) {
			return rh, err
		},
	}
}

func (m *MockAccountHandleMaker) newSyncdeleter(ctx context.Context, b *v1alpha1.Account) (syncdeleter, error) {
	return m.MockNew(ctx, b)
}

type account struct {
	*v1alpha1.Account
}

func newAccount(ns, name string) *account {
	return &account{Account: &v1alpha1.Account{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  ns,
			Name:       name,
			Finalizers: []string{},
		},
	}}
}

func (a *account) withUID(uid string) *account {
	a.ObjectMeta.UID = types.UID(uid)
	return a
}

func (a *account) withCondition(c corev1alpha1.Condition) *account {
	a.Status.ConditionedStatus.SetCondition(c)
	return a
}

func (a *account) withFailedCondition(reason, msg string) *account {
	a.Status.SetFailed(reason, msg)
	return a
}

func (a *account) withDeleteTimestamp(t metav1.Time) *account {
	a.Account.ObjectMeta.DeletionTimestamp = &t
	return a
}

func (a *account) withFinalizer(f string) *account {
	a.Account.ObjectMeta.Finalizers = append(a.Account.ObjectMeta.Finalizers, f)
	return a
}

func (a *account) withProvider(name string) *account {
	a.Spec.ProviderRef = corev1.LocalObjectReference{Name: name}
	return a
}

func (a *account) withReclaimPolicy(policy corev1alpha1.ReclaimPolicy) *account {
	a.Spec.ReclaimPolicy = policy
	return a
}

func (a *account) withStorageAccountSpec(spec *v1alpha1.StorageAccountSpec) *account {
	a.Spec.StorageAccountSpec = spec
	return a
}

func (a *account) withStorageAccountStatus(status *v1alpha1.StorageAccountStatus) *account {
	a.Status.StorageAccountStatus = status
	return a
}

func (a *account) withSpecStatusFromProperties(props *storage.AccountProperties) *account {
	acct := &storage.Account{
		AccountProperties: props,
	}
	a.withStorageAccountSpec(v1alpha1.NewStorageAccountSpec(acct)).
		withStorageAccountStatus(v1alpha1.NewStorageAccountStatus(acct))
	return a
}

func newStorageAccountSpec() *v1alpha1.StorageAccountSpec {
	return v1alpha1.NewStorageAccountSpec(&storage.Account{})
}

func newStoragAccountSpecWithProperties() *v1alpha1.StorageAccountSpec {
	return v1alpha1.NewStorageAccountSpec(&storage.Account{AccountProperties: &storage.AccountProperties{}})
}

type storageAccount struct {
	*storage.Account
}

func newStorageAccount() *storageAccount {
	return &storageAccount{
		Account: &storage.Account{},
	}
}

func (sa *storageAccount) withAccountProperties(ap *storage.AccountProperties) *storageAccount {
	sa.Account.AccountProperties = ap
	return sa
}

type storageAccountProperties struct {
	*storage.AccountProperties
}

func newStorageAccountProperties() *storageAccountProperties {
	return &storageAccountProperties{
		AccountProperties: &storage.AccountProperties{},
	}
}

func (sap *storageAccountProperties) withProvisioningStage(ps storage.ProvisioningState) *storageAccountProperties {
	sap.ProvisioningState = ps
	return sap
}

type provider struct {
	*azurev1alpha1.Provider
}

func newProvider(ns, name string) *provider {
	return &provider{Provider: &azurev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}}
}

func (p *provider) withCondition(c corev1alpha1.Condition) *provider {
	p.Status.ConditionedStatus.SetCondition(c)
	return p
}

func (p *provider) withSecret(name, key string) *provider {
	p.Spec.Secret = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name,
		},
		Key: key,
	}
	return p
}

type secret struct {
	*corev1.Secret
}

func newSecret(ns, name string) *secret {
	return &secret{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		},
	}
}

func (s *secret) withKeyData(key, data string) *secret {
	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}
	s.Data[key] = []byte(data)
	return s
}

const (
	testNamespace   = "default"
	testAccountName = "testAccount"
)

func TestReconciler_Reconcile(t *testing.T) {
	ns := testNamespace
	name := testAccountName
	key := types.NamespacedName{Namespace: ns, Name: name}
	req := reconcile.Request{NamespacedName: key}
	ctx := context.TODO()
	rsDone := reconcile.Result{}

	type fields struct {
		client client.Client
		maker  syncdeleterMaker
	}
	type want struct {
		res  reconcile.Result
		err  error
		acct *v1alpha1.Account
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name:   "get-err-not-found",
			fields: fields{client: fake.NewFakeClient(), maker: nil},
			want:   want{res: rsDone},
		},
		{
			name: "get-error-other",
			fields: fields{
				client: &test.MockClient{
					MockGet: func(context.Context, client.ObjectKey, runtime.Object) error {
						return errors.New("test-get-error")
					},
				},
			},
			want: want{res: rsDone, err: errors.New("test-get-error")},
		},
		{
			name: "account-handler-error",
			fields: fields{
				client: fake.NewFakeClient(newAccount(ns, name).withFinalizer("foo.bar").Account),
				maker:  newMockAccountHandleMaker(nil, errors.New("handler-syncdeleterMaker-error")),
			},
			want: want{
				res: resultRequeue,
				acct: newAccount(ns, name).
					withFailedCondition(failedToGetHandler, "handler-syncdeleterMaker-error").
					withFinalizer("foo.bar").Account,
			},
		},
		{
			name: "reconcile-delete",
			fields: fields{
				client: fake.NewFakeClient(newAccount(ns, name).
					withDeleteTimestamp(metav1.NewTime(time.Now())).Account),
				maker: newMockAccountHandleMaker(newMockAccountSyncDeleter(), nil),
			},
			want: want{res: rsDone},
		},
		{
			name: "reconcile-sync",
			fields: fields{
				client: fake.NewFakeClient(newAccount(ns, name).Account),
				maker:  newMockAccountHandleMaker(newMockAccountSyncDeleter(), nil),
			},
			want: want{res: requeueOnSuccess},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Client:           tt.fields.client,
				syncdeleterMaker: tt.fields.maker,
			}
			got, err := r.Reconcile(req)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("Reconciler.Reconcile() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("Reconciler.Reconcile() result = %v, wantRs %v\n%s", got, tt.want.res, diff)
			}
			if tt.want.acct != nil {
				b := &v1alpha1.Account{}
				if err := r.Get(ctx, key, b); err != nil {
					t.Errorf("Reconciler.Reconcile() account error: %s", err)
				}
				if diff := deep.Equal(b, tt.want.acct); diff != nil {
					t.Errorf("Reconciler.Reconcile() account = \n%+v, wantObj \n%+v\n%s", b, tt.want.acct, diff)
				}
			}
		})
	}
}

func Test_accountHandleMaker_newHandler(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	bucketName := testAccountName
	providerName := "test-provider"
	secretName := "test-secret"
	secretKey := "creds"
	secretData := `{"clientId": "0f32e96b-b9a4-49ce-a857-243a33b20e5c",
	"clientSecret": "49d8cab5-d47a-4d1a-9133-5c5db29c345d",
	"subscriptionId": "bf1b0e59-93da-42e0-82c6-5a1d94227911",
	"tenantId": "302de427-dba9-4452-8583-a4268e46de6b",
	"activeDirectoryEndpointUrl": "https://login.microsoftonline.com",
	"resourceManagerEndpointUrl": "https://management.azure.com/",
	"activeDirectoryGraphResourceId": "https://graph.windows.net/",
	"sqlManagementEndpointUrl": "https://management.core.windows.net:8443/",
	"galleryEndpointUrl": "https://gallery.azure.com/",
	"managementEndpointUrl": "https://management.core.windows.net/"}`

	tests := []struct {
		name    string
		kube    client.Client
		acct    *v1alpha1.Account
		want    syncdeleter
		wantErr error
	}{
		{
			name: "err provider is not found",
			kube: fake.NewFakeClient(),
			acct: newAccount(ns, bucketName).withProvider(providerName).Account,
			wantErr: kerrors.NewNotFound(schema.GroupResource{
				Group:    azurev1alpha1.Group,
				Resource: "providers"}, "test-provider"),
		},
		{
			name: "provider is not ready",
			kube: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, "", "")).Provider),
			acct:    newAccount(ns, bucketName).withProvider("test-provider").Account,
			wantErr: errors.Errorf("provider: %s is not ready", ns+"/test-provider"),
		},
		{
			name: "provider secret is not found",
			kube: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
				withSecret(secretName, secretKey).Provider),
			acct: newAccount(ns, bucketName).withProvider("test-provider").Account,
			wantErr: errors.WithStack(
				errors.Errorf("cannot get provider's secret %s/%s: secrets \"%s\" not found", ns, secretName, secretName)),
		},
		{
			name: "invalid credentials",
			kube: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
				withSecret(secretName, secretKey).Provider,
				newSecret(ns, secretName).Secret),
			acct: newAccount(ns, bucketName).withProvider("test-provider").Account,
			wantErr: errors.WithStack(
				errors.Errorf("cannot create storageClient from json: cannot unmarshal Azure client secret data: unexpected end of JSON input")),
		},
		{
			name: "kube created",
			kube: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
				withSecret(secretName, secretKey).Provider,
				newSecret(ns, secretName).withKeyData(secretKey, secretData).Secret),
			acct: newAccount(ns, bucketName).withProvider("test-provider").Account,
			want: newAccountSyncDeleter(&azurestorage.AccountHandle{}, nil, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &accountSyncdeleterMaker{
				Client: tt.kube,
			}
			got, err := m.newSyncdeleter(ctx, tt.acct)
			if diff := deep.Equal(err, tt.wantErr); diff != nil {
				t.Errorf("accountSyncdeleterMaker.newSyncdeleter() error = \n%v, wantErr: \n%v\n%s", err, tt.wantErr, diff)
				return
			}
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("accountSyncdeleterMaker.newSyncdeleter() = \n%+v, want \n%+v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test_syncdeleter_delete(t *testing.T) {
	ctx := context.TODO()
	ns := "default"
	bucketName := "test-account"
	type fields struct {
		ao   azurestorage.AccountOperations
		cc   client.Client
		acct *v1alpha1.Account
	}
	type want struct {
		err  error
		res  reconcile.Result
		acct *v1alpha1.Account
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "retain-policy",
			fields: fields{
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimRetain).
					withFinalizer(finalizer).withFinalizer("test").Account,
				cc: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
			},
			want: want{
				err: nil,
				res: reconcile.Result{},
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimRetain).
					withFinalizer("test").Account,
			},
		},
		{
			name: "delete-successful",
			fields: fields{
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Account,
				cc: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				ao: azurestoragefake.NewMockAccountOperations(),
			},
			want: want{
				err:  nil,
				res:  reconcile.Result{},
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).Account,
			},
		},
		{
			name: "delete-failed",
			fields: fields{
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Account,
				cc: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				ao: &azurestoragefake.MockAccountOperations{
					MockDelete: func(ctx context.Context) error {
						return errors.New("test-delete-error")
					},
				},
			},
			want: want{
				err: nil,
				res: resultRequeue,
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).
					withFailedCondition(failedToDelete, "test-delete-error").
					Account,
			},
		},
		{
			name: "delete-non-existent",
			fields: fields{
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Account,
				cc: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				ao: &azurestoragefake.MockAccountOperations{
					MockDelete: func(ctx context.Context) error {
						return autorest.DetailedError{
							StatusCode: http.StatusNotFound,
						}
					},
				},
			},
			want: want{
				err:  nil,
				res:  reconcile.Result{},
				acct: newAccount(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := newAccountSyncDeleter(tt.fields.ao, tt.fields.cc, tt.fields.acct)
			got, err := bh.delete(ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("accountSyncDeleter.delete() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("accountSyncDeleter.delete() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.acct, tt.want.acct); diff != nil {
				t.Errorf("accountSyncDeleter.delete() account = \n%+v, wantObj \n%+v\n%s", tt.fields.acct, tt.want.acct, diff)
				return
			}
		})
	}
}

func Test_syncdeleter_sync(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testAccountName
	type fields struct {
		ao   azurestorage.AccountOperations
		kube client.Client
		acct *v1alpha1.Account
	}
	type want struct {
		err  error
		res  reconcile.Result
		acct *v1alpha1.Account
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "attrs error",
			fields: fields{
				ao: &azurestoragefake.MockAccountOperations{
					MockGet: func(i context.Context) (attrs *storage.Account, e error) {
						return nil, errors.WithStack(errors.New("test-attrs-error"))
					},
				},
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				acct: newAccount(ns, name).withUID("test-uid").Account,
			},
			want: want{
				res:  resultRequeue,
				acct: newAccount(ns, name).withUID("test-uid").withFailedCondition(failedToRetrieve, "test-attrs-error").Account,
			},
		},
		{
			name: "attrs not found (create)",
			fields: fields{
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				ao: &azurestoragefake.MockAccountOperations{
					MockGet: func(i context.Context) (attrs *storage.Account, e error) {
						return nil, autorest.DetailedError{
							StatusCode: http.StatusNotFound,
						}
					},
				},
				acct: newAccount(ns, name).withUID("test-uid").Account,
			},
			want: want{
				res:  requeueOnSuccess,
				acct: newAccount(ns, name).withUID("test-uid").Account,
			},
		},
		{
			name: "update",
			fields: fields{
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				ao: &azurestoragefake.MockAccountOperations{
					MockGet: func(i context.Context) (attrs *storage.Account, e error) {
						return &storage.Account{}, nil
					},
				},
				acct: newAccount(ns, name).withUID("test-uid").Account,
			},
			want: want{
				res:  requeueOnSuccess,
				acct: newAccount(ns, name).withUID("test-uid").Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &accountSyncDeleter{
				createupdater:     newMockAccountCreateUpdater(),
				AccountOperations: tt.fields.ao,
				kube:              tt.fields.kube,
				acct:              tt.fields.acct,
			}

			got, err := bh.sync(ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("accountSyncDeleter.delete() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("accountSyncDeleter.delete() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.acct, tt.want.acct); diff != nil {
				t.Errorf("accountSyncDeleter.delete() account = \n%+v, wantObj \n%+v\n%s", tt.fields.acct, tt.want.acct, diff)
				return
			}
		})
	}
}

func Test_createupdater_create(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testAccountName
	type fields struct {
		sb        syncbacker
		ao        azurestorage.AccountOperations
		kube      client.Client
		acct      *v1alpha1.Account
		projectID string
	}
	type want struct {
		err error
		res reconcile.Result
		obj *v1alpha1.Account
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "create failed",
			fields: fields{
				ao: &azurestoragefake.MockAccountOperations{
					MockCreate: func(ctx context.Context, params storage.AccountCreateParameters) (*storage.Account, error) {
						return nil, errors.New("test-create-error")
					},
				},
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				acct: newAccount(ns, name).Account,
			},
			want: want{
				res: resultRequeue,
				obj: newAccount(ns, name).
					withFailedCondition(failedToCreate, "test-create-error").
					withFinalizer(finalizer).
					Account,
			},
		},
		{
			name: "create successful",
			fields: fields{
				sb: &MockAccountSyncbacker{
					MockSyncback: func(ctx context.Context, a *storage.Account) (result reconcile.Result, e error) {
						return resultRequeue, errors.New("test-syncback-error")
					},
				},
				ao:   azurestoragefake.NewMockAccountOperations(),
				kube: test.NewMockClient(),
				acct: newAccount(ns, name).Account,
			},
			want: want{
				err: errors.New("test-syncback-error"),
				res: resultRequeue,
				obj: newAccount(ns, name).
					withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
					withFinalizer(finalizer).
					Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &accountCreateUpdater{
				syncbacker:        tt.fields.sb,
				AccountOperations: tt.fields.ao,
				kube:              tt.fields.kube,
				acct:              tt.fields.acct,
				projectID:         tt.fields.projectID,
			}
			got, err := bh.create(ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("accountCreateUpdater.create() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("accountCreateUpdater.create() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.acct, tt.want.obj); diff != nil {
				t.Errorf("accountCreateUpdater.create() account = \n%+v, wantObj \n%+v\n%s", tt.fields.acct, tt.want.obj, diff)
				return
			}
		})
	}
}

func Test_bucketCreateUpdater_update(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testAccountName

	type fields struct {
		sb   syncbacker
		ao   azurestorage.AccountOperations
		kube client.Client
		acct *v1alpha1.Account
	}
	type want struct {
		res  reconcile.Result
		err  error
		acct *v1alpha1.Account
	}
	tests := []struct {
		name   string
		fields fields
		attrs  *storage.Account
		want   want
	}{
		{
			name: "not ready",
			attrs: newStorageAccount().
				withAccountProperties(newStorageAccountProperties().
					withProvisioningStage(storage.Creating).AccountProperties).Account,
			fields: fields{
				sb: &MockAccountSyncbacker{
					MockSyncback: func(ctx context.Context, a *storage.Account) (result reconcile.Result, e error) {
						return requeueOnSuccess, nil
					},
				},
			},
			want: want{
				res: requeueOnSuccess,
			},
		},
		{
			name: "no changes",
			attrs: &storage.Account{
				AccountProperties: &storage.AccountProperties{ProvisioningState: storage.Succeeded},
			},
			fields: fields{
				acct: newAccount(ns, name).withStorageAccountSpec(newStoragAccountSpecWithProperties()).Account,
			},
			want: want{
				res:  requeueOnSuccess,
				acct: newAccount(ns, name).withStorageAccountSpec(newStoragAccountSpecWithProperties()).Account,
			},
		},
		{
			name: "update failed",
			attrs: &storage.Account{
				AccountProperties: &storage.AccountProperties{ProvisioningState: storage.Succeeded},
				Location:          to.StringPtr("test-location"),
			},
			fields: fields{
				acct: newAccount(ns, name).withStorageAccountSpec(newStoragAccountSpecWithProperties()).Account,
				ao: &azurestoragefake.MockAccountOperations{
					MockUpdate: func(ctx context.Context, update storage.AccountUpdateParameters) (attrs *storage.Account, e error) {
						return nil, errors.New("test-account-update-error")
					},
				},
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
			},
			want: want{
				res: resultRequeue,
				acct: newAccount(ns, name).
					withStorageAccountSpec(newStoragAccountSpecWithProperties()).
					withFailedCondition(failedToUpdate, "test-account-update-error").Account,
			},
		},
		{
			name: "update success",
			attrs: &storage.Account{
				AccountProperties: &storage.AccountProperties{ProvisioningState: storage.Succeeded},
				Location:          to.StringPtr("test-location"),
			},
			fields: fields{
				sb: &MockAccountSyncbacker{
					MockSyncback: func(ctx context.Context, a *storage.Account) (result reconcile.Result, e error) {
						return requeueOnSuccess, nil
					},
				},
				acct: newAccount(ns, name).withStorageAccountSpec(newStoragAccountSpecWithProperties()).Account,
				ao: &azurestoragefake.MockAccountOperations{
					MockUpdate: func(ctx context.Context, update storage.AccountUpdateParameters) (attrs *storage.Account, e error) {
						return &storage.Account{Location: to.StringPtr("test-location")}, nil
					},
				},
				kube: test.NewMockClient(),
			},
			want: want{
				res:  requeueOnSuccess,
				acct: newAccount(ns, name).withStorageAccountSpec(newStoragAccountSpecWithProperties()).Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &accountCreateUpdater{
				syncbacker:        tt.fields.sb,
				AccountOperations: tt.fields.ao,
				kube:              tt.fields.kube,
				acct:              tt.fields.acct,
			}
			got, err := bh.update(ctx, tt.attrs)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("accountCreateUpdater.update() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("accountCreateUpdater.update() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.acct, tt.want.acct); diff != nil {
				t.Errorf("accountCreateUpdater.update() account = \n%+v, wantObj \n%+v\n%s", tt.fields.acct, tt.want.acct, diff)
				return
			}
		})
	}
}

func Test_accountSyncBacker_syncback(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testAccountName
	type fields struct {
		secretupdater secretupdater
		kube          client.Client
		acct          *v1alpha1.Account
	}
	type want struct {
		res  reconcile.Result
		err  error
		acct *v1alpha1.Account
	}
	tests := []struct {
		name   string
		fields fields
		acct   *storage.Account
		want   want
	}{
		{
			name: "update-failed",
			fields: fields{
				secretupdater: &MockAccountSecretupdater{},
				acct:          newAccount(ns, name).Account,
				kube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-spec-update-error")
					},
				},
			},
			acct: &storage.Account{},
			want: want{
				err:  errors.New("test-spec-update-error"),
				res:  resultRequeue,
				acct: newAccount(ns, name).withStorageAccountSpec(newStorageAccountSpec()).Account,
			},
		},
		{
			name: "provision status is not succeeded",
			fields: fields{
				acct: newAccount(ns, name).Account,
				kube: test.NewMockClient(),
			},
			acct: newStorageAccount().
				withAccountProperties(newStorageAccountProperties().
					withProvisioningStage(storage.Creating).AccountProperties).Account,
			want: want{
				res: requeueOnWait,
				acct: newAccount(ns, name).withSpecStatusFromProperties(&storage.AccountProperties{
					ProvisioningState: storage.Creating,
				}).Account,
			},
		},
		{
			name: "update-secret-failed",
			fields: fields{
				secretupdater: &MockAccountSecretupdater{
					MockUpdateSecret: func(ctx context.Context, a *storage.Account) error {
						return errors.New("test-secret-update-error")
					},
				},
				acct: newAccount(ns, name).Account,
				kube: test.NewMockClient(),
			},
			acct: &storage.Account{AccountProperties: &storage.AccountProperties{ProvisioningState: storage.Succeeded}},
			want: want{
				res: resultRequeue,
				acct: newAccount(ns, name).
					withSpecStatusFromProperties(&storage.AccountProperties{ProvisioningState: storage.Succeeded}).
					withFailedCondition(failedToSaveSecret, "test-secret-update-error").
					Account,
			},
		},
		{
			name: "success",
			fields: fields{
				secretupdater: &MockAccountSecretupdater{
					MockUpdateSecret: func(ctx context.Context, a *storage.Account) error { return nil },
				},
				acct: newAccount(ns, name).withStorageAccountSpec(v1alpha1.NewStorageAccountSpec(&storage.Account{})).Account,
				kube: test.NewMockClient(),
			},
			acct: &storage.Account{AccountProperties: &storage.AccountProperties{ProvisioningState: storage.Succeeded}},
			want: want{
				res: requeueOnSuccess,
				acct: newAccount(ns, name).
					withSpecStatusFromProperties(&storage.AccountProperties{ProvisioningState: storage.Succeeded}).
					Account,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acu := &accountSyncbacker{
				secretupdater: tt.fields.secretupdater,
				kube:          tt.fields.kube,
				acct:          tt.fields.acct,
			}
			got, err := acu.syncback(ctx, tt.acct)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("accountSyncBackSecretUpdater.syncback() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("accountSyncBackSecretUpdater.syncback() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.acct, tt.want.acct); diff != nil {
				t.Errorf("accountSyncBackSecretUpdater.syncback() account = \n%+v, wantObj \n%+v\n%s", tt.fields.acct, tt.want.acct, diff)
				return
			}
		})
	}
}

func Test_accountSecretUpdater_updatesecret(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testAccountName
	type fields struct {
		ops  azurestorage.AccountOperations
		kube client.Client
		acct *v1alpha1.Account
	}

	tests := []struct {
		name    string
		fields  fields
		acct    *storage.Account
		wantErr error
	}{
		{
			name: "failed-list-keys",
			fields: fields{
				ops: &azurestoragefake.MockAccountOperations{
					MockListKeys: func(ctx context.Context) (keys []storage.AccountKey, e error) {
						return nil, errors.New("test-list-keys-error")
					},
				},
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: azurev1alpha1.Group, Resource: "secret"}, name)
					},
				},
				acct: newAccount(ns, name).Account,
			},
			acct: &storage.Account{
				AccountProperties: &storage.AccountProperties{
					PrimaryEndpoints: &storage.Endpoints{
						Blob: to.StringPtr("test-blob-endpoint"),
					},
				},
			},
			wantErr: errors.Wrapf(errors.New("test-list-keys-error"), "failed to list account keys"),
		},
		{
			name: "account-keys-list-empty",
			fields: fields{
				ops: &azurestoragefake.MockAccountOperations{
					MockListKeys: func(ctx context.Context) (keys []storage.AccountKey, e error) {
						return []storage.AccountKey{}, nil
					},
				},
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: azurev1alpha1.Group, Resource: "secret"}, name)
					},
				},
				acct: newAccount(ns, name).Account,
			},
			acct: &storage.Account{
				AccountProperties: &storage.AccountProperties{
					PrimaryEndpoints: &storage.Endpoints{
						Blob: to.StringPtr("test-blob-endpoint"),
					},
				},
			},
			wantErr: errors.New("account keys are empty"),
		},
		{
			name: "create-new-secret",
			fields: fields{
				ops: &azurestoragefake.MockAccountOperations{
					MockListKeys: func(ctx context.Context) (keys []storage.AccountKey, e error) {
						return []storage.AccountKey{
							{
								KeyName: to.StringPtr("test-key"),
								Value:   to.StringPtr("test-value"),
							},
						}, nil
					},
				},
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: azurev1alpha1.Group, Resource: "secret"}, name)
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				acct: newAccount(ns, name).Account,
			},
			acct: &storage.Account{
				AccountProperties: &storage.AccountProperties{
					PrimaryEndpoints: &storage.Endpoints{
						Blob: to.StringPtr("test-blob-endpoint"),
					},
				},
			},
		},
		{
			name: "create-new-secret-failed",
			fields: fields{
				ops: &azurestoragefake.MockAccountOperations{
					MockListKeys: func(ctx context.Context) (keys []storage.AccountKey, e error) {
						return []storage.AccountKey{
							{
								KeyName: to.StringPtr("test-key"),
								Value:   to.StringPtr("test-value"),
							},
						}, nil
					},
				},
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: azurev1alpha1.Group, Resource: "secret"}, name)
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-create-secret-error")
					},
				},
				acct: newAccount(ns, name).Account,
			},
			acct: &storage.Account{
				AccountProperties: &storage.AccountProperties{
					PrimaryEndpoints: &storage.Endpoints{
						Blob: to.StringPtr("test-blob-endpoint"),
					},
				},
			},
			wantErr: errors.Wrapf(errors.New("test-create-secret-error"), "failed to create secret: %s/%s", ns, name),
		},
		{
			name: "update-existing-secret",
			fields: fields{
				ops: &azurestoragefake.MockAccountOperations{
					MockListKeys: func(ctx context.Context) (keys []storage.AccountKey, e error) {
						return []storage.AccountKey{
							{
								KeyName: to.StringPtr("test-key"),
								Value:   to.StringPtr("test-value"),
							},
						}, nil
					},
				},
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return nil
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return kerrors.NewAlreadyExists(schema.GroupResource{Group: azurev1alpha1.Group, Resource: "secret"}, name)
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				acct: newAccount(ns, name).Account,
			},
			acct: &storage.Account{
				AccountProperties: &storage.AccountProperties{
					PrimaryEndpoints: &storage.Endpoints{
						Blob: to.StringPtr("test-blob-endpoint"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asu := &accountSecretUpdater{
				AccountOperations: tt.fields.ops,
				acct:              tt.fields.acct,
				kube:              tt.fields.kube,
			}
			err := asu.updatesecret(ctx, tt.acct)
			if diff := deep.Equal(err, tt.wantErr); diff != nil {
				t.Errorf("accountSyncBackSecretUpdater.syncback() error = %v, wantErr %v\n%s", err, tt.wantErr, diff)
				return
			}
		})
	}
}

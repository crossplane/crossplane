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
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/go-cmp/cmp"
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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/storage/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

func init() {
	_ = gcp.AddToScheme(scheme.Scheme)
}

type MockBucketCreateUpdater struct {
	MockCreate func(context.Context) (reconcile.Result, error)
	MockUpdate func(context.Context, *storage.BucketAttrs) (reconcile.Result, error)
}

func (m *MockBucketCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	return m.MockCreate(ctx)
}

func (m *MockBucketCreateUpdater) update(ctx context.Context, a *storage.BucketAttrs) (reconcile.Result, error) {
	return m.MockUpdate(ctx, a)
}

var _ createupdater = &MockBucketCreateUpdater{}

type MockBucketSyncDeleter struct {
	MockDelete func(context.Context) (reconcile.Result, error)
	MockSync   func(context.Context) (reconcile.Result, error)
}

func newMockBucketSyncDeleter() *MockBucketSyncDeleter {
	return &MockBucketSyncDeleter{
		MockSync: func(i context.Context) (result reconcile.Result, e error) {
			return requeueOnSuccess, nil
		},
		MockDelete: func(i context.Context) (result reconcile.Result, e error) {
			return result, nil
		},
	}
}

func (m *MockBucketSyncDeleter) delete(ctx context.Context) (reconcile.Result, error) {
	return m.MockDelete(ctx)
}

func (m *MockBucketSyncDeleter) sync(ctx context.Context) (reconcile.Result, error) {
	return m.MockSync(ctx)
}

var _ syncdeleter = &MockBucketSyncDeleter{}

type MockBucketFactory struct {
	MockNew func(context.Context, *v1alpha1.Bucket) (syncdeleter, error)
}

func newMockBucketFactory(rh syncdeleter, err error) *MockBucketFactory {
	return &MockBucketFactory{
		MockNew: func(i context.Context, bucket *v1alpha1.Bucket) (handler syncdeleter, e error) {
			return rh, err
		},
	}
}

func (m *MockBucketFactory) newSyncDeleter(ctx context.Context, b *v1alpha1.Bucket) (syncdeleter, error) {
	return m.MockNew(ctx, b)
}

type bucket struct {
	*v1alpha1.Bucket
}

func newBucket(ns, name string) *bucket {
	return &bucket{Bucket: &v1alpha1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  ns,
			Name:       name,
			Finalizers: []string{},
		},
	}}
}

func (b *bucket) withUID(uid string) *bucket {
	b.ObjectMeta.UID = types.UID(uid)
	return b
}

func (b *bucket) withServiceAccountSecretRef(name string) *bucket {
	b.Spec.ServiceAccountSecretRef = &corev1.LocalObjectReference{Name: name}
	return b
}

func (b *bucket) withFailedCondition(reason, msg string) *bucket {
	b.Status.SetFailed(reason, msg)
	return b
}

func (b *bucket) withDeleteTimestamp(t metav1.Time) *bucket {
	b.Bucket.ObjectMeta.DeletionTimestamp = &t
	return b
}

func (b *bucket) withFinalizer(f string) *bucket {
	b.Bucket.ObjectMeta.Finalizers = append(b.Bucket.ObjectMeta.Finalizers, f)
	return b
}

func (b *bucket) withProvider(name string) *bucket {
	b.Spec.ProviderRef = corev1.LocalObjectReference{Name: name}
	return b
}

type provider struct {
	*gcpv1alpha1.Provider
}

func newProvider(ns, name string) *provider {
	return &provider{Provider: &gcpv1alpha1.Provider{
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

func assertFailure(t *testing.T, gotRsn, gotMsg, wantRsn, wantMsg string) {
	if gotRsn != wantRsn {
		t.Errorf("bucketSyncDeleter.sync() fail reason = %s, want %s", gotRsn, wantRsn)
	}
	if gotMsg != wantMsg {
		t.Errorf("bucketSyncDeleter.sync() fail msg = %s, want %s", gotMsg, wantMsg)
	}
}

const (
	testNamespace  = "default"
	testBucketName = "testBucket"
)

func TestReconciler_Reconcile(t *testing.T) {
	ns := testNamespace
	name := testBucketName
	key := types.NamespacedName{Namespace: ns, Name: name}
	req := reconcile.Request{NamespacedName: key}
	ctx := context.TODO()
	rsDone := reconcile.Result{}

	type fields struct {
		client  client.Client
		factory factory
	}
	tests := []struct {
		name    string
		fields  fields
		wantRs  reconcile.Result
		wantErr error
		wantObj *v1alpha1.Bucket
	}{
		{
			name:    "GetErrNotFound",
			fields:  fields{fake.NewFakeClient(), nil},
			wantRs:  rsDone,
			wantErr: nil,
		},
		{
			name: "GetErrorOther",
			fields: fields{
				client: &test.MockClient{
					MockGet: func(context.Context, client.ObjectKey, runtime.Object) error {
						return errors.New("test-get-error")
					},
				},
				factory: nil},
			wantRs:  rsDone,
			wantErr: errors.New("test-get-error"),
		},
		{
			name: "BucketHandlerError",
			fields: fields{
				client:  fake.NewFakeClient(newBucket(ns, name).withFinalizer("foo.bar").Bucket),
				factory: newMockBucketFactory(nil, errors.New("handler-factory-error")),
			},
			wantRs:  resultRequeue,
			wantErr: nil,
			wantObj: newBucket(ns, name).
				withFailedCondition(failedToGetHandler, "handler-factory-error").
				withFinalizer("foo.bar").Bucket,
		},
		{
			name: "ReconcileDelete",
			fields: fields{
				client: fake.NewFakeClient(newBucket(ns, name).
					withDeleteTimestamp(metav1.NewTime(time.Now())).Bucket),
				factory: newMockBucketFactory(newMockBucketSyncDeleter(), nil),
			},
			wantRs:  rsDone,
			wantErr: nil,
		},
		{
			name: "ReconcileSync",
			fields: fields{
				client:  fake.NewFakeClient(newBucket(ns, name).Bucket),
				factory: newMockBucketFactory(newMockBucketSyncDeleter(), nil),
			},
			wantRs:  requeueOnSuccess,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Client:  tt.fields.client,
				factory: tt.fields.factory,
			}
			got, err := r.Reconcile(req)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("Reconciler.Reconcile() error = %v, wantErr %v\n%s", err, tt.wantErr, diff)
				return
			}
			if diff := cmp.Diff(got, tt.wantRs); diff != "" {
				t.Errorf("Reconciler.Reconcile() result = %v, wantRs %v\n%s", got, tt.wantRs, diff)
			}
			if tt.wantObj != nil {
				b := &v1alpha1.Bucket{}
				if err := r.Get(ctx, key, b); err != nil {
					t.Errorf("Reconciler.Reconcile() bucket error: %s", err)
				}
				if diff := cmp.Diff(b, tt.wantObj); diff != "" {
					t.Errorf("Reconciler.Reconcile() bucket = \n%+v, wantObj \n%+v\n%s", b, tt.wantObj, diff)
				}
			}
		})
	}
}

func Test_bucketFactory_newHandler(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	bucketName := testBucketName
	providerName := "test-provider"
	secretName := "test-secret"
	secretKey := "creds"
	secretData := `{
	"type": "service_account",
	"project_id": "%s",
	"private_key_id": "%s",
	"private_key": "-----BEGIN PRIVATE KEY-----\n%s\n-----END PRIVATE KEY-----\n",
	"client_email": "%s",
	"client_id": "%s",
	"auth_uri": "https://accounts.google.com/bucket/oauth2/auth",
	"token_uri": "https://oauth2.googleapis.com/token",
	"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	"client_x509_cert_url": "%s"}`
	type want struct {
		err error
		sd  syncdeleter
	}
	tests := []struct {
		name   string
		Client client.Client
		bucket *v1alpha1.Bucket
		want   want
	}{
		{
			name:   "ErrProviderIsNotFound",
			Client: fake.NewFakeClient(),
			bucket: newBucket(ns, bucketName).withProvider(providerName).Bucket,
			want: want{
				err: kerrors.NewNotFound(schema.GroupResource{
					Group:    gcpv1alpha1.Group,
					Resource: "providers"}, "test-provider"),
			},
		},
		{
			name: "ProviderIsNotReady",
			Client: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, "", "")).Provider),
			bucket: newBucket(ns, bucketName).withProvider("test-provider").Bucket,
			want: want{
				err: errors.Errorf("provider: %s is not ready", ns+"/test-provider"),
			},
		},
		{
			name: "ProviderSecretIsNotFound",
			Client: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
				withSecret(secretName, secretKey).Provider),
			bucket: newBucket(ns, bucketName).withProvider("test-provider").Bucket,
			want: want{
				err: errors.WithStack(
					errors.Errorf("cannot get provider's secret %s/%s: secrets \"%s\" not found", ns, secretName, secretName)),
			},
		},
		{
			name: "InvalidCredentials",
			Client: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
				withSecret(secretName, secretKey).Provider,
				newSecret(ns, secretName).Secret),
			bucket: newBucket(ns, bucketName).withProvider("test-provider").Bucket,
			want: want{
				err: errors.WithStack(
					errors.Errorf("cannot retrieve creds from json: unexpected end of JSON input")),
			},
		},
		{
			name: "Successful",
			Client: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
				withSecret(secretName, secretKey).Provider,
				newSecret(ns, secretName).withKeyData(secretKey, secretData).Secret),
			bucket: newBucket(ns, bucketName).withUID("test-uid").withProvider("test-provider").Bucket,
			want: want{
				sd: newBucketSyncDeleter(
					newBucketClients(
						newBucket(ns, bucketName).withUID("test-uid").withProvider("test-provider").Bucket,
						nil, nil), ""),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &bucketFactory{
				Client: tt.Client,
			}
			got, err := m.newSyncDeleter(ctx, tt.bucket)
			if diff := cmp.Diff(err, tt.want.err); diff != "" {
				t.Errorf("bucketFactory.newSyncDeleter() error = \n%v, wantErr: \n%v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.sd); diff != "" {
				t.Errorf("bucketFactory.newSyncDeleter() = \n%+v, want \n%+v\n%s", got, tt.want.sd, diff)
			}
		})
	}
}

func Test_bucketSyncDeleter_delete(t *testing.T) {
	ctx := context.TODO()
	type fields struct {
		ops operations
	}
	type want struct {
		err error
		res reconcile.Result
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "RetainPolicy",
			fields: fields{
				ops: &mockOperations{
					mockIsReclaimDelete: func() bool { return false },
					mockRemoveFinalizer: func() {},
					mockUpdateObject:    func(ctx context.Context) error { return nil },
				},
			},
			want: want{
				res: reconcile.Result{},
			},
		},
		{
			name: "DeleteSuccessful",
			fields: fields{
				ops: &mockOperations{
					mockIsReclaimDelete: func() bool { return true },
					mockDeleteBucket:    func(ctx context.Context) error { return nil },
					mockRemoveFinalizer: func() {},
					mockUpdateObject:    func(ctx context.Context) error { return nil },
				},
			},
			want: want{
				err: nil,
				res: reconcile.Result{},
			},
		},
		{
			name: "DeleteFailedNotFound",
			fields: fields{
				ops: &mockOperations{
					mockIsReclaimDelete: func() bool { return true },
					mockDeleteBucket: func(ctx context.Context) error {
						return storage.ErrBucketNotExist
					},
					mockRemoveFinalizer: func() {},
					mockUpdateObject:    func(ctx context.Context) error { return nil },
				},
			},
			want: want{
				err: nil,
				res: reconcile.Result{},
			},
		},
		{
			name: "DeleteFailedOther",
			fields: fields{
				ops: &mockOperations{
					mockIsReclaimDelete: func() bool { return true },
					mockDeleteBucket: func(ctx context.Context) error {
						return errors.New("test-error")
					},
					mockFailReconcile: func(ctx context.Context, reason, msg string) error {
						return nil
					},
				},
			},
			want: want{
				res: resultRequeue,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bsd := newBucketSyncDeleter(tt.fields.ops, "")
			got, err := bsd.delete(ctx)
			if diff := cmp.Diff(err, tt.want.err); diff != "" {
				t.Errorf("bucketSyncDeleter.delete() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("bucketSyncDeleter.delete() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
		})
	}
}

func Test_bucketSyncDeleter_sync(t *testing.T) {
	ctx := context.TODO()

	secretError := errors.New("test-update-secret-error")
	bucket404 := storage.ErrBucketNotExist
	getAttrsError := errors.New("test-get-attributes-error")

	type fields struct {
		ops operations
		cu  createupdater
	}
	type want struct {
		err error
		res reconcile.Result
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "FailedToUpdateConnectionSecret",
			fields: fields{
				ops: &mockOperations{
					mockUpdateSecret: func(ctx context.Context) error { return secretError },
					mockFailReconcile: func(ctx context.Context, reason, msg string) error {
						assertFailure(t, reason, msg, failedToUpdateSecret, secretError.Error())
						return nil
					},
				},
			},
			want: want{res: resultRequeue},
		},
		{
			name: "AttrsErrorOther",
			fields: fields{
				ops: &mockOperations{
					mockUpdateSecret: func(ctx context.Context) error { return nil },
					mockGetAttributes: func(ctx context.Context) (*storage.BucketAttrs, error) {
						return nil, getAttrsError
					},
					mockFailReconcile: func(ctx context.Context, reason, msg string) error {
						assertFailure(t, reason, msg, failedToRetrieve, getAttrsError.Error())
						return nil
					},
				},
			},
			want: want{res: resultRequeue},
		},
		{
			name: "CreateBucket",
			fields: fields{
				ops: &mockOperations{
					mockUpdateSecret: func(ctx context.Context) error { return nil },
					mockGetAttributes: func(ctx context.Context) (*storage.BucketAttrs, error) {
						return nil, bucket404
					},
				},
				cu: &MockBucketCreateUpdater{
					MockCreate: func(ctx context.Context) (reconcile.Result, error) {
						return reconcile.Result{}, nil
					},
				},
			},
			want: want{res: reconcile.Result{}},
		},
		{
			name: "UpdateBucket",
			fields: fields{
				ops: &mockOperations{
					mockUpdateSecret: func(ctx context.Context) error { return nil },
					mockGetAttributes: func(ctx context.Context) (*storage.BucketAttrs, error) {
						return &storage.BucketAttrs{}, bucket404
					},
				},
				cu: &MockBucketCreateUpdater{
					MockUpdate: func(ctx context.Context, attrs *storage.BucketAttrs) (reconcile.Result, error) {
						return requeueOnSuccess, nil
					},
				},
			},
			want: want{res: requeueOnSuccess},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &bucketSyncDeleter{
				operations:    tt.fields.ops,
				createupdater: tt.fields.cu,
			}

			got, err := bh.sync(ctx)
			if diff := cmp.Diff(err, tt.want.err); diff != "" {
				t.Errorf("bucketSyncDeleter.sync() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("bucketSyncDeleter.sync() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
		})
	}
}

func Test_bucketCreateUpdater_create(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")

	type fields struct {
		ops       operations
		projectID string
	}
	type want struct {
		err error
		res reconcile.Result
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "FailureToCreate",
			fields: fields{
				ops: &mockOperations{
					mockAddFinalizer: func() {},
					mockCreateBucket: func(ctx context.Context, projectID string) error { return testError },
					mockFailReconcile: func(ctx context.Context, reason, msg string) error {
						assertFailure(t, reason, msg, failedToCreate, testError.Error())
						return nil
					},
				},
			},
			want: want{
				res: resultRequeue,
			},
		},
		{
			name: "FailureToGetAttributes",
			fields: fields{
				ops: &mockOperations{
					mockAddFinalizer:  func() {},
					mockCreateBucket:  func(ctx context.Context, projectID string) error { return nil },
					mockSetReady:      func() {},
					mockGetAttributes: func(ctx context.Context) (*storage.BucketAttrs, error) { return nil, testError },
					mockFailReconcile: func(ctx context.Context, reason, msg string) error {
						assertFailure(t, reason, msg, failedToRetrieve, testError.Error())
						return nil
					},
				},
			},
			want: want{
				res: resultRequeue,
			},
		},
		{
			name: "FailureToUpdateObject",
			fields: fields{
				ops: &mockOperations{
					mockAddFinalizer:  func() {},
					mockCreateBucket:  func(ctx context.Context, projectID string) error { return nil },
					mockSetReady:      func() {},
					mockGetAttributes: func(ctx context.Context) (*storage.BucketAttrs, error) { return nil, nil },
					mockSetSpecAttrs:  func(attrs *storage.BucketAttrs) {},
					mockUpdateObject:  func(ctx context.Context) error { return testError },
				},
			},
			want: want{
				err: testError,
				res: resultRequeue,
			},
		},
		{
			name: "Success",
			fields: fields{
				ops: &mockOperations{
					mockAddFinalizer:   func() {},
					mockCreateBucket:   func(ctx context.Context, projectID string) error { return nil },
					mockSetReady:       func() {},
					mockGetAttributes:  func(ctx context.Context) (*storage.BucketAttrs, error) { return nil, nil },
					mockSetSpecAttrs:   func(attrs *storage.BucketAttrs) {},
					mockUpdateObject:   func(ctx context.Context) error { return nil },
					mockSetStatusAttrs: func(attrs *storage.BucketAttrs) {},
					mockUpdateStatus:   func(ctx context.Context) error { return nil },
				},
			},
			want: want{
				res: requeueOnSuccess,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &bucketCreateUpdater{
				operations: tt.fields.ops,
				projectID:  tt.fields.projectID,
			}
			got, err := bh.create(ctx)
			if diff := cmp.Diff(err, tt.want.err); diff != "" {
				t.Errorf("bucketCreateUpdater.create() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("bucketCreateUpdater.create() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
		})
	}
}

func Test_bucketCreateUpdater_update(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")

	type fields struct {
		ops       operations
		projectID string
	}
	type want struct {
		err error
		res reconcile.Result
	}
	tests := []struct {
		name   string
		fields fields
		args   *storage.BucketAttrs
		want   want
	}{
		{
			name: "NoChanges",
			fields: fields{
				ops: &mockOperations{
					mockGetSpecAttrs: func() v1alpha1.BucketUpdatableAttrs {
						return v1alpha1.BucketUpdatableAttrs{}
					},
				},
				projectID: "",
			},
			args: &storage.BucketAttrs{},
			want: want{res: requeueOnSuccess},
		},
		{
			name: "FailureToUpdateBucket",
			fields: fields{
				ops: &mockOperations{
					mockGetSpecAttrs: func() v1alpha1.BucketUpdatableAttrs {
						return v1alpha1.BucketUpdatableAttrs{RequesterPays: true}
					},
					mockUpdateBucket: func(ctx context.Context, labels map[string]string) (*storage.BucketAttrs, error) {
						return nil, testError
					},
					mockFailReconcile: func(ctx context.Context, reason, msg string) error {
						assertFailure(t, reason, msg, failedToUpdate, testError.Error())
						return nil
					},
				},
				projectID: "",
			},
			args: &storage.BucketAttrs{},
			want: want{res: resultRequeue},
		},
		{
			name: "FailureToUpdateObject",
			fields: fields{
				ops: &mockOperations{
					mockGetSpecAttrs: func() v1alpha1.BucketUpdatableAttrs {
						return v1alpha1.BucketUpdatableAttrs{RequesterPays: true}
					},
					mockUpdateBucket: func(ctx context.Context, labels map[string]string) (*storage.BucketAttrs, error) {
						return nil, nil
					},
					mockSetSpecAttrs: func(attrs *storage.BucketAttrs) {},
					mockUpdateObject: func(ctx context.Context) error { return testError },
				},
				projectID: "",
			},
			args: &storage.BucketAttrs{},
			want: want{
				err: testError,
				res: resultRequeue,
			},
		},
		{
			name: "Successful",
			fields: fields{
				ops: &mockOperations{
					mockGetSpecAttrs: func() v1alpha1.BucketUpdatableAttrs {
						return v1alpha1.BucketUpdatableAttrs{RequesterPays: true}
					},
					mockUpdateBucket: func(ctx context.Context, labels map[string]string) (*storage.BucketAttrs, error) {
						return nil, nil
					},
					mockSetSpecAttrs: func(attrs *storage.BucketAttrs) {},
					mockUpdateObject: func(ctx context.Context) error { return nil },
					mockUpdateStatus: func(ctx context.Context) error { return nil },
				},
				projectID: "",
			},
			args: &storage.BucketAttrs{},
			want: want{
				res: requeueOnSuccess,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &bucketCreateUpdater{
				operations: tt.fields.ops,
				projectID:  tt.fields.projectID,
			}
			got, err := bh.update(ctx, tt.args)
			if diff := cmp.Diff(err, tt.want.err); diff != "" {
				t.Errorf("bucketCreateUpdater.update() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("bucketCreateUpdater.update() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
		})
	}
}

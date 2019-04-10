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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/storage/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	gcpstorage "github.com/crossplaneio/crossplane/pkg/clients/gcp/storage"
	gcpstoragefake "github.com/crossplaneio/crossplane/pkg/clients/gcp/storage/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

func init() {
	_ = gcp.AddToScheme(scheme.Scheme)
}

type MockBucketCreateUpdater struct {
	MockCreate func(context.Context) (reconcile.Result, error)
	MockUpdate func(context.Context, *storage.BucketAttrs) (reconcile.Result, error)
}

func newMockBucketCreateUpdater() *MockBucketCreateUpdater {
	return &MockBucketCreateUpdater{
		MockUpdate: func(i context.Context, attrs *storage.BucketAttrs) (result reconcile.Result, e error) {
			return requeueOnSuccess, nil
		},
		MockCreate: func(i context.Context) (result reconcile.Result, e error) {
			return requeueOnSuccess, nil
		},
	}
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

func (m *MockBucketFactory) newHandler(ctx context.Context, b *v1alpha1.Bucket) (syncdeleter, error) {
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

func (b *bucket) withCondition(c corev1alpha1.Condition) *bucket {
	b.Status.ConditionedStatus.SetCondition(c)
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

func (b *bucket) withReclaimPolicy(policy corev1alpha1.ReclaimPolicy) *bucket {
	b.Spec.ReclaimPolicy = policy
	return b
}

func (b *bucket) withSpecRequesterPays(rp bool) *bucket {
	b.Spec.BucketSpecAttrs.RequesterPays = rp
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
			name:    "get err-not-found",
			fields:  fields{fake.NewFakeClient(), nil},
			wantRs:  rsDone,
			wantErr: nil,
		},
		{
			name: "get error other",
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
			name: "bucket handler error",
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
			name: "reconcile delete",
			fields: fields{
				client: fake.NewFakeClient(newBucket(ns, name).
					withDeleteTimestamp(metav1.NewTime(time.Now())).Bucket),
				factory: newMockBucketFactory(newMockBucketSyncDeleter(), nil),
			},
			wantRs:  rsDone,
			wantErr: nil,
		},
		{
			name: "reconcile sync",
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
			if diff := deep.Equal(err, tt.wantErr); diff != nil {
				t.Errorf("Reconciler.Reconcile() error = %v, wantErr %v\n%s", err, tt.wantErr, diff)
				return
			}
			if diff := deep.Equal(got, tt.wantRs); diff != nil {
				t.Errorf("Reconciler.Reconcile() result = %v, wantRs %v\n%s", got, tt.wantRs, diff)
			}
			if tt.wantObj != nil {
				b := &v1alpha1.Bucket{}
				if err := r.Get(ctx, key, b); err != nil {
					t.Errorf("Reconciler.Reconcile() bucket error: %s", err)
				}
				if diff := deep.Equal(b, tt.wantObj); diff != nil {
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
			name:   "err provider is not found",
			Client: fake.NewFakeClient(),
			bucket: newBucket(ns, bucketName).withProvider(providerName).Bucket,
			want: want{
				err: kerrors.NewNotFound(schema.GroupResource{
					Group:    gcpv1alpha1.Group,
					Resource: "providers"}, "test-provider"),
			},
		},
		{
			name: "provider is not ready",
			Client: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, "", "")).Provider),
			bucket: newBucket(ns, bucketName).withProvider("test-provider").Bucket,
			want: want{
				err: errors.Errorf("provider: %s is not ready", ns+"/test-provider"),
			},
		},
		{
			name: "provider secret is not found",
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
			name: "invalid credentials",
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
			name: "successful",
			Client: fake.NewFakeClient(newProvider(ns, providerName).
				withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
				withSecret(secretName, secretKey).Provider,
				newSecret(ns, secretName).withKeyData(secretKey, secretData).Secret),
			bucket: newBucket(ns, bucketName).withUID("test-uid").withProvider("test-provider").Bucket,
			want: want{
				sd: newBucketSyncDeleter(&gcpstorage.BucketClient{BucketHandle: &storage.BucketHandle{}}, nil, nil, ""),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &bucketFactory{
				Client: tt.Client,
			}
			got, err := m.newHandler(ctx, tt.bucket)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("bucketFactory.newHandler() error = \n%v, wantErr: \n%v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.sd); diff != nil {
				t.Errorf("bucketFactory.newHandler() = \n%+v, want \n%+v\n%s", got, tt.want.sd, diff)
			}
		})
	}
}

func Test_bucketHandler_delete(t *testing.T) {
	ctx := context.TODO()
	ns := "default"
	bucketName := "test-bucket"
	type fields struct {
		sc  gcpstorage.Client
		cc  client.Client
		obj *v1alpha1.Bucket
	}
	type want struct {
		err error
		res reconcile.Result
		obj *v1alpha1.Bucket
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "retain policy",
			fields: fields{
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimRetain).
					withFinalizer(finalizer).withFinalizer("test").Bucket,
				cc: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
			},
			want: want{
				err: nil,
				res: reconcile.Result{},
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimRetain).
					withFinalizer("test").Bucket,
			},
		},
		{
			name: "delete successful",
			fields: fields{
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Bucket,
				cc: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				sc: gcpstoragefake.NewMockBucketClient(),
			},
			want: want{
				err: nil,
				res: reconcile.Result{},
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).Bucket,
			},
		},
		{
			name: "delete failed",
			fields: fields{
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Bucket,
				cc: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				sc: &gcpstoragefake.MockBucketClient{
					MockDelete: func(ctx context.Context) error {
						return errors.New("test-delete-error")
					},
				},
			},
			want: want{
				err: nil,
				res: resultRequeue,
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).
					withFailedCondition(failedToDelete, "test-delete-error").
					Bucket,
			},
		},
		{
			name: "delete non-existent",
			fields: fields{
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Bucket,
				cc: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				sc: &gcpstoragefake.MockBucketClient{
					MockDelete: func(ctx context.Context) error {
						return storage.ErrBucketNotExist
					},
				},
			},
			want: want{
				err: nil,
				res: reconcile.Result{},
				obj: newBucket(ns, bucketName).withReclaimPolicy(corev1alpha1.ReclaimDelete).Bucket,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := newBucketSyncDeleter(tt.fields.sc, tt.fields.cc, tt.fields.obj, "")
			got, err := bh.delete(ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("bucketSyncDeleter.delete() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("bucketSyncDeleter.delete() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.obj, tt.want.obj); diff != nil {
				t.Errorf("bucketSyncDeleter.delete() bucket = \n%+v, wantObj \n%+v\n%s", tt.fields.obj, tt.want.obj, diff)
				return
			}
		})
	}
}

func Test_bucketHandler_sync(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testBucketName
	type fields struct {
		sc  gcpstorage.Client
		cc  client.Client
		obj *v1alpha1.Bucket
	}
	type want struct {
		err error
		res reconcile.Result
		obj *v1alpha1.Bucket
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "secret error",
			fields: fields{
				cc: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-error-saving-secret")
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				obj: newBucket(ns, name).Bucket,
			},
			want: want{
				err: nil,
				res: resultRequeue,
				obj: newBucket(ns, name).withFailedCondition(failedToSaveSecret, "test-error-saving-secret").Bucket,
			},
		},
		{
			name: "attrs error",
			fields: fields{
				sc: &gcpstoragefake.MockBucketClient{
					MockAttrs: func(i context.Context) (attrs *storage.BucketAttrs, e error) {
						return nil, errors.WithStack(errors.New("test-attrs-error"))
					},
				},
				cc: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
				obj: newBucket(ns, name).withUID("test-uid").Bucket,
			},
			want: want{
				err: nil,
				res: resultRequeue,
				obj: newBucket(ns, name).withUID("test-uid").withFailedCondition(failedToRetrieve, "test-attrs-error").Bucket,
			},
		},
		{
			name: "attrs not found (create)",
			fields: fields{
				cc: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				sc: &gcpstoragefake.MockBucketClient{
					MockAttrs: func(i context.Context) (attrs *storage.BucketAttrs, e error) {
						return nil, storage.ErrBucketNotExist
					},
				},
				obj: newBucket(ns, name).withUID("test-uid").Bucket,
			},
			want: want{
				err: nil,
				res: requeueOnSuccess,
				obj: newBucket(ns, name).withUID("test-uid").Bucket,
			},
		},
		{
			name: "update",
			fields: fields{
				cc: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				sc: &gcpstoragefake.MockBucketClient{
					MockAttrs: func(i context.Context) (attrs *storage.BucketAttrs, e error) {
						return &storage.BucketAttrs{}, nil
					},
				},
				obj: newBucket(ns, name).withUID("test-uid").Bucket,
			},
			want: want{
				err: nil,
				res: requeueOnSuccess,
				obj: newBucket(ns, name).withUID("test-uid").Bucket,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &bucketSyncDeleter{
				createupdater: newMockBucketCreateUpdater(),
				Client:        tt.fields.sc,
				kube:          tt.fields.cc,
				object:        tt.fields.obj,
			}

			got, err := bh.sync(ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("bucketSyncDeleter.delete() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("bucketSyncDeleter.delete() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.obj, tt.want.obj); diff != nil {
				t.Errorf("bucketSyncDeleter.delete() bucket = \n%+v, wantObj \n%+v\n%s", tt.fields.obj, tt.want.obj, diff)
				return
			}
		})
	}
}

func Test_bucketCreateUpdater_create(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testBucketName
	type fields struct {
		sc        gcpstorage.Client
		kube      client.Client
		bucket    *v1alpha1.Bucket
		projectID string
	}
	type want struct {
		err    error
		res    reconcile.Result
		bucket *v1alpha1.Bucket
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "create error",
			fields: fields{
				sc: &gcpstoragefake.MockBucketClient{
					MockCreate: func(ctx context.Context, pid string, attrs *storage.BucketAttrs) error {
						return errors.New("test-create-error")
					},
				},
				kube:   test.NewMockClient(),
				bucket: newBucket(ns, name).Bucket,
			},
			want: want{
				err: nil,
				res: resultRequeue,
				bucket: newBucket(ns, name).
					withFailedCondition(failedToCreate, "test-create-error").
					withFinalizer(finalizer).
					Bucket,
			},
		},
		{
			name: "create success, attrs error",
			fields: fields{
				sc: &gcpstoragefake.MockBucketClient{
					MockCreate: func(ctx context.Context, pid string, attrs *storage.BucketAttrs) error {
						return nil
					},
					MockAttrs: func(i context.Context) (attrs *storage.BucketAttrs, e error) {
						return nil, errors.New("test-attrs-error")
					},
				},
				kube:   test.NewMockClient(),
				bucket: newBucket(ns, name).Bucket,
			},
			want: want{
				err: nil,
				res: resultRequeue,
				bucket: newBucket(ns, name).
					withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
					withFailedCondition(failedToRetrieve, "test-attrs-error").
					withFinalizer(finalizer).
					Bucket,
			},
		},
		{
			name: "create success, update error",
			fields: fields{
				sc: gcpstoragefake.NewMockBucketClient(),
				kube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-update-error")
					},
				},
				bucket: newBucket(ns, name).Bucket,
			},
			want: want{
				err: errors.New("test-update-error"),
				res: resultRequeue,
				bucket: newBucket(ns, name).
					withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
					withFinalizer(finalizer).
					Bucket,
			},
		},
		{
			name: "create success",
			fields: fields{
				sc: gcpstoragefake.NewMockBucketClient(),
				kube: &test.MockClient{
					MockUpdate:       func(ctx context.Context, obj runtime.Object) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
				bucket: newBucket(ns, name).Bucket,
			},
			want: want{
				err: nil,
				res: requeueOnSuccess,
				bucket: newBucket(ns, name).
					withCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", "")).
					withFinalizer(finalizer).
					Bucket,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &bucketCreateUpdater{
				Client:    tt.fields.sc,
				kube:      tt.fields.kube,
				bucket:    tt.fields.bucket,
				projectID: tt.fields.projectID,
			}
			got, err := bh.create(ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("bucketCreateUpdater.create() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("bucketCreateUpdater.create() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if tt.want.bucket != nil {
				if diff := deep.Equal(tt.fields.bucket, tt.want.bucket); diff != nil {
					t.Errorf("bucketCreateUpdater.create() bucket = \n%+v, wantObj \n%+v\n%s", tt.fields.bucket, tt.want.bucket, diff)
					return
				}
			}
		})
	}
}

func Test_bucketCreateUpdater_update(t *testing.T) {
	ctx := context.TODO()
	ns := testNamespace
	name := testBucketName
	type fields struct {
		sc gcpstorage.Client
		cc client.Client
		o  *v1alpha1.Bucket
	}
	type want struct {
		res reconcile.Result
		err error
		obj *v1alpha1.Bucket
	}
	tests := []struct {
		name   string
		fields fields
		attrs  *storage.BucketAttrs
		want   want
	}{
		{
			name:  "no changes",
			attrs: &storage.BucketAttrs{},
			fields: fields{
				o: newBucket(ns, name).Bucket,
			},
			want: want{
				err: nil,
				res: requeueOnSuccess,
				obj: newBucket(ns, name).Bucket,
			},
		},
		{
			name:  "update failed",
			attrs: &storage.BucketAttrs{},
			fields: fields{
				o: newBucket(ns, name).withSpecRequesterPays(true).Bucket,
				sc: &gcpstoragefake.MockBucketClient{
					MockUpdate: func(ctx context.Context, update storage.BucketAttrsToUpdate) (attrs *storage.BucketAttrs, e error) {
						return nil, errors.New("test-bucket-update-error")
					},
				},
				cc: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				},
			},
			want: want{
				err: nil,
				res: resultRequeue,
				obj: newBucket(ns, name).withSpecRequesterPays(true).
					withFailedCondition(failedToUpdate, "test-bucket-update-error").Bucket,
			},
		},
		{
			name:  "update back failed",
			attrs: &storage.BucketAttrs{},
			fields: fields{
				o:  newBucket(ns, name).withSpecRequesterPays(true).Bucket,
				sc: gcpstoragefake.NewMockBucketClient(),
				cc: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-spec-update-error")
					},
				},
			},
			want: want{
				err: errors.New("test-spec-update-error"),
				res: resultRequeue,
				obj: newBucket(ns, name).withSpecRequesterPays(false).Bucket,
			},
		},
		{
			name:  "update success",
			attrs: &storage.BucketAttrs{},
			fields: fields{
				o:  newBucket(ns, name).withSpecRequesterPays(true).Bucket,
				sc: gcpstoragefake.NewMockBucketClient(),
				cc: test.NewMockClient(),
			},
			want: want{
				err: nil,
				res: requeueOnSuccess,
				obj: newBucket(ns, name).withSpecRequesterPays(false).Bucket,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bh := &bucketCreateUpdater{
				Client: tt.fields.sc,
				kube:   tt.fields.cc,
				bucket: tt.fields.o,
			}
			got, err := bh.update(ctx, tt.attrs)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("bucketCreateUpdater.update() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("bucketCreateUpdater.update() result = %v, wantRes %v\n%s", got, tt.want.res, diff)
				return
			}
			if diff := deep.Equal(tt.fields.o, tt.want.obj); diff != nil {
				t.Errorf("bucketSyncDeleter.delete() bucket = \n%+v, wantObj \n%+v\n%s", tt.fields.o, tt.want.obj, diff)
				return
			}
		})
	}
}

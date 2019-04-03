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

package container

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/go-test/deep"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
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
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/storage"
	azurestoragefake "github.com/crossplaneio/crossplane/pkg/clients/azure/storage/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

func init() {
	_ = azure.AddToScheme(scheme.Scheme)
}

type mockCreateUpdater struct {
	mockCreate func(context.Context) (reconcile.Result, error)
	mockUpdate func(context.Context, *azblob.PublicAccessType, azblob.Metadata) (reconcile.Result, error)
}

func (m *mockCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	return m.mockCreate(ctx)
}

func (m *mockCreateUpdater) update(ctx context.Context, pat *azblob.PublicAccessType, meta azblob.Metadata) (reconcile.Result, error) {
	return m.mockUpdate(ctx, pat, meta)
}

func newMockCreateUpdater() *mockCreateUpdater {
	return &mockCreateUpdater{
		mockCreate: func(context.Context) (result reconcile.Result, e error) {
			return reconcile.Result{}, nil
		},
		mockUpdate: func(context.Context, *azblob.PublicAccessType, azblob.Metadata) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		},
	}
}

var _ createupdater = &mockCreateUpdater{}

type mockSyncdeleter struct {
	mockDelete func(context.Context) (reconcile.Result, error)
	mockSync   func(context.Context) (reconcile.Result, error)
}

func (m *mockSyncdeleter) delete(ctx context.Context) (reconcile.Result, error) {
	return m.mockDelete(ctx)
}

func (m *mockSyncdeleter) sync(ctx context.Context) (reconcile.Result, error) {
	return m.mockSync(ctx)
}

var _ syncdeleter = &mockSyncdeleter{}

type mockSyncdeleteMaker struct {
	mockNewSyncdeleter func(context.Context, *v1alpha1.Container) (syncdeleter, error)
}

func (m *mockSyncdeleteMaker) newSyncdeleter(ctx context.Context, c *v1alpha1.Container) (syncdeleter, error) {
	return m.mockNewSyncdeleter(ctx, c)
}

var _ syncdeleterMaker = &mockSyncdeleteMaker{}

type container struct {
	*v1alpha1.Container
}

func newContainer(ns, name string) *container {
	return &container{
		Container: &v1alpha1.Container{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  ns,
				Name:       name,
				Finalizers: []string{},
			},
		},
	}
}

func (c *container) withAccountRef(name string) *container {
	c.Container.Spec.AccountRef = v1.LocalObjectReference{Name: name}
	return c
}

func (c *container) withDeleteTimestamp(t time.Time) *container {
	c.Container.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t}
	return c
}

func (c *container) withFailedCondition(reason, msg string) *container {
	c.Status.SetFailed(reason, msg)
	return c
}

func (c *container) withUnsetAllConditions() *container {
	c.Status.UnsetAllConditions()
	return c
}

func (c *container) withReadyCondition() *container {
	c.Status.SetReady()
	return c
}

func (c *container) withFinalizer(f string) *container {
	c.Container.ObjectMeta.Finalizers = append(c.Container.ObjectMeta.Finalizers, f)
	return c
}

func (c *container) withSpecNameFormat(f string) *container {
	c.Container.Spec.NameFormat = f
	return c
}

func (c *container) withReclaimPolicy(p corev1alpha1.ReclaimPolicy) *container {
	c.Container.Spec.ReclaimPolicy = p
	return c
}

func (c *container) withSpecPAC(pac azblob.PublicAccessType) *container {
	c.Container.Spec.PublicAccessType = pac
	return c
}

func newContainerNotFoundError(name string) error {
	return kerrors.NewNotFound(
		schema.GroupResource{Group: v1alpha1.Group, Resource: v1alpha1.ContainerKind}, name)
}

func newAccountNotFoundError(name string) error {
	return kerrors.NewNotFound(
		schema.GroupResource{Group: v1alpha1.Group, Resource: v1alpha1.AccountKind + "s"}, name)
}

func newSecret(ns, name string, data map[string][]byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Data: data,
	}
}

func newSecretNotFoundError(name string) error {
	return kerrors.NewNotFound(
		schema.GroupResource{Group: v1.GroupName, Resource: "secrets"}, name)
}

func newStorageNotFoundError() error {
	return azblob.NewResponseError(nil, &http.Response{StatusCode: http.StatusNotFound}, "")
}

const (
	testNamespace     = "default"
	testContainerName = "testContainer"
	testAccountName   = "testAccount"
)

func TestReconciler_Reconcile(t *testing.T) {
	key := types.NamespacedName{Namespace: testNamespace, Name: testContainerName}
	req := reconcile.Request{NamespacedName: key}
	ctx := context.TODO()

	type fields struct {
		Client           client.Client
		syncdeleterMaker syncdeleterMaker
	}
	type want struct {
		err error
		res reconcile.Result
		con *v1alpha1.Container
	}
	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		{
			name: "get err not-found",
			fields: fields{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return newContainerNotFoundError(testContainerName)
					},
				},
				syncdeleterMaker: nil,
			},
			want: want{
				res: reconcile.Result{},
			},
		},
		{
			name: "get err other",
			fields: fields{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errors.New("test-get-error")
					},
				},
				syncdeleterMaker: nil,
			},
			want: want{
				res: reconcile.Result{},
				err: errors.New("test-get-error"),
			},
		},
		{
			name: "syncdelete maker error",
			fields: fields{
				Client: fake.NewFakeClient(newContainer(testNamespace, testContainerName).
					withFinalizer("foo.bar").Container),
				syncdeleterMaker: &mockSyncdeleteMaker{
					mockNewSyncdeleter: func(ctx context.Context, c *v1alpha1.Container) (syncdeleter, error) {
						return nil, errors.New("test-new-syncdeleter-error")
					},
				},
			},
			want: want{
				res: resultRequeue,
				con: newContainer(testNamespace, testContainerName).
					withFinalizer("foo.bar").
					withFailedCondition(failedToGetHandler, "test-new-syncdeleter-error").
					Container,
			},
		},
		{
			name: "delete",
			fields: fields{
				Client: fake.NewFakeClient(newContainer(testNamespace, testContainerName).
					withDeleteTimestamp(time.Now()).Container),
				syncdeleterMaker: &mockSyncdeleteMaker{
					mockNewSyncdeleter: func(ctx context.Context, c *v1alpha1.Container) (syncdeleter, error) {
						return &mockSyncdeleter{
							mockDelete: func(ctx context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}, nil
					},
				},
			},
			want: want{
				res: reconcile.Result{},
			},
		},
		{
			name: "sync",
			fields: fields{
				Client: fake.NewFakeClient(newContainer(testNamespace, testContainerName).Container),
				syncdeleterMaker: &mockSyncdeleteMaker{
					mockNewSyncdeleter: func(ctx context.Context, c *v1alpha1.Container) (syncdeleter, error) {
						return &mockSyncdeleter{
							mockSync: func(ctx context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}, nil
					},
				},
			},
			want: want{
				res: reconcile.Result{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Client:           tt.fields.Client,
				syncdeleterMaker: tt.fields.syncdeleterMaker,
			}
			got, err := r.Reconcile(req)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("Reconciler.Reconcile() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("Reconciler.Reconcile() result = %v, wantRs %v\n%s", got, tt.want.res, diff)
			}
			if tt.want.con != nil {
				c := &v1alpha1.Container{}
				if err := tt.fields.Client.Get(ctx, key, c); err != nil {
					t.Errorf("Reconciler.Reconcile() container error: %s", err)
				}
				if diff := deep.Equal(c, tt.want.con); diff != nil {
					t.Errorf("Reconciler.Reconcile() container = \n%+v, wantObj \n%+v\n%s", c, tt.want.con, diff)
				}
			}
		})
	}
}

func Test_containerSyncdeleterMaker_newSyncdeleter(t *testing.T) {
	key := types.NamespacedName{Namespace: testNamespace, Name: testContainerName}
	newCont := func() *container {
		return newContainer(testNamespace, testContainerName)
	}
	ctx := context.TODO()
	testAccountKey := "dGVzdC1rZXkK"

	ch, err := storage.NewContainerHandle(testAccountName, testAccountKey, testContainerName)
	if err != nil {
		t.Errorf("containerSyncdeleterMaker.newSyncdeleter() unexpected error %v", err)
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx context.Context
		c   *v1alpha1.Container
	}
	type want struct {
		err    error
		syndel syncdeleter
		cont   *v1alpha1.Container
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name: "failed to get account - not found, no delete",
			fields: fields{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return newAccountNotFoundError(testAccountName)
					},
				},
			},
			args: args{
				ctx: ctx,
				c:   newCont().withAccountRef(testAccountName).withFinalizer(finalizer).Container,
			},
			want: want{
				err: errors.Wrapf(newAccountNotFoundError(testAccountName),
					"failed to retrieve storage account reference: %s/%s", testNamespace, testAccountName),
			},
		},
		{
			name: "failed to get account - not found, yes delete",
			fields: fields{
				Client: fake.NewFakeClient(newCont().withAccountRef(testAccountName).
					withFinalizer(finalizer).
					Container),
			},
			args: args{
				ctx: ctx,
				c: newCont().withAccountRef(testAccountName).withFinalizer(finalizer).
					withDeleteTimestamp(time.Now()).
					Container,
			},
			want: want{
				err: errors.Wrapf(newAccountNotFoundError(testAccountName),
					"failed to retrieve storage account reference: %s/%s", testNamespace, testAccountName),
			},
		},
		{
			name: "account reference secret not found",
			fields: fields{
				Client: fake.NewFakeClient(
					v1alpha1.NewTestAccount(testNamespace, testAccountName).WithStatusConnectionRef(testAccountName).Account,
					newCont().withAccountRef(testAccountName).withFinalizer(finalizer).Container),
			},
			args: args{
				ctx: ctx,
				c: newCont().withAccountRef(testAccountName).withFinalizer(finalizer).
					Container,
			},
			want: want{
				err: errors.Wrapf(newSecretNotFoundError(testAccountName),
					"failed to retrieve storage account secret: %s/%s", testNamespace, testAccountName),
			},
		},
		{
			name: "failed to create container handle",
			fields: fields{
				Client: fake.NewFakeClient(
					newCont().withAccountRef(testAccountName).withFinalizer(finalizer).Container,
					newSecret(testNamespace, testAccountName, map[string][]byte{
						corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(testAccountName),
						corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte("test-key"),
					}),
					v1alpha1.NewTestAccount(testNamespace, testAccountName).WithStatusConnectionRef(testAccountName).Account),
			},
			args: args{
				ctx: ctx,
				c: newCont().withAccountRef(testAccountName).
					withFinalizer(finalizer).
					withSpecNameFormat(testContainerName).
					Container,
			},
			want: want{
				err: errors.Wrapf(errors.New("illegal base64 data at input byte 4"),
					"failed to create client handle: %s, storage account: %s", testContainerName, testAccountName),
			},
		},
		{
			name: "success",
			fields: fields{
				Client: fake.NewFakeClient(
					newCont().withAccountRef(testAccountName).withFinalizer(finalizer).Container,
					newSecret(testNamespace, testAccountName, map[string][]byte{
						corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(testAccountName),
						corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte("dGVzdC1rZXkK"),
					}),
					v1alpha1.NewTestAccount(testNamespace, testAccountName).WithStatusConnectionRef(testAccountName).Account),
			},
			args: args{
				ctx: ctx,
				c: newCont().withAccountRef(testAccountName).
					withFinalizer(finalizer).
					withSpecNameFormat(testContainerName).
					Container,
			},
			want: want{
				syndel: &containerSyncdeleter{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &containerSyncdeleterMaker{
				Client: tt.fields.Client,
			}
			got, err := m.newSyncdeleter(tt.args.ctx, tt.args.c)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("containerSyncdeleterMaker.newSyncdeleter() error = \n%v, wantErr \n%v\n%s", err, tt.want.err, diff)
				return
			}
			if tt.want.syndel != nil {
				tt.want.syndel = &containerSyncdeleter{
					createupdater: &containerCreateUpdater{
						ContainerOperations: ch,
						kube:                tt.fields.Client,
						container:           tt.args.c,
					},
					ContainerOperations: ch,
					kube:                tt.fields.Client,
					container:           tt.args.c,
				}
				if diff := deep.Equal(got, tt.want.syndel); diff != nil {
					t.Errorf("containerSyncdeleterMaker.newSyncdeleter() = %v, want %v\n%s", got, tt.want.syndel, diff)
				}
			}
			if tt.want.cont != nil {
				cont := &v1alpha1.Container{}
				if err := tt.fields.Client.Get(tt.args.ctx, key, cont); err != nil {
					t.Errorf("containerSyncdeleterMaker.newSyncdeleter() error validating continer: %v, expected nil", err)
				}
				if diff := deep.Equal(cont, tt.want.cont); diff != nil {
					t.Errorf("containerSyncdeleterMaker.newSyncdeleter() container = %v, want %v\n%s", got, tt.want.cont, diff)
				}
			}
		})
	}
}

func Test_containerSyncdeleter_delete(t *testing.T) {
	ctx := context.TODO()

	type fields struct {
		createupdater       createupdater
		ContainerOperations storage.ContainerOperations
		kube                client.Client
		container           *v1alpha1.Container
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		res  reconcile.Result
		err  error
		cont *v1alpha1.Container
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name: "reclaim: retain",
			fields: fields{
				kube: test.NewMockClient(),
				container: newContainer(testNamespace, testContainerName).
					withReclaimPolicy(corev1alpha1.ReclaimRetain).
					withFinalizer(finalizer).Container,
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: newContainer(testNamespace, testContainerName).
					withReclaimPolicy(corev1alpha1.ReclaimRetain).Container,
			},
		},
		{
			name: "delete error: not found",
			fields: fields{
				kube: test.NewMockClient(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockDelete: func(ctx context.Context) error {
						return autorest.DetailedError{StatusCode: http.StatusNotFound}
					},
				},
				container: newContainer(testNamespace, testContainerName).
					withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Container,
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: newContainer(testNamespace, testContainerName).
					withReclaimPolicy(corev1alpha1.ReclaimDelete).Container,
			},
		},
		{
			name: "delete error: other",
			fields: fields{
				kube: test.NewMockClient(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockDelete: func(ctx context.Context) error {
						return errors.New("test-delete-error")
					},
				},
				container: newContainer(testNamespace, testContainerName).
					withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).Container,
			},
			args: args{ctx: ctx},
			want: want{
				res: resultRequeue,
				cont: newContainer(testNamespace, testContainerName).
					withReclaimPolicy(corev1alpha1.ReclaimDelete).
					withFinalizer(finalizer).
					withFailedCondition(failedToDelete, "test-delete-error").
					Container,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csd := &containerSyncdeleter{
				createupdater:       tt.fields.createupdater,
				ContainerOperations: tt.fields.ContainerOperations,
				kube:                tt.fields.kube,
				container:           tt.fields.container,
			}
			got, err := csd.delete(tt.args.ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("containerSyncdeleter.delete() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("containerSyncdeleter.delete() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := deep.Equal(tt.fields.container, tt.want.cont); diff != nil {
				t.Errorf("containerSyncdeleter.delete() container = \n%v, want \n%v\n%s", tt.fields.container, tt.want.cont, diff)
			}
		})
	}
}

func Test_containerSyncdeleter_sync(t *testing.T) {
	ctx := context.TODO()
	type fields struct {
		createupdater       createupdater
		ContainerOperations storage.ContainerOperations
		kube                client.Client
		container           *v1alpha1.Container
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		res  reconcile.Result
		err  error
		cont *v1alpha1.Container
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name: "get error: not-found",
			fields: fields{
				createupdater: newMockCreateUpdater(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
						return nil, nil, newStorageNotFoundError()
					},
				},
			},
			args: args{ctx: ctx},
			want: want{},
		},
		{
			name: "get error: other",
			fields: fields{
				createupdater: newMockCreateUpdater(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
						return nil, nil, errors.New("test-get-error")
					},
				},
				container: newContainer(testNamespace, testContainerName).Container,
				kube:      test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res: resultRequeue,
				cont: newContainer(testNamespace, testContainerName).
					withFailedCondition(failedToRetrieve, "test-get-error").Container,
			},
		},
		{
			name: "create",
			fields: fields{
				createupdater: newMockCreateUpdater(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
						return nil, nil, nil
					},
				},
				container: newContainer(testNamespace, testContainerName).Container,
				kube:      test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res:  reconcile.Result{},
				cont: newContainer(testNamespace, testContainerName).Container,
			},
		},
		{
			name: "update",
			fields: fields{
				createupdater: newMockCreateUpdater(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
						return azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer), nil, nil
					},
				},
				container: newContainer(testNamespace, testContainerName).Container,
				kube:      test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res:  reconcile.Result{},
				cont: newContainer(testNamespace, testContainerName).Container,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csd := &containerSyncdeleter{
				createupdater:       tt.fields.createupdater,
				ContainerOperations: tt.fields.ContainerOperations,
				kube:                tt.fields.kube,
				container:           tt.fields.container,
			}
			got, err := csd.sync(tt.args.ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("containerSyncdeleter.sync() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("containerSyncdeleter.sync() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := deep.Equal(tt.fields.container, tt.want.cont); diff != nil {
				t.Errorf("containerSyncdeleter.sync() container = \n%v, want \n%v\n%s", tt.fields.container, tt.want.cont, diff)
			}
		})
	}
}

func Test_containerCreateUpdater_create(t *testing.T) {
	ctx := context.TODO()
	type fields struct {
		ContainerOperations storage.ContainerOperations
		kube                client.Client
		container           *v1alpha1.Container
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		res  reconcile.Result
		err  error
		cont *v1alpha1.Container
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name: "update finalizer failed",
			fields: fields{
				container: newContainer(testNamespace, testContainerName).Container,
				kube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-update-error")
					},
				},
			},
			args: args{ctx: ctx},
			want: want{
				res:  resultRequeue,
				err:  errors.Wrapf(errors.New("test-update-error"), "failed to update container spec"),
				cont: newContainer(testNamespace, testContainerName).withFinalizer(finalizer).Container,
			},
		},
		{
			name: "create failed",
			fields: fields{
				container: newContainer(testNamespace, testContainerName).Container,
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockCreate: func(ctx context.Context, pub azblob.PublicAccessType, meta azblob.Metadata) error {
						return errors.New("test-create-error")
					},
				},
				kube: test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res: resultRequeue,
				cont: newContainer(testNamespace, testContainerName).
					withFinalizer(finalizer).
					withFailedCondition(failedToCreate, "test-create-error").Container,
			},
		},
		{
			name: "create successful, initial status not ready",
			fields: fields{
				container: newContainer(testNamespace, testContainerName).
					withFailedCondition(failedToCreate, "test-error").Container,
				ContainerOperations: azurestoragefake.NewMockContainerOperations(),
				kube:                test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: newContainer(testNamespace, testContainerName).
					withFinalizer(finalizer).
					withFailedCondition(failedToCreate, "test-error").
					withUnsetAllConditions().
					withReadyCondition().
					Container,
			},
		},
		{
			name: "create successful",
			fields: fields{
				container:           newContainer(testNamespace, testContainerName).Container,
				ContainerOperations: azurestoragefake.NewMockContainerOperations(),
				kube:                test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: newContainer(testNamespace, testContainerName).
					withFinalizer(finalizer).
					withReadyCondition().
					Container,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ccu := &containerCreateUpdater{
				ContainerOperations: tt.fields.ContainerOperations,
				kube:                tt.fields.kube,
				container:           tt.fields.container,
			}
			got, err := ccu.create(tt.args.ctx)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("containerCreateUpdater.create() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("containerCreateUpdater.create() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := deep.Equal(tt.fields.container, tt.want.cont); diff != nil {
				t.Errorf("containerCreateUpdater.create() container = \n%v, want \n%v\n%s", tt.fields.container, tt.want.cont, diff)
			}
		})
	}
}

func Test_containerCreateUpdater_update(t *testing.T) {
	ctx := context.TODO()
	type fields struct {
		ContainerOperations storage.ContainerOperations
		kube                client.Client
		container           *v1alpha1.Container
	}
	type args struct {
		ctx        context.Context
		accessType *azblob.PublicAccessType
		meta       azblob.Metadata
	}
	type want struct {
		res  reconcile.Result
		err  error
		cont *v1alpha1.Container
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name: "no change, not ready",
			fields: fields{
				container: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).Container,
				kube: test.NewMockClient(),
			},
			args: args{
				ctx:        ctx,
				accessType: azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer),
			},
			want: want{
				res: reconcile.Result{},
				cont: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).
					withReadyCondition().Container,
			},
		},
		{
			name: "no change, is ready",
			fields: fields{
				container: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).
					withReadyCondition().Container,
			},
			args: args{
				ctx:        ctx,
				accessType: azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer),
			},
			want: want{
				res: requeueOnSuccess,
				cont: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).
					withReadyCondition().Container,
			},
		},
		{
			name: "container update failed",
			fields: fields{
				container: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).
					withReadyCondition().Container,
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockUpdate: func(ctx context.Context, publicAccessType azblob.PublicAccessType, meta azblob.Metadata) error {
						return errors.New("test-container-update-error")
					},
				},
				kube: test.NewMockClient(),
			},
			args: args{
				ctx:        ctx,
				accessType: azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer),
				meta: azblob.Metadata{
					"foo": "bar",
				},
			},
			want: want{
				res: resultRequeue,
				cont: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).
					withReadyCondition().
					withFailedCondition(failedToUpdate, "test-container-update-error").Container,
			},
		},
		{
			name: "container update successful",
			fields: fields{
				container: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).
					withReadyCondition().Container,
				ContainerOperations: azurestoragefake.NewMockContainerOperations(),
				kube:                test.NewMockClient(),
			},
			args: args{
				ctx:        ctx,
				accessType: azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer),
				meta: azblob.Metadata{
					"foo": "bar",
				},
			},
			want: want{
				res: requeueOnSuccess,
				cont: newContainer(testNamespace, testContainerName).
					withSpecPAC(azblob.PublicAccessContainer).
					withReadyCondition().Container,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ccu := &containerCreateUpdater{
				ContainerOperations: tt.fields.ContainerOperations,
				kube:                tt.fields.kube,
				container:           tt.fields.container,
			}
			got, err := ccu.update(tt.args.ctx, tt.args.accessType, tt.args.meta)
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Errorf("containerCreateUpdater.update() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := deep.Equal(got, tt.want.res); diff != nil {
				t.Errorf("containerCreateUpdater.update() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := deep.Equal(tt.fields.container, tt.want.cont); diff != nil {
				t.Errorf("containerCreateUpdater.update() container = \n%v, want \n%v\n%s", tt.fields.container, tt.want.cont, diff)
			}
		})
	}
}

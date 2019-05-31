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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
	v1alpha1test "github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1/test"
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
			name: "GetErrNotFound",
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
			name: "GetErrOther",
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
			name: "SyncdeleteMakerError",
			fields: fields{
				Client: fake.NewFakeClient(v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithFinalizer("foo.bar").Container),
				syncdeleterMaker: &mockSyncdeleteMaker{
					mockNewSyncdeleter: func(ctx context.Context, c *v1alpha1.Container) (syncdeleter, error) {
						return nil, errors.New("test-new-syncdeleter-error")
					},
				},
			},
			want: want{
				res: resultRequeue,
				con: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithFinalizer("foo.bar").
					WithFailedDeprecatedCondition(failedToGetHandler, "test-new-syncdeleter-error").
					Container,
			},
		},
		{
			name: "Delete",
			fields: fields{
				Client: fake.NewFakeClient(v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithDeleteTimestamp(time.Now()).Container),
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
			name: "Sync",
			fields: fields{
				Client: fake.NewFakeClient(v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container),
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
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("Reconciler.Reconcile() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("Reconciler.Reconcile() result = %v, wantRs %v\n%s", got, tt.want.res, diff)
			}
			if tt.want.con != nil {
				c := &v1alpha1.Container{}
				if err := tt.fields.Client.Get(ctx, key, c); err != nil {
					t.Errorf("Reconciler.Reconcile() container error: %s", err)
				}
				if diff := cmp.Diff(c, tt.want.con); diff != "" {
					t.Errorf("Reconciler.Reconcile() container = \n%+v, wantObj \n%+v\n%s", c, tt.want.con, diff)
				}
			}
		})
	}
}

func Test_containerSyncdeleterMaker_newSyncdeleter(t *testing.T) {
	key := types.NamespacedName{Namespace: testNamespace, Name: testContainerName}
	newCont := func() *v1alpha1test.MockContainer {
		return v1alpha1test.NewMockContainer(testNamespace, testContainerName)
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
			name: "FailedToGetAccountNotFoundNoDelete",
			fields: fields{
				Client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return newAccountNotFoundError(testAccountName)
					},
				},
			},
			args: args{
				ctx: ctx,
				c:   newCont().WithSpecAccountRef(testAccountName).WithFinalizer(finalizer).Container,
			},
			want: want{
				err: errors.Wrapf(newAccountNotFoundError(testAccountName),
					"failed to retrieve storage account reference: %s/%s", testNamespace, testAccountName),
			},
		},
		{
			name: "FailedToGetAccountNotFoundYesDelete",
			fields: fields{
				Client: fake.NewFakeClient(newCont().WithSpecAccountRef(testAccountName).
					WithFinalizer(finalizer).
					Container),
			},
			args: args{
				ctx: ctx,
				c: newCont().WithSpecAccountRef(testAccountName).WithFinalizer(finalizer).
					WithDeleteTimestamp(time.Now()).
					Container,
			},
			want: want{
				err: errors.Wrapf(newAccountNotFoundError(testAccountName),
					"failed to retrieve storage account reference: %s/%s", testNamespace, testAccountName),
			},
		},
		{
			name: "AccountReferenceSecretNotFound",
			fields: fields{
				Client: fake.NewFakeClient(
					v1alpha1test.NewMockAccount(testNamespace, testAccountName).WithStatusConnectionRef(testAccountName).Account,
					newCont().WithSpecAccountRef(testAccountName).WithFinalizer(finalizer).Container),
			},
			args: args{
				ctx: ctx,
				c: newCont().WithSpecAccountRef(testAccountName).WithFinalizer(finalizer).
					Container,
			},
			want: want{
				err: errors.Wrapf(newSecretNotFoundError(testAccountName),
					"failed to retrieve storage account secret: %s/%s", testNamespace, testAccountName),
			},
		},
		{
			name: "FailedToCreateContainerHandle",
			fields: fields{
				Client: fake.NewFakeClient(
					newCont().WithSpecAccountRef(testAccountName).WithFinalizer(finalizer).Container,
					newSecret(testNamespace, testAccountName, map[string][]byte{
						corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(testAccountName),
						corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte("test-key"),
					}),
					v1alpha1test.NewMockAccount(testNamespace, testAccountName).WithStatusConnectionRef(testAccountName).Account),
			},
			args: args{
				ctx: ctx,
				c: newCont().WithSpecAccountRef(testAccountName).
					WithFinalizer(finalizer).
					WithSpecNameFormat(testContainerName).
					Container,
			},
			want: want{
				err: errors.Wrapf(errors.New("illegal base64 data at input byte 4"),
					"failed to create client handle: %s, storage account: %s", testContainerName, testAccountName),
			},
		},
		{
			name: "Success",
			fields: fields{
				Client: fake.NewFakeClient(
					newCont().WithSpecAccountRef(testAccountName).WithFinalizer(finalizer).Container,
					newSecret(testNamespace, testAccountName, map[string][]byte{
						corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(testAccountName),
						corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte("dGVzdC1rZXkK"),
					}),
					v1alpha1test.NewMockAccount(testNamespace, testAccountName).WithStatusConnectionRef(testAccountName).Account),
			},
			args: args{
				ctx: ctx,
				c: newCont().WithSpecAccountRef(testAccountName).
					WithFinalizer(finalizer).
					WithSpecNameFormat(testContainerName).
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
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
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
				// BUG(negz): This test is broken. It appears to intend to compare
				// unexported fields, but does not. This behaviour was maintained
				// when porting the test from https://github.com/go-test/deep to cmp.
				if diff := cmp.Diff(got, tt.want.syndel,
					cmpopts.IgnoreUnexported(containerSyncdeleter{}),
					cmpopts.IgnoreUnexported(azblob.ContainerURL{}),
				); diff != "" {
					t.Errorf("containerSyncdeleterMaker.newSyncdeleter() = %v, want %v\n%s", got, tt.want.syndel, diff)
				}
			}
			if tt.want.cont != nil {
				cont := &v1alpha1.Container{}
				if err := tt.fields.Client.Get(tt.args.ctx, key, cont); err != nil {
					t.Errorf("containerSyncdeleterMaker.newSyncdeleter() error validating continer: %v, expected nil", err)
				}
				if diff := cmp.Diff(cont, tt.want.cont); diff != "" {
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
			name: "ReclaimRetain",
			fields: fields{
				kube: test.NewMockClient(),
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecReclaimPolicy(corev1alpha1.ReclaimRetain).
					WithFinalizer(finalizer).Container,
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecReclaimPolicy(corev1alpha1.ReclaimRetain).
					WithFinalizers([]string{}).Container,
			},
		},
		{
			name: "DeleteErrorNotFound",
			fields: fields{
				kube: test.NewMockClient(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockDelete: func(ctx context.Context) error {
						return autorest.DetailedError{StatusCode: http.StatusNotFound}
					},
				},
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecReclaimPolicy(corev1alpha1.ReclaimDelete).
					WithFinalizer(finalizer).Container,
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecReclaimPolicy(corev1alpha1.ReclaimDelete).
					WithFinalizers([]string{}).Container,
			},
		},
		{
			name: "DeleteErrorOther",
			fields: fields{
				kube: test.NewMockClient(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockDelete: func(ctx context.Context) error {
						return errors.New("test-delete-error")
					},
				},
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecReclaimPolicy(corev1alpha1.ReclaimDelete).
					WithFinalizer(finalizer).Container,
			},
			args: args{ctx: ctx},
			want: want{
				res: resultRequeue,
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecReclaimPolicy(corev1alpha1.ReclaimDelete).
					WithFinalizer(finalizer).
					WithFailedDeprecatedCondition(failedToDelete, "test-delete-error").
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
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("containerSyncdeleter.delete() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("containerSyncdeleter.delete() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := cmp.Diff(tt.fields.container, tt.want.cont); diff != "" {
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
			name: "GetErrorNotFound",
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
			name: "GetErrorOther",
			fields: fields{
				createupdater: newMockCreateUpdater(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
						return nil, nil, errors.New("test-get-error")
					},
				},
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
				kube:      test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res: resultRequeue,
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithFailedDeprecatedCondition(failedToRetrieve, "test-get-error").Container,
			},
		},
		{
			name: "Create",
			fields: fields{
				createupdater: newMockCreateUpdater(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
						return nil, nil, nil
					},
				},
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
				kube:      test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res:  reconcile.Result{},
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
			},
		},
		{
			name: "Update",
			fields: fields{
				createupdater: newMockCreateUpdater(),
				ContainerOperations: &azurestoragefake.MockContainerOperations{
					MockGet: func(ctx context.Context) (*azblob.PublicAccessType, azblob.Metadata, error) {
						return azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer), nil, nil
					},
				},
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
				kube:      test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res:  reconcile.Result{},
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
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
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("containerSyncdeleter.sync() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("containerSyncdeleter.sync() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := cmp.Diff(tt.fields.container, tt.want.cont); diff != "" {
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
			name: "UpdateFinalizerFailed",
			fields: fields{
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
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
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).WithFinalizer(finalizer).Container,
			},
		},
		{
			name: "CreateFailed",
			fields: fields{
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
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
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithFinalizer(finalizer).
					WithFailedDeprecatedCondition(failedToCreate, "test-create-error").Container,
			},
		},
		{
			name: "CreateSuccessfulInitialStatusNotReady",
			fields: fields{
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithFailedDeprecatedCondition(failedToCreate, "test-error").Container,
				ContainerOperations: azurestoragefake.NewMockContainerOperations(),
				kube:                test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithFinalizer(finalizer).
					WithFailedDeprecatedCondition(failedToCreate, "test-error").
					WithUnsetAllDeprecatedConditions().
					WithReadyDeprecatedCondition().
					Container,
			},
		},
		{
			name: "CreateSuccessful",
			fields: fields{
				container:           v1alpha1test.NewMockContainer(testNamespace, testContainerName).Container,
				ContainerOperations: azurestoragefake.NewMockContainerOperations(),
				kube:                test.NewMockClient(),
			},
			args: args{ctx: ctx},
			want: want{
				res: reconcile.Result{},
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithFinalizer(finalizer).
					WithReadyDeprecatedCondition().
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
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("containerCreateUpdater.create() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("containerCreateUpdater.create() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := cmp.Diff(tt.fields.container, tt.want.cont); diff != "" {
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
			name: "NoChangeNotReady",
			fields: fields{
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).Container,
				kube: test.NewMockClient(),
			},
			args: args{
				ctx:        ctx,
				accessType: azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer),
			},
			want: want{
				res: reconcile.Result{},
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).
					WithReadyDeprecatedCondition().Container,
			},
		},
		{
			name: "NoChangeIsReady",
			fields: fields{
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).
					WithReadyDeprecatedCondition().Container,
			},
			args: args{
				ctx:        ctx,
				accessType: azurestoragefake.PublicAccessTypePtr(azblob.PublicAccessContainer),
			},
			want: want{
				res: requeueOnSuccess,
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).
					WithReadyDeprecatedCondition().Container,
			},
		},
		{
			name: "ContainerUpdateFailed",
			fields: fields{
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).
					WithReadyDeprecatedCondition().Container,
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
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).
					WithReadyDeprecatedCondition().
					WithFailedDeprecatedCondition(failedToUpdate, "test-container-update-error").Container,
			},
		},
		{
			name: "ContainerUpdateSuccessful",
			fields: fields{
				container: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).
					WithReadyDeprecatedCondition().Container,
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
				cont: v1alpha1test.NewMockContainer(testNamespace, testContainerName).
					WithSpecPAC(azblob.PublicAccessContainer).
					WithReadyDeprecatedCondition().Container,
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
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("containerCreateUpdater.update() error = %v, wantErr %v\n%s", err, tt.want.err, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("containerCreateUpdater.update() = %v, want %v\n%s", got, tt.want.res, diff)
			}
			if diff := cmp.Diff(tt.fields.container, tt.want.cont); diff != "" {
				t.Errorf("containerCreateUpdater.update() container = \n%v, want \n%v\n%s", tt.fields.container, tt.want.cont, diff)
			}
		})
	}
}

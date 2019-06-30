package database

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	core "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

type mockCreateUpdater struct {
	mockCreate func(context.Context) (reconcile.Result, error)
	mockUpdate func(context.Context, *sqladmin.DatabaseInstance) (reconcile.Result, error)
}

var _ createupdater = &mockCreateUpdater{}

func (m *mockCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	return m.mockCreate(ctx)
}
func (m *mockCreateUpdater) update(ctx context.Context, di *sqladmin.DatabaseInstance) (reconcile.Result, error) {
	return m.mockUpdate(ctx, di)
}

func assertUpdateReconcileStatusSuccess(t *testing.T, e error) error {
	if e != nil {
		t.Errorf("update() unexpectd error: %v", e)
	}
	return e
}

func Test_handleNotFound(t *testing.T) {
	tests := map[string]struct {
		args error
		want error
	}{
		"NoError": {
			args: nil,
			want: nil,
		},
		"ErrorNotFoundK8S": {
			args: kerrors.NewNotFound(schema.GroupResource{}, ""),
			want: nil,
		},
		"ErrorNotFoundGoogleAPI": {
			args: &googleapi.Error{
				Code: http.StatusNotFound,
			},
			want: nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, handleNotFound(tt.args)); diff != "" {
				t.Errorf("handleNotFound() error %s", diff)
			}
		})
	}
}

func Test_instanceCreateUpdater_update(t *testing.T) {
	type fields struct {
		operations managedOperations
	}
	type args struct {
		ctx  context.Context
		inst *sqladmin.DatabaseInstance
	}
	type want struct {
		err error
		res reconcile.Result
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"UpdateStatusFailure": {
			fields: fields{
				operations: &mockManagedOperations{
					localOperations: &mockLocalOperations{
						mockUpdateInstanceStatus: func(ctx context.Context, di *sqladmin.DatabaseInstance) error {
							return errTest
						},
					},
				},
			},
			args: args{
				inst: &sqladmin.DatabaseInstance{},
			},
			want: want{
				err: errors.Wrapf(errTest, "failed to update instance status"),
				res: requeueNow,
			},
		},
		"InstanceIsNotReady": {
			fields: fields{
				operations: &mockManagedOperations{
					localOperations: &mockLocalOperations{
						mockUpdateInstanceStatus: func(ctx context.Context, di *sqladmin.DatabaseInstance) error { return nil },
						mockIsInstanceReady:      func() bool { return false },
						mockUpdateReconcileStatus: func(ctx context.Context, e error) error {
							return assertUpdateReconcileStatusSuccess(t, e)
						},
					},
				},
			},
			args: args{
				inst: &sqladmin.DatabaseInstance{},
			},
			want: want{
				res: requeueWait,
			},
		},
		"InstanceNeedsAnUpdate": {
			fields: fields{
				operations: &mockManagedOperations{
					localOperations: &mockLocalOperations{
						mockUpdateInstanceStatus: func(ctx context.Context, di *sqladmin.DatabaseInstance) error { return nil },
						mockIsInstanceReady:      func() bool { return true },
						mockNeedUpdate:           func(di *sqladmin.DatabaseInstance) bool { return true },
						mockUpdateReconcileStatus: func(ctx context.Context, e error) error {
							return assertUpdateReconcileStatusSuccess(t, e)
						},
					},
					mockUpdateInstance: func(ctx context.Context) error { return nil },
				},
			},
			args: args{
				inst: &sqladmin.DatabaseInstance{},
			},
			want: want{
				res: requeueNow,
			},
		},
		"UpdateUserCreds": {
			fields: fields{
				operations: &mockManagedOperations{
					localOperations: &mockLocalOperations{
						mockUpdateInstanceStatus: func(ctx context.Context, di *sqladmin.DatabaseInstance) error { return nil },
						mockIsInstanceReady:      func() bool { return true },
						mockNeedUpdate:           func(di *sqladmin.DatabaseInstance) bool { return false },
						mockUpdateReconcileStatus: func(ctx context.Context, e error) error {
							if e != nil {
								t.Errorf("update() unexpectd error: %v", e)
							}
							return nil
						},
					},
					mockUpdateUserCreds: func(ctx context.Context) error { return nil },
				},
			},
			args: args{
				inst: &sqladmin.DatabaseInstance{},
			},
			want: want{
				res: requeueSync,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := newInstanceCreateUpdater(tt.fields.operations)
			got, err := ih.update(tt.args.ctx, tt.args.inst)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("update() error %s", diff)
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("update() got %s", diff)
			}
		})
	}
}

func Test_instanceCreateUpdater_create(t *testing.T) {
	type fields struct {
		operations managedOperations
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		res reconcile.Result
		err error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"AddFinalizerFailure": {
			fields: fields{
				operations: &mockManagedOperations{
					localOperations: &mockLocalOperations{
						mockAddFinalizer: func(ctx context.Context) error {
							return errTest
						},
					},
				},
			},
			want: want{
				res: requeueNow,
				err: errors.Wrapf(errTest, "Failed to update instance object"),
			},
		},
		"CreateInstance": {
			fields: fields{
				operations: &mockManagedOperations{
					localOperations: &mockLocalOperations{
						mockAddFinalizer: func(ctx context.Context) error { return nil },
						mockUpdateReconcileStatus: func(ctx context.Context, e error) error {
							return assertUpdateReconcileStatusSuccess(t, e)
						},
					},
					mockCreateInstance: func(ctx context.Context) error { return nil },
				},
			},
			want: want{
				res: requeueNow,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := newInstanceCreateUpdater(tt.fields.operations)
			got, err := ih.create(tt.args.ctx)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("create() error %s", diff)
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("create() got %s", diff)
			}
		})
	}
}

func Test_instanceSyncDeleter_sync(t *testing.T) {
	type fields struct {
		operations    managedOperations
		createupdater createupdater
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		res reconcile.Result
		err error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"FailedToRetriveInstance": {
			fields: fields{
				operations: &mockManagedOperations{
					mockGetInstance: func(ctx context.Context) (instance *sqladmin.DatabaseInstance, e error) {
						return nil, errTest
					},
					localOperations: &mockLocalOperations{
						mockUpdateReconcileStatus: func(ctx context.Context, e error) error {
							if diff := cmp.Diff(errTest, e, test.EquateErrors()); diff != "" {
								t.Errorf("sync() error %s", diff)
							}
							return nil
						},
					},
				},
				createupdater: nil,
			},
			want: want{
				res: requeueNow,
			},
		},
		"SyncCreate": {
			fields: fields{
				operations: &mockManagedOperations{
					mockGetInstance: func(ctx context.Context) (instance *sqladmin.DatabaseInstance, e error) {
						return nil, &googleapi.Error{
							Code: http.StatusNotFound,
						}
					},
				},
				createupdater: &mockCreateUpdater{
					mockCreate: func(ctx context.Context) (reconcile.Result, error) {
						return requeueNow, nil
					},
				},
			},
			want: want{
				res: requeueNow,
			},
		},
		"SyncCUpdate": {
			fields: fields{
				operations: &mockManagedOperations{
					mockGetInstance: func(ctx context.Context) (instance *sqladmin.DatabaseInstance, e error) {
						return &sqladmin.DatabaseInstance{}, nil
					},
				},
				createupdater: &mockCreateUpdater{
					mockUpdate: func(ctx context.Context, di *sqladmin.DatabaseInstance) (reconcile.Result, error) {
						return requeueWait, nil
					},
				},
			},
			want: want{
				res: requeueWait,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			sd := &instanceSyncDeleter{
				managedOperations: tt.fields.operations,
				createupdater:     tt.fields.createupdater,
			}
			got, err := sd.sync(tt.args.ctx)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("sync() error %s", diff)
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("sync() got %s", diff)
			}
		})
	}
}

func Test_instanceSyncDeleter_delete(t *testing.T) {
	type fields struct {
		operations    managedOperations
		createupdater createupdater
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		res reconcile.Result
		err error
	}
	tests := map[string]struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		"DeleteError": {
			fields: fields{
				operations: &mockManagedOperations{
					mockDeleteInstance: func(ctx context.Context) error { return errTest },
					localOperations: &mockLocalOperations{
						mockIsReclaimDelete: func() bool { return true },
						mockUpdateReconcileStatus: func(ctx context.Context, e error) error {
							if diff := cmp.Diff(errTest, e, test.EquateErrors()); diff != "" {
								t.Errorf("delete() error %s", diff)
							}
							return nil
						},
					},
				},
				createupdater: nil,
			},
			want: want{
				res: requeueNow,
			},
		},
		"DeleteNonExistent": {
			fields: fields{
				operations: &mockManagedOperations{
					mockDeleteInstance: func(ctx context.Context) error {
						return &googleapi.Error{
							Code: http.StatusNotFound,
						}
					},
					localOperations: &mockLocalOperations{
						mockIsReclaimDelete: func() bool { return true },
						mockRemoveFinalizer: func(ctx context.Context) error { return nil },
					},
				},
				createupdater: nil,
			},
			want: want{
				res: requeueNow,
			},
		},
		"DeleteExistent": {
			fields: fields{
				operations: &mockManagedOperations{
					mockDeleteInstance: func(ctx context.Context) error { return nil },
					localOperations: &mockLocalOperations{
						mockIsReclaimDelete: func() bool { return true },
						mockRemoveFinalizer: func(ctx context.Context) error { return nil },
					},
				},
				createupdater: nil,
			},
			want: want{
				res: requeueNow,
			},
		},
		"ReclaimRetain": {
			fields: fields{
				operations: &mockManagedOperations{
					localOperations: &mockLocalOperations{
						mockIsReclaimDelete: func() bool { return false },
						mockRemoveFinalizer: func(ctx context.Context) error { return nil },
					},
				},
				createupdater: nil,
			},
			want: want{
				res: requeueNow,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := &instanceSyncDeleter{
				managedOperations: tt.fields.operations,
				createupdater:     tt.fields.createupdater,
			}
			got, err := sd.delete(tt.args.ctx)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("delete() error %s", diff)
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("delete() got %s", diff)
			}
		})
	}
}

func Test_operationsFactory_makeSyncDeleter(t *testing.T) {
	f := &operationsFactory{}
	f.makeSyncDeleter(&mockManagedOperations{})
}

func Test_operationsFactory_makeManagedOperations(t *testing.T) {
	type args struct {
		ctx  context.Context
		inst *v1alpha1.CloudsqlInstance
		ops  localOperations
	}
	type want struct {
		err error
		ops managedOperations
	}
	mockInstance := &v1alpha1.CloudsqlInstance{
		Spec: v1alpha1.CloudsqlInstanceSpec{
			ResourceSpec: *newInstanceSpec().
				withProviderRef(&core.ObjectReference{}).build(),
		},
	}
	mockProvider := func(p *gcpv1alpha1.Provider) {
		pp := &gcpv1alpha1.Provider{
			ObjectMeta: meta.ObjectMeta{
				Namespace: "default",
			},
			Spec: gcpv1alpha1.ProviderSpec{
				Secret: core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{
						Name: "test-provider-secret",
					},
				},
			},
		}
		pp.DeepCopyInto(p)
	}
	mockSecret := func(s *core.Secret, key types.NamespacedName) {
		nn := types.NamespacedName{Namespace: "default", Name: "test-provider-secret"}
		if key != nn {
			t.Errorf("makeManagedOperations() invalid secret key %s", key)
		}
		ss := &core.Secret{
			Data: map[string][]byte{
				"key": []byte(""),
			},
		}
		ss.DeepCopyInto(s)
	}

	tests := map[string]struct {
		kube client.Client
		args args
		want want
	}{
		"FailedToGetProvider": {
			kube: &test.MockClient{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					if _, ok := obj.(*gcpv1alpha1.Provider); ok {
						return errTest
					}
					t.Errorf("makeManagedOperations() unexpected type %T", obj)
					return nil
				},
			},
			args: args{
				inst: mockInstance,
			},
			want: want{
				err: errTest,
				ops: nil,
			},
		},
		"FailedToGetProviderSecret": {
			kube: &test.MockClient{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					if p, ok := obj.(*gcpv1alpha1.Provider); ok {
						mockProvider(p)
						return nil
					}
					if _, ok := obj.(*core.Secret); ok {
						nn := types.NamespacedName{Namespace: "default", Name: "test-provider-secret"}
						if key != nn {
							t.Errorf("makeManagedOperations() invalid secret key %s", key)
						}
						return errTest
					}

					t.Errorf("makeManagedOperations() unexpected type %T", obj)
					return nil
				},
			},
			args: args{
				inst: mockInstance,
			},
			want: want{
				err: errors.Wrapf(errTest, "cannot get provider's secret default/test-provider-secret"),
				ops: nil,
			},
		},
		"FailedToGetCredentials": {
			kube: &test.MockClient{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					if p, ok := obj.(*gcpv1alpha1.Provider); ok {
						mockProvider(p)
						return nil
					}
					if s, ok := obj.(*core.Secret); ok {
						mockSecret(s, key)
						return nil
					}
					t.Errorf("makeManagedOperations() unexpected type %T", obj)
					return nil
				},
			},
			args: args{
				inst: mockInstance,
			},
			want: want{
				err: errors.Wrapf(errors.New("unexpected end of JSON input"),
					"cannot retrieve creds from json"),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := &operationsFactory{
				Client: tt.kube,
			}
			got, err := f.makeManagedOperations(tt.args.ctx, tt.args.inst, tt.args.ops)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("makeManagedOperations() error %s", diff)
			}
			if diff := cmp.Diff(tt.want.ops, got); diff != "" {
				t.Errorf("makeManagedOperations() got %s", diff)
			}
		})
	}
}

func TestReconciler_Reconcile(t *testing.T) {
	assert := func(t *testing.T, obj runtime.Object) *v1alpha1.CloudsqlInstance {
		c, err := assertObjectInstance(obj)
		if err != nil {
			t.Errorf("Reconcile() %s", err.Error())
		}
		return c
	}
	type fields struct {
		kube    client.Client
		factory factory
	}
	type args struct {
		request reconcile.Request
	}
	type want struct {
		err error
		res reconcile.Result
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"FailedToGetInstance": {
			fields: fields{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						assert(t, obj)
						if key != testKey {
							t.Errorf("Reconcile() invalid key: %s", key)
						}
						return errTest
					},
				},
			},
			args: args{request: reconcile.Request{NamespacedName: testKey}},
			want: want{
				err: errTest,
				res: requeueNever,
			},
		},
		"FailedToMakeManagedOperations": {
			fields: fields{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						assert(t, obj)
						return nil
					},
				},
				factory: &mockFactory{
					mockMakeLocalOperations: func(instance *v1alpha1.CloudsqlInstance, i client.Client) localOperations {
						return &mockLocalOperations{
							mockUpdateReconcileStatus: func(ctx context.Context, e error) error {
								if diff := cmp.Diff(errTest, e, test.EquateErrors()); diff != "" {
									t.Errorf("Reconcile() makeLocalOperations error %s", diff)
								}
								return nil
							},
						}
					},
					mockMakeManagedOperations: func(ctx context.Context, instance *v1alpha1.CloudsqlInstance, ops localOperations) (managedOperations, error) {
						return nil, errTest
					},
				},
			},
			args: args{request: reconcile.Request{NamespacedName: testKey}},
			want: want{
				err: nil,
				res: requeueNow,
			},
		},
		"Deleting": {
			fields: fields{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						inst := assert(t, obj)
						t := meta.Now()
						inst.DeletionTimestamp = &t
						return nil
					},
				},
				factory: &mockFactory{
					mockMakeLocalOperations: func(instance *v1alpha1.CloudsqlInstance, i client.Client) localOperations {
						return nil
					},
					mockMakeManagedOperations: func(ctx context.Context, instance *v1alpha1.CloudsqlInstance, ops localOperations) (managedOperations, error) {
						return nil, nil
					},
					mockMakeSyncDeleter: func(ops managedOperations) syncdeleter {
						return &mockSyncDeleter{
							mockSync: nil,
							mockDelete: func(ctx context.Context) (result reconcile.Result, e error) {
								return requeueNow, nil
							},
						}
					},
				},
			},
			args: args{request: reconcile.Request{NamespacedName: testKey}},
			want: want{
				err: nil,
				res: requeueNow,
			},
		},
		"Syncing": {
			fields: fields{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						assert(t, obj)
						return nil
					},
				},
				factory: &mockFactory{
					mockMakeLocalOperations: func(instance *v1alpha1.CloudsqlInstance, i client.Client) localOperations {
						return nil
					},
					mockMakeManagedOperations: func(ctx context.Context, instance *v1alpha1.CloudsqlInstance, ops localOperations) (managedOperations, error) {
						return nil, nil
					},
					mockMakeSyncDeleter: func(ops managedOperations) syncdeleter {
						return &mockSyncDeleter{
							mockSync: func(ctx context.Context) (result reconcile.Result, e error) {
								return requeueSync, nil
							},
						}
					},
				},
			},
			args: args{request: reconcile.Request{NamespacedName: testKey}},
			want: want{
				err: nil,
				res: requeueSync,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := &Reconciler{
				Client:  tt.fields.kube,
				factory: tt.fields.factory,
			}
			got, err := r.Reconcile(tt.args.request)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile() error = %s", diff)
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("Reconcile() got = %s", diff)
			}
		})
	}
}

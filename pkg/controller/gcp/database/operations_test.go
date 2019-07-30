package database

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	core "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	meta1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudsql"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudsql/fake"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	testNs   = "test-ns"
	testName = "test-name"
	testUID  = "test-uid"
)

var (
	errTest = errors.New("test-error")

	testMeta = meta1.ObjectMeta{
		Name:      testName,
		Namespace: testNs,
		UID:       testUID,
	}

	testKey = types.NamespacedName{Namespace: testNs, Name: testName}

	testSecret = func(ep, pw string) *core.Secret {
		t := true
		return &core.Secret{
			ObjectMeta: meta1.ObjectMeta{
				Name:      testName,
				Namespace: testNs,
				OwnerReferences: []meta1.OwnerReference{
					{
						APIVersion: v1alpha1.SchemeGroupVersion.String(),
						Kind:       v1alpha1.CloudsqlInstanceKind,
						Name:       testName,
						UID:        testUID,
						Controller: &t,
					},
				},
			},
			Data: map[string][]byte{
				corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(ep),
				corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(pw),
				corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(v1alpha1.MysqlDefaultUser),
			},
		}
	}
)

type mockLocalOperations struct {
	// Bucket object managedOperations
	mockAddFinalizer    func(context.Context) error
	mockIsReclaimDelete func() bool
	mockIsInstanceReady func() bool
	mockNeedUpdate      func(*sqladmin.DatabaseInstance) bool
	mockRemoveFinalizer func(context.Context) error

	// Controller-runtime managedOperations
	mockUpdateObject           func(context.Context) error
	mockUpdateInstanceStatus   func(context.Context, *sqladmin.DatabaseInstance) error
	mockUpdateReconcileStatus  func(context.Context, error) error
	mockUpdateConnectionSecret func(context.Context) (*core.Secret, error)
}

var _ localOperations = &mockLocalOperations{}

func (m *mockLocalOperations) addFinalizer(ctx context.Context) error {
	return m.mockAddFinalizer(ctx)
}
func (m *mockLocalOperations) isReclaimDelete() bool {
	return m.mockIsReclaimDelete()
}
func (m *mockLocalOperations) isInstanceReady() bool {
	return m.mockIsInstanceReady()
}
func (m *mockLocalOperations) needsUpdate(di *sqladmin.DatabaseInstance) bool {
	return m.mockNeedUpdate(di)
}
func (m *mockLocalOperations) removeFinalizer(ctx context.Context) error {
	return m.mockRemoveFinalizer(ctx)
}
func (m *mockLocalOperations) updateObject(ctx context.Context) error {
	return m.mockUpdateObject(ctx)
}
func (m *mockLocalOperations) updateInstanceStatus(ctx context.Context, di *sqladmin.DatabaseInstance) error {
	return m.mockUpdateInstanceStatus(ctx, di)
}
func (m *mockLocalOperations) updateReconcileStatus(ctx context.Context, err error) error {
	return m.mockUpdateReconcileStatus(ctx, err)
}
func (m *mockLocalOperations) updateConnectionSecret(ctx context.Context) (*core.Secret, error) {
	return m.mockUpdateConnectionSecret(ctx)
}

type mockManagedOperations struct {
	localOperations

	// DatabaseInstance managedOperations
	mockGetInstance    func(context.Context) (*sqladmin.DatabaseInstance, error)
	mockCreateInstance func(context.Context) error
	mockUpdateInstance func(context.Context) error
	mockDeleteInstance func(context.Context) error

	// DatabaseUser managedOperations
	mockUpdateUserCreds func(context.Context) error
}

var _ managedOperations = &mockManagedOperations{}

func (m *mockManagedOperations) getInstance(ctx context.Context) (*sqladmin.DatabaseInstance, error) {
	return m.mockGetInstance(ctx)
}
func (m *mockManagedOperations) createInstance(ctx context.Context) error {
	return m.mockCreateInstance(ctx)
}
func (m *mockManagedOperations) updateInstance(ctx context.Context) error {
	return m.mockUpdateInstance(ctx)
}
func (m *mockManagedOperations) deleteInstance(ctx context.Context) error {
	return m.mockDeleteInstance(ctx)
}
func (m *mockManagedOperations) updateUserCreds(ctx context.Context) error {
	return m.mockUpdateUserCreds(ctx)
}

type mockFactory struct {
	mockMakeLocalOperations   func(*v1alpha1.CloudsqlInstance, client.Client) localOperations
	mockMakeManagedOperations func(context.Context, *v1alpha1.CloudsqlInstance, localOperations) (managedOperations, error)
	mockMakeSyncDeleter       func(managedOperations) syncdeleter
}

var _ factory = &mockFactory{}

func (m *mockFactory) makeLocalOperations(inst *v1alpha1.CloudsqlInstance, kube client.Client) localOperations {
	return m.mockMakeLocalOperations(inst, kube)
}
func (m *mockFactory) makeManagedOperations(ctx context.Context, inst *v1alpha1.CloudsqlInstance, ops localOperations) (managedOperations, error) {
	return m.mockMakeManagedOperations(ctx, inst, ops)
}
func (m *mockFactory) makeSyncDeleter(ops managedOperations) syncdeleter {
	return m.mockMakeSyncDeleter(ops)
}

type mockSyncDeleter struct {
	mockSync   func(context.Context) (reconcile.Result, error)
	mockDelete func(context.Context) (reconcile.Result, error)
}

var _ syncdeleter = &mockSyncDeleter{}

func (m *mockSyncDeleter) sync(ctx context.Context) (reconcile.Result, error) {
	return m.mockSync(ctx)
}
func (m *mockSyncDeleter) delete(ctx context.Context) (reconcile.Result, error) {
	return m.mockDelete(ctx)
}

type instanceSpec struct {
	*corev1alpha1.ResourceSpec
}

func newInstanceSpec() *instanceSpec {
	return &instanceSpec{
		ResourceSpec: &corev1alpha1.ResourceSpec{},
	}
}
func (i *instanceSpec) build() *corev1alpha1.ResourceSpec {
	return i.ResourceSpec
}
func (i *instanceSpec) withProviderRef(ref *core.ObjectReference) *instanceSpec {
	i.ProviderReference = ref
	return i
}
func (i *instanceSpec) withReclaimPolicy(p corev1alpha1.ReclaimPolicy) *instanceSpec {
	i.ReclaimPolicy = p
	return i
}
func (i *instanceSpec) withWriteConnectionSecretRef(ref core.LocalObjectReference) *instanceSpec {
	i.WriteConnectionSecretToReference = ref
	return i
}

type instanceStatus struct {
	*corev1alpha1.ResourceStatus
}

func newInstanceStatus() *instanceStatus {
	return &instanceStatus{
		ResourceStatus: &corev1alpha1.ResourceStatus{},
	}
}
func (i *instanceStatus) build() *corev1alpha1.ResourceStatus {
	return i.ResourceStatus
}
func (i *instanceStatus) withConditions(c ...corev1alpha1.Condition) *instanceStatus {
	i.ResourceStatus.Conditions = c
	return i
}

type instance struct {
	*v1alpha1.CloudsqlInstance
}

func newInstance() *instance {
	return &instance{
		CloudsqlInstance: &v1alpha1.CloudsqlInstance{},
	}
}
func (i *instance) build() *v1alpha1.CloudsqlInstance {
	return i.CloudsqlInstance
}
func (i *instance) withFinalizers(f []string) *instance {
	i.Finalizers = f
	return i
}
func (i *instance) withObjectMeta(om meta1.ObjectMeta) *instance {
	i.ObjectMeta = om
	return i
}
func (i *instance) withResourceSpec(rs *corev1alpha1.ResourceSpec) *instance {
	i.Spec.ResourceSpec = *rs
	return i
}

func assertObjectInstance(obj runtime.Object) (*v1alpha1.CloudsqlInstance, error) {
	inst, ok := obj.(*v1alpha1.CloudsqlInstance)
	if !ok {
		return nil, errors.Errorf("unexpected object type: %T", obj)
	}
	return inst, nil
}

// Because the instance name creation is determined by some logic,
// putting the test logic to calculate the expected instance name inside
// this method allows us to maintain it in a single place.
func getExpectedInstanceName(testUID string) string {
	return strings.ToLower(v1alpha1.CloudsqlInstanceKind + "-" + testUID)
}

func Test_localHandler_addFinalizer(t *testing.T) {
	type fields struct {
		instance *v1alpha1.CloudsqlInstance
		kube     client.Client
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		err        error
		finalizers []string
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"NoFinalizer": {
			fields: fields{
				instance: &v1alpha1.CloudsqlInstance{},
				kube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						if _, err := assertObjectInstance(obj); err != nil {
							t.Errorf("addFinalizer(): %v", err)
						}
						return nil
					},
				},
			},
			want: want{
				finalizers: []string{finalizer},
			},
		},
		"SomeOtherFinalizer": {
			fields: fields{
				instance: newInstance().withFinalizers([]string{"foo"}).build(),
				kube:     test.NewMockClient(),
			},
			want: want{
				finalizers: []string{"foo", finalizer},
			},
		},
		"ExistingFinalizer": {
			fields: fields{
				instance: newInstance().withFinalizers([]string{"foo", finalizer}).build(),
				kube:     test.NewMockClient(),
			},
			want: want{
				finalizers: []string{"foo", finalizer},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := newLocalHandler(tt.fields.instance, tt.fields.kube)
			if diff := cmp.Diff(tt.want.err, ih.addFinalizer(tt.args.ctx)); diff != "" {
				t.Errorf("addFinalizer() error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tt.want.finalizers, tt.fields.instance.Finalizers); diff != "" {
				t.Errorf("addFinalizer() -want, +got: %s", diff)
			}
		})
	}
}

func Test_localHandler_removeFinalizer(t *testing.T) {
	type fields struct {
		instance *v1alpha1.CloudsqlInstance
		kube     client.Client
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		err        error
		finalizers []string
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"NoFinalizer": {
			fields: fields{
				instance: &v1alpha1.CloudsqlInstance{},
				kube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						if _, err := assertObjectInstance(obj); err != nil {
							t.Errorf("removeFinalizer(): %v", err)
						}
						return nil
					},
				},
			},
		},
		"SomeOtherFinalizer": {
			fields: fields{
				instance: newInstance().withFinalizers([]string{"foo"}).build(),
				kube:     test.NewMockClient(),
			},
			want: want{
				finalizers: []string{"foo"},
			},
		},
		"ExistingFinalizer": {
			fields: fields{
				instance: newInstance().withFinalizers([]string{"foo", finalizer}).build(),
				kube:     test.NewMockClient(),
			},
			want: want{
				finalizers: []string{"foo"},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: tt.fields.instance,
				client:           tt.fields.kube,
			}
			if diff := cmp.Diff(tt.want.err, ih.removeFinalizer(tt.args.ctx)); diff != "" {
				t.Errorf("removeFinalizer() error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tt.want.finalizers, tt.fields.instance.Finalizers); diff != "" {
				t.Errorf("removeFinalizer() -want, +got: %s", diff)
			}
		})
	}
}

func Test_localHandler_isInstanceReady(t *testing.T) {
	tests := map[string]struct {
		status v1alpha1.CloudsqlInstanceStatus
		want   bool
	}{
		"Default": {
			status: v1alpha1.CloudsqlInstanceStatus{},
			want:   false,
		},
		"Ready": {
			status: v1alpha1.CloudsqlInstanceStatus{
				State: v1alpha1.StateRunnable,
			},
			want: true,
		},
		"NotReady": {
			status: v1alpha1.CloudsqlInstanceStatus{
				State: "foo-bar",
			},
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: &v1alpha1.CloudsqlInstance{
					Status: tt.status,
				},
			}
			if got := ih.isInstanceReady(); got != tt.want {
				t.Errorf("isInstanceReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_localHandler_isReclaimDelete(t *testing.T) {
	tests := map[string]struct {
		inst *v1alpha1.CloudsqlInstance
		want bool
	}{
		"Default": {
			inst: &v1alpha1.CloudsqlInstance{},
			want: false,
		},
		"Delete": {
			inst: newInstance().withResourceSpec(
				newInstanceSpec().withReclaimPolicy(corev1alpha1.ReclaimDelete).build()).build(),
			want: true,
		},
		"Retain": {
			inst: newInstance().withResourceSpec(
				newInstanceSpec().withReclaimPolicy(corev1alpha1.ReclaimRetain).build()).build(),
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: tt.inst,
			}
			if got := ih.isReclaimDelete(); got != tt.want {
				t.Errorf("isReclaimDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_localHandler_needUpdate(t *testing.T) {
	handler := &localHandler{}
	if got := handler.needsUpdate(&sqladmin.DatabaseInstance{}); got {
		t.Errorf("needsUpdate() = %v, should always be false", got)
	}
}

func Test_localHandler_updateObject(t *testing.T) {
	type fields struct {
		obj  *v1alpha1.CloudsqlInstance
		kube client.Client
	}
	type args struct {
		ctx context.Context
	}
	inst := &v1alpha1.CloudsqlInstance{}
	assert := func(obj runtime.Object) {
		i, err := assertObjectInstance(obj)
		if err != nil {
			t.Errorf("updateObject() unexpectd type %T", obj)
		}
		if i != inst {
			t.Errorf("updateObject() unexpected instance: %v, should be: %v", i, inst)
		}
	}
	testError := errors.New("test-error")
	tests := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"Success": {
			fields: fields{
				obj: inst,
				kube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						assert(obj)
						return nil
					},
				},
			},
		},
		"Failure": {
			fields: fields{
				obj: inst,
				kube: &test.MockClient{
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						assert(obj)
						return testError
					},
				},
			},
			want: testError,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: tt.fields.obj,
				client:           tt.fields.kube,
			}
			if diff := cmp.Diff(tt.want, ih.updateObject(tt.args.ctx), test.EquateErrors()); diff != "" {
				t.Errorf("updateObject() error -want, +got: %s", diff)
			}
		})
	}
}

func Test_localHandler_updateInstanceStatus(t *testing.T) {
	testError := errors.New("test-error")
	inst := &v1alpha1.CloudsqlInstance{}
	assert := func(obj runtime.Object) {
		i, err := assertObjectInstance(obj)
		if err != nil {
			t.Errorf("updateObject() unexpectd type %T", obj)
		}
		if i != inst {
			t.Errorf("updateObject() unexpected instance: %v, should be: %v", i, inst)
		}
	}

	type fields struct {
		inst *v1alpha1.CloudsqlInstance
		kube client.Client
	}
	type args struct {
		ctx  context.Context
		inst *sqladmin.DatabaseInstance
	}
	type want struct {
		err    error
		status *v1alpha1.CloudsqlInstanceStatus
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Success": {
			fields: fields{
				inst: inst,
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						assert(obj)
						return nil
					},
				},
			},
			args: args{
				inst: &sqladmin.DatabaseInstance{
					State: "Fooing",
				},
			},
			want: want{
				status: &v1alpha1.CloudsqlInstanceStatus{
					State: "Fooing",
				},
			},
		},
		"Failure": {
			fields: fields{
				inst: inst,
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						assert(obj)
						return testError
					},
				},
			},
			want: want{
				err: testError,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: tt.fields.inst,
				client:           tt.fields.kube,
			}
			if diff := cmp.Diff(tt.want.err, ih.updateInstanceStatus(tt.args.ctx, tt.args.inst), test.EquateErrors()); diff != "" {
				t.Errorf("updateInstanceStatus() -want, +got: %s", diff)
			}
		})
	}
}

func Test_localHandler_updateReconcileStatus(t *testing.T) {
	testError := errors.New("test-error")
	inst := &v1alpha1.CloudsqlInstance{}
	assert := func(obj runtime.Object) {
		i, err := assertObjectInstance(obj)
		if err != nil {
			t.Errorf("updateObject() unexpectd type %T", obj)
		}
		if i != inst {
			t.Errorf("updateObject() unexpected instance: %v, should be: %v", i, inst)
		}
	}

	type fields struct {
		inst *v1alpha1.CloudsqlInstance
		kube client.Client
	}
	type args struct {
		ctx context.Context
		err error
	}
	type want struct {
		err    error
		status v1alpha1.CloudsqlInstanceStatus
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"UpdateFailure": {
			fields: fields{
				inst: inst,
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						assert(obj)
						return testError
					},
				},
			},
			want: want{
				err: testError,
				status: v1alpha1.CloudsqlInstanceStatus{
					ResourceStatus: *newInstanceStatus().withConditions(corev1alpha1.ReconcileSuccess()).build(),
				},
			},
		},
		"UpdateSuccessReconcileSuccess": {
			fields: fields{
				inst: inst,
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						assert(obj)
						return nil
					},
				},
			},
			want: want{
				status: v1alpha1.CloudsqlInstanceStatus{
					ResourceStatus: *newInstanceStatus().withConditions(corev1alpha1.ReconcileSuccess()).build(),
				},
			},
		},
		"UpdateSuccessReconcileError": {
			fields: fields{
				inst: inst,
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error {
						assert(obj)
						return nil
					},
				},
			},
			args: args{
				err: testError,
			},
			want: want{
				status: v1alpha1.CloudsqlInstanceStatus{
					ResourceStatus: *newInstanceStatus().withConditions(corev1alpha1.ReconcileError(testError)).build(),
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: tt.fields.inst,
				client:           tt.fields.kube,
			}
			if diff := cmp.Diff(tt.want.err, ih.updateReconcileStatus(tt.args.ctx, tt.args.err), test.EquateErrors()); diff != "" {
				t.Errorf("updateReconcileStatus() error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tt.want.status, ih.Status); diff != "" {
				t.Errorf("updateReconcileStatus() -want, +got: %s", diff)
			}
		})
	}
}

func Test_localHandler_getConnectionSecret(t *testing.T) {
	type fields struct {
		inst *v1alpha1.CloudsqlInstance
		kube client.Client
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		sec *core.Secret
		err error
	}
	testError := errors.New("test-error")
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"FailedToGet": {
			fields: fields{
				inst: newInstance().
					withObjectMeta(testMeta).
					withResourceSpec(
						newInstanceSpec().
							withWriteConnectionSecretRef(core.LocalObjectReference{Name: "test-secret"}).
							build()).build(),
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						if _, ok := obj.(*core.Secret); !ok {
							t.Errorf("getConnectionSecret() unexpected object type %T", obj)
						}
						if diff := cmp.Diff(types.NamespacedName{Namespace: testNs, Name: "test-secret"}, key); diff != "" {
							t.Errorf("getConnectionSecret() unexpected key -want, +got: %s", diff)
						}
						return testError
					},
				},
			},
			want: want{
				err: testError,
				sec: &core.Secret{},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: tt.fields.inst,
				client:           tt.fields.kube,
			}
			got, err := ih.getConnectionSecret(tt.args.ctx)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("getConnectionSecret() error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tt.want.sec, got); diff != "" {
				t.Errorf("getConnectionSecret() -want, +got: %s", diff)
			}
		})
	}
}

func Test_localHandler_updateConnectionSecret(t *testing.T) {
	assertKey := func(key client.ObjectKey) {
		if key != testKey {
			t.Errorf("updateConnectionSecret() unexpected key: %v, expected: %v", key, testKey)
		}
	}
	assertObj := func(obj runtime.Object) *core.Secret {
		s, ok := obj.(*core.Secret)
		if !ok {
			t.Errorf("updateConnectionSecret() unexpected object type: %T", obj)
		}
		return s
	}

	type fields struct {
		inst *v1alpha1.CloudsqlInstance
		kube client.Client
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		sec *core.Secret
		err error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"DoesntExistCreateNew": {
			fields: fields{
				inst: newInstance().
					withObjectMeta(testMeta).
					withResourceSpec(
						newInstanceSpec().
							withWriteConnectionSecretRef(core.LocalObjectReference{Name: testName}).build()).build(),
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						assertKey(key)
						assertObj(obj)
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						assertObj(obj)
						return nil
					},
					MockUpdate: nil,
				},
			},
			want: want{
				sec: testSecret("", ""),
			},
		},
		"ExistsButBelongsToAnother": {
			fields: fields{
				inst: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
					Spec: v1alpha1.CloudsqlInstanceSpec{
						ResourceSpec: *newInstanceSpec().
							withWriteConnectionSecretRef(core.LocalObjectReference{Name: testName}).build(),
					},
					Status: v1alpha1.CloudsqlInstanceStatus{
						Endpoint: "new-ep",
					},
				},
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						assertKey(key)
						s := assertObj(obj)
						ts := testSecret("test-ep", "test-pass")
						ts.OwnerReferences[0].UID = "foo"
						ts.DeepCopyInto(s)
						return nil
					},
				},
			},
			want: want{
				sec: nil,
				err: errors.Wrap(errors.New("connection secret test-ns/test-name exists and is not controlled by test-ns/test-name"),
					"could not mutate object for update"),
			},
		},
		"ExistsUpdateEndpoint": {
			fields: fields{
				inst: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
					Spec: v1alpha1.CloudsqlInstanceSpec{
						ResourceSpec: *newInstanceSpec().
							withWriteConnectionSecretRef(core.LocalObjectReference{Name: testName}).build(),
					},
					Status: v1alpha1.CloudsqlInstanceStatus{
						Endpoint: "new-ep",
					},
				},
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						assertKey(key)
						s := assertObj(obj)
						ts := testSecret("test-ep", "test-pass")
						ts.DeepCopyInto(s)
						return nil
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						assertObj(obj)
						return nil
					},
				},
			},
			want: want{
				sec: testSecret("new-ep", "test-pass"),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &localHandler{
				CloudsqlInstance: tt.fields.inst,
				client:           tt.fields.kube,
			}
			got, err := ih.updateConnectionSecret(tt.args.ctx)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("updateConnectionSecret() error -want, +got: %s\n%v\n%v", diff, tt.want.err, err)
			}

			if tt.want.sec == nil {
				if got != nil {
					t.Errorf("updateConnectionSecret() secret want: nil, got: %v", got)
				}
				return
			}

			// check for non-empty password, then reset to nil
			if string(got.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey]) == "" {
				t.Errorf("updateConnectionSecret() data, empty password field: %v", got)
			}
			got.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey] = nil
			tt.want.sec.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey] = nil
			if diff := cmp.Diff(tt.want.sec, got); diff != "" {
				t.Errorf("updateConnectionSecret() -want, +got: %s", diff)
			}
		})
	}
}

func Test_managedHandler_getInstance(t *testing.T) {
	type fields struct {
		obj      *v1alpha1.CloudsqlInstance
		instance cloudsql.InstanceService
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		status v1alpha1.CloudsqlInstanceStatus
		err    error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Default": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
				},
				instance: &fake.MockInstanceClient{
					MockGet: func(ctx context.Context, s string) (*sqladmin.DatabaseInstance, error) {
						expectedInstanceName := getExpectedInstanceName(testUID)
						if diff := cmp.Diff(expectedInstanceName, s); diff != "" {
							t.Errorf("getInstance() instance name -want, +got: %s", diff)
						}
						return &sqladmin.DatabaseInstance{
							IpAddresses: []*sqladmin.IpMapping{
								{IpAddress: "test.ip.address"},
							},
							State: "thinking-about",
						}, nil
					},
				},
			},
			want: want{
				status: v1alpha1.CloudsqlInstanceStatus{
					State:          "thinking-about",
					Endpoint:       "test.ip.address",
					ResourceStatus: *newInstanceStatus().withConditions(corev1alpha1.Unavailable()).build(),
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &managedHandler{
				CloudsqlInstance: tt.fields.obj,
				instance:         tt.fields.instance,
			}
			_, err := ih.getInstance(tt.args.ctx)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("getInstance() error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tt.want.status, tt.fields.obj.Status); diff != "" {
				t.Errorf("getInstance() -want, +got: %s", diff)
			}
		})
	}
}

func Test_managedHandler_createInstance(t *testing.T) {
	type fields struct {
		obj      *v1alpha1.CloudsqlInstance
		instance cloudsql.InstanceService
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		status v1alpha1.CloudsqlInstanceStatus
		err    error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Default": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
				},
				instance: &fake.MockInstanceClient{
					MockCreate: func(ctx context.Context, instance *sqladmin.DatabaseInstance) error {
						expectedInstanceName := getExpectedInstanceName(testUID)
						if instance == nil {
							t.Errorf("createInstance() create instance is nil")
							return nil
						}
						if diff := cmp.Diff(expectedInstanceName, instance.Name); diff != "" {
							t.Errorf("createInstance() create -want, +got: %s", diff)
						}
						return nil
					},
				},
			},
			want: want{
				status: v1alpha1.CloudsqlInstanceStatus{
					ResourceStatus: *newInstanceStatus().withConditions(corev1alpha1.Creating()).build(),
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &managedHandler{
				CloudsqlInstance: tt.fields.obj,
				instance:         tt.fields.instance,
			}
			if diff := cmp.Diff(tt.want.err, ih.createInstance(tt.args.ctx)); diff != "" {
				t.Errorf("createInstance() error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tt.want.status, tt.fields.obj.Status); diff != "" {
				t.Errorf("createInstance() status -want, +got: %s", diff)
			}
		})
	}
}

func Test_managedHandler_updateInstance(t *testing.T) {
	type fields struct {
		obj      *v1alpha1.CloudsqlInstance
		instance cloudsql.InstanceService
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		err error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Default": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
				},
				instance: &fake.MockInstanceClient{
					MockUpdate: func(ctx context.Context, name string, instance *sqladmin.DatabaseInstance) error {
						expectedInstanceName := getExpectedInstanceName(testUID)
						if instance == nil {
							t.Errorf("updateInstance() create instance is nil")
							return nil
						}
						if diff := cmp.Diff(expectedInstanceName, instance.Name); diff != "" {
							t.Errorf("updateInstance() create -want, +got: %s", diff)
						}
						if diff := cmp.Diff(expectedInstanceName, instance.Name); diff != "" {
							t.Errorf("updateInstance() create -want, +got: %s", diff)
						}
						return nil
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &managedHandler{
				CloudsqlInstance: tt.fields.obj,
				instance:         tt.fields.instance,
			}
			if diff := cmp.Diff(tt.want.err, ih.updateInstance(tt.args.ctx)); diff != "" {
				t.Errorf("updateInstance() error -want, +got: %s", diff)
			}
		})
	}
}

func Test_managedHandler_deleteInstance(t *testing.T) {
	type fields struct {
		obj      *v1alpha1.CloudsqlInstance
		instance cloudsql.InstanceService
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		err error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"Default": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
				},
				instance: &fake.MockInstanceClient{
					MockDelete: func(ctx context.Context, name string) error {
						expectedInstanceName := getExpectedInstanceName(testUID)
						if diff := cmp.Diff(expectedInstanceName, name); diff != "" {
							t.Errorf("deleteInstance() create -want, +got: %s", diff)
						}
						return nil
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &managedHandler{
				CloudsqlInstance: tt.fields.obj,
				instance:         tt.fields.instance,
			}
			if diff := cmp.Diff(tt.want.err, ih.deleteInstance(tt.args.ctx)); diff != "" {
				t.Errorf("deleteInstance() error -want, +got: %s", diff)
			}
		})
	}
}

func Test_managedHandler_getUser(t *testing.T) {
	type fields struct {
		obj  *v1alpha1.CloudsqlInstance
		user cloudsql.UserService
	}
	type args struct {
		ctx context.Context
	}
	type want struct {
		user *sqladmin.User
		err  error
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   want
	}{
		"UserFound": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
				},
				user: &fake.MockUserClient{
					MockList: func(i context.Context, s string) (users []*sqladmin.User, e error) {
						expectedInstanceName := getExpectedInstanceName(testUID)
						if s != expectedInstanceName {
							t.Errorf("getUser() list - unexpected instance name %s", s)
						}
						return []*sqladmin.User{
							{Name: "foo"}, {Name: "bar"}, {Name: v1alpha1.MysqlDefaultUser},
						}, nil
					},
				},
			},
			want: want{
				user: &sqladmin.User{Name: v1alpha1.MysqlDefaultUser},
			},
		},
		"UserNotFound": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
				},
				user: &fake.MockUserClient{
					MockList: func(i context.Context, s string) (users []*sqladmin.User, e error) {
						expectedInstanceName := getExpectedInstanceName(testUID)
						if s != expectedInstanceName {
							t.Errorf("getUser() list - unexpected instance name %s", s)
						}
						return []*sqladmin.User{
							{Name: "foo"}, {Name: "bar"},
						}, nil
					},
				},
			},
			want: want{
				err: &googleapi.Error{
					Code:    http.StatusNotFound,
					Message: "user: root is not found",
				},
			},
		},
		"ListError": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{
					ObjectMeta: testMeta,
				},
				user: &fake.MockUserClient{
					MockList: func(i context.Context, s string) (users []*sqladmin.User, e error) {
						return nil, errTest
					},
				},
			},
			want: want{
				err: errTest,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &managedHandler{
				CloudsqlInstance: tt.fields.obj,
				user:             tt.fields.user,
			}
			got, err := ih.getUser(tt.args.ctx)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("getUser() error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tt.want.user, got); diff != "" {
				t.Errorf("getUser() -want, +got: %s", diff)
			}
		})
	}
}

func Test_managedHandler_updateUserCreds(t *testing.T) {
	type fields struct {
		obj  *v1alpha1.CloudsqlInstance
		ops  localOperations
		user cloudsql.UserService
	}
	type args struct {
		ctx context.Context
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"FailedToUpdateConnectionSecret": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{},
				ops: &mockLocalOperations{
					mockUpdateConnectionSecret: func(ctx context.Context) (*core.Secret, error) {
						return nil, errTest
					},
				},
			},
			want: errors.Wrapf(errTest, "failed to update connection secret"),
		},
		"FailedToGetUser": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{},
				ops: &mockLocalOperations{
					mockUpdateConnectionSecret: func(ctx context.Context) (*core.Secret, error) {
						return testSecret("foo", "bar"), nil
					},
				},
				user: &fake.MockUserClient{
					MockList: func(ctx context.Context, s string) ([]*sqladmin.User, error) {
						return nil, errTest
					},
				},
			},
			want: errors.Wrapf(errTest, "failed to get user"),
		},
		"SuccessfulUpdate": {
			fields: fields{
				obj: &v1alpha1.CloudsqlInstance{},
				ops: &mockLocalOperations{
					mockUpdateConnectionSecret: func(ctx context.Context) (*core.Secret, error) {
						return testSecret("new-endpoint", "new-password"), nil
					},
				},
				user: &fake.MockUserClient{
					MockList: func(ctx context.Context, s string) ([]*sqladmin.User, error) {
						return []*sqladmin.User{
							{Name: v1alpha1.MysqlDefaultUser, Password: "old-password"},
						}, nil
					},
					MockUpdate: func(ctx context.Context, s string, s2 string, user *sqladmin.User) error {
						if user.Password != "new-password" {
							t.Errorf("updateUserCreds() update should change the password")
						}
						return nil
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ih := &managedHandler{
				CloudsqlInstance: tt.fields.obj,
				localOperations:  tt.fields.ops,
				user:             tt.fields.user,
			}
			if diff := cmp.Diff(tt.want, ih.updateUserCreds(tt.args.ctx), test.EquateErrors()); diff != "" {
				t.Errorf("updateUserCreds() error -want, +got: %s", diff)
			}
		})
	}
}

func Test_newManagedHandler(t *testing.T) {
	type args struct {
		ctx      context.Context
		instance *v1alpha1.CloudsqlInstance
		ops      localOperations
		creds    *google.Credentials
	}
	type want struct {
		err error
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"Default": {
			args: args{
				creds: &google.Credentials{},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newManagedHandler(tt.args.ctx, tt.args.instance, tt.args.ops, tt.args.creds)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("newManagedHandler() error -want, +got: %s", diff)
			}
		})
	}
}

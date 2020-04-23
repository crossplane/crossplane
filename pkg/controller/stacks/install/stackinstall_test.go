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

package install

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	stacksapi "github.com/crossplane/crossplane/apis/stacks"
	"github.com/crossplane/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/stacks/hosted"
	"github.com/crossplane/crossplane/pkg/stacks"
)

const (
	namespace               = "cool-namespace"
	hostControllerNamespace = "controller-namespace"
	uidString               = "definitely-a-uuid"
	uid                     = types.UID(uidString)
	resourceName            = "cool-stackinstall"
	stackPackageImage       = "cool/stack-package:rad"
	tsControllerImage       = "cool/fake-ts-controller:0.0.0"
	noForcedImagePullPolicy = ""
)

var (
	ctx     = context.Background()
	errBoom = errors.New("boom")
)

func init() {
	_ = stacksapi.AddToScheme(scheme.Scheme)
	_ = apiextensions.AddToScheme(scheme.Scheme)

}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

// Resource modifiers
type resourceModifier func(v1alpha1.StackInstaller)

func withFinalizers(finalizers ...string) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetFinalizers(finalizers) }
}

func withResourceVersion(version string) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetResourceVersion(version) }
}

func withConditions(c ...runtimev1alpha1.Condition) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetConditions(c...) }
}

func withDeletionTimestamp(t time.Time) resourceModifier {
	return func(r v1alpha1.StackInstaller) {
		r.SetDeletionTimestamp(&metav1.Time{Time: t})
	}
}

func withInstallJob(jobRef *corev1.ObjectReference) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetInstallJob(jobRef) }
}

// TODO(displague) this should be used in a test that asserts stackinstalls
// get status.stacks when the stack already exists and is properly labeled
//nolint:deadcode,unused
func withStackRecord(stackRecord *corev1.ObjectReference) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetStackRecord(stackRecord) }
}

// withPackage allows a test to set a StackInstaller's package
// Another option would have been to modify the interface to allow this,
// but it is preferable if we treat the package field as immutable.
func withPackage(pkg string) resourceModifier {
	return func(r v1alpha1.StackInstaller) {
		if si, ok := r.(*v1alpha1.StackInstall); ok {
			si.Spec.Package = pkg
		} else if csi, ok := r.(*v1alpha1.ClusterStackInstall); ok {
			csi.Spec.Package = pkg
		}
	}
}

func withSource(src string) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetSource(src) }
}

func withImagePullPolicy(policy corev1.PullPolicy) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetImagePullPolicy(policy) }
}

func withImagePullSecrets(secrets []corev1.LocalObjectReference) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetImagePullSecrets(secrets) }
}

func withGVK(gvk schema.GroupVersionKind) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetGroupVersionKind(gvk) }
}

func stackInstallResource(rm ...resourceModifier) *v1alpha1.StackInstall {
	r := &v1alpha1.StackInstall{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       resourceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.StackInstallSpec{},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

func clusterInstallResource(rm ...resourceModifier) *v1alpha1.ClusterStackInstall {
	r := &v1alpha1.ClusterStackInstall{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       resourceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.StackInstallSpec{},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

// mock implementations
type mockFactory struct {
	MockNewHandler func(logging.Logger, v1alpha1.StackInstaller, k8sClients, *hosted.Config, *stacks.ExecutorInfo, string, string) handler
}

func (f *mockFactory) newHandler(log logging.Logger, i v1alpha1.StackInstaller, k8s k8sClients, hostAwareConfig *hosted.Config, ei *stacks.ExecutorInfo, tsControllerImage, forceImagePullPolicy string) handler {
	return f.MockNewHandler(log, i, k8s, hostAwareConfig, ei, tsControllerImage, forceImagePullPolicy)
}

type mockHandler struct {
	MockSync   func(context.Context) (reconcile.Result, error)
	MockCreate func(context.Context) (reconcile.Result, error)
	MockUpdate func(context.Context) (reconcile.Result, error)
	MockDelete func(context.Context) (reconcile.Result, error)
}

func (m *mockHandler) sync(ctx context.Context) (reconcile.Result, error) {
	return m.MockSync(ctx)
}

func (m *mockHandler) create(ctx context.Context) (reconcile.Result, error) {
	return m.MockCreate(ctx)
}

func (m *mockHandler) update(ctx context.Context) (reconcile.Result, error) {
	return m.MockUpdate(ctx)
}

func (m *mockHandler) delete(ctx context.Context) (reconcile.Result, error) {
	return m.MockDelete(ctx)
}

type mockExecutorInfoDiscoverer struct {
	MockDiscoverExecutorInfo func(ctx context.Context) (*stacks.ExecutorInfo, error)
}

func (m *mockExecutorInfoDiscoverer) Discover(ctx context.Context) (*stacks.ExecutorInfo, error) {
	return m.MockDiscoverExecutorInfo(ctx)
}

func TestReconcile(t *testing.T) {
	type want struct {
		result       reconcile.Result
		stackInstall *v1alpha1.StackInstall
		err          error
	}

	tests := []struct {
		name string
		req  reconcile.Request
		rec  *Reconciler
		want want
	}{
		{
			name: "SuccessfulSyncStackInstall",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							*obj.(*v1alpha1.StackInstall) = *(stackInstallResource())
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					hostKube:   nil,
					hostClient: nil,
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(logging.Logger, v1alpha1.StackInstaller, k8sClients, *hosted.Config, *stacks.ExecutorInfo, string, string) handler {
						return &mockHandler{
							MockSync: func(context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}
					},
				},
				log: logging.NewNopLogger(),
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "SuccessfulSyncClusterStackInstall",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							*obj.(*v1alpha1.ClusterStackInstall) = *(clusterInstallResource())
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.ClusterStackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(logging.Logger, v1alpha1.StackInstaller, k8sClients, *hosted.Config, *stacks.ExecutorInfo, string, string) handler {
						return &mockHandler{
							MockSync: func(context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}
					},
				},
				log: logging.NewNopLogger(),
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "SuccessfulSyncFoundStack",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: fake.NewFakeClient(
						stackInstallResource(),
						&v1alpha1.Stack{ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace, UID: uid}},
					),
					hostKube:   nil,
					hostClient: nil,
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &handlerFactory{},
				log:     logging.NewNopLogger(),
			},
			want: want{result: requeueOnSuccess,
				stackInstall: stackInstallResource(
					withStackRecord(&corev1.ObjectReference{
						Name: resourceName, Namespace: namespace, UID: uid, Kind: v1alpha1.StackKind,
						APIVersion: v1alpha1.StackGroupVersionKind.GroupVersion().String(),
					}),
					withConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess()),
					withFinalizers(installFinalizer),
				), err: nil},
		},
		{
			name: "SuccessfulDelete",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							*obj.(*v1alpha1.StackInstall) = *(stackInstallResource(withDeletionTimestamp(time.Now())))
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(logging.Logger, v1alpha1.StackInstaller, k8sClients, *hosted.Config, *stacks.ExecutorInfo, string, string) handler {
						return &mockHandler{
							MockDelete: func(context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}
					},
				},
				log: logging.NewNopLogger(),
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "StackGetFailed",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							switch obj := obj.(type) {
							case *v1alpha1.StackInstall:
								*obj = *(stackInstallResource())
							case *v1alpha1.Stack:
								return errBoom
							}
							return nil
						},
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
					},
					hostKube:   nil,
					hostClient: nil,
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &handlerFactory{},
				log:     logging.NewNopLogger(),
			},
			want: want{result: resultRequeue,
				stackInstall: stackInstallResource(),
				err:          nil,
			},
		},
		{
			name: "DiscoverExecutorInfoFailed",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							*obj.(*v1alpha1.StackInstall) = *(stackInstallResource())
							return nil
						},
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
					},
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return nil, errors.New("test-discover-executorInfo-error")
					},
				},
				factory: nil,
				log:     logging.NewNopLogger(),
			},
			want: want{result: resultRequeue, err: nil},
		},
		{
			name: "ConflictingInstallJobFound",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					hostKube: func() client.Client {
						si := stackInstallResource()
						labels := stacks.ParentLabels(si)
						labels[stacks.LabelParentNamespace] = "not-cool-namespace"
						job := job()
						job.SetLabels(labels)
						return fake.NewFakeClient(job)
					}(),
					kube: func() client.Client {
						si := stackInstallResource()
						return fake.NewFakeClient(si)
					}(),
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &handlerFactory{},
				log:     logging.NewNopLogger(),
			},
			want: want{result: resultRequeue, err: nil,
				stackInstall: stackInstallResource(
					withFinalizers(installFinalizer),
					withConditions(runtimev1alpha1.Creating(), runtimev1alpha1.ReconcileError(errors.Errorf("stale job %s/%s prevents stackinstall", namespace, resourceName))),
				)},
		},
		{
			name: "InstallJobFound",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					hostKube: func() client.Client {
						si := stackInstallResource()
						labels := stacks.ParentLabels(si)
						job := job()
						job.SetLabels(labels)
						return fake.NewFakeClient(job)
					}(),
					kube: func() client.Client {
						si := stackInstallResource()
						return fake.NewFakeClient(si)
					}(),
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &handlerFactory{},
				log:     logging.NewNopLogger(),
			},
			want: want{result: requeueOnSuccess, err: nil,
				stackInstall: stackInstallResource(
					withFinalizers(installFinalizer),
					withInstallJob(&corev1.ObjectReference{
						Name:       resourceName,
						Namespace:  namespace,
						Kind:       "Job",
						APIVersion: batchv1.SchemeGroupVersion.String(),
					}),
					withConditions(
						runtimev1alpha1.Creating(),
						runtimev1alpha1.ReconcileSuccess(),
					),
				),
			},
		},
		{
			name: "ResourceNotFound",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							return kerrors.NewNotFound(schema.GroupResource{Group: v1alpha1.Group}, key.Name)
						},
					},
				},
				stackinator:            func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: nil,
				factory:                nil,
				log:                    logging.NewNopLogger(),
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "ResourceGetError",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				k8sClients: k8sClients{
					kube: &test.MockClient{
						MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
							return errors.New("test-get-error")
						},
					},
				},
				stackinator:            func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: nil,
				factory:                nil,
				log:                    logging.NewNopLogger(),
			},
			want: want{result: reconcile.Result{}, err: errors.New("test-get-error")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			gotResult, gotErr := tt.rec.Reconcile(tt.req)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("Reconcile() -want, +got:\n%v", diff)
			}

			if tt.want.stackInstall != nil {
				got := &v1alpha1.StackInstall{}
				assertKubernetesObject(t, g, got, tt.want.stackInstall, tt.rec.kube)
			}
		})
	}
}

// TestStackInstallDelete tests the delete function of the stack install handler
func TestStackInstallDelete(t *testing.T) {
	tn := time.Now()

	type want struct {
		result reconcile.Result
		err    error
		si     *v1alpha1.StackInstall
	}

	tests := []struct {
		name    string
		handler *stackInstallHandler
		want    want
	}{
		{
			name: "FailList",
			handler: &stackInstallHandler{
				// stack install starts with a finalizer and a deletion timestamp
				ext: stackInstallResource(withFinalizers(installFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						return errBoom
					},
					MockDeleteAllOf:  func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
					MockUpdate:       func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: stackInstallResource(
					withFinalizers(installFinalizer),
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "FailDeleteAllOf",
			handler: &stackInstallHandler{
				// stack install starts with a finalizer and a deletion timestamp
				ext: stackInstallResource(withFinalizers(installFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						switch list := list.(type) {
						case *v1alpha1.StackList:
							list.Items = []v1alpha1.Stack{{}}
						case *v1alpha1.StackDefinitionList:
							list.Items = []v1alpha1.StackDefinition{{}}
						}
						return nil
					},
					MockDeleteAllOf:  func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return errBoom },
					MockUpdate:       func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: stackInstallResource(
					withFinalizers(installFinalizer),
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "FailDeleteAllOfHosted",
			handler: &stackInstallHandler{
				// stack install starts with a finalizer and a deletion timestamp
				ext:             stackInstallResource(withFinalizers(installFinalizer), withDeletionTimestamp(tn)),
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						// set the list's items to a fake list of CRDs to delete
						switch list := list.(type) {
						case *v1alpha1.StackList:
							list.Items = []v1alpha1.Stack{}
						case *v1alpha1.StackDefinitionList:
							list.Items = []v1alpha1.StackDefinition{}
						}
						return nil
					},
					MockDeleteAllOf:  func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
					MockUpdate:       func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error { return nil },
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return errBoom },
				},
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: stackInstallResource(
					withFinalizers(installFinalizer),
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "FailUpdate",
			handler: &stackInstallHandler{
				// stack install starts with a finalizer and a deletion timestamp
				ext: stackInstallResource(withFinalizers(installFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						switch list := list.(type) {
						case *v1alpha1.StackList:
							list.Items = []v1alpha1.Stack{}
						case *v1alpha1.StackDefinitionList:
							list.Items = []v1alpha1.StackDefinition{}
						}
						return nil
					},
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
					MockUpdate: func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
						return errBoom
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
				},
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: stackInstallResource(
					// the finalizer will have been removed from our test object at least in memory
					// (even though the update call to the API server failed)
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "RetryWhenStackExists",
			handler: &stackInstallHandler{
				// stack install starts with a finalizer and a deletion timestamp
				ext: stackInstallResource(withFinalizers(installFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						switch list := list.(type) {
						case *v1alpha1.StackList:
							list.Items = []v1alpha1.Stack{{}}
						case *v1alpha1.StackDefinitionList:
							list.Items = []v1alpha1.StackDefinition{{}}
						}
						return nil
					},
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
					MockDelete:      func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error { return nil },
					MockUpdate: func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
						return errBoom
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: stackInstallResource(
					// the finalizer will have been removed from our test object at least in memory
					// (even though the update call to the API server failed)
					withDeletionTimestamp(tn),
					withFinalizers("finalizer.stackinstall.crossplane.io"),
					withConditions(runtimev1alpha1.ReconcileError(errors.New("Stack resources have not been deleted")))),
			},
		},
		{
			name: "SuccessfulDelete",
			handler: &stackInstallHandler{
				// stack install starts with a finalizer and a deletion timestamp
				ext: stackInstallResource(withFinalizers(installFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						switch list := list.(type) {
						case *v1alpha1.StackList:
							list.Items = []v1alpha1.Stack{}
						case *v1alpha1.StackDefinitionList:
							list.Items = []v1alpha1.StackDefinition{}
						}
						return nil
					},
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
					MockUpdate:      func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
				si:     stackInstallResource(withDeletionTimestamp(tn)), // finalizers get removed by delete function
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotErr := tt.handler.delete(ctx)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("delete() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("delete() -want result, +got result:\n%v", diff)
			}

			if diff := cmp.Diff(tt.want.si, tt.handler.ext, test.EquateConditions()); diff != "" {
				t.Errorf("delete() -want stackInstall, +got stackInstall:\n%v", diff)
			}
		})
	}
}

func TestHandlerFactory(t *testing.T) {
	tests := []struct {
		name    string
		factory factory
		want    handler
	}{
		{
			name:    "SimpleCreate",
			factory: &handlerFactory{},
			want: &stackInstallHandler{
				kube: nil,
				jobCompleter: &stackInstallJobCompleter{
					client:       nil,
					podLogReader: &K8sReader{Client: nil},
					log:          logging.NewNopLogger(),
				},
				executorInfo:             &stacks.ExecutorInfo{Image: stackPackageImage},
				ext:                      stackInstallResource(),
				log:                      logging.NewNopLogger(),
				templatesControllerImage: tsControllerImage,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.factory.newHandler(logging.NewNopLogger(), stackInstallResource(), k8sClients{}, nil, &stacks.ExecutorInfo{Image: stackPackageImage}, tsControllerImage, noForcedImagePullPolicy)

			diff := cmp.Diff(tt.want, got,
				cmp.AllowUnexported(
					stackInstallHandler{},
					stackInstallJobCompleter{},
					K8sReader{},
				))
			if diff != "" {
				t.Errorf("newHandler() -want, +got:\n%v", diff)
			}
		})
	}
}

type crdModifier func(*apiextensions.CustomResourceDefinition)

func withCRDVersion(version string) crdModifier {
	return func(c *apiextensions.CustomResourceDefinition) {
		c.Spec.Version = version
		c.Spec.Versions = append(c.Spec.Versions, apiextensions.CustomResourceDefinitionVersion{Name: version})
	}
}

func withCRDLabels(labels map[string]string) crdModifier {
	return func(c *apiextensions.CustomResourceDefinition) {
		meta.AddLabels(c, labels)
	}
}

func withCRDGroupKind(group, kind string) crdModifier {
	singular := strings.ToLower(kind)
	plural := singular + "s"
	list := kind + "List"

	return func(c *apiextensions.CustomResourceDefinition) {
		c.Spec.Group = group
		c.Spec.Names.Kind = kind
		c.Spec.Names.Plural = plural
		c.Spec.Names.ListKind = list
		c.Spec.Names.Singular = singular
		c.SetName(plural + "." + group)
	}
}

func withCRDDeletionTimestamp(t time.Time) crdModifier {
	return func(r *apiextensions.CustomResourceDefinition) {
		r.SetDeletionTimestamp(&metav1.Time{Time: t})
	}
}

func crd(cm ...crdModifier) apiextensions.CustomResourceDefinition {
	// basic crd with defaults
	t := true
	c := apiextensions.CustomResourceDefinition{
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Scope: "Namespaced",
			Conversion: &apiextensions.CustomResourceConversion{
				Strategy:                 apiextensions.NoneConverter,
				WebhookClientConfig:      nil,
				ConversionReviewVersions: nil,
			},
			PreserveUnknownFields: &t,
		},
	}
	for _, m := range cm {
		m(&c)
	}
	return c
}

func Test_stackInstallHandler_deleteOrphanedCRDs(t *testing.T) {
	type fields struct {
		clientFunc func() client.Client
		ext        *v1alpha1.StackInstall
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	var (
		nsLabel = fmt.Sprintf(stacks.LabelNamespaceFmt, namespace)

		// MultiParentLabels refer to a stack name. While stackInstallResource() is a
		// stackinstall, it is an object that has a name that matches the stack
		label = stacks.MultiParentLabel(stackInstallResource())
	)
	tests := []struct {
		name     string
		fields   fields
		want     []apiextensions.CustomResourceDefinition
		unwanted []apiextensions.CustomResourceDefinition
		wantErr  error
	}{
		{
			name: "FailedList",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					return &test.MockClient{
						MockList: test.NewMockListFn(errBoom),
					}
				},
			},
			want:    []apiextensions.CustomResourceDefinition{},
			wantErr: errBoom,
		},
		{
			name: "FailedDelete",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{stacks.LabelKubernetesManagedBy: stacks.LabelValueStackManager}),
					)
					f := fake.NewFakeClient(&c)
					return &test.MockClient{
						MockList:   f.List,
						MockDelete: test.NewMockDeleteFn(errBoom),
					}
				},
			},
			wantErr: errBoom,
		},
		{
			name: "Unmanaged",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))
					return fake.NewFakeClient(&c)
				},
			},
			want: []apiextensions.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version))},
		},
		{
			name: "AlreadyDeleted",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{stacks.LabelKubernetesManagedBy: stacks.LabelValueStackManager}),
					)
					f := fake.NewFakeClient(&c)
					return &test.MockClient{
						MockList:   f.List,
						MockGet:    f.Get,
						MockDelete: test.NewMockDeleteFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					}
				},
			},
			want: []apiextensions.CustomResourceDefinition{},
		},
		{
			name: "StillInUseDiscoveryLabels",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{stacks.LabelKubernetesManagedBy: stacks.LabelValueStackManager, nsLabel: "true"}),
					)
					return fake.NewFakeClient(&c)
				},
			},
			want: []apiextensions.CustomResourceDefinition{
				crd(
					withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{stacks.LabelKubernetesManagedBy: stacks.LabelValueStackManager, nsLabel: "true"}),
				),
			},
		},
		{
			name: "StillInUseMultiParentLabels",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{stacks.LabelKubernetesManagedBy: stacks.LabelValueStackManager, label: "true"}),
					)
					return fake.NewFakeClient(&c)
				},
			},
			want: []apiextensions.CustomResourceDefinition{
				crd(
					withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{stacks.LabelKubernetesManagedBy: stacks.LabelValueStackManager, label: "true"}),
				),
			},
		},
		{
			name: "SafeToDelete",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{stacks.LabelKubernetesManagedBy: stacks.LabelValueStackManager}),
					)
					return fake.NewFakeClient(&c)
				},
			},
			unwanted: []apiextensions.CustomResourceDefinition{crd(withCRDGroupKind(group, kind))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			h := &stackInstallHandler{
				kube: tt.fields.clientFunc(),
				ext:  tt.fields.ext,
				log:  logging.NewNopLogger(),
			}
			gotErr := h.deleteOrphanedCRDs(context.TODO())

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("stackHandler.deleteOrphanedCRDs(...): -want error, +got error: %s", diff)
			}

			if tt.want != nil {
				for _, wanted := range tt.want {
					got := &apiextensions.CustomResourceDefinition{}
					assertKubernetesObject(t, g, got, &wanted, h.kube)
				}
			}

			if tt.unwanted != nil {
				for _, unwanted := range tt.unwanted {
					got := &apiextensions.CustomResourceDefinition{}
					assertNoKubernetesObject(t, g, got, &unwanted, h.kube)
				}
			}

		})
	}
}

func Test_stackInstallHandler_removeCRDParentLabels(t *testing.T) {
	type fields struct {
		clientFunc func() client.Client
		ext        *v1alpha1.StackInstall
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	var (
		labels = stacks.ParentLabels(stackInstallResource())
	)
	tests := []struct {
		name     string
		fields   fields
		want     []apiextensions.CustomResourceDefinition
		unwanted []apiextensions.CustomResourceDefinition
		wantErr  error
	}{
		{
			name: "FailedList",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					return &test.MockClient{
						MockList: test.NewMockListFn(errBoom),
					}
				},
			},
			want:    []apiextensions.CustomResourceDefinition{},
			wantErr: errBoom,
		},
		{
			name: "FailedPatch",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(labels),
					)
					f := fake.NewFakeClient(&c)
					return &test.MockClient{
						MockList:  f.List,
						MockPatch: test.NewMockPatchFn(errBoom),
					}
				},
			},
			wantErr: errBoom,
		},
		{
			name: "Unlabeled",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))
					f := fake.NewFakeClient(&c)
					return &test.MockClient{
						MockList:  f.List,
						MockPatch: test.NewMockPatchFn(errBoom),
					}
				},
			},
			want:    []apiextensions.CustomResourceDefinition{},
			wantErr: nil,
		},
		{
			name: "Labeled",
			fields: fields{
				ext: stackInstallResource(),
				clientFunc: func() client.Client {
					c := crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(labels),
					)
					return fake.NewFakeClient(&c)
				},
			},
			want: []apiextensions.CustomResourceDefinition{crd(
				withCRDGroupKind(group, kind),
				withCRDVersion(version),
			),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			h := &stackInstallHandler{
				kube: tt.fields.clientFunc(),
				ext:  tt.fields.ext,
				log:  logging.NewNopLogger(),
			}
			gotErr := h.removeCRDParentLabels(labels)(context.TODO())

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("stackHandler.deleteOrphanedCRDs(...): -want error, +got error: %s", diff)
			}

			if tt.want != nil {
				for _, wanted := range tt.want {
					got := &apiextensions.CustomResourceDefinition{}
					assertKubernetesObject(t, g, got, &wanted, h.kube)
				}
			}

			if tt.unwanted != nil {
				for _, unwanted := range tt.unwanted {
					got := &apiextensions.CustomResourceDefinition{}
					assertNoKubernetesObject(t, g, got, &unwanted, h.kube)
				}
			}

		})
	}
}

type objectWithGVK interface {
	runtime.Object
	metav1.Object
}

func assertKubernetesObject(t *testing.T, g *GomegaWithT, got objectWithGVK, want metav1.Object, kube client.Client) {
	n := types.NamespacedName{Name: want.GetName(), Namespace: want.GetNamespace()}
	g.Expect(kube.Get(ctx, n, got)).NotTo(HaveOccurred())

	// NOTE(muvaf): retrieved objects have TypeMeta and
	// ObjectMeta.ResourceVersion filled but since we work on strong-typed
	// objects, we don't need to check them.
	got.SetResourceVersion(want.GetResourceVersion())

	if diff := cmp.Diff(want, got, test.EquateConditions(), ignoreGVK()); diff != "" {
		t.Errorf("-want, +got:\n%s", diff)
	}
}

func assertNoKubernetesObject(t *testing.T, g *GomegaWithT, got runtime.Object, unwanted metav1.Object, kube client.Client) {
	n := types.NamespacedName{Name: unwanted.GetName(), Namespace: unwanted.GetNamespace()}
	g.Expect(kube.Get(ctx, n, got)).To(HaveOccurred())
}

// ignoreGVK returns a cmp.Option that ignores the unstructured.Unstructured
// root map strings identified by r
func ignoreGVK() cmp.Option {
	return cmp.FilterPath(func(p cmp.Path) bool {
		s := p.GoString()
		roots := []string{"apiVersion", "kind"}
		for _, root := range roots {
			if s == `{*unstructured.Unstructured}.Object["`+root+`"].(string)` {
				return true
			}
		}

		if strings.HasSuffix(s, "TypeMeta.APIVersion") || strings.HasSuffix(s, "TypeMeta.Kind") {
			return true
		}

		return false
	}, cmp.Ignore())
}

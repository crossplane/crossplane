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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	stacksapi "github.com/crossplaneio/crossplane/apis/stacks"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/stacks"
)

const (
	namespace         = "cool-namespace"
	uid               = types.UID("definitely-a-uuid")
	resourceName      = "cool-stackinstall"
	stackPackageImage = "cool/stack-package:rad"
)

var (
	ctx = context.Background()
)

func init() {
	_ = stacksapi.AddToScheme(scheme.Scheme)
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

// Resource modifiers
type resourceModifier func(v1alpha1.StackInstaller)

func withConditions(c ...runtimev1alpha1.Condition) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetConditions(c...) }
}

func withInstallJob(jobRef *corev1.ObjectReference) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetInstallJob(jobRef) }
}

func withStackRecord(stackRecord *corev1.ObjectReference) resourceModifier {
	return func(r v1alpha1.StackInstaller) { r.SetStackRecord(stackRecord) }
}

func resource(rm ...resourceModifier) *v1alpha1.StackInstall {
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
	MockNewHandler func(context.Context, v1alpha1.StackInstaller, client.Client, kubernetes.Interface, *stacks.ExecutorInfo) handler
}

func (f *mockFactory) newHandler(ctx context.Context, i v1alpha1.StackInstaller,
	kube client.Client, kubeclient kubernetes.Interface, ei *stacks.ExecutorInfo) handler {
	return f.MockNewHandler(ctx, i, kube, kubeclient, ei)
}

type mockHandler struct {
	MockSync   func(context.Context) (reconcile.Result, error)
	MockCreate func(context.Context) (reconcile.Result, error)
	MockUpdate func(context.Context) (reconcile.Result, error)
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

type mockExecutorInfoDiscoverer struct {
	MockDiscoverExecutorInfo func(ctx context.Context) (*stacks.ExecutorInfo, error)
}

func (m *mockExecutorInfoDiscoverer) Discover(ctx context.Context) (*stacks.ExecutorInfo, error) {
	return m.MockDiscoverExecutorInfo(ctx)
}

func TestReconcile(t *testing.T) {
	type want struct {
		result reconcile.Result
		err    error
	}

	tests := []struct {
		name string
		req  reconcile.Request
		rec  *Reconciler
		want want
	}{
		{
			name: "SuccessfulSync",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.StackInstall) = *(resource())
						return nil
					},
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(context.Context, v1alpha1.StackInstaller, client.Client, kubernetes.Interface, *stacks.ExecutorInfo) handler {
						return &mockHandler{
							MockSync: func(context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}
					},
				},
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "SuccessfulSync",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.ClusterStackInstall) = *(clusterInstallResource())
						return nil
					},
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.ClusterStackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return &stacks.ExecutorInfo{Image: stackPackageImage}, nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(context.Context, v1alpha1.StackInstaller, client.Client, kubernetes.Interface, *stacks.ExecutorInfo) handler {
						return &mockHandler{
							MockSync: func(context.Context) (reconcile.Result, error) {
								return reconcile.Result{}, nil
							},
						}
					},
				},
			},
			want: want{result: reconcile.Result{}, err: nil},
		},

		{
			name: "DiscoverExecutorInfoFailed",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.StackInstall) = *(resource())
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				stackinator: func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: &mockExecutorInfoDiscoverer{
					MockDiscoverExecutorInfo: func(ctx context.Context) (*stacks.ExecutorInfo, error) {
						return nil, errors.New("test-discover-executorInfo-error")
					},
				},
				factory: nil,
			},
			want: want{result: resultRequeue, err: nil},
		},
		{
			name: "ResourceNotFound",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: v1alpha1.Group}, key.Name)
					},
				},
				stackinator:            func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: nil,
				factory:                nil,
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "ResourceGetError",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errors.New("test-get-error")
					},
				},
				stackinator:            func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} },
				executorInfoDiscoverer: nil,
				factory:                nil,
			},
			want: want{result: reconcile.Result{}, err: errors.New("test-get-error")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotErr := tt.rec.Reconcile(tt.req)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("Reconcile() -want, +got:\n%v", diff)
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
				kube:         nil,
				jobCompleter: &stackInstallJobCompleter{client: nil, podLogReader: &K8sReader{Client: nil}},
				executorInfo: &stacks.ExecutorInfo{Image: stackPackageImage},
				ext:          resource(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.factory.newHandler(ctx, resource(), nil, nil, &stacks.ExecutorInfo{Image: stackPackageImage})

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

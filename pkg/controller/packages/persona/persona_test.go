/*
Copyright 2020 The Crossplane Authors.

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

package persona

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/packages"
)

const (
	namespace = "cool-namespace"
	uid       = types.UID("definitely-a-uuid")
)

var (
	ctx     = context.Background()
	errBoom = errors.New("boom")

	expectedViewClusterRole = &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "crossplane:ns:" + namespace + ":view",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "Namespace", Name: namespace, UID: uid}},
			Labels: map[string]string{
				"namespace.crossplane.io/" + namespace: "true",
				"crossplane.io/scope":                  "namespace",
				packages.LabelParentGroup:              "",
				packages.LabelParentKind:               "",
				packages.LabelParentName:               namespace,
				packages.LabelParentNamespace:          "",
				packages.LabelParentVersion:            "",
			},
		},
		Rules: nil,
		AggregationRule: &rbac.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						"namespace.crossplane.io/cool-namespace":         "true",
						"rbac.crossplane.io/aggregate-to-namespace-view": "true",
					},
				},
				{
					MatchLabels: map[string]string{
						"rbac.crossplane.io/aggregate-to-namespace-default-view": "true",
					},
				},
			},
		},
	}
)

func init() {
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = rbac.AddToScheme(scheme.Scheme)
}

var _ reconcile.Reconciler = &Reconciler{}

type resourceModifier func(*corev1.Namespace)

func withDeletionTimestamp(t time.Time) resourceModifier {
	return func(ns *corev1.Namespace) {
		ns.SetDeletionTimestamp(&metav1.Time{Time: t})
	}
}

func withPersonaManagement() resourceModifier {
	return func(ns *corev1.Namespace) {
		meta.AddLabels(ns, personaEnablingLabels())
	}
}

func personaEnablingLabels() map[string]string {
	return map[string]string{managedRolesLabel: managedRolesEnabled}
}

func resource(rm ...resourceModifier) *corev1.Namespace {
	r := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			UID:  uid,
		},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

type mockFactory struct {
	MockNewHandler func(logging.Logger, *corev1.Namespace, client.Client) handler
}

func (f *mockFactory) newHandler(log logging.Logger, ns *corev1.Namespace, c client.Client) handler {
	return f.MockNewHandler(log, ns, c)
}

type mockHandler struct {
	MockSync   func(context.Context) (reconcile.Result, error)
	MockCreate func(context.Context) error
	MockDelete func(context.Context) error
}

func (m *mockHandler) sync(ctx context.Context) (reconcile.Result, error) {
	return m.MockSync(ctx)
}

func (m *mockHandler) create(ctx context.Context) error {
	return m.MockCreate(ctx)
}

func (m *mockHandler) delete(ctx context.Context) error {
	return m.MockDelete(ctx)
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
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*corev1.Namespace) = *(resource())
						return nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(logging.Logger, *corev1.Namespace, client.Client) handler {
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
			name: "ResourceNotFound",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: v1alpha1.Group}, key.Name)
					},
				},
				factory: nil,
				log:     logging.NewNopLogger(),
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "ResourceGetError",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return fmt.Errorf("test-get-error")
					},
				},
				factory: nil,
				log:     logging.NewNopLogger(),
			},
			want: want{result: reconcile.Result{}, err: fmt.Errorf("test-get-error")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotErr := tt.rec.Reconcile(tt.req)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("Reconcile() -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNSPersonaCreate(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		err    error
		cr     []*rbac.ClusterRole
		result reconcile.Result
	}

	tests := []struct {
		name       string
		ns         *corev1.Namespace
		clientFunc func(*corev1.Namespace) client.Client
		want       want
	}{
		{
			name:       "NoManagementRequested",
			ns:         resource(),
			clientFunc: func(ns *corev1.Namespace) client.Client { return fake.NewFakeClient(ns) },
			want: want{
				err:    nil,
				cr:     nil,
				result: reconcile.Result{},
			},
		},
		{
			name: "CreateClusterRoleError",
			ns:   resource(withPersonaManagement()),
			clientFunc: func(ns *corev1.Namespace) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*rbac.ClusterRole); ok {
							return errBoom
						}
						return nil
					},
				}
			},
			want: want{
				err:    errors.Wrap(errBoom, errFailedToCreateClusterRole),
				cr:     nil,
				result: resultRequeue,
			},
		},
		{
			name: "Success",
			ns:   resource(withPersonaManagement()),
			clientFunc: func(ns *corev1.Namespace) client.Client {
				return fake.NewFakeClient(ns)
			},
			want: want{
				err:    nil,
				cr:     []*rbac.ClusterRole{expectedViewClusterRole},
				result: reconcile.Result{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			handler := &nsPersonaHandler{
				kube: tt.clientFunc(tt.ns),
				ns:   tt.ns,
				log:  logging.NewNopLogger(),
			}

			gotResult, gotErr := handler.sync(ctx)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("create(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("delete() -want result, +got result:\n%v", diff)
			}

			if tt.want.cr != nil {
				for _, wanted := range tt.want.cr {
					got := &rbac.ClusterRole{}
					assertKubernetesObject(t, g, got, wanted, handler.kube)
				}
			}
		})
	}
}

// TestNamespaceDelete tests the delete function of the Namespace handler
func TestNSPersonaDelete(t *testing.T) {
	tn := time.Now()

	type want struct {
		result  reconcile.Result
		err     error
		ns      *corev1.Namespace
		present []*rbac.ClusterRole
		gone    []*rbac.ClusterRole
	}

	tests := []struct {
		name     string
		initObjs []runtime.Object
		clientFn func(initObjs ...runtime.Object) client.Client
		ns       *corev1.Namespace
		want     want
	}{
		{
			name:     "FailDeleteAllOf",
			ns:       resource(),
			initObjs: []runtime.Object{expectedViewClusterRole},
			clientFn: func(initObjs ...runtime.Object) client.Client {
				return &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, _ ...client.DeleteAllOfOption) error { return errBoom },
				}
			},
			want: want{
				result:  resultRequeue,
				err:     errors.Wrap(errBoom, errFailedToDeleteClusterRoles),
				ns:      resource(),
				present: []*rbac.ClusterRole{expectedViewClusterRole},
			},
		},
		{
			name:     "SuccessfulDelete",
			ns:       resource(withDeletionTimestamp(tn)),
			initObjs: []runtime.Object{expectedViewClusterRole},
			clientFn: fake.NewFakeClient,
			want: want{
				result: reconcile.Result{},
				err:    nil,
				ns:     resource(withDeletionTimestamp(tn)),
				gone:   []*rbac.ClusterRole{expectedViewClusterRole},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			handler := &nsPersonaHandler{
				kube: tt.clientFn(append(tt.initObjs, tt.ns)...),
				log:  logging.NewNopLogger(),
				ns:   tt.ns,
			}
			gotResult, gotErr := handler.sync(ctx)

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("delete() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("delete() -want result, +got result:\n%v", diff)
			}

			if diff := cmp.Diff(tt.want.ns, handler.ns, test.EquateConditions()); diff != "" {
				t.Errorf("delete() -want Namespace, +got Namespace:\n%v", diff)
			}

			if tt.want.present == nil {
				for _, wanted := range tt.want.present {
					got := &rbac.ClusterRole{}
					assertKubernetesObject(t, g, got, wanted, handler.kube)
				}
			}

			if tt.want.gone == nil {
				for _, unwanted := range tt.want.gone {
					got := &rbac.ClusterRole{}
					assertNoKubernetesObject(t, g, got, unwanted, handler.kube)
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
	got.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{})
	got.SetResourceVersion(want.GetResourceVersion())

	if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
		t.Errorf("-want, +got:\n%s", diff)
	}
}

func assertNoKubernetesObject(t *testing.T, g *GomegaWithT, got runtime.Object, unwanted metav1.Object, kube client.Client) {
	n := types.NamespacedName{Name: unwanted.GetName(), Namespace: unwanted.GetNamespace()}
	g.Expect(kube.Get(ctx, n, got)).To(HaveOccurred())
}

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

package extension

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/extensions"
	"github.com/crossplaneio/crossplane/pkg/apis/extensions/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace    = "cool-namespace"
	uid          = types.UID("definitely-a-uuid")
	resourceName = "cool-extension"

	controllerDeploymentName = "cool-controller-deployment"
	controllerContainerName  = "cool-container"
	controllerImageName      = "cool/controller-image:rad"
)

var (
	ctx = context.Background()
)

func init() {
	_ = extensions.AddToScheme(scheme.Scheme)
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

// ************************************************************************************************
// Resource modifiers
// ************************************************************************************************
type resourceModifier func(*v1alpha1.Extension)

func withConditions(c ...corev1alpha1.DeprecatedCondition) resourceModifier {
	return func(r *v1alpha1.Extension) { r.Status.DeprecatedConditionedStatus.Conditions = c }
}

func withControllerSpec(cs v1alpha1.ControllerSpec) resourceModifier {
	return func(r *v1alpha1.Extension) { r.Spec.Controller = cs }
}

func withPolicyRules(policyRules []rbac.PolicyRule) resourceModifier {
	return func(r *v1alpha1.Extension) { r.Spec.Permissions.Rules = policyRules }
}

func resource(rm ...resourceModifier) *v1alpha1.Extension {
	r := &v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       resourceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.ExtensionSpec{},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

// ************************************************************************************************
// mockFactory and mockHandler
// ************************************************************************************************
type mockFactory struct {
	MockNewHandler func(context.Context, *v1alpha1.Extension, client.Client) handler
}

func (f *mockFactory) newHandler(ctx context.Context, r *v1alpha1.Extension, c client.Client) handler {
	return f.MockNewHandler(ctx, r, c)
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

// ************************************************************************************************
// Default initializer functions
// ************************************************************************************************
func defaultControllerSpec() v1alpha1.ControllerSpec {
	return v1alpha1.ControllerSpec{
		Deployment: &v1alpha1.ControllerDeployment{
			Name: controllerDeploymentName,
			Spec: apps.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  controllerContainerName,
								Image: controllerImageName,
							},
						},
					},
				},
			},
		},
	}
}

func defaultPolicyRules() []rbac.PolicyRule {
	return []rbac.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}}}
}

// ************************************************************************************************
// TestReconcile
// ************************************************************************************************
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
						*obj.(*v1alpha1.Extension) = *(resource())
						return nil
					},
				},
				factory: &mockFactory{
					MockNewHandler: func(context.Context, *v1alpha1.Extension, client.Client) handler {
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
			name: "ResourceNotFound",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{Group: v1alpha1.Group}, key.Name)
					},
				},
				factory: nil,
			},
			want: want{result: reconcile.Result{}, err: nil},
		},
		{
			name: "ResourceGetError",
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return fmt.Errorf("test-get-error")
					},
				},
				factory: nil,
			},
			want: want{result: reconcile.Result{}, err: fmt.Errorf("test-get-error")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotErr := tt.rec.Reconcile(tt.req)

			if diff := cmp.Diff(gotErr, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile() want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(gotResult, tt.want.result); diff != "" {
				t.Errorf("Reconcile() got != want:\n%v", diff)
			}
		})
	}
}

// ************************************************************************************************
// TestCreate
// ************************************************************************************************
func TestCreate(t *testing.T) {
	type want struct {
		result reconcile.Result
		err    error
		r      *v1alpha1.Extension
	}

	tests := []struct {
		name       string
		r          *v1alpha1.Extension
		clientFunc func(*v1alpha1.Extension) client.Client
		want       want
	}{
		{
			name: "FailRBAC",
			r:    resource(withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						if _, ok := obj.(*corev1.ServiceAccount); ok {
							return errors.New("test-create-sa-error")
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				}
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				r: resource(
					withPolicyRules(defaultPolicyRules()),
					withConditions(corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonCreatingRBAC,
						Message: fmt.Errorf("failed to create service account: %+v", errors.New("test-create-sa-error")).Error(),
					})),
			},
		},
		{
			name: "FailDeployment",
			r: resource(
				withPolicyRules(defaultPolicyRules()),
				withControllerSpec(defaultControllerSpec())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						if _, ok := obj.(*apps.Deployment); ok {
							return errors.New("test-create-deployment-error")
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
				}
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				r: resource(
					withPolicyRules(defaultPolicyRules()),
					withControllerSpec(defaultControllerSpec()),
					withConditions(corev1alpha1.DeprecatedCondition{
						Type:    corev1alpha1.DeprecatedFailed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonCreatingDeployment,
						Message: fmt.Errorf("failed to create deployment: %+v", errors.New("test-create-deployment-error")).Error(),
					})),
			},
		},
		{
			name:       "SuccessfulCreate",
			r:          resource(),
			clientFunc: func(r *v1alpha1.Extension) client.Client { return fake.NewFakeClient(r) },
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				r: resource(
					withConditions(corev1alpha1.DeprecatedCondition{Type: corev1alpha1.DeprecatedReady, Status: corev1.ConditionTrue})),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &extensionHandler{
				kube: tt.clientFunc(tt.r),
				ext:  tt.r,
			}

			got, err := handler.create(ctx)

			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("create() want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(got, tt.want.result); diff != "" {
				t.Errorf("create() got != want:\n%v", diff)
			}

			if diff := cmp.Diff(tt.r, tt.want.r); diff != "" {
				t.Errorf("create() got != want:\n%v", diff)
			}
		})
	}
}

// ************************************************************************************************
// TestProcessRBAC
// ************************************************************************************************
func TestProcessRBAC(t *testing.T) {
	type want struct {
		err error
		sa  *corev1.ServiceAccount
		cr  *rbac.ClusterRole
		crb *rbac.ClusterRoleBinding
	}

	tests := []struct {
		name       string
		r          *v1alpha1.Extension
		clientFunc func(*v1alpha1.Extension) client.Client
		want       want
	}{
		{
			name:       "NoPermissionsRequested",
			r:          resource(),
			clientFunc: func(r *v1alpha1.Extension) client.Client { return fake.NewFakeClient(r) },
			want: want{
				err: nil,
				sa:  nil,
				cr:  nil,
				crb: nil,
			},
		},
		{
			name: "CreateServiceAccountError",
			r:    resource(withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						if _, ok := obj.(*corev1.ServiceAccount); ok {
							return errors.New("test-create-sa-error")
						}
						return nil
					},
				}
			},
			want: want{
				err: fmt.Errorf("failed to create service account: %+v", errors.New("test-create-sa-error")),
				sa:  nil,
				cr:  nil,
				crb: nil,
			},
		},
		{
			name: "CreateClusterRoleError",
			r:    resource(withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						if _, ok := obj.(*rbac.ClusterRole); ok {
							return errors.New("test-create-cr-error")
						}
						return nil
					},
				}
			},
			want: want{
				err: fmt.Errorf("failed to create cluster role: %+v", errors.New("test-create-cr-error")),
				sa:  nil,
				cr:  nil,
				crb: nil,
			},
		},
		{
			name: "CreateClusterRoleBindingError",
			r:    resource(withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						if _, ok := obj.(*rbac.ClusterRoleBinding); ok {
							return errors.New("test-create-crb-error")
						}
						return nil
					},
				}
			},
			want: want{
				err: fmt.Errorf("failed to create cluster role binding: %+v", errors.New("test-create-crb-error")),
				sa:  nil,
				cr:  nil,
				crb: nil,
			},
		},
		{
			name:       "Success",
			r:          resource(withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Extension) client.Client { return fake.NewFakeClient(r) },
			want: want{
				err: nil,
				sa: &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            resourceName,
						Namespace:       namespace,
						OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(resource()))}}},
				cr: &rbac.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(resource()))}},
					Rules:      defaultPolicyRules(),
				},
				crb: &rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName, OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(resource()))}},
					RoleRef:    rbac.RoleRef{APIGroup: rbac.GroupName, Kind: "ClusterRole", Name: resourceName},
					Subjects:   []rbac.Subject{{Name: resourceName, Namespace: namespace, Kind: rbac.ServiceAccountKind}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			handler := &extensionHandler{
				kube: tt.clientFunc(tt.r),
				ext:  tt.r,
			}

			err := handler.processRBAC(ctx)

			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("processRBAC() want error != got error:\n%s", diff)
			}

			if tt.want.sa != nil {
				got := &corev1.ServiceAccount{}
				assertKubernetesObject(t, g, got, tt.want.sa, handler.kube)
			}

			if tt.want.cr != nil {
				got := &rbac.ClusterRole{}
				assertKubernetesObject(t, g, got, tt.want.cr, handler.kube)
			}

			if tt.want.crb != nil {
				got := &rbac.ClusterRoleBinding{}
				assertKubernetesObject(t, g, got, tt.want.crb, handler.kube)
			}
		})
	}
}

// ************************************************************************************************
// TestProcessDeployment
// ************************************************************************************************
func TestProcessDeployment(t *testing.T) {
	type want struct {
		err           error
		d             *apps.Deployment
		controllerRef *corev1.ObjectReference
	}

	tests := []struct {
		name       string
		r          *v1alpha1.Extension
		clientFunc func(*v1alpha1.Extension) client.Client
		want       want
	}{
		{
			name:       "NoControllerRequested",
			r:          resource(),
			clientFunc: func(r *v1alpha1.Extension) client.Client { return fake.NewFakeClient(r) },
			want: want{
				err:           nil,
				d:             nil,
				controllerRef: nil,
			},
		},
		{
			name: "CreateDeploymentError",
			r:    resource(withControllerSpec(defaultControllerSpec())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return errors.New("test-create-error")
					},
				}
			},
			want: want{
				err:           fmt.Errorf("failed to create deployment: %+v", errors.New("test-create-error")),
				d:             nil,
				controllerRef: nil,
			},
		},
		{
			name:       "Success",
			r:          resource(withControllerSpec(defaultControllerSpec())),
			clientFunc: func(r *v1alpha1.Extension) client.Client { return fake.NewFakeClient(r) },
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:            controllerDeploymentName,
						Namespace:       namespace,
						OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(resource()))},
					},
					Spec: apps.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								ServiceAccountName: resourceName,
								Containers: []corev1.Container{
									{Name: controllerContainerName, Image: controllerImageName},
								},
							},
						},
					},
				},
				controllerRef: &corev1.ObjectReference{
					Name:      controllerDeploymentName,
					Namespace: namespace,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			handler := &extensionHandler{
				kube: tt.clientFunc(tt.r),
				ext:  tt.r,
			}

			err := handler.processDeployment(ctx)

			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("processDeployment want error != got error:\n%s", diff)
			}

			if tt.want.d != nil {
				got := &apps.Deployment{}
				assertKubernetesObject(t, g, got, tt.want.d, handler.kube)
			}

			if diff := cmp.Diff(handler.ext.Status.ControllerRef, tt.want.controllerRef); diff != "" {
				t.Errorf("got != want:\n%v", diff)
			}
		})
	}
}

func assertKubernetesObject(t *testing.T, g *GomegaWithT, got runtime.Object, want metav1.Object, kube client.Client) {
	n := types.NamespacedName{Name: want.GetName(), Namespace: want.GetNamespace()}
	g.Expect(kube.Get(ctx, n, got)).NotTo(HaveOccurred())
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("got != want:\n%v", diff)
	}
}

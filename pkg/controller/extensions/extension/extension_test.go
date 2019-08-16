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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
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

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/apis/extensions"
	"github.com/crossplaneio/crossplane/apis/extensions/v1alpha1"
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
	controllerJobName        = "cool-controller-job"
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

func withConditions(c ...corev1alpha1.Condition) resourceModifier {
	return func(r *v1alpha1.Extension) { r.Status.SetConditions(c...) }
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

func defaultJobControllerSpec() v1alpha1.ControllerSpec {
	return v1alpha1.ControllerSpec{
		Job: &v1alpha1.ControllerJob{
			Name: controllerJobName,
			Spec: batch.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
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

			if diff := cmp.Diff(tt.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile() -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, gotResult); diff != "" {
				t.Errorf("Reconcile() -want, +got:\n%s", diff)
			}
		})
	}
}

// ************************************************************************************************
// TestCreate
// ************************************************************************************************
func TestCreate(t *testing.T) {
	errBoom := errors.New("boom")

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
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*corev1.ServiceAccount); ok {
							return errBoom
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				}
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				r: resource(
					withPolicyRules(defaultPolicyRules()),
					withConditions(
						corev1alpha1.Creating(),
						corev1alpha1.ReconcileError(errors.Wrap(errBoom, "failed to create service account")),
					),
				),
			},
		},
		{
			name: "FailDeployment",
			r: resource(
				withPolicyRules(defaultPolicyRules()),
				withControllerSpec(defaultControllerSpec())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*apps.Deployment); ok {
							return errBoom
						}
						return nil
					},
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				}
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				r: resource(
					withPolicyRules(defaultPolicyRules()),
					withControllerSpec(defaultControllerSpec()),
					withConditions(
						corev1alpha1.Creating(),
						corev1alpha1.ReconcileError(errors.Wrap(errBoom, "failed to create deployment")),
					),
				),
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
					withConditions(corev1alpha1.Available(), corev1alpha1.ReconcileSuccess()),
				),
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

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("create(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.result, got); diff != "" {
				t.Errorf("create(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.r, tt.r, test.EquateConditions()); diff != "" {
				t.Errorf("create() resource: -want, +got:\n%s", diff)
			}
		})
	}
}

// ************************************************************************************************
// TestProcessRBAC
// ************************************************************************************************
func TestProcessRBAC(t *testing.T) {
	errBoom := errors.New("boom")

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
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*corev1.ServiceAccount); ok {
							return errBoom
						}
						return nil
					},
				}
			},
			want: want{
				err: errors.Wrap(errBoom, "failed to create service account"),
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
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*rbac.ClusterRole); ok {
							return errBoom
						}
						return nil
					},
				}
			},
			want: want{
				err: errors.Wrap(errBoom, "failed to create cluster role"),
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
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*rbac.ClusterRoleBinding); ok {
							return errBoom
						}
						return nil
					},
				}
			},
			want: want{
				err: errors.Wrap(errBoom, "failed to create cluster role binding"),
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
						Name:      resourceName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(resource(), v1alpha1.ExtensionGroupVersionKind)),
						},
					},
				},
				cr: &rbac.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(resource(), v1alpha1.ExtensionGroupVersionKind)),
						},
					},
					Rules: defaultPolicyRules(),
				},
				crb: &rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: resourceName,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(resource(), v1alpha1.ExtensionGroupVersionKind)),
						},
					},
					RoleRef:  rbac.RoleRef{APIGroup: rbac.GroupName, Kind: "ClusterRole", Name: resourceName},
					Subjects: []rbac.Subject{{Name: resourceName, Namespace: namespace, Kind: rbac.ServiceAccountKind}},
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

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("processRBAC(): -want error, +got error:\n%s", diff)
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
	errBoom := errors.New("boom")

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
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return errBoom
					},
				}
			},
			want: want{
				err:           errors.Wrap(errBoom, "failed to create deployment"),
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
						Name:      controllerDeploymentName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(resource(), v1alpha1.ExtensionGroupVersionKind)),
						},
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
					Name:       controllerDeploymentName,
					Namespace:  namespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
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

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("processDeployment -want error, +got error:\n%s", diff)
			}

			if tt.want.d != nil {
				got := &apps.Deployment{}
				assertKubernetesObject(t, g, got, tt.want.d, handler.kube)
			}

			if diff := cmp.Diff(tt.want.controllerRef, handler.ext.Status.ControllerRef); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

// ************************************************************************************************
// TestProcessJob
// ************************************************************************************************
func TestProcessJob(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		err           error
		j             *batch.Job
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
				j:             nil,
				controllerRef: nil,
			},
		},
		{
			name: "CreateJobError",
			r:    resource(withControllerSpec(defaultJobControllerSpec())),
			clientFunc: func(r *v1alpha1.Extension) client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return errBoom
					},
				}
			},
			want: want{
				err:           errors.Wrap(errBoom, "failed to create job"),
				j:             nil,
				controllerRef: nil,
			},
		},
		{
			name:       "Success",
			r:          resource(withControllerSpec(defaultJobControllerSpec())),
			clientFunc: func(r *v1alpha1.Extension) client.Client { return fake.NewFakeClient(r) },
			want: want{
				err: nil,
				j: &batch.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllerJobName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(resource(), v1alpha1.ExtensionGroupVersionKind)),
						},
					},
					Spec: batch.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy:      corev1.RestartPolicyNever,
								ServiceAccountName: resourceName,
								Containers: []corev1.Container{
									{Name: controllerContainerName, Image: controllerImageName},
								},
							},
						},
					},
				},
				controllerRef: &corev1.ObjectReference{
					Name:       controllerJobName,
					Namespace:  namespace,
					Kind:       "Job",
					APIVersion: "batch/v1",
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

			err := handler.processJob(ctx)

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("processJob -want error, +got error:\n%s", diff)
			}

			if tt.want.j != nil {
				got := &batch.Job{}
				assertKubernetesObject(t, g, got, tt.want.j, handler.kube)
			}

			if diff := cmp.Diff(tt.want.controllerRef, handler.ext.Status.ControllerRef); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

func assertKubernetesObject(t *testing.T, g *GomegaWithT, got runtime.Object, want metav1.Object, kube client.Client) {
	n := types.NamespacedName{Name: want.GetName(), Namespace: want.GetNamespace()}
	g.Expect(kube.Get(ctx, n, got)).NotTo(HaveOccurred())
	if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
		t.Errorf("-want, +got:\n%s", diff)
	}
}

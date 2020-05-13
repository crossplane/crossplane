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

package pkg

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
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

	"github.com/crossplane/crossplane/apis/packages"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/packages/hosted"
	packagespkg "github.com/crossplane/crossplane/pkg/packages"
)

const (
	namespace               = "cool-namespace"
	hostControllerNamespace = "controller-namespace"
	uid                     = types.UID("definitely-a-uuid")
	resourceName            = "cool-package"
	roleName                = "package:cool-namespace:cool-package:0.0.1:system"

	controllerDeploymentName = "cool-package-controller"
	controllerContainerName  = "cool-container"
	controllerImageName      = "cool/controller-image:rad"
	dashAlphabet             = "-abcdefghijklmnopqrstuvwxyz"
)

var (
	ctx     = context.Background()
	errBoom = errors.New("boom")
)

func init() {
	_ = packages.AddToScheme(scheme.Scheme)
	_ = apiextensionsv1beta1.AddToScheme(scheme.Scheme)
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

// ************************************************************************************************
// Resource modifiers
// ************************************************************************************************
type resourceModifier func(*v1alpha1.Package)

type deploymentSpecModifier func(*apps.DeploymentSpec)

func withFinalizers(finalizers ...string) resourceModifier {
	return func(r *v1alpha1.Package) { r.SetFinalizers(finalizers) }
}

func withResourceVersion(version string) resourceModifier {
	return func(r *v1alpha1.Package) { r.SetResourceVersion(version) }
}

func withCRDs(crds ...metav1.TypeMeta) resourceModifier {
	return func(r *v1alpha1.Package) { r.Spec.CRDs = crds }
}

func withDeletionTimestamp(t time.Time) resourceModifier {
	return func(r *v1alpha1.Package) {
		r.SetDeletionTimestamp(&metav1.Time{Time: t})
	}
}

func withGVK(gvk schema.GroupVersionKind) resourceModifier {
	return func(r *v1alpha1.Package) { r.SetGroupVersionKind(gvk) }
}

func withConditions(c ...runtimev1alpha1.Condition) resourceModifier {
	return func(r *v1alpha1.Package) { r.Status.SetConditions(c...) }
}

func withNamespacedName(nsn types.NamespacedName) resourceModifier {
	return func(r *v1alpha1.Package) {
		r.SetNamespace(nsn.Namespace)
		r.SetName(nsn.Name)
	}
}

func withControllerSpec(cs v1alpha1.ControllerSpec) resourceModifier {
	return func(r *v1alpha1.Package) { r.Spec.Controller = cs }
}

func withPolicyRules(policyRules []rbac.PolicyRule) resourceModifier {
	return func(r *v1alpha1.Package) { r.Spec.Permissions.Rules = policyRules }
}

func withPermissionScope(permissionScope string) resourceModifier {
	return func(r *v1alpha1.Package) { r.Spec.PermissionScope = permissionScope }
}

type saModifier func(*corev1.ServiceAccount)

func withTokenSecret(ref corev1.ObjectReference) saModifier {
	return func(sa *corev1.ServiceAccount) { sa.Secrets = append(sa.Secrets, ref) }
}

func withSANamespacedName(nsn types.NamespacedName) saModifier {
	return func(s *corev1.ServiceAccount) {
		s.SetNamespace(nsn.Namespace)
		s.SetName(nsn.Name)
	}
}

func withDeploymentPullSecrets(secrets ...string) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		for i := range secrets {
			ds.Template.Spec.ImagePullSecrets = append(ds.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secrets[i]})
		}
	}
}

func withDeploymentPullPolicy(policy corev1.PullPolicy) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		for i := range ds.Template.Spec.InitContainers {
			ds.Template.Spec.InitContainers[i].ImagePullPolicy = policy
		}
		for i := range ds.Template.Spec.Containers {
			ds.Template.Spec.Containers[i].ImagePullPolicy = policy
		}
	}
}

func sa(sm ...saModifier) *corev1.ServiceAccount {
	s := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
		},
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}

	for _, m := range sm {
		m(s)
	}
	return s
}

func saSecret(resourceName, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
		},
		Data: saTokenData(),
	}
}

func saTokenData() map[string][]byte {
	return map[string][]byte{
		"token": []byte("token-val"),
	}
}

func resource(rm ...resourceModifier) *v1alpha1.Package {
	r := &v1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       resourceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.PackageSpec{
			AppMetadataSpec: v1alpha1.AppMetadataSpec{
				Version: "0.0.1",
			},
		},
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
	MockNewHandler func(logging.Logger, *v1alpha1.Package, client.Client, client.Client, *hosted.Config, bool, bool, string) handler
}

func (f *mockFactory) newHandler(log logging.Logger, r *v1alpha1.Package, c client.Client, h client.Client, hc *hosted.Config, allowCore, allowFullDeployment bool, forceImagePullPolicy string) handler {
	return f.MockNewHandler(log, r, c, nil, nil, allowCore, allowFullDeployment, forceImagePullPolicy)
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

// ************************************************************************************************
// Default initializer functions
// ************************************************************************************************

func withDeploymentTmplMeta(name, namespace string, labels map[string]string) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		ds.Template.SetName(name)
		ds.Template.SetNamespace(namespace)
		meta.AddLabels(&ds.Template, labels)
	}
}

func withDeploymentMatchLabels(labels map[string]string) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		if ds.Selector.MatchLabels == nil {
			ds.Selector.MatchLabels = map[string]string{}
		}

		for k, v := range labels {
			ds.Selector.MatchLabels[k] = v
		}

		meta.AddLabels(&ds.Template, labels)
	}
}

func withDeploymentContainer(name, image string) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		ds.Template.Spec.Containers = append(ds.Template.Spec.Containers, corev1.Container{Name: name, Image: image})
	}
}

func withDeploymentSA(name string) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		ds.Template.Spec.ServiceAccountName = name
	}
}

func withDeploymentSecurityContext(sc *corev1.PodSecurityContext) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		ds.Template.Spec.SecurityContext = sc
	}
}

func withDeploymentContainerSecurityContext(sc *corev1.SecurityContext) deploymentSpecModifier {
	return func(ds *apps.DeploymentSpec) {
		for _, c := range [][]corev1.Container{
			ds.Template.Spec.Containers,
			ds.Template.Spec.InitContainers,
		} {
			for i := range c {
				c[i].SecurityContext = sc
			}
		}
	}
}

func deploymentSpec(dsm ...deploymentSpecModifier) *apps.DeploymentSpec {
	ds := &apps.DeploymentSpec{Selector: &metav1.LabelSelector{}}

	for _, m := range dsm {
		m(ds)
	}
	return ds
}

func defaultControllerSpec(dsm ...deploymentSpecModifier) v1alpha1.ControllerSpec {
	m := append(
		append([]deploymentSpecModifier{},
			withDeploymentContainer(controllerContainerName, controllerImageName),
			withDeploymentSA("some-sa-which-will-be-overridden-with-package-name"),
		), dsm...,
	)
	ds := deploymentSpec(m...)

	cs := v1alpha1.ControllerSpec{
		Deployment: &v1alpha1.ControllerDeployment{
			Name: controllerDeploymentName,
			Spec: *ds,
		},
	}

	return cs
}

func targetNamespace(name string) corev1.Namespace {
	return corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		UID:  uid,
	}}
}

func defaultPolicyRules() []rbac.PolicyRule {
	return []rbac.PolicyRule{{APIGroups: []string{""}, Resources: []string{"configmaps", "events", "secrets"}, Verbs: []string{"*"}}}
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
						*obj.(*v1alpha1.Package) = *(resource())
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				factory: &mockFactory{
					MockNewHandler: func(logging.Logger, *v1alpha1.Package, client.Client, client.Client, *hosted.Config, bool, bool, string) handler {
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
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
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
			req:  reconcile.Request{NamespacedName: types.NamespacedName{Name: resourceName, Namespace: namespace}},
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

// ************************************************************************************************
// TestCreate
// ************************************************************************************************
func TestCreate(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		result reconcile.Result
		err    error
		r      *v1alpha1.Package
	}

	tests := []struct {
		name       string
		r          *v1alpha1.Package
		clientFunc func(*v1alpha1.Package) client.Client
		allowCore  bool
		want       want
	}{
		{
			name: "FailRestrictedPermissions",
			r: resource(
				withPolicyRules([]rbac.PolicyRule{{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				}}),
				withFinalizers(packagesFinalizer)),
			clientFunc: func(r *v1alpha1.Package) client.Client {
				mc := test.NewMockClient()
				mc.MockStatusUpdate = func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil }
				return mc
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				r: resource(
					withFinalizers(packagesFinalizer),
					withPolicyRules([]rbac.PolicyRule{{
						APIGroups: []string{"*"},
						Resources: []string{"*"},
						Verbs:     []string{"*"},
					}}),
					withConditions(
						runtimev1alpha1.Creating(),
						runtimev1alpha1.ReconcileError(errors.Errorf("permissions contain a restricted rule")),
					),
				),
			},
		},
		{
			name: "FailRBAC",
			r: resource(
				withPolicyRules(defaultPolicyRules()),
				withFinalizers(packagesFinalizer)),
			clientFunc: func(r *v1alpha1.Package) client.Client {
				mc := test.NewMockClient()
				mc.MockCreate = func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
					if _, ok := obj.(*corev1.ServiceAccount); ok {
						return errBoom
					}
					return nil
				}
				mc.MockStatusUpdate = func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil }
				return mc
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				r: resource(
					withFinalizers(packagesFinalizer),
					withPolicyRules(defaultPolicyRules()),
					withConditions(
						runtimev1alpha1.Creating(),
						runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, "failed to create service account")),
					),
				),
			},
		},
		{
			name: "FailDeployment",
			r: resource(
				withPolicyRules(defaultPolicyRules()),
				withControllerSpec(defaultControllerSpec()),
				withFinalizers(packagesFinalizer)),
			clientFunc: func(r *v1alpha1.Package) client.Client {
				mc := test.NewMockClient()
				mc.MockCreate = func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
					if _, ok := obj.(*apps.Deployment); ok {
						return errBoom
					}
					return nil
				}
				mc.MockGet = func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch obj := obj.(type) {
					case *corev1.Namespace:
						*(obj) = targetNamespace(key.Name)
					case *apps.Deployment:
						return kerrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "Deployment"}, key.String())
					default:
						return errors.New("unexpected client GET call")
					}
					return nil
				}
				mc.MockStatusUpdate = func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil }
				return mc
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				r: resource(
					withFinalizers(packagesFinalizer),
					withPolicyRules(defaultPolicyRules()),
					withControllerSpec(defaultControllerSpec()),
					withConditions(
						runtimev1alpha1.Creating(),
						runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, "failed to create deployment")),
					),
				),
			},
		},
		{
			name: "SuccessfulCreate",
			r: resource(
				withGVK(v1alpha1.PackageGroupVersionKind),
				withFinalizers(packagesFinalizer),
				withResourceVersion("1")),
			clientFunc: func(r *v1alpha1.Package) client.Client { return fake.NewFakeClient(r) },
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				r: resource(
					withGVK(v1alpha1.PackageGroupVersionKind),
					withConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess()),
					withFinalizers(packagesFinalizer),
					withResourceVersion("2"),
				),
			},
		},
		{
			name: "SuccessfulClusterCreate",
			r: resource(
				withPermissionScope("Cluster"),
				withGVK(v1alpha1.PackageGroupVersionKind),
				withFinalizers(packagesFinalizer),
				withResourceVersion("1")),
			clientFunc: func(r *v1alpha1.Package) client.Client { return fake.NewFakeClient(r) },
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				r: resource(
					withGVK(v1alpha1.PackageGroupVersionKind),
					withPermissionScope("Cluster"),
					withConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess()),
					withFinalizers(packagesFinalizer),
					withResourceVersion("2"),
				),
			},
		},
		{
			name: "SuccessfulCreateAllowCore",
			r: resource(
				withGVK(v1alpha1.PackageGroupVersionKind),
				withFinalizers(packagesFinalizer),
				withResourceVersion("1"),
				withPolicyRules([]rbac.PolicyRule{{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				}})),
			allowCore:  true,
			clientFunc: func(r *v1alpha1.Package) client.Client { return fake.NewFakeClient(r) },
			want: want{
				result: requeueOnSuccess,
				err:    nil,
				r: resource(
					withGVK(v1alpha1.PackageGroupVersionKind),
					withConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess()),
					withFinalizers(packagesFinalizer),
					withResourceVersion("2"),
					withPolicyRules([]rbac.PolicyRule{{
						APIGroups: []string{"*"},
						Resources: []string{"*"},
						Verbs:     []string{"*"},
					}},
					)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &packageHandler{
				kube:      tt.clientFunc(tt.r),
				hostKube:  tt.clientFunc(tt.r),
				ext:       tt.r,
				log:       logging.NewNopLogger(),
				allowCore: tt.allowCore,
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

func TestProcessRBAC_Namespaced(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		err error
		sa  *corev1.ServiceAccount
		cr  []*rbac.ClusterRole
		crb *rbac.RoleBinding
	}

	tests := []struct {
		name       string
		r          *v1alpha1.Package
		clientFunc func(*v1alpha1.Package) client.Client
		want       want
	}{
		{
			name:       "NoPermissionsRequested",
			r:          resource(),
			clientFunc: func(r *v1alpha1.Package) client.Client { return fake.NewFakeClient(r) },
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
			clientFunc: func(r *v1alpha1.Package) client.Client {
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
			clientFunc: func(r *v1alpha1.Package) client.Client {
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
			name: "CreateRoleBindingError",
			r:    resource(withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Package) client.Client {
				mc := test.NewMockClient()
				mc.MockCreate = func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
					if _, ok := obj.(*rbac.RoleBinding); ok {
						return errBoom
					}
					return nil
				}
				mc.MockGet = func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch obj := obj.(type) {
					case *corev1.Namespace:
						*obj = targetNamespace(key.Name)
					default:
						return errors.New("unexpected client GET call")
					}
					return nil
				}
				return mc
			},
			want: want{
				err: errors.Wrap(errBoom, "failed to create role binding"),
				sa:  nil,
				cr:  nil,
				crb: nil,
			},
		},
		{
			name: "Success",
			r:    resource(withPermissionScope("Namespaced"), withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Package) client.Client {
				tns := targetNamespace(namespace)
				return fake.NewFakeClient(r, &tns)
			},
			want: want{
				err: nil,
				sa: &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(resource(), v1alpha1.PackageGroupVersionKind)),
						},
					},
				},
				cr: []*rbac.ClusterRole{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            roleName,
							OwnerReferences: nil,
							Labels:          packagespkg.ParentLabels(resource()),
						},
						Rules: defaultPolicyRules(),
					},
				},
				crb: &rbac.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(meta.ReferenceTo(resource(), v1alpha1.PackageGroupVersionKind)),
						},
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     roleName,
					},
					Subjects: []rbac.Subject{{Name: resourceName, Namespace: namespace, Kind: rbac.ServiceAccountKind}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			handler := &packageHandler{
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
				for _, wanted := range tt.want.cr {
					got := &rbac.ClusterRole{}
					assertKubernetesObject(t, g, got, wanted, handler.kube)
				}
			}

			if tt.want.crb != nil {
				got := &rbac.RoleBinding{}
				assertKubernetesObject(t, g, got, tt.want.crb, handler.kube)
			}
		})
	}
}

func TestProcessRBAC_Cluster(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		err error
		sa  *corev1.ServiceAccount
		cr  []*rbac.ClusterRole
		crb *rbac.ClusterRoleBinding
	}

	tests := []struct {
		name       string
		r          *v1alpha1.Package
		clientFunc func(*v1alpha1.Package) client.Client
		want       want
	}{
		{
			name:       "NoPermissionsRequested",
			r:          resource(withPermissionScope("Cluster")),
			clientFunc: func(r *v1alpha1.Package) client.Client { return fake.NewFakeClient(r) },
			want: want{
				err: nil,
				sa:  nil,
				cr:  nil,
				crb: nil,
			},
		},
		{
			name: "CreateServiceAccountError",
			r:    resource(withPermissionScope("Cluster"), withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Package) client.Client {
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
			name: "CreateRoleError",
			r:    resource(withPermissionScope("Cluster"), withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Package) client.Client {
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
			name: "CreateRoleBindingError",
			r:    resource(withPermissionScope("Cluster"), withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Package) client.Client {
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
			r:          resource(withPermissionScope("Cluster"), withPolicyRules(defaultPolicyRules())),
			clientFunc: func(r *v1alpha1.Package) client.Client { return fake.NewFakeClient(r) },
			want: want{
				err: nil,
				sa: &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							meta.AsOwner(
								meta.ReferenceTo(resource(
									withPermissionScope("Cluster")),
									v1alpha1.PackageGroupVersionKind,
								),
							),
						},
					},
				},
				cr: []*rbac.ClusterRole{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            roleName,
							OwnerReferences: nil,
							Labels:          packagespkg.ParentLabels(resource(withPermissionScope("Cluster"))),
						},
						Rules: defaultPolicyRules(),
					},
				},
				crb: &rbac.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            resourceName,
						OwnerReferences: nil,
						Labels:          packagespkg.ParentLabels(resource(withPermissionScope("Cluster"))),
					},
					RoleRef: rbac.RoleRef{
						APIGroup: rbac.GroupName,
						Kind:     "ClusterRole",
						Name:     roleName,
					},
					Subjects: []rbac.Subject{{Name: resourceName, Namespace: namespace, Kind: rbac.ServiceAccountKind}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			handler := &packageHandler{
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
				for _, wanted := range tt.want.cr {
					got := &rbac.ClusterRole{}
					assertKubernetesObject(t, g, got, wanted, handler.kube)
				}
			}

			if tt.want.crb != nil {
				got := &rbac.ClusterRoleBinding{}
				assertKubernetesObject(t, g, got, tt.want.crb, handler.kube)
			}
		})
	}
}

// ************************************************************************************************
// TestSyncSATokenSecret
// ************************************************************************************************
func TestSyncSATokenSecret(t *testing.T) {
	errBoom := errors.New("boom")

	type want struct {
		err error
		s   *corev1.Secret
	}

	tests := []struct {
		name           string
		initObjs       []runtime.Object
		clientFunc     func(initObjs ...runtime.Object) client.Client
		hostClientFunc func() client.Client
		hostawareCfg   *hosted.Config
		want           want
	}{
		{
			name:           "SANotFound",
			initObjs:       []runtime.Object{},
			clientFunc:     fake.NewFakeClient,
			hostClientFunc: func() client.Client { return fake.NewFakeClient() },
			want: want{
				err: errors.Wrap(errors.New(fmt.Sprintf("serviceaccounts \"%s\" not found", resourceName)),
					errServiceAccountNotFound),
			},
		},
		{
			name:     "FailedToGetSA",
			initObjs: []runtime.Object{},
			clientFunc: func(initObjs ...runtime.Object) client.Client {
				return &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						if _, ok := obj.(*corev1.ServiceAccount); ok {
							return errBoom
						}
						return nil
					},
				}
			},
			want: want{
				err: errors.Wrap(errBoom, errFailedToGetServiceAccount),
			},
		},
		{
			name:           "TokenSecretNotGenerated",
			initObjs:       []runtime.Object{sa(), saSecret(resourceName, namespace)},
			clientFunc:     fake.NewFakeClient,
			hostClientFunc: func() client.Client { return fake.NewFakeClient() },
			want: want{
				err: errors.New(errServiceAccountTokenSecretNotGeneratedYet),
			},
		},
		{
			name:           "TokenSecretNotFound",
			initObjs:       []runtime.Object{sa(withTokenSecret(corev1.ObjectReference{Name: resourceName, Namespace: namespace}))},
			clientFunc:     fake.NewFakeClient,
			hostClientFunc: func() client.Client { return fake.NewFakeClient() },
			want: want{
				err: errors.Wrap(errors.New(fmt.Sprintf("secrets \"%s\" not found", resourceName)),
					errFailedToGetServiceAccountTokenSecret),
			},
		},
		{
			name:       "FailedToCreateSecret",
			initObjs:   []runtime.Object{sa(withTokenSecret(corev1.ObjectReference{Name: resourceName, Namespace: namespace})), saSecret(resourceName, namespace)},
			clientFunc: fake.NewFakeClient,
			hostClientFunc: func() client.Client {
				return &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*corev1.Secret); ok {
							return errBoom
						}
						return nil
					},
				}
			},
			hostawareCfg: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
			want: want{
				err: errors.Wrap(errBoom, errFailedToCreateTokenSecret),
			},
		},
		{
			name:       "Success",
			initObjs:   []runtime.Object{sa(withTokenSecret(corev1.ObjectReference{Name: resourceName, Namespace: namespace})), saSecret(resourceName, namespace)},
			clientFunc: fake.NewFakeClient,
			hostClientFunc: func() client.Client {
				return fake.NewFakeClient()
			},
			hostawareCfg: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
			want: want{
				err: nil,
				s: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s.%s", namespace, resourceName),
						Namespace: hostControllerNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       controllerDeploymentName,
								APIVersion: "apps/v1",
								Kind:       "Deployment",
							},
						},
					},
					Data: saTokenData(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			initObjs := tt.initObjs
			handler := &packageHandler{
				kube: tt.clientFunc(initObjs...),
			}
			if tt.hostawareCfg == nil {
				handler.hostKube = handler.kube
			} else {
				handler.hostKube = tt.hostClientFunc()
			}

			owner := metav1.OwnerReference{
				Name:       controllerDeploymentName,
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			}
			fromSARef := corev1.ObjectReference{
				Name:      resourceName,
				Namespace: namespace,
			}
			toSecretRef := corev1.ObjectReference{
				Name:      fmt.Sprintf("%s.%s", namespace, resourceName),
				Namespace: hostControllerNamespace,
			}
			err := handler.syncSATokenSecret(ctx, owner, fromSARef, toSecretRef)

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("syncSATokenSecret -want error, +got error:\n%s", diff)
			}

			if tt.want.s != nil {
				got := &corev1.Secret{}
				assertKubernetesObject(t, g, got, tt.want.s, handler.hostKube)
			}
		})
	}
}

// ************************************************************************************************
// TestProcessDeployment
// ************************************************************************************************
func TestProcessDeployment(t *testing.T) {
	trueVal := true
	errBoom := errors.New("boom")
	testDep := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
			UID:       uid,
		},
	}

	type want struct {
		err           error
		d             *apps.Deployment
		controllerRef *corev1.ObjectReference
	}

	tests := []struct {
		name                 string
		r                    *v1alpha1.Package
		initObjs             []runtime.Object
		clientFunc           func(initObjs ...runtime.Object) client.Client
		hostClientFunc       func() client.Client
		hostawareCfg         *hosted.Config
		forceImagePullPolicy string
		passFullDeployment   bool
		want                 want
	}{
		{
			name:       "NoControllerRequested",
			r:          resource(),
			clientFunc: fake.NewFakeClient,
			want: want{
				err:           nil,
				d:             nil,
				controllerRef: nil,
			},
		},
		{
			name: "GetDeploymentSuccess",
			r:    resource(withControllerSpec(defaultControllerSpec())),
			clientFunc: func(initObjs ...runtime.Object) client.Client {
				return &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch o := obj.(type) {
						case *apps.Deployment:
							testDep.DeepCopyInto(o)
							return nil
						default:
							return errors.New("unexpected client GET call")
						}
					},
				}
			},
			want: want{
				controllerRef: meta.ReferenceTo(testDep, apps.SchemeGroupVersion.WithKind("Deployment")),
			},
		},
		{
			name: "CreateDeploymentError",
			r:    resource(withControllerSpec(defaultControllerSpec())),
			clientFunc: func(initObjs ...runtime.Object) client.Client {
				return &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch obj.(type) {
						case *apps.Deployment:
							return kerrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "Deployment"}, key.String())
						default:
							return errors.New("unexpected client GET call")
						}
					},
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
			name:       "CreateDeploymentErrorHosted",
			r:          resource(withControllerSpec(defaultControllerSpec())),
			clientFunc: fake.NewFakeClient,
			hostClientFunc: func() client.Client {
				return &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch obj.(type) {
						case *apps.Deployment:
							return kerrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "Deployment"}, key.String())
						default:
							return errors.New("unexpected client GET call")
						}
					},
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						return errBoom
					},
				}
			},
			hostawareCfg: &hosted.Config{
				HostControllerNamespace: hostControllerNamespace,
			},
			want: want{
				err:           errors.Wrap(errBoom, "failed to create deployment"),
				d:             nil,
				controllerRef: nil,
			},
		},
		{
			name:       "HostedError_SyncSATokenFailedToCreateSecret",
			r:          resource(withControllerSpec(defaultControllerSpec())),
			initObjs:   []runtime.Object{sa(withTokenSecret(corev1.ObjectReference{Name: resourceName, Namespace: namespace})), saSecret(resourceName, namespace)},
			clientFunc: fake.NewFakeClient,
			hostClientFunc: func() client.Client {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet:  test.NewMockGetFn(nil),
					MockCreate: func(ctx context.Context, obj runtime.Object, _ ...client.CreateOption) error {
						if _, ok := obj.(*corev1.Secret); ok {
							return errBoom
						}
						return nil
					},
				}
			},
			hostawareCfg: &hosted.Config{
				HostControllerNamespace: hostControllerNamespace,
			},
			want: want{
				err: errors.Wrap(
					errors.Wrap(errBoom, errFailedToCreateTokenSecret),
					errFailedToSyncSASecret),
				d:             nil,
				controllerRef: nil,
			},
		},
		{
			name:       "Success",
			r:          resource(withControllerSpec(defaultControllerSpec())),
			clientFunc: fake.NewFakeClient,
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllerDeploymentName,
						Namespace: namespace,
						Labels:    packagespkg.ParentLabels(resource(withControllerSpec(defaultControllerSpec()))),
					},
					Spec: *deploymentSpec(
						withDeploymentTmplMeta(controllerDeploymentName, "", nil),
						withDeploymentMatchLabels(map[string]string{"app": controllerDeploymentName}),
						withDeploymentSA(resourceName),
						withDeploymentContainer(controllerContainerName, controllerImageName),
						withDeploymentSecurityContext(&corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot}),
						withDeploymentContainerSecurityContext(&corev1.SecurityContext{
							AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							Privileged:               &privileged,
							RunAsNonRoot:             &runAsNonRoot,
						}),
					),
				},
				controllerRef: &corev1.ObjectReference{
					Name:       controllerDeploymentName,
					Namespace:  namespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
		{
			name:       "SuccessPullPolicy",
			r:          resource(withControllerSpec(defaultControllerSpec(withDeploymentPullPolicy(corev1.PullAlways)))),
			clientFunc: fake.NewFakeClient,
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllerDeploymentName,
						Namespace: namespace,
						Labels:    packagespkg.ParentLabels(resource(withControllerSpec(defaultControllerSpec()))),
					},
					Spec: *deploymentSpec(
						withDeploymentTmplMeta(controllerDeploymentName, "", nil),
						withDeploymentMatchLabels(map[string]string{"app": controllerDeploymentName}),
						withDeploymentSA(resourceName),
						withDeploymentContainer(controllerContainerName, controllerImageName),
						withDeploymentPullPolicy(corev1.PullAlways),
						withDeploymentSecurityContext(&corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot}),
						withDeploymentContainerSecurityContext(&corev1.SecurityContext{
							AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							Privileged:               &privileged,
							RunAsNonRoot:             &runAsNonRoot,
						}),
					),
				},
				controllerRef: &corev1.ObjectReference{
					Name:       controllerDeploymentName,
					Namespace:  namespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
		{
			name: "SuccessForcedPullPolicy",
			r: resource(
				withControllerSpec(
					defaultControllerSpec(
						withDeploymentPullPolicy(corev1.PullNever),
					),
				),
			),
			forceImagePullPolicy: string(corev1.PullAlways),
			clientFunc:           fake.NewFakeClient,
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllerDeploymentName,
						Namespace: namespace,
						Labels:    packagespkg.ParentLabels(resource(withControllerSpec(defaultControllerSpec()))),
					},
					Spec: *deploymentSpec(
						withDeploymentTmplMeta(controllerDeploymentName, "", nil),
						withDeploymentMatchLabels(map[string]string{"app": controllerDeploymentName}),
						withDeploymentSA(resourceName),
						withDeploymentContainer(controllerContainerName, controllerImageName),
						withDeploymentPullPolicy(corev1.PullAlways),
						withDeploymentSecurityContext(&corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot}),
						withDeploymentContainerSecurityContext(&corev1.SecurityContext{
							AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							Privileged:               &privileged,
							RunAsNonRoot:             &runAsNonRoot,
						}),
					),
				},
				controllerRef: &corev1.ObjectReference{
					Name:       controllerDeploymentName,
					Namespace:  namespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
		{
			name:       "SuccessPullSecrets",
			r:          resource(withControllerSpec(defaultControllerSpec(withDeploymentPullSecrets("foo")))),
			clientFunc: fake.NewFakeClient,
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllerDeploymentName,
						Namespace: namespace,
						Labels:    packagespkg.ParentLabels(resource(withControllerSpec(defaultControllerSpec()))),
					},
					Spec: *deploymentSpec(
						withDeploymentTmplMeta(controllerDeploymentName, "", nil),
						withDeploymentMatchLabels(map[string]string{"app": controllerDeploymentName}),
						withDeploymentSA(resourceName),
						withDeploymentContainer(controllerContainerName, controllerImageName),
						withDeploymentPullSecrets("foo"),
						withDeploymentSecurityContext(&corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot}),
						withDeploymentContainerSecurityContext(&corev1.SecurityContext{
							AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							Privileged:               &privileged,
							RunAsNonRoot:             &runAsNonRoot,
						}),
					),
				},
				controllerRef: &corev1.ObjectReference{
					Name:       controllerDeploymentName,
					Namespace:  namespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
		{
			name: "SuccessPassFullDeployment",
			r: resource(withControllerSpec(defaultControllerSpec(withDeploymentContainerSecurityContext(&corev1.SecurityContext{
				AllowPrivilegeEscalation: &trueVal,
				Privileged:               &trueVal,
			})))),
			clientFunc:         fake.NewFakeClient,
			passFullDeployment: true,
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      controllerDeploymentName,
						Namespace: namespace,
						Labels:    packagespkg.ParentLabels(resource(withControllerSpec(defaultControllerSpec()))),
					},
					Spec: *deploymentSpec(
						withDeploymentTmplMeta(controllerDeploymentName, "", nil),
						withDeploymentMatchLabels(map[string]string{"app": controllerDeploymentName}),
						withDeploymentSA(resourceName),
						withDeploymentContainer(controllerContainerName, controllerImageName),
						withDeploymentContainerSecurityContext(&corev1.SecurityContext{
							AllowPrivilegeEscalation: &trueVal,
							Privileged:               &trueVal,
						}),
					),
				},
				controllerRef: &corev1.ObjectReference{
					Name:       controllerDeploymentName,
					Namespace:  namespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
		{
			name:           "SuccessHosted",
			r:              resource(withControllerSpec(defaultControllerSpec())),
			initObjs:       []runtime.Object{sa(withTokenSecret(corev1.ObjectReference{Name: resourceName, Namespace: namespace})), saSecret(resourceName, namespace)},
			clientFunc:     fake.NewFakeClient,
			hostClientFunc: func() client.Client { return fake.NewFakeClient() },
			hostawareCfg: &hosted.Config{
				HostControllerNamespace: hostControllerNamespace,
			},
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:        fmt.Sprintf("%s.%s", namespace, controllerDeploymentName),
						Namespace:   hostControllerNamespace,
						Labels:      packagespkg.ParentLabels(resource(withControllerSpec(defaultControllerSpec()))),
						Annotations: hosted.ObjectReferenceAnnotationsOnHost("package", resourceName, namespace),
					},
					Spec: apps.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": controllerDeploymentName,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": controllerDeploymentName,
								},
								Name: controllerDeploymentName,
							},
							Spec: corev1.PodSpec{
								ServiceAccountName:           "",
								SecurityContext:              &corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot},
								AutomountServiceAccountToken: &disableAutoMount,
								Containers: []corev1.Container{
									{
										Name:  controllerContainerName,
										Image: controllerImageName,
										Env: []corev1.EnvVar{
											{
												Name:  envK8SServiceHost,
												Value: "",
											},
											{
												Name:  envK8SServicePort,
												Value: "",
											},
											{
												Name:  envPodNamespace,
												Value: namespace,
											},
										},
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: &allowPrivilegeEscalation,
											Privileged:               &privileged,
											RunAsNonRoot:             &runAsNonRoot,
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      saVolumeName,
												ReadOnly:  true,
												MountPath: saMountPath,
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: saVolumeName,
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: fmt.Sprintf("%s.%s", namespace, resourceName),
											},
										},
									},
								},
							},
						},
					},
				},
				controllerRef: &corev1.ObjectReference{
					Name:       fmt.Sprintf("%s.%s", namespace, controllerDeploymentName),
					Namespace:  hostControllerNamespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
		{
			name: "SuccessHostedTruncated",
			r: resource(
				withNamespacedName(types.NamespacedName{
					Name:      resourceName + dashAlphabet,
					Namespace: namespace + dashAlphabet,
				}),
				withControllerSpec(defaultControllerSpec())),
			initObjs: []runtime.Object{sa(
				withSANamespacedName(types.NamespacedName{
					Name:      resourceName + dashAlphabet,
					Namespace: namespace + dashAlphabet,
				}),
				withTokenSecret(corev1.ObjectReference{
					Name:      resourceName + dashAlphabet,
					Namespace: namespace + dashAlphabet,
				})), saSecret(resourceName+dashAlphabet, namespace+dashAlphabet)},
			clientFunc:     fake.NewFakeClient,
			hostClientFunc: func() client.Client { return fake.NewFakeClient() },
			hostawareCfg: &hosted.Config{
				HostControllerNamespace: hostControllerNamespace,
			},
			want: want{
				err: nil,
				d: &apps.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cool-namespace-abcdefghijklmnopqrstuvwxyz.cool-package-ab-fup65",
						Namespace: hostControllerNamespace,
						Labels: packagespkg.ParentLabels(resource(
							withNamespacedName(types.NamespacedName{
								Name:      resourceName + dashAlphabet,
								Namespace: namespace + dashAlphabet,
							}),
							withControllerSpec(defaultControllerSpec()))),
						Annotations: hosted.ObjectReferenceAnnotationsOnHost("package", resourceName+dashAlphabet, namespace+dashAlphabet),
					},
					Spec: apps.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": resourceName + dashAlphabet + "-controller",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": resourceName + dashAlphabet + "-controller",
								},
								Name: resourceName + dashAlphabet + "-controller",
							},
							Spec: corev1.PodSpec{
								ServiceAccountName:           "",
								AutomountServiceAccountToken: &disableAutoMount,
								SecurityContext:              &corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot},
								Containers: []corev1.Container{
									{
										Name:  controllerContainerName,
										Image: controllerImageName,
										Env: []corev1.EnvVar{
											{
												Name:  envK8SServiceHost,
												Value: "",
											},
											{
												Name:  envK8SServicePort,
												Value: "",
											},
											{
												Name:  envPodNamespace,
												Value: namespace + dashAlphabet,
											},
										},
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: &allowPrivilegeEscalation,
											Privileged:               &privileged,
											RunAsNonRoot:             &runAsNonRoot,
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      saVolumeName,
												ReadOnly:  true,
												MountPath: saMountPath,
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: saVolumeName,
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "cool-namespace-abcdefghijklmnopqrstuvwxyz.cool-package-ab-vq7wj",
											},
										},
									},
								},
							},
						},
					},
				},
				controllerRef: &corev1.ObjectReference{
					Name:       "cool-namespace-abcdefghijklmnopqrstuvwxyz.cool-package-ab-fup65",
					Namespace:  hostControllerNamespace,
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			initObjs := append(tt.initObjs, tt.r)
			handler := &packageHandler{
				kube:                 tt.clientFunc(initObjs...),
				hostAwareConfig:      tt.hostawareCfg,
				ext:                  tt.r,
				forceImagePullPolicy: tt.forceImagePullPolicy,
				allowFullDeployment:  tt.passFullDeployment,
			}
			if tt.hostawareCfg == nil {
				handler.hostKube = handler.kube
			} else {
				handler.hostKube = tt.hostClientFunc()
			}

			err := handler.processDeployment(ctx)

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("processDeployment -want error, +got error:\n%s", diff)
			}

			if tt.want.d != nil {
				got := &apps.Deployment{}
				assertKubernetesObject(t, g, got, tt.want.d, handler.hostKube)
			}

			if diff := cmp.Diff(tt.want.controllerRef, handler.ext.Status.ControllerRef); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}

type objectWithGVK interface {
	runtime.Object
	metav1.Object
}

// TestPackageDelete tests the delete function of the package handler
func TestPackageDelete(t *testing.T) {
	tn := time.Now()

	type want struct {
		result reconcile.Result
		err    error
		si     *v1alpha1.Package
	}

	tests := []struct {
		name    string
		handler *packageHandler
		want    want
	}{
		{
			name: "FailDeleteAllOf",
			handler: &packageHandler{
				// package starts with a finalizer and a deletion timestamp
				ext: resource(withFinalizers(packagesFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return errBoom },
				},
				log: logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: resource(
					withFinalizers(packagesFinalizer),
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "FailDeleteAllOfDeploymentsHosted",
			handler: &packageHandler{
				// package starts with a finalizer and a deletion timestamp
				ext: resource(withFinalizers(packagesFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
						if _, ok := obj.(*apps.Deployment); ok {
							return errBoom
						}
						return nil
					},
				},
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: resource(
					withFinalizers(packagesFinalizer),
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "FailDeleteAllOfJobsHosted",
			handler: &packageHandler{
				// package starts with a finalizer and a deletion timestamp
				ext: resource(withFinalizers(packagesFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockStatusUpdate: func(ctx context.Context, obj runtime.Object, _ ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
						if _, ok := obj.(*batch.Job); ok {
							return errBoom
						}
						return nil
					},
				},
				hostAwareConfig: &hosted.Config{HostControllerNamespace: hostControllerNamespace},
				log:             logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: resource(
					withFinalizers(packagesFinalizer),
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "FailUpdate",
			handler: &packageHandler{
				// package install starts with a finalizer and a deletion timestamp
				ext: resource(withFinalizers(packagesFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						// set a fake list of cluster resources to delete
						switch list := list.(type) {
						case *rbac.ClusterRoleBindingList:
							list.Items = []rbac.ClusterRoleBinding{{
								ObjectMeta: metav1.ObjectMeta{Name: "crdToDelete"},
							}}
						case *rbac.ClusterRoleList:
							list.Items = []rbac.ClusterRole{{
								ObjectMeta: metav1.ObjectMeta{Name: "crdToDelete"},
							}}
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
				log: logging.NewNopLogger(),
			},
			want: want{
				result: resultRequeue,
				err:    nil,
				si: resource(
					// the finalizer will have been removed from our test object at least in memory
					// (even though the update call to the API server failed)
					withDeletionTimestamp(tn),
					withConditions(runtimev1alpha1.ReconcileError(errBoom))),
			},
		},
		{
			name: "SuccessfulDelete",
			handler: &packageHandler{
				// package install starts with a finalizer and a deletion timestamp
				ext: resource(withFinalizers(packagesFinalizer), withDeletionTimestamp(tn)),
				kube: &test.MockClient{
					MockList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
						// set a fake list of cluster resources to delete
						switch list := list.(type) {
						case *rbac.ClusterRoleBindingList:
							list.Items = []rbac.ClusterRoleBinding{{
								ObjectMeta: metav1.ObjectMeta{Name: "crdToDelete"},
							}}
						case *rbac.ClusterRoleList:
							list.Items = []rbac.ClusterRole{{
								ObjectMeta: metav1.ObjectMeta{Name: "crdToDelete"},
							}}
						}
						return nil
					},
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
					MockUpdate:      func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error { return nil },
				},
				hostKube: &test.MockClient{
					MockDeleteAllOf: func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error { return nil },
				},
				log: logging.NewNopLogger(),
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
				si:     resource(withDeletionTimestamp(tn)), // finalizers get removed by delete function
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
				t.Errorf("delete() -want packageInstall, +got packageInstall:\n%v", diff)
			}
		})
	}
}

func Test_packageHandler_prepareHostAwareDeployment(t *testing.T) {
	type fields struct {
		kube            client.Client
		hostKube        client.Client
		hostAwareConfig *hosted.Config
		ext             *v1alpha1.Package
	}
	type args struct {
		tokenSecret string
		d           *apps.Deployment
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		fields
		args
		want
	}{
		"hostAwareNotEnabled": {
			fields: fields{},
			args: args{
				tokenSecret: resourceName,
				d:           nil,
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := &packageHandler{
				kube:            tc.fields.kube,
				hostKube:        tc.fields.hostKube,
				hostAwareConfig: tc.fields.hostAwareConfig,
				ext:             tc.fields.ext,
			}
			gotErr := h.prepareHostAwareDeployment(tc.args.d, tc.args.tokenSecret)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("prepareHostAwareDeployment(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_packageHandler_prepareHostAwarePodSpec(t *testing.T) {
	type fields struct {
		kube            client.Client
		hostKube        client.Client
		hostAwareConfig *hosted.Config
		ext             *v1alpha1.Package
	}
	type args struct {
		tokenSecret string
		ps          *corev1.PodSpec
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		fields
		args
		want
	}{
		"hostAwareNotEnabled": {
			fields: fields{},
			args: args{
				tokenSecret: resourceName,
				ps:          nil,
			},
			want: want{
				err: errors.New(errHostAwareModeNotEnabled),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := &packageHandler{
				kube:            tc.fields.kube,
				hostKube:        tc.fields.hostKube,
				hostAwareConfig: tc.fields.hostAwareConfig,
				ext:             tc.fields.ext,
			}
			gotErr := h.prepareHostAwarePodSpec(tc.args.tokenSecret, tc.args.ps)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("prepareHostAwarePodSpec(...): -want error, +got error: %s", diff)
			}
		})
	}
}

type crdModifier func(*apiextensionsv1beta1.CustomResourceDefinition)

func withCRDVersion(version string) crdModifier {
	return func(c *apiextensionsv1beta1.CustomResourceDefinition) {
		c.Spec.Version = version
		c.Spec.Versions = append(c.Spec.Versions, apiextensionsv1beta1.CustomResourceDefinitionVersion{Name: version})
	}
}

func withCRDScope(scope apiextensionsv1beta1.ResourceScope) crdModifier {
	return func(c *apiextensionsv1beta1.CustomResourceDefinition) {
		c.Spec.Scope = scope
	}
}

func withCRDLabels(labels map[string]string) crdModifier {
	return func(c *apiextensionsv1beta1.CustomResourceDefinition) {
		meta.AddLabels(c, labels)
	}
}

func withCRDSubresources() crdModifier {
	return func(c *apiextensionsv1beta1.CustomResourceDefinition) {
		c.Spec.Subresources = &apiextensionsv1beta1.CustomResourceSubresources{
			Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
			Scale:  &apiextensionsv1beta1.CustomResourceSubresourceScale{},
		}
	}
}

func withCRDGroupKind(group, kind string) crdModifier {
	singular := strings.ToLower(kind)
	plural := singular + "s"
	list := kind + "List"

	return func(c *apiextensionsv1beta1.CustomResourceDefinition) {
		c.Spec.Group = group
		c.Spec.Names.Kind = kind
		c.Spec.Names.Plural = plural
		c.Spec.Names.ListKind = list
		c.Spec.Names.Singular = singular
		c.SetName(plural + "." + group)
	}
}

func crd(cm ...crdModifier) apiextensionsv1beta1.CustomResourceDefinition {
	// basic crd with defaults
	t := true
	c := apiextensionsv1beta1.CustomResourceDefinition{
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Scope: "Namespaced",
			Conversion: &apiextensionsv1beta1.CustomResourceConversion{
				Strategy:                 apiextensionsv1beta1.NoneConverter,
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

type crModifier func(*rbac.ClusterRole)

func withClusterRoleLabels(labels map[string]string) crModifier {
	return func(cr *rbac.ClusterRole) {
		meta.AddLabels(cr, labels)
	}
}

func withClusterRoleRules(rules []rbac.PolicyRule) crModifier {
	return func(cr *rbac.ClusterRole) {
		cr.Rules = append(cr.Rules, rules...)

	}
}

func clusterRole(name string, crm ...crModifier) rbac.ClusterRole {
	c := rbac.ClusterRole{}
	c.SetName(name)
	for _, m := range crm {
		m(&c)
	}
	return c
}

func Test_crdListFulfilled(t *testing.T) {
	type args struct {
		want v1alpha1.CRDList
		got  []apiextensionsv1beta1.CustomResourceDefinition
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "MissingFromCRDList",
			args: args{v1alpha1.CRDList{
				metav1.TypeMeta{Kind: kind, APIVersion: apiVersion},
			}, []apiextensionsv1beta1.CustomResourceDefinition{}},
			wantErr: fmt.Errorf(`Missing CRD with APIVersion %q and Kind %q`, apiVersion, kind),
		},
		{
			name: "WrongCRDVersion",
			args: args{v1alpha1.CRDList{
				metav1.TypeMeta{Kind: kind, APIVersion: apiVersion},
			}, []apiextensionsv1beta1.CustomResourceDefinition{
				crd(
					withCRDGroupKind(group, kind),
					withCRDVersion("foo"),
				)}},
			wantErr: fmt.Errorf(`Missing CRD with APIVersion %q and Kind %q`, apiVersion, kind),
		},
		{
			name: "DifferentCRDKind",
			args: args{v1alpha1.CRDList{
				metav1.TypeMeta{Kind: kind, APIVersion: apiVersion},
			}, []apiextensionsv1beta1.CustomResourceDefinition{
				crd(
					withCRDGroupKind(group, "foo"),
					withCRDVersion(version),
				)}},
			wantErr: fmt.Errorf(`Missing CRD with APIVersion %q and Kind %q`, apiVersion, kind),
		},
		{
			name: "PartialMatch",
			args: args{v1alpha1.CRDList{
				metav1.TypeMeta{Kind: kind, APIVersion: apiVersion},
				metav1.TypeMeta{Kind: kind + "z", APIVersion: apiVersion},
			}, []apiextensionsv1beta1.CustomResourceDefinition{crd(
				withCRDGroupKind(group, kind),
				withCRDVersion(version),
			)}},
			wantErr: fmt.Errorf(`Missing CRD with APIVersion %q and Kind %q`, apiVersion, kind+"z"),
		},
		{
			name: "Success",
			args: args{v1alpha1.CRDList{
				metav1.TypeMeta{Kind: kind, APIVersion: apiVersion},
			}, []apiextensionsv1beta1.CustomResourceDefinition{crd(
				withCRDGroupKind(group, kind),
				withCRDVersion(version),
			)}},
			wantErr: nil,
		},
		{
			name: "SuccessMultiple",
			args: args{v1alpha1.CRDList{
				metav1.TypeMeta{Kind: kind, APIVersion: apiVersion},
				metav1.TypeMeta{Kind: kind + "z", APIVersion: apiVersion},
			}, []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
				),
				crd(withCRDGroupKind(group, kind+"z"),
					withCRDVersion(version),
				)}},
			wantErr: nil,
		},
		{
			name: "SuccessWithExtra",
			args: args{v1alpha1.CRDList{metav1.TypeMeta{Kind: kind, APIVersion: apiVersion}}, []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
				),
				crd(withCRDGroupKind(group, kind+"z"),
					withCRDVersion(version),
				)}},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := crdListFulfilled(tt.args.want, tt.args.got)

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("crdListFulfilled(...): -want error, +got error: %s", diff)
			}
		})
	}
}

func Test_packageHandler_crdsFromPackage(t *testing.T) {
	type fields struct {
		kube            client.Client
		hostKube        client.Client
		hostAwareConfig *hosted.Config
		ext             *v1alpha1.Package
		log             logging.Logger
	}
	type args struct {
		ctx context.Context
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []apiextensionsv1beta1.CustomResourceDefinition
		wantErr error
	}{
		{
			name: "MissingFromCRDList",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
			},
			args:    args{context.TODO()},
			want:    []apiextensionsv1beta1.CustomResourceDefinition{},
			wantErr: nil,
		},
		{
			name: "ErrorListingCRDs",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				kube: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
			},
			args:    args{context.TODO()},
			want:    nil,
			wantErr: errors.Wrap(errBoom, "CRDs could not be listed"),
		},
		{
			name: "MissingCRDVersion",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				kube: &test.MockClient{
					MockList: func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
						switch list := list.(type) {
						case *apiextensionsv1beta1.CustomResourceDefinitionList:
							list.Items = append(list.Items,
								crd(
									withCRDGroupKind(group, kind),
									withCRDVersion("foo"),
								),
							)
						default:
							return errors.New("unexpected list for testing")
						}
						return nil
					},
				},
			},
			want:    []apiextensionsv1beta1.CustomResourceDefinition{},
			wantErr: nil,
		},
		{
			name: "MissingCRDKind",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				kube: &test.MockClient{
					MockList: func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
						switch list := list.(type) {
						case *apiextensionsv1beta1.CustomResourceDefinitionList:
							list.Items = append(list.Items,
								crd(
									withCRDGroupKind(group, "Differentkind"),
									withCRDVersion(version),
								),
							)
						default:
							return errors.New("unexpected list for testing")
						}
						return nil
					},
				},
			},
			want:    []apiextensionsv1beta1.CustomResourceDefinition{},
			wantErr: nil,
		},
		{
			name: "Success",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				kube: &test.MockClient{
					MockList: func(_ context.Context, list runtime.Object, _ ...client.ListOption) error {
						switch list := list.(type) {
						case *apiextensionsv1beta1.CustomResourceDefinitionList:
							list.Items = append(list.Items,
								crd(
									withCRDGroupKind(group, kind), withCRDVersion(version),
								),
							)
						default:
							return errors.New("unexpected list for testing")
						}
						return nil
					},
				},
			},
			args: args{context.TODO()},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind), withCRDVersion(version)),
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &packageHandler{
				kube:            tt.fields.kube,
				hostKube:        tt.fields.hostKube,
				hostAwareConfig: tt.fields.hostAwareConfig,
				ext:             tt.fields.ext,
				log:             tt.fields.log,
			}
			got, gotErr := h.crdsFromPackage(tt.args.ctx)

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("packageHandler.crdsFromPackage(...): -want error, +got error: %s", diff)
			}

			if diff := cmp.Diff(tt.want, got, test.EquateErrors()); diff != "" {
				t.Fatalf("packageHandler.crdsFromPackage(...): -want, +got: %s", diff)
			}
		})
	}
}

func Test_packageHandler_createNamespaceLabelsCRDHandler(t *testing.T) {
	type fields struct {
		clientFunc func() client.Client
		ext        *v1alpha1.Package
	}
	type args struct {
		ctx  context.Context
		crds []apiextensionsv1beta1.CustomResourceDefinition
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	var (
		nsLabel = fmt.Sprintf(packagespkg.LabelNamespaceFmt, namespace)
	)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []apiextensionsv1beta1.CustomResourceDefinition
		wantErr error
	}{
		{
			name: "MissingCRD",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					return fake.NewFakeClient()
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}),
					)},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{},
			wantErr: kerrors.NewNotFound(
				schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"},
				plural+"."+group),
		},
		{
			name: "UnmanagedCRD",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))
					return fake.NewFakeClient(&c)
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version))},
		},
		{
			name: "ManagedCRD",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))
					return fake.NewFakeClient(&c)
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, nsLabel: "true"}))},
		},
		{
			name: "ManagedCRDAlreadyLabeled",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					c := crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, nsLabel: "true"}))
					return fake.NewFakeClient(&c)
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, nsLabel: "true"}))},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, nsLabel: "true"}))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			h := &packageHandler{
				kube: tt.fields.clientFunc(),
				ext:  tt.fields.ext,
				log:  logging.NewNopLogger(),
			}
			fn := h.createNamespaceLabelsCRDHandler()
			gotErr := fn(tt.args.ctx, tt.args.crds)

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("packageHandler.createNamespaceLabelsCRDHandler.fn(...): -want error, +got error: %s", diff)
			}

			if tt.want != nil {
				for _, wanted := range tt.want {
					got := &apiextensionsv1beta1.CustomResourceDefinition{}
					assertKubernetesObject(t, g, got, &wanted, h.kube)
				}
			}
		})
	}
}

func Test_packageHandler_createMultipleParentLabelsCRDHandler(t *testing.T) {
	type fields struct {
		clientFunc func() client.Client
		ext        *v1alpha1.Package
	}
	type args struct {
		ctx  context.Context
		crds []apiextensionsv1beta1.CustomResourceDefinition
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	var (
		label = packagespkg.MultiParentLabel(resource())
	)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []apiextensionsv1beta1.CustomResourceDefinition
		wantErr error
	}{
		{
			name: "MissingCRD",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					return fake.NewFakeClient()
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(
						withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}),
					),
				},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{},
			wantErr: kerrors.NewNotFound(
				schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"},
				plural+"."+group),
		},
		{
			name: "UnmanagedCRD",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))
					return fake.NewFakeClient(&c)
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version))},
		},
		{
			name: "ManagedCRD",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))
					return fake.NewFakeClient(&c)
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label: "true"}))},
		},
		{
			name: "ManagedCRDAlreadyLabeled",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label: "true"}))
					return fake.NewFakeClient(&c)
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label: "true"}))},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label: "true"}))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			h := &packageHandler{
				kube: tt.fields.clientFunc(),
				ext:  tt.fields.ext,
				log:  logging.NewNopLogger(),
			}
			fn := h.createMultipleParentLabelsCRDHandler()
			gotErr := fn(tt.args.ctx, tt.args.crds)

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("packageHandler.createMultipleParentLabelsCRDHandler.fn(...): -want error, +got error: %s", diff)
			}

			if tt.want != nil {
				for _, wanted := range tt.want {
					got := &apiextensionsv1beta1.CustomResourceDefinition{}
					assertKubernetesObject(t, g, got, &wanted, h.kube)
				}
			}
		})
	}
}

func Test_packageHandler_createPersonaClusterRolesCRDHandler(t *testing.T) {
	type fields struct {
		clientFunc func() client.Client
		ext        *v1alpha1.Package
	}
	type args struct {
		ctx  context.Context
		crds []apiextensionsv1beta1.CustomResourceDefinition
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	var (
		name = packagespkg.PersonaRoleName(resource(), "view")
	)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []rbac.ClusterRole
		wantErr error
	}{
		{
			name: "CreateFailed",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					return &test.MockClient{MockCreate: test.NewMockCreateFn(errBoom)}
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))},
			},
			want:    nil,
			wantErr: errors.Wrap(errBoom, "failed to create persona cluster roles"),
		},
		{
			name: "ExistingClusterRole",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					c := clusterRole(name)
					return fake.NewFakeClient(&c)
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))},
			},
			want: []rbac.ClusterRole{clusterRole(name)},
		},
		{
			name: "WithSubresources",
			fields: fields{
				ext: resource(),
				clientFunc: func() client.Client {
					return fake.NewFakeClient()
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDSubresources())},
			},
			want: []rbac.ClusterRole{clusterRole(name, withClusterRoleLabels(map[string]string{
				"core.crossplane.io/parent-group":                "",
				"core.crossplane.io/parent-kind":                 "",
				"core.crossplane.io/parent-name":                 "cool-package",
				"core.crossplane.io/parent-namespace":            "cool-namespace",
				"core.crossplane.io/parent-version":              "",
				"namespace.crossplane.io/cool-namespace":         "true",
				"rbac.crossplane.io/aggregate-to-namespace-view": "true",
			}), withClusterRoleRules([]rbac.PolicyRule{{Verbs: []string{"get", "list", "watch"}, APIGroups: []string{group}, Resources: []string{plural, plural + "/status", plural + "/scale"}}}))},
		},
		{
			name: "WithClusterScope",
			fields: fields{
				ext: resource(withPermissionScope("Cluster")),
				clientFunc: func() client.Client {
					return fake.NewFakeClient()
				},
			},
			args: args{
				ctx: context.TODO(),
				crds: []apiextensionsv1beta1.CustomResourceDefinition{
					crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDScope(apiextensionsv1beta1.ClusterScoped))},
			},
			want: []rbac.ClusterRole{clusterRole(name, withClusterRoleLabels(map[string]string{
				"core.crossplane.io/parent-group":                  "",
				"core.crossplane.io/parent-kind":                   "",
				"core.crossplane.io/parent-name":                   "cool-package",
				"core.crossplane.io/parent-namespace":              "cool-namespace",
				"core.crossplane.io/parent-version":                "",
				"rbac.crossplane.io/aggregate-to-environment-view": "true",
			}), withClusterRoleRules([]rbac.PolicyRule{{Verbs: []string{"get", "list", "watch"}, APIGroups: []string{group}, Resources: []string{plural}}}))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			h := &packageHandler{
				kube: tt.fields.clientFunc(),
				ext:  tt.fields.ext,
				log:  logging.NewNopLogger(),
			}
			fn := h.createPersonaClusterRolesCRDHandler()
			gotErr := fn(tt.args.ctx, tt.args.crds)

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Fatalf("packageHandler.createPersonaClusterRolesCRDHandler.fn(...): -want error, +got error: %s", diff)
			}

			if tt.want != nil {
				for _, wanted := range tt.want {
					got := &rbac.ClusterRole{}
					assertKubernetesObject(t, g, got, &wanted, h.kube)
				}
			}
		})
	}
}

func assertKubernetesObject(t *testing.T, g *GomegaWithT, got objectWithGVK, want metav1.Object, kube client.Client) {
	n := types.NamespacedName{Name: want.GetName(), Namespace: want.GetNamespace()}
	g.Expect(kube.Get(ctx, n, got)).NotTo(HaveOccurred())

	// NOTE(muvaf): retrieved objects have TypeMeta and ObjectMeta.ResourceVersion
	// filled but since we work on strong-typed objects, we don't need to check
	// them.
	got.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{})
	got.SetResourceVersion(want.GetResourceVersion())

	if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
		t.Errorf("-want, +got:\n%s", diff)
	}
}

func Test_packageHandler_removeCRDLabels(t *testing.T) {
	type fields struct {
		clientFn func() client.Client
		ext      *v1alpha1.Package
	}

	const (
		group      = "samples.upbound.io"
		version    = "v1alpha1"
		kind       = "Mytype"
		plural     = "mytypes"
		apiVersion = group + "/" + version
	)

	var (
		label = packagespkg.MultiParentLabel(resource())
	)

	tests := []struct {
		name    string
		fields  fields
		want    []apiextensionsv1beta1.CustomResourceDefinition
		wantErr error
	}{
		{
			name: "CouldNotListCRDs",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				clientFn: func() client.Client {
					return &test.MockClient{
						MockList: test.NewMockListFn(errBoom),
					}
				},
			},
			wantErr: errors.Wrap(errBoom, "CRDs could not be listed"),
		},
		{
			name: "UnmanagedCRD",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				clientFn: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version))
					return fake.NewFakeClient(&c)
				},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version))},
		},
		{
			name: "ManagedWithoutMultiParentLabel",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				clientFn: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))
					return fake.NewFakeClient(&c)
				},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))},
		},
		{
			name: "PatchFailed",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				clientFn: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))
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
			name: "ManagedWithMultiParentLabel",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion})),
				clientFn: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label: "true"}))
					return fake.NewFakeClient(&c)
				},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager}))},
		},
		{
			name: "PackagesSharingOneOfTwoCRDs",
			fields: fields{
				ext: resource(withCRDs(metav1.TypeMeta{Kind: kind, APIVersion: apiVersion}, metav1.TypeMeta{Kind: kind + "2", APIVersion: apiVersion})),
				clientFn: func() client.Client {
					c := crd(withCRDGroupKind(group, kind),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label: "true"}))
					c2 := crd(withCRDGroupKind(group, kind+"2"),
						withCRDVersion(version),
						withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label: "true", label + ".package2": "true"}))

					return fake.NewFakeClient(&c, &c2)
				},
			},
			want: []apiextensionsv1beta1.CustomResourceDefinition{
				crd(withCRDGroupKind(group, kind),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager})),
				crd(withCRDGroupKind(group, kind+"2"),
					withCRDVersion(version),
					withCRDLabels(map[string]string{packagespkg.LabelKubernetesManagedBy: packagespkg.LabelValuePackageManager, label + ".package2": "true"})),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			h := &packageHandler{
				kube: tt.fields.clientFn(),
				ext:  tt.fields.ext,
				log:  logging.NewNopLogger(),
			}
			gotErr := h.removeCRDLabels(context.TODO())

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("-want error, +got error:\n%s", diff)
			}

			if tt.want != nil {
				for _, wanted := range tt.want {
					got := &apiextensionsv1beta1.CustomResourceDefinition{}
					assertKubernetesObject(t, g, got, &wanted, h.kube)
				}
			}
		})
	}
}

func Test_packageHandler_validatePackagePermissions(t *testing.T) {
	everything := []rbac.PolicyRule{{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	}}

	mixed := append(packagespkg.PackageCoreRBACRules, everything...)
	tests := []struct {
		name      string
		allowCore bool
		ext       *v1alpha1.Package
		wantErr   error
	}{
		{
			name:      "DefaultPolicy",
			allowCore: false,
			ext:       resource(withPolicyRules(packagespkg.PackageCoreRBACRules)),
			wantErr:   nil,
		},
		{
			name:      "DefaultPolicyWithoutRestrictions",
			allowCore: true,
			ext:       resource(withPolicyRules(packagespkg.PackageCoreRBACRules)),
			wantErr:   nil,
		},
		{
			name:      "EverythingPolicy",
			allowCore: false,
			ext:       resource(withPolicyRules(everything)),
			wantErr:   errors.New("permissions contain a restricted rule"),
		},
		{
			name:      "EverythingPolicyWithoutRestrictions",
			allowCore: true,
			ext:       resource(withPolicyRules(everything)),
			wantErr:   nil,
		},
		{
			name:      "MixedPolicy",
			allowCore: false,
			ext:       resource(withPolicyRules(mixed)),
			wantErr:   errors.New("permissions contain a restricted rule"),
		},
		{
			name:      "MixedPolicyWithoutRestrictions",
			allowCore: true,
			ext:       resource(withPolicyRules(mixed)),
			wantErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &packageHandler{
				allowCore: tt.allowCore,
				ext:       tt.ext,
				log:       logging.NewNopLogger(),
			}
			gotErr := h.validatePackagePermissions()

			if diff := cmp.Diff(tt.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("-want error, +got error:\n%s", diff)
			}
		})
	}
}

func Test_policiesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    rbac.PolicyRule
		b    rbac.PolicyRule
		want bool
	}{
		{
			name: "EmptyMatch",
			a:    rbac.PolicyRule{},
			b:    rbac.PolicyRule{},
			want: true,
		},
		{
			name: "APIGroups",
			a:    rbac.PolicyRule{APIGroups: []string{"foo"}},
			b:    rbac.PolicyRule{APIGroups: []string{"bar"}},
			want: false,
		},
		{
			name: "Resources",
			a:    rbac.PolicyRule{Resources: []string{"foo"}},
			b:    rbac.PolicyRule{Resources: []string{"bar"}},
			want: false,
		},
		{
			name: "Verbs",
			a:    rbac.PolicyRule{Verbs: []string{"foo"}},
			b:    rbac.PolicyRule{Verbs: []string{"bar"}},
			want: false,
		},
		{
			name: "ResourceNames",
			a:    rbac.PolicyRule{ResourceNames: []string{"foo"}},
			b:    rbac.PolicyRule{ResourceNames: []string{"bar"}},
			want: false,
		},
		{
			name: "NonResourceURLs",
			a:    rbac.PolicyRule{NonResourceURLs: []string{"foo"}},
			b:    rbac.PolicyRule{NonResourceURLs: []string{"bar"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policiesEqual(tt.a, tt.b)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("policiesEqual(), -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_isDefaultPackagePolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy rbac.PolicyRule
		want   bool
	}{
		{
			name:   "EmptyPolicy",
			policy: rbac.PolicyRule{},
			want:   false,
		},
		{
			name:   "DefaultPolicy",
			policy: packagespkg.PackageCoreRBACRules[0],
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDefaultPackagePolicy(tt.policy)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("isDefaultPackagePolicy(), -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_isPermittedAPIGroup(t *testing.T) {
	tests := []struct {
		name     string
		apiGroup string
		want     bool
	}{
		{
			name:     "DotLess",
			apiGroup: "",
			want:     false,
		},
		{
			name:     "k8s.io",
			apiGroup: "foo.k8s.io",
			want:     false,
		},
		{
			name:     "Permitted",
			apiGroup: "foo.package.example.com",
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermittedAPIGroup(tt.apiGroup)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("isPermittedAPIGroup(), -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_isPermittedPackagePolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy rbac.PolicyRule
		want   bool
	}{
		{
			name:   "EmptyPolicy",
			policy: rbac.PolicyRule{},
			want:   false,
		},
		{
			name:   "EmptyAPIGroup",
			policy: rbac.PolicyRule{APIGroups: []string{""}, Resources: []string{"*"}, Verbs: []string{"*"}},
			want:   false,
		},
		{
			name:   "Everything",
			policy: rbac.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			want:   false,
		},
		{
			name:   "NonResourceURLPolicy",
			policy: rbac.PolicyRule{NonResourceURLs: []string{"foo"}},
			want:   false,
		},
		{
			name:   "DefaultPolicy",
			policy: packagespkg.PackageCoreRBACRules[0],
			want:   true,
		},
		{
			name: "PermittedPolicy",
			policy: rbac.PolicyRule{
				APIGroups: []string{"foo.bar", "bar.foo"},
				Resources: []string{"foo", "bar"},
				Verbs:     []string{"*"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPermittedPackagePolicy(tt.policy)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("isPermittedPackagePolicy(), -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_stringSlicesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "Empty",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "Uneven",
			a:    []string{"a"},
			b:    []string{"a", "b"},
			want: false,
		},
		{
			name: "OrderMatters",
			a:    []string{"b", "a"},
			b:    []string{"a", "b"},
			want: false,
		},
		{
			name: "Mismatched",
			a:    []string{"b", "a"},
			b:    []string{"c", "d"},
			want: false,
		},
		{
			name: "Matched",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "c"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringSlicesEqual(tt.a, tt.b)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("stringSlicesEqual(), -want, +got:\n%s", diff)
			}
		})
	}
}

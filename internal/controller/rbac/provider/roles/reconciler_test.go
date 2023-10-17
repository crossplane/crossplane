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

package roles

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	now := metav1.Now()
	family := "litfam"

	ourUID := types.UID("our-own-uid")
	familyUID := types.UID("uid-of-another-provider-in-our-family")

	type args struct {
		mgr  manager.Manager
		opts []ReconcilerOption
	}
	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderRevisionNotFound": {
			reason: "We should not return an error if the ProviderRevision was not found.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetProviderRevisionError": {
			reason: "We should return any other error encountered while getting a ProviderRevision.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetPR),
			},
		},
		"ProviderRevisionDeleted": {
			reason: "We should return early if the namespace was deleted.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetDeletionTimestamp(&now)
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ListProviderRevisionsError": {
			reason: "We should return an error encountered listing ProviderRevisions.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								pr := obj.(*v1.ProviderRevision)
								pr.SetLabels(map[string]string{v1.LabelProviderFamily: family})
								return nil
							}),
							MockList: test.NewMockListFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListPRs),
			},
		},
		"ValidatePermissionRequestsError": {
			reason: "We should return an error encountered validating permission requests.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(nil),
						},
					}),
					WithPermissionRequestsValidator(PermissionRequestsValidatorFn(func(ctx context.Context, requested ...rbacv1.PolicyRule) ([]Rule, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errValidatePermissions),
			},
		},
		"PermissionRequestRejected": {
			reason: "We should return early without requeuing when a permission request is rejected.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(nil),
						},
					}),
					WithPermissionRequestsValidator(PermissionRequestsValidatorFn(func(ctx context.Context, requested ...rbacv1.PolicyRule) ([]Rule, error) {
						return []Rule{{}}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ApplyClusterRoleError": {
			reason: "We should return an error encountered applying a ClusterRole.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v1.ProviderRevision, []Resource) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{}}
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyRole),
			},
		},
		"SuccessfulNoOp": {
			reason: "We should not requeue when no ClusterRoles need applying.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(nil),
						},
						Applicator: resource.ApplyFn(func(ctx context.Context, o client.Object, ao ...resource.ApplyOption) error {
							// Simulate a no-op change by not allowing the update.
							return resource.AllowUpdateIf(func(_, _ runtime.Object) bool { return false })(ctx, o, o)
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v1.ProviderRevision, []Resource) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{}}
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulApply": {
			reason: "We should not requeue when we successfully apply our ClusterRoles.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetUID(ourUID)
								pr.SetLabels(map[string]string{v1.LabelProviderFamily: family})
								pr.Spec.Package = "cool/provider:v1.0.0"
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*v1.ProviderRevisionList)
								l.Items = []v1.ProviderRevision{
									{
										ObjectMeta: metav1.ObjectMeta{UID: familyUID},
										Spec:       v1.ProviderRevisionSpec{PackageRevisionSpec: v1.PackageRevisionSpec{Package: "cool/other-provider:v1.0.0"}},
									},
									{
										ObjectMeta: metav1.ObjectMeta{UID: familyUID},
										Spec:       v1.ProviderRevisionSpec{PackageRevisionSpec: v1.PackageRevisionSpec{Package: "evil/other-provider:v1.0.0"}},
									},
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v1.ProviderRevision, []Resource) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{}}
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, append(tc.args.opts, WithLogger(testLog))...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDefinedResources(t *testing.T) {
	cases := map[string]struct {
		refs []xpv1.TypedReference
		want []Resource
	}{
		"UnparseableAPIVersion": {
			refs: []xpv1.TypedReference{{
				APIVersion: "too/many/slashes",
			}},
			want: []Resource{},
		},
		"WrongGroup": {
			refs: []xpv1.TypedReference{{
				APIVersion: "example.org/v1",
				Kind:       "CustomResourceDefinition",
			}},
			want: []Resource{},
		},
		"WrongKind": {
			refs: []xpv1.TypedReference{{
				APIVersion: extv1.SchemeGroupVersion.String(),
				Kind:       "ConversionReview",
			}},
			want: []Resource{},
		},
		"InvalidName": {
			refs: []xpv1.TypedReference{{
				APIVersion: extv1.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
				Name:       "I'm different!",
			}},
			want: []Resource{},
		},
		"DefinedResource": {
			refs: []xpv1.TypedReference{{
				APIVersion: extv1.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
				Name:       "pinballs.example.org",
			}},
			want: []Resource{{
				Group:  "example.org",
				Plural: "pinballs",
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := DefinedResources(tc.refs)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("DefinedResources(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestClusterRolesDiffer(t *testing.T) {
	cases := map[string]struct {
		current runtime.Object
		desired runtime.Object
		want    bool
	}{
		"Equal": {
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			want: false,
		},
		"LabelsDiffer": {
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"b": "b"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			want: true,
		},
		"RulesDiffer": {
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ClusterRolesDiffer(tc.current, tc.desired)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ClusterRolesDiffer(...): -want, +got\n:%s", diff)
			}
		})
	}
}

func TestOrgDiffer(t *testing.T) {
	cases := map[string]struct {
		registry string
		a        string
		b        string
		want     bool
	}{
		"SameOrg": {
			registry: "xpkg.example.org",
			a:        "xpkg.example.org/cool/provider:v1.0.0",
			b:        "cool/other-provider:v1.0.0",
			want:     false,
		},
		"DifferentOrgs": {
			registry: "xpkg.example.org",
			a:        "cool/provider:v1.0.0",
			b:        "evil/other-provider:v1.0.0",
			want:     true,
		},
		"DifferentRegistries": {
			registry: "xpkg.example.org",
			a:        "xpkg.example.org/cool/provider:v1.0.0",
			b:        "index.docker.io/cool/other-provider:v1.0.0",
			want:     true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := OrgDiffer{DefaultRegistry: tc.registry}
			got := d.Differs(tc.a, tc.b)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("SameOrg(...): -want, +got\n:%s", diff)
			}
		})
	}
}

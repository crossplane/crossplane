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
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
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
		"ListOwnedResourcesError": {
			reason: "We should return an error encountered listing the CRDs owned by the ProviderRevision.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								o.(*v1.ProviderRevision).SetName("provider-revision")
								return nil
							}),
							MockList: test.NewMockListFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, "cannot list CustomResourceDefinitions for ProviderRevision %q", "provider-revision"),
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
						Applicator: resource.ApplyFn(func(ctx context.Context, o client.Object, _ ...resource.ApplyOption) error {
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
								// ownedResources also lists CRDs and MRDs; only
								// populate the ProviderRevision family list here.
								l, ok := o.(*v1.ProviderRevisionList)
								if !ok {
									return nil
								}
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
		"PauseReconcile": {
			reason: "Pause reconciliation if the pause annotation is set.",
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
								pr.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
								return nil
							}),
						},
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

// clusterRoleGrants reports whether the ClusterRole grants any verb on the
// supplied API group and resource.
func clusterRoleGrants(cr rbacv1.ClusterRole, group, plural string) bool {
	for _, rule := range cr.Rules {
		if slices.Contains(rule.APIGroups, group) && slices.Contains(rule.Resources, plural) {
			return true
		}
	}

	return false
}

// TestReconcileGrantsOwnedResources covers the case where a ProviderRevision's
// status.objectRefs is incomplete - e.g. lost after a backup/restore, or not yet
// updated after a managed resource was established. The provider's system
// ClusterRole must still grant access to the CRDs and MRDs the revision controls
// (derived from the live API server), otherwise the provider crashes for lack of
// RBAC on resources it runs controllers for.
func TestReconcileGrantsOwnedResources(t *testing.T) {
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	prUID := types.UID("provider-revision-uid")

	// ownedBy points a resource's controller reference at our ProviderRevision.
	ownedBy := []metav1.OwnerReference{{
		APIVersion: "pkg.crossplane.io/v1",
		Kind:       "ProviderRevision",
		Name:       "provider-revision",
		UID:        prUID,
		Controller: ptr.To(true),
	}}

	// getProviderRevision returns a Get that populates our ProviderRevision with
	// the supplied references in its status.objectRefs.
	getProviderRevision := func(refs ...xpv2.TypedReference) test.MockGetFn {
		return test.NewMockGetFn(nil, func(o client.Object) error {
			pr := o.(*v1.ProviderRevision)
			pr.SetName("provider-revision")
			pr.SetUID(prUID)
			pr.Status.ObjectRefs = refs
			return nil
		})
	}

	// listOwned returns a List that returns the supplied CRD and MRD metadata.
	// ownedResources lists CRDs and MRDs as metadata only, so both calls arrive
	// as PartialObjectMetadataLists distinguished by their kind.
	listOwned := func(crds, mrds []metav1.PartialObjectMetadata) test.MockListFn {
		return test.NewMockListFn(nil, func(o client.ObjectList) error {
			l, ok := o.(*metav1.PartialObjectMetadataList)
			if !ok {
				return nil
			}
			switch l.GetObjectKind().GroupVersionKind().Kind {
			case "CustomResourceDefinitionList":
				l.Items = crds
			case "ManagedResourceDefinitionList":
				l.Items = mrds
			}
			return nil
		})
	}

	type args struct {
		client client.Client
	}

	type want struct {
		// grants maps a "group/plural" resource to whether the provider's system
		// ClusterRole should grant access to it.
		grants map[string]bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"OwnedResourcesAbsentFromObjectRefs": {
			reason: "Resources the revision owns must be granted even when status.objectRefs is empty, and resources it doesn't own must not be.",
			args: args{
				client: &test.MockClient{
					MockGet: getProviderRevision(),
					MockList: listOwned(
						[]metav1.PartialObjectMetadata{
							{ObjectMeta: metav1.ObjectMeta{Name: "policies.rbac.example.org", OwnerReferences: ownedBy}},
							// Not controlled by our revision - must be ignored.
							{ObjectMeta: metav1.ObjectMeta{Name: "widgets.unowned.example.org"}},
						},
						[]metav1.PartialObjectMetadata{
							{ObjectMeta: metav1.ObjectMeta{Name: "tests.synthetic.example.org", OwnerReferences: ownedBy}},
						},
					),
				},
			},
			want: want{
				grants: map[string]bool{
					"rbac.example.org/policies":   true,
					"synthetic.example.org/tests": true,
					"unowned.example.org/widgets": false,
				},
			},
		},
		"UnionOfObjectRefsAndOwnedResources": {
			reason: "Resources from status.objectRefs and owned resources must both be granted.",
			args: args{
				client: &test.MockClient{
					MockGet: getProviderRevision(xpv2.TypedReference{
						APIVersion: extv1.SchemeGroupVersion.String(),
						Kind:       "CustomResourceDefinition",
						Name:       "monitors.alerting.example.org",
					}),
					MockList: listOwned(
						[]metav1.PartialObjectMetadata{
							{ObjectMeta: metav1.ObjectMeta{Name: "policies.rbac.example.org", OwnerReferences: ownedBy}},
						},
						nil,
					),
				},
			},
			want: want{
				grants: map[string]bool{
					"alerting.example.org/monitors": true,
					"rbac.example.org/policies":     true,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			applied := map[string]rbacv1.ClusterRole{}
			ca := resource.ClientApplicator{
				Client: tc.args.client,
				Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
					cr := o.(*rbacv1.ClusterRole)
					applied[cr.GetName()] = *cr.DeepCopy()
					return nil
				}),
			}

			r := NewReconciler(&fake.Manager{}, WithClientApplicator(ca), WithLogger(testLog))
			if _, err := r.Reconcile(context.Background(), reconcile.Request{}); err != nil {
				t.Fatalf("%s\nr.Reconcile(...): unexpected error: %v", tc.reason, err)
			}

			system := applied[SystemClusterRoleName("provider-revision")]

			got := make(map[string]bool, len(tc.want.grants))
			for gr := range tc.want.grants {
				group, plural, _ := strings.Cut(gr, "/")
				got[gr] = clusterRoleGrants(system, group, plural)
			}

			if diff := cmp.Diff(tc.want.grants, got); diff != "" {
				t.Errorf("%s\nr.Reconcile(...): -want grants, +got grants:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDefinedResources(t *testing.T) {
	cases := map[string]struct {
		refs []xpv2.TypedReference
		want []Resource
	}{
		"UnparseableAPIVersion": {
			refs: []xpv2.TypedReference{{
				APIVersion: "too/many/slashes",
			}},
			want: []Resource{},
		},
		"WrongGroup": {
			refs: []xpv2.TypedReference{{
				APIVersion: "example.org/v1",
				Kind:       "CustomResourceDefinition",
			}},
			want: []Resource{},
		},
		"WrongKind": {
			refs: []xpv2.TypedReference{{
				APIVersion: extv1.SchemeGroupVersion.String(),
				Kind:       "ConversionReview",
			}},
			want: []Resource{},
		},
		"InvalidName": {
			refs: []xpv2.TypedReference{{
				APIVersion: extv1.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
				Name:       "I'm different!",
			}},
			want: []Resource{},
		},
		"DefinedResource": {
			refs: []xpv2.TypedReference{{
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
		a    string
		b    string
		want bool
	}{
		"SameOrgWithRegistry": {
			a:    "xpkg.example.org/cool/provider:v1.0.0",
			b:    "xpkg.example.org/cool/other-provider:v1.0.0",
			want: false,
		},
		"SameOrgWithNoRegistry": {
			a:    "cool/provider:v1.0.0",
			b:    "cool/other-provider:v1.0.0",
			want: false,
		},
		"DifferentOrgsWithSameRegistry": {
			a:    "xpkg.example.org/cool/provider:v1.0.0",
			b:    "xpkg.example.org/evil/other-provider:v1.0.0",
			want: true,
		},
		"DifferentOrgsWithDifferentRegistries": {
			a:    "xpkg.example.org/cool/provider:v1.0.0",
			b:    "index.docker.io/cool/other-provider:v1.0.0",
			want: true,
		},
		"DifferentOrgsWithNoRegistryOnA": {
			a:    "cool/provider:v1.0.0",
			b:    "xpkg.example.org/cool/other-provider:v1.0.0",
			want: true,
		},
		"DifferentOrgsWithNoRegistryOnB": {
			a:    "index.docker.io/cool/provider:v1.0.0",
			b:    "cool/other-provider:v1.0.0",
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := OrgDiffer{}

			got := d.Differs(tc.a, tc.b)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("SameOrg(...): -want, +got\n:%s", diff)
			}
		})
	}
}

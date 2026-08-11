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

package binding

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	now := metav1.Now()

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
		"ListDeploymentsError": {
			reason: "We should return an error encountered listing Deployments.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								return nil
							}),
							MockList: test.NewMockListFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeployments),
			},
		},
		"ApplyClusterRoleBindingError": {
			reason: "We should return an error encountered applying a ClusterRoleBinding.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								d.Spec.DesiredState = v1.PackageRevisionActive
								return nil
							}),
							MockList: test.NewMockListFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyBinding),
			},
		},
		"SuccessfulApply": {
			reason: "We should not requeue when we successfully apply our ClusterRoleBindings.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetName("cool-provider")
								d.SetUID(types.UID("cool-uid"))
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								d.Spec.DesiredState = v1.PackageRevisionActive
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*appsv1.DeploymentList)
								l.Items = []appsv1.Deployment{{
									ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{
										APIVersion: v1.SchemeGroupVersion.String(),
										Kind:       v1.ProviderRevisionKind,
										Name:       "cool-provider",
										UID:        types.UID("cool-uid"),
										Controller: new(true),
									}}},
								}}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return nil
						}),
					}),
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
								d := o.(*v1.ProviderRevision)
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								d.Spec.DesiredState = v1.PackageRevisionActive
								d.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
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

func TestReconcileSubjects(t *testing.T) {
	const namespace = "crossplane-system"

	pr := &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{
		Name: "cool-provider",
		UID:  types.UID("cool-uid"),
	}}
	controller := metav1.OwnerReference{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       v1.ProviderRevisionKind,
		Name:       pr.GetName(),
		UID:        pr.GetUID(),
		Controller: new(true),
	}

	wrongAPIVersion := controller
	wrongAPIVersion.APIVersion = "pkg.crossplane.io/v1alpha1"
	wrongKind := controller
	wrongKind.Kind = "ConfigurationRevision"
	wrongName := controller
	wrongName.Name = "some-other-provider"
	wrongUID := controller
	wrongUID.UID = types.UID("some-other-uid")
	notController := controller
	notController.Controller = new(false)

	deployment := func(name, ns, sa string, ref metav1.OwnerReference) appsv1.Deployment {
		return appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       ns,
				OwnerReferences: []metav1.OwnerReference{ref},
			},
			Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{ServiceAccountName: sa},
			}},
		}
	}

	var applied *rbacv1.ClusterRoleBinding
	r := NewReconciler(&fake.Manager{},
		WithNamespace(namespace),
		WithClientApplicator(resource.ClientApplicator{
			Client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					pr.DeepCopyInto(o.(*v1.ProviderRevision))
					return nil
				}),
				MockList: func(_ context.Context, o client.ObjectList, opts ...client.ListOption) error {
					got := (&client.ListOptions{}).ApplyOptions(opts).Namespace
					if got != namespace {
						t.Errorf("List(...): got namespace %q, want %q", got, namespace)
					}

					l := o.(*appsv1.DeploymentList)
					l.Items = []appsv1.Deployment{
						deployment("runtime", namespace, "runtime-sa", controller),
						deployment("foreign", "other-namespace", "foreign-sa", controller),
						deployment("wrong-api-version", namespace, "wrong-api-version-sa", wrongAPIVersion),
						deployment("wrong-kind", namespace, "wrong-kind-sa", wrongKind),
						deployment("wrong-name", namespace, "wrong-name-sa", wrongName),
						deployment("wrong-uid", namespace, "wrong-uid-sa", wrongUID),
						deployment("not-controller", namespace, "not-controller-sa", notController),
					}
					return nil
				},
			},
			Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
				applied = o.(*rbacv1.ClusterRoleBinding).DeepCopy()
				return nil
			}),
		}))

	if _, err := r.Reconcile(context.Background(), reconcile.Request{}); err != nil {
		t.Fatalf("Reconcile(...): unexpected error: %s", err)
	}
	if applied == nil {
		t.Fatal("Reconcile(...): did not apply a ClusterRoleBinding")
	}

	want := []rbacv1.Subject{{
		Kind:      rbacv1.ServiceAccountKind,
		Namespace: namespace,
		Name:      "runtime-sa",
	}}
	if diff := cmp.Diff(want, applied.Subjects); diff != "" {
		t.Errorf("Reconcile(...): -want subjects, +got subjects:\n%s", diff)
	}
}

func TestInNamespace(t *testing.T) {
	const namespace = "crossplane-system"

	p := inNamespace(namespace)
	for name, tc := range map[string]struct {
		namespace string
		want      bool
	}{
		"PackageRuntimeNamespace": {namespace: namespace, want: true},
		"ForeignNamespace":        {namespace: "other-namespace", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			got := p.Create(ctrlevent.CreateEvent{Object: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: tc.namespace},
			}})
			if got != tc.want {
				t.Errorf("inNamespace(%q): got %t, want %t", tc.namespace, got, tc.want)
			}
		})
	}
}

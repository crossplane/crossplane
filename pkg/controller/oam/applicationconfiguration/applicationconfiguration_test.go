/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	htcp://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package applicationconfiguration

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

type acParam func(*v1alpha2.ApplicationConfiguration)

func withConditions(c ...runtimev1alpha1.Condition) acParam {
	return func(ac *v1alpha2.ApplicationConfiguration) {
		ac.SetConditions(c...)
	}
}

func withWorkloadStatuses(ws ...v1alpha2.WorkloadStatus) acParam {
	return func(ac *v1alpha2.ApplicationConfiguration) {
		ac.Status.Workloads = ws
	}
}

func ac(p ...acParam) *v1alpha2.ApplicationConfiguration {
	ac := &v1alpha2.ApplicationConfiguration{}
	for _, fn := range p {
		fn(ac)
	}
	return ac
}

func TestReconciler(t *testing.T) {
	errBoom := errors.New("boom")
	errUnexpectedStatus := errors.New("unexpected status")

	namespace := "ns"
	componentName := "coolcomponent"

	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion("v")
	workload.SetKind("workload")
	workload.SetNamespace(namespace)
	workload.SetName("workload")

	trait := &unstructured.Unstructured{}
	trait.SetAPIVersion("v")
	trait.SetKind("trait")
	trait.SetNamespace(namespace)
	trait.SetName("trait")

	type args struct {
		m manager.Manager
		o []ReconcilerOption
	}
	type want struct {
		result reconcile.Result
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GetApplicationConfigurationError": {
			reason: "Errors getting the ApplicationConfiguration under reconciliation should be returned",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetAppConfig),
			},
		},
		"RenderComponentsError": {
			reason: "Errors rendering components should be reflected as a status condition",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {

							want := ac(withConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errRenderComponents))))
							if diff := cmp.Diff(want, o.(*v1alpha2.ApplicationConfiguration)); diff != "" {
								t.Errorf("\nclient.Status().Update(): -want, +got:\n%s", diff)
								return errUnexpectedStatus
							}

							return nil
						}),
					},
				},
				o: []ReconcilerOption{
					WithRenderer(ComponentRenderFn(func(_ context.Context, _ *v1alpha2.ApplicationConfiguration) ([]Workload, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ApplyComponentsError": {
			reason: "Errors applying components should be reflected as a status condition",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
							want := ac(withConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errApplyComponents))))
							if diff := cmp.Diff(want, o.(*v1alpha2.ApplicationConfiguration)); diff != "" {
								t.Errorf("\nclient.Status().Update(): -want, +got:\n%s", diff)
								return errUnexpectedStatus
							}
							return nil
						}),
					},
				},
				o: []ReconcilerOption{
					WithRenderer(ComponentRenderFn(func(_ context.Context, _ *v1alpha2.ApplicationConfiguration) ([]Workload, error) {
						return []Workload{{Workload: workload}}, nil
					})),
					WithApplicator(WorkloadApplyFn(func(_ context.Context, _ []Workload, _ ...resource.ApplyOption) error {
						return errBoom
					})),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"GCDeleteError": {
			reason: "Errors deleting a garbage collected resource should be reflected as a status condition",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockDelete: test.NewMockDeleteFn(errBoom),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
							want := ac(withConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errGCComponent))))
							if diff := cmp.Diff(want, o.(*v1alpha2.ApplicationConfiguration)); diff != "" {
								t.Errorf("\nclient.Status().Update(): -want, +got:\n%s", diff)
								return errUnexpectedStatus
							}
							return nil
						}),
					},
				},
				o: []ReconcilerOption{
					WithRenderer(ComponentRenderFn(func(_ context.Context, _ *v1alpha2.ApplicationConfiguration) ([]Workload, error) {
						return []Workload{}, nil
					})),
					WithApplicator(WorkloadApplyFn(func(_ context.Context, _ []Workload, _ ...resource.ApplyOption) error {
						return nil
					})),
					WithGarbageCollector(GarbageCollectorFn(func(_ string, _ []v1alpha2.WorkloadStatus, _ []Workload) []unstructured.Unstructured {
						return []unstructured.Unstructured{*workload}
					})),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"Success": {
			reason: "Rendered workloads should be reflected in status",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockDelete: test.NewMockDeleteFn(nil),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
							want := ac(
								withConditions(runtimev1alpha1.ReconcileSuccess()),
								withWorkloadStatuses(v1alpha2.WorkloadStatus{
									ComponentName: componentName,
									Reference: runtimev1alpha1.TypedReference{
										APIVersion: workload.GetAPIVersion(),
										Kind:       workload.GetKind(),
										Name:       workload.GetName(),
									},
								}),
							)
							if diff := cmp.Diff(want, o.(*v1alpha2.ApplicationConfiguration), cmpopts.EquateEmpty()); diff != "" {
								t.Errorf("\nclient.Status().Update(): -want, +got:\n%s", diff)
								return errUnexpectedStatus
							}
							return nil
						}),
					},
				},
				o: []ReconcilerOption{
					WithRenderer(ComponentRenderFn(func(_ context.Context, _ *v1alpha2.ApplicationConfiguration) ([]Workload, error) {
						return []Workload{{ComponentName: componentName, Workload: workload}}, nil
					})),
					WithApplicator(WorkloadApplyFn(func(_ context.Context, _ []Workload, _ ...resource.ApplyOption) error {
						return nil
					})),
					WithGarbageCollector(GarbageCollectorFn(func(_ string, _ []v1alpha2.WorkloadStatus, _ []Workload) []unstructured.Unstructured {
						return []unstructured.Unstructured{*trait}
					})),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: longWait},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.o...)
			got, err := r.Reconcile(reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWorkloadStatus(t *testing.T) {
	namespace := "ns"
	componentName := "coolcomponent"

	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion("v")
	workload.SetKind("workload")
	workload.SetNamespace(namespace)
	workload.SetName("workload")

	trait := &unstructured.Unstructured{}
	trait.SetAPIVersion("v")
	trait.SetKind("trait")
	trait.SetNamespace(namespace)
	trait.SetName("trait")

	cases := map[string]struct {
		w    Workload
		want v1alpha2.WorkloadStatus
	}{
		"Success": {
			w: Workload{
				ComponentName: componentName,
				Workload:      workload,
				Traits:        []unstructured.Unstructured{*trait},
			},
			want: v1alpha2.WorkloadStatus{
				ComponentName: componentName,
				Reference: runtimev1alpha1.TypedReference{
					APIVersion: workload.GetAPIVersion(),
					Kind:       workload.GetKind(),
					Name:       workload.GetName(),
				},
				Traits: []v1alpha2.WorkloadTrait{
					{
						Reference: runtimev1alpha1.TypedReference{
							APIVersion: trait.GetAPIVersion(),
							Kind:       trait.GetKind(),
							Name:       trait.GetName(),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.w.Status()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nw.Status(): -want, +got:\n%s\n", diff)
			}
		})
	}

}

func TestEligible(t *testing.T) {
	namespace := "ns"

	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion("v")
	workload.SetKind("workload")
	workload.SetNamespace(namespace)
	workload.SetName("workload")

	trait := &unstructured.Unstructured{}
	trait.SetAPIVersion("v")
	trait.SetKind("trait")
	trait.SetNamespace(namespace)
	trait.SetName("trait")

	type args struct {
		namespace string
		ws        []v1alpha2.WorkloadStatus
		w         []Workload
	}
	cases := map[string]struct {
		reason string
		args   args
		want   []unstructured.Unstructured
	}{
		"TraitNotApplied": {
			reason: "A referenced trait is eligible for garbage collection if it was not applied",
			args: args{
				namespace: namespace,
				ws: []v1alpha2.WorkloadStatus{
					{
						Reference: runtimev1alpha1.TypedReference{
							APIVersion: workload.GetAPIVersion(),
							Kind:       workload.GetKind(),
							Name:       workload.GetName(),
						},
						Traits: []v1alpha2.WorkloadTrait{
							{
								Reference: runtimev1alpha1.TypedReference{
									APIVersion: trait.GetAPIVersion(),
									Kind:       trait.GetKind(),
									Name:       trait.GetName(),
								},
							},
						},
					},
				},
				w: []Workload{{Workload: workload}},
			},
			want: []unstructured.Unstructured{*trait},
		},
		"NeitherApplied": {
			reason: "A referenced workload and its trait is eligible for garbage collection if they were not applied",
			args: args{
				namespace: namespace,
				ws: []v1alpha2.WorkloadStatus{
					{
						Reference: runtimev1alpha1.TypedReference{
							APIVersion: workload.GetAPIVersion(),
							Kind:       workload.GetKind(),
							Name:       workload.GetName(),
						},
						Traits: []v1alpha2.WorkloadTrait{
							{
								Reference: runtimev1alpha1.TypedReference{
									APIVersion: trait.GetAPIVersion(),
									Kind:       trait.GetKind(),
									Name:       trait.GetName(),
								},
							},
						},
					},
				},
			},
			want: []unstructured.Unstructured{*workload, *trait},
		},
		"BothApplied": {
			reason: "A referenced workload and its trait are not eligible for garbage collection if they were applied",
			args: args{
				namespace: namespace,
				ws: []v1alpha2.WorkloadStatus{
					{
						Reference: runtimev1alpha1.TypedReference{
							APIVersion: workload.GetAPIVersion(),
							Kind:       workload.GetKind(),
							Name:       workload.GetName(),
						},
						Traits: []v1alpha2.WorkloadTrait{
							{
								Reference: runtimev1alpha1.TypedReference{
									APIVersion: trait.GetAPIVersion(),
									Kind:       trait.GetKind(),
									Name:       trait.GetName(),
								},
							},
						},
					},
				},
				w: []Workload{{Workload: workload, Traits: []unstructured.Unstructured{*trait}}},
			},
			want: []unstructured.Unstructured{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := eligible(tc.args.namespace, tc.args.ws, tc.args.w)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\neligible(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

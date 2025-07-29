/*
Copyright 2025 The Crossplane Authors.

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

package activationpolicy

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// Helper functions for creating test objects.
type MRAPModifier func(mrap *v1alpha1.ManagedResourceActivationPolicy)

func NewMRAP(m ...MRAPModifier) *v1alpha1.ManagedResourceActivationPolicy {
	mrap := &v1alpha1.ManagedResourceActivationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mrap",
		},
		Spec: v1alpha1.ManagedResourceActivationPolicySpec{
			Activations: []v1alpha1.ActivationPolicy{
				"*.aws.crossplane.io",
			},
		},
	}
	for _, fn := range m {
		fn(mrap)
	}
	return mrap
}

func WithMRAPDeletionTimestamp(t time.Time) MRAPModifier {
	return func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
		mrap.SetDeletionTimestamp(&metav1.Time{Time: t})
	}
}

func WithMRAPPaused() MRAPModifier {
	return func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
		mrap.SetAnnotations(map[string]string{
			meta.AnnotationKeyReconciliationPaused: "true",
		})
	}
}

func WithMRAPActivations(activations ...string) MRAPModifier {
	return func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
		policies := make([]v1alpha1.ActivationPolicy, len(activations))
		for i, a := range activations {
			policies[i] = v1alpha1.ActivationPolicy(a)
		}
		mrap.Spec.Activations = policies
	}
}

type MRDModifier func(mrd *v1alpha1.ManagedResourceDefinition)

func NewMRD(name string, m ...MRDModifier) *v1alpha1.ManagedResourceDefinition {
	mrd := &v1alpha1.ManagedResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			State: v1alpha1.ManagedResourceDefinitionInactive,
		},
	}
	for _, fn := range m {
		fn(mrd)
	}
	return mrd
}

func WithMRDState(state v1alpha1.ManagedResourceDefinitionState) MRDModifier {
	return func(mrd *v1alpha1.ManagedResourceDefinition) {
		mrd.Spec.State = state
	}
}

// A get function that supplies the input resource.
func WithMRAP(t *testing.T, mrap *v1alpha1.ManagedResourceActivationPolicy) func(context.Context, client.ObjectKey, client.Object) error {
	t.Helper()
	return func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		t.Helper()
		if o, ok := obj.(*v1alpha1.ManagedResourceActivationPolicy); ok {
			*o = *mrap
		}
		return nil
	}
}

// A status update function that ensures the supplied object is the MRAP we want.
func WantMRAP(t *testing.T, want *v1alpha1.ManagedResourceActivationPolicy) func(context.Context, client.Object, ...client.SubResourceUpdateOption) error {
	t.Helper()
	return func(_ context.Context, got client.Object, _ ...client.SubResourceUpdateOption) error {
		t.Helper()
		if diff := cmp.Diff(want, got, cmpopts.EquateApproxTime(3*time.Second)); diff != "" {
			t.Errorf("WantMRAP(...): -want, +got: %s", diff)
		}
		return nil
	}
}

// A list function that supplies the input MRD list.
func WithMRDList(t *testing.T, mrds ...*v1alpha1.ManagedResourceDefinition) func(context.Context, client.ObjectList, ...client.ListOption) error {
	t.Helper()
	return func(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
		t.Helper()
		if o, ok := obj.(*v1alpha1.ManagedResourceDefinitionList); ok {
			items := make([]v1alpha1.ManagedResourceDefinition, len(mrds))
			for i, mrd := range mrds {
				items[i] = *mrd
			}
			o.Items = items
		}
		return nil
	}
}

// A patch function that validates the patch operation.
func WantMRDPatch(t *testing.T, expectedPatches map[string]v1alpha1.ManagedResourceDefinitionState) func(context.Context, client.Object, client.Patch, ...client.PatchOption) error {
	t.Helper()
	return func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
		t.Helper()
		if mrd, ok := obj.(*v1alpha1.ManagedResourceDefinition); ok {
			if expectedState, exists := expectedPatches[mrd.GetName()]; exists {
				if mrd.Spec.State != expectedState {
					t.Errorf("WantMRDPatch(...): expected state %s for MRD %s, got %s", expectedState, mrd.GetName(), mrd.Spec.State)
				}
			}
		}
		return nil
	}
}

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()

	type args struct {
		c client.Client
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
		"MRAPNotFound": {
			reason: "We should not return an error if the ManagedResourceActivationPolicy was not found.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"GetMRAPError": {
			reason: "We should return error encountered while getting the ManagedResourceActivationPolicy.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetMRAP),
			},
		},
		"MRAPBeingDeleted": {
			reason: "We should update status to Terminating and return without error when MRAP is being deleted.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPDeletionTimestamp(now.Time))),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.SetDeletionTimestamp(&now)
						mrap.SetConditions(v1alpha1.TerminatingActivationPolicy())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MRAPDeletionStatusUpdateError": {
			reason: "We should requeue on conflict when updating status during deletion.",
			args: args{
				c: &test.MockClient{
					MockGet:          WithMRAP(t, NewMRAP(WithMRAPDeletionTimestamp(now.Time))),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(kerrors.NewConflict(schema.GroupResource{}, "", errBoom)),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"MRAPDeletionStatusUpdateErrorNonConflict": {
			reason: "We should return error when status update fails during deletion with non-conflict error.",
			args: args{
				c: &test.MockClient{
					MockGet:          WithMRAP(t, NewMRAP(WithMRAPDeletionTimestamp(now.Time))),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateStatus),
			},
		},
		"ReconciliationPaused": {
			reason: "We should return no error and no requeue when reconciliation is paused.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPPaused())),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ListMRDError": {
			reason: "We should return error and update status when listing MRDs fails.",
			args: args{
				c: &test.MockClient{
					MockGet:  WithMRAP(t, NewMRAP()),
					MockList: test.NewMockListFn(errBoom),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.SetConditions(v1alpha1.BlockedActivationPolicy().WithMessage(errListMRD))
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListMRD),
			},
		},
		"ListMRDErrorStatusUpdateConflict": {
			reason: "We should requeue on conflict when updating status after list MRD error.",
			args: args{
				c: &test.MockClient{
					MockGet:          WithMRAP(t, NewMRAP()),
					MockList:         test.NewMockListFn(errBoom),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(kerrors.NewConflict(schema.GroupResource{}, "", errBoom)),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"NoMRDsToActivate": {
			reason: "We should succeed when no MRDs match the activation policy.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPActivations("*.aws.crossplane.io"))),
					MockList: WithMRDList(t,
						NewMRD("database.gcp.crossplane.io"),
						NewMRD("storage.azure.crossplane.io"),
					),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.Spec.Activations = []v1alpha1.ActivationPolicy{"*.aws.crossplane.io"}
						mrap.Status.Activated = nil
						mrap.SetConditions(v1alpha1.Healthy())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ActivateSingleMRD": {
			reason: "We should activate a single MRD that matches the activation policy.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPActivations("*.aws.crossplane.io"))),
					MockList: WithMRDList(t,
						NewMRD("bucket.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
						NewMRD("database.gcp.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
					),
					MockPatch: WantMRDPatch(t, map[string]v1alpha1.ManagedResourceDefinitionState{
						"bucket.aws.crossplane.io": v1alpha1.ManagedResourceDefinitionActive,
					}),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.Spec.Activations = []v1alpha1.ActivationPolicy{"*.aws.crossplane.io"}
						mrap.Status.Activated = []string{"bucket.aws.crossplane.io"}
						mrap.SetConditions(v1alpha1.Healthy())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ActivateMultipleMRDs": {
			reason: "We should activate multiple MRDs that match the activation policy.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPActivations("*.aws.crossplane.io"))),
					MockList: WithMRDList(t,
						NewMRD("bucket.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
						NewMRD("instance.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
						NewMRD("database.gcp.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
					),
					MockPatch: WantMRDPatch(t, map[string]v1alpha1.ManagedResourceDefinitionState{
						"bucket.aws.crossplane.io":   v1alpha1.ManagedResourceDefinitionActive,
						"instance.aws.crossplane.io": v1alpha1.ManagedResourceDefinitionActive,
					}),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.Spec.Activations = []v1alpha1.ActivationPolicy{"*.aws.crossplane.io"}
						mrap.Status.Activated = []string{"bucket.aws.crossplane.io", "instance.aws.crossplane.io"}
						mrap.SetConditions(v1alpha1.Healthy())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"SkipAlreadyActiveMRD": {
			reason: "We should skip MRDs that are already active and include them in status.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPActivations("*.aws.crossplane.io"))),
					MockList: WithMRDList(t,
						NewMRD("bucket.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionActive)),
						NewMRD("instance.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
					),
					MockPatch: WantMRDPatch(t, map[string]v1alpha1.ManagedResourceDefinitionState{
						"instance.aws.crossplane.io": v1alpha1.ManagedResourceDefinitionActive,
					}),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.Spec.Activations = []v1alpha1.ActivationPolicy{"*.aws.crossplane.io"}
						mrap.Status.Activated = []string{"bucket.aws.crossplane.io", "instance.aws.crossplane.io"}
						mrap.SetConditions(v1alpha1.Healthy())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"PartialActivationFailure": {
			reason: "We should set unhealthy status when some MRDs fail to activate.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPActivations("*.aws.crossplane.io"))),
					MockList: WithMRDList(t,
						NewMRD("bucket.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
						NewMRD("instance.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
					),
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if mrd, ok := obj.(*v1alpha1.ManagedResourceDefinition); ok {
							if mrd.GetName() == "bucket.aws.crossplane.io" {
								return errBoom
							}
						}
						return nil
					},
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.Spec.Activations = []v1alpha1.ActivationPolicy{"*.aws.crossplane.io"}
						mrap.Status.Activated = []string{"instance.aws.crossplane.io"}
						mrap.SetConditions(v1alpha1.Unhealthy().WithMessage("failed to activate 1 of 1 ManagedResourceDefinitions"))
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MultipleActivationPolicies": {
			reason: "We should activate MRDs matching any of multiple activation policies.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPActivations("*.aws.crossplane.io", "*.gcp.crossplane.io"))),
					MockList: WithMRDList(t,
						NewMRD("bucket.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
						NewMRD("storage.gcp.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
						NewMRD("database.azure.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
					),
					MockPatch: WantMRDPatch(t, map[string]v1alpha1.ManagedResourceDefinitionState{
						"bucket.aws.crossplane.io":  v1alpha1.ManagedResourceDefinitionActive,
						"storage.gcp.crossplane.io": v1alpha1.ManagedResourceDefinitionActive,
					}),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.Spec.Activations = []v1alpha1.ActivationPolicy{"*.aws.crossplane.io", "*.gcp.crossplane.io"}
						mrap.Status.Activated = []string{"bucket.aws.crossplane.io", "storage.gcp.crossplane.io"}
						mrap.SetConditions(v1alpha1.Healthy())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"EmptyActivationPolicies": {
			reason: "We should succeed with empty status when no activation policies are defined.",
			args: args{
				c: &test.MockClient{
					MockGet: WithMRAP(t, NewMRAP(WithMRAPActivations())),
					MockList: WithMRDList(t,
						NewMRD("bucket.aws.crossplane.io", WithMRDState(v1alpha1.ManagedResourceDefinitionInactive)),
					),
					MockStatusUpdate: WantMRAP(t, NewMRAP(func(mrap *v1alpha1.ManagedResourceActivationPolicy) {
						mrap.Spec.Activations = []v1alpha1.ActivationPolicy{}
						mrap.Status.Activated = nil
						mrap.SetConditions(v1alpha1.Healthy())
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"StatusUpdateError": {
			reason: "We should return error when final status update fails.",
			args: args{
				c: &test.MockClient{
					MockGet:          WithMRAP(t, NewMRAP(WithMRAPActivations("*.aws.crossplane.io"))),
					MockList:         WithMRDList(t),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateStatus),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Reconciler{
				Client:     tc.args.c,
				log:        logging.NewNopLogger(),
				record:     event.NewNopRecorder(),
				conditions: conditions.ObservedGenerationPropagationManager{},
			}

			got, err := r.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-mrap"},
			})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

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

package cronoperation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

// EquateApproxDuration returns a cmp.Option that considers two time.Duration values
// to be equal if they are within the given tolerance of each other. This is useful
// for testing timing-sensitive operations where minor differences in execution time
// can cause test failures.
func EquateApproxDuration(tolerance time.Duration) cmp.Option {
	return cmp.Comparer(func(x, y time.Duration) bool {
		diff := x - y
		if diff < 0 {
			diff = -diff
		}
		return diff <= tolerance
	})
}

func TestReconcile(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	type params struct {
		client client.Client
		opts   []ReconcilerOption
	}

	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		want   want
	}{
		"NotFound": {
			reason: "We should return early if the CronOperation was not found.",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetError": {
			reason: "We should return an error if we can't get the CronOperation",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errors.New("boom")),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"Deleted": {
			reason: "We should return early if the CronOperation was deleted.",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								DeletionTimestamp: new(metav1.Now()),
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"Paused": {
			reason: "We should return early if the CronOperation is paused.",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"crossplane.io/paused": "true",
								},
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ListOperationsError": {
			reason: "We should return an error if we can't list Operations",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-cron",
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule: "0 * * * *",
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList:         test.NewMockListFn(errors.New("boom")),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"InvalidSchedule": {
			reason: "We should return an error if the cron schedule is invalid",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-cron",
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule: "invalid-schedule",
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList:         test.NewMockListFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errors.New("status update failed")),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, _ time.Time) (time.Time, error) {
						return time.Time{}, errors.New("invalid schedule")
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"NextScheduledInFuture": {
			reason: "We should requeue when the next scheduled operation is in the future",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-cron",
								CreationTimestamp: metav1.Time{Time: past},
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule: "0 * * * *",
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList:         test.NewMockListFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, _ time.Time) (time.Time, error) {
						return future, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: time.Hour},
			},
		},
		"MissedDeadline": {
			reason: "We should requeue for future when we missed the deadline",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-cron",
								CreationTimestamp: metav1.Time{Time: past},
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule:                "0 * * * *",
								StartingDeadlineSeconds: new(int64(60)), // 1 minute deadline
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList:         test.NewMockListFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, last time.Time) (time.Time, error) {
						if last.Before(past) {
							// First call: return a time that's past deadline
							return past.Add(-2 * time.Hour), nil
						}
						// Second call: return future time
						return future, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: time.Hour},
			},
		},
		"ConcurrencyPolicyForbid": {
			reason: "We should requeue for future when concurrency policy forbids and operations are running",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-cron",
								CreationTimestamp: metav1.Time{Time: past},
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule:          "0 * * * *",
								ConcurrencyPolicy: ptr.To(v1alpha1.ConcurrencyPolicyForbid),
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.OperationList)
						// Add a running operation
						list.Items = []v1alpha1.Operation{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "running-op",
								},
								Status: v1alpha1.OperationStatus{
									ConditionedStatus: xpv2.ConditionedStatus{
										Conditions: []xpv2.Condition{
											{
												Type:   v1alpha1.TypeSucceeded,
												Status: "Unknown",
												Reason: v1alpha1.ReasonPipelineRunning,
											},
										},
									},
								},
							},
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					MockDelete:       test.NewMockDeleteFn(nil),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, last time.Time) (time.Time, error) {
						if last.Before(past) {
							// First call: return a time that's due now
							return past.Add(-30 * time.Minute), nil
						}
						// Second call: return future time
						return future, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: time.Hour},
			},
		},
		"ConcurrencyPolicyReplaceDeleteError": {
			reason: "We should return an error if we can't delete running operations for replace policy",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-cron",
								CreationTimestamp: metav1.Time{Time: past},
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule:          "0 * * * *",
								ConcurrencyPolicy: ptr.To(v1alpha1.ConcurrencyPolicyReplace),
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.OperationList)
						// Add a running operation
						list.Items = []v1alpha1.Operation{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "running-op",
								},
								Status: v1alpha1.OperationStatus{
									ConditionedStatus: xpv2.ConditionedStatus{
										Conditions: []xpv2.Condition{
											{
												Type:   v1alpha1.TypeSucceeded,
												Status: "Unknown",
												Reason: v1alpha1.ReasonPipelineRunning,
											},
										},
									},
								},
							},
						}
						return nil
					}),
					MockDelete:       test.NewMockDeleteFn(errors.New("boom")),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, _ time.Time) (time.Time, error) {
						// Return a time that's due now
						return past.Add(-30 * time.Minute), nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"CreateOperationError": {
			reason: "We should return an error if we can't create the scheduled Operation",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-cron",
								CreationTimestamp: metav1.Time{Time: past},
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule: "0 * * * *",
								OperationTemplate: v1alpha1.OperationTemplate{
									Spec: v1alpha1.OperationSpec{
										Mode: v1alpha1.OperationModePipeline,
										Pipeline: []v1alpha1.PipelineStep{
											{
												Step: "test-step",
												FunctionRef: v1alpha1.FunctionReference{
													Name: "test-function",
												},
											},
										},
									},
								},
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList:         test.NewMockListFn(nil),
					MockCreate:       test.NewMockCreateFn(errors.New("boom")),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, _ time.Time) (time.Time, error) {
						// Return a time that's due now
						return past.Add(-30 * time.Minute), nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "We should successfully create an operation and update status",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-cron",
								UID:               types.UID("test-uid"),
								CreationTimestamp: metav1.Time{Time: past},
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule: "0 * * * *",
								OperationTemplate: v1alpha1.OperationTemplate{
									Spec: v1alpha1.OperationSpec{
										Mode: v1alpha1.OperationModePipeline,
										Pipeline: []v1alpha1.PipelineStep{
											{
												Step: "test-step",
												FunctionRef: v1alpha1.FunctionReference{
													Name: "test-function",
												},
											},
										},
									},
								},
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList: test.NewMockListFn(nil),
					MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
						// Verify the created operation has the right properties
						op := obj.(*v1alpha1.Operation)
						if op.Name != "test-cron--2147483648" { // Unix timestamp will be negative due to past time
							t.Errorf("Expected operation name to contain cron name, got: %s", op.Name)
						}
						if op.Labels[v1alpha1.LabelCronOperationName] != "test-cron" {
							t.Errorf("Expected operation to have cron operation label")
						}
						if len(op.OwnerReferences) != 1 || op.OwnerReferences[0].Name != "test-cron" {
							t.Errorf("Expected operation to have owner reference to cron operation")
						}
						return nil
					}),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, last time.Time) (time.Time, error) {
						if last.Before(past) {
							// First call: return a time that's due now
							return past.Add(-30 * time.Minute), nil
						}
						// Second call: return future time
						return future, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: time.Hour},
			},
		},
		"GarbageCollectionError": {
			reason: "We should return an error if we can't garbage collect old operations",
			params: params{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						co := &v1alpha1.CronOperation{
							ObjectMeta: metav1.ObjectMeta{
								Name:              "test-cron",
								CreationTimestamp: metav1.Time{Time: past},
							},
							Spec: v1alpha1.CronOperationSpec{
								Schedule:               "0 * * * *",
								SuccessfulHistoryLimit: new(int32(1)),
								FailedHistoryLimit:     new(int32(1)),
							},
						}
						co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
						return nil
					}),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						list := obj.(*v1alpha1.OperationList)
						// Add old operations that should be garbage collected
						list.Items = []v1alpha1.Operation{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:              "old-op",
									CreationTimestamp: metav1.Time{Time: past.Add(-2 * time.Hour)},
								},
								Status: v1alpha1.OperationStatus{
									ConditionedStatus: xpv2.ConditionedStatus{
										Conditions: []xpv2.Condition{
											{
												Type:   v1alpha1.TypeSucceeded,
												Status: "True",
												Reason: v1alpha1.ReasonPipelineSuccess,
											},
										},
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:              "newer-op",
									CreationTimestamp: metav1.Time{Time: past.Add(-1 * time.Hour)},
								},
								Status: v1alpha1.OperationStatus{
									ConditionedStatus: xpv2.ConditionedStatus{
										Conditions: []xpv2.Condition{
											{
												Type:   v1alpha1.TypeSucceeded,
												Status: "True",
												Reason: v1alpha1.ReasonPipelineSuccess,
											},
										},
									},
								},
							},
						}
						return nil
					}),
					MockDelete:       test.NewMockDeleteFn(errors.New("boom")),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithScheduler(SchedulerFn(func(_ string, _ time.Time) (time.Time, error) {
						return future, nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.params.client, tc.params.opts...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "test-cron"},
			})

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.r, got, EquateApproxDuration(1*time.Second)); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want result, +got result:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestReconcileClockSkewCollision reproduces
// https://github.com/crossplane/crossplane/issues/7524.
//
// It simulates an Operation that was created for a scheduled slot, but
// whose K8s creationTimestamp was stamped by the API server ~1s before that
// slot's boundary (clock skew between the controller and the API server).
//
// Before the fix, LastScheduleTime is derived from the skewed
// creationTimestamp, so Next(schedule, creationTimestamp) recomputes the
// *same* scheduled slot on every subsequent reconcile, and the controller
// tries - forever - to re-create an Operation that already exists.
//
// After the fix, LastScheduleTime is derived from the scheduled time
// encoded in the Operation's own name, which is not affected by clock
// skew, so the controller correctly advances to the next slot instead of
// colliding with the existing Operation.
func TestReconcileClockSkewCollision(t *testing.T) {
	now := time.Now().UTC()

	// A real "*/5 * * * *" tick, comfortably in the past so it's due.
	boundary := now.Truncate(5 * time.Minute)
	scheduled := boundary.Add(-10 * time.Minute)

	// The API server stamped creationTimestamp 1s *before* the scheduled
	// boundary - the clock skew described in the issue.
	skewedCreation := scheduled.Add(-1 * time.Second)

	// The CronOperation itself is much older, so its own creation
	// timestamp is never the deciding factor here.
	coCreation := scheduled.Add(-24 * time.Hour)

	existingOpName := fmt.Sprintf("test-cron-%d", scheduled.Unix())

	c := &test.MockClient{
		MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
			co := &v1alpha1.CronOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cron",
					CreationTimestamp: metav1.Time{Time: coCreation},
				},
				Spec: v1alpha1.CronOperationSpec{
					Schedule: "*/5 * * * *",
					OperationTemplate: v1alpha1.OperationTemplate{
						Spec: v1alpha1.OperationSpec{
							Mode: v1alpha1.OperationModePipeline,
							Pipeline: []v1alpha1.PipelineStep{
								{
									Step:        "test-step",
									FunctionRef: v1alpha1.FunctionReference{Name: "test-function"},
								},
							},
						},
					},
				},
			}
			co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
			return nil
		}),
		MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
			list := obj.(*v1alpha1.OperationList)
			list.Items = []v1alpha1.Operation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              existingOpName,
						CreationTimestamp: metav1.Time{Time: skewedCreation},
					},
					Status: v1alpha1.OperationStatus{
						ConditionedStatus: xpv2.ConditionedStatus{
							Conditions: []xpv2.Condition{
								{
									Type:   v1alpha1.TypeSucceeded,
									Status: "True",
									Reason: v1alpha1.ReasonPipelineSuccess,
								},
							},
						},
					},
				},
			}
			return nil
		}),
		MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
			op := obj.(*v1alpha1.Operation)
			if op.GetName() == existingOpName {
				// The exact race from the issue: the controller tries to
				// recreate the Operation that already occupies this slot.
				return kerrors.NewAlreadyExists(schema.GroupResource{Resource: "operations"}, op.GetName())
			}
			return nil
		}),
		MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
	}

	// Deliberately use the real cron scheduler (the Reconciler's default),
	// not a mocked one, so we exercise the actual Next() computation the
	// issue describes.
	r := NewReconciler(c)

	got, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-cron"},
	})
	if err != nil {
		t.Errorf("r.Reconcile(...): unexpected error - this is the infinite AlreadyExists loop from issue #7524: %v", err)
	}

	if got.RequeueAfter <= 0 {
		t.Errorf("r.Reconcile(...): got RequeueAfter = %v, want a positive requeue interval for the next tick", got.RequeueAfter)
	}
}

// TestReconcileCreateAlreadyExistsIsTreatedAsSuccess exercises the
// defensive half of the fix for issue #7524: even independent of the
// clock-skew root cause, if Create() reports AlreadyExists for the exact
// Operation name we just computed, that's not a real error - an Operation
// for this schedule slot already exists (e.g. because our informer cache
// hasn't yet observed an Operation created by an earlier reconcile). The
// controller should treat that as success rather than erroring out and
// requeuing forever.
func TestReconcileCreateAlreadyExistsIsTreatedAsSuccess(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	due := now.Add(-time.Minute)

	calls := 0
	c := &test.MockClient{
		MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
			co := &v1alpha1.CronOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cron",
					CreationTimestamp: metav1.Time{Time: past},
				},
				Spec: v1alpha1.CronOperationSpec{
					Schedule: "0 * * * *",
				},
			}
			co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
			return nil
		}),
		// The Operation that already occupies this slot isn't visible in
		// our list yet (e.g. informer cache lag after a partial previous
		// reconcile).
		MockList: test.NewMockListFn(nil),
		MockCreate: test.NewMockCreateFn(nil, func(obj client.Object) error {
			return kerrors.NewAlreadyExists(schema.GroupResource{Resource: "operations"}, obj.(*v1alpha1.Operation).GetName())
		}),
		MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
	}

	r := NewReconciler(c, WithScheduler(SchedulerFn(func(_ string, _ time.Time) (time.Time, error) {
		calls++
		if calls == 1 {
			// First call determines "next" - due now.
			return due, nil
		}
		// Second call determines "future".
		return future, nil
	})))

	got, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-cron"},
	})
	if err != nil {
		t.Errorf("r.Reconcile(...): unexpected error treating Create's AlreadyExists as success: %v", err)
	}

	want := reconcile.Result{RequeueAfter: future.Sub(now)}
	if diff := cmp.Diff(want, got, EquateApproxDuration(time.Second)); diff != "" {
		t.Errorf("r.Reconcile(...): -want, +got:\n%s", diff)
	}
}

func TestNewOperation(t *testing.T) {
	scheduled := time.Unix(1609459200, 0) // 2021-01-01 00:00:00 UTC

	type args struct {
		co        *v1alpha1.CronOperation
		scheduled time.Time
	}
	type want struct {
		op *v1alpha1.Operation
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should create operation with correct metadata and owner reference",
			args: args{
				co: &v1alpha1.CronOperation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cron",
						UID:  types.UID("test-uid"),
					},
					Spec: v1alpha1.CronOperationSpec{
						OperationTemplate: v1alpha1.OperationTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"template": "label",
								},
							},
							Spec: v1alpha1.OperationSpec{
								Mode: v1alpha1.OperationModePipeline,
								Pipeline: []v1alpha1.PipelineStep{
									{
										Step: "test-step",
										FunctionRef: v1alpha1.FunctionReference{
											Name: "test-function",
										},
									},
								},
							},
						},
					},
				},
				scheduled: scheduled,
			},
			want: want{
				op: &v1alpha1.Operation{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cron-1609459200",
						Labels: map[string]string{
							"template":                      "label",
							v1alpha1.LabelCronOperationName: "test-cron",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "ops.crossplane.io/v1alpha1",
								Kind:               "CronOperation",
								Name:               "test-cron",
								UID:                types.UID("test-uid"),
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
						},
					},
					Spec: v1alpha1.OperationSpec{
						Mode: v1alpha1.OperationModePipeline,
						Pipeline: []v1alpha1.PipelineStep{
							{
								Step: "test-step",
								FunctionRef: v1alpha1.FunctionReference{
									Name: "test-function",
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewOperation(tc.args.co, tc.args.scheduled)
			if diff := cmp.Diff(tc.want.op, got); diff != "" {
				t.Errorf("\n%s\nNewOperation(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

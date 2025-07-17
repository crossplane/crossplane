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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
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
		mgr  manager.Manager
		opts []ReconcilerOption
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetError": {
			reason: "We should return an error if we can't get the CronOperation",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errors.New("boom")),
					},
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							co := &v1alpha1.CronOperation{
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: ptr.To(metav1.Now()),
								},
							}
							co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
							return nil
						}),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"Suspended": {
			reason: "We should return early if the CronOperation is suspended.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							co := &v1alpha1.CronOperation{
								Spec: v1alpha1.CronOperationSpec{
									Suspend: ptr.To(true),
								},
							}
							co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
							return nil
						}),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ListOperationsError": {
			reason: "We should return an error if we can't list Operations",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
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
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"InvalidSchedule": {
			reason: "We should return an error if the cron schedule is invalid",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							co := &v1alpha1.CronOperation{
								ObjectMeta: metav1.ObjectMeta{
									Name:              "test-cron",
									CreationTimestamp: metav1.Time{Time: past},
								},
								Spec: v1alpha1.CronOperationSpec{
									Schedule:                "0 * * * *",
									StartingDeadlineSeconds: ptr.To(int64(60)), // 1 minute deadline
								},
							}
							co.DeepCopyInto(obj.(*v1alpha1.CronOperation))
							return nil
						}),
						MockList:         test.NewMockListFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
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
										ConditionedStatus: xpv1.ConditionedStatus{
											Conditions: []xpv1.Condition{
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
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
										ConditionedStatus: xpv1.ConditionedStatus{
											Conditions: []xpv1.Condition{
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
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
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							co := &v1alpha1.CronOperation{
								ObjectMeta: metav1.ObjectMeta{
									Name:              "test-cron",
									CreationTimestamp: metav1.Time{Time: past},
								},
								Spec: v1alpha1.CronOperationSpec{
									Schedule:               "0 * * * *",
									SuccessfulHistoryLimit: ptr.To(int32(1)),
									FailedHistoryLimit:     ptr.To(int32(1)),
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
										ConditionedStatus: xpv1.ConditionedStatus{
											Conditions: []xpv1.Condition{
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
										ConditionedStatus: xpv1.ConditionedStatus{
											Conditions: []xpv1.Condition{
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
			r := NewReconciler(tc.params.mgr, tc.params.opts...)

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
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
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

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

package lifecycle

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	"github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
)

func TestLatestCreateTime(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	later := now.Add(time.Hour)

	type args struct {
		ops []v1alpha1.Operation
	}
	type want struct {
		latest time.Time
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptySlice": {
			reason: "Should return zero time for empty slice",
			args: args{
				ops: []v1alpha1.Operation{},
			},
			want: want{
				latest: time.Time{},
			},
		},
		"SingleOperation": {
			reason: "Should return the single operation's creation time",
			args: args{
				ops: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Time{Time: now},
						},
					},
				},
			},
			want: want{
				latest: now,
			},
		},
		"MultipleOperations": {
			reason: "Should return the latest creation time from multiple operations",
			args: args{
				ops: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Time{Time: earlier},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Time{Time: later},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Time{Time: now},
						},
					},
				},
			},
			want: want{
				latest: later,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := LatestCreateTime(tc.args.ops...)
			if diff := cmp.Diff(tc.want.latest, got); diff != "" {
				t.Errorf("\n%s\nLatestCreateTime(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLatestSucceededTransitionTime(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	later := now.Add(time.Hour)

	type args struct {
		ops []v1alpha1.Operation
	}
	type want struct {
		latest time.Time
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptySlice": {
			reason: "Should return zero time for empty slice",
			args: args{
				ops: []v1alpha1.Operation{},
			},
			want: want{
				latest: time.Time{},
			},
		},
		"SingleOperation": {
			reason: "Should return the single operation's succeeded transition time",
			args: args{
				ops: []v1alpha1.Operation{
					{
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:               v1alpha1.TypeSucceeded,
										LastTransitionTime: metav1.Time{Time: now},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				latest: now,
			},
		},
		"MultipleOperations": {
			reason: "Should return the latest succeeded transition time from multiple operations",
			args: args{
				ops: []v1alpha1.Operation{
					{
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:               v1alpha1.TypeSucceeded,
										LastTransitionTime: metav1.Time{Time: earlier},
									},
								},
							},
						},
					},
					{
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:               v1alpha1.TypeSucceeded,
										LastTransitionTime: metav1.Time{Time: later},
									},
								},
							},
						},
					},
					{
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:               v1alpha1.TypeSucceeded,
										LastTransitionTime: metav1.Time{Time: now},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				latest: later,
			},
		},
		"NoSucceededConditions": {
			reason: "Should return zero time when no operations have succeeded conditions",
			args: args{
				ops: []v1alpha1.Operation{
					{
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:               "Other",
										LastTransitionTime: metav1.Time{Time: now},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				latest: time.Time{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := LatestSucceededTransitionTime(tc.args.ops...)
			if diff := cmp.Diff(tc.want.latest, got); diff != "" {
				t.Errorf("\n%s\nLatestSucceededTransitionTime(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithReason(t *testing.T) {
	type args struct {
		reason xpv1.ConditionReason
		ops    []v1alpha1.Operation
	}
	type want struct {
		filtered []v1alpha1.Operation
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptySlice": {
			reason: "Should return empty slice for empty input",
			args: args{
				reason: v1alpha1.ReasonPipelineSuccess,
				ops:    []v1alpha1.Operation{},
			},
			want: want{
				filtered: []v1alpha1.Operation{},
			},
		},
		"NoMatches": {
			reason: "Should return empty slice when no operations match the reason",
			args: args{
				reason: v1alpha1.ReasonPipelineSuccess,
				ops: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op1"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Reason: v1alpha1.ReasonPipelineRunning,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				filtered: []v1alpha1.Operation{},
			},
		},
		"SomeMatches": {
			reason: "Should return only operations that match the specified reason",
			args: args{
				reason: v1alpha1.ReasonPipelineSuccess,
				ops: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op1"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Reason: v1alpha1.ReasonPipelineSuccess,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op2"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Reason: v1alpha1.ReasonPipelineRunning,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op3"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Reason: v1alpha1.ReasonPipelineSuccess,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				filtered: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op1"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Reason: v1alpha1.ReasonPipelineSuccess,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op3"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Reason: v1alpha1.ReasonPipelineSuccess,
									},
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
			got := WithReason(tc.args.reason, tc.args.ops...)
			if diff := cmp.Diff(tc.want.filtered, got); diff != "" {
				t.Errorf("\n%s\nWithReason(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMarkGarbage(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	latest := now.Add(time.Hour)

	type args struct {
		keepSucceeded int32
		keepFailed    int32
		ops           []v1alpha1.Operation
	}
	type want struct {
		del []v1alpha1.Operation
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ZeroLimits": {
			reason: "Should delete all operations when both limits are zero",
			args: args{
				keepSucceeded: 0,
				keepFailed:    0,
				ops: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op1"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op2"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				del: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op1"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "op2"},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
		},
		"KeepSucceededOnly": {
			reason: "Should delete excess succeeded operations beyond the specified limit",
			args: args{
				keepSucceeded: 1,
				keepFailed:    0,
				ops: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op1",
							CreationTimestamp: metav1.Time{Time: latest},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op2",
							CreationTimestamp: metav1.Time{Time: earlier},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op3",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				del: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op2",
							CreationTimestamp: metav1.Time{Time: earlier},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op3",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
		},
		"KeepFailedOnly": {
			reason: "Should delete excess failed operations beyond the specified limit",
			args: args{
				keepSucceeded: 0,
				keepFailed:    1,
				ops: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op1",
							CreationTimestamp: metav1.Time{Time: latest},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op2",
							CreationTimestamp: metav1.Time{Time: earlier},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				del: []v1alpha1.Operation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "op2",
							CreationTimestamp: metav1.Time{Time: earlier},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				},
			},
		},
		"MixedScenario": {
			reason: "Should delete excess succeeded operations beyond the specified limit",
			args: args{
				keepSucceeded: 2,
				keepFailed:    0,
				ops: []v1alpha1.Operation{
					// 3 succeeded operations (latest first after sorting)
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "succeeded1",
							CreationTimestamp: metav1.Time{Time: latest},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "succeeded2",
							CreationTimestamp: metav1.Time{Time: now},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "succeeded3",
							CreationTimestamp: metav1.Time{Time: earlier},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
					// 1 failed operation (should be deleted - keepFailed=0)
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "failed1",
							CreationTimestamp: metav1.Time{Time: now.Add(-30 * time.Minute)},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
					// 1 running operation (should be kept)
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "running1",
							CreationTimestamp: metav1.Time{Time: now.Add(-15 * time.Minute)},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionUnknown,
									},
								},
							},
						},
					},
					// 1 operation with no condition (should be kept)
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "no-condition",
							CreationTimestamp: metav1.Time{Time: now.Add(-45 * time.Minute)},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{},
							},
						},
					},
				},
			},
			want: want{
				del: []v1alpha1.Operation{
					// Failed operation (keepFailed=0 means delete all failed)
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "failed1",
							CreationTimestamp: metav1.Time{Time: now.Add(-30 * time.Minute)},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
					// Oldest succeeded operation (beyond the 2 we keep)
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "succeeded3",
							CreationTimestamp: metav1.Time{Time: earlier},
						},
						Status: v1alpha1.OperationStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									{
										Type:   v1alpha1.TypeSucceeded,
										Status: corev1.ConditionTrue,
									},
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
			del := MarkGarbage(tc.args.keepSucceeded, tc.args.keepFailed, tc.args.ops...)
			if diff := cmp.Diff(tc.want.del, del, cmpopts.SortSlices(func(a, b v1alpha1.Operation) bool {
				return a.Name < b.Name
			})); diff != "" {
				t.Errorf("\n%s\nMarkGarbage(...) del: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunningOperationRefs(t *testing.T) {
	type args struct {
		running []string
	}
	type want struct {
		refs []v1alpha1.RunningOperationRef
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Empty": {
			reason: "Should return empty slice for empty input",
			args: args{
				running: []string{},
			},
			want: want{
				refs: []v1alpha1.RunningOperationRef{},
			},
		},
		"Single": {
			reason: "Should convert single name to RunningOperationRef",
			args: args{
				running: []string{"op1"},
			},
			want: want{
				refs: []v1alpha1.RunningOperationRef{
					{Name: "op1"},
				},
			},
		},
		"Multiple": {
			reason: "Should convert multiple names to RunningOperationRefs",
			args: args{
				running: []string{"op1", "op2", "op3"},
			},
			want: want{
				refs: []v1alpha1.RunningOperationRef{
					{Name: "op1"},
					{Name: "op2"},
					{Name: "op3"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RunningOperationRefs(tc.args.running)
			if diff := cmp.Diff(tc.want.refs, got); diff != "" {
				t.Errorf("\n%s\nRunningOperationRefs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

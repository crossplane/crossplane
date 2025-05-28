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

package operation

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	"github.com/crossplane/crossplane/apis/daytwo/v1alpha1"
	"github.com/crossplane/crossplane/internal/xfn"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))

	opPending := &v1alpha1.Operation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-operation",
			UID:  types.UID("test-uid"),
		},
		Spec: v1alpha1.OperationSpec{
			Pipeline: []v1alpha1.PipelineStep{
				{
					Step: "test-step",
					FunctionRef: v1alpha1.FunctionReference{
						Name: "test-function",
					},
				},
			},
		},
	}

	opComplete := &v1alpha1.Operation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "completed-operation",
			UID:  types.UID("completed-uid"),
		},
		Spec: v1alpha1.OperationSpec{
			Pipeline: []v1alpha1.PipelineStep{
				{
					Step: "test-step",
					FunctionRef: v1alpha1.FunctionReference{
						Name: "test-function",
					},
				},
			},
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
	}

	opWithFailures := &v1alpha1.Operation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "failed-operation",
			UID:  types.UID("failed-uid"),
		},
		Spec: v1alpha1.OperationSpec{
			FailureLimit: ptr.To(int64(2)),
			Pipeline: []v1alpha1.PipelineStep{
				{
					Step: "test-step",
					FunctionRef: v1alpha1.FunctionReference{
						Name: "test-function",
					},
				},
			},
		},
		Status: v1alpha1.OperationStatus{
			Failures: 2,
		},
	}

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
		"OperationNotFound": {
			reason: "We should not return an error if the Operation was not found.",
			args: args{
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
		"GetOperationError": {
			reason: "We should return any other error encountered while getting an Operation.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, "cannot get Operation"),
			},
		},
		"OperationDeleted": {
			reason: "We should return without error if the Operation exists but is being deleted.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							now := metav1.Now()
							*obj.(*v1alpha1.Operation) = v1alpha1.Operation{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}}
							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{Requeue: false},
				err: nil,
			},
		},
		"OperationAlreadyComplete": {
			reason: "We should not reconcile an Operation that is already complete.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1alpha1.Operation) = *opComplete
							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{Requeue: false},
				err: nil,
			},
		},
		"OperationFailureLimitReached": {
			reason: "We should not retry an Operation that has reached its failure limit.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1alpha1.Operation) = *opWithFailures
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
							op := obj.(*v1alpha1.Operation)
							// Verify the operation is marked complete and failed
							if !op.IsComplete() && op.Status.GetCondition(v1alpha1.TypeSucceeded).Status == corev1.ConditionFalse {
								t.Errorf("Expected operation to be marked complete")
							}
							return nil
						}),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
		"UpdateStatusError": {
			reason: "We should return an error if we cannot update the Operation status.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1alpha1.Operation) = *opPending
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, "cannot update Operation status"),
			},
		},
		"SuccessfulExecution": {
			reason: "We should successfully execute a simple operation pipeline.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1alpha1.Operation) = *opPending
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithFunctionRunner(xfn.FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
						return &fnv1.RunFunctionResponse{}, nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
			},
		},
		"SuccessfulExecutionWithOutput": {
			reason: "We should successfully execute a simple operation pipeline.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*v1alpha1.Operation) = *opPending
							return nil
						}),
						MockStatusUpdate: func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							op, ok := obj.(*v1alpha1.Operation)
							// look for the op to be finished.
							if ok && op.IsComplete() {
								if len(op.Status.Pipeline) != 1 {
									t.Errorf("Expected 1 pipeline, got %d", len(op.Status.Pipeline))
								}
								p := op.Status.Pipeline[0]
								if p.Step != "test-step" {
									t.Errorf("Expected step test-function, got %s", p.Step)
								}
								j, err := p.Output.MarshalJSON()
								if err != nil {
									t.Errorf("Failed to marshal output: %v", err)
								}
								if want := `{"hello":"test-function"}`; string(j) != want {
									t.Errorf("Expected output to be %s, got %s", want, string(j))
								}
							}
							return nil
						},
					},
				},
				opts: []ReconcilerOption{
					WithFunctionRunner(xfn.FunctionRunnerFn(func(_ context.Context, name string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
						return &fnv1.RunFunctionResponse{
							Output: func() *structpb.Struct {
								s, _ := structpb.NewStruct(map[string]interface{}{
									"hello": name,
								})
								return s
							}(),
						}, nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: nil,
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

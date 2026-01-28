/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package step

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	pipelinev1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/pipelineinspector/proto/v1alpha1"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func TestContextWithStepMeta(t *testing.T) {
	type args struct {
		ctx             context.Context
		TraceID         string
		compositionName string
		stepIndex       int32
		iteration       int32
	}

	cases := map[string]struct {
		reason string
		args   args
		want   struct {
			TraceID         string
			compositionName string
			stepIndex       int32
			iteration       int32
		}
	}{
		"StoresAllValues": {
			reason: "Should store all values in context.",
			args: args{
				ctx:             context.Background(),
				TraceID:         "trace-123",
				compositionName: "my-composition",
				stepIndex:       2,
				iteration:       3,
			},
			want: struct {
				TraceID         string
				compositionName string
				stepIndex       int32
				iteration       int32
			}{
				TraceID:         "trace-123",
				compositionName: "my-composition",
				stepIndex:       2,
				iteration:       3,
			},
		},
		"HandlesNilContext": {
			reason: "Should create background context when nil is passed.",
			args: args{
				ctx:             nil,
				TraceID:         "trace-456",
				compositionName: "other-composition",
				stepIndex:       0,
				iteration:       0,
			},
			want: struct {
				TraceID         string
				compositionName string
				stepIndex       int32
				iteration       int32
			}{
				TraceID:         "trace-456",
				compositionName: "other-composition",
				stepIndex:       0,
				iteration:       0,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := ContextWithStepMeta(tc.args.ctx, tc.args.TraceID, tc.args.compositionName, tc.args.stepIndex, tc.args.iteration)

			if ctx == nil {
				t.Fatal("expected non-nil context")
			}

			if got := ctx.Value(ContextKeyTraceID); got != tc.want.TraceID {
				t.Errorf("\n%s\nContextWithStepMeta(...) TraceId: want %q, got %q", tc.reason, tc.want.TraceID, got)
			}
			if got := ctx.Value(ContextKeyCompositionName); got != tc.want.compositionName {
				t.Errorf("\n%s\nContextWithStepMeta(...) CompositionName: want %q, got %q", tc.reason, tc.want.compositionName, got)
			}
			if got := ctx.Value(ContextKeyStepIndex); got != tc.want.stepIndex {
				t.Errorf("\n%s\nContextWithStepMeta(...) StepIndex: want %d, got %d", tc.reason, tc.want.stepIndex, got)
			}
			if got := ctx.Value(ContextKeyIteration); got != tc.want.iteration {
				t.Errorf("\n%s\nContextWithStepMeta(...) Iteration: want %d, got %d", tc.reason, tc.want.iteration, got)
			}
		})
	}
}

func TestContextWithStepIteration(t *testing.T) {
	cases := map[string]struct {
		reason    string
		ctx       context.Context
		iteration int32
	}{
		"UpdatesIteration": {
			reason:    "Should update iteration in existing context.",
			ctx:       ContextWithStepMeta(context.Background(), "trace", "comp", 0, 0),
			iteration: 5,
		},
		"HandlesNilContext": {
			reason:    "Should create background context when nil is passed.",
			ctx:       nil,
			iteration: 3,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := ContextWithStepIteration(tc.ctx, tc.iteration)

			if ctx == nil {
				t.Fatal("expected non-nil context")
			}

			if got := ctx.Value(ContextKeyIteration); got != tc.iteration {
				t.Errorf("\n%s\nContextWithStepIteration(...): want %d, got %d", tc.reason, tc.iteration, got)
			}
		})
	}
}

func TestBuildMetadata(t *testing.T) {
	type args struct {
		ctx          context.Context
		functionName string
		req          *fnv1.RunFunctionRequest
	}

	type want struct {
		meta *pipelinev1alpha1.StepMeta
		err  error
	}

	validCtx := ContextWithStepMeta(context.Background(), "trace-abc", "my-composition", 2, 5)

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulBuild": {
			reason: "Should build metadata from valid context and request.",
			args: args{
				ctx:          validCtx,
				functionName: "function-auto-ready",
				req: &fnv1.RunFunctionRequest{
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("example.org/v1"),
									"kind":       structpb.NewStringValue("XDatabase"),
									"metadata": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"name":      structpb.NewStringValue("my-db"),
											"namespace": structpb.NewStringValue("default"),
											"uid":       structpb.NewStringValue("uid-123"),
										},
									}),
								},
							},
						},
					},
				},
			},
			want: want{
				meta: &pipelinev1alpha1.StepMeta{
					TraceId:                     "trace-abc",
					StepIndex:                   2,
					Iteration:                   5,
					FunctionName:                "function-auto-ready",
					CompositionName:             "my-composition",
					CompositeResourceApiVersion: "example.org/v1",
					CompositeResourceKind:       "XDatabase",
					CompositeResourceName:       "my-db",
					CompositeResourceNamespace:  "default",
					CompositeResourceUid:        "uid-123",
				},
			},
		},
		"NilContext": {
			reason: "Should return error when context is nil.",
			args: args{
				ctx:          nil,
				functionName: "test-function",
				req:          &fnv1.RunFunctionRequest{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MissingTraceId": {
			reason: "Should return error when trace ID is missing from context.",
			args: args{
				ctx:          context.Background(),
				functionName: "test-function",
				req:          &fnv1.RunFunctionRequest{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MissingStepIndex": {
			reason: "Should return error when step index is missing from context.",
			args: args{
				ctx:          context.WithValue(context.Background(), ContextKeyTraceID, "trace"),
				functionName: "test-function",
				req:          &fnv1.RunFunctionRequest{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MissingCompositionName": {
			reason: "Should return error when composition name is missing from context.",
			args: args{
				ctx: context.WithValue(
					context.WithValue(context.Background(), ContextKeyTraceID, "trace"),
					ContextKeyStepIndex, int32(0),
				),
				functionName: "test-function",
				req:          &fnv1.RunFunctionRequest{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MissingIterationIsOptional": {
			reason: "Should succeed when iteration is missing (defaults to 0).",
			args: args{
				ctx: context.WithValue(
					context.WithValue(
						context.WithValue(context.Background(), ContextKeyTraceID, "trace"),
						ContextKeyStepIndex, int32(1),
					),
					ContextKeyCompositionName, "comp",
				),
				functionName: "test-function",
				req:          &fnv1.RunFunctionRequest{},
			},
			want: want{
				meta: &pipelinev1alpha1.StepMeta{
					TraceId:         "trace",
					StepIndex:       1,
					Iteration:       0,
					FunctionName:    "test-function",
					CompositionName: "comp",
				},
			},
		},
		"EmptyRequest": {
			reason: "Should handle empty request without composite resource.",
			args: args{
				ctx:          validCtx,
				functionName: "test-function",
				req:          &fnv1.RunFunctionRequest{},
			},
			want: want{
				meta: &pipelinev1alpha1.StepMeta{
					TraceId:         "trace-abc",
					StepIndex:       2,
					Iteration:       5,
					FunctionName:    "test-function",
					CompositionName: "my-composition",
				},
			},
		},
		"NilRequest": {
			reason: "Should handle nil request.",
			args: args{
				ctx:          validCtx,
				functionName: "test-function",
				req:          nil,
			},
			want: want{
				meta: &pipelinev1alpha1.StepMeta{
					TraceId:         "trace-abc",
					StepIndex:       2,
					Iteration:       5,
					FunctionName:    "test-function",
					CompositionName: "my-composition",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := BuildMetadata(tc.args.ctx, tc.args.functionName, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildMetadata(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if tc.want.err != nil {
				return
			}

			// Check that SpanId was generated (non-empty UUID).
			if got.GetSpanId() == "" {
				t.Errorf("\n%s\nBuildMetadata(...): expected SpanId to be set", tc.reason)
			}
			got.SpanId = ""

			// Check that Timestamp was set.
			if got.GetTimestamp().AsTime().IsZero() {
				t.Errorf("\n%s\nBuildMetadata(...): expected Timestamp to be set", tc.reason)
			}
			got.Timestamp = nil

			// Compare other fields (ignoring SpanId and Timestamp which are dynamic).
			if diff := cmp.Diff(tc.want.meta, got,
				protocmp.Transform(),
			); diff != "" {
				t.Errorf("\n%s\nBuildMetadata(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetStringField(t *testing.T) {
	cases := map[string]struct {
		reason string
		s      *structpb.Struct
		key    string
		want   string
	}{
		"NilStruct": {
			reason: "Should return empty string for nil struct.",
			s:      nil,
			key:    "key",
			want:   "",
		},
		"MissingKey": {
			reason: "Should return empty string for missing key.",
			s: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"other": structpb.NewStringValue("value"),
				},
			},
			key:  "key",
			want: "",
		},
		"PresentKey": {
			reason: "Should return value for present key.",
			s: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"key": structpb.NewStringValue("my-value"),
				},
			},
			key:  "key",
			want: "my-value",
		},
		"EmptyFields": {
			reason: "Should return empty string for empty fields map.",
			s:      &structpb.Struct{Fields: map[string]*structpb.Value{}},
			key:    "key",
			want:   "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getStringField(tc.s, tc.key)
			if got != tc.want {
				t.Errorf("\n%s\ngetStringField(...): want %q, got %q", tc.reason, tc.want, got)
			}
		})
	}
}

func TestGetStructField(t *testing.T) {
	nestedStruct := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"nested": structpb.NewStringValue("value"),
		},
	}

	cases := map[string]struct {
		reason string
		s      *structpb.Struct
		key    string
		want   *structpb.Struct
	}{
		"NilStruct": {
			reason: "Should return nil for nil struct.",
			s:      nil,
			key:    "key",
			want:   nil,
		},
		"MissingKey": {
			reason: "Should return nil for missing key.",
			s: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"other": structpb.NewStructValue(nestedStruct),
				},
			},
			key:  "key",
			want: nil,
		},
		"PresentKey": {
			reason: "Should return nested struct for present key.",
			s: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"key": structpb.NewStructValue(nestedStruct),
				},
			},
			key:  "key",
			want: nestedStruct,
		},
		"EmptyFields": {
			reason: "Should return nil for empty fields map.",
			s:      &structpb.Struct{Fields: map[string]*structpb.Value{}},
			key:    "key",
			want:   nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getStructField(tc.s, tc.key)
			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\ngetStructField(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

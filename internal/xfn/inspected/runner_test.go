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

package inspected

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	pipelinev1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/pipelineinspector/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/step"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// FunctionRunnerFn wraps a function as a FunctionRunner.
type FunctionRunnerFn func(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)

func (fn FunctionRunnerFn) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	return fn(ctx, name, req)
}

// MockPipelineInspector is a mock implementation of PipelineInspector for testing.
type MockPipelineInspector struct {
	EmitRequestCalled  bool
	EmitResponseCalled bool
	EmitRequestErr     error
	EmitResponseErr    error
	LastRequest        *fnv1.RunFunctionRequest
	LastResponse       *fnv1.RunFunctionResponse
	LastRequestMeta    *pipelinev1alpha1.StepMeta
	LastResponseMeta   *pipelinev1alpha1.StepMeta
	LastResponseErr    error
}

func (m *MockPipelineInspector) EmitRequest(_ context.Context, req *fnv1.RunFunctionRequest, meta *pipelinev1alpha1.StepMeta) error {
	m.EmitRequestCalled = true
	m.LastRequest = req
	m.LastRequestMeta = meta
	return m.EmitRequestErr
}

func (m *MockPipelineInspector) EmitResponse(_ context.Context, rsp *fnv1.RunFunctionResponse, err error, meta *pipelinev1alpha1.StepMeta) error {
	m.EmitResponseCalled = true
	m.LastResponse = rsp
	m.LastResponseMeta = meta
	m.LastResponseErr = err
	return m.EmitResponseErr
}

// MockMetrics is a mock implementation of Metrics for testing.
type MockMetrics struct {
	RequestErrors  []string
	ResponseErrors []string
}

func (m *MockMetrics) ErrorOnRequest(name string) {
	m.RequestErrors = append(m.RequestErrors, name)
}

func (m *MockMetrics) ErrorOnResponse(name string) {
	m.ResponseErrors = append(m.ResponseErrors, name)
}

func TestRunFunction(t *testing.T) {
	type params struct {
		wrap      FunctionRunner
		inspector *MockPipelineInspector
		metrics   *MockMetrics
	}

	type args struct {
		ctx  context.Context
		name string
		req  *fnv1.RunFunctionRequest
	}

	type want struct {
		rsp                *fnv1.RunFunctionResponse
		err                error
		emitRequestCalled  bool
		emitResponseCalled bool
		requestErrors      []string
		responseErrors     []string
	}

	// Create a valid context with all required step metadata.
	validCtx := step.ForCompositions(step.ContextWithStepMetaForCompositions(context.Background(), "trace-123", "test-step", 0, "my-composition"))

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"SuccessfulEmission": {
			reason: "Should emit request, run function, emit response, and return response.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{Tag: "test"},
					}, nil
				}),
				inspector: &MockPipelineInspector{},
				metrics:   &MockMetrics{},
			},
			args: args{
				ctx:  validCtx,
				name: "test-function",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "test"},
				},
				emitRequestCalled:  true,
				emitResponseCalled: true,
			},
		},
		"MetadataExtractionError": {
			reason: "Should skip inspection but still run function when metadata extraction fails.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{Tag: "still-works"},
					}, nil
				}),
				inspector: &MockPipelineInspector{},
				metrics:   &MockMetrics{},
			},
			args: args{
				ctx:  context.Background(), // Missing required context keys
				name: "test-function",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "still-works"},
				},
				emitRequestCalled:  false,
				emitResponseCalled: false,
			},
		},
		"EmitRequestError": {
			reason: "Should record metric and continue when EmitRequest fails.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{Tag: "still-runs"},
					}, nil
				}),
				inspector: &MockPipelineInspector{
					EmitRequestErr: errors.New("emit request failed"),
				},
				metrics: &MockMetrics{},
			},
			args: args{
				ctx:  validCtx,
				name: "test-function",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "still-runs"},
				},
				emitRequestCalled:  true,
				emitResponseCalled: true,
				requestErrors:      []string{"test-function"},
			},
		},
		"EmitResponseError": {
			reason: "Should record metric but still return result when EmitResponse fails.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{Tag: "response"},
					}, nil
				}),
				inspector: &MockPipelineInspector{
					EmitResponseErr: errors.New("emit response failed"),
				},
				metrics: &MockMetrics{},
			},
			args: args{
				ctx:  validCtx,
				name: "test-function",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "response"},
				},
				emitRequestCalled:  true,
				emitResponseCalled: true,
				responseErrors:     []string{"test-function"},
			},
		},
		"WrappedFunctionError": {
			reason: "Should emit response with error and propagate the error.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return nil, errors.New("function failed")
				}),
				inspector: &MockPipelineInspector{},
				metrics:   &MockMetrics{},
			},
			args: args{
				ctx:  validCtx,
				name: "test-function",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				err:                cmpopts.AnyError,
				emitRequestCalled:  true,
				emitResponseCalled: true,
			},
		},
		"BothEmitErrors": {
			reason: "Should record both metrics but still return wrapped function result.",
			params: params{
				wrap: FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
					return &fnv1.RunFunctionResponse{
						Meta: &fnv1.ResponseMeta{Tag: "success"},
					}, nil
				}),
				inspector: &MockPipelineInspector{
					EmitRequestErr:  errors.New("emit request failed"),
					EmitResponseErr: errors.New("emit response failed"),
				},
				metrics: &MockMetrics{},
			},
			args: args{
				ctx:  validCtx,
				name: "test-function",
				req:  &fnv1.RunFunctionRequest{},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "success"},
				},
				emitRequestCalled:  true,
				emitResponseCalled: true,
				requestErrors:      []string{"test-function"},
				responseErrors:     []string{"test-function"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewRunner(tc.params.wrap, tc.params.inspector,
				WithMetrics(tc.params.metrics),
				WithLogger(logging.NewNopLogger()))

			rsp, err := r.RunFunction(tc.args.ctx, tc.args.name, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if tc.want.emitRequestCalled != tc.params.inspector.EmitRequestCalled {
				t.Errorf("\n%s\nEmitRequestCalled: want %v, got %v", tc.reason, tc.want.emitRequestCalled, tc.params.inspector.EmitRequestCalled)
			}

			if tc.want.emitResponseCalled != tc.params.inspector.EmitResponseCalled {
				t.Errorf("\n%s\nEmitResponseCalled: want %v, got %v", tc.reason, tc.want.emitResponseCalled, tc.params.inspector.EmitResponseCalled)
			}

			if diff := cmp.Diff(tc.want.requestErrors, tc.params.metrics.RequestErrors); diff != "" {
				t.Errorf("\n%s\nRequestErrors: -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.responseErrors, tc.params.metrics.ResponseErrors); diff != "" {
				t.Errorf("\n%s\nResponseErrors: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunFunctionMetadataConsistency(t *testing.T) {
	// This test verifies that the same metadata is passed to both EmitRequest and EmitResponse.
	validCtx := step.ForCompositions(step.ContextWithStepMetaForCompositions(context.Background(), "trace-abc", "test-step", 2, "test-composition"))

	inspector := &MockPipelineInspector{}
	metrics := &MockMetrics{}

	wrap := FunctionRunnerFn(func(_ context.Context, _ string, _ *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
		return &fnv1.RunFunctionResponse{}, nil
	})

	r := NewRunner(wrap, inspector,
		WithMetrics(metrics),
		WithLogger(logging.NewNopLogger()))

	_, err := r.RunFunction(validCtx, "my-function", &fnv1.RunFunctionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metadata was captured.
	if inspector.LastRequestMeta == nil {
		t.Fatal("expected LastRequestMeta to be set")
	}
	if inspector.LastResponseMeta == nil {
		t.Fatal("expected LastResponseMeta to be set")
	}

	// Verify the same metadata instance is passed to both.
	if diff := cmp.Diff(inspector.LastRequestMeta, inspector.LastResponseMeta, protocmp.Transform()); diff != "" {
		t.Errorf("metadata mismatch between request and response (-want +got):\n%s", diff)
	}

	// Verify metadata fields are as expected.
	meta := inspector.LastRequestMeta
	if diff := cmp.Diff(&pipelinev1alpha1.StepMeta{
		FunctionName: "my-function",
		TraceId:      "trace-abc",
		StepName:     "test-step",
		StepIndex:    2,
		Context: &pipelinev1alpha1.StepMeta_CompositionMeta{
			CompositionMeta: &pipelinev1alpha1.CompositionMeta{
				CompositionName: "test-composition",
			},
		},
	}, meta, protocmp.Transform(), protocmp.IgnoreFields(&pipelinev1alpha1.StepMeta{}, "span_id", "timestamp")); diff != "" {
		t.Errorf("metadata fields mismatch (-want +got):\n%s", diff)
	}
}

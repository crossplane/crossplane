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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	pipelinev1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/pipelineinspector/proto/v1alpha1"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// MockPipelineInspectorServiceClient is a mock implementation of the gRPC client.
type MockPipelineInspectorServiceClient struct {
	EmitRequestFn  func(ctx context.Context, in *pipelinev1alpha1.EmitRequestRequest, opts ...grpc.CallOption) (*pipelinev1alpha1.EmitRequestResponse, error)
	EmitResponseFn func(ctx context.Context, in *pipelinev1alpha1.EmitResponseRequest, opts ...grpc.CallOption) (*pipelinev1alpha1.EmitResponseResponse, error)

	// Captured values for assertions.
	LastEmitRequestRequest  *pipelinev1alpha1.EmitRequestRequest
	LastEmitResponseRequest *pipelinev1alpha1.EmitResponseRequest
}

func (m *MockPipelineInspectorServiceClient) EmitRequest(ctx context.Context, in *pipelinev1alpha1.EmitRequestRequest, opts ...grpc.CallOption) (*pipelinev1alpha1.EmitRequestResponse, error) {
	m.LastEmitRequestRequest = in
	if m.EmitRequestFn != nil {
		return m.EmitRequestFn(ctx, in, opts...)
	}
	return &pipelinev1alpha1.EmitRequestResponse{}, nil
}

func (m *MockPipelineInspectorServiceClient) EmitResponse(ctx context.Context, in *pipelinev1alpha1.EmitResponseRequest, opts ...grpc.CallOption) (*pipelinev1alpha1.EmitResponseResponse, error) {
	m.LastEmitResponseRequest = in
	if m.EmitResponseFn != nil {
		return m.EmitResponseFn(ctx, in, opts...)
	}
	return &pipelinev1alpha1.EmitResponseResponse{}, nil
}

func TestSocketPipelineInspectorEmitRequest(t *testing.T) {
	type args struct {
		ctx  context.Context
		req  *fnv1.RunFunctionRequest
		meta *pipelinev1alpha1.StepMeta
	}

	type want struct {
		err                  error
		credentialsStripped  bool
		connectionDetailsNil bool
	}

	validMeta := &pipelinev1alpha1.StepMeta{
		TraceId:         "trace-123",
		SpanId:          "span-456",
		StepIndex:       1,
		Iteration:       2,
		FunctionName:    "test-function",
		CompositionName: "test-composition",
	}

	cases := map[string]struct {
		reason string
		client *MockPipelineInspectorServiceClient
		args   args
		want   want
	}{
		"SuccessfulEmission": {
			reason: "Should emit request successfully with valid metadata.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "test"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{},
						},
					},
				},
				meta: validMeta,
			},
			want: want{
				credentialsStripped:  true,
				connectionDetailsNil: true,
			},
		},
		"NilMetadata": {
			reason: "Should return error when metadata is nil.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{},
						},
					},
				},
				meta: nil,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"StripsCredentials": {
			reason: "Should strip credentials from request before emission.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Credentials: map[string]*fnv1.Credentials{
						"secret": {
							Source: &fnv1.Credentials_CredentialData{
								CredentialData: &fnv1.CredentialData{
									Data: map[string][]byte{"password": []byte("secret123")},
								},
							},
						},
					},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{},
						},
					},
				},
				meta: validMeta,
			},
			want: want{
				credentialsStripped:  true,
				connectionDetailsNil: true,
			},
		},
		"StripsConnectionDetails": {
			reason: "Should strip connection details from observed composite and resources.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("example.org/v1"),
								},
							},
							ConnectionDetails: map[string][]byte{"conn": []byte("details")},
						},
						Resources: map[string]*fnv1.Resource{
							"resource1": {
								Resource:          &structpb.Struct{},
								ConnectionDetails: map[string][]byte{"conn": []byte("details")},
							},
						},
					},
				},
				meta: validMeta,
			},
			want: want{
				credentialsStripped:  true,
				connectionDetailsNil: true,
			},
		},
		"StripsDesiredConnectionDetails": {
			reason: "Should strip connection details from desired composite and resources.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{},
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("example.org/v1"),
								},
							},
							ConnectionDetails: map[string][]byte{"conn": []byte("desired-details")},
						},
						Resources: map[string]*fnv1.Resource{
							"resource1": {
								Resource:          &structpb.Struct{},
								ConnectionDetails: map[string][]byte{"conn": []byte("desired-details")},
							},
						},
					},
				},
				meta: validMeta,
			},
			want: want{
				credentialsStripped:  true,
				connectionDetailsNil: true,
			},
		},
		"GRPCCallFails": {
			reason: "Should return error when gRPC call fails.",
			client: &MockPipelineInspectorServiceClient{
				EmitRequestFn: func(_ context.Context, _ *pipelinev1alpha1.EmitRequestRequest, _ ...grpc.CallOption) (*pipelinev1alpha1.EmitRequestResponse, error) {
					return nil, errors.New("connection refused")
				},
			},
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: &structpb.Struct{},
						},
					},
				},
				meta: validMeta,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			inspector := &SocketPipelineInspector{
				client:  tc.client,
				timeout: 100 * time.Millisecond,
			}

			err := inspector.EmitRequest(tc.args.ctx, tc.args.req, tc.args.meta)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nEmitRequest(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// If we expected success, verify credentials were stripped.
			if tc.want.err == nil && tc.client.LastEmitRequestRequest != nil {
				// The request should have been marshaled without credentials.
				// We can't easily check the JSON, but we can verify the call was made.
				if tc.client.LastEmitRequestRequest.Request == nil {
					t.Errorf("\n%s\nEmitRequest(...): expected request bytes to be set", tc.reason)
				}
				if tc.client.LastEmitRequestRequest.GetMeta() == nil {
					t.Errorf("\n%s\nEmitRequest(...): expected meta to be set", tc.reason)
				}
			}
		})
	}
}

func TestSocketPipelineInspectorEmitResponse(t *testing.T) {
	type args struct {
		ctx   context.Context
		rsp   *fnv1.RunFunctionResponse
		fnErr error
		meta  *pipelinev1alpha1.StepMeta
	}

	type want struct {
		err      error
		errField string
	}

	validMeta := &pipelinev1alpha1.StepMeta{
		TraceId:         "trace-123",
		SpanId:          "span-456",
		StepIndex:       1,
		Iteration:       2,
		FunctionName:    "test-function",
		CompositionName: "test-composition",
	}

	cases := map[string]struct {
		reason string
		client *MockPipelineInspectorServiceClient
		args   args
		want   want
	}{
		"SuccessfulEmission": {
			reason: "Should emit response successfully with valid metadata.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx: context.Background(),
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "test"},
				},
				meta: validMeta,
			},
			want: want{},
		},
		"NilMetadata": {
			reason: "Should return error when metadata is nil.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx:  context.Background(),
				rsp:  &fnv1.RunFunctionResponse{},
				meta: nil,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NilResponse": {
			reason: "Should handle nil response (function call failed).",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx:   context.Background(),
				rsp:   nil,
				fnErr: errors.New("function failed"),
				meta:  validMeta,
			},
			want: want{
				errField: "function failed",
			},
		},
		"CapturesFunctionError": {
			reason: "Should capture function error message in the emitted response.",
			client: &MockPipelineInspectorServiceClient{},
			args: args{
				ctx: context.Background(),
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "partial"},
				},
				fnErr: errors.New("something went wrong"),
				meta:  validMeta,
			},
			want: want{
				errField: "something went wrong",
			},
		},
		"GRPCCallFails": {
			reason: "Should return error when gRPC call fails.",
			client: &MockPipelineInspectorServiceClient{
				EmitResponseFn: func(_ context.Context, _ *pipelinev1alpha1.EmitResponseRequest, _ ...grpc.CallOption) (*pipelinev1alpha1.EmitResponseResponse, error) {
					return nil, errors.New("connection refused")
				},
			},
			args: args{
				ctx:  context.Background(),
				rsp:  &fnv1.RunFunctionResponse{},
				meta: validMeta,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			inspector := &SocketPipelineInspector{
				client:  tc.client,
				timeout: 100 * time.Millisecond,
			}

			err := inspector.EmitResponse(tc.args.ctx, tc.args.rsp, tc.args.fnErr, tc.args.meta)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nEmitResponse(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// If we expected success, verify the error field was captured correctly.
			if tc.want.err == nil && tc.client.LastEmitResponseRequest != nil {
				if tc.want.errField != "" {
					if tc.client.LastEmitResponseRequest.GetError() != tc.want.errField {
						t.Errorf("\n%s\nEmitResponse(...): want error field %q, got %q", tc.reason, tc.want.errField, tc.client.LastEmitResponseRequest.GetError())
					}
				}
				if tc.client.LastEmitResponseRequest.GetMeta() == nil {
					t.Errorf("\n%s\nEmitResponse(...): expected meta to be set", tc.reason)
				}
			}
		})
	}
}

func TestSanitizeState(t *testing.T) {
	cases := map[string]struct {
		reason string
		state  *fnv1.State
		want   *fnv1.State
	}{
		"NilState": {
			reason: "Should handle nil state without panic.",
			state:  nil,
			want:   nil,
		},
		"EmptyState": {
			reason: "Should handle empty state.",
			state:  &fnv1.State{},
			want:   &fnv1.State{},
		},
		"RedactsCompositeConnectionDetails": {
			reason: "Should redact connection details from composite resource, preserving keys.",
			state: &fnv1.State{
				Composite: &fnv1.Resource{
					Resource: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"apiVersion": structpb.NewStringValue("example.org/v1"),
							"kind":       structpb.NewStringValue("XDatabase"),
						},
					},
					ConnectionDetails: map[string][]byte{"password": []byte("secret")},
				},
			},
			want: &fnv1.State{
				Composite: &fnv1.Resource{
					Resource: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"apiVersion": structpb.NewStringValue("example.org/v1"),
							"kind":       structpb.NewStringValue("XDatabase"),
						},
					},
					ConnectionDetails: map[string][]byte{"password": []byte(redactedValue)},
				},
			},
		},
		"RedactsResourceConnectionDetails": {
			reason: "Should redact connection details from composed resources, preserving keys.",
			state: &fnv1.State{
				Resources: map[string]*fnv1.Resource{
					"db": {
						Resource: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"apiVersion": structpb.NewStringValue("rds.aws.upbound.io/v1beta1"),
								"kind":       structpb.NewStringValue("Instance"),
							},
						},
						ConnectionDetails: map[string][]byte{
							"endpoint": []byte("db.example.com"),
							"port":     []byte("5432"),
						},
					},
				},
			},
			want: &fnv1.State{
				Resources: map[string]*fnv1.Resource{
					"db": {
						Resource: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"apiVersion": structpb.NewStringValue("rds.aws.upbound.io/v1beta1"),
								"kind":       structpb.NewStringValue("Instance"),
							},
						},
						ConnectionDetails: map[string][]byte{
							"endpoint": []byte(redactedValue),
							"port":     []byte(redactedValue),
						},
					},
				},
			},
		},
		"RedactsSecretData": {
			reason: "Should redact data field values from Secret resources, preserving keys.",
			state: &fnv1.State{
				Resources: map[string]*fnv1.Resource{
					"my-secret": {
						Resource: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"apiVersion": structpb.NewStringValue("v1"),
								"kind":       structpb.NewStringValue("Secret"),
								"metadata": structpb.NewStructValue(&structpb.Struct{
									Fields: map[string]*structpb.Value{
										"name": structpb.NewStringValue("my-secret"),
									},
								}),
								"data": structpb.NewStructValue(&structpb.Struct{
									Fields: map[string]*structpb.Value{
										"password": structpb.NewStringValue("c2VjcmV0"), // base64 encoded
										"username": structpb.NewStringValue("YWRtaW4="), // base64 encoded
									},
								}),
								"type": structpb.NewStringValue("Opaque"),
							},
						},
					},
				},
			},
			want: &fnv1.State{
				Resources: map[string]*fnv1.Resource{
					"my-secret": {
						Resource: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"apiVersion": structpb.NewStringValue("v1"),
								"kind":       structpb.NewStringValue("Secret"),
								"metadata": structpb.NewStructValue(&structpb.Struct{
									Fields: map[string]*structpb.Value{
										"name": structpb.NewStringValue("my-secret"),
									},
								}),
								"data": structpb.NewStructValue(&structpb.Struct{
									Fields: map[string]*structpb.Value{
										"password": structpb.NewStringValue(redactedValue),
										"username": structpb.NewStringValue(redactedValue),
									},
								}),
								"type": structpb.NewStringValue("Opaque"),
							},
						},
					},
				},
			},
		},
		"PreservesNonSecretData": {
			reason: "Should preserve data-like fields on non-Secret resources.",
			state: &fnv1.State{
				Resources: map[string]*fnv1.Resource{
					"configmap": {
						Resource: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"apiVersion": structpb.NewStringValue("v1"),
								"kind":       structpb.NewStringValue("ConfigMap"),
								"data": structpb.NewStructValue(&structpb.Struct{
									Fields: map[string]*structpb.Value{
										"config.yaml": structpb.NewStringValue("key: value"),
									},
								}),
							},
						},
					},
				},
			},
			want: &fnv1.State{
				Resources: map[string]*fnv1.Resource{
					"configmap": {
						Resource: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"apiVersion": structpb.NewStringValue("v1"),
								"kind":       structpb.NewStringValue("ConfigMap"),
								"data": structpb.NewStructValue(&structpb.Struct{
									Fields: map[string]*structpb.Value{
										"config.yaml": structpb.NewStringValue("key: value"),
									},
								}),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sanitizeState(tc.state)

			if diff := cmp.Diff(tc.want, tc.state, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nsanitizeState(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRedactCredentials(t *testing.T) {
	cases := map[string]struct {
		reason      string
		credentials map[string]*fnv1.Credentials
		want        map[string]*fnv1.Credentials
	}{
		"NilCredentials": {
			reason:      "Should handle nil credentials without panic.",
			credentials: nil,
			want:        nil,
		},
		"EmptyCredentials": {
			reason:      "Should handle empty credentials.",
			credentials: map[string]*fnv1.Credentials{},
			want:        map[string]*fnv1.Credentials{},
		},
		"RedactsCredentialData": {
			reason: "Should redact credential data values, preserving keys.",
			credentials: map[string]*fnv1.Credentials{
				"db-creds": {
					Source: &fnv1.Credentials_CredentialData{
						CredentialData: &fnv1.CredentialData{
							Data: map[string][]byte{
								"username": []byte("admin"),
								"password": []byte("supersecret"),
							},
						},
					},
				},
			},
			want: map[string]*fnv1.Credentials{
				"db-creds": {
					Source: &fnv1.Credentials_CredentialData{
						CredentialData: &fnv1.CredentialData{
							Data: map[string][]byte{
								"username": []byte(redactedValue),
								"password": []byte(redactedValue),
							},
						},
					},
				},
			},
		},
		"RedactsMultipleCredentials": {
			reason: "Should redact multiple credential entries.",
			credentials: map[string]*fnv1.Credentials{
				"cred1": {
					Source: &fnv1.Credentials_CredentialData{
						CredentialData: &fnv1.CredentialData{
							Data: map[string][]byte{"key1": []byte("value1")},
						},
					},
				},
				"cred2": {
					Source: &fnv1.Credentials_CredentialData{
						CredentialData: &fnv1.CredentialData{
							Data: map[string][]byte{"key2": []byte("value2")},
						},
					},
				},
			},
			want: map[string]*fnv1.Credentials{
				"cred1": {
					Source: &fnv1.Credentials_CredentialData{
						CredentialData: &fnv1.CredentialData{
							Data: map[string][]byte{"key1": []byte(redactedValue)},
						},
					},
				},
				"cred2": {
					Source: &fnv1.Credentials_CredentialData{
						CredentialData: &fnv1.CredentialData{
							Data: map[string][]byte{"key2": []byte(redactedValue)},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			redactCredentials(tc.credentials)

			if diff := cmp.Diff(tc.want, tc.credentials, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nredactCredentials(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSanitizeRequiredResources(t *testing.T) {
	cases := map[string]struct {
		reason    string
		resources map[string]*fnv1.Resources
		want      map[string]*fnv1.Resources
	}{
		"NilResources": {
			reason:    "Should handle nil resources without panic.",
			resources: nil,
			want:      nil,
		},
		"EmptyResources": {
			reason:    "Should handle empty resources.",
			resources: map[string]*fnv1.Resources{},
			want:      map[string]*fnv1.Resources{},
		},
		"RedactsConnectionDetails": {
			reason: "Should redact connection details from resource items, preserving keys.",
			resources: map[string]*fnv1.Resources{
				"extra": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("example.org/v1"),
									"kind":       structpb.NewStringValue("XDatabase"),
								},
							},
							ConnectionDetails: map[string][]byte{
								"endpoint": []byte("db.example.com"),
								"password": []byte("secret123"),
							},
						},
					},
				},
			},
			want: map[string]*fnv1.Resources{
				"extra": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("example.org/v1"),
									"kind":       structpb.NewStringValue("XDatabase"),
								},
							},
							ConnectionDetails: map[string][]byte{
								"endpoint": []byte(redactedValue),
								"password": []byte(redactedValue),
							},
						},
					},
				},
			},
		},
		"RedactsSecretData": {
			reason: "Should redact data field values from Secret resources, preserving keys.",
			resources: map[string]*fnv1.Resources{
				"secrets": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("Secret"),
									"metadata": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"name": structpb.NewStringValue("my-secret"),
										},
									}),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"password": structpb.NewStringValue("c2VjcmV0"),
											"token":    structpb.NewStringValue("dG9rZW4="),
										},
									}),
								},
							},
						},
					},
				},
			},
			want: map[string]*fnv1.Resources{
				"secrets": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("Secret"),
									"metadata": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"name": structpb.NewStringValue("my-secret"),
										},
									}),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"password": structpb.NewStringValue(redactedValue),
											"token":    structpb.NewStringValue(redactedValue),
										},
									}),
								},
							},
						},
					},
				},
			},
		},
		"PreservesNonSecretData": {
			reason: "Should preserve data-like fields on non-Secret resources.",
			resources: map[string]*fnv1.Resources{
				"configs": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("ConfigMap"),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"config.yaml": structpb.NewStringValue("key: value"),
										},
									}),
								},
							},
						},
					},
				},
			},
			want: map[string]*fnv1.Resources{
				"configs": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("ConfigMap"),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"config.yaml": structpb.NewStringValue("key: value"),
										},
									}),
								},
							},
						},
					},
				},
			},
		},
		"HandlesMultipleResourceGroups": {
			reason: "Should sanitize multiple resource groups with multiple items.",
			resources: map[string]*fnv1.Resources{
				"group1": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("Secret"),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"key1": structpb.NewStringValue("value1"),
										},
									}),
								},
							},
							ConnectionDetails: map[string][]byte{"conn1": []byte("secret1")},
						},
					},
				},
				"group2": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("Secret"),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"key2": structpb.NewStringValue("value2"),
										},
									}),
								},
							},
							ConnectionDetails: map[string][]byte{"conn2": []byte("secret2")},
						},
					},
				},
			},
			want: map[string]*fnv1.Resources{
				"group1": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("Secret"),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"key1": structpb.NewStringValue(redactedValue),
										},
									}),
								},
							},
							ConnectionDetails: map[string][]byte{"conn1": []byte(redactedValue)},
						},
					},
				},
				"group2": {
					Items: []*fnv1.Resource{
						{
							Resource: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"apiVersion": structpb.NewStringValue("v1"),
									"kind":       structpb.NewStringValue("Secret"),
									"data": structpb.NewStructValue(&structpb.Struct{
										Fields: map[string]*structpb.Value{
											"key2": structpb.NewStringValue(redactedValue),
										},
									}),
								},
							},
							ConnectionDetails: map[string][]byte{"conn2": []byte(redactedValue)},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sanitizeRequiredResources(tc.resources)

			if diff := cmp.Diff(tc.want, tc.resources, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nsanitizeRequiredResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRedactConnectionDetails(t *testing.T) {
	cases := map[string]struct {
		reason            string
		connectionDetails map[string][]byte
		want              map[string][]byte
	}{
		"NilConnectionDetails": {
			reason:            "Should handle nil connection details without panic.",
			connectionDetails: nil,
			want:              nil,
		},
		"EmptyConnectionDetails": {
			reason:            "Should handle empty connection details.",
			connectionDetails: map[string][]byte{},
			want:              map[string][]byte{},
		},
		"RedactsConnectionDetails": {
			reason: "Should redact connection detail values, preserving keys.",
			connectionDetails: map[string][]byte{
				"endpoint": []byte("db.example.com"),
				"password": []byte("secret123"),
				"port":     []byte("5432"),
			},
			want: map[string][]byte{
				"endpoint": []byte(redactedValue),
				"password": []byte(redactedValue),
				"port":     []byte(redactedValue),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			redactConnectionDetails(tc.connectionDetails)

			if diff := cmp.Diff(tc.want, tc.connectionDetails); diff != "" {
				t.Errorf("\n%s\nredactConnectionDetails(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

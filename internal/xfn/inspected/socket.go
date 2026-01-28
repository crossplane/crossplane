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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pipelinev1alpha1 "github.com/crossplane/crossplane-runtime/v2/apis/pipelineinspector/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

const (
	// Default timeout for emit calls.
	defaultEmitTimeout = 100 * time.Millisecond

	// redactedValue is used to replace sensitive data values while preserving keys.
	redactedValue = "**REDACTED**"
)

// A SocketPipelineInspector emits pipeline execution data to a Pipeline
// Inspector sidecar via Unix domain socket.
type SocketPipelineInspector struct {
	client  pipelinev1alpha1.PipelineInspectorServiceClient
	timeout time.Duration
}

// A SocketPipelineInspectorOption configures a SocketPipelineInspector.
type SocketPipelineInspectorOption func(*SocketPipelineInspector)

// NewSocketPipelineInspector creates a new SocketPipelineInspector that
// connects to the given Unix socket path. The connection is established lazily
// on first use.
func NewSocketPipelineInspector(socketPath string, o ...SocketPipelineInspectorOption) (*SocketPipelineInspector, error) {
	// Connect to Unix domain socket.
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	e := &SocketPipelineInspector{
		client:  pipelinev1alpha1.NewPipelineInspectorServiceClient(conn),
		timeout: defaultEmitTimeout,
	}

	for _, fn := range o {
		fn(e)
	}

	return e, nil
}

// EmitRequest emits the function request before execution. Credentials are
// stripped from the request before emission for security.
func (e *SocketPipelineInspector) EmitRequest(ctx context.Context, req *fnv1.RunFunctionRequest, meta *pipelinev1alpha1.StepMeta) error {
	if meta == nil {
		return errors.New("step metadata is required to emit pipeline request")
	}

	// Create a copy with sensitive data redacted for security.
	sanitizedReq, ok := proto.Clone(req).(*fnv1.RunFunctionRequest)
	if !ok {
		return errors.New("failed to clone pipeline request for sanitization")
	}

	redactCredentials(sanitizedReq.GetCredentials())
	sanitizeState(sanitizedReq.GetObserved())
	sanitizeState(sanitizedReq.GetDesired())

	// Serialize the request to JSON bytes.
	reqBytes, err := protojson.Marshal(sanitizedReq)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal pipeline request for function %s", meta.GetFunctionName())
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	_, err = e.client.EmitRequest(ctx, &pipelinev1alpha1.EmitRequestRequest{
		Request: reqBytes,
		Meta:    meta,
	})
	return errors.Wrapf(err, "failed to emit pipeline request for function %s", meta.GetFunctionName())
}

// EmitResponse emits the function response after execution.
func (e *SocketPipelineInspector) EmitResponse(ctx context.Context, rsp *fnv1.RunFunctionResponse, fnErr error, meta *pipelinev1alpha1.StepMeta) error {
	if meta == nil {
		return errors.New("step metadata is required to emit pipeline response")
	}
	errMsg := ""
	if fnErr != nil {
		errMsg = fnErr.Error()
	}

	// Serialize the response to JSON bytes (if not nil).
	var rspBytes []byte
	if rsp != nil {
		var err error
		rspBytes, err = protojson.Marshal(rsp)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal pipeline response for function %s", meta.GetFunctionName())
		}
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	_, err := e.client.EmitResponse(ctx, &pipelinev1alpha1.EmitResponseRequest{
		Response: rspBytes,
		Error:    errMsg,
		Meta:     meta,
	})
	return errors.Wrapf(err, "failed to emit pipeline response for function %s", meta.GetFunctionName())
}

// redactCredentials redacts credential data values while preserving the keys.
// Values are replaced with []byte(redactedValue).
func redactCredentials(credentials map[string]*fnv1.Credentials) {
	for _, cred := range credentials {
		if data := cred.GetCredentialData(); data != nil {
			for k := range data.GetData() {
				data.Data[k] = []byte(redactedValue)
			}
		}
	}
}

// sanitizeState redacts sensitive data from a State object, including
// connection details from the composite resource and all composed resources,
// and the data field from any Secret resources.
func sanitizeState(state *fnv1.State) {
	if state == nil {
		return
	}
	if comp := state.GetComposite(); comp != nil {
		redactConnectionDetails(comp.GetConnectionDetails())
	}
	for _, cr := range state.GetResources() {
		redactConnectionDetails(cr.GetConnectionDetails())
		stripSecretData(cr.GetResource())
	}
}

// redactConnectionDetails redacts connection detail values while preserving the keys.
// Values are replaced with []byte(redactedValue).
func redactConnectionDetails(connectionDetails map[string][]byte) {
	for k := range connectionDetails {
		connectionDetails[k] = []byte(redactedValue)
	}
}

// stripSecretData redacts the data field values from a resource if it is a Kubernetes Secret.
// The keys are preserved but values are replaced with redactedValue.
func stripSecretData(resource *structpb.Struct) {
	if resource == nil {
		return
	}
	fields := resource.GetFields()
	if fields == nil {
		return
	}

	// Check if this is a Secret (apiVersion: v1, kind: Secret).
	apiVersion := fields["apiVersion"].GetStringValue()
	kind := fields["kind"].GetStringValue()
	if apiVersion == "v1" && kind == "Secret" {
		if data := fields["data"].GetStructValue(); data != nil {
			for k := range data.GetFields() {
				data.Fields[k] = structpb.NewStringValue(redactedValue)
			}
		}
	}
}

/*
Copyright 2024 The Crossplane Authors.

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

package managed

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/apis/changelogs/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

const (
	defaultSendTimeout = 10 * time.Second
)

// ChangeLogger is an interface for recording changes made to resources to the
// change logs.
type ChangeLogger interface {
	Log(ctx context.Context, managed resource.Managed, opType v1alpha1.OperationType, changeErr error, ad AdditionalDetails) error
}

// GRPCChangeLogger processes changes to resources and helps to send them to the
// change log gRPC service.
type GRPCChangeLogger struct {
	client          v1alpha1.ChangeLogServiceClient
	providerVersion string
	sendTimeout     time.Duration
}

// NewGRPCChangeLogger creates a new gRPC based ChangeLogger initialized with
// the given client.
func NewGRPCChangeLogger(client v1alpha1.ChangeLogServiceClient, o ...GRPCChangeLoggerOption) *GRPCChangeLogger {
	g := &GRPCChangeLogger{
		client:      client,
		sendTimeout: defaultSendTimeout,
	}

	for _, clo := range o {
		clo(g)
	}

	return g
}

// A GRPCChangeLoggerOption configures a GRPCChangeLoggerOption.
type GRPCChangeLoggerOption func(*GRPCChangeLogger)

// WithProviderVersion sets the provider version to be included in the change
// log entry.
func WithProviderVersion(version string) GRPCChangeLoggerOption {
	return func(g *GRPCChangeLogger) {
		g.providerVersion = version
	}
}

// WithSendTimeout sets the timeout for sending and/or waiting for change log
// entries to the change log service.
func WithSendTimeout(timeout time.Duration) GRPCChangeLoggerOption {
	return func(g *GRPCChangeLogger) {
		g.sendTimeout = timeout
	}
}

// Log sends the given change log entry to the change log service.
func (g *GRPCChangeLogger) Log(ctx context.Context, managed resource.Managed, opType v1alpha1.OperationType, changeErr error, ad AdditionalDetails) error {
	// get an error message from the error if it exists
	var changeErrMessage *string
	if changeErr != nil {
		changeErrMessage = ptr.To(changeErr.Error())
	}

	// capture the full state of the managed resource from before we performed the change
	snapshot, err := resource.AsProtobufStruct(managed)
	if err != nil {
		return errors.Wrap(err, "cannot snapshot managed resource")
	}

	gvk := managed.GetObjectKind().GroupVersionKind()

	entry := &v1alpha1.ChangeLogEntry{
		Timestamp:         timestamppb.Now(),
		Provider:          g.providerVersion,
		ApiVersion:        gvk.GroupVersion().String(),
		Kind:              gvk.Kind,
		Name:              managed.GetName(),
		ExternalName:      meta.GetExternalName(managed),
		Operation:         opType,
		Snapshot:          snapshot,
		ErrorMessage:      changeErrMessage,
		AdditionalDetails: ad,
	}

	// create a specific context and timeout for sending the change log entry
	// that is different than the parent context that is for the entire
	// reconciliation
	sendCtx, sendCancel := context.WithTimeout(ctx, g.sendTimeout)
	defer sendCancel()

	// send everything we've got to the change log service
	_, err = g.client.SendChangeLog(sendCtx, &v1alpha1.SendChangeLogRequest{Entry: entry}, grpc.WaitForReady(true))

	return errors.Wrap(err, "cannot send change log entry")
}

// nopChangeLogger does nothing for recording change logs, this is the default
// implementation if a provider has not enabled the change logs feature.
type nopChangeLogger struct{}

func newNopChangeLogger() *nopChangeLogger {
	return &nopChangeLogger{}
}

func (n *nopChangeLogger) Log(_ context.Context, _ resource.Managed, _ v1alpha1.OperationType, _ error, _ AdditionalDetails) error {
	return nil
}

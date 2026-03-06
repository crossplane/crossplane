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
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/apis/changelogs/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

// A mock implementation of the ChangeLogServiceClient interface to help with
// testing and verifying change log entries.
type changeLogServiceClient struct {
	requests []*v1alpha1.SendChangeLogRequest
	sendFn   func(ctx context.Context, in *v1alpha1.SendChangeLogRequest, opts ...grpc.CallOption) (*v1alpha1.SendChangeLogResponse, error)
}

func (c *changeLogServiceClient) SendChangeLog(ctx context.Context, in *v1alpha1.SendChangeLogRequest, opts ...grpc.CallOption) (*v1alpha1.SendChangeLogResponse, error) {
	c.requests = append(c.requests, in)
	if c.sendFn != nil {
		return c.sendFn(ctx, in, opts...)
	}

	return nil, nil
}

func TestChangeLogger(t *testing.T) {
	type args struct {
		mr  resource.Managed
		ad  AdditionalDetails
		err error
		c   *changeLogServiceClient
	}

	type want struct {
		requests []*v1alpha1.SendChangeLogRequest
		err      error
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ChangeLogsSuccess": {
			reason: "Change log entry should be recorded successfully.",
			args: args{
				mr: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        "cool-managed",
					Annotations: map[string]string{meta.AnnotationKeyExternalName: "cool-managed"},
				}},
				err: errBoom,
				ad:  AdditionalDetails{"key": "value", "key2": "value2"},
				c:   &changeLogServiceClient{requests: []*v1alpha1.SendChangeLogRequest{}},
			},
			want: want{
				// a well fleshed out change log entry should be sent
				requests: []*v1alpha1.SendChangeLogRequest{
					{
						Entry: &v1alpha1.ChangeLogEntry{
							Timestamp:    timestamppb.Now(),
							Provider:     "provider-cool:v9.99.999",
							ApiVersion:   (&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupVersion().String(),
							Kind:         (&fake.Managed{}).GetObjectKind().GroupVersionKind().Kind,
							Name:         "cool-managed",
							ExternalName: "cool-managed",
							Operation:    v1alpha1.OperationType_OPERATION_TYPE_CREATE,
							Snapshot: mustObjectAsProtobufStruct(&fake.Managed{ObjectMeta: metav1.ObjectMeta{
								Name:        "cool-managed",
								Annotations: map[string]string{meta.AnnotationKeyExternalName: "cool-managed"},
							}}),
							ErrorMessage:      ptr.To("boom"),
							AdditionalDetails: AdditionalDetails{"key": "value", "key2": "value2"},
						},
					},
				},
			},
		},
		"SendChangeLogsFailure": {
			reason: "Error from sending change log entry should be handled and recorded.",
			args: args{
				mr: &fake.Managed{},
				c: &changeLogServiceClient{
					requests: []*v1alpha1.SendChangeLogRequest{},
					// make the send change log function return an error
					sendFn: func(_ context.Context, _ *v1alpha1.SendChangeLogRequest, _ ...grpc.CallOption) (*v1alpha1.SendChangeLogResponse, error) {
						return &v1alpha1.SendChangeLogResponse{}, errBoom
					},
				},
			},
			want: want{
				// we'll still see a change log entry, but it won't make it all
				// the way to its destination and we should see an event for
				// that failure
				requests: []*v1alpha1.SendChangeLogRequest{
					{
						Entry: &v1alpha1.ChangeLogEntry{
							// we expect less fields to be set on the change log
							// entry because we're not initializing the managed
							// resource with much data in this simulated failure
							// test case
							Timestamp:  timestamppb.Now(),
							Provider:   "provider-cool:v9.99.999",
							ApiVersion: (&fake.Managed{}).GetObjectKind().GroupVersionKind().GroupVersion().String(),
							Kind:       (&fake.Managed{}).GetObjectKind().GroupVersionKind().Kind,
							Operation:  v1alpha1.OperationType_OPERATION_TYPE_CREATE,
							Snapshot:   mustObjectAsProtobufStruct(&fake.Managed{}),
						},
					},
				},
				err: errors.Wrap(errBoom, "cannot send change log entry"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			change := NewGRPCChangeLogger(tc.args.c, WithProviderVersion("provider-cool:v9.99.999"))
			err := change.Log(context.Background(), tc.args.mr, v1alpha1.OperationType_OPERATION_TYPE_CREATE, tc.args.err, tc.args.ad)

			if diff := cmp.Diff(tc.want.requests, tc.args.c.requests, equateApproxTimepb(time.Second)...); diff != "" {
				t.Errorf("\nReason: %s\nr.RecordChangeLog(...): -want requests, +got requests:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nr.RecordChangeLog(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func mustObjectAsProtobufStruct(o runtime.Object) *structpb.Struct {
	s, err := resource.AsProtobufStruct(o)
	if err != nil {
		panic(err)
	}

	return s
}

// A set of cmp.Option that enables usage of cmpopts.EquateApproxTime for
// timestamppb.Timestamp types.
// Source: https://github.com/golang/protobuf/issues/1347
func equateApproxTimepb(margin time.Duration) []cmp.Option {
	return cmp.Options{
		cmpopts.EquateApproxTime(margin),
		protocmp.Transform(),
		cmp.FilterPath(
			func(p cmp.Path) bool {
				if p.Last().Type() == reflect.TypeFor[protocmp.Message]() {
					a, b := p.Last().Values()
					return msgIsTimestamp(a) && msgIsTimestamp(b)
				}

				return false
			},
			cmp.Transformer("timestamppb", func(t protocmp.Message) time.Time {
				return time.Unix(t["seconds"].(int64), int64(t["nanos"].(int32))).UTC()
			}),
		),
	}
}

func msgIsTimestamp(x reflect.Value) bool {
	return x.Interface().(protocmp.Message).Descriptor().FullName() == "google.protobuf.Timestamp"
}

/*
Copyright 2026 The Crossplane Authors.

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

package contextfn

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}
	return s
}

func inputStruct(t *testing.T, mode string) *structpb.Struct {
	t.Helper()
	b, err := json.Marshal(input{Mode: mode})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	s := &structpb.Struct{}
	if err := s.UnmarshalJSON(b); err != nil {
		t.Fatalf("unmarshal input struct: %v", err)
	}
	return s
}

func TestSeed(t *testing.T) {
	cases := map[string]struct {
		contextData map[string]any
		reqContext  map[string]any
		want        map[string]any
	}{
		"EmptyIncomingContext": {
			contextData: map[string]any{"k1": "v1", "nested": map[string]any{"a": 1.0}},
			reqContext:  nil,
			want:        map[string]any{"k1": "v1", "nested": map[string]any{"a": 1.0}},
		},
		"OverlaysExistingContext": {
			contextData: map[string]any{"override": "new"},
			reqContext:  map[string]any{"keep": "yes", "override": "old"},
			want:        map[string]any{"keep": "yes", "override": "new"},
		},
		"NilContextData": {
			contextData: nil,
			reqContext:  map[string]any{"keep": "yes"},
			want:        map[string]any{"keep": "yes"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newServer(tc.contextData)
			req := &fnv1.RunFunctionRequest{
				Meta:  &fnv1.RequestMeta{Tag: "tag"},
				Input: inputStruct(t, modeSeed),
			}
			if tc.reqContext != nil {
				req.Context = mustStruct(t, tc.reqContext)
			}

			rsp, err := s.RunFunction(context.Background(), req)
			if err != nil {
				t.Fatalf("RunFunction: %v", err)
			}
			if diff := cmp.Diff(mustStruct(t, tc.want), rsp.GetContext(), protocmp.Transform()); diff != "" {
				t.Errorf("context (-want +got):\n%s", diff)
			}
			if rsp.GetMeta().GetTag() != "tag" {
				t.Errorf("meta.tag: want %q, got %q", "tag", rsp.GetMeta().GetTag())
			}
		})
	}
}

func TestCapture(t *testing.T) {
	incoming := map[string]any{"foo": "bar", "n": 42.0}

	s := newServer(nil)
	req := &fnv1.RunFunctionRequest{
		Meta:    &fnv1.RequestMeta{Tag: "tag"},
		Input:   inputStruct(t, modeCapture),
		Context: mustStruct(t, incoming),
	}

	rsp, err := s.RunFunction(context.Background(), req)
	if err != nil {
		t.Fatalf("RunFunction: %v", err)
	}

	if rsp.GetContext() != nil {
		t.Errorf("capture should not forward context, got %v", rsp.GetContext())
	}
	if diff := cmp.Diff(mustStruct(t, incoming), s.capturedContext(), protocmp.Transform()); diff != "" {
		t.Errorf("captured (-want +got):\n%s", diff)
	}
}

func TestCaptureNilContext(t *testing.T) {
	s := newServer(nil)
	req := &fnv1.RunFunctionRequest{
		Meta:  &fnv1.RequestMeta{Tag: "tag"},
		Input: inputStruct(t, modeCapture),
	}

	if _, err := s.RunFunction(context.Background(), req); err != nil {
		t.Fatalf("RunFunction: %v", err)
	}
	if s.capturedContext() != nil {
		t.Errorf("captured: want nil, got %v", s.capturedContext())
	}
}

func TestUnknownMode(t *testing.T) {
	s := newServer(nil)
	req := &fnv1.RunFunctionRequest{
		Meta:  &fnv1.RequestMeta{Tag: "tag"},
		Input: inputStruct(t, "bogus"),
	}

	_, err := s.RunFunction(context.Background(), req)
	if err == nil {
		t.Fatal("want error for unknown mode")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("status code: want %v, got %v", codes.InvalidArgument, got)
	}
}

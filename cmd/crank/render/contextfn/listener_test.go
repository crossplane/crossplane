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
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

func TestListenerRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data := map[string]any{"k": "v"}
	h, err := Start(ctx, logging.NewNopLogger(), data)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer h.Stop()

	conn, err := grpc.NewClient(h.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	client := fnv1.NewFunctionRunnerServiceClient(conn)
	rsp, err := client.RunFunction(ctx, &fnv1.RunFunctionRequest{
		Meta:  &fnv1.RequestMeta{Tag: "tag"},
		Input: inputStruct(t, modeSeed),
	})
	if err != nil {
		t.Fatalf("RunFunction: %v", err)
	}

	if diff := cmp.Diff(mustStruct(t, data), rsp.GetContext(), protocmp.Transform()); diff != "" {
		t.Errorf("context (-want +got):\n%s", diff)
	}
}

func TestStopRemovesSocket(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h, err := Start(ctx, logging.NewNopLogger(), nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if _, err := os.Stat(h.socketPath); err != nil {
		t.Fatalf("socket should exist: %v", err)
	}

	h.Stop()

	if _, err := os.Stat(h.socketPath); !os.IsNotExist(err) {
		t.Errorf("socket should not exist after Stop, got err=%v", err)
	}

	// Second Stop is a no-op.
	h.Stop()
}

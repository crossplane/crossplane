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

package main

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
)

// Identity is baked into the binary at build time via
// -ldflags "-X main.Identity=<value>". Each published version of this function
// uses a different value ("revision-one", "revision-two", ...) so that a
// composite can observe which function revision actually served its request.
// This is what lets the e2e pinning test distinguish the function revision that
// handled a request from the composition revision that requested it.
var Identity = "unknown"

// Function writes the identity of the function revision that served a request
// into the composite's status.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
}

// RunFunction writes the function's build-time Identity to the composite's
// status.servedBy field.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	rsp := response.To(req, response.DefaultTTL)

	dxr, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, fmt.Errorf("cannot get desired composite resource: %w", err))
		return rsp, nil
	}

	if err := unstructured.SetNestedField(dxr.Resource.Object, Identity, "status", "servedBy"); err != nil {
		response.Fatal(rsp, fmt.Errorf("cannot set status.servedBy: %w", err))
		return rsp, nil
	}

	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, fmt.Errorf("cannot set desired composite resource: %w", err))
		return rsp, nil
	}

	response.Normalf(rsp, "served by function revision %q", Identity)
	return rsp, nil
}

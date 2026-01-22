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

// Package statemetrics contains utilities for recording Crossplane resource state metrics.
package statemetrics

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const subSystem = "crossplane"

// A StateRecorder records the state of given GroupVersionKind.
type StateRecorder interface {
	Record(ctx context.Context, gvk schema.GroupVersionKind)
	Start(ctx context.Context) error
}

// A NopStateRecorder does nothing.
type NopStateRecorder struct{}

// NewNopStateRecorder returns a NopStateRecorder that does nothing.
func NewNopStateRecorder() *NopStateRecorder {
	return &NopStateRecorder{}
}

// Record does nothing.
func (r *NopStateRecorder) Record(_ context.Context, _ schema.GroupVersionKind) {}

// Start does nothing.
func (r *NopStateRecorder) Start(_ context.Context) error { return nil }

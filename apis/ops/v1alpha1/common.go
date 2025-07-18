/*
Copyright 2025 The Crossplane Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConcurrencyPolicy specifies how to treat concurrent executions of an
// operation.
type ConcurrencyPolicy string

const (
	// ConcurrencyPolicyAllow allows concurrent executions.
	ConcurrencyPolicyAllow ConcurrencyPolicy = "Allow"

	// ConcurrencyPolicyForbid forbids concurrent executions, skipping the next
	// run if the previous run hasn't finished yet.
	ConcurrencyPolicyForbid ConcurrencyPolicy = "Forbid"

	// ConcurrencyPolicyReplace replaces the currently running operation with a
	// new one.
	ConcurrencyPolicyReplace ConcurrencyPolicy = "Replace"
)

// OperationTemplate is a template for creating an Operation.
type OperationTemplate struct {
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the Operation to be created.
	Spec OperationSpec `json:"spec"`
}

// A RunningOperationRef is a reference to a running operation.
type RunningOperationRef struct {
	// Name of the active operation.
	Name string `json:"name"`
}

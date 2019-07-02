/*
Copyright 2018 The Crossplane Authors.

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

// BindingPhase represents the current binding phase of a resource or claim.
type BindingPhase string

// Binding phases.
const (
	// BindingPhaseUnknown resources cannot be bound to another resource because
	// they are in an unknown binding phase.
	BindingPhaseUnknown BindingPhase = ""

	// BindingPhaseUnbindable resources cannot be bound to another resource, for
	// example because they are currently unavailable, or being created.
	BindingPhaseUnbindable BindingPhase = "Unbindable"

	// BindingPhaseUnbound resources are available for binding to another
	// resource.
	BindingPhaseUnbound BindingPhase = "Unbound"

	// BindingPhaseBound resources are bound to another resource.
	BindingPhaseBound BindingPhase = "Bound"
)

// A BindingStatus represents the bindability and binding of a resource.
type BindingStatus struct {
	// Phase represents the binding phase of the resource.
	// +kubebuilder:validation:Enum=Unbindable,Unbound,Bound
	Phase BindingPhase `json:"bindingPhase,omitempty"`
}

// SetBindingPhase sets the binding phase of the resource.
func (s *BindingStatus) SetBindingPhase(p BindingPhase) {
	s.Phase = p
}

// GetBindingPhase gets the binding phase of the resource.
func (s *BindingStatus) GetBindingPhase() BindingPhase {
	return s.Phase
}

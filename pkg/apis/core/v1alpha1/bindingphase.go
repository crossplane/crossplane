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

// BindingState is to identify the current binding status of given resources
type BindingState string

const (
	// BindingStateBound signifies that resource has one ore more active bindings
	BindingStateBound BindingState = "Bound"

	// BindingStateUnbound signifies that the resource has no active bindings
	// Default BindingState
	BindingStateUnbound BindingState = "Unbound"
)

// BindingStatus defines set of supported operations
type BindingStatus interface {
	SetBound()
	SetUnbound()
	IsBound() bool
	IsUnbound() bool
}

type BindingStatusObject interface {
	BindingStatus() BindingStatus
}

// BindingStatusPhase defines field(s) representing resource status
type BindingStatusPhase struct {
	Phase BindingState `json:"bindingPhase,omitempty"` // binding status of the instance
}

// SetBound set binding status to Bound
func (b *BindingStatusPhase) SetBound() {
	b.Phase = BindingStateBound
}

// SetUnbound set binding status to Unbound
func (b *BindingStatusPhase) SetUnbound() {
	b.Phase = BindingStateUnbound
}

// IsBound returns true if status is bound
func (b *BindingStatusPhase) IsBound() bool {
	return b.Phase == BindingStateBound
}

// IsUnbound returns true if status is unbound
func (b *BindingStatusPhase) IsUnbound() bool {
	return b.Phase == BindingStateUnbound
}

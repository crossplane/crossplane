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

//go:generate go run ../../../../vendor/golang.org/x/tools/cmd/stringer/stringer.go ../../../../vendor/golang.org/x/tools/cmd/stringer/importer19.go -type=BindingState -trimprefix BindingState

package v1alpha1

import "encoding/json"

// BindingState is to identify the current binding status of given resources
type BindingState int

// MarshalJSON returns a JSON representation of a BindingState.
func (s BindingState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// Binding states.
const (
	BindingStateUnbound BindingState = iota
	BindingStateBound
)

// BindingStatus defines set of supported operations
type BindingStatus interface {
	SetBound(bound bool)
	IsBound() bool
}

// BindingStatusPhase defines field(s) representing resource status.
type BindingStatusPhase struct {
	// Phase represents the binding status of a resource.
	Phase BindingState `json:"bindingPhase,omitempty"`
}

// SetBound set binding status to Bound
func (b *BindingStatusPhase) SetBound(bound bool) {
	if bound {
		b.Phase = BindingStateBound
		return
	}
	b.Phase = BindingStateUnbound
}

// IsBound returns true if status is bound
func (b *BindingStatusPhase) IsBound() bool {
	return b.Phase == BindingStateBound
}

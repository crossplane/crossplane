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

//go:generate go run ../../../../vendor/golang.org/x/tools/cmd/stringer/stringer.go ../../../../vendor/golang.org/x/tools/cmd/stringer/importer19.go -type=BindingPhase -trimprefix BindingPhase

package v1alpha1

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// BindingPhase is to identify the current binding status of given resources
type BindingPhase int

// MarshalJSON returns a JSON representation of a BindingPhase.
func (p BindingPhase) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshalJSON returns a BindingPhase from its JSON representation.
func (p *BindingPhase) UnmarshalJSON(b []byte) error {
	var ps string
	if err := json.Unmarshal(b, &ps); err != nil {
		return err
	}
	switch ps {
	case BindingPhaseUnbindable.String():
		*p = BindingPhaseUnbindable
	case BindingPhaseUnbound.String():
		*p = BindingPhaseUnbound
	case BindingPhaseBound.String():
		*p = BindingPhaseBound
	default:
		return errors.Errorf("unknown binding state %s", ps)
	}
	return nil
}

// Binding phases.
const (
	// BindingPhaseUnbindable resources cannot be bound to another resource, for
	// example because they are currently unavailable, or being created.
	BindingPhaseUnbindable BindingPhase = iota

	// BindingPhaseUnbound resources are available for binding to another
	// resource.
	BindingPhaseUnbound

	// BindingPhaseBound resources are bound to another resource.
	BindingPhaseBound
)

// A BindingStatus represents the bindability and binding of a resource.
type BindingStatus struct {
	// Phase represents the binding phase of the resource.
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

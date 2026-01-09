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

package common

// ObservedStatus contains the recent reconciliation stats.
type ObservedStatus struct {
	// ObservedGeneration is the latest metadata.generation
	// which resulted in either a ready state, or stalled due to error
	// it can not recover from without human intervention.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// SetObservedGeneration sets the generation of the main resource
// during the last reconciliation.
func (s *ObservedStatus) SetObservedGeneration(generation int64) {
	s.ObservedGeneration = generation
}

// GetObservedGeneration returns the last observed generation of the main resource.
func (s *ObservedStatus) GetObservedGeneration() int64 {
	return s.ObservedGeneration
}

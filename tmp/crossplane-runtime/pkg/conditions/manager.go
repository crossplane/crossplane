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

// Package conditions enables consistent interactions with an object's status conditions.
package conditions

import (
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// ObjectWithConditions is the interface definition that allows.
type ObjectWithConditions interface {
	resource.Object
	resource.Conditioned
}

// Manager is an interface for a stateless factory-like object that produces ConditionSet objects.
type Manager interface {
	// For returns an implementation of a ConditionSet to operate on a specific ObjectWithConditions.
	For(o ObjectWithConditions) ConditionSet
}

// ConditionSet holds operations for interacting with an object's conditions.
type ConditionSet interface {
	// MarkConditions adds or updates the conditions onto the managed resource object. Unlike a "Set" method, this also
	// can add contextual updates to the condition such as propagating the correct observedGeneration to the conditions
	// being changed.
	MarkConditions(condition ...xpv1.Condition)
}

// ObservedGenerationPropagationManager is the top level factor for producing a ConditionSet
// on behalf of a ObjectWithConditions resource, the ConditionSet is only currently concerned with
// propagating observedGeneration to conditions that are being updated.
// observedGenerationPropagationManager implements Manager.
type ObservedGenerationPropagationManager struct{}

// For implements Manager.For.
func (m ObservedGenerationPropagationManager) For(o ObjectWithConditions) ConditionSet {
	return &observedGenerationPropagationConditionSet{o: o}
}

// observedGenerationPropagationConditionSet propagates the meta.generation of the given object
// to the observedGeneration of any condition being set via the `MarkConditions` method.
type observedGenerationPropagationConditionSet struct {
	o ObjectWithConditions
}

// MarkConditions implements ConditionSet.MarkConditions.
func (c *observedGenerationPropagationConditionSet) MarkConditions(condition ...xpv1.Condition) {
	if c == nil || c.o == nil {
		return
	}
	// Foreach condition we have been sent to mark, update the observed generation.
	for i := range condition {
		condition[i].ObservedGeneration = c.o.GetGeneration()
	}

	c.o.SetConditions(condition...)
}

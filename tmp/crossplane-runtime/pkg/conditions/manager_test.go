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

package conditions

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

// Check that conditionsImpl implements ConditionManager.
var _ Manager = (*ObservedGenerationPropagationManager)(nil)

// Check that conditionSet implements ConditionSet.
var _ ConditionSet = (*observedGenerationPropagationConditionSet)(nil)

func TestOGConditionSetMark(t *testing.T) {
	manager := new(ObservedGenerationPropagationManager)

	tests := map[string]struct {
		reason string
		start  []xpv1.Condition
		mark   []xpv1.Condition
		want   []xpv1.Condition
	}{
		"ProvideNoConditions": {
			reason: "If updating a resource without conditions with no new conditions, conditions should remain empty.",
			start:  nil,
			mark:   nil,
			want:   nil,
		},
		"EmptyAppendCondition": {
			reason: "If starting with a resource without conditions, and we mark a condition, it should propagate to conditions with the correct generation.",
			start:  nil,
			mark:   []xpv1.Condition{xpv1.ReconcileSuccess()},
			want:   []xpv1.Condition{xpv1.ReconcileSuccess().WithObservedGeneration(42)},
		},
		"ExistingMarkNothing": {
			reason: "If the resource has a condition and we update nothing, nothing should change.",
			start:  []xpv1.Condition{xpv1.Available().WithObservedGeneration(1)},
			mark:   nil,
			want:   []xpv1.Condition{xpv1.Available().WithObservedGeneration(1)},
		},
		"ExistingUpdated": {
			reason: "If a resource starts with a condition, and we update it, we should see the observedGeneration be updated",
			start:  []xpv1.Condition{xpv1.ReconcileSuccess().WithObservedGeneration(1)},
			mark:   []xpv1.Condition{xpv1.ReconcileSuccess()},
			want:   []xpv1.Condition{xpv1.ReconcileSuccess().WithObservedGeneration(42)},
		},
		"ExistingAppended": {
			reason: "If a resource has an existing condition and we make another condition, the new condition should merge into the conditions list.",
			start:  []xpv1.Condition{xpv1.Available().WithObservedGeneration(1)},
			mark:   []xpv1.Condition{xpv1.ReconcileSuccess()},
			want:   []xpv1.Condition{xpv1.Available().WithObservedGeneration(1), xpv1.ReconcileSuccess().WithObservedGeneration(42)},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ut := newManaged(42, tt.start...)
			c := manager.For(ut)
			c.MarkConditions(tt.mark...)

			if diff := cmp.Diff(tt.want, ut.Conditions, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
				t.Errorf("\nReason: %s\n-want, +got:\n%s", tt.reason, diff)
			}
		})
	}

	t.Run("ManageNilObject", func(t *testing.T) {
		c := manager.For(nil)
		if c == nil {
			t.Errorf("manager.For(nil) = %v, want non-nil", c)
		}
		// Test that Marking on a Manager that has a nil object does not end up panicking.
		c.MarkConditions(xpv1.ReconcileSuccess())
		// Success!
	})
}

func TestOGManagerFor(t *testing.T) {
	tests := map[string]struct {
		reason string
		o      ObjectWithConditions
		want   ConditionSet
	}{
		"NilObject": {
			reason: "Even if an object is nil, the manager should return a non-nil ConditionSet",
			want:   &observedGenerationPropagationConditionSet{},
		},
		"Object": {
			reason: "Object propagates into manager.",
			o:      &fake.Managed{},
			want: &observedGenerationPropagationConditionSet{
				o: &fake.Managed{},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			m := &ObservedGenerationPropagationManager{}
			if got := m.For(tt.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nReason: %s\nFor() = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}

func newManaged(generation int64, conditions ...xpv1.Condition) *fake.Managed {
	mg := &fake.Managed{}
	mg.Generation = generation
	mg.SetConditions(conditions...)

	return mg
}

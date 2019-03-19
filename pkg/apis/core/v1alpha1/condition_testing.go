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

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// AssertConditions verifies the given expected conditions against the given actual conditions.
// This is helpful for unit tests since the Condition type has a timestamp that makes full
// object equal comparisons difficult.
// TODO: consider to use ConditionMatcher or ConditionedStatusMatcher instead
func AssertConditions(g *gomega.GomegaWithT, expected []Condition, actual ConditionedStatus) {
	for _, ec := range expected {
		// find the condition of the matching type, it should exist
		ac := actual.Condition(ec.Type)
		g.Expect(ac).NotTo(gomega.BeNil())
		g.Expect(*ac).To(MatchCondition(ec))
	}
}

// ConditionMatcher is a gomega matcher for Conditions.
// +k8s:deepcopy-gen=false
type ConditionMatcher struct {
	expected interface{}
}

// Match returns true if the underlying condition matches the supplied one.
func (cm *ConditionMatcher) Match(actual interface{}) (success bool, err error) {
	e, ok := cm.expected.(Condition)
	if !ok {
		return false, fmt.Errorf("expected value is not a Condition: %v", cm.expected)
	}
	a, ok := actual.(Condition)
	if !ok {
		return false, fmt.Errorf("actual value is not a Condition: %v", actual)
	}

	return e.Equal(a), nil
}

// FailureMessage is printed when conditions do not match.
func (cm *ConditionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto match, actual\n\t%#v", cm.expected, actual)
}

// NegatedFailureMessage is printed when conditions match unexpectedly.
func (cm *ConditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to match, actual\n\t%#v", cm.expected, actual)
}

// MatchCondition returns a new gomga matcher for Conditions.
func MatchCondition(expected interface{}) types.GomegaMatcher {
	return &ConditionMatcher{
		expected: expected,
	}
}

// ConditionedStatusMatcher is a gomega matcher for ConditionedStatuses.
// +k8s:deepcopy-gen=false
type ConditionedStatusMatcher struct {
	expected interface{}
}

// Match returns true if the underlying conditioned status matches the supplied
// one.
func (csm *ConditionedStatusMatcher) Match(actual interface{}) (success bool, err error) {
	e, ok := csm.expected.(ConditionedStatus)
	if !ok {
		return false, fmt.Errorf("expected value is not a ConditionedStatus: %v", csm.expected)
	}
	a, ok := actual.(ConditionedStatus)
	if !ok {
		return false, fmt.Errorf("actual value is not a ConditionedStatus: %v", actual)
	}

	if len(e.Conditions) != len(a.Conditions) {
		return false, nil
	}

	for _, ce := range e.Conditions {
		ca := a.Condition(ce.Type)
		if ca == nil {
			return false, nil
		}
		cm := &ConditionMatcher{ce}
		ok, err := cm.Match(*ca)
		if !ok {
			return false, err
		}
	}

	return true, nil
}

// FailureMessage is printed when conditioned statuses do not match.
func (csm *ConditionedStatusMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto mach, actual\n\t%#v", csm.expected, actual)
}

// NegatedFailureMessage is printed when conditioned statuses match
// unexpectedly.
func (csm *ConditionedStatusMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to mach, actual\n\t%#v", csm.expected, actual)
}

// MatchConditionedStatus returns a new gomega matcher for conditioned statuses.
func MatchConditionedStatus(expected interface{}) types.GomegaMatcher {
	return &ConditionedStatusMatcher{
		expected: expected,
	}
}

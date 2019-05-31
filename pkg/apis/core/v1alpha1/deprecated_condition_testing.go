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
// This is helpful for unit tests since the DeprecatedCondition type has a timestamp that makes full
// object equal comparisons difficult.
// TODO: consider to use DeprecatedConditionMatcher or DeprecatedConditionedStatusMatcher instead
func AssertConditions(g *gomega.GomegaWithT, expected []DeprecatedCondition, actual DeprecatedConditionedStatus) {
	for _, ec := range expected {
		// find the condition of the matching type, it should exist
		ac := actual.DeprecatedCondition(ec.Type)
		g.Expect(ac).NotTo(gomega.BeNil())
		g.Expect(*ac).To(MatchDeprecatedCondition(ec))
	}
}

// DeprecatedConditionMatcher is a gomega matcher for Conditions.
// +k8s:deepcopy-gen=false
type DeprecatedConditionMatcher struct {
	expected interface{}
}

// Match returns true if the underlying condition matches the supplied one.
func (cm *DeprecatedConditionMatcher) Match(actual interface{}) (success bool, err error) {
	e, ok := cm.expected.(DeprecatedCondition)
	if !ok {
		return false, fmt.Errorf("expected value is not a DeprecatedCondition: %v", cm.expected)
	}
	a, ok := actual.(DeprecatedCondition)
	if !ok {
		return false, fmt.Errorf("actual value is not a DeprecatedCondition: %v", actual)
	}

	return e.Equal(a), nil
}

// FailureMessage is printed when conditions do not match.
func (cm *DeprecatedConditionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto match, actual\n\t%#v", cm.expected, actual)
}

// NegatedFailureMessage is printed when conditions match unexpectedly.
func (cm *DeprecatedConditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to match, actual\n\t%#v", cm.expected, actual)
}

// MatchDeprecatedCondition returns a new gomga matcher for Conditions.
func MatchDeprecatedCondition(expected interface{}) types.GomegaMatcher {
	return &DeprecatedConditionMatcher{
		expected: expected,
	}
}

// DeprecatedConditionedStatusMatcher is a gomega matcher for DeprecatedConditionedStatuses.
// +k8s:deepcopy-gen=false
type DeprecatedConditionedStatusMatcher struct {
	expected interface{}
}

// Match returns true if the underlying conditioned status matches the supplied
// one.
func (csm *DeprecatedConditionedStatusMatcher) Match(actual interface{}) (success bool, err error) {
	e, ok := csm.expected.(DeprecatedConditionedStatus)
	if !ok {
		return false, fmt.Errorf("expected value is not a DeprecatedConditionedStatus: %v", csm.expected)
	}
	a, ok := actual.(DeprecatedConditionedStatus)
	if !ok {
		return false, fmt.Errorf("actual value is not a DeprecatedConditionedStatus: %v", actual)
	}

	if len(e.Conditions) != len(a.Conditions) {
		return false, nil
	}

	for _, ce := range e.Conditions {
		ca := a.DeprecatedCondition(ce.Type)
		if ca == nil {
			return false, nil
		}
		cm := &DeprecatedConditionMatcher{ce}
		ok, err := cm.Match(*ca)
		if !ok {
			return false, err
		}
	}

	return true, nil
}

// FailureMessage is printed when conditioned statuses do not match.
func (csm *DeprecatedConditionedStatusMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto mach, actual\n\t%#v", csm.expected, actual)
}

// NegatedFailureMessage is printed when conditioned statuses match
// unexpectedly.
func (csm *DeprecatedConditionedStatusMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to mach, actual\n\t%#v", csm.expected, actual)
}

// MatchDeprecatedConditionedStatus returns a new gomega matcher for conditioned statuses.
func MatchDeprecatedConditionedStatus(expected interface{}) types.GomegaMatcher {
	return &DeprecatedConditionedStatusMatcher{
		expected: expected,
	}
}

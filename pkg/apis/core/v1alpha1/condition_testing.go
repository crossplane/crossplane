/*
Copyright 2018 The Conductor Authors.

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

// ConditionMatcher
// +k8s:deepcopy-gen=false
type ConditionMatcher struct {
	expected interface{}
}

func (cm *ConditionMatcher) Match(actual interface{}) (success bool, err error) {
	e, ok := cm.expected.(Condition)
	if !ok {
		return false, fmt.Errorf("expected value is not a Condition: %v", cm.expected)
	}
	a, ok := actual.(Condition)
	if !ok {
		return false, fmt.Errorf("actual value is not a Condition: %v", actual)
	}

	return e.Type == a.Type &&
		e.Status == a.Status &&
		e.Reason == a.Reason &&
		e.Message == a.Message, nil
}

func (cm *ConditionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto mach, actual\n\t%#v", cm.expected, actual)
}

func (cm *ConditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to mach, actual\n\t%#v", cm.expected, actual)
}

func MatchCondition(expected interface{}) types.GomegaMatcher {
	return &ConditionMatcher{
		expected: expected,
	}
}

// ConditionedStatusMatcher
// +k8s:deepcopy-gen=false
type ConditionedStatusMatcher struct {
	expected interface{}
}

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

func (csm *ConditionedStatusMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto mach, actual\n\t%#v", csm.expected, actual)
}

func (csm *ConditionedStatusMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to mach, actual\n\t%#v", csm.expected, actual)
}

func MatchConditionedStatus(expected interface{}) types.GomegaMatcher {
	return &ConditionedStatusMatcher{
		expected: expected,
	}
}

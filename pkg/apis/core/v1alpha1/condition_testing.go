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

import "github.com/onsi/gomega"

// AssertConditions verifies the given expected conditions against the given actual conditions.
// This is helpful for unit tests since the Condition type has a timestamp that makes full
// object equal comparisons difficult.
func AssertConditions(g *gomega.GomegaWithT, expected []Condition, actual ConditionedStatus) {
	for _, ec := range expected {
		// find the condition of the matching type, it should exist
		ac := actual.Condition(ec.Type)
		g.Expect(ac).NotTo(gomega.BeNil())

		// compare the individual properties of the actual condition. note that we skip timestamp here since
		// that will be different on every test run.
		g.Expect(ac.Type).To(gomega.Equal(ec.Type))
		g.Expect(ac.Status).To(gomega.Equal(ec.Status))
		g.Expect(ac.Reason).To(gomega.Equal(ec.Reason))
		g.Expect(ac.Message).To(gomega.Equal(ec.Message))
	}
}

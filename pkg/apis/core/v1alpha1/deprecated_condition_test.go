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
	"testing"

	. "github.com/onsi/gomega"
)

func TestDeprecatedConditionedStatus_UnsetAllDeprecatedConditions(t *testing.T) {
	g := NewGomegaWithT(t)

	cs := &DeprecatedConditionedStatus{}
	cs.SetReady()
	g.Expect(cs.IsReady()).To(BeTrue())

	cs.UnsetAllDeprecatedConditions()
	g.Expect(cs.IsReady()).To(BeFalse())

	cs.SetFailed("foo", "bar")
	g.Expect(cs.IsFailed()).To(BeTrue())
	g.Expect(cs.IsReady()).To(BeFalse())

	cs.UnsetAllDeprecatedConditions()
	cs.SetReady()
	g.Expect(cs.IsFailed()).To(BeFalse())
	g.Expect(cs.IsReady()).To(BeTrue())
}

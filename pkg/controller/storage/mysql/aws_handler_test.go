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

package mysql

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestValidEngineVersion(t *testing.T) {
	g := NewGomegaWithT(t)

	valid := func(class, instance, expected string) {
		v, err := validateEngineVersion(class, instance)
		g.Expect(v).To(Equal(expected))
		g.Expect(err).NotTo(HaveOccurred())
	}
	valid("", "", "")
	valid("5.6", "", "5.6")
	valid("", "5.7", "5.7")
	valid("5.6.45", "5.6", "5.6.45")

	v, err := validateEngineVersion("5.6", "5.7")
	g.Expect(v).To(BeEmpty())
	g.Expect(err).To(And(HaveOccurred(), MatchError("invalid class: [5.6], instance: [5.7] values combination")))
}

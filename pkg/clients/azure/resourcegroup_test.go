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

package azure

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestResourceGroupName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		input       string
		expectedErr string
	}{
		// name is OK, should not be modified
		{"foo", ""},
		// name ends with a period, should not be allowed
		{"foo.", "name of resource group may not end in a period"},
		// longer than 90 characters, chould not be allowed
		{"resource-group-name-S2Ixh9w8DmsW0oMwVv4oXbC9Lv3Sn2ARwjp86fwSpb3GOmdFqVZy4la7qwO1OrGbn9uDOEzU2oL01oG4", "name of resource group may not be longer than 90 characters"},
		// shorter than 1 character, chould not be allowed
		{"", "name of resource group must be at least one character"},
	}

	for _, tt := range cases {
		err := CheckResourceGroupName(tt.input)
		if tt.expectedErr != "" {
			g.Expect(err).To(gomega.HaveOccurred())
			g.Expect(err.Error()).To(gomega.Equal(tt.expectedErr))
		} else {
			g.Expect(err).NotTo(gomega.HaveOccurred())
		}
	}
}

/*
Copyright 2019 The Crossplane Authors.

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

func TestSanitizeClusterName(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		input    string
		expected string
	}{
		// name is OK, should not be modified
		{"foo", "foo"},
		// name too long, should be truncated down to max length
		{"aks-ca60851e-168b-4cee-b3e3-3cc4bb031103", "aks-ca60851e-168b-4cee-b3e3-3cc"},
		// truncated length would result in a trailing hyphen, it should also be removed
		{"aks-ca60851e-168b-4cee-b3e3-3c--", "aks-ca60851e-168b-4cee-b3e3-3c"},
	}

	for _, tt := range cases {
		actual := SanitizeClusterName(tt.input)
		g.Expect(actual).To(gomega.Equal(tt.expected))
	}
}

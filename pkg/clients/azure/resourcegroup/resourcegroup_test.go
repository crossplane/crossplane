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

package resourcegroup

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/google/go-cmp/cmp"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

const (
	name     = "cool-rg"
	location = "us-west-1"
)

func TestNewParameters(t *testing.T) {
	cases := []struct {
		name string
		r    *v1alpha1.ResourceGroup
		want resources.Group
	}{
		{
			name: "Successful",
			r: &v1alpha1.ResourceGroup{
				Spec: v1alpha1.ResourceGroupSpec{
					Name:     name,
					Location: location,
				},
			},
			want: resources.Group{
				Name:     azure.ToStringPtr(name),
				Location: azure.ToStringPtr(location),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewParameters(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

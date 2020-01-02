/*
Copyright 2020 The Crossplane Authors.

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

	"github.com/google/go-cmp/cmp"
)

var (
	// verify that StackInstall and ClusterStackInstall implement StackInstaller
	_ StackInstaller = &StackInstall{}
	_ StackInstaller = &ClusterStackInstall{}
)

func TestStackInstallSpec_Image(t *testing.T) {
	tests := []struct {
		name string
		spec StackInstallSpec
		want string
	}{
		{
			name: "NoPackageSource",
			spec: StackInstallSpec{
				Package: "cool/package:rad",
			},
			want: "cool/package:rad",
		},
		{
			name: "PackageSourceSpecified",
			spec: StackInstallSpec{
				Source:  "registry.hub.docker.com",
				Package: "cool/package:rad",
			},
			want: "registry.hub.docker.com/cool/package:rad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.Image()

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Image() -want, +got:\n%v", diff)
			}
		})
	}
}

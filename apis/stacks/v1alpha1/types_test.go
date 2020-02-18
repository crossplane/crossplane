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

func TestStackInstallSpec_ImageWithSource(t *testing.T) {
	type want struct {
		url string
	}

	tests := []struct {
		name   string
		reason string
		spec   StackInstallSpec
		want   want
	}{
		{
			name:   "NoPackageSource",
			reason: "Packages without StackInstall source should be unchanged",
			spec: StackInstallSpec{
				Package: "cool/package:rad",
			},
			want: want{
				url: "cool/package:rad",
			},
		},
		{
			name:   "PackageSourceSpecified",
			reason: "Sourceless packages should inherit StackInstall source",
			spec: StackInstallSpec{
				Source:  "foo.bar",
				Package: "cool/package:rad",
			},
			want: want{
				url: "foo.bar/cool/package:rad",
			},
		},
		{
			name:   "DockerSourceSpecified",
			reason: "Docker hub sources should not have special treatment",
			spec: StackInstallSpec{
				Source:  "registry.hub.docker.com",
				Package: "cool/package:rad",
			},
			want: want{
				url: "registry.hub.docker.com/cool/package:rad",
			},
		},
		{
			name:   "SourceWithProtocol",
			reason: "Sources should honor host, port, and prefix",
			spec: StackInstallSpec{
				Source:  "insecure:3000/prefix/",
				Package: "cool/tagless-package",
			},
			want: want{
				url: "insecure:3000/prefix/cool/tagless-package",
			},
		},
		{
			name:   "InvalidSource",
			reason: "Invalid sources should not deter ImageWithSource",
			spec: StackInstallSpec{
				Source:  "bad:host:and:port",
				Package: "cool/tagless-package",
			},
			want: want{
				url: "bad:host:and:port/cool/tagless-package",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.Image()

			if diff := cmp.Diff(tt.want.url, got); diff != "" {
				t.Errorf("Reason: %s\n-want, +got:\n%v", tt.reason, diff)
			}
		})
	}
}

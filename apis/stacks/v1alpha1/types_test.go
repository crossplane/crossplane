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
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var (
	// verify that StackInstall and ClusterStackInstall implement StackInstaller
	_ StackInstaller = &StackInstall{}
	_ StackInstaller = &ClusterStackInstall{}
)

func TestStackInstallSpec_Image(t *testing.T) {
	type want struct {
		url string
		err error
	}

	tests := []struct {
		name string
		spec StackInstallSpec
		want want
	}{
		{
			name: "NoPackageSource",
			spec: StackInstallSpec{
				Package: "cool/package:rad",
			},
			want: want{
				url: "cool/package:rad",
			},
		},
		{
			name: "PackageSourceSpecified",
			spec: StackInstallSpec{
				Source:  "registry.hub.docker.com",
				Package: "cool/package:rad",
			},
			want: want{
				url: "registry.hub.docker.com/cool/package:rad",
			},
		},
		{
			name: "SourceWithProtocol",
			spec: StackInstallSpec{
				Source:  "http://insecure:3000/prefix/",
				Package: "cool/tagless-package",
			},
			want: want{
				url: "http://insecure:3000/prefix/cool/tagless-package",
			},
		},
		{
			name: "InvalidSource",
			spec: StackInstallSpec{
				Source:  "http://bad:host:and:port",
				Package: "cool/tagless-package",
			},
			want: want{
				err: errors.Wrap(errors.New("parse http://bad:host:and:port: invalid port \":port\" after host"), "failed to parse source"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.spec.ImageStr()

			if diff := cmp.Diff(tt.want.url, got); diff != "" {
				t.Errorf("Image() url -want, +got:\n%v", diff)
			}

			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Image() err -want, +got:\n%v", diff)
			}

		})
	}
}

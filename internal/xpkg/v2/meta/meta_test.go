/*
Copyright 2023 The Crossplane Authors.

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

package meta

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
)

func TestConfigTemplate(t *testing.T) {
	providerAws := "crossplane/provider-aws"
	providerGcp := "crossplane/provider-gcp"
	providerAwsVer := ">=v0.14.0"
	providerGcpVer := ">=v0.15.0"

	cases := map[string]struct {
		reason string
		ctx    xpkg.InitContext
		want   []byte
		err    error
	}{
		"NameProvided": {
			reason: "We should return a Configuration with just name filled in.",
			ctx: xpkg.InitContext{
				Name: "test",
			},
			want: []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: test
spec: {}
`),
			err: nil,
		},
		"NameNotProvided": {
			reason: "We should return an error if name not provided.",
			ctx:    xpkg.InitContext{},
			want:   nil,
			err:    errors.New(errXPkgNameNotProvided),
		},
		"CrossplaneVersionProvidedNoDependsOn": {
			reason: "We should return a Configuration with crossplane version filled, without dependsOn slice.",
			ctx: xpkg.InitContext{
				Name:      "test",
				XPVersion: ">=v1.0.0-1",
			},
			want: []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: test
spec:
  crossplane:
    version: '>=v1.0.0-1'
`),
			err: nil,
		},
		"DependsOnProvidedNoCrossplaneVersion": {
			reason: "We should return a Configuration without crossplane version filled, but with dependsOn slice.",
			ctx: xpkg.InitContext{
				Name: "test",
				DependsOn: []v1.Dependency{
					{
						Provider: &providerAws,
						Version:  providerAwsVer,
					},
				},
			},
			want: []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: test
spec:
  dependsOn:
  - provider: crossplane/provider-aws
    version: '>=v0.14.0'
`),
			err: nil,
		},
		"MultipleDependsOnProvidedNoCrossplaneVersion": {
			reason: "We should return a Configuration without crossplane version filled and multiple dependsOn versions.",
			ctx: xpkg.InitContext{
				Name: "test",
				DependsOn: []v1.Dependency{
					{
						Provider: &providerAws,
						Version:  providerAwsVer,
					},
					{
						Provider: &providerGcp,
						Version:  providerGcpVer,
					},
				},
			},
			want: []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: test
spec:
  dependsOn:
  - provider: crossplane/provider-aws
    version: '>=v0.14.0'
  - provider: crossplane/provider-gcp
    version: '>=v0.15.0'
`),
			err: nil,
		},
		"MultipleDependsOnProvidedCrossplaneVersionProvided": {
			reason: "We should return a Configuration with crossplane version filled and all dependsOn versions.",
			ctx: xpkg.InitContext{
				Name:      "test",
				XPVersion: ">=v1.0.0-1",
				DependsOn: []v1.Dependency{
					{
						Provider: &providerAws,
						Version:  providerAwsVer,
					},
					{
						Provider: &providerGcp,
						Version:  providerGcpVer,
					},
				},
			},
			want: []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: test
spec:
  crossplane:
    version: '>=v1.0.0-1'
  dependsOn:
  - provider: crossplane/provider-aws
    version: '>=v0.14.0'
  - provider: crossplane/provider-gcp
    version: '>=v0.15.0'
`),
			err: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := NewConfigXPkg(tc.ctx)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewConfigXPkg(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nNewConfigXPkg(...): -want, +got:\n%s", "equal", diff)
			}
		})
	}
}

func TestProviderTemplate(t *testing.T) {
	cases := map[string]struct {
		reason string
		ctx    xpkg.InitContext
		want   []byte
		err    error
	}{
		"NameProvidedNoControllerImage": {
			reason: "We should return an error as controller image is required.",
			ctx: xpkg.InitContext{
				Name: "test",
			},
			want: nil,
			err:  errors.New(errCtrlImageNotProvided),
		},
		"ControllerImageNoName": {
			reason: "We should return an error as name is required.",
			ctx: xpkg.InitContext{
				Image: "docker.io/provider-aws-controller:v1.0.0",
			},
			want: nil,
			err:  errors.New(errXPkgNameNotProvided),
		},
		"NameAndControllerImage": {
			reason: "We should return a Provider with name and controller image filled in.",
			ctx: xpkg.InitContext{
				Name:  "test",
				Image: "docker.io/provider-aws-controller:v1.0.0",
			},
			want: []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: test
spec:
  controller:
    image: docker.io/provider-aws-controller:v1.0.0
`),
		},
		"NameAndControllerImageAndCrossplaneConstraint": {
			reason: "We should return a Provider with name, controller image, and crossplane constraint filled in.",
			ctx: xpkg.InitContext{
				Name:      "test",
				XPVersion: ">=1.0.1-0",
				Image:     "docker.io/provider-aws-controller:v1.0.0",
			},
			want: []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: test
spec:
  controller:
    image: docker.io/provider-aws-controller:v1.0.0
  crossplane:
    version: '>=1.0.1-0'
`),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := NewProviderXPkg(tc.ctx)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewProviderXPkg(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nNewProviderXPkg(...): -want, +got:\n%s", "equal", diff)
			}
		})
	}
}

/*
Copyright 2024 The Crossplane Authors.

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

package xpkg

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func TestIsPackageType(t *testing.T) {
	type args struct {
		gk schema.GroupKind
	}

	type want struct {
		ok bool
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"V1ProviderOK": {
			reason: "Should return true for a v1 Provider",
			args: args{
				gk: v1.ProviderGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"V1ConfigurationOK": {
			reason: "Should return true for a v1 Configuration",
			args: args{
				gk: v1.ConfigurationGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"V1beta1FunctionOK": {
			reason: "Should return true for a v1beta1 Function",
			args: args{
				gk: v1beta1.FunctionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"V1ProviderRevisionKO": {
			reason: "Should return false for a v1 ProviderRevision",
			args: args{
				gk: v1.ProviderRevisionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: false,
			},
		},
		"V1ConfigurationRevisionKO": {
			reason: "Should return false for a v1 ConfigurationRevision",
			args: args{
				gk: v1.ConfigurationRevisionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: false,
			},
		},
		"V1beta1FunctionRevisionKO": {
			reason: "Should return false for a v1beta1 FunctionRevision",
			args: args{
				gk: v1beta1.FunctionRevisionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: false,
			},
		},
		"ElseKO": {
			reason: "Should return false for a random GK",
			args: args{
				gk: schema.GroupKind{
					Group: "foo",
					Kind:  "bar",
				},
			},
			want: want{
				ok: false,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := IsPackageType(tc.args.gk)
			if got != tc.want.ok {
				t.Errorf("%s\nIsPackageType() = %v, want %v", tc.reason, got, tc.want.ok)
			}
		})
	}
}

func TestIsPackageRevisionType(t *testing.T) {
	type args struct {
		gk schema.GroupKind
	}

	type want struct {
		ok bool
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"V1ProviderKO": {
			reason: "Should return false for a v1 Provider",
			args: args{
				gk: v1.ProviderGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: false,
			},
		},
		"V1ConfigurationKO": {
			reason: "Should return false for a v1 Configuration",
			args: args{
				gk: v1.ConfigurationGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: false,
			},
		},
		"V1beta1FunctionKO": {
			reason: "Should return false for a v1beta1 Function",
			args: args{
				gk: v1beta1.FunctionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: false,
			},
		},
		"V1ProviderRevisionOK": {
			reason: "Should return true for a v1 ProviderRevision",
			args: args{
				gk: v1.ProviderRevisionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"V1ConfigurationRevisionOK": {
			reason: "Should return true for a v1 ConfigurationRevision",
			args: args{
				gk: v1.ConfigurationRevisionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"V1beta1FunctionRevisionOK": {
			reason: "Should return true for a v1beta1 FunctionRevision",
			args: args{
				gk: v1beta1.FunctionRevisionGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"ElseKO": {
			reason: "Should return false for a random GK",
			args: args{
				gk: schema.GroupKind{
					Group: "foo",
					Kind:  "bar",
				},
			},
			want: want{
				ok: false,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := IsPackageRevisionType(tc.args.gk)
			if got != tc.want.ok {
				t.Errorf("%s\nIsPackageRevisionType() = %v, want %v", tc.reason, got, tc.want.ok)
			}
		})
	}
}

func TestIsPackageRuntimeConfigType(t *testing.T) {
	type args struct {
		gk schema.GroupKind
	}
	type want struct {
		ok bool
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"V1Alpha1ControllerConfigOK": {
			reason: "Should return true for a v1alpha1 ControllerConfig",
			args: args{
				gk: v1alpha1.ControllerConfigGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"V1Beta1DeploymentRuntimeConfigOK": {
			reason: "Should return true for a v1beta1 DeploymentRuntimeConfig",
			args: args{
				gk: v1beta1.DeploymentRuntimeConfigGroupVersionKind.GroupKind(),
			},
			want: want{
				ok: true,
			},
		},
		"ElseKO": {
			reason: "Should return false for a random GK",
			args: args{
				gk: schema.GroupKind{
					Group: "foo",
					Kind:  "bar",
				},
			},
			want: want{
				ok: false,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := IsPackageRuntimeConfigType(tc.args.gk)
			if got != tc.want.ok {
				t.Errorf("%s\nIsPackageRuntimeConfigType() = %v, want %v", tc.reason, got, tc.want.ok)
			}
		})
	}
}

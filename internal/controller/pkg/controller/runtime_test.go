/*
Copyright 2025 The Crossplane Authors.

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

package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestParsePackageRuntime(t *testing.T) {
	type args struct {
		input string
	}

	type want struct {
		runtime ActiveRuntime
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyInput": {
			reason: "Should return default runtime when input is empty",
			args: args{
				input: "",
			},
			want: want{
				err: errors.New("invalid package runtime setting \"\", expected: runtime=kind"),
			},
		},
		"SingleValidMapping": {
			reason: "Should parse single valid package runtime mapping",
			args: args{
				input: "Provider=Deployment",
			},
			want: want{
				runtime: NewActiveRuntime(
					WithPackageRuntime(ProviderPackageKind, PackageRuntimeDeployment),
				),
			},
		},
		"MultipleValidMappings": {
			reason: "Should parse multiple valid package runtime mappings",
			args: args{
				input: "Provider=Deployment;Function=External",
			},
			want: want{
				runtime: NewActiveRuntime(
					WithPackageRuntime(ProviderPackageKind, PackageRuntimeDeployment),
					WithPackageRuntime(FunctionPackageKind, PackageRuntimeExternal),
				),
			},
		},
		"AllValidMappings": {
			reason: "Should parse all valid package kinds and runtimes",
			args: args{
				input: "Provider=Deployment;Function=External;Configuration=Deployment",
			},
			want: want{
				runtime: NewActiveRuntime(
					WithPackageRuntime(ProviderPackageKind, PackageRuntimeDeployment),
					WithPackageRuntime(FunctionPackageKind, PackageRuntimeExternal),
					WithPackageRuntime(ConfigurationPackageKind, PackageRuntimeDeployment),
				),
			},
		},
		"InvalidFormat": {
			reason: "Should return error for invalid format",
			args: args{
				input: "Provider",
			},
			want: want{
				err: errors.New("invalid package runtime setting \"Provider\", expected: runtime=kind"),
			},
		},
		"InvalidFormatMultipleEquals": {
			reason: "Should return error for multiple equals signs",
			args: args{
				input: "Provider=Deployment=Extra",
			},
			want: want{
				err: errors.New("invalid package runtime setting \"Provider=Deployment=Extra\", expected: runtime=kind"),
			},
		},
		"InvalidFormatNoEquals": {
			reason: "Should return error for no equals sign",
			args: args{
				input: "ProviderDeployment",
			},
			want: want{
				err: errors.New("invalid package runtime setting \"ProviderDeployment\", expected: runtime=kind"),
			},
		},
		"UnknownPackageKind": {
			reason: "Should return error for unknown package kind",
			args: args{
				input: "UnknownKind=Deployment",
			},
			want: want{
				err: errors.New("unknown package runtime kind \"UnknownKind\", supported: [Configuration, Provider, Function]"),
			},
		},
		"UnknownRuntime": {
			reason: "Should return error for unknown runtime",
			args: args{
				input: "Provider=UnknownRuntime",
			},
			want: want{
				err: errors.New("unknown package runtime \"UnknownRuntime\", supported: [Deployment, External]"),
			},
		},
		"MixedValidAndInvalid": {
			reason: "Should return error on first invalid mapping even if others are valid",
			args: args{
				input: "Provider=Deployment;UnknownKind=External",
			},
			want: want{
				err: errors.New("unknown package runtime kind \"UnknownKind\", supported: [Configuration, Provider, Function]"),
			},
		},
		"EmptyPackageKind": {
			reason: "Should return error for empty package kind",
			args: args{
				input: "=Deployment",
			},
			want: want{
				err: errors.New("unknown package runtime kind \"\", supported: [Configuration, Provider, Function]"),
			},
		},
		"EmptyRuntime": {
			reason: "Should return error for empty runtime",
			args: args{
				input: "Provider=",
			},
			want: want{
				err: errors.New("unknown package runtime \"\", supported: [Deployment, External]"),
			},
		},
		"WhitespaceInInput": {
			reason: "Should handle whitespace in input",
			args: args{
				input: " Provider = Deployment ",
			},
			want: want{
				runtime: NewActiveRuntime(
					WithPackageRuntime(ProviderPackageKind, PackageRuntimeDeployment),
				),
			},
		},
		"ValidExternalRuntime": {
			reason: "Should parse valid External runtime",
			args: args{
				input: "Function=External",
			},
			want: want{
				runtime: NewActiveRuntime(
					WithPackageRuntime(FunctionPackageKind, PackageRuntimeExternal),
				),
			},
		},
		"DuplicatePackageKind": {
			reason: "Should handle duplicate package kinds (last one wins)",
			args: args{
				input: "Provider=Deployment;Provider=External",
			},
			want: want{
				runtime: NewActiveRuntime(
					WithPackageRuntime(ProviderPackageKind, PackageRuntimeDeployment),
					WithPackageRuntime(ProviderPackageKind, PackageRuntimeExternal),
				),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParsePackageRuntime(tc.args.input)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParsePackageRuntime(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.runtime, got, cmp.AllowUnexported(ActiveRuntime{})); diff != "" {
				t.Errorf("\n%s\nParsePackageRuntime(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

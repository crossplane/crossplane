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

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
)

func TestFriendlyID(t *testing.T) {
	type args struct {
		pkg  string
		hash string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   string
	}{
		"BothUnderLimit": {
			reason: "If both package and hash are under limit neither should be truncated.",
			args: args{
				pkg:  "provider-aws",
				hash: "1234567",
			},
			want: "provider-aws-1234567",
		},
		"PackageOverLimit": {
			reason: "If package is over limit it should be truncated.",
			args: args{
				pkg:  "provider-aws-plusabunchofothernonsensethatisgoingtogetslicedoff",
				hash: "1234567",
			},
			want: "provider-aws-plusabunchofothernonsensethatisgoingt-1234567",
		},
		"HashOverLimit": {
			reason: "If hash is over limit it should be truncated.",
			args: args{
				pkg:  "provider-aws",
				hash: "1234567891234567",
			},
			want: "provider-aws-123456789123",
		},
		"BothOverLimit": {
			reason: "If both package and hash are over limit both should be truncated.",
			args: args{
				pkg:  "provider-aws-plusabunchofothernonsensethatisgoingtogetslicedoff",
				hash: "1234567891234567",
			},
			want: "provider-aws-plusabunchofothernonsensethatisgoingt-123456789123",
		},
		"ReplacePeriod": {
			reason: "All period characters should be replaced with a dash.",
			args: args{
				pkg:  "provider.aws-plusabunchofothernonsensethatisgoingtogetslicedoff",
				hash: "1234.567891234567",
			},
			want: "provider-aws-plusabunchofothernonsensethatisgoingt-1234-5678912",
		},
		"DigestIsName": {
			reason: "A valid DNS label should be returned when package digest is a name.",
			args: args{
				pkg:  "provider-in-cluster",
				hash: "provider-in-cluster",
			},
			want: "provider-in-cluster-provider-in",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want := FriendlyID(tc.args.pkg, tc.args.hash)

			if diff := cmp.Diff(tc.want, want); diff != "" {
				t.Errorf("\n%s\nFriendlyID(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestToDNSLabel(t *testing.T) {
	cases := map[string]struct {
		reason string
		arg    string
		want   string
	}{
		"ReplaceAll": {
			reason: "All valid symbols should be replaced with dash.",
			arg:    "-hi/my.name/is-",
			want:   "hi-my-name-is",
		},
		"TrimTo63": {
			reason: "A string longer than 63 valid or replaceable characters should be truncated.",
			arg:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			want:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		"TrimTo63MinusDashes": {
			reason: "A string longer than 63 valid or replaceable characters should be truncated with trailing symbol removed.",
			arg:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa----",
			want:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want := ToDNSLabel(tc.arg)

			if diff := cmp.Diff(tc.want, want); diff != "" {
				t.Errorf("\n%s\nToDNSLabel(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSourceFromReference(t *testing.T) {
	cases := map[string]struct {
		reason string
		arg    name.Reference
		want   string
	}{
		"SuccessfulTagWithDocker": {
			reason: "If registry is docker.io it should be reflected in parsed source.",
			arg: func() name.Reference {
				ref, _ := name.ParseReference("docker.io/hasheddan/xpkg-test:v0.1.0")
				return ref
			}(),
			want: "docker.io/hasheddan/xpkg-test",
		},
		"SuccessfulTagWithDockerIndex": {
			reason: "If registry is index.docker.io it should be reflected in parsed source.",
			arg: func() name.Reference {
				ref, _ := name.ParseReference("index.docker.io/hasheddan/xpkg-test:v0.1.0")
				return ref
			}(),
			want: "index.docker.io/hasheddan/xpkg-test",
		},
		"SuccessfulTagWithRegistryDefaulting": {
			reason: "If no registry is supplied, but defaulting is enabled, default registry should not be reflected in parsed source.",
			arg: func() name.Reference {
				ref, _ := name.ParseReference("hasheddan/xpkg-test:v0.1.0", name.WithDefaultRegistry("registry.upbound.io"))
				return ref
			}(),
			want: "hasheddan/xpkg-test",
		},
		"SuccessfulDigestWithRegistryDefaulting": {
			reason: "If no registry is supplied, but defaulting is enabled, default registry should not be reflected in parsed source.",
			arg: func() name.Reference {
				ref, _ := name.ParseReference("hasheddan/xpkg-test@sha256:c88b938d6e7b2ed43d40b71e5a55df9c60fa653bea0c0961f3294fac46d5b56e", name.WithDefaultRegistry("registry.upbound.io"))
				return ref
			}(),
			want: "hasheddan/xpkg-test",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want := ParsePackageSourceFromReference(tc.arg)

			if diff := cmp.Diff(tc.want, want); diff != "" {
				t.Errorf("\n%s\nParseSourceFromReference(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildPath(t *testing.T) {
	type args struct {
		path string
		name string
		ext  string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   string
	}{
		"NoExtension": {
			reason: "We should append extension if it does not exist.",
			args: args{
				path: "path/to/somewhere",
				name: "test",
				ext:  XpkgExtension,
			},
			want: "path/to/somewhere/test.xpkg",
		},
		"ReplaceExtensionName": {
			reason: "We should replace an extension if one exists in name.",
			args: args{
				path: "path/to/somewhere",
				name: "test.tar",
				ext:  XpkgExtension,
			},
			want: "path/to/somewhere/test.xpkg",
		},
		"ReplaceExtensionPath": {
			reason: "We should replace an extension if one exists in path.",
			args: args{
				path: "path/to/somewhere.tar",
				name: "",
				ext:  XpkgExtension,
			},
			want: "path/to/somewhere.xpkg",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			full := BuildPath(tc.args.path, tc.args.name, tc.args.ext)

			if diff := cmp.Diff(tc.want, full); diff != "" {
				t.Errorf("\n%s\nBuildPath(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

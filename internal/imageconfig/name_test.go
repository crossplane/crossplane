/*
Copyright 2026 The Crossplane Authors.

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

package imageconfig

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
)

func imageConfig(name, packageName string, prefixes ...string) v1beta1.ImageConfig {
	ms := make([]v1beta1.ImageMatch, len(prefixes))
	for i, p := range prefixes {
		ms[i] = v1beta1.ImageMatch{Type: v1beta1.Prefix, Prefix: p}
	}

	return v1beta1.ImageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.ImageConfigSpec{
			MatchImages: ms,
			PackageName: packageName,
		},
	}
}

func listFn(cfgs ...v1beta1.ImageConfig) func(_ context.Context, l client.ObjectList, _ ...client.ListOption) error {
	return test.NewMockListFn(nil, func(l client.ObjectList) error {
		l.(*v1beta1.ImageConfigList).Items = cfgs
		return nil
	})
}

func TestNamerName(t *testing.T) {
	errBoom := errors.New("boom")

	// The reference the package manager would resolve for a dependency.
	ref, err := name.ParseReference("xpkg.crossplane.io/crossplane-contrib/provider-aws:v1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	// A mirror with a deeper repository path, where the default name is at its
	// worst and the distinction between .Name and .Remainder is visible.
	nested, err := name.ParseReference("my.registry/mirror/providers/provider-kubernetes:v1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		c   client.Reader
		ref name.Reference
	}

	type want struct {
		objName     string
		imageConfig string
		err         error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoImageConfigs": {
			reason: "We should fall back to the image's repository path when no ImageConfigs exist.",
			args: args{
				c:   &test.MockClient{MockList: listFn()},
				ref: ref,
			},
			want: want{objName: "crossplane-contrib-provider-aws"},
		},
		"NoPackageName": {
			reason: "We should ignore ImageConfigs that don't configure a package name.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("rewrite-only", "", "xpkg.crossplane.io"),
				)},
				ref: ref,
			},
			want: want{objName: "crossplane-contrib-provider-aws"},
		},
		"NoMatchingPrefix": {
			reason: "We should fall back when no ImageConfig matches the image.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("other", "nope", "registry.example.com"),
				)},
				ref: ref,
			},
			want: want{objName: "crossplane-contrib-provider-aws"},
		},
		"LiteralName": {
			reason: "We should use the literal name from the matching ImageConfig.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("aws", "aws", "xpkg.crossplane.io/crossplane-contrib/provider-aws"),
				)},
				ref: ref,
			},
			want: want{objName: "aws", imageConfig: "aws"},
		},
		"TemplateRemainder": {
			reason: "We should render a template using the matched prefix.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("contrib", "<< .Remainder >>", "xpkg.crossplane.io/crossplane-contrib/"),
				)},
				ref: ref,
			},
			want: want{objName: "provider-aws", imageConfig: "contrib"},
		},
		"TemplateFuncs": {
			reason: "The template functions should be available.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("funcs", `<< trimPrefix "provider-" .Name >>`, "xpkg.crossplane.io"),
				)},
				ref: ref,
			},
			want: want{objName: "aws", imageConfig: "funcs"},
		},
		"NestedPathNoImageConfigs": {
			reason: "The default name for a deeply nested mirror path flattens every segment, which is the problem packageName exists to solve.",
			args: args{
				c:   &test.MockClient{MockList: listFn()},
				ref: nested,
			},
			want: want{objName: "mirror-providers-provider-kubernetes"},
		},
		"NestedPathName": {
			reason: ".Name is the last segment of the repository, however deeply nested the path is.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("mirror", "<< .Name >>", "my.registry/"),
				)},
				ref: nested,
			},
			want: want{objName: "provider-kubernetes", imageConfig: "mirror"},
		},
		"NestedPathRemainder": {
			reason: ".Remainder is everything after the matched prefix, so a shallow prefix leaves the intermediate path segments in the name.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("mirror", "<< .Remainder >>", "my.registry/"),
				)},
				ref: nested,
			},
			want: want{objName: "mirror-providers-provider-kubernetes", imageConfig: "mirror"},
		},
		"NestedPathRemainderWithDeepPrefix": {
			reason: "Matching the full mirror prefix makes .Remainder the package's own name.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("mirror", "<< .Remainder >>", "my.registry/mirror/providers/"),
				)},
				ref: nested,
			},
			want: want{objName: "provider-kubernetes", imageConfig: "mirror"},
		},
		"GoDefaultDelimitersAreNotATemplate": {
			reason: "Templates are delimited by << and >>, so a {{ }} value is inert text rather than a template. The API server rejects such a value, since it contains no << and isn't a valid DNS label, so this only matters if validation is bypassed.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("braces", "{{ .Remainder }}", "xpkg.crossplane.io"),
				)},
				ref: ref,
			},
			want: want{objName: "emainder", imageConfig: "braces"},
		},
		"LongestPrefixWins": {
			reason: "The ImageConfig with the longest matching prefix should win, across all ImageConfigs.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("broad", "broad", "xpkg.crossplane.io"),
					imageConfig("specific", "specific", "xpkg.crossplane.io/crossplane-contrib/provider-aws"),
				)},
				ref: ref,
			},
			want: want{objName: "specific", imageConfig: "specific"},
		},
		"Normalized": {
			reason: "A rendered name that isn't a valid DNS label should be normalized.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("norm", "<< .Repository >>", "xpkg.crossplane.io"),
				)},
				ref: ref,
			},
			want: want{objName: "crossplane-contrib-provider-aws", imageConfig: "norm"},
		},
		"ErrEmptyName": {
			reason: "We should return an error when a template renders to a name that normalizes to nothing.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("empty", "___", "xpkg.crossplane.io/crossplane-contrib/provider-aws"),
				)},
				ref: ref,
			},
			want: want{
				imageConfig: "empty",
				err:         errors.Errorf(errFmtEmptyName, "empty", "xpkg.crossplane.io/crossplane-contrib/provider-aws"),
			},
		},
		"ErrParseTemplate": {
			reason: "We should return an error when the template can't be parsed.",
			args: args{
				c: &test.MockClient{MockList: listFn(
					imageConfig("bad", "<< nope >>", "xpkg.crossplane.io"),
				)},
				ref: ref,
			},
			want: want{
				imageConfig: "bad",
				err:         errors.Wrap(errors.New(`template: packageName:1: function "nope" not defined`), errParseTemplate),
			},
		},
		"ErrListImageConfigs": {
			reason: "We should return an error when we can't list ImageConfigs.",
			args: args{
				c:   &test.MockClient{MockList: test.NewMockListFn(errBoom)},
				ref: ref,
			},
			want: want{err: errors.Wrap(errBoom, errListImageConfigs)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			objName, ic, err := NewNamer(tc.args.c).Name(context.Background(), tc.args.ref)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nName(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.objName, objName); diff != "" {
				t.Errorf("\n%s\nName(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.imageConfig, ic); diff != "" {
				t.Errorf("\n%s\nName(...): -want ImageConfig, +got ImageConfig:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSplitImage(t *testing.T) {
	cases := map[string]struct {
		image          string
		wantRegistry   string
		wantRepository string
	}{
		"Registry":         {"xpkg.crossplane.io/contrib/pkg", "xpkg.crossplane.io", "contrib/pkg"},
		"RegistryWithPort": {"localhost:5000/pkg", "localhost:5000", "pkg"},
		"Localhost":        {"localhost/pkg", "localhost", "pkg"},
		"NoRegistry":       {"contrib/pkg", "", "contrib/pkg"},
		"NoSlash":          {"pkg", "", "pkg"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			registry, repository := splitImage(tc.image)
			if diff := cmp.Diff(tc.wantRegistry, registry); diff != "" {
				t.Errorf("splitImage(%q): -want registry, +got registry:\n%s", tc.image, diff)
			}
			if diff := cmp.Diff(tc.wantRepository, repository); diff != "" {
				t.Errorf("splitImage(%q): -want repository, +got repository:\n%s", tc.image, diff)
			}
		})
	}
}

func TestTemplateData(t *testing.T) {
	cases := map[string]struct {
		reason string
		want   TemplateData
	}{
		"Simple": {
			reason: "A two segment repository splits into a single path segment and a name.",
			want: TemplateData{
				Image:       "xpkg.crossplane.io/crossplane-contrib/provider-aws",
				Registry:    "xpkg.crossplane.io",
				Repository:  "crossplane-contrib/provider-aws",
				Path:        "crossplane-contrib",
				Name:        "provider-aws",
				Match:       "xpkg.crossplane.io/crossplane-contrib/",
				Remainder:   "provider-aws",
				ImageConfig: "cool",
			},
		},
		"NestedPath": {
			reason: "A deeply nested mirror path keeps every intermediate segment in .Path, leaving .Name the last segment alone.",
			want: TemplateData{
				Image:       "my.registry/mirror/providers/provider-kubernetes",
				Registry:    "my.registry",
				Repository:  "mirror/providers/provider-kubernetes",
				Path:        "mirror/providers",
				Name:        "provider-kubernetes",
				Match:       "my.registry/",
				Remainder:   "mirror/providers/provider-kubernetes",
				ImageConfig: "cool",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := imageConfig(tc.want.ImageConfig, "cool", tc.want.Match)

			got := templateData(&cfg, tc.want.Image, tc.want.Match)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\ntemplateData(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

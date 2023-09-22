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

package cache

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	xpapiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/marshaler/xpkg"
)

func TestFlush(t *testing.T) {
	fs := afero.NewMemMapFs()

	cache, _ := NewLocal(
		"/tmp/cache",
		WithFS(fs),
	)

	type args struct {
		pkg *xpkg.ParsedPackage
	}

	type want struct {
		imetaCount int
		metaCount  int
		crdCount   int
		xrdCount   int
		compCount  int
		flushErr   error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderSuccess": {
			reason: "Should produce the expected number of definitions from test provider package.",
			args: args{
				pkg: &xpkg.ParsedPackage{
					Reg:     "index.docker.io",
					DepName: "crossplane/provider-aws",
					Ver:     "v0.20.0",
					MetaObj: &xpmetav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "provider-aws",
						},
					},
					Objs: []runtime.Object{
						&apiextv1.CustomResourceDefinition{
							TypeMeta: apimetav1.TypeMeta{
								APIVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
							},
							ObjectMeta: apimetav1.ObjectMeta{
								Name: "crd1",
							},
						},
						&apiextv1.CustomResourceDefinition{
							TypeMeta: apimetav1.TypeMeta{
								APIVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
							},
							ObjectMeta: apimetav1.ObjectMeta{
								Name: "crd2",
							},
						},
					},
					SHA: "adfadfadsfasdfasdfsadf",
				},
			},
			want: want{
				imetaCount: 1,
				metaCount:  1,
				crdCount:   2,
				xrdCount:   0,
			},
		},
		"ConfigurationSuccess": {
			reason: "Should produce the expected number of definitions from test configuration package.",
			args: args{
				pkg: &xpkg.ParsedPackage{
					Reg:     "index.docker.io",
					DepName: "crossplane/configuration-aws",
					Ver:     "v0.20.0",
					MetaObj: &xpmetav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Configuration",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "platform-ref-aws",
						},
					},
					Objs: []runtime.Object{
						&xpapiextv1.CompositeResourceDefinition{
							TypeMeta: apimetav1.TypeMeta{
								APIVersion: "apiextensions.crossplane.io/v1",
								Kind:       "CompositeResourceDefinition",
							},
							ObjectMeta: apimetav1.ObjectMeta{
								Name: "xrd1",
							},
						},
						&xpapiextv1.CompositeResourceDefinition{
							TypeMeta: apimetav1.TypeMeta{
								APIVersion: "apiextensions.crossplane.io/v1",
								Kind:       "CompositeResourceDefinition",
							},
							ObjectMeta: apimetav1.ObjectMeta{
								Name: "xrd2",
							},
						},
						&xpapiextv1.Composition{
							TypeMeta: apimetav1.TypeMeta{
								APIVersion: "apiextensions.crossplane.io/v1",
								Kind:       "Composition",
							},
							ObjectMeta: apimetav1.ObjectMeta{
								Name: "comp",
							},
						},
					},
					SHA: "adfadfadsfasdfasdfsadf",
				},
			},
			want: want{
				imetaCount: 1,
				metaCount:  1,
				crdCount:   0,
				xrdCount:   2,
				compCount:  1,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := cache.newEntry(tc.args.pkg)

			stats, err := e.flush()

			if diff := cmp.Diff(tc.want.imetaCount, stats.imageMeta); diff != "" {
				t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.metaCount, stats.metas); diff != "" {
				t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.crdCount, stats.crds); diff != "" {
				t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.xrdCount, stats.xrds); diff != "" {
				t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.compCount, stats.comps); diff != "" {
				t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.flushErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
			}

		})
	}
}

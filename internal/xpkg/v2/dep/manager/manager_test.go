// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/cache"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/resolver/image"
)

func TestResolveTransitiveDependencies(t *testing.T) {
	// SUT - recursively reading in meta and pulling deps using the manager

	fs := afero.NewMemMapFs()
	c, _ := cache.NewLocal("/tmp/cache", cache.WithFS(fs))

	type depMeta struct {
		dep  v1beta1.Dependency
		meta runtime.Object
	}

	type args struct {
		// root represents the root dependency and its corresponding meta file
		// that may or may not have transitive dependencies
		root depMeta
		// leaf represents the leaf dependency and its corresponding meta file
		leaf depMeta
	}

	type want struct {
		// entries we expect to exist in system given the above args
		entries []v1beta1.Dependency
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoTransitiveDependencies": {
			reason: "Should successfully store the root dependency.",
			args: args{
				root: depMeta{
					dep: v1beta1.Dependency{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
					meta: &metav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						Spec: metav1.ProviderSpec{
							MetaSpec: metav1.MetaSpec{},
						},
					},
				},
			},
			want: want{
				entries: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
				},
			},
		},
		"TransitiveDependency": {
			reason: "Should successfully store both the root and the transitive dependency.",
			args: args{
				root: depMeta{
					dep: v1beta1.Dependency{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
					meta: &metav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						Spec: metav1.ProviderSpec{
							MetaSpec: metav1.MetaSpec{
								DependsOn: []metav1.Dependency{
									{
										Provider: pointer.String("crossplane/provider-aws-dependency"),
										Version:  "v1.10.0",
									},
								},
							},
						},
					},
				},
				leaf: depMeta{
					dep: v1beta1.Dependency{
						Package:     "crossplane/provider-aws-dependency",
						Constraints: "v1.10.0",
					},
					meta: &metav1.Provider{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "meta.pkg.crossplane.io/v1alpha1",
							Kind:       "Provider",
						},
						Spec: metav1.ProviderSpec{
							MetaSpec: metav1.MetaSpec{},
						},
					},
				},
			},
			want: want{
				entries: []v1beta1.Dependency{
					{
						Package:     "crossplane/provider-aws",
						Constraints: "v0.1.0",
					},
					{
						Package:     "crossplane/provider-aws-dependency",
						Constraints: "v1.10.0",
					},
				},
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			ref, _ := name.ParseReference(image.FullTag(tc.args.root.dep))
			lref, _ := name.ParseReference(image.FullTag(tc.args.leaf.dep))

			m, _ := New(
				WithCache(c),
				WithResolver(
					image.NewResolver(
						image.WithFetcher(
							NewMockFetcher(
								WithPackageObjects(ref, tc.args.root.meta),
								WithPackageObjects(lref, tc.args.leaf.meta),
							),
						),
					),
				),
			)

			_, acc, err := m.AddAll(context.Background(), tc.args.root.dep)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nResolveTransitiveDependencies(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// make sure the # of accumulated entries is equal to the expected entries
			if diff := cmp.Diff(len(tc.want.entries), len(acc)); diff != "" {
				t.Errorf("\n%s\nResolveTransitiveDependencies(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			for _, e := range tc.want.entries {
				// for each expressed entry, we should not get a NotExists
				_, err := m.c.Get(e)

				if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nResolveTransitiveDependencies(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestSnapshot(t *testing.T) {
	ctx := context.Background()
	fs := afero.NewMemMapFs()
	c, _ := cache.NewLocal("/tmp/cache", cache.WithFS(fs))

	type args struct {
		dep  v1beta1.Dependency
		meta runtime.Object
		objs []runtime.Object
	}
	type want struct {
		keys []string
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessAllRelatedDepsInCacheNonDockerHubPackage": {
			args: args{
				dep: v1beta1.Dependency{
					Package:     "registry.upbound.io/upbound/platform-ref-aws",
					Type:        v1beta1.ConfigurationPackageType,
					Constraints: "v0.2.1",
				},
				meta: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1alpha1",
						Kind:       "Provider",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{},
					},
				},
				objs: []runtime.Object{
					&apiextv1.CustomResourceDefinition{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "apiextensions.k8s.io/v1",
							Kind:       "CustomResourceDefinition",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "crd1",
						},
						Spec: apiextv1.CustomResourceDefinitionSpec{
							Names: apiextv1.CustomResourceDefinitionNames{
								Plural:   "tests",
								Singular: "test",
								Kind:     "testcrd",
							},
							Group: "crossplane.io",
							Versions: []apiextv1.CustomResourceDefinitionVersion{
								{
									Name: "v1alpha1",
									Schema: &apiextv1.CustomResourceValidation{
										OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
											ID:          "id-v1alpha1",
											Description: "desc1",
										},
									},
								},
								{
									Name: "v1beta1",
									Schema: &apiextv1.CustomResourceValidation{
										OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
											ID:          "id-v1beta1",
											Description: "desc2",
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				keys: []string{
					"registry.upbound.io/upbound/platform-ref-aws",
				},
			},
		},
		"SuccessAllRelatedDepsInCacheDockerHubPackage": {
			args: args{
				dep: v1beta1.Dependency{
					Package:     "crossplane/provider-aws",
					Type:        v1beta1.ConfigurationPackageType,
					Constraints: "v0.2.1",
				},
				meta: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1alpha1",
						Kind:       "Provider",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{},
					},
				},
				objs: []runtime.Object{
					&apiextv1.CustomResourceDefinition{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "apiextensions.k8s.io/v1",
							Kind:       "CustomResourceDefinition",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "crd1",
						},
						Spec: apiextv1.CustomResourceDefinitionSpec{
							Names: apiextv1.CustomResourceDefinitionNames{
								Plural:   "tests",
								Singular: "test",
								Kind:     "testcrd",
							},
							Group: "crossplane.io",
							Versions: []apiextv1.CustomResourceDefinitionVersion{
								{
									Name: "v1alpha1",
									Schema: &apiextv1.CustomResourceValidation{
										OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
											ID:          "id-v1alpha1",
											Description: "desc1",
										},
									},
								},
								{
									Name: "v1beta1",
									Schema: &apiextv1.CustomResourceValidation{
										OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
											ID:          "id-v1beta1",
											Description: "desc2",
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				keys: []string{
					"crossplane/provider-aws",
				},
			},
		},
		"ErrorInvalidDepVersion": {
			args: args{
				dep: v1beta1.Dependency{
					Package:     "registry.upbound.io/upbound/platform-ref-aws",
					Type:        v1beta1.ConfigurationPackageType,
					Constraints: "no",
				},
				meta: &metav1.Provider{
					TypeMeta: apimetav1.TypeMeta{
						APIVersion: "meta.pkg.crossplane.io/v1alpha1",
						Kind:       "Provider",
					},
					Spec: metav1.ProviderSpec{
						MetaSpec: metav1.MetaSpec{},
					},
				},
				objs: []runtime.Object{
					&apiextv1.CustomResourceDefinition{
						TypeMeta: apimetav1.TypeMeta{
							APIVersion: "apiextensions.k8s.io/v1",
							Kind:       "CustomResourceDefinition",
						},
						ObjectMeta: apimetav1.ObjectMeta{
							Name: "crd1",
						},
						Spec: apiextv1.CustomResourceDefinitionSpec{
							Names: apiextv1.CustomResourceDefinitionNames{
								Plural:   "tests",
								Singular: "test",
								Kind:     "testcrd",
							},
							Group: "crossplane.io",
							Versions: []apiextv1.CustomResourceDefinitionVersion{
								{
									Name: "v1alpha1",
									Schema: &apiextv1.CustomResourceValidation{
										OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
											ID:          "id-v1alpha1",
											Description: "desc1",
										},
									},
								},
								{
									Name: "v1beta1",
									Schema: &apiextv1.CustomResourceValidation{
										OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
											ID:          "id-v1beta1",
											Description: "desc2",
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				keys: []string{},
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {

			ref, _ := name.ParseReference(image.FullTag(tc.args.dep))

			m, _ := New(
				WithCache(c),
				WithResolver(
					image.NewResolver(
						image.WithFetcher(
							NewMockFetcher(
								WithPackageObjects(ref, tc.args.meta, tc.args.objs...),
							),
						),
					),
				),
			)

			// add the pkg to the cache
			m.addPkg(ctx, tc.args.dep)

			got, err := m.View(ctx, []v1beta1.Dependency{tc.args.dep})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nSnapshot(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			for _, k := range tc.want.keys {
				_, ok := got.Packages()[k]

				if diff := cmp.Diff(true, ok); diff != "" {
					t.Errorf("\n%s\nSnapshot(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

type MockFetcher struct {
	pkgMeta map[name.Reference][]runtime.Object
	tags    []string
	err     error
}

func NewMockFetcher(opts ...MockFetcherOption) *MockFetcher {
	f := &MockFetcher{
		pkgMeta: make(map[name.Reference][]runtime.Object),
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

// MockFetcherOption modifies the mock resolver.
type MockFetcherOption func(*MockFetcher)

func WithPackageObjects(ref name.Reference, meta runtime.Object, objs ...runtime.Object) MockFetcherOption {
	return func(m *MockFetcher) {
		pkg := make([]runtime.Object, 0)
		pkg = append(pkg, meta)
		pkg = append(pkg, objs...)
		m.pkgMeta[ref] = pkg
	}
}

func (m *MockFetcher) Fetch(_ context.Context, ref name.Reference, _ ...string) (v1.Image, error) {
	objs, ok := m.pkgMeta[ref]
	if !ok {
		return nil, errors.New("entry does not exist in pkgMeta map")
	}
	return newPackageImage(objs...), nil
}
func (m *MockFetcher) Head(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
	h, _ := v1.NewHash("test")

	return &v1.Descriptor{
		Digest: h,
	}, nil
}
func (m *MockFetcher) Tags(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
	if m.tags != nil {
		return m.tags, nil
	}
	return nil, m.err
}

func newPackageImage(objs ...runtime.Object) v1.Image {
	rbuf := new(bytes.Buffer)
	for _, o := range objs {
		b, _ := yaml.Marshal(o)

		rbuf.Write(b)
		rbuf.WriteString("\n---\n")
	}

	wbuf := new(bytes.Buffer)
	tw := tar.NewWriter(wbuf)
	hdr := &tar.Header{
		Name: xpkg.StreamFile,
		Mode: int64(xpkg.StreamFileMode),
		Size: int64(len(rbuf.Bytes())),
	}

	_ = tw.WriteHeader(hdr)
	_, _ = io.Copy(tw, rbuf)
	_ = tw.Close()
	packLayer, _ := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		// NOTE(hasheddan): we must construct a new reader each time as we
		// ingest packImg in multiple tests below.
		return io.NopCloser(bytes.NewReader(wbuf.Bytes())), nil
	})
	packImg, _ := mutate.AppendLayers(empty.Image, packLayer)

	return packImg
}

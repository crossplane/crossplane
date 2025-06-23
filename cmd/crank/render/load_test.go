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

package render

import (
	"embed"
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	apiextensionsv2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

//go:embed testdata
var testdatafs embed.FS

func TestLoadCompositeResource(t *testing.T) {
	fs := afero.FromIOFS{FS: testdatafs}

	type want struct {
		xr  *composite.Unstructured
		err error
	}

	cases := map[string]struct {
		file string
		want want
	}{
		"Success": {
			file: "testdata/xr.yaml",
			want: want{
				xr: &composite.Unstructured{
					Unstructured: unstructured.Unstructured{
						Object: MustLoadJSON(`{
							"apiVersion": "nop.example.org/v1alpha1",
							"kind": "XNopResource",
							"metadata": {
								"name": "test-render"
							},
							"spec": {
								"coolField": "I'm cool!"
							}
						}`),
					},
				},
			},
		},
		"NoSuchFile": {
			file: "testdata/nonexist.yaml",
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr, err := LoadCompositeResource(fs, tc.file)

			if diff := cmp.Diff(tc.want.xr, xr, test.EquateConditions()); diff != "" {
				t.Errorf("LoadCompositeResource(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadCompositeResource(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLoadXRD(t *testing.T) {
	fs := afero.FromIOFS{FS: testdatafs}

	type want struct {
		xrd *apiextensionsv2.CompositeResourceDefinition
		err error
	}

	cases := map[string]struct {
		file string
		want want
	}{
		"Success": {
			file: "testdata/xrd.yaml",
			want: want{
				xrd: &apiextensionsv2.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiextensionsv1.CompositeResourceDefinitionKind,
						APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{Name: "xnopresources.nop.example.org"},
					Spec: apiextensionsv2.CompositeResourceDefinitionSpec{
						Group: "nop.example.org",
						Names: v1.CustomResourceDefinitionNames{
							Kind:       "XNopResource",
							Plural:     "xnopresources",
							Singular:   "xnopresource",
							ShortNames: []string{"xnr"},
						},
						Versions: []apiextensionsv2.CompositeResourceDefinitionVersion{
							{
								Name:   "v1",
								Served: true,
								Schema: &apiextensionsv2.CompositeResourceValidation{
									OpenAPIV3Schema: runtime.RawExtension{
										Raw: []byte(`{"description":"A test resource","properties":{"spec":{"properties":{"coolField":{"type":"string"}},"type":"object"}},"type":"object"}`),
									},
								},
							},
						},
					},
				},
			},
		},
		"NoSuchFile": {
			file: "testdata/nonexist.yaml",
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xrd, err := LoadXRD(fs, tc.file)

			if diff := cmp.Diff(tc.want.xrd, xrd, test.EquateConditions()); diff != "" {
				t.Errorf("LoadXRD(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadXRD(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLoadComposition(t *testing.T) {
	fs := afero.FromIOFS{FS: testdatafs}

	type want struct {
		comp *apiextensionsv1.Composition
		err  error
	}

	cases := map[string]struct {
		file string
		want want
	}{
		"Success": {
			file: "testdata/composition.yaml",
			want: want{
				comp: &apiextensionsv1.Composition{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiextensionsv1.CompositionKind,
						APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{Name: "xnopresources.nop.example.org"},
					Spec: apiextensionsv1.CompositionSpec{
						CompositeTypeRef: apiextensionsv1.TypeReference{
							APIVersion: "nop.example.org/v1alpha1",
							Kind:       "XNopResource",
						},
						Mode: apiextensionsv1.CompositionModePipeline,
						Pipeline: []apiextensionsv1.PipelineStep{{
							Step:        "be-a-dummy",
							FunctionRef: apiextensionsv1.FunctionReference{Name: "function-dummy"},
						}},
					},
				},
			},
		},
		"NoSuchFile": {
			file: "testdata/nonexist.yaml",
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NotAComposition": {
			file: "testdata/xr.yaml",
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr, err := LoadComposition(fs, tc.file)

			if diff := cmp.Diff(tc.want.comp, xr, test.EquateConditions()); diff != "" {
				t.Errorf("LoadComposition(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadComposition(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLoadFunctions(t *testing.T) {
	fs := afero.FromIOFS{FS: testdatafs}

	type want struct {
		fns []pkgv1.Function
		err error
	}

	cases := map[string]struct {
		file string
		want want
	}{
		"Success": {
			file: "testdata/functions.yaml",
			want: want{
				fns: []pkgv1.Function{
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       pkgv1.FunctionKind,
							APIVersion: pkgv1.SchemeGroupVersion.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-auto-ready",
							Annotations: map[string]string{
								AnnotationKeyRuntime:              string(AnnotationValueRuntimeDocker),
								AnnotationKeyRuntimeDockerCleanup: string(AnnotationValueRuntimeDockerCleanupOrphan),
							},
						},
						Spec: pkgv1.FunctionSpec{
							PackageSpec: pkgv1.PackageSpec{
								Package: "xpkg.crossplane.io/crossplane-contrib/function-auto-ready:v0.1.2",
							},
						},
					},
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       pkgv1.FunctionKind,
							APIVersion: pkgv1.SchemeGroupVersion.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-dummy",
							Annotations: map[string]string{
								AnnotationKeyRuntime:                  string(AnnotationValueRuntimeDevelopment),
								AnnotationKeyRuntimeDevelopmentTarget: "localhost:9444",
							},
						},
						Spec: pkgv1.FunctionSpec{
							PackageSpec: pkgv1.PackageSpec{
								Package: "xpkg.crossplane.io/crossplane-contrib/function-dummy:v0.2.1",
							},
						},
					},
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       pkgv1.FunctionKind,
							APIVersion: pkgv1.SchemeGroupVersion.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-auto-ready",
							Annotations: map[string]string{
								AnnotationKeyRuntime:               string(AnnotationValueRuntimeDocker),
								AnnotationKeyRuntimeDockerCleanup:  string(AnnotationValueRuntimeDockerCleanupOrphan),
								AnnotationKeyRuntimeNamedContainer: "function-auto-ready",
							},
						},
						Spec: pkgv1.FunctionSpec{
							PackageSpec: pkgv1.PackageSpec{
								Package: "xpkg.crossplane.io/crossplane-contrib/function-auto-ready:v0.1.2",
							},
						},
					},
				},
			},
		},
		"NoSuchFile": {
			file: "testdata/nonexist.yaml",
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NotAFunction": {
			file: "testdata/xr.yaml",
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr, err := LoadFunctions(fs, tc.file)

			if diff := cmp.Diff(tc.want.fns, xr, test.EquateConditions()); diff != "" {
				t.Errorf("LoadFunctions(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadFunctions(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLoadObservedResources(t *testing.T) {
	fs := afero.FromIOFS{FS: testdatafs}

	type want struct {
		ors []composed.Unstructured
		err error
	}

	cases := map[string]struct {
		file string
		want want
	}{
		"Success": {
			file: "testdata/observed.yaml",
			want: want{
				ors: []composed.Unstructured{
					{
						Unstructured: unstructured.Unstructured{Object: MustLoadJSON(`{
							"apiVersion": "example.org/v1alpha1",
							"kind": "ComposedResource",
							"metadata": {
								"name": "test-render-a",
								"annotations": {
									"crossplane.io/composition-resource-name": "resource-a"
								}
							},
							"spec": {
								"coolField": "I'm cool!"
							}
						}`)},
					},
					{
						Unstructured: unstructured.Unstructured{Object: MustLoadJSON(`{
							"apiVersion": "example.org/v1alpha1",
							"kind": "ComposedResource",
							"metadata": {
								"name": "test-render-b",
								"annotations": {
									"crossplane.io/composition-resource-name": "resource-b"
								}
							},
							"spec": {
								"coolerField": "I'm cooler!"
							}
						}`)},
					},
				},
			},
		},
		"NoSuchFile": {
			file: "testdata/nonexist.yaml",
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr, err := LoadObservedResources(fs, tc.file)

			if diff := cmp.Diff(tc.want.ors, xr, test.EquateConditions()); diff != "" {
				t.Errorf("LoadObservedResources(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadObservedResources(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLoadExtraResources(t *testing.T) {
	fs := afero.FromIOFS{FS: testdatafs}

	type args struct {
		file string
		fs   afero.Fs
	}

	type want struct {
		out []unstructured.Unstructured
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				file: "testdata/extra-resources.yaml",
				fs:   fs,
			},
			want: want{
				out: []unstructured.Unstructured{
					{
						Object: MustLoadJSON(`{
							"apiVersion": "example.org/v1alpha1",
							"kind": "ExtraResourceA",
							"metadata": {
								"name": "test-extra-a",
								"annotations": {
									"some-annotation": "some-value"
								}
							},
							"spec": {
								"coolField": "I'm cool!"
							}
						}`),
					},
					{
						Object: MustLoadJSON(`{
							"apiVersion": "example.org/v1",
							"kind": "ExtraResourceB",
							"metadata": {
								"name": "test-extra-b",
								"annotations": {
									"some-other-annotation": "some-other-value"
								},
								"labels": {
									"some-label": "another-value"
								}
							},
							"spec": {
								"coolerField": "I'm cooler!"
							}
						}`),
					},
				},
			},
		},
		"NoSuchFile": {
			args: args{
				file: "testdata/nonexist.yaml",
				fs:   fs,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, err := LoadExtraResources(tc.args.fs, tc.args.file)

			if diff := cmp.Diff(tc.want.out, f, test.EquateConditions()); diff != "" {
				t.Errorf("LoadExtraResources(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadExtraResources(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLoadYAMLStream(t *testing.T) {
	type args struct {
		file string
		fs   afero.Fs
	}

	type want struct {
		out [][]byte
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				file: "testdata/observed.yaml",
				fs: afero.FromIOFS{
					FS: fstest.MapFS{
						"testdata/observed.yaml": &fstest.MapFile{
							Data: []byte(`---
test: "test"
---
test: "test2"
`),
						},
					},
				},
			},
			want: want{
				out: [][]byte{
					[]byte("---\ntest: \"test\"\n"),
					[]byte("test: \"test2\"\n"),
				},
			},
		},
		"NoSuchFile": {
			args: args{
				file: "testdata/nonexist.yaml",
				fs:   afero.FromIOFS{FS: fstest.MapFS{}},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SuccessWithSubdirectory": {
			args: args{
				file: "testdata",
				fs: afero.FromIOFS{FS: fstest.MapFS{
					"testdata/file-1.yaml": &fstest.MapFile{
						Data: []byte(`---
test: "file-1"
`),
					},
					"testdata/file-2.yaml": &fstest.MapFile{
						Data: []byte(`---
test: "file-2"
`),
					},
					"testdata/file-3.txt": &fstest.MapFile{
						Data: []byte(`THIS SHOULD NOT BE LOADED`),
					},
					"testdata/subdir/file-4.yaml": &fstest.MapFile{
						Data: []byte(`THIS IS IN A SUBDIRECTORY AND SHOULD NOT BE LOADED`),
					},
				}},
			},
			want: want{
				out: [][]byte{
					[]byte("---\ntest: \"file-1\"\n"),
					[]byte("---\ntest: \"file-2\"\n"),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, err := LoadYAMLStream(tc.args.fs, tc.args.file)

			if diff := cmp.Diff(tc.want.out, f, cmpopts.AcyclicTransformer("string", func(in []byte) string {
				return string(in)
			})); diff != "" {
				t.Errorf("LoadYAMLStreamFromFile(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadYAMLStreamFromFile(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func MustLoadJSON(j string) map[string]any {
	out := make(map[string]any)
	if err := json.Unmarshal([]byte(j), &out); err != nil {
		panic(err)
	}

	return out
}

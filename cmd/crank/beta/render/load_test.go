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
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

func TestLoadCompositeResource(t *testing.T) {
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
			xr, err := LoadCompositeResource(tc.file)

			if diff := cmp.Diff(tc.want.xr, xr, test.EquateConditions()); diff != "" {
				t.Errorf("LoadCompositeResource(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadCompositeResource(..), -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLoadComposition(t *testing.T) {
	pipeline := apiextensionsv1.CompositionModePipeline

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
						Mode: &pipeline,
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr, err := LoadComposition(tc.file)

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

	type want struct {
		fns []pkgv1beta1.Function
		err error
	}
	cases := map[string]struct {
		file string
		want want
	}{
		"Success": {
			file: "testdata/functions.yaml",
			want: want{
				fns: []pkgv1beta1.Function{
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       pkgv1beta1.FunctionKind,
							APIVersion: pkgv1beta1.SchemeGroupVersion.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-auto-ready",
							Annotations: map[string]string{
								AnnotationKeyRuntime:              string(AnnotationValueRuntimeDocker),
								AnnotationKeyRuntimeDockerCleanup: string(AnnotationValueRuntimeDockerCleanupOrphan),
							},
						},
						Spec: pkgv1beta1.FunctionSpec{
							PackageSpec: pkgv1.PackageSpec{
								Package: "xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.1.2",
							},
						},
					},
					{
						TypeMeta: metav1.TypeMeta{
							Kind:       pkgv1beta1.FunctionKind,
							APIVersion: pkgv1beta1.SchemeGroupVersion.String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "function-dummy",
							Annotations: map[string]string{
								AnnotationKeyRuntime:                  string(AnnotationValueRuntimeDevelopment),
								AnnotationKeyRuntimeDevelopmentTarget: "localhost:9444",
							},
						},
						Spec: pkgv1beta1.FunctionSpec{
							PackageSpec: pkgv1.PackageSpec{
								Package: "xpkg.upbound.io/crossplane-contrib/function-dummy:v0.2.1",
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
			xr, err := LoadFunctions(tc.file)

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
			xr, err := LoadObservedResources(tc.file)

			if diff := cmp.Diff(tc.want.ors, xr, test.EquateConditions()); diff != "" {
				t.Errorf("LoadObservedResources(..), -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("LoadObservedResources(..), -want, +got:\n%s", diff)
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

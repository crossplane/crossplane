/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	htcp://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package applicationconfiguration

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/oam/v1alpha2"
)

func TestRenderComponents(t *testing.T) {
	errBoom := errors.New("boom")

	namespace := "ns"
	acName := "coolappconfig"
	acUID := types.UID("definitely-a-uuid")
	componentName := "coolcomponent"
	workloadName := "coolworkload"
	traitName := "coolTrait"

	ac := &v1alpha2.ApplicationConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      acName,
			UID:       acUID,
		},
		Spec: v1alpha2.ApplicationConfigurationSpec{
			Components: []v1alpha2.ApplicationConfigurationComponent{
				{
					ComponentName: componentName,
					Traits:        []v1alpha2.ComponentTrait{{}},
				},
			},
		},
	}

	ref := metav1.NewControllerRef(ac, v1alpha2.ApplicationConfigurationGroupVersionKind)

	type fields struct {
		client   client.Reader
		params   ParameterResolver
		workload ResourceRenderer
		trait    ResourceRenderer
	}
	type args struct {
		ctx context.Context
		ac  *v1alpha2.ApplicationConfiguration
	}
	type want struct {
		w   []Workload
		err error
	}
	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"GetError": {
			reason: "An error getting a component should be returned",
			fields: fields{
				client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			},
			args: args{ac: ac},
			want: want{
				err: errors.Wrapf(errBoom, errFmtGetComponent, componentName),
			},
		},
		"ResolveParamsError": {
			reason: "An error resolving the parameters of a component should be returned",
			fields: fields{
				client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				params: ParameterResolveFn(func(_ []v1alpha2.ComponentParameter, _ []v1alpha2.ComponentParameterValue) ([]Parameter, error) {
					return nil, errBoom
				}),
			},
			args: args{ac: ac},
			want: want{
				err: errors.Wrapf(errBoom, errFmtResolveParams, componentName),
			},
		},
		"RenderWorkloadError": {
			reason: "An error rendering a component's workload should be returned",
			fields: fields{
				client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				params: ParameterResolveFn(func(_ []v1alpha2.ComponentParameter, _ []v1alpha2.ComponentParameterValue) ([]Parameter, error) {
					return nil, nil
				}),
				workload: ResourceRenderFn(func(_ []byte, _ ...Parameter) (*unstructured.Unstructured, error) {
					return nil, errBoom
				}),
			},
			args: args{ac: ac},
			want: want{
				err: errors.Wrapf(errBoom, errFmtRenderWorkload, componentName),
			},
		},
		"RenderTraitError": {
			reason: "An error rendering a component's traits should be returned",
			fields: fields{
				client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				params: ParameterResolveFn(func(_ []v1alpha2.ComponentParameter, _ []v1alpha2.ComponentParameterValue) ([]Parameter, error) {
					return nil, nil
				}),
				workload: ResourceRenderFn(func(_ []byte, _ ...Parameter) (*unstructured.Unstructured, error) {
					return &unstructured.Unstructured{}, nil
				}),
				trait: ResourceRenderFn(func(_ []byte, _ ...Parameter) (*unstructured.Unstructured, error) {
					return nil, errBoom
				}),
			},
			args: args{ac: ac},
			want: want{
				err: errors.Wrapf(errBoom, errFmtRenderTrait, componentName),
			},
		},
		"Success": {
			reason: "One workload and one trait should successfully be rendered",
			fields: fields{
				client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				params: ParameterResolveFn(func(_ []v1alpha2.ComponentParameter, _ []v1alpha2.ComponentParameterValue) ([]Parameter, error) {
					return nil, nil
				}),
				workload: ResourceRenderFn(func(_ []byte, _ ...Parameter) (*unstructured.Unstructured, error) {
					w := &unstructured.Unstructured{}
					w.SetName(workloadName)
					return w, nil
				}),
				trait: ResourceRenderFn(func(_ []byte, _ ...Parameter) (*unstructured.Unstructured, error) {
					t := &unstructured.Unstructured{}
					t.SetName(traitName)
					return t, nil
				}),
			},
			args: args{ac: ac},
			want: want{
				w: []Workload{
					{
						ComponentName: componentName,
						Workload: func() *unstructured.Unstructured {
							w := &unstructured.Unstructured{}
							w.SetNamespace(namespace)
							w.SetName(workloadName)
							w.SetOwnerReferences([]metav1.OwnerReference{*ref})
							return w
						}(),
						Traits: []unstructured.Unstructured{
							func() unstructured.Unstructured {
								t := &unstructured.Unstructured{}
								t.SetNamespace(namespace)
								t.SetName(traitName)
								t.SetOwnerReferences([]metav1.OwnerReference{*ref})
								return *t
							}(),
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &components{tc.fields.client, tc.fields.params, tc.fields.workload, tc.fields.trait}
			got, err := r.Render(tc.args.ctx, tc.args.ac)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Render(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.w, got); diff != "" {
				t.Errorf("\n%s\nr.Render(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestRenderWorkload(t *testing.T) {
	namespace := "ns"
	paramName := "coolparam"
	strVal := "coolstring"
	intVal := 32

	type args struct {
		data []byte
		p    []Parameter
	}
	type want struct {
		workload *unstructured.Unstructured
		err      error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnmarshalError": {
			reason: "Errors unmarshalling JSON should be returned",
			args: args{
				data: []byte(`wat`),
			},
			want: want{
				err: errors.Wrapf(errors.New("invalid character 'w' looking for beginning of value"), errUnmarshalWorkload),
			},
		},
		"SetStringError": {
			reason: "Errors setting a string value should be returned",
			args: args{
				data: []byte(`{"metadata":{}}`),
				p: []Parameter{{
					Name:       paramName,
					Value:      intstr.FromString(strVal),
					FieldPaths: []string{"metadata[0]"},
				}},
			},
			want: want{
				err: errors.Wrapf(errors.New("metadata is not an array"), errFmtSetParam, paramName),
			},
		},
		"SetNumberError": {
			reason: "Errors setting a number value should be returned",
			args: args{
				data: []byte(`{"metadata":{}}`),
				p: []Parameter{{
					Name:       paramName,
					Value:      intstr.FromInt(intVal),
					FieldPaths: []string{"metadata[0]"},
				}},
			},
			want: want{
				err: errors.Wrapf(errors.New("metadata is not an array"), errFmtSetParam, paramName),
			},
		},
		"Success": {
			reason: "A workload should be returned with the supplied parameters set",
			args: args{
				data: []byte(`{"metadata":{"namespace":"` + namespace + `","name":"name"}}`),
				p: []Parameter{{
					Name:       paramName,
					Value:      intstr.FromString(strVal),
					FieldPaths: []string{"metadata.name"},
				}},
			},
			want: want{
				workload: func() *unstructured.Unstructured {
					w := &unstructured.Unstructured{}
					w.SetNamespace(namespace)
					w.SetName(strVal)
					return w
				}(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := renderWorkload(tc.args.data, tc.args.p...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nrenderWorkload(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.workload, got); diff != "" {
				t.Errorf("\n%s\nrenderWorkload(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestRenderTrait(t *testing.T) {
	apiVersion := "coolversion"
	kind := "coolkind"

	type args struct {
		data []byte
	}
	type want struct {
		workload *unstructured.Unstructured
		err      error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnmarshalError": {
			reason: "Errors unmarshalling JSON should be returned",
			args: args{
				data: []byte(`wat`),
			},
			want: want{
				err: errors.Wrapf(errors.New("invalid character 'w' looking for beginning of value"), errUnmarshalTrait),
			},
		},
		"Success": {
			reason: "A workload should be returned with the supplied parameters set",
			args: args{
				data: []byte(`{"apiVersion":"` + apiVersion + `","kind":"` + kind + `"}`),
			},
			want: want{
				workload: func() *unstructured.Unstructured {
					w := &unstructured.Unstructured{}
					w.SetAPIVersion(apiVersion)
					w.SetKind(kind)
					return w
				}(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := renderTrait(tc.args.data)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nrenderTrait(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.workload, got); diff != "" {
				t.Errorf("\n%s\nrenderTrait(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestResolveParams(t *testing.T) {
	paramName := "coolparam"
	required := true
	paths := []string{"metadata.name"}
	value := "cool"

	type args struct {
		cp  []v1alpha2.ComponentParameter
		cpv []v1alpha2.ComponentParameterValue
	}
	type want struct {
		p   []Parameter
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MissingRequired": {
			reason: "An error should be returned when a required parameter is omitted",
			args: args{
				cp: []v1alpha2.ComponentParameter{
					{
						Name:     paramName,
						Required: &required,
					},
				},
				cpv: []v1alpha2.ComponentParameterValue{},
			},
			want: want{
				err: errors.Errorf(errFmtRequiredParam, paramName),
			},
		},
		"Unsupported": {
			reason: "An error should be returned when an unsupported parameter value is supplied",
			args: args{
				cp: []v1alpha2.ComponentParameter{},
				cpv: []v1alpha2.ComponentParameterValue{
					{
						Name: paramName,
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtUnsupportedParam, paramName),
			},
		},
		"MissingNotRequired": {
			reason: "Nothing should be returned when an optional parameter is omitted",
			args: args{
				cp: []v1alpha2.ComponentParameter{
					{
						Name: paramName,
					},
				},
				cpv: []v1alpha2.ComponentParameterValue{},
			},
			want: want{},
		},
		"SupportedAndSet": {
			reason: "A parameter should be returned when it is supported and set",
			args: args{
				cp: []v1alpha2.ComponentParameter{
					{
						Name:       paramName,
						FieldPaths: paths,
					},
				},
				cpv: []v1alpha2.ComponentParameterValue{
					{
						Name:  paramName,
						Value: intstr.FromString(value),
					},
				},
			},
			want: want{
				p: []Parameter{
					{
						Name:       paramName,
						FieldPaths: paths,
						Value:      intstr.FromString(value),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := resolve(tc.args.cp, tc.args.cpv)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nresolve(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.p, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nresolve(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

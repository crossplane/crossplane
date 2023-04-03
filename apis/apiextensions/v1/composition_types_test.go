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

package v1

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestComposedTemplate_GetBaseObject(t *testing.T) {
	type args struct {
		ct *ComposedTemplate
	}
	type want struct {
		output client.Object
		err    error
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidBaseObject": {
			reason: "Valid base object should be parsed properly",
			args: args{
				ct: &ComposedTemplate{
					Base: runtime.RawExtension{
						Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"foo"}}`),
					},
				},
			},
			want: want{
				output: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name": "foo",
						},
					},
				},
			},
		},
		"InvalidBaseObject": {
			reason: "Invalid base object should return an error",
			args: args{
				ct: &ComposedTemplate{
					Base: runtime.RawExtension{
						Raw: []byte(`{$$$WRONG$$$:"v1","kind":"Service","metadata":{"name":"foo"}}`),
					},
				},
			},
			want: want{
				err: xperrors.Wrap(errors.New("invalid character '$' looking for beginning of object key string"), errUnableToParse),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tc.args.ct.GetBaseObject()
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetBaseObject(...): -want error, +got error: \n%s", tc.reason, diff)
				return
			}
			if diff := cmp.Diff(tc.want.output, got); diff != "" {
				t.Errorf("\n%s\nGetBaseObject(...): -want, +got: \n%s", tc.reason, diff)
			}
		})
	}
}

func TestReadinessCheck_Validate(t *testing.T) {
	type args struct {
		r *ReadinessCheck
	}
	type want struct {
		output *field.Error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidTypeNone": {
			reason: "Type none should be valid",
			args: args{
				r: &ReadinessCheck{
					Type: ReadinessCheckTypeNone,
				},
			},
		},
		"ValidTypeMatchString": {
			reason: "Type matchString should be valid",
			args: args{
				r: &ReadinessCheck{
					Type:        ReadinessCheckTypeMatchString,
					MatchString: "foo",
					FieldPath:   "spec.foo",
				},
			},
		},
		"InvalidType": {
			reason: "Invalid type",
			args: args{
				r: &ReadinessCheck{
					Type: "foo",
				},
			},
			want: want{
				output: &field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "type",
					BadValue: "foo",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.r.Validate()
			if diff := cmp.Diff(tc.want.output, got, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nValidate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

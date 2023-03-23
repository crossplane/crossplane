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
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestComposedTemplate_GetBaseObject(t *testing.T) {
	tests := []struct {
		name    string
		ct      *ComposedTemplate
		want    client.Object
		wantErr bool
	}{
		{
			name: "Valid base object",
			ct: &ComposedTemplate{
				Base: runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"foo"}}`),
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"name": "foo",
					},
				},
			},
		},
		{
			name: "Invalid base object",
			ct: &ComposedTemplate{
				Base: runtime.RawExtension{
					Raw: []byte(`{$$$WRONG$$$:"v1","kind":"Service","metadata":{"name":"foo"}}`),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ct.GetBaseObject()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBaseObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetBaseObject(...): -want, +got: \n%s", diff)
			}
		})
	}
}

func TestReadinessCheck_Validate(t *testing.T) {
	tests := []struct {
		name string
		r    *ReadinessCheck
		want *field.Error
	}{
		{
			name: "Valid type none",
			r: &ReadinessCheck{
				Type: ReadinessCheckTypeNone,
			},
		},
		{
			name: "Valid type matchLabels",
			r: &ReadinessCheck{
				Type:        ReadinessCheckTypeMatchString,
				MatchString: "foo",
				FieldPath:   "spec.foo",
			},
		},
		{
			name: "Invalid type",
			r: &ReadinessCheck{
				Type: "foo",
			},
			want: &field.Error{
				Type:     field.ErrorTypeInvalid,
				Field:    "type",
				BadValue: "foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Validate()
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("Validate(...): -want, +got:\n%s", diff)
			}
		})
	}
}

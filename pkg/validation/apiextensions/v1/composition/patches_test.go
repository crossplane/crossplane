/*
Copyright 2023 the Crossplane Authors.

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

package composition

import (
	"testing"

	"github.com/crossplane/crossplane/pkg/validation/schema"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func Test_validateTransforms(t *testing.T) {
	type args struct {
		transforms       []v1.Transform
		fromType, toType schema.KnownJSONType
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Should validate empty transforms to the same type successfully",
			args: args{
				transforms: []v1.Transform{},
				fromType:   "string",
				toType:     "string",
			},
		},
		{
			name:    "Should reject empty transforms to a different type",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{},
				fromType:   "string",
				toType:     "integer",
			},
		},
		{
			name: "Should accept empty transforms to a different type when its integer to number",
			args: args{
				transforms: []v1.Transform{},
				fromType:   "integer",
				toType:     "number",
			},
		},
		{
			name: "Should validate convert transforms successfully",
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
				},
				fromType: "string",
				toType:   "integer",
			},
		},
		{
			name: "Should validate convert integer to number transforms successfully",
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "float64",
						},
					},
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
				},
				fromType: "string",
				toType:   "number",
			},
		},
		{
			name:    "Should reject convert number to integer transforms",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "float64",
						},
					},
				},
				fromType: "string",
				toType:   "integer",
			},
		},
		{
			name: "Should validate multiple convert transforms successfully",
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "string",
						},
					},
				},
				fromType: "string",
				toType:   "string",
			},
		},
		{
			name:    "Should reject invalid transform types",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformType("doesnotexist"),
					},
				},
				fromType: "string",
				toType:   "string",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateTransformsIOTypes(tt.args.transforms, tt.args.fromType, tt.args.toType); (err != nil) != tt.wantErr {
				t.Errorf("validateTransformsIOTypes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

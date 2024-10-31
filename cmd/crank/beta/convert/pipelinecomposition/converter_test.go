/*
Copyright 2024 The Crossplane Authors.

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

package pipelinecomposition

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestConvertPnTToPipeline(t *testing.T) {
	type args struct {
		in           *unstructured.Unstructured
		functionName string
	}

	type want struct {
		out *unstructured.Unstructured
		err error
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessWithNoEnvironment": {
			reason: "Should successfully convert a Composition not using the environment.",
			args: args{
				in: fromYAML(t, `
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
   name: foo
spec:
   compositeTypeRef:
      apiVersion: example.crossplane.io/v1
      kind: XR
   mode: Resources
   patchSets:
   - name: patchset-0
     patches:
     - type: FromCompositeFieldPath
       fromFieldPath: "envVal"
       toFieldPath: "spec.val"
     - type: ToCompositeFieldPath
       fromFieldPath: "envVal"
       toFieldPath: "spec.val"
       policy:
         fromFieldPath: optional
         mergeOptions:
           keepMapValues: true
   resources:
   - name: bucket
     base:
       apiVersion: s3.aws.upbound.io/v1beta1
       kind: Bucket
       spec:
         forProvider:
           region: us-east-2
     patches:
       - type: FromEnvironmentFieldPath
         fromFieldPath: "someFieldInTheEnvironment"
         toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
       - type: ToEnvironmentFieldPath
         fromFieldPath: "status.someOtherFieldInTheResource"
         toFieldPath: "someOtherFieldInTheEnvironment"
   - # name: resource-1 # this should be defaulted
     base:
       apiVersion: s3.aws.upbound.io/v1beta1
       kind: Bucket
       spec:
         forProvider:
           region: us-east-2
     patches:
       - type: FromEnvironmentFieldPath
         fromFieldPath: "someFieldInTheEnvironment"
         toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
       - # type: FromCompositeFieldPath # this should be defaulted
         fromFieldPath: "status.someOtherFieldInTheResource"
         toFieldPath: "someOtherFieldInTheEnvironment"
`),
			},
			want: want{
				out: fromYAML(t, `
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
   name: foo
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  mode: Pipeline
  pipeline:
  - step: patch-and-transform
    functionRef:
      name: function-patch-and-transform
    input:
      apiVersion: pt.fn.crossplane.io/v1beta1
      kind: Resources
      patchSets:
      - name: patchset-0
        patches:
        - type: FromCompositeFieldPath
          fromFieldPath: "envVal"
          toFieldPath: "spec.val"
        - type: ToCompositeFieldPath
          fromFieldPath: "envVal"
          toFieldPath: "spec.val"
          policy:
            fromFieldPath: optional
            toFieldPath: MergeObjects
      resources:
      - name: bucket
        base:
          apiVersion: s3.aws.upbound.io/v1beta1
          kind: Bucket
          spec:
            forProvider:
              region: us-east-2
        patches:
        - type: FromEnvironmentFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
        - type: ToEnvironmentFieldPath
          fromFieldPath: "status.someOtherFieldInTheResource"
          toFieldPath: "someOtherFieldInTheEnvironment"
      - name: resource-1
        base:
          apiVersion: s3.aws.upbound.io/v1beta1
          kind: Bucket
          spec:
            forProvider:
              region: us-east-2
        patches:
        - type: FromEnvironmentFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
        - type: FromCompositeFieldPath
          fromFieldPath: "status.someOtherFieldInTheResource"
          toFieldPath: "someOtherFieldInTheEnvironment"
`),
			},
		},
		"SuccessWithEnvironmentPatches": {
			reason: "Should successfully convert a Composition using environment patches, preserving other fields in the environment.",
			args: args{
				in: fromYAML(t, `
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
   name: foo
spec:
   compositeTypeRef:
      apiVersion: example.crossplane.io/v1
      kind: XR
   mode: Resources
   patchSets:
   - name: patchset-0
     patches:
     - type: FromCompositeFieldPath
       fromFieldPath: "envVal"
       toFieldPath: "spec.val"
     - type: ToCompositeFieldPath
       fromFieldPath: "envVal"
       toFieldPath: "spec.val"
       policy:
         fromFieldPath: optional
         mergeOptions:
           keepMapValues: true
   environment:
      defaultData:
        foo: bar
      environmentConfigs:
      - type: Reference
        ref:
           name: example-config
      patches:
      - type: ToCompositeFieldPath
        fromFieldPath: "someFieldInTheEnvironment"
        toFieldPath: "status.someFieldFromTheEnvironment"
      - # type: FromCompositeFieldPath # this should be defaulted
        fromFieldPath: "spec.someFieldInTheXR"
        toFieldPath: "someFieldFromTheXR"
   resources:
   - name: bucket
     base:
       apiVersion: s3.aws.upbound.io/v1beta1
       kind: Bucket
       spec:
         forProvider:
           region: us-east-2
     patches:
       - type: FromEnvironmentFieldPath
         fromFieldPath: "someFieldInTheEnvironment"
         toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
       - type: ToEnvironmentFieldPath
         fromFieldPath: "status.someOtherFieldInTheResource"
         toFieldPath: "someOtherFieldInTheEnvironment"
   - # name: resource-1 # this should be defaulted
     base:
       apiVersion: s3.aws.upbound.io/v1beta1
       kind: Bucket
       spec:
         forProvider:
           region: us-east-2
     patches:
       - type: FromEnvironmentFieldPath
         fromFieldPath: "someFieldInTheEnvironment"
         toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
       - # type: FromCompositeFieldPath # this should be defaulted
         fromFieldPath: "status.someOtherFieldInTheResource"
         toFieldPath: "someOtherFieldInTheEnvironment"
`),
			},
			want: want{
				out: fromYAML(t, `
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
   name: foo
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  environment:
    defaultData:
      foo: bar
    environmentConfigs:
    - type: Reference
      ref:
        name: example-config
  mode: Pipeline
  pipeline:
  - step: patch-and-transform
    functionRef:
      name: function-patch-and-transform
    input:
      apiVersion: pt.fn.crossplane.io/v1beta1
      kind: Resources
      patchSets:
      - name: patchset-0
        patches:
        - type: FromCompositeFieldPath
          fromFieldPath: "envVal"
          toFieldPath: "spec.val"
        - type: ToCompositeFieldPath
          fromFieldPath: "envVal"
          toFieldPath: "spec.val"
          policy:
            fromFieldPath: optional
            toFieldPath: MergeObjects
      environment:
        patches:
        - type: ToCompositeFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "status.someFieldFromTheEnvironment"
        - type: FromCompositeFieldPath
          fromFieldPath: "spec.someFieldInTheXR"
          toFieldPath: "someFieldFromTheXR"
      resources:
      - name: bucket
        base:
          apiVersion: s3.aws.upbound.io/v1beta1
          kind: Bucket
          spec:
            forProvider:
              region: us-east-2
        patches:
        - type: FromEnvironmentFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
        - type: ToEnvironmentFieldPath
          fromFieldPath: "status.someOtherFieldInTheResource"
          toFieldPath: "someOtherFieldInTheEnvironment"
      - name: resource-1
        base:
          apiVersion: s3.aws.upbound.io/v1beta1
          kind: Bucket
          spec:
            forProvider:
              region: us-east-2
        patches:
        - type: FromEnvironmentFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
        - type: FromCompositeFieldPath
          fromFieldPath: "status.someOtherFieldInTheResource"
          toFieldPath: "someOtherFieldInTheEnvironment"
`),
			},
		},
		"SuccessWithNoPatchSets": {
			reason: "Should successfully convert a Composition not using PatchSets.",
			args: args{
				in: fromYAML(t, `
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
   name: foo
spec:
   compositeTypeRef:
      apiVersion: example.crossplane.io/v1
      kind: XR
   mode: Resources
   environment:
      defaultData:
        foo: bar
      environmentConfigs:
      - type: Reference
        ref:
           name: example-config
      patches:
      - type: ToCompositeFieldPath
        fromFieldPath: "someFieldInTheEnvironment"
        toFieldPath: "status.someFieldFromTheEnvironment"
      - # type: FromCompositeFieldPath # this should be defaulted
        fromFieldPath: "spec.someFieldInTheXR"
        toFieldPath: "someFieldFromTheXR"
   resources:
   - name: bucket
     base:
       apiVersion: s3.aws.upbound.io/v1beta1
       kind: Bucket
       spec:
         forProvider:
           region: us-east-2
     patches:
       - type: FromEnvironmentFieldPath
         fromFieldPath: "someFieldInTheEnvironment"
         toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
       - type: ToEnvironmentFieldPath
         fromFieldPath: "status.someOtherFieldInTheResource"
         toFieldPath: "someOtherFieldInTheEnvironment"
   - # name: resource-1 # this should be defaulted
     base:
       apiVersion: s3.aws.upbound.io/v1beta1
       kind: Bucket
       spec:
         forProvider:
           region: us-east-2
     patches:
       - type: FromEnvironmentFieldPath
         fromFieldPath: "someFieldInTheEnvironment"
         toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
       - # type: FromCompositeFieldPath # this should be defaulted
         fromFieldPath: "status.someOtherFieldInTheResource"
         toFieldPath: "someOtherFieldInTheEnvironment"
`),
			},
			want: want{
				out: fromYAML(t, `
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
   name: foo
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  environment:
    defaultData:
      foo: bar
    environmentConfigs:
    - type: Reference
      ref:
        name: example-config
  mode: Pipeline
  pipeline:
  - step: patch-and-transform
    functionRef:
      name: function-patch-and-transform
    input:
      apiVersion: pt.fn.crossplane.io/v1beta1
      kind: Resources
      environment:
        patches:
        - type: ToCompositeFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "status.someFieldFromTheEnvironment"
        - type: FromCompositeFieldPath
          fromFieldPath: "spec.someFieldInTheXR"
          toFieldPath: "someFieldFromTheXR"
      resources:
      - name: bucket
        base:
          apiVersion: s3.aws.upbound.io/v1beta1
          kind: Bucket
          spec:
            forProvider:
              region: us-east-2
        patches:
        - type: FromEnvironmentFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
        - type: ToEnvironmentFieldPath
          fromFieldPath: "status.someOtherFieldInTheResource"
          toFieldPath: "someOtherFieldInTheEnvironment"
      - name: resource-1
        base:
          apiVersion: s3.aws.upbound.io/v1beta1
          kind: Bucket
          spec:
            forProvider:
              region: us-east-2
        patches:
        - type: FromEnvironmentFieldPath
          fromFieldPath: "someFieldInTheEnvironment"
          toFieldPath: "spec.forProvider.someFieldFromTheEnvironment"
        - type: FromCompositeFieldPath
          fromFieldPath: "status.someOtherFieldInTheResource"
          toFieldPath: "someOtherFieldInTheEnvironment"
`),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := convertPnTToPipeline(tt.args.in, tt.args.functionName)
			if diff := cmp.Diff(tt.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("convertPnTToPipeline() %s error -want, +got:\n%s", tt.reason, diff)
			}
			if diff := cmp.Diff(tt.want.out, got); diff != "" {
				t.Errorf("convertPnTToPipeline() %s -want, +got:\n%s", tt.reason, diff)
			}
		})
	}
}

func fromYAML(t *testing.T, in string) *unstructured.Unstructured {
	t.Helper()
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(in), &obj)
	if err != nil {
		t.Fatalf("fromYAML: %s", err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestSetMissingConnectionDetailFields(t *testing.T) {
	kubeconfigKey := "kubeconfig"
	fv := v1.ConnectionDetailTypeFromValue
	ffp := v1.ConnectionDetailTypeFromFieldPath
	fcsk := v1.ConnectionDetailTypeFromConnectionSecretKey
	type args struct {
		sk v1.ConnectionDetail
	}
	type want struct {
		sk v1.ConnectionDetail
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ConnectionDetailMissingKeyAndName": {
			reason: "Correctly add Type and Name",
			args: args{
				sk: v1.ConnectionDetail{
					FromConnectionSecretKey: &kubeconfigKey,
				},
			},
			want: want{
				sk: v1.ConnectionDetail{
					Name:                    &kubeconfigKey,
					FromConnectionSecretKey: &kubeconfigKey,
					Type:                    &fcsk,
				},
			},
		},
		"FromValueMissingType": {
			reason: "Correctly add Type",
			args: args{
				sk: v1.ConnectionDetail{
					Name:  &kubeconfigKey,
					Value: &kubeconfigKey,
				},
			},
			want: want{
				sk: v1.ConnectionDetail{
					Name:  &kubeconfigKey,
					Value: &kubeconfigKey,
					Type:  &fv,
				},
			},
		},
		"FromFieldPathMissingType": {
			reason: "Correctly add Type",
			args: args{
				sk: v1.ConnectionDetail{
					Name:          &kubeconfigKey,
					FromFieldPath: &kubeconfigKey,
				},
			},
			want: want{
				sk: v1.ConnectionDetail{
					Name:          &kubeconfigKey,
					FromFieldPath: &kubeconfigKey,
					Type:          &ffp,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sk := setMissingConnectionDetailFields(tc.args.sk)
			if diff := cmp.Diff(tc.want.sk, sk); diff != "" {
				t.Errorf("%s\nsetMissingConnectionDetailFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetTransformTypeRequiredFields(t *testing.T) {
	group := 1
	mult := int64(1024)
	tobase64 := v1.StringConversionTypeToBase64
	type args struct {
		tt v1.Transform
	}
	type want struct {
		tt v1.Transform
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MathMultiplyMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					Math: &v1.MathTransform{Multiply: &mult},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{Multiply: &mult, Type: v1.MathTransformTypeMultiply},
				},
			},
		},
		"MathClampMinMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					Math: &v1.MathTransform{ClampMin: &mult},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{
						ClampMin: &mult,
						Type:     v1.MathTransformTypeClampMin,
					},
				},
			},
		},
		"MathClampMaxMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					Math: &v1.MathTransform{ClampMax: &mult},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{
						ClampMax: &mult,
						Type:     v1.MathTransformTypeClampMax,
					},
				},
			},
		},
		"StringConvertMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					String: &v1.StringTransform{
						Convert: &tobase64,
					},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type:    v1.StringTransformTypeConvert,
						Convert: &tobase64,
					},
				},
			},
		},
		"StringRegexMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					String: &v1.StringTransform{
						Regexp: &v1.StringTransformRegexp{
							Match: "'^eu-(.*)-'",
							Group: &group,
						},
					},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type: v1.StringTransformTypeRegexp,
						Regexp: &v1.StringTransformRegexp{
							Match: "'^eu-(.*)-'",
							Group: &group,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tt := setTransformTypeRequiredFields(tc.args.tt)
			if diff := cmp.Diff(tc.want.tt, tt); diff != "" {
				t.Errorf("%s\nsetTransformTypeRequiredFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMigratePatch(t *testing.T) {
	type args struct {
		p map[string]interface{}
	}
	type want struct {
		p   map[string]interface{}
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"TransformArrayMissingFields": {
			reason: "Nested missing Types are filled in for a transform array",
			args: args{
				p: fromYAML(t, `
type: FromCompositeFieldPath
fromFieldPath: spec.id
toFieldPath: spec.id
transforms:
- string:
    fmt: test3-%s
- math:
    multiply: 1`,
				).UnstructuredContent(),
			},
			want: want{
				p: fromYAML(t, `
type: FromCompositeFieldPath
fromFieldPath: spec.id
toFieldPath: spec.id
transforms:
- type: string
  string:
    type: Format
    fmt: test3-%s
- type: math
  math:
    type: Multiply
    multiply: 1`,
				).UnstructuredContent(),
			},
		},
		"PatchWithoutTransforms": {
			args: args{
				p: fromYAML(t, `
type: FromCompositeFieldPath
fromFieldPath: spec.id
toFieldPath: spec.id
`).UnstructuredContent(),
			},
			want: want{
				p: fromYAML(t, `
type: FromCompositeFieldPath
fromFieldPath: spec.id
toFieldPath: spec.id
`).UnstructuredContent(),
			},
		},
		"PatchWithTransforms": {
			reason: "Nested missing Types are filled in for a transform array",
			args: args{
				p: fromYAML(t, `
type: FromCompositeFieldPath
fromFieldPath: spec.id
toFieldPath: spec.id
transforms:
- string:
    fmt: test3-%s
- math:
    multiply: 1
`).UnstructuredContent(),
			},
			want: want{
				p: fromYAML(t, `
type: FromCompositeFieldPath
fromFieldPath: spec.id
toFieldPath: spec.id
transforms:
- type: string
  string:
    type: Format
    fmt: test3-%s
- type: math
  math:
    type: Multiply
    multiply: 1
`).UnstructuredContent(),
			},
		},
		"PatchWithoutType": {
			args: args{
				p: fromYAML(t, `
fromFieldPath: spec.id
toFieldPath: spec.id
`).UnstructuredContent(),
			},
			want: want{
				p: fromYAML(t, `
type: FromCompositeFieldPath
fromFieldPath: spec.id
toFieldPath: spec.id
`).UnstructuredContent(),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := migratePatch(tc.args.p)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nsetMissingPatchSetFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.p, got.UnstructuredContent(),
				cmp.FilterValues(func(x, y interface{}) bool {
					isNumeric := func(v interface{}) bool {
						return v != nil && reflect.TypeOf(v).ConvertibleTo(reflect.TypeOf(float64(0)))
					}
					return isNumeric(x) && isNumeric(y)
				}, cmp.Transformer("string", func(in interface{}) float64 {
					return reflect.ValueOf(in).Convert(reflect.TypeOf(float64(0))).Float()
				}))); diff != "" {
				t.Errorf("%s\nsetMissingPatchSetFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMigrateResource(t *testing.T) {
	type args struct {
		r map[string]interface{}
		i int
	}
	type want struct {
		r   map[string]interface{}
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoNameProvided": {
			reason: "ResourceName Not provided",
			args: args{
				i: 42,
				r: fromYAML(t, `
base:
  apiVersion: nop.crossplane.io/v1
  kind: TestResource
  spec: {}
`).UnstructuredContent(),
			},
			want: want{
				r: fromYAML(t, `
name: resource-42
base:
  apiVersion: nop.crossplane.io/v1
  kind: TestResource
  spec: {}
`).UnstructuredContent(),
			},
		},
		"EmptyNameProvided": {
			reason: "ResourceName Not provided",
			args: args{
				i: 42,
				r: fromYAML(t, `
name: ""
base:
  apiVersion: nop.crossplane.io/v1
  kind: TestResource
  spec: {}
`).UnstructuredContent(),
			},
			want: want{
				r: fromYAML(t, `
name: resource-42
base:
  apiVersion: nop.crossplane.io/v1
  kind: TestResource
  spec: {}
`).UnstructuredContent(),
			},
		},
		"NameProvidedWithConnectionDetail": {
			reason: "ResourceName Not provided",
			args: args{
				i: 42,
				r: fromYAML(t, `
name: foo
base:
  apiVersion: nop.crossplane.io/v1
  kind: TestResource
  spec: {}
connectionDetails:
- fromConnectionSecretKey: crossplane
`).UnstructuredContent(),
			},
			want: want{
				r: fromYAML(t, `
name: foo
base:
  apiVersion: nop.crossplane.io/v1
  kind: TestResource
  spec: {}
connectionDetails:
- fromConnectionSecretKey: crossplane
  type: FromConnectionSecretKey
`).UnstructuredContent(),
			},
		},
		"ResourcesHasSimplePatches": {
			args: args{
				i: 43,
				r: fromYAML(t, `
name: bar
patches:
- type: ToCompositeFieldPath
  fromFieldPath: envVal
  toFieldPath: spec.val
- type: ToCompositeFieldPath
  fromFieldPath: envVal
  toFieldPath: spec.val
  policy:
    fromFieldPath: optional
    mergeOptions:
      keepMapValues: true
`).UnstructuredContent(),
			},
			want: want{
				r: fromYAML(t, `
name: bar
patches:
- type: ToCompositeFieldPath
  fromFieldPath: envVal
  toFieldPath: spec.val
- type: ToCompositeFieldPath
  fromFieldPath: envVal
  toFieldPath: spec.val
  policy:
    fromFieldPath: optional
    toFieldPath: MergeObjects
`).UnstructuredContent(),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := migrateResource(tc.want.r, tc.args.i)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nmigrateResource(...): -want i, +got i:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got.UnstructuredContent(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nmigrateResource(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMigrateMergeOptions(t *testing.T) {
	/*
	   #	MergeOptions    appendSlice     keepMapValues   policy.toFieldPath
	   1	nil             N/A             n/A             nil (defaults to Replace)
	   2	non-nil         nil or false    true            MergeObjects
	   3	non-nil         true            nil or false    ForceMergeObjectsAppendArrays
	   4	non-nil         nil or false    nil or false    ForceMergeObjects
	   5	non-nil         true            True            MergeObjectsAppendArrays
	*/
	cases := map[string]struct {
		reason string
		args   *commonv1.MergeOptions
		want   *ToFieldPathPolicy
	}{
		"Nil": { // case 1
			reason: "MergeOptions is nil",
			args:   nil,
			want:   nil,
		},
		"KeepMapValuesTrueAppendSliceNil": { // case 2.a
			reason: "AppendSlice is nil && KeepMapValues is true",
			args: &commonv1.MergeOptions{
				KeepMapValues: ptr.To(true),
			},
			want: ptr.To(ToFieldPathPolicyMergeObjects),
		},
		"KeepMapValuesTrueAppendSliceFalse": { // case 2.b
			reason: "AppendSlice is false && KeepMapValues is true",
			args: &commonv1.MergeOptions{
				KeepMapValues: ptr.To(true),
				AppendSlice:   ptr.To(false),
			},
			want: ptr.To(ToFieldPathPolicyMergeObjects),
		},
		"KeepMapValuesNilAppendSliceTrue": { // case 3.a
			reason: "AppendSlice is true && KeepMapValues is nil",
			args: &commonv1.MergeOptions{
				AppendSlice: ptr.To(true),
			},
			want: ptr.To(ToFieldPathPolicyForceMergeObjectsAppendArrays),
		},
		"AppendSliceTrueKeepMapValuesFalse": { // case 3.b
			reason: "AppendSlice is true && KeepMapValues is false",
			args: &commonv1.MergeOptions{
				AppendSlice:   ptr.To(true),
				KeepMapValues: ptr.To(false),
			},
			want: ptr.To(ToFieldPathPolicyForceMergeObjectsAppendArrays),
		},
		"Empty": { // case 4.a
			reason: "Both AppendSlice and KeepMapValues are nil",
			args:   &commonv1.MergeOptions{},
			want:   ptr.To(ToFieldPathPolicyForceMergeObjects),
		},
		"KeepMapValuesNilAppendSliceFalse": { // case 4.b
			reason: "AppendSlice is false and KeepMapValues is nil",
			args: &commonv1.MergeOptions{
				AppendSlice: ptr.To(false),
			},
			want: ptr.To(ToFieldPathPolicyForceMergeObjects),
		},
		"AppendSliceNilKeepMapValuesFalse": { // case 4.c
			reason: "AppendSlice is nil and KeepMapValues is false",
			args: &commonv1.MergeOptions{
				KeepMapValues: ptr.To(false),
			},
			want: ptr.To(ToFieldPathPolicyForceMergeObjects),
		},
		"ApepndSliceFalseKeepMapValuesFalse": { // case 4.d
			reason: "AppendSlice is false and KeepMapValues is false",
			args: &commonv1.MergeOptions{
				AppendSlice:   ptr.To(false),
				KeepMapValues: ptr.To(false),
			},
			want: ptr.To(ToFieldPathPolicyForceMergeObjects),
		},
		"AppendSliceTrueKeepMapValuesTrue": { // case 5
			reason: "AppendSlice is true and KeepMapValues is true",
			args: &commonv1.MergeOptions{
				AppendSlice:   ptr.To(true),
				KeepMapValues: ptr.To(true),
			},
			want: ptr.To(ToFieldPathPolicyMergeObjectsAppendArrays),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := migrateMergeOptions(tc.args)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nmigrateMergeOptions(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

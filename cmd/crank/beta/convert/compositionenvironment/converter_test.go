package compositionenvironment

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestConvertToFunctionEnvironmentConfigs(t *testing.T) {
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
		"Success": {
			reason: "Should successfully convert a Composition to use function-environment-configs.",
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
  mode: Pipeline
  environment:
    policy:
      resolution: Required
      resolve: Always
    defaultData:
      foo:
        bar: baz
      key: value
    environmentConfigs:
    - type: Reference
      ref:
        name: example-config
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
      resources:
      - name: bucket
        base:
          apiVersion: s3.aws.crossplane.io/v1beta1
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
          toFieldPath: "someOtherFieldInTheEnvironment"`),
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
  - step: environment-configs
    functionRef:
      name: function-environment-configs
    input:
      apiVersion: environmentconfigs.fn.crossplane.io/v1beta1
      kind: Input
      spec:
        policy:
          resolution: Required
        defaultData:
          foo:
            bar: baz
          key: value
        environmentConfigs:
        - type: Reference
          ref:
            name: example-config
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
      resources:
      - name: bucket
        base:
          apiVersion: s3.aws.crossplane.io/v1beta1
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
`),
			},
		},
		"SuccessWithNoEnvironment": {
			reason: "Should do nothing if the Composition has no environment defined.",
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
      resources:
      - name: bucket
        base:
          apiVersion: s3.aws.crossplane.io/v1beta1
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
          toFieldPath: "someOtherFieldInTheEnvironment"`),
			},
			want: want{
				out: nil,
			},
		},
		"FailWithResources": {
			reason: "Should refuse to convert a Composition that still uses Resources mode.",
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
`),
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ConvertToFunctionEnvironmentConfigs(tt.args.in, tt.args.functionName)
			if diff := cmp.Diff(tt.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("ConvertToFunctionEnvironmentConfigs() %s error -want, +got:\n%s", tt.reason, diff)
			}
			if diff := cmp.Diff(tt.want.out, got); diff != "" {
				t.Errorf("ConvertToFunctionEnvironmentConfigs() %s -want, +got:\n%s", tt.reason, diff)
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

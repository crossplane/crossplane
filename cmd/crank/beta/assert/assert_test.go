package assert

import (
	"bytes"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAssert(t *testing.T) {
	type args struct {
		expectedResources []*unstructured.Unstructured
		actualResources   []*unstructured.Unstructured
		skipSuccessLogs   bool
	}
	type want struct {
		output string
		err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MatchOnName": {
			reason: "Should match resources based on name",
			args: args{
				expectedResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "match",
							},
							"spec": map[string]interface{}{
								"replicas": 3,
								"config": map[string]interface{}{
									"key": "value",
								},
							},
						},
					},
				},
				actualResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "not-match-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "match",
							},
							"spec": map[string]interface{}{
								"replicas": 3,
								"config": map[string]interface{}{
									"key": "value",
								},
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "not-match-2",
							},
						},
					},
				},
				skipSuccessLogs: false,
			},
			want: want{
				output: "[✓] test.org/v1, Kind=Test, Name=match asserted successfully\n",
				err:    nil,
			},
		},
		"MatchOnLabels": {
			reason: "Should match resources based on labels when name is not provided",
			args: args{
				expectedResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"app": "myapp",
									"env": "prod",
								},
							},
							"spec": map[string]interface{}{
								"replicas": 3,
							},
						},
					},
				},
				actualResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test1",
								"labels": map[string]interface{}{
									"app":     "myapp",
									"env":     "prod",
									"version": "v1",
								},
							},
							"spec": map[string]interface{}{
								"replicas": 3,
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test2",
								"labels": map[string]interface{}{
									"app": "otherapp",
									"env": "prod",
								},
							},
							"spec": map[string]interface{}{
								"replicas": 2,
							},
						},
					},
				},
				skipSuccessLogs: false,
			},
			want: want{
				output: "[✓] test.org/v1, Kind=Test, Labels=[app: myapp, env: prod] asserted successfully\n",
				err:    nil,
			},
		},
		"MatchOnGVK": {
			reason: "Should match resources based on GVK when name and labels are not provided",
			args: args{
				expectedResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"spec": map[string]interface{}{
								"replicas": 3,
							},
						},
					},
				},
				actualResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test1",
							},
							"spec": map[string]interface{}{
								"replicas": 3,
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "OtherTest",
							"metadata": map[string]interface{}{
								"name": "test2",
							},
							"spec": map[string]interface{}{
								"replicas": 2,
							},
						},
					},
				},
				skipSuccessLogs: false,
			},
			want: want{
				output: "[✓] test.org/v1, Kind=Test asserted successfully\n",
				err:    nil,
			},
		},
		"MatchingResources": {
			reason: "Should match resources with complex spec data",
			args: args{
				expectedResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "ComplexTest",
							"metadata": map[string]interface{}{
								"name": "complex1",
							},
							"spec": map[string]interface{}{
								"replicas": 3,
								"config": map[string]interface{}{
									"key1": "value1",
									"key2": 42,
								},
								"ports": []interface{}{
									map[string]interface{}{
										"name": "http",
										"port": 80,
									},
									map[string]interface{}{
										"name": "https",
										"port": 443,
									},
								},
							},
						},
					},
				},
				actualResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "ComplexTest",
							"metadata": map[string]interface{}{
								"name": "complex1",
							},
							"spec": map[string]interface{}{
								"replicas": 3,
								"config": map[string]interface{}{
									"key1": "value1",
									"key2": 42,
								},
								"ports": []interface{}{
									map[string]interface{}{
										"name": "http",
										"port": 80,
									},
									map[string]interface{}{
										"name": "https",
										"port": 443,
									},
								},
							},
						},
					},
				},
				skipSuccessLogs: false,
			},
			want: want{
				output: "[✓] test.org/v1, Kind=ComplexTest, Name=complex1 asserted successfully\n",
				err:    nil,
			},
		},
		"ValueMismatch": {
			reason: "Should report value mismatches",
			args: args{
				expectedResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"metadata": map[string]interface{}{
							"name": "test1",
						},
						"spec": map[string]interface{}{
							"replicas": 3,
							"config": map[string]interface{}{
								"key": "value",
							},
							"ports": []interface{}{
								map[string]interface{}{
									"name": "http",
									"port": 80,
								},
								map[string]interface{}{
									"name": "https",
									"port": 443,
								},
							},
						},
					},
				}},
				actualResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"metadata": map[string]interface{}{
							"name": "test1",
						},
						"spec": map[string]interface{}{
							"replicas": 2,
							"config": map[string]interface{}{
								"key": "different",
							},
							"ports": []interface{}{
								map[string]interface{}{
									"name": "http",
									"port": 8080,
								},
								map[string]interface{}{
									"name": "https",
									"port": 443,
								},
							},
						},
					},
				}},
			},
			want: want{
				output: "[x] test.org/v1, Kind=Test, Name=test1\n" +
					" - spec.config.key: value mismatch: expected value, got different\n" +
					" - spec.ports.[0].port: value mismatch: expected 80, got 8080\n" +
					" - spec.replicas: value mismatch: expected 3, got 2\n",
				err: nil,
			},
		},
		"TypeMismatch": {
			reason: "Should report type mismatches",
			args: args{
				expectedResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"metadata": map[string]interface{}{
							"name": "test1",
						},
						"spec": map[string]interface{}{
							"replicas": 3,
						},
					},
				}},
				actualResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"metadata": map[string]interface{}{
							"name": "test1",
						},
						"spec": map[string]interface{}{
							"replicas": "3",
						},
					},
				}},
			},
			want: want{
				output: "[x] test.org/v1, Kind=Test, Name=test1\n" +
					" - spec.replicas: type mismatch: expected int, got string\n",
				err: nil,
			},
		},
		"ArrayLengthMismatch": {
			reason: "Should report array length mismatches",
			args: args{
				expectedResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"metadata": map[string]interface{}{
							"name": "test1",
						},
						"spec": map[string]interface{}{
							"ports": []interface{}{80, 443},
						},
					},
				}},
				actualResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"metadata": map[string]interface{}{
							"name": "test1",
						},
						"spec": map[string]interface{}{
							"ports": []interface{}{80},
						},
					},
				}},
			},
			want: want{
				output: "[x] test.org/v1, Kind=Test, Name=test1\n" +
					" - spec.ports: expected an array of length 2, but got an array of length 1\n",
				err: nil,
			},
		},
		"MissingKey": {
			reason: "Should report missing key",
			args: args{
				expectedResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"spec": map[string]interface{}{
							"replicas": 3,
						},
					},
				}},
				actualResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"spec":       map[string]interface{}{},
					},
				},
				}},
			want: want{
				output: "[x] test.org/v1, Kind=Test\n" +
					" - spec.replicas: key is missing from map\n",
				err: nil,
			},
		},
		"MissingResource": {
			reason: "Should report missing resources",
			args: args{
				expectedResources: []*unstructured.Unstructured{{
					Object: map[string]interface{}{
						"apiVersion": "test.org/v1",
						"kind":       "Test",
						"metadata": map[string]interface{}{
							"name": "test1",
						},
					},
				}},
				actualResources: []*unstructured.Unstructured{},
			},
			want: want{
				output: "[x] test.org/v1, Kind=Test, Name=test1\n" +
					" - resource is missing\n",
				err: nil,
			},
		},
		"SkipSuccessLogs": {
			reason: "Should skip success logs but report mismatches when skipSuccessLogs is true",
			args: args{
				expectedResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test1",
							},
							"spec": map[string]interface{}{
								"replicas": 3,
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test2",
							},
							"spec": map[string]interface{}{
								"replicas": 2,
							},
						},
					},
				},
				actualResources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test1",
							},
							"spec": map[string]interface{}{
								"replicas": 3,
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "test.org/v1",
							"kind":       "Test",
							"metadata": map[string]interface{}{
								"name": "test2",
							},
							"spec": map[string]interface{}{
								"replicas": 4,
							},
						},
					},
				},
				skipSuccessLogs: true,
			},
			want: want{
				output: "[x] test.org/v1, Kind=Test, Name=test2\n" +
					" - spec.replicas: value mismatch: expected 2, got 4\n",
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := &bytes.Buffer{}
			err := Assert(tc.args.expectedResources, tc.args.actualResources, tc.args.skipSuccessLogs, w)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nAssert(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			got := w.String()
			if diff := cmp.Diff(tc.want.output, got); diff != "" {
				t.Errorf("%s\nAssert(...): -want output, +got output:\n%s", tc.reason, diff)
			}
		})
	}
}

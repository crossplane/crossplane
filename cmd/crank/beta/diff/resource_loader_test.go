package diff

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestLoadResources(t *testing.T) {

	type args struct {
		files []string
		stdin io.Reader
	}
	type want struct {
		resources []*unstructured.Unstructured
		err       error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SingleFile": {
			reason: "Should successfully load resources from a single file",
			args: args{
				files: []string{"testdata/file3.yaml"},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name": "test-resource",
							},
						},
					},
				},
			},
		},
		"MultipleFilesOnStdin": {
			reason: "Should successfully load multiple resources from stdin",
			args: args{
				stdin: strings.NewReader(`
apiVersion: example.org/v1
kind: Resource
metadata:
  name: stdin-resource-1
---
apiVersion: example.org/v1
kind: Resource
metadata:
  name: stdin-resource-2
`),
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name": "stdin-resource-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name": "stdin-resource-2",
							},
						},
					},
				},
			},
		},
		"MultipleFiles": {
			reason: "Should successfully load resources from multiple files",
			args: args{
				files: []string{"testdata/file1.yaml", "testdata/file2.yaml"},
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name": "test-resource-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name": "test-resource-2",
							},
						},
					},
				},
			},
		},
		"FilesAndStdin": {
			reason: "Should successfully load resources from files and stdin when using --",
			args: args{
				files: []string{"testdata/file1.yaml", "--"},
				stdin: strings.NewReader(`
apiVersion: example.org/v1
kind: Resource
metadata:
  name: stdin-resource-1
`),
			},
			want: want{
				resources: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name": "stdin-resource-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "example.org/v1",
							"kind":       "Resource",
							"metadata": map[string]interface{}{
								"name": "test-resource-1",
							},
						},
					},
				},
			},
		},
		"InvalidYAML": {
			reason: "Should return error for invalid YAML",
			args: args{
				stdin: strings.NewReader(`{`),
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NoResourcesFound": {
			reason: "Should return error when no resources are found",
			args: args{
				stdin: strings.NewReader(``),
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MissingMetadata": {
			reason: "Should return error for resource missing apiVersion or kind",
			args: args{
				stdin: strings.NewReader(`
metadata:
  name: test-resource
`),
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Set up stdin for the test
			oldStdin := os.Stdin
			if tc.args.stdin != nil {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				os.Stdin = r
				go func() {
					_, _ = io.Copy(w, tc.args.stdin)
					w.Close()
				}()
			}

			got, err := LoadResources(tc.args.files)
			os.Stdin = oldStdin

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoadResources(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.resources, got); diff != "" {
				t.Errorf("\n%s\nLoadResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

package printer

import (
	"errors"
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/google/go-cmp/cmp"
)

// Define a test for SaveGraph
func TestSaveGraph(t *testing.T) {
	type args struct {
		resource k8s.Resource
		fields   []string
		path     string
	}

	type want struct {
		fileExists bool
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		// Test valid resource
		"ResourceWithChildren": {
			reason: "Should created PNG file containing this structure: ObjectStorage -> XObjectStorage -> [Bucket, User]",
			args: args{
				resource: k8s.Resource{
					Manifest: k8s.DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					Event:    "Successfully selected composition",
					Children: []k8s.Resource{
						{
							Manifest: k8s.DummyManifest("XObjectStorage", "test-resource-hash", "True", "True"),
							Children: []k8s.Resource{
								{
									Manifest: k8s.DummyManifest("Bucket", "test-resource-bucket-hash", "True", "True"),
									Event:    "Synced bucket",
								},
								{
									Manifest: k8s.DummyManifest("User", "test-resource-user-hash", "True", "True"),
									Event:    "User ready",
								},
							},
						},
					},
				},
				fields: []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "event"},
				path:   "graph.png",
			},
			want: want{
				fileExists: true,
				err:        nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			// Remove file in case it already exists
			os.Remove("graph.png")

			// Create a GraphPrinter with a buffer writer
			graphPrinter := &GraphPrinter{}
			err := graphPrinter.SaveGraph(tc.args.resource, tc.args.fields, tc.args.path)
			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}

			// Check if png exists
			exists := true
			if _, err := os.Stat(tc.args.path); errors.Is(err, os.ErrNotExist) {
				exists = false
			}

			// Check if file exists
			if diff := cmp.Diff(tc.want.fileExists, exists); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}

		})

	}
}

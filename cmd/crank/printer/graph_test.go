package printer

import (
	"bytes"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/google/go-cmp/cmp"
)

// Define a test for PrintDotGraph
func TestPrintDotGraph(t *testing.T) {
	type args struct {
		resource k8s.Resource
		fields   []string
	}

	type want struct {
		dot_string string
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
					Manifest:           DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					LatestEventMessage: "Successfully selected composition",
					Children: []*k8s.Resource{
						{
							Manifest: DummyManifest("XObjectStorage", "test-resource-hash", "True", "True"),
							Children: []*k8s.Resource{
								{
									Manifest:           DummyManifest("Bucket", "test-resource-bucket-hash", "True", "True"),
									LatestEventMessage: "Synced bucket",
								},
								{
									Manifest:           DummyManifest("User", "test-resource-user-hash", "True", "True"),
									LatestEventMessage: "User ready",
								},
							},
						},
					},
				},
				fields: []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "event"},
			},
			want: want{
				dot_string: "graph  {\n\t\n\tn3[label=\"\\nname: test-resource-bucket-hash\\nkind: Bucket\\nnamespace: default\\napiversion: test.cloud/v1alpha1\\nsynced: True\\nready: True\\nevent: Synced bucket\",penwidth=\"2\"];\n\tn1[label=\"\\nname: test-resource\\nkind: ObjectStorage\\nnamespace: default\\napiversion: test.cloud/v1alpha1\\nsynced: True\\nready: True\\nevent: Successfully selected composition\",penwidth=\"2\"];\n\tn4[label=\"\\nname: test-resource-user-hash\\nkind: User\\nnamespace: default\\napiversion: test.cloud/v1alpha1\\nsynced: True\\nready: True\\nevent: User ready\",penwidth=\"2\"];\n\tn2[label=\"\\nname: test-resource-hash\\nkind: XObjectStorage\\nnamespace: default\\napiversion: test.cloud/v1alpha1\\nsynced: True\\nready: True\\nevent: \",penwidth=\"2\"];\n\tn1--n2;\n\tn2--n3;\n\tn2--n4;\n\t\n}\n",
				err:        nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a GraphPrinter
			p := &GraphPrinter{}
			var buf bytes.Buffer
			err := p.Print(&buf, tc.args.resource, tc.args.fields)
			got := buf.String()

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\ngraphPrinter.SaveGraph(): -want, +got:\n%s", tc.reason, diff)
			}

			// Check if dot_string is corrext
			if diff := cmp.Diff(tc.want.dot_string, got); diff != "" {
				t.Errorf("%s\ngraphPrinter.SaveGraph(): -want, +got:\n%s", tc.reason, diff)
			}

		})

	}
}

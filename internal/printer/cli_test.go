package printer

import (
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/internal/k8s"
	"github.com/google/go-cmp/cmp"
	"github.com/olekukonko/tablewriter"
)

func TestCliTable(t *testing.T) {
	type args struct {
		resource k8s.Resource
		fields   []string
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
		// Test valid resource
		"ResourceWithChildren": {
			reason: "CLI table should be able to print Resource struct containing children",
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
				fields: []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"},
			},
			want: want{
				output: `
+----------------+---------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
|     PARENT     |           NAME            |      KIND      | NAMESPACE |     APIVERSION      | SYNCED | READY | MESSAGE |             EVENT              |
+----------------+---------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
|                | test-resource             | ObjectStorage  | default   | test.cloud/v1alpha1 | True   | True  |         | Successfully selected          |
|                |                           |                |           |                     |        |       |         | composition                    |
| ObjectStorage  | test-resource-hash        | XObjectStorage | default   | test.cloud/v1alpha1 | True   | True  |         |                                |
| XObjectStorage | test-resource-bucket-hash | Bucket         | default   | test.cloud/v1alpha1 | True   | True  |         | Synced bucket                  |
| XObjectStorage | test-resource-user-hash   | User           | default   | test.cloud/v1alpha1 | True   | True  |         | User ready                     |
+----------------+---------------------------+----------------+-----------+---------------------+--------+-------+---------+--------------------------------+
				`,
				err: nil,
			},
		},
		// Single resource
		"SingleResource": {
			reason: "A single resource with no children",
			args: args{
				resource: k8s.Resource{
					Manifest: k8s.DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					Event:    "ObjectStorage is ready",
				},
				fields: []string{"parent", "name", "kind", "namespace", "apiversion", "synced", "ready", "message", "event"},
			},
			want: want{
				output: `
+--------+---------------+---------------+-----------+---------------------+--------+-------+---------+------------------------+
| PARENT |     NAME      |     KIND      | NAMESPACE |     APIVERSION      | SYNCED | READY | MESSAGE |         EVENT          |
+--------+---------------+---------------+-----------+---------------------+--------+-------+---------+------------------------+
|        | test-resource | ObjectStorage | default   | test.cloud/v1alpha1 | True   | True  |         | ObjectStorage is ready |
+--------+---------------+---------------+-----------+---------------------+--------+-------+---------+------------------------+				
				`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a strings.Builder to capture the output
			tableString := &strings.Builder{}

			// Build new table
			table := tablewriter.NewWriter(tableString)
			table.SetHeader(tc.args.fields)
			err := CliTableAddResource(table, tc.args.fields, tc.args.resource, "")

			// Capture the output of the table
			table.Render()
			got := tableString.String()

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}
			// Check table
			if diff := cmp.Diff(strings.TrimSpace(tc.want.output), strings.TrimSpace(got)); diff != "" {
				t.Errorf("%s\nExample(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

}

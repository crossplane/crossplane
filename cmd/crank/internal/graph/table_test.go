package graph

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestCliTable(t *testing.T) {
	type args struct {
		resource Resource
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
				resource: Resource{
					manifest:           DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					latestEventMessage: "Successfully selected composition",
					Children: []*Resource{
						{
							manifest: DummyManifest("XObjectStorage", "test-resource-hash", "True", "True"),
							Children: []*Resource{
								{
									manifest:           DummyManifest("Bucket", "test-resource-bucket-hash", "True", "True"),
									latestEventMessage: "Synced bucket",
								},
								{
									manifest:           DummyManifest("User", "test-resource-user-hash", "True", "True"),
									latestEventMessage: "User ready",
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
				resource: Resource{
					manifest:           DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					latestEventMessage: "ObjectStorage is ready",
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
			p := Table{}
			var buf bytes.Buffer
			err := p.Print(&buf, tc.args.resource, tc.args.fields)
			got := buf.String()

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nCliTableAddResource(): -want, +got:\n%s", tc.reason, diff)
			}
			// Check table
			if diff := cmp.Diff(strings.TrimSpace(tc.want.output), strings.TrimSpace(got)); diff != "" {
				t.Errorf("%s\nCliTableAddResource(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

}

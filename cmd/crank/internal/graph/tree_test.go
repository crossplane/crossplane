package graph

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestCliTree(t *testing.T) {
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
			reason: "CLI tree should be able to print Resource struct containing children",
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
				fields: []string{"name", "kind"},
			},
			want: want{
				// Note: Use spaces instead of tabs for intendation
				output: `
└─ Name: test-resource, Kind: ObjectStorage
  └─ Name: test-resource-hash, Kind: XObjectStorage
    ├─ Name: test-resource-bucket-hash, Kind: Bucket
    └─ Name: test-resource-user-hash, Kind: User		
				`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := Tree{
				Indent: "",
				IsLast: true,
			}
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

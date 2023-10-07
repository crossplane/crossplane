package k8s

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func TestDiagnose(t *testing.T) {

	// Test 2 cases
	// 1. All resources healthy
	// 2. Only a few unhealthy
	// 2.1. one not synced, one not ready, one both

	type args struct {
		resource Resource
	}

	type want struct {
		output Resource
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AllResourcesHealthy": {
			reason: "Pass only healthy resources to Diagnose. Check for false-positives.",
			args: args{
				resource: Resource{
					Manifest: DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					Event:    "Successfully selected composition",
					Children: []Resource{
						{
							Manifest: DummyManifest("XObjectStorage", "test-resource-hash", "True", "True"),
							Children: []Resource{
								{
									Manifest: DummyManifest("Bucket", "test-resource-bucket-hash", "True", "True"),
									Event:    "Synced bucket",
								},
								{
									Manifest: DummyManifest("User", "test-resource-user-hash", "True", "True"),
									Event:    "User ready",
								},
							},
						},
					},
				},
			},
			want: want{
				output: Resource{},
				err:    nil,
			},
		},
		"UnhealthyResources": {
			reason: "Pass unhealthy resources. Cases: 'Synced'=='False'; 'Ready'=='False'; Synced & Ready=='False'",
			args: args{
				resource: Resource{
					Manifest: DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					Event:    "Successfully selected composition",
					Children: []Resource{
						{
							Manifest: DummyManifest("XObjectStorage", "test-resource-hash", "True", "True"),
							Children: []Resource{
								{
									Manifest: DummyManifest("Bucket", "test-resource-bucket-hash", "False", "True"),
									Event:    "Synced bucket",
								},
								{
									Manifest: DummyManifest("User", "test-resource-user-hash", "True", "False"),
									Event:    "User ready",
								},
								{
									Manifest: DummyManifest("Policy", "test-resource-policy-hash", "False", "False"),
								},
							},
						},
					},
				},
			},
			want: want{
				output: Resource{
					Manifest: DummyManifest("Bucket", "test-resource-bucket-hash", "False", "True"),
					Event:    "Synced bucket",
					Children: []Resource{
						{
							Manifest: DummyManifest("User", "test-resource-user-hash", "True", "False"),
							Event:    "User ready",
						},
						{
							Manifest: DummyManifest("Policy", "test-resource-policy-hash", "False", "False"),
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Diagnose(tc.args.resource, Resource{})

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nDiagnose(): -want, +got:\n%s", tc.reason, diff)
			}
			// Check table
			if diff := cmp.Diff(tc.want.output, got); diff != "" {
				t.Errorf("%s\nDiagnose() Detect unhealthy resources: -want, +got:\n%s", tc.reason, diff)
			}
		})

	}
}

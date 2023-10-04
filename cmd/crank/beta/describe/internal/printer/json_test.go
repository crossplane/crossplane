/*
Copyright 2023 The Crossplane Authors.

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

package printer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/cmd/crank/beta/describe/internal/resource"
)

func TestJSONPrinter(t *testing.T) {
	type args struct {
		resource *resource.Resource
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
			reason: "Should print a complex Resource with children and events.",
			args: args{
				resource: &resource.Resource{
					Unstructured: DummyManifest("ObjectStorage", "test-resource", "True", "True"),
					LatestEvent: &v1.Event{
						Message: "Successfully selected composition",
					},
					Children: []*resource.Resource{
						{
							Unstructured: DummyManifest("XObjectStorage", "test-resource-hash", "True", "True"),
							Children: []*resource.Resource{
								{
									Unstructured: DummyManifest("Bucket", "test-resource-bucket-hash", "True", "True"),
									LatestEvent: &v1.Event{
										Message: "Synced bucket",
									},
								},
								{
									Unstructured: DummyManifest("User", "test-resource-user-hash", "True", "True"),
									LatestEvent: &v1.Event{
										Message: "User ready",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				// Note: Use spaces instead of tabs for intendation
				output: `
{
  "object": {
    "apiVersion": "test.cloud/v1alpha1",
    "kind": "ObjectStorage",
    "metadata": {
      "name": "test-resource",
      "namespace": "default"
    },
    "status": {
      "conditions": [
        {
          "status": "True",
          "type": "Synced"
        },
        {
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  "children": [
    {
      "object": {
        "apiVersion": "test.cloud/v1alpha1",
        "kind": "XObjectStorage",
        "metadata": {
          "name": "test-resource-hash",
          "namespace": "default"
        },
        "status": {
          "conditions": [
            {
              "status": "True",
              "type": "Synced"
            },
            {
              "status": "True",
              "type": "Ready"
            }
          ]
        }
      },
      "children": [
        {
          "object": {
            "apiVersion": "test.cloud/v1alpha1",
            "kind": "Bucket",
            "metadata": {
              "name": "test-resource-bucket-hash",
              "namespace": "default"
            },
            "status": {
              "conditions": [
                {
                  "status": "True",
                  "type": "Synced"
                },
                {
                  "status": "True",
                  "type": "Ready"
                }
              ]
            }
          },
          "latestEvent": {
            "metadata": {
              "creationTimestamp": null
            },
            "involvedObject": {},
            "message": "Synced bucket",
            "source": {},
            "firstTimestamp": null,
            "lastTimestamp": null,
            "eventTime": null,
            "reportingComponent": "",
            "reportingInstance": ""
          }
        },
        {
          "object": {
            "apiVersion": "test.cloud/v1alpha1",
            "kind": "User",
            "metadata": {
              "name": "test-resource-user-hash",
              "namespace": "default"
            },
            "status": {
              "conditions": [
                {
                  "status": "True",
                  "type": "Synced"
                },
                {
                  "status": "True",
                  "type": "Ready"
                }
              ]
            }
          },
          "latestEvent": {
            "metadata": {
              "creationTimestamp": null
            },
            "involvedObject": {},
            "message": "User ready",
            "source": {},
            "firstTimestamp": null,
            "lastTimestamp": null,
            "eventTime": null,
            "reportingComponent": "",
            "reportingInstance": ""
          }
        }
      ]
    }
  ],
  "latestEvent": {
    "metadata": {
      "creationTimestamp": null
    },
    "involvedObject": {},
    "message": "Successfully selected composition",
    "source": {},
    "firstTimestamp": null,
    "lastTimestamp": null,
    "eventTime": null,
    "reportingComponent": "",
    "reportingInstance": ""
  }
}
				`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := JSONPrinter{}
			var buf bytes.Buffer
			err := p.Print(&buf, tc.args.resource)
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

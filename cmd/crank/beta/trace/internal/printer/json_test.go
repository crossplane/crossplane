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
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
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
		"ComplexResourceWithChildren": {
			reason: "Should print a complex Resource with children.",
			args: args{
				resource: GetComplexResource(),
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
          "type": "Synced",
          "status": "True",
          "lastTransitionTime": null,
          "reason": ""
        },
        {
          "type": "Ready",
          "status": "True",
          "lastTransitionTime": null,
          "reason": ""
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
          "name": "test-resource-hash"
        },
        "status": {
          "conditions": [
            {
              "type": "Synced",
              "status": "True",
              "lastTransitionTime": null,
              "reason": ""
            },
            {
              "type": "Ready",
              "status": "True",
              "lastTransitionTime": null,
              "reason": ""
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
              "name": "test-resource-bucket-hash"
            },
            "status": {
              "conditions": [
                {
                  "type": "Synced",
                  "status": "True",
                  "lastTransitionTime": null,
                  "reason": ""
                },
                {
                  "type": "Ready",
                  "status": "True",
                  "lastTransitionTime": null,
                  "reason": ""
                }
              ]
            }
          },
          "children": [
            {
              "object": {
                "apiVersion": "test.cloud/v1alpha1",
                "kind": "User",
                "metadata": {
                  "name": "test-resource-child-1-bucket-hash"
                },
                "status": {
                  "conditions": [
                    {
                      "type": "Synced",
                      "status": "True",
                      "lastTransitionTime": null,
                      "reason": ""
                    },
                    {
                      "type": "Ready",
                      "status": "False",
                      "lastTransitionTime": null,
                      "reason": "SomethingWrongHappened",
                      "message": "Error with bucket child 1"
                    }
                  ]
                }
              }
            },
            {
              "object": {
                "apiVersion": "test.cloud/v1alpha1",
                "kind": "User",
                "metadata": {
                  "name": "test-resource-child-mid-bucket-hash"
                },
                "status": {
                  "conditions": [
                    {
                      "type": "Synced",
                      "status": "False",
                      "lastTransitionTime": null,
                      "reason": "CantSync",
                      "message": "Sync error with bucket child mid"
                    },
                    {
                      "type": "Ready",
                      "status": "True",
                      "lastTransitionTime": null,
                      "reason": "AllGood"
                    }
                  ]
                }
              }
            },
            {
              "object": {
                "apiVersion": "test.cloud/v1alpha1",
                "kind": "User",
                "metadata": {
                  "name": "test-resource-child-2-bucket-hash"
                },
                "status": {
                  "conditions": [
                    {
                      "type": "Synced",
                      "status": "True",
                      "lastTransitionTime": null,
                      "reason": ""
                    },
                    {
                      "type": "Ready",
                      "status": "False",
                      "lastTransitionTime": null,
                      "reason": "SomethingWrongHappened",
                      "message": "Error with bucket child 2"
                    }
                  ]
                }
              },
              "children": [
                {
                  "object": {
                    "apiVersion": "test.cloud/v1alpha1",
                    "kind": "User",
                    "metadata": {
                      "name": "test-resource-child-2-1-bucket-hash"
                    },
                    "status": {
                      "conditions": [
                        {
                          "type": "Synced",
                          "status": "True",
                          "lastTransitionTime": null,
                          "reason": ""
                        }
                      ]
                    }
                  }
                }
              ]
            }
          ]
        },
        {
          "object": {
            "apiVersion": "test.cloud/v1alpha1",
            "kind": "User",
            "metadata": {
              "name": "test-resource-user-hash"
            },
            "status": {
              "conditions": [
                {
                  "type": "Ready",
                  "status": "True",
                  "lastTransitionTime": null,
                  "reason": ""
                },
                {
                  "type": "Synced",
                  "status": "Unknown",
                  "lastTransitionTime": null,
                  "reason": ""
                }
              ]
            }
          }
        }
      ]
    }
  ]
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
			gotJSON := buf.String()

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nCliTableAddResource(): -want, +got:\n%s", tc.reason, diff)
			}
			// Unmarshal expected and actual output to compare them as maps
			// instead of strings, to avoid order dependent failures
			var output, got map[string]any
			if err := json.Unmarshal([]byte(tc.want.output), &output); err != nil {
				t.Errorf("JSONPrinter.Print() error unmarshalling expected output: %s", err)
			}
			if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
				t.Errorf("JSONPrinter.Print() error unmarshalling actual output: %s", err)
			}
			// Check table
			if diff := cmp.Diff(output, got); diff != "" {
				t.Errorf("%s\nCliTableAddResource(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

}

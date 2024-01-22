// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package printer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

func TestDefaultPrinter(t *testing.T) {
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
			reason: "Should print a complex Resource with children.",
			args: args{
				resource: GetComplexResource(),
			},
			want: want{
				// Note: Use spaces instead of tabs for intendation
				output: `
NAME                                                   SYNCED    READY   STATUS                                              
ObjectStorage/test-resource (default)                  True      True                                                        
└─ XObjectStorage/test-resource-hash                   True      True                                                        
   ├─ Bucket/test-resource-bucket-hash                 True      True                                                        
   │  ├─ User/test-resource-child-1-bucket-hash        True      False   SomethingWrongHappened: Error with bucket child 1   
   │  ├─ User/test-resource-child-mid-bucket-hash      False     True    CantSync: Sync error with bucket child mid          
   │  └─ User/test-resource-child-2-bucket-hash        True      False   SomethingWrongHappened: Error with bucket child 2   
   │     └─ User/test-resource-child-2-1-bucket-hash   True      -                                                           
   └─ User/test-resource-user-hash                     Unknown   True                                                        
`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := DefaultPrinter{}
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

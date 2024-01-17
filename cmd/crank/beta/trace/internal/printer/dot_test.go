// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package printer

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
)

// Define a test for PrintDotGraph
func TestPrintDotGraph(t *testing.T) {
	type args struct {
		resource *resource.Resource
	}

	type want struct {
		dotString string
		err       error
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
				dotString: `graph  {
	
	n1[label="Name: ObjectStorage/test-resource\nApiVersion: test.cloud/v1alpha1\nNamespace: default\nReady: True\nSynced: True\n",penwidth="2"];
	n2[label="Name: XObjectStorage/test-resource-hash\nApiVersion: test.cloud/v1alpha1\nReady: True\nSynced: True\n",penwidth="2"];
	n3[label="Name: Bucket/test-resource-bucket-hash\nApiVersion: test.cloud/v1alpha1\nReady: True\nSynced: True\n",penwidth="2"];
	n4[label="Name: User/test-resource-user-hash\nApiVersion: test.cloud/v1alpha1\nReady: True\nSynced: Unknown\n",penwidth="2"];
	n5[label="Name: User/test-resource-child-1-bucket-hash\nApiVersion: test.cloud/v1alpha1\nReady: False\nSynced: True\n",penwidth="2"];
	n6[label="Name: User/test-resource-child-mid-bucket-hash\nApiVersion: test.cloud/v1alpha1\nReady: True\nSynced: False\n",penwidth="2"];
	n7[label="Name: User/test-resource-child-2-bucket-hash\nApiVersion: test.cloud/v1alpha1\nReady: False\nSynced: True\n",penwidth="2"];
	n8[label="Name: User/test-resource-child-2-1-bucket-hash\nApiVersion: test.cloud/v1alpha1\nReady: \nSynced: True\n",penwidth="2"];
	n1--n2;
	n2--n3;
	n2--n4;
	n3--n5;
	n3--n6;
	n3--n7;
	n7--n8;
	
}
`,
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a GraphPrinter
			p := &DotPrinter{}
			var buf bytes.Buffer
			err := p.Print(&buf, tc.args.resource)
			got := buf.String()

			// Check error
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\ndotPrinter.Print(): -want, +got:\n%s", tc.reason, diff)
			}

			// Check if dotString is correct
			if diff := cmp.Diff(tc.want.dotString, got); diff != "" {
				t.Errorf("%s\nDotPrinter.Print(): -want, +got:\n%s", tc.reason, diff)
			}

		})

	}
}

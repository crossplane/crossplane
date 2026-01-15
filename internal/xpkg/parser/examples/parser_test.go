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

package examples

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

// mockAnnotatedReadCloser implements parser.AnnotatedReadCloser for testing.
type mockAnnotatedReadCloser struct {
	io.ReadCloser
	annotation any
}

func (m *mockAnnotatedReadCloser) Annotate() any {
	return m.annotation
}

func TestParse(t *testing.T) {
	invalidYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  invalid: [broken`
	validYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

	type annotation struct {
		path     string
		position int
	}

	type args struct {
		reader     io.ReadCloser
		annotation any
	}

	type want struct {
		objCount       int
		err            error
		errContains    string
		expectAnyError bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilReader": {
			reason: "Should return empty examples when reader is nil.",
			args: args{
				reader: nil,
			},
			want: want{
				objCount: 0,
				err:      nil,
			},
		},
		"ValidYAML": {
			reason: "Should successfully parse valid YAML.",
			args: args{
				reader: io.NopCloser(strings.NewReader(validYAML)),
			},
			want: want{
				objCount: 1,
				err:      nil,
			},
		},
		"InvalidYAMLNoAnnotation": {
			reason: "Should return error without annotation when reader is not AnnotatedReadCloser.",
			args: args{
				reader: io.NopCloser(strings.NewReader(invalidYAML)),
			},
			want: want{
				objCount:       0,
				expectAnyError: true,
			},
		},
		"InvalidYAMLWithAnnotation": {
			reason: "Should include annotation in error message when reader is AnnotatedReadCloser.",
			args: args{
				reader:     io.NopCloser(strings.NewReader(invalidYAML)),
				annotation: annotation{path: "/examples/test.yaml", position: 42},
			},
			want: want{
				objCount:       0,
				expectAnyError: true,
				errContains:    "/examples/test.yaml",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			reader := tc.args.reader
			if tc.args.annotation != nil {
				reader = &mockAnnotatedReadCloser{
					ReadCloser: tc.args.reader,
					annotation: tc.args.annotation,
				}
			}

			ex, err := p.Parse(context.Background(), reader)

			if tc.want.expectAnyError {
				if err == nil {
					t.Errorf("\n%s\nParse(...): expected error, got nil", tc.reason)
					return
				}
				if tc.want.errContains != "" && !strings.Contains(err.Error(), tc.want.errContains) {
					t.Errorf("\n%s\nParse(...): expected error to contain %q, got %q", tc.reason, tc.want.errContains, err.Error())
				}
			}

			if !tc.want.expectAnyError {
				if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nParse(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}

			if diff := cmp.Diff(tc.want.objCount, len(ex.objects)); diff != "" {
				t.Errorf("\n%s\nParse(...): -want objCount, +got objCount:\n%s", tc.reason, diff)
			}
		})
	}
}

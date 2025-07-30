/*
Copyright 2025 The Crossplane Authors.

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

package xerrors

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestComposedResourceErrorError(t *testing.T) {
	errBoom := errors.New("boom")

	testComposed := &composed.Unstructured{
		Unstructured: unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "example.org/v1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"name": "test-resource",
				},
			},
		},
	}

	type args struct {
		message  string
		composed *composed.Unstructured
		err      error
	}
	type want struct {
		result string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MessageOnly": {
			reason: "Should return message when no wrapped error",
			args: args{
				message:  "failed to compose resource",
				composed: testComposed,
				err:      nil,
			},
			want: want{
				result: "failed to compose resource",
			},
		},
		"MessageWithWrappedError": {
			reason: "Should return message with wrapped error when both present",
			args: args{
				message:  "failed to compose resource",
				composed: testComposed,
				err:      errBoom,
			},
			want: want{
				result: "failed to compose resource: boom",
			},
		},
		"EmptyMessage": {
			reason: "Should handle empty message",
			args: args{
				message:  "",
				composed: testComposed,
				err:      nil,
			},
			want: want{
				result: "",
			},
		},
		"EmptyMessageWithError": {
			reason: "Should handle empty message with wrapped error",
			args: args{
				message:  "",
				composed: testComposed,
				err:      errBoom,
			},
			want: want{
				result: ": boom",
			},
		},
		"NilComposed": {
			reason: "Should handle nil composed resource",
			args: args{
				message:  "failed to compose resource",
				composed: nil,
				err:      errBoom,
			},
			want: want{
				result: "failed to compose resource: boom",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := ComposedResourceError{
				Message:  tc.args.message,
				Composed: tc.args.composed,
				Err:      tc.args.err,
			}

			got := e.Error()

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nError(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestComposedResourceError_Unwrap(t *testing.T) {
	errBoom := errors.New("boom")

	testComposed := &composed.Unstructured{
		Unstructured: unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "example.org/v1",
				"kind":       "TestResource",
			},
		},
	}

	type args struct {
		message  string
		composed *composed.Unstructured
		err      error
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"WithWrappedError": {
			reason: "Should return wrapped error when present",
			args: args{
				message:  "failed to compose resource",
				composed: testComposed,
				err:      errBoom,
			},
			want: want{
				err: errBoom,
			},
		},
		"WithoutWrappedError": {
			reason: "Should return nil when no wrapped error",
			args: args{
				message:  "failed to compose resource",
				composed: testComposed,
				err:      nil,
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := ComposedResourceError{
				Message:  tc.args.message,
				Composed: tc.args.composed,
				Err:      tc.args.err,
			}

			got := e.Unwrap()

			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUnwrap(): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

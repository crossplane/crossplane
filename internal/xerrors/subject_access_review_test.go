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
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestSubjectAccessReviewErrorError(t *testing.T) {
	errBoom := errors.New("boom")

	testResource := schema.GroupVersionResource{
		Group:    "example.org",
		Version:  "v1",
		Resource: "bars",
	}

	type args struct {
		user        string
		resource    schema.GroupVersionResource
		namespace   string
		deniedVerbs []string
		err         error
	}
	type want struct {
		result string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ClusterScopedResource": {
			reason: "Should format error for cluster-scoped resource",
			args: args{
				user:        "system:serviceaccount:crossplane-system:crossplane",
				resource:    testResource,
				namespace:   "",
				deniedVerbs: []string{"get", "list", "watch"},
				err:         nil,
			},
			want: want{
				result: "system:serviceaccount:crossplane-system:crossplane is not allowed to [get, list, watch] resource bars.example.org/v1",
			},
		},
		"NamespacedResource": {
			reason: "Should format error for namespaced resource",
			args: args{
				user:        "system:serviceaccount:crossplane-system:crossplane",
				resource:    testResource,
				namespace:   "test-namespace",
				deniedVerbs: []string{"create", "update", "delete"},
				err:         nil,
			},
			want: want{
				result: "system:serviceaccount:crossplane-system:crossplane is not allowed to [create, update, delete] resource bars.example.org/v1 in namespace test-namespace",
			},
		},
		"SingleVerb": {
			reason: "Should format error for single denied verb",
			args: args{
				user:        "system:serviceaccount:crossplane-system:crossplane",
				resource:    testResource,
				namespace:   "",
				deniedVerbs: []string{"delete"},
				err:         nil,
			},
			want: want{
				result: "system:serviceaccount:crossplane-system:crossplane is not allowed to [delete] resource bars.example.org/v1",
			},
		},
		"WithWrappedError": {
			reason: "Should include wrapped error in message",
			args: args{
				user:        "system:serviceaccount:crossplane-system:crossplane",
				resource:    testResource,
				namespace:   "",
				deniedVerbs: []string{"get"},
				err:         errBoom,
			},
			want: want{
				result: "system:serviceaccount:crossplane-system:crossplane is not allowed to [get] resource bars.example.org/v1: boom",
			},
		},
		"EmptyDeniedVerbs": {
			reason: "Should handle empty denied verbs list",
			args: args{
				user:        "system:serviceaccount:crossplane-system:crossplane",
				resource:    testResource,
				namespace:   "",
				deniedVerbs: []string{},
				err:         nil,
			},
			want: want{
				result: "system:serviceaccount:crossplane-system:crossplane is not allowed to [] resource bars.example.org/v1",
			},
		},
		"CoreResource": {
			reason: "Should handle core Kubernetes resources",
			args: args{
				user: "system:serviceaccount:crossplane-system:crossplane",
				resource: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "secrets",
				},
				namespace:   "kube-system",
				deniedVerbs: []string{"get", "list"},
				err:         nil,
			},
			want: want{
				result: "system:serviceaccount:crossplane-system:crossplane is not allowed to [get, list] resource secrets/v1 in namespace kube-system",
			},
		},
		"EmptyUser": {
			reason: "Should handle empty user",
			args: args{
				user:        "",
				resource:    testResource,
				namespace:   "test-namespace",
				deniedVerbs: []string{"patch"},
				err:         nil,
			},
			want: want{
				result: " is not allowed to [patch] resource bars.example.org/v1 in namespace test-namespace",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := SubjectAccessReviewError{
				User:        tc.args.user,
				Resource:    tc.args.resource,
				Namespace:   tc.args.namespace,
				DeniedVerbs: tc.args.deniedVerbs,
				Err:         tc.args.err,
			}

			got := e.Error()

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nError(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSubjectAccessReviewError_Unwrap(t *testing.T) {
	errBoom := errors.New("boom")

	testResource := schema.GroupVersionResource{
		Group:    "example.org",
		Version:  "v1",
		Resource: "bars",
	}

	type args struct {
		user        string
		resource    schema.GroupVersionResource
		namespace   string
		deniedVerbs []string
		err         error
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
				user:        "system:serviceaccount:crossplane-system:crossplane",
				resource:    testResource,
				namespace:   "",
				deniedVerbs: []string{"get"},
				err:         errBoom,
			},
			want: want{
				err: errBoom,
			},
		},
		"WithoutWrappedError": {
			reason: "Should return nil when no wrapped error",
			args: args{
				user:        "system:serviceaccount:crossplane-system:crossplane",
				resource:    testResource,
				namespace:   "",
				deniedVerbs: []string{"get"},
				err:         nil,
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := SubjectAccessReviewError{
				User:        tc.args.user,
				Resource:    tc.args.resource,
				Namespace:   tc.args.namespace,
				DeniedVerbs: tc.args.deniedVerbs,
				Err:         tc.args.err,
			}

			got := e.Unwrap()

			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUnwrap(): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

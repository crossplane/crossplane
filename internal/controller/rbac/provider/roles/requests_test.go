/*
Copyright 2020 The Crossplane Authors.

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

package roles

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestAllowed(t *testing.T) {
	cases := map[string]struct {
		reason string
		allow  []Rule
		check  Rule
		want   bool
	}{
		"SimpleURL": {
			reason: "Looking up a simple non-resource URL should succeed",
			allow:  []Rule{{NonResourceURL: "/apis", Verb: "get"}},
			check:  Rule{NonResourceURL: "/apis", Verb: "get"},
			want:   true,
		},
		"MissingVerb": {
			reason: "Looking up a verb that isn't allowed should fail",
			allow: []Rule{
				{NonResourceURL: "/apis", Verb: "delete"},
				{NonResourceURL: "*", Verb: "get"},
			},
			check: Rule{NonResourceURL: "/other", Verb: "delete"},
			want:  false,
		},
		"SimpleResource": {
			reason: "Looking up a simple resource should succeed",
			allow:  []Rule{{APIGroup: "", Resource: "examples", ResourceName: "*", Verb: "get"}},
			check:  Rule{APIGroup: "", Resource: "examples", ResourceName: "*", Verb: "get"},
			want:   true,
		},
		"WildcardResource": {
			reason: "Looking up a simple resource against a wildcard should succeed",
			allow: []Rule{
				{APIGroup: "", Resource: "*", ResourceName: "*", Verb: "get"},
				{APIGroup: "", Resource: "other", ResourceName: "*", Verb: "list"},
			},
			check: Rule{APIGroup: "", Resource: "examples", ResourceName: "*", Verb: "get"},
			want:  true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			n := newNode()
			for _, a := range tc.allow {
				n.Allow(a.path())
			}
			got := n.Allowed(tc.check.path())

			if got != tc.want {
				t.Errorf("\n%s\nn.Allowed(...): got %t, want %t", tc.reason, got, tc.want)
			}
		})
	}
}

func TestExpand(t *testing.T) {
	type args struct {
		rs  []rbacv1.PolicyRule
		ctx context.Context
	}
	type want struct {
		err   error
		rules []Rule
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"SimpleURL": {
			reason: "It should be possible to expand a simple, granular non-resource RBAC rule.",
			args: args{
				rs: []rbacv1.PolicyRule{{
					NonResourceURLs: []string{"/api"},
					Verbs:           []string{"get"},
				}},
			},
			want: want{
				rules: []Rule{{
					NonResourceURL: "/api",
					Verb:           "get",
				}},
			},
		},
		"SimpleResource": {
			reason: "It should be possible to expand a simple, granular resource RBAC rule.",
			args: args{
				rs: []rbacv1.PolicyRule{{
					APIGroups: []string{""},
					Resources: []string{"*"},
					Verbs:     []string{"get"},
				}},
			},
			want: want{
				rules: []Rule{{
					APIGroup:     "",
					Resource:     "*",
					ResourceName: "*",
					Verb:         "get",
				}},
			},
		},
		"ComplexResource": {
			reason: "It should be possible to expand a more complex resource RBAC rule.",
			args: args{
				rs: []rbacv1.PolicyRule{
					{APIGroups: []string{""}, Resources: []string{"*"}, Verbs: []string{"get", "list", "watch"}},
					{APIGroups: []string{"example"}, Resources: []string{"examples", "others"}, ResourceNames: []string{"barry", "hank"}, Verbs: []string{"get"}},
				},
			},
			want: want{
				rules: []Rule{
					{APIGroup: "", Resource: "*", ResourceName: "*", Verb: "get"},
					{APIGroup: "", Resource: "*", ResourceName: "*", Verb: "list"},
					{APIGroup: "", Resource: "*", ResourceName: "*", Verb: "watch"},
					{APIGroup: "example", Resource: "examples", ResourceName: "barry", Verb: "get"},
					{APIGroup: "example", Resource: "examples", ResourceName: "hank", Verb: "get"},
					{APIGroup: "example", Resource: "others", ResourceName: "barry", Verb: "get"},
					{APIGroup: "example", Resource: "others", ResourceName: "hank", Verb: "get"},
				},
			},
		},
		"Combo": {
			reason: "We should faithfully expand a rule with both URLs and resources. This is invalid, but we let Kubernetes police that.",
			args: args{
				rs: []rbacv1.PolicyRule{{
					APIGroups:       []string{""},
					Resources:       []string{"*"},
					NonResourceURLs: []string{"/api"},
					Verbs:           []string{"get"},
				}},
			},
			want: want{
				rules: []Rule{
					{
						NonResourceURL: "/api",
						Verb:           "get",
					},
					{
						APIGroup:     "",
						Resource:     "*",
						ResourceName: "*",
						Verb:         "get",
					},
				},
			},
		},
		"ComboCtxCancelled": {
			reason: "We should return an error if the context is cancelled.",
			args: args{
				rs: []rbacv1.PolicyRule{{
					APIGroups:       []string{""},
					Resources:       []string{"*"},
					NonResourceURLs: []string{"/api"},
					Verbs:           []string{"get"},
				}},
				ctx: func() context.Context {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					return ctx
				}(),
			},
			want: want{
				err: context.Canceled,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := tc.args.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			got, err := Expand(ctx, tc.rs...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nExpand(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rules, got); diff != "" {
				t.Errorf("\n%s\nExpand(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestValidatePermissionRequests(t *testing.T) {
	errBoom := errors.New("boom")

	type fields struct {
		c        client.Client
		roleName string
	}

	type args struct {
		ctx      context.Context
		requests []rbacv1.PolicyRule
	}

	type want struct {
		rs  []Rule
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"GetClusterRoleError": {
			fields: fields{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetClusterRole),
			},
		},
		"SuccessfulReject": {
			fields: fields{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						cr := obj.(*rbacv1.ClusterRole)
						cr.Rules = []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"secrets", "configmaps", "events"},
								Verbs:     []string{"*"},
							},
							{
								APIGroups: []string{"apps", "extensions"},
								Resources: []string{"deployments"},
								Verbs:     []string{"get"},
							},
							{
								APIGroups: []string{"apps"},
								Resources: []string{"deployments"},
								Verbs:     []string{"list"},
							},
							{
								APIGroups:     []string{""},
								Resources:     []string{"pods"},
								ResourceNames: []string{"this-one-really-cool-pod"},
								Verbs:         []string{"*"},
							},
						}
						return nil
					}),
				},
			},
			args: args{
				ctx: context.Background(),
				requests: []rbacv1.PolicyRule{
					// Allowed - we allow * on secrets.
					{
						APIGroups: []string{""},
						Resources: []string{"secrets"},
						Verbs:     []string{"*"},
					},
					// Allowed - we allow * on configmaps.
					{
						APIGroups: []string{""},
						Resources: []string{"configmaps"},
						Verbs:     []string{"get", "list", "watch"},
					},
					// Rejected - we don't allow get on extensions/deployments.
					{
						APIGroups: []string{"extensions"},
						Resources: []string{"deployments"},
						Verbs:     []string{"get", "list"},
					},
					// Allowed - we allow get and list on apps/deployments.
					{
						APIGroups: []string{"apps"},
						Resources: []string{"deployments"},
						Verbs:     []string{"get", "list"},
					},
					// Rejected - we only allow access to really cool pods.
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				},
			},
			want: want{
				rs: []Rule{
					{APIGroup: "extensions", Resource: "deployments", ResourceName: "*", Verb: "list"},
					{APIGroup: "", Resource: "pods", ResourceName: "*", Verb: "get"},
					{APIGroup: "", Resource: "pods", ResourceName: "*", Verb: "list"},
				},
			},
		},
		"SuccessfulRejectEvenWithTimeout": {
			fields: fields{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						cr := obj.(*rbacv1.ClusterRole)
						cr.Rules = []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{"secrets", "configmaps", "events"},
								Verbs:     []string{"*"},
							},
							{
								APIGroups: []string{"apps", "extensions"},
								Resources: []string{"deployments"},
								Verbs:     []string{"get"},
							},
							{
								APIGroups: []string{"apps"},
								Resources: []string{"deployments"},
								Verbs:     []string{"list"},
							},
							{
								APIGroups:     []string{""},
								Resources:     []string{"pods"},
								ResourceNames: []string{"this-one-really-cool-pod"},
								Verbs:         []string{"*"},
							},
						}
						return nil
					}),
				},
			},
			args: args{
				ctx: func() context.Context {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					return ctx
				}(),
				requests: []rbacv1.PolicyRule{
					// Allowed - we allow * on secrets.
					{
						APIGroups: []string{""},
						Resources: []string{"secrets"},
						Verbs:     []string{"*"},
					},
					// Allowed - we allow * on configmaps.
					{
						APIGroups: []string{""},
						Resources: []string{"configmaps"},
						Verbs:     []string{"get", "list", "watch"},
					},
					// Rejected - we don't allow get on extensions/deployments.
					{
						APIGroups: []string{"extensions"},
						Resources: []string{"deployments"},
						Verbs:     []string{"get", "list"},
					},
					// Allowed - we allow get and list on apps/deployments.
					{
						APIGroups: []string{"apps"},
						Resources: []string{"deployments"},
						Verbs:     []string{"get", "list"},
					},
					// Rejected - we only allow access to really cool pods.
					{
						APIGroups: []string{""},
						Resources: []string{"pods"},
						Verbs:     []string{"get", "list"},
					},
				},
			},
			want: want{
				err: errors.Wrap(context.Canceled, errExpandClusterRoleRules),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := NewClusterRoleBackedValidator(tc.fields.c, tc.fields.roleName)
			got, err := v.ValidatePermissionRequests(tc.args.ctx, tc.args.requests...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nExpand(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rs, got); diff != "" {
				t.Errorf("\n%s\nExpand(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

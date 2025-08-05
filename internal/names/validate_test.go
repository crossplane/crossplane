/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package names

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestValidateNameChars(t *testing.T) {
	type args struct {
		name string
		gk   schema.GroupKind
	}
	type want struct {
		result bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidDNS1123Name": {
			reason: "Should return true for a valid DNS1123 subdomain name",
			args: args{
				name: "valid-resource-name",
				gk:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			},
			want: want{
				result: true,
			},
		},
		"ValidSingleChar": {
			reason: "Should return true for a single alphanumeric character",
			args: args{
				name: "a",
				gk:   schema.GroupKind{Group: "v1", Kind: "Pod"},
			},
			want: want{
				result: true,
			},
		},
		"ValidWithNumbers": {
			reason: "Should return true for names with numbers",
			args: args{
				name: "resource-123",
				gk:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			},
			want: want{
				result: true,
			},
		},
		"ValidRBACWithColon": {
			reason: "Should return true for RBAC resources with colons",
			args: args{
				name: "system:admin:user",
				gk:   schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "Role"},
			},
			want: want{
				result: true,
			},
		},
		"ValidClusterRoleWithColon": {
			reason: "Should return true for ClusterRole with colons",
			args: args{
				name: "system:controller:admin",
				gk:   schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
			},
			want: want{
				result: true,
			},
		},
		"ValidRoleBindingWithColon": {
			reason: "Should return true for RoleBinding with colons",
			args: args{
				name: "system:serviceaccount:kube-system:default",
				gk:   schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"},
			},
			want: want{
				result: true,
			},
		},
		"ValidClusterRoleBindingWithColon": {
			reason: "Should return true for ClusterRoleBinding with colons",
			args: args{
				name: "cluster-admin:system:masters",
				gk:   schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"},
			},
			want: want{
				result: true,
			},
		},
		"InvalidStartsWithHyphen": {
			reason: "Should return false for names starting with hyphen",
			args: args{
				name: "-invalid-name",
				gk:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			},
			want: want{
				result: false,
			},
		},
		"InvalidEndsWithHyphen": {
			reason: "Should return false for names ending with hyphen",
			args: args{
				name: "invalid-name-",
				gk:   schema.GroupKind{Group: "v1", Kind: "Pod"},
			},
			want: want{
				result: false,
			},
		},
		"InvalidUppercase": {
			reason: "Should return false for names containing uppercase letters",
			args: args{
				name: "Invalid-Name",
				gk:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			},
			want: want{
				result: false,
			},
		},
		"InvalidSpecialChars": {
			reason: "Should return false for names with invalid special characters",
			args: args{
				name: "invalid@name",
				gk:   schema.GroupKind{Group: "v1", Kind: "Service"},
			},
			want: want{
				result: false,
			},
		},
		"InvalidTooLong": {
			reason: "Should return false for names longer than 253 characters",
			args: args{
				name: "a" + strings.Repeat("b", 253), // 254 characters total
				gk:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			},
			want: want{
				result: false,
			},
		},
		"ValidExactly253Chars": {
			reason: "Should return true for names with exactly 253 characters",
			args: args{
				name: "a" + strings.Repeat("b", 251) + "c", // 253 characters total
				gk:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			},
			want: want{
				result: true,
			},
		},
		"InvalidEmpty": {
			reason: "Should return false for empty names",
			args: args{
				name: "",
				gk:   schema.GroupKind{Group: "v1", Kind: "Pod"},
			},
			want: want{
				result: false,
			},
		},
		"InvalidUnderscores": {
			reason: "Should return false for names containing underscores",
			args: args{
				name: "invalid_name",
				gk:   schema.GroupKind{Group: "v1", Kind: "Service"},
			},
			want: want{
				result: false,
			},
		},
		"InvalidNonRBACWithColon": {
			reason: "Should return false for non-RBAC resources with colons",
			args: args{
				name: "invalid:name",
				gk:   schema.GroupKind{Group: "apps", Kind: "Deployment"},
			},
			want: want{
				result: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, _ := ValidateName(tc.args.name, tc.args.gk)

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nvalidateNameChars(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

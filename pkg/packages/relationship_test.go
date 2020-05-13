/*
Copyright 2019 The Crossplane Authors.

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

package packages

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	namespace             = "cool-namespace"
	uidString             = "definitely-a-uuid"
	resourceName          = "cool-resource"
	thirtyTwoAs           = strings.Repeat("a", 32)
	sixtyOneAs            = strings.Repeat("a", 61)
	sixtyThreeAs          = strings.Repeat("a", 63)
	truncatedAsAndZ       = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-ukl4i"
	truncatedSixtyThreeAs = "aaaaaaaaaaaaaaaaaaaaaaaaaa-apyj6"
)

func resource(namespace, resourceName, uidString string) *corev1.Secret {
	uid := types.UID(uidString)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      resourceName,
			UID:       uid,
		},
	}
}
func TestParentLabels(t *testing.T) {
	tests := []struct {
		name string
		i    KindlyIdentifier
		want map[string]string
	}{
		{
			name: "Success",
			i:    resource(namespace, resourceName, uidString),
			want: map[string]string{
				LabelParentNamespace: namespace,
				LabelParentName:      resourceName,
				LabelParentGroup:     "",
				LabelParentKind:      "",
				LabelParentVersion:   "",
			},
		},
		{
			name: "Max",
			i:    resource(sixtyThreeAs, sixtyThreeAs, uidString),
			want: map[string]string{
				LabelParentNamespace: sixtyThreeAs,
				LabelParentName:      sixtyThreeAs,
				LabelParentGroup:     "",
				LabelParentKind:      "",
				LabelParentVersion:   "",
			},
		},
		{
			name: "Truncated",
			i:    resource(sixtyThreeAs+"z", sixtyThreeAs+"z", uidString),
			want: map[string]string{
				LabelParentNamespace: truncatedAsAndZ,
				LabelParentName:      truncatedAsAndZ,
				LabelParentGroup:     "",
				LabelParentKind:      "",
				LabelParentVersion:   "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParentLabels(tt.i)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ParentLabels() -want, +got:\n%v", diff)
			}
		})
	}
}

func TestMultiParentLabel(t *testing.T) {

	tests := []struct {
		name          string
		packageParent metav1.Object
		want          string
	}{
		{
			name:          "Simple",
			packageParent: resource(namespace, resourceName, uidString),
			want:          "parent.packages.crossplane.io/cool-namespace-cool-resource",
		},
		{
			name:          "NotTruncatedNS",
			packageParent: resource(thirtyTwoAs, "z", uidString),
			want:          "parent.packages.crossplane.io/" + thirtyTwoAs + "-z",
		},
		{
			name:          "TruncatedNS",
			packageParent: resource(sixtyThreeAs+"z", resourceName, uidString),
			want:          "parent.packages.crossplane.io/aaaaaaaaaaaaaaaaaaaaaaaaaa-ukl4i-" + resourceName,
		},
		{
			name:          "NotTruncatedOnNameLength",
			packageParent: resource("n", sixtyOneAs, uidString),
			want:          "parent.packages.crossplane.io/n-" + sixtyOneAs,
		},
		{
			name:          "TruncatedOnNameLength",
			packageParent: resource("n", sixtyThreeAs, uidString),
			want:          "parent.packages.crossplane.io/n-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-k5upc",
		},
		{
			name:          "TruncatedOnNameNotNS",
			packageParent: resource(thirtyTwoAs, sixtyThreeAs, uidString),
			want:          "parent.packages.crossplane.io/" + thirtyTwoAs + "-aaaaaaaaaaaaaaaaaaaaaaaa-ga5cb",
		},
		{
			name:          "DoubleTruncated",
			packageParent: resource(sixtyThreeAs, sixtyThreeAs, uidString),
			want:          "parent.packages.crossplane.io/" + truncatedSixtyThreeAs + "-aaaaaaaaaaaaaaaaaaaaaaaa-4qqns",
		},
		{
			name:          "DoubleTruncatedDifferentNameSameNSPrefix",
			packageParent: resource(sixtyThreeAs, sixtyThreeAs+"z", uidString),
			want:          "parent.packages.crossplane.io/" + truncatedSixtyThreeAs + "-aaaaaaaaaaaaaaaaaaaaaaaa-7fywo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MultiParentLabel(tt.packageParent)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("MultiParentLabel() -want, +got:\n%v", diff)
			}
		})
	}
}

func TestHasPrefixedLabel(t *testing.T) {
	type args struct {
		obj      metav1.Object
		prefixes []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "NotFound",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{"not/close": "foo"})
					return r
				}(),
				prefixes: []string{"test/prefix"},
			},
			want: false,
		},

		{
			name: "WholePrefix",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{"test/prefix": "foo"})
					return r
				}(),
				prefixes: []string{"test/prefix"},
			},
			want: true,
		},
		{
			name: "PartialPrefixIgnored",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{"test/pre": "foo"})
					return r
				}(),
				prefixes: []string{"test/prefix"},
			},
			want: false,
		},
		{
			name: "PrefixFoundWithExtra",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{"test/prefix-extra": "foo"})
					return r
				}(),
				prefixes: []string{"test/prefix"},
			},
			want: true,
		},
		{
			name: "FoundWithOtherLabels",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{
						"test/prefix": "foo",
						"test/other":  "bar",
					})
					return r
				}(),
				prefixes: []string{"test/prefix"},
			},
			want: true,
		},
		{
			name: "FoundAllPrefixes",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{
						"test/prefix": "foo",
						"test/other":  "bar",
					})
					return r
				}(),
				prefixes: []string{"test/prefix", "test/other"},
			},
			want: true,
		},
		{
			name: "FoundSomePrefixes",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{"test/prefix": "foo"})
					return r
				}(),
				prefixes: []string{"test/prefix", "test/other"},
			},
			want: true,
		},
		{
			name: "NotAnyPrefixesFound",
			args: args{
				obj: func() metav1.Object {
					r := resource("a", "b", "c")
					r.SetLabels(map[string]string{
						"test/prefix": "foo",
						"test/other":  "bar",
					})
					return r
				}(),
				prefixes: []string{"test/foo", "test/bar"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasPrefixedLabel(tt.args.obj, tt.args.prefixes...)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("HasPrefixedLabel() -want, +got:\n%v", diff)
			}
		})
	}
}

func TestMultiParentLabelPrefix(t *testing.T) {
	type args struct {
		packageParent metav1.Object
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MultiParentLabelPrefix(tt.args.packageParent); got != tt.want {
				t.Errorf("MultiParentLabelPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

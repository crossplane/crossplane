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

package names

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestChildName(t *testing.T) {
	type args struct {
		parentName string
		parentUID  string
		childName  string
	}

	cases := map[string]struct {
		reason string
		args
		want string
	}{
		"ShortName": {
			reason: "Should concatenate parent and hash when the result fits within 63 characters",
			args: args{
				parentName: "cool-resource-",
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "database",
			},
			want: "cool-resource-38ce86992a27",
		},
		"LongNameRequiresTruncation": {
			reason: "Should truncate parent name when the full name would exceed 63 characters",
			args: args{
				parentName: strings.Repeat("very-long-parent-name-", 3), // 66 characters
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "database",
			},
			want: "very-long-parent-name-very-long-parent-name-very-l-38ce86992a27",
		},
		"ParentWithoutHyphen": {
			reason: "Should add hyphen when parent name doesn't end with one",
			args: args{
				parentName: "resource",
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "database",
			},
			want: "resource-38ce86992a27",
		},
		"ParentWithHyphen": {
			reason: "Should not add extra hyphen when parent already ends with one",
			args: args{
				parentName: "resource-",
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "database",
			},
			want: "resource-38ce86992a27",
		},
		"TruncateAndAddHyphen": {
			reason: "Should truncate parent name and add hyphen when parent doesn't end with hyphen",
			args: args{
				parentName: strings.Repeat("very-long-parent-name-without-ending-hyphen", 2), // 86 characters, no trailing hyphen
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "database",
			},
			want: "very-long-parent-name-without-ending-hyphenvery-lo-38ce86992a27",
		},
		"EmptyParentName": {
			reason: "Should handle empty parent name",
			args: args{
				parentName: "",
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "database",
			},
			want: "38ce86992a27",
		},
		"DifferentUIDSameNameA": {
			reason: "First UID with database child name",
			args: args{
				parentName: "parent-",
				parentUID:  "uid-123",
				childName:  "database",
			},
			want: "parent-5f968d238736",
		},
		"DifferentUIDSameNameB": {
			reason: "Different UID with same child name should produce different hash",
			args: args{
				parentName: "parent-",
				parentUID:  "uid-456",
				childName:  "database",
			},
			want: "parent-bd4f105d0576",
		},
		"SameUIDDifferentChildA": {
			reason: "First child name with same UID",
			args: args{
				parentName: "parent-",
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "database",
			},
			want: "parent-38ce86992a27",
		},
		"SameUIDDifferentChildB": {
			reason: "Different child name with same UID should produce different hash",
			args: args{
				parentName: "parent-",
				parentUID:  "75e4a668-035f-4ce8-8c45-f4d3ac850155",
				childName:  "storage",
			},
			want: "parent-abcd22a110fe",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ChildName(tc.parentName, tc.parentUID, tc.childName)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nChildName(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

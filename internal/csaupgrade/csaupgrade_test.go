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

package csaupgrade

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMaybeFixFieldOwnership(t *testing.T) {
	type args struct {
		managedFields  []v1.ManagedFieldsEntry
		ssaManagerName string
		filter         filterFn
	}
	type want struct {
		migrated      bool
		managedFields []v1.ManagedFieldsEntry
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MigrateMainResource": {
			reason: "Migrate field manager on main resource only",
			args: args{
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   csaManagerNames[0],
						Operation: v1.ManagedFieldsOperationUpdate,
					},
					{
						Manager:     csaManagerNames[0],
						Operation:   v1.ManagedFieldsOperationUpdate,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
				ssaManagerName: "ssa",
				filter:         SkipSubresources,
			},
			want: want{
				migrated: true,
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   "ssa",
						Operation: v1.ManagedFieldsOperationApply,
					},
					{
						Manager:     csaManagerNames[0],
						Operation:   v1.ManagedFieldsOperationUpdate,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
			},
		},
		"MigrateSubresource": {
			reason: "Migrate field manager on subresource only",
			args: args{
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   csaManagerNames[0],
						Operation: v1.ManagedFieldsOperationUpdate,
					},
					{
						Manager:     csaManagerNames[0],
						Operation:   v1.ManagedFieldsOperationUpdate,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
				ssaManagerName: "ssa",
				filter:         OnlySubresources,
			},
			want: want{
				migrated: true,
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   csaManagerNames[0],
						Operation: v1.ManagedFieldsOperationUpdate,
					},
					{
						Manager:     "ssa",
						Operation:   v1.ManagedFieldsOperationApply,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
			},
		},
		"MigrateAllResource": {
			reason: "Migrate field manager both on main and sub resource",
			args: args{
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   csaManagerNames[0],
						Operation: v1.ManagedFieldsOperationUpdate,
					},
					{
						Manager:     csaManagerNames[0],
						Operation:   v1.ManagedFieldsOperationUpdate,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
				ssaManagerName: "ssa",
				filter:         All,
			},
			want: want{
				migrated: true,
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   "ssa",
						Operation: v1.ManagedFieldsOperationApply,
					},
					{
						Manager:     "ssa",
						Operation:   v1.ManagedFieldsOperationApply,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
			},
		},
		"MigrateNoResource": {
			reason: "Do not migrate if client-side managers are not found",
			args: args{
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   "bar",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
					{
						Manager:     "bar",
						Operation:   v1.ManagedFieldsOperationUpdate,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
				ssaManagerName: "ssa",
				filter:         All,
			},
			want: want{
				migrated: false,
				managedFields: []v1.ManagedFieldsEntry{
					{
						Manager:   "bar",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
					{
						Manager:     "bar",
						Operation:   v1.ManagedFieldsOperationUpdate,
						Subresource: "status",
					},
					{
						Manager:   "foo",
						Operation: v1.ManagedFieldsOperationUpdate,
					},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			u := &unstructured.Unstructured{}
			u.SetManagedFields(tc.args.managedFields)
			if got := MaybeFixFieldOwnership(u, tc.args.ssaManagerName, tc.args.filter); got != tc.want.migrated {
				t.Errorf("MaybeFixFieldOwnership() = %v, want %v\n%s\n", got, tc.want.migrated, tc.reason)
			}
			if diff := cmp.Diff(tc.want.managedFields, u.GetManagedFields()); diff != "" {
				t.Errorf("\nMaybeFixFieldOwnership(...) unexpected managed fields: %s: -want, +got:%s", tc.reason, diff)
			}
		})
	}
}

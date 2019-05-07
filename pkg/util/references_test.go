/*
Copyright 2018 The Crossplane Authors.

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

package util

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddOwnerReference(t *testing.T) {
	type args struct {
		om *metav1.ObjectMeta
		or metav1.OwnerReference
	}
	tests := []struct {
		name string
		args args
		want *metav1.ObjectMeta
	}{
		{
			name: "MetaIsNil",
			args: args{om: nil, or: metav1.OwnerReference{Name: "foo"}},
			want: nil,
		},
		{
			name: "MetaOrIsNil",
			args: args{om: &metav1.ObjectMeta{}, or: metav1.OwnerReference{Name: "foo"}},
			want: &metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{Name: "foo"}}},
		},
		{
			name: "NoDupes",
			args: args{
				om: &metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{Name: "bar"}}},
				or: metav1.OwnerReference{Name: "foo"},
			},
			want: &metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{
				{Name: "bar"},
				{Name: "foo"},
			}},
		},
		{
			name: "Dupes",
			args: args{
				om: &metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{
					{Name: "foo"},
					{Name: "bar"},
				}},
				or: metav1.OwnerReference{Name: "foo"},
			},
			want: &metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{
				{Name: "foo"},
				{Name: "bar"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddOwnerReference(tt.args.om, tt.args.or)
			if diff := cmp.Diff(tt.args.om, tt.want); diff != "" {
				t.Errorf("AddOwnerReferenece() %s", diff)
			}
		})
	}
}

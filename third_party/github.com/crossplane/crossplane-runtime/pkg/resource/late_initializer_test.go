/*
Copyright 2021 The Crossplane Authors.

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

package resource

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLateInitializeStringPtr(t *testing.T) {
	s1 := "desired"
	s2 := "observed"
	type args struct {
		org  *string
		from *string
	}
	type want struct {
		result  *string
		changed bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"Original": {
			args: args{
				org:  &s1,
				from: &s2,
			},
			want: want{
				result:  &s1,
				changed: false,
			},
		},
		"LateInitialized": {
			args: args{
				org:  nil,
				from: &s2,
			},
			want: want{
				result:  &s2,
				changed: true,
			},
		},
		"Neither": {
			args: args{
				org:  nil,
				from: nil,
			},
			want: want{
				result:  nil,
				changed: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			li := NewLateInitializer()
			got := li.LateInitializeStringPtr(tc.org, tc.from)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("LateInitializeStringPtr(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.changed, li.IsChanged()); diff != "" {
				t.Errorf("IsChanged(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLateInitializeInt64Ptr(t *testing.T) {
	i1 := int64(10)
	i2 := int64(20)
	type args struct {
		org  *int64
		from *int64
	}
	type want struct {
		result  *int64
		changed bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"Original": {
			args: args{
				org:  &i1,
				from: &i2,
			},
			want: want{
				result:  &i1,
				changed: false,
			},
		},
		"LateInitialized": {
			args: args{
				org:  nil,
				from: &i2,
			},
			want: want{
				result:  &i2,
				changed: true,
			},
		},
		"Neither": {
			args: args{
				org:  nil,
				from: nil,
			},
			want: want{
				result:  nil,
				changed: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			li := NewLateInitializer()
			got := li.LateInitializeInt64Ptr(tc.org, tc.from)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("LateInitializeBoolPtr(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.changed, li.IsChanged()); diff != "" {
				t.Errorf("IsChanged(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLateInitializeBoolPtr(t *testing.T) {
	trueVal := true
	falseVal := false
	type args struct {
		org  *bool
		from *bool
	}
	type want struct {
		result  *bool
		changed bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"Original": {
			args: args{
				org:  &trueVal,
				from: &falseVal,
			},
			want: want{
				result:  &trueVal,
				changed: false,
			},
		},
		"LateInitialized": {
			args: args{
				org:  nil,
				from: &trueVal,
			},
			want: want{
				result:  &trueVal,
				changed: true,
			},
		},
		"Neither": {
			args: args{
				org:  nil,
				from: nil,
			},
			want: want{
				result:  nil,
				changed: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			li := NewLateInitializer()
			got := li.LateInitializeBoolPtr(tc.org, tc.from)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("LateInitializeBoolPtr(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.changed, li.IsChanged()); diff != "" {
				t.Errorf("IsChanged(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestLateInitializeTimePtr(t *testing.T) {
	t1 := metav1.Now()
	t2 := time.Now().Add(time.Minute)
	t2m := metav1.NewTime(t2)
	type args struct {
		org  *metav1.Time
		from *time.Time
	}
	type want struct {
		result  *metav1.Time
		changed bool
	}
	cases := map[string]struct {
		args
		want
	}{
		"Original": {
			args: args{
				org:  &t1,
				from: &t2,
			},
			want: want{
				result:  &t1,
				changed: false,
			},
		},
		"LateInitialized": {
			args: args{
				org:  nil,
				from: &t2,
			},
			want: want{
				result:  &t2m,
				changed: true,
			},
		},
		"Neither": {
			args: args{
				org:  nil,
				from: nil,
			},
			want: want{
				result:  nil,
				changed: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			li := NewLateInitializer()
			got := li.LateInitializeTimePtr(tc.org, tc.from)
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("LateInitializeTimePtr(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.changed, li.IsChanged()); diff != "" {
				t.Errorf("IsChanged(...): -want, +got:\n%s", diff)
			}
		})
	}
}

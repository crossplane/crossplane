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

package claim

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestK8sEntries(t *testing.T) {
	type args struct {
		entries map[string]string
	}
	type want struct {
		r map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"KeepAppKubernetesIoInstanceLabel": {
			reason: "app.kubernetes.io/instance label should be kept",
			args: args{
				entries: map[string]string{
					"app.kubernetes.io/instance": "test",
				},
			},
			want: want{
				r: map[string]string{
					"app.kubernetes.io/instance": "test",
				},
			},
		},
		"KeepAppKubernetesIoNameLabel": {
			reason: "app.kubernetes.io/name label should be kept",
			args: args{
				entries: map[string]string{
					"app.kubernetes.io/name": "test",
				},
			},
			want: want{
				r: map[string]string{
					"app.kubernetes.io/name": "test",
				},
			},
		},
		"KeepNonk8sLabel": {
			reason: "non-k8s labels should be kept",
			args: args{
				entries: map[string]string{
					"test-label": "test",
				},
			},
			want: want{
				r: map[string]string{
					"test-label": "test",
				},
			},
		},
		"StripLastApplied": {
			reason: "kubectl.kubernetes.io/last-applied-configuration should be stripped",
			args: args{
				entries: map[string]string{
					"kubectl.kubernetes.io/last-applied-configuration": "test",
				},
			},
			want: want{
				r: map[string]string{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := withoutReservedK8sEntries(tc.args.entries)
			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nwithoutReservedK8sEntries(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

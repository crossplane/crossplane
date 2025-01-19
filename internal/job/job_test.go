/*
Copyright 2024 The Crossplane Authors.

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

package job

import (
	"testing"
	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestJobInitializer(t *testing.T) {
	type args struct {
		log              logging.Logger
		k8sClient        kubernetes.Interface
		crossplaneClient client.Client
	}
	type want struct {
		len int
	}
	cases := map[string]struct {
		args
		want
	}{
		"Success": {
			args: args{
				log:              logging.NewNopLogger(),
				k8sClient:        kubernetes.New(&fake.RESTClient{}),
				crossplaneClient: test.NewMockClient(),
			},
			want: want{
				len: 1,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			jobs := NewJobs(tc.args.log, tc.args.k8sClient, tc.args.crossplaneClient)
			if diff := cmp.Diff(tc.want.len, len(jobs), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}

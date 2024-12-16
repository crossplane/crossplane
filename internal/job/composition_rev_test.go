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

package job

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest/fake"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestCompositionRevisionCleanupJob(t *testing.T) {

	type args struct {
		log              logging.Logger
		k8sClient        *kubernetes.Clientset
		crossplaneClient client.Client
		Ctx              context.Context
		ItemsToKeep      map[string]struct{}
		KeepTopNItems    int
	}
	type want struct {
		processedCount int
		err            error
	}
	cases := map[string]struct {
		args
		want
	}{
		"SuccessWithNoKeepingItemsAndKeepOneRevision": {
			args: args{
				log:              logging.NewNopLogger(),
				k8sClient:        kubernetes.New(&fake.RESTClient{}),
				crossplaneClient: test.NewMockClient(),

				Ctx:           context.Background(),
				ItemsToKeep:   map[string]struct{}{},
				KeepTopNItems: 1,
			},
			want: want{
				processedCount: 1,
				err:            nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			job := NewCompositionRevisionCleanupJob(tc.args.log, tc.args.k8sClient, tc.args.crossplaneClient)
			processedCount, err := job.Run(tc.args.Ctx, tc.args.ItemsToKeep, tc.args.KeepTopNItems)
			if diff := cmp.Diff(tc.want.processedCount, processedCount, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}

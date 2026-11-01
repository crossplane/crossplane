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

package cronoperation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

func TestCronOperationPredicate(t *testing.T) {
	co := func(generation int64, paused bool) *v1alpha1.CronOperation {
		annotations := map[string]string{}
		if paused {
			annotations[meta.AnnotationKeyReconciliationPaused] = "true"
		}
		return &v1alpha1.CronOperation{
			ObjectMeta: metav1.ObjectMeta{
				Generation:  generation,
				Annotations: annotations,
			},
		}
	}

	cases := map[string]struct {
		reason string
		old    *v1alpha1.CronOperation
		new    *v1alpha1.CronOperation
		want   bool
	}{
		"GenerationChanged": {
			reason: "We should reconcile when the spec changes.",
			old:    co(1, false),
			new:    co(2, false),
			want:   true,
		},
		"PauseRemoved": {
			reason: "We should reconcile when the pause annotation is removed, even though generation is unchanged.",
			old:    co(1, true),
			new:    co(1, false),
			want:   true,
		},
		"PauseAdded": {
			reason: "We should reconcile when the pause annotation is added, even though generation is unchanged.",
			old:    co(1, false),
			new:    co(1, true),
			want:   true,
		},
		"PauseUnchanged": {
			reason: "We should not reconcile when neither generation nor pause state changes.",
			old:    co(1, true),
			new:    co(1, true),
			want:   false,
		},
		"UnpausedUnchanged": {
			reason: "We should not reconcile when neither generation nor pause state changes.",
			old:    co(1, false),
			new:    co(1, false),
			want:   false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := cronOperationPredicate().Update(event.UpdateEvent{
				ObjectOld: tc.old,
				ObjectNew: tc.new,
			})
			if got != tc.want {
				t.Errorf("%s: got %v, want %v", tc.reason, got, tc.want)
			}
		})
	}
}

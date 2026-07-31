/*
Copyright 2026 The Crossplane Authors.

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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

func TestEnqueueCompositionRevisionsForFunctionRevision(t *testing.T) {
	const digest = "sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	revs := []v1.CompositionRevision{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "by-name"},
			Spec: v1.CompositionRevisionSpec{Pipeline: []v1.PipelineStep{
				{Step: "a", FunctionRef: &v1.FunctionReference{Name: "crossplane-contrib-function-cool"}},
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "by-package"},
			Spec: v1.CompositionRevisionSpec{Pipeline: []v1.PipelineStep{
				{Step: "a", Function: "xpkg.crossplane.io/crossplane-contrib/function-cool@" + digest},
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "by-tagged-package"},
			Spec: v1.CompositionRevisionSpec{Pipeline: []v1.PipelineStep{
				{Step: "a", Function: "xpkg.crossplane.io/crossplane-contrib/function-cool:v1.0.0@" + digest},
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "other-registry"},
			Spec: v1.CompositionRevisionSpec{Pipeline: []v1.PipelineStep{
				{Step: "a", Function: "registry.example.org/crossplane-contrib/function-cool@" + digest},
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "other-function"},
			Spec: v1.CompositionRevisionSpec{Pipeline: []v1.PipelineStep{
				{Step: "a", FunctionRef: &v1.FunctionReference{Name: "function-other"}},
			}},
		},
	}

	kube := &test.MockClient{
		MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
			obj.(*v1.CompositionRevisionList).Items = revs
			return nil
		}),
	}

	fr := &pkgv1.FunctionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "crossplane-contrib-function-cool-abc123",
			Labels: map[string]string{pkgv1.LabelParentPackage: "crossplane-contrib-function-cool"},
		},
		Spec: pkgv1.FunctionRevisionSpec{
			PackageRevisionSpec: pkgv1.PackageRevisionSpec{
				Package: "xpkg.crossplane.io/crossplane-contrib/function-cool@" + digest,
			},
		},
	}

	q := &recordingQueue{}
	h := EnqueueCompositionRevisionsForFunctionRevision(kube, logging.NewNopLogger())
	h.Create(context.Background(), event.CreateEvent{Object: fr}, q)

	// Steps that reference the package with or without a tag should match, but
	// the same digest from another registry should not.
	want := []string{"by-name", "by-package", "by-tagged-package"}
	if diff := cmp.Diff(want, q.names, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("enqueued: -want, +got:\n%s", diff)
	}
}

type recordingQueue struct {
	workqueue.TypedRateLimitingInterface[reconcile.Request]

	names []string
}

func (q *recordingQueue) Add(r reconcile.Request) {
	q.names = append(q.names, r.Name)
}

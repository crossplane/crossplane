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

package applicationconfiguration

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestApplyWorkloads(t *testing.T) {
	errBoom := errors.New("boom")

	namespace := "ns"

	workload := &unstructured.Unstructured{}
	workload.SetAPIVersion("v")
	workload.SetKind("workload")
	workload.SetNamespace(namespace)
	workload.SetName("workload")
	workload.SetUID(types.UID("workload-uid"))

	trait := &unstructured.Unstructured{}
	trait.SetAPIVersion("v")
	trait.SetKind("trait")
	trait.SetNamespace(namespace)
	trait.SetName("trait")
	trait.SetUID(types.UID("trait-uid"))

	type args struct {
		ctx context.Context
		w   []Workload
	}

	cases := map[string]struct {
		reason string
		client resource.Applicator
		args   args
		want   error
	}{
		"ApplyWorkloadError": {
			reason: "Errors applying a workload should be reflected as a status condition",
			client: resource.ApplyFn(func(_ context.Context, o runtime.Object, _ ...resource.ApplyOption) error {
				if w, ok := o.(*unstructured.Unstructured); ok && w.GetUID() == workload.GetUID() {
					return errBoom
				}
				return nil
			}),
			args: args{w: []Workload{{Workload: workload}}},
			want: errors.Wrapf(errBoom, errFmtApplyWorkload, workload.GetName()),
		},
		"ApplyTraitError": {
			reason: "Errors applying a trait should be reflected as a status condition",
			client: resource.ApplyFn(func(_ context.Context, o runtime.Object, _ ...resource.ApplyOption) error {
				if t, ok := o.(*unstructured.Unstructured); ok && t.GetUID() == trait.GetUID() {
					return errBoom
				}
				return nil
			}),
			args: args{w: []Workload{{Workload: workload, Traits: []unstructured.Unstructured{*trait}}}},
			want: errors.Wrapf(errBoom, errFmtApplyTrait, trait.GetName()),
		},
		"Success": {
			reason: "Applied workloads and traits should be returned as a set of UIDs",
			client: resource.ApplyFn(func(_ context.Context, o runtime.Object, _ ...resource.ApplyOption) error { return nil }),
			args:   args{w: []Workload{{Workload: workload, Traits: []unstructured.Unstructured{*trait}}}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := workloads{client: tc.client}
			err := a.Apply(tc.args.ctx, tc.args.w)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\na.Apply(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

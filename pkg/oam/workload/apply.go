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

package workload

import (
	"context"
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/resource"

	workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

var (
	errNotKubeApp            = "object is not a KubernetesApplication"
	errMergeKubeAppTemplates = "cannot merge KubernetesApplicationResourceTemplates"
)

type template struct {
	gvk  schema.GroupVersionKind
	name string
}

// KubeAppApplyOption ensures resource templates are merged instead of replaced
// before patch if they have the same name and GroupVersionKind. We must merge
// the current and desired templates prior to submitting a Patch to the API
// server because KubernetesApplicationResourceTemplates are stored as an array
// in the KubernetesApplication. This means that entire templates will be
// replaced when a single field is different, per
// https://tools.ietf.org/html/rfc7386. We instead patch each of the resource
// templates individually before passing along the entire KubernetesApplication
// to resource.Apply.
func KubeAppApplyOption() resource.ApplyOption {
	return func(_ context.Context, current, desired runtime.Object) error {
		c, ok := current.(*workloadv1alpha1.KubernetesApplication)
		if !ok {
			return errors.New(errNotKubeApp)
		}
		d, ok := desired.(*workloadv1alpha1.KubernetesApplication)
		if !ok {
			return errors.New(errNotKubeApp)
		}

		index := make(map[template]int)
		for i, t := range d.Spec.ResourceTemplates {
			temp := &unstructured.Unstructured{}
			if err := json.Unmarshal(t.Spec.Template.Raw, temp); err != nil {
				return errors.Wrap(err, errMergeKubeAppTemplates)
			}
			index[template{gvk: temp.GroupVersionKind(), name: t.GetName()}] = i
		}

		for _, t := range c.Spec.ResourceTemplates {
			temp := &unstructured.Unstructured{}
			if err := json.Unmarshal(t.Spec.Template.Raw, temp); err != nil {
				return errors.Wrap(err, errMergeKubeAppTemplates)
			}
			i, ok := index[template{gvk: temp.GroupVersionKind(), name: t.GetName()}]
			if !ok {
				continue
			}

			merged, err := jsonpatch.MergePatch(t.Spec.Template.Raw, d.Spec.ResourceTemplates[i].Spec.Template.Raw)
			if err != nil {
				return errors.Wrap(err, errMergeKubeAppTemplates)
			}
			d.Spec.ResourceTemplates[i].Spec.Template = runtime.RawExtension{Raw: merged}
		}

		return nil
	}
}

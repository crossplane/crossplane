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

package op

import (
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

// LoadOperation loads an Operation from a YAML file. If the file contains a
// CronOperation or WatchOperation, the Operation template is extracted.
func LoadOperation(fs afero.Fs, path string) (*opsv1alpha1.Operation, error) {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read operation file %q", path)
	}

	// Peek at the GVK to determine which type to unmarshal into.
	var meta metav1.TypeMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal type metadata from %q", path)
	}

	switch gvk := meta.GroupVersionKind(); gvk {
	case opsv1alpha1.OperationGroupVersionKind:
		op := &opsv1alpha1.Operation{}
		if err := yaml.Unmarshal(data, op); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal Operation from %q", path)
		}
		return op, nil

	case opsv1alpha1.CronOperationGroupVersionKind:
		cop := &opsv1alpha1.CronOperation{}
		if err := yaml.Unmarshal(data, cop); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal CronOperation from %q", path)
		}
		// Use the template's metadata (labels, annotations) like the real
		// controller does. The real controller always overwrites the name with
		// a generated one; we use the parent's name for simplicity.
		op := &opsv1alpha1.Operation{
			TypeMeta:   metav1.TypeMeta{APIVersion: meta.APIVersion, Kind: opsv1alpha1.OperationKind},
			ObjectMeta: cop.Spec.OperationTemplate.ObjectMeta,
			Spec:       cop.Spec.OperationTemplate.Spec,
		}
		op.SetName(cop.GetName())
		return op, nil

	case opsv1alpha1.WatchOperationGroupVersionKind:
		wop := &opsv1alpha1.WatchOperation{}
		if err := yaml.Unmarshal(data, wop); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal WatchOperation from %q", path)
		}
		// Use the template's metadata (labels, annotations) like the real
		// controller does. The real controller always overwrites the name with
		// a generated one; we use the parent's name for simplicity.
		op := &opsv1alpha1.Operation{
			TypeMeta:   metav1.TypeMeta{APIVersion: meta.APIVersion, Kind: opsv1alpha1.OperationKind},
			ObjectMeta: wop.Spec.OperationTemplate.ObjectMeta,
			Spec:       *wop.Spec.OperationTemplate.Spec.DeepCopy(),
		}
		op.SetName(wop.GetName())
		return op, nil

	default:
		return nil, errors.Errorf("not an operation type: %s/%s", gvk.Kind, meta.GetObjectKind().GroupVersionKind().GroupVersion())
	}
}

// InjectWatchedResource adds a RequiredResourceSelector for the watched resource
// to every pipeline step. This replicates what the WatchOperation controller does
// when creating an Operation from a WatchOperation.
func InjectWatchedResource(op *opsv1alpha1.Operation, watched *unstructured.Unstructured) {
	sel := opsv1alpha1.RequiredResourceSelector{
		RequirementName: opsv1alpha1.RequirementNameWatchedResource,
		APIVersion:      watched.GetAPIVersion(),
		Kind:            watched.GetKind(),
		Name:            ptr.To(watched.GetName()),
	}
	if watched.GetNamespace() != "" {
		sel.Namespace = ptr.To(watched.GetNamespace())
	}

	for i := range op.Spec.Pipeline {
		step := &op.Spec.Pipeline[i]
		if step.Requirements == nil {
			step.Requirements = &opsv1alpha1.FunctionRequirements{}
		}
		step.Requirements.RequiredResources = append(step.Requirements.RequiredResources, sel)
	}
}

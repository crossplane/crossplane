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

	opsv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
)

// LoadOperation loads an Operation from a YAML file.
func LoadOperation(fs afero.Fs, path string, rrs []unstructured.Unstructured) (*opsv1alpha1.Operation, error) {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read operation file %q", path)
	}

	op := &opsv1alpha1.Operation{}
	if err := yaml.Unmarshal(data, op); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal operation from %q", path)
	}

	// Validate that it's an Operation by checking the GVK
	switch gvk := op.GroupVersionKind(); gvk {
	case opsv1alpha1.OperationGroupVersionKind:
		return op, nil
	case opsv1alpha1.CronOperationGroupVersionKind:
		cop := opsv1alpha1.CronOperation{}
		if err := yaml.Unmarshal(data, &cop); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal operation from %q", path)
		}
		op.Kind = opsv1alpha1.OperationKind
		op.Spec = cop.Spec.OperationTemplate.Spec
		return op, nil
	case opsv1alpha1.WatchOperationGroupVersionKind:
		wop := opsv1alpha1.WatchOperation{}
		if err := yaml.Unmarshal(data, &wop); err != nil {
			return nil, errors.Wrapf(err, "cannot unmarshal operation from %q", path)
		}
		return newOperationFromWatchOperation(&wop, rrs)

	default:
		return nil, errors.Errorf("not an operation type: %s/%s", gvk.Kind, op.GetName())
	}
}

// newOperationFromWatchOperation creates a new Operation from a WatchOperation's template,
// injecting the watched resource selector into all pipeline steps.
func newOperationFromWatchOperation(wo *opsv1alpha1.WatchOperation, requiredResources []unstructured.Unstructured) (*opsv1alpha1.Operation, error) {
	// Find the watched resource (marked with annotation ops.crossplane.io/watched-resource: "True")
	var watched *unstructured.Unstructured
	for i := range requiredResources {
		res := &requiredResources[i]
		annotations := res.GetAnnotations()
		if annotations != nil && annotations[opsv1alpha1.RequirementNameWatchedResource] == "True" {
			watched = res
			break
		}
	}

	// If no watched resource found, return error
	if watched == nil {
		return nil, errors.New("no watched resource found in required resources - expected resource with annotation ops.crossplane.io/watched-resource: \"True\"")
	}

	// Build the selector for the watched resource
	sel := opsv1alpha1.RequiredResourceSelector{
		RequirementName: opsv1alpha1.RequirementNameWatchedResource,
		APIVersion:      watched.GetAPIVersion(),
		Kind:            watched.GetKind(),
		Name:            ptr.To(watched.GetName()),
	}
	if watched.GetNamespace() != "" {
		sel.Namespace = ptr.To(watched.GetNamespace())
	}

	// Create operation from template and inject selector into each pipeline step
	op := &opsv1alpha1.Operation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: opsv1alpha1.SchemeGroupVersion.String(),
			Kind:       opsv1alpha1.OperationKind,
		},
		ObjectMeta: wo.ObjectMeta,
		Spec:       *wo.Spec.OperationTemplate.Spec.DeepCopy(),
	}

	for i := range op.Spec.Pipeline {
		step := &op.Spec.Pipeline[i]
		if step.Requirements == nil {
			step.Requirements = &opsv1alpha1.FunctionRequirements{}
		}
		step.Requirements.RequiredResources = append(step.Requirements.RequiredResources, sel)
	}

	return op, nil
}

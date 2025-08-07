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
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	opsv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
)

// LoadOperation loads an Operation from a YAML file.
func LoadOperation(fs afero.Fs, path string) (*opsv1alpha1.Operation, error) {
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
	default:
		return nil, errors.Errorf("not an operation: %s/%s", gvk.Kind, op.GetName())
	}
}

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

package composed

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// todo: temporary. copied from templating-controller

// GetCondition returns the condition for the given ConditionType if exists,
// otherwise returns nil
func GetCondition(cr interface{ UnstructuredContent() map[string]interface{} }, ct v1alpha1.ConditionType) (v1alpha1.Condition, error) {
	fetchedConditions, exists, err := unstructured.NestedFieldCopy(cr.UnstructuredContent(), "status")
	if err != nil {
		return v1alpha1.Condition{}, err
	}
	if !exists {
		return v1alpha1.Condition{Type: ct, Status: v1.ConditionUnknown}, nil
	}
	conditionsJSON, err := json.Marshal(fetchedConditions)
	if err != nil {
		return v1alpha1.Condition{}, err
	}
	conditioned := v1alpha1.ConditionedStatus{}
	if err := json.Unmarshal(conditionsJSON, &conditioned); err != nil {
		return v1alpha1.Condition{}, err
	}
	return conditioned.GetCondition(ct), nil
}

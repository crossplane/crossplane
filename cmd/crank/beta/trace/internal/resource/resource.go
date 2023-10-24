/*
Copyright 2023 The Crossplane Authors.

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

// Package resource contains the definition of the Resource used by all trace
// printers, and the client used to get a Resource and its children.
package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// Resource struct represents a kubernetes resource.
type Resource struct {
	Unstructured unstructured.Unstructured `json:"object"`
	Error        error                     `json:"error,omitempty"`
	Children     []*Resource               `json:"children,omitempty"`
}

// GetCondition of this resource.
func (r *Resource) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(r.Unstructured.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	// We didn't use xpv1.CondidionedStatus.GetCondition because that's defaulting the
	// status to unknown if the condition is not found at all.
	for _, c := range conditioned.Conditions {
		if c.Type == ct {
			return c
		}
	}
	return xpv1.Condition{}
}

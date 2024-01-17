// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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

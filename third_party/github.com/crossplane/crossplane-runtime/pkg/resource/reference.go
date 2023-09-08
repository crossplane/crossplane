/*
Copyright 2019 The Crossplane Authors.

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

package resource

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReferenceStatusType is an enum type for the possible values for a Reference Status
type ReferenceStatusType int

// Reference statuses.
const (
	ReferenceStatusUnknown ReferenceStatusType = iota
	ReferenceNotFound
	ReferenceNotReady
	ReferenceReady
)

func (t ReferenceStatusType) String() string {
	return []string{"Unknown", "NotFound", "NotReady", "Ready"}[t]
}

// ReferenceStatus has the name and status of a reference
type ReferenceStatus struct {
	Name   string
	Status ReferenceStatusType
}

func (r ReferenceStatus) String() string {
	return fmt.Sprintf("{reference:%s status:%s}", r.Name, r.Status)
}

// A CanReference is a resource that can reference another resource in its
// spec in order to automatically resolve corresponding spec field values
// by inspecting the referenced resource.
type CanReference runtime.Object

// An AttributeReferencer resolves cross-resource attribute references. See
// https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
// for more information
type AttributeReferencer interface {
	// GetStatus retries the referenced resource, as well as other non-managed
	// resources (like a `Provider`) and reports their readiness for use as a
	// referenced resource.
	GetStatus(ctx context.Context, res CanReference, r client.Reader) ([]ReferenceStatus, error)

	// Build retrieves the referenced resource, as well as other non-managed
	// resources (like a `Provider`), and builds the referenced attribute,
	// returning it as a string value.
	Build(ctx context.Context, res CanReference, r client.Reader) (value string, err error)

	// Assign accepts a managed resource object, and assigns the given value to
	// its corresponding property.
	Assign(res CanReference, value string) error
}

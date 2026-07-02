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

package composite

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/dependency"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// ReferenceFromSelector converts a composition function's required-resource
// selector into a dependency reference.
func ReferenceFromSelector(s *fnv1.ResourceSelector) dependency.Reference {
	r := dependency.Reference{
		GVK:       schema.FromAPIVersionAndKind(s.GetApiVersion(), s.GetKind()),
		Namespace: s.GetNamespace(),
	}

	switch m := s.GetMatch().(type) {
	case *fnv1.ResourceSelector_MatchName:
		r.Name = m.MatchName
	case *fnv1.ResourceSelector_MatchLabels:
		r.Labels = m.MatchLabels.GetLabels()
	}

	return r
}

// SelectorFromReference converts a dependency reference back into a
// required-resource selector, so its resources can be fetched.
func SelectorFromReference(r dependency.Reference) *fnv1.ResourceSelector {
	s := &fnv1.ResourceSelector{
		ApiVersion: r.GVK.GroupVersion().String(),
		Kind:       r.GVK.Kind,
	}

	if r.Namespace != "" {
		ns := r.Namespace
		s.Namespace = &ns
	}

	if r.Name != "" {
		s.Match = &fnv1.ResourceSelector_MatchName{MatchName: r.Name}
		return s
	}

	s.Match = &fnv1.ResourceSelector_MatchLabels{MatchLabels: &fnv1.MatchLabels{Labels: r.Labels}}

	return s
}

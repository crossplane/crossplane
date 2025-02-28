/*
Copyright 2022 The Crossplane Authors.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LatestRevision returns the latest revision of the supplied composition.
// We use a hash of the labels, the annotations, and the spec to decide to create a new revision.
// If we revert back to an older state, we increase the existing revision's revision number.
func LatestRevision(c *Composition, revs []CompositionRevision) *CompositionRevision {
	// to make sure that we always return a revision controlled by the composition
	latest := CompositionRevision{}

	for i := range revs {
		if !metav1.IsControlledBy(&revs[i], c) {
			continue
		}
		if latest.Spec.Revision < revs[i].Spec.Revision {
			latest = revs[i]
		}
	}

	// revision numbers start from 1, this means that we have no revision in the list
	// controlled by the composition
	if latest.Spec.Revision == 0 {
		return nil
	}

	return &latest
}

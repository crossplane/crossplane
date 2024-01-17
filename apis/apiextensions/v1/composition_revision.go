// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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

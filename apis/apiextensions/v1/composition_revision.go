package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// LatestRevision returns the latest revision of the supplied composition.
// We use a hash of the labels, the annotations, and the spec to decide to create a new revision.
// If we revert back to an older state, we increase the existing revision's revision number.
func LatestRevision(c *Composition, revs []v1alpha1.CompositionRevision) *v1alpha1.CompositionRevision {
	if len(revs) == 0 {
		return nil
	}

	latest := revs[0]
	for i := range revs {
		if !metav1.IsControlledBy(&revs[i], c) {
			continue
		}
		if latest.Spec.Revision < revs[i].Spec.Revision {
			latest = revs[i]
		}
	}

	return &latest
}

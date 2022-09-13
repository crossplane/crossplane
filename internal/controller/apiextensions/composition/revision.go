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

package composition

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// NewCompositionRevision creates a new revision of the supplied Composition.
func NewCompositionRevision(c *v1.Composition, revision int64, compSpecHash string) *v1alpha1.CompositionRevision {
	cr := &v1alpha1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: c.GetName() + "-",
			Labels: map[string]string{
				v1alpha1.LabelCompositionName:     c.GetName(),
				v1alpha1.LabelCompositionSpecHash: compSpecHash,
			},
		},
		Spec: v1alpha1.CompositionRevisionSpec{
			CompositionSpec: *c.Spec.DeepCopy(),
			Revision:        revision,
		},
	}

	ref := meta.TypedReferenceTo(c, v1.CompositionGroupVersionKind)
	meta.AddOwnerReference(cr, meta.AsController(ref))

	cr.Status.SetConditions(v1alpha1.CompositionSpecMatches())

	return cr
}

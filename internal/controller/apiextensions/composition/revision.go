// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package composition

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// NewCompositionRevision creates a new revision of the supplied Composition.
func NewCompositionRevision(c *v1.Composition, revision int64) *v1.CompositionRevision {
	hash := c.Hash()
	if len(hash) >= 63 {
		hash = hash[0:63]
	}

	nameSuffix := hash
	if len(nameSuffix) >= 7 {
		nameSuffix = nameSuffix[0:7]
	}

	cr := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", c.GetName(), nameSuffix),
			Labels: map[string]string{
				v1.LabelCompositionName: c.GetName(),
				// We cannot have a label value longer than 63 chars
				// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
				v1.LabelCompositionHash: hash,
			},
		},
		Spec: NewCompositionRevisionSpec(c.Spec, revision),
	}

	ref := meta.TypedReferenceTo(c, v1.CompositionGroupVersionKind)
	meta.AddOwnerReference(cr, meta.AsController(ref))

	for k, v := range c.GetLabels() {
		cr.ObjectMeta.Labels[k] = v
	}

	return cr
}

// NewCompositionRevisionSpec translates a composition's spec to a composition
// revision spec.
func NewCompositionRevisionSpec(cs v1.CompositionSpec, revision int64) v1.CompositionRevisionSpec {
	conv := v1.GeneratedRevisionSpecConverter{}
	rs := conv.ToRevisionSpec(cs)
	rs.Revision = revision
	return rs
}

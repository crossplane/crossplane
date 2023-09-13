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

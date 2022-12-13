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
	"encoding/json"
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha2"
)

const (
	errMarshalComposition = "cannot marshal composition"
)

// NewCompositionRevision creates a new revision of the supplied Composition.
func NewCompositionRevision(c *v1.Composition, revision int64, compSpecHash string) (*v1alpha2.CompositionRevision, error) {
	cr := &v1alpha2.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", c.GetName(), compSpecHash[0:7]),
			Labels: map[string]string{
				v1alpha2.LabelCompositionName: c.GetName(),
				// We cannot have a label value longer than 63 chars
				// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
				v1alpha2.LabelCompositionHash: compSpecHash[0:63],
			},
		},
	}

	comp := v1.Composition{}
	comp.SetGroupVersionKind(c.GroupVersionKind())
	comp.SetName(c.GetName())
	comp.SetLabels(c.GetLabels())
	comp.SetAnnotations(c.GetAnnotations())
	c.Spec.DeepCopyInto(&comp.Spec)

	manifest, err := json.Marshal(comp)
	if err != nil {
		return nil, errors.Wrap(err, errMarshalComposition)
	}
	cr.Spec = v1alpha2.CompositionRevisionSpec{
		Revision: revision,
		Composition: extv1.JSON{
			Raw: manifest,
		},
	}

	ref := meta.TypedReferenceTo(c, v1.CompositionGroupVersionKind)
	meta.AddOwnerReference(cr, meta.AsController(ref))

	for k, v := range c.GetLabels() {
		cr.ObjectMeta.Labels[k] = v
	}

	return cr, nil
}

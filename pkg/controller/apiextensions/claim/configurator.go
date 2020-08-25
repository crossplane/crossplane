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

package claim

import (
	"context"
	"errors"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/pkg/controller/apiextensions/composite/composed"
)

// Configure the supplied composite resource. The composite resource name is
// derived from the supplied claim, as {name}-{random-string}. The claim's
// external name annotation, if any, is propagated to the composite resource.
func Configure(_ context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	ucm, ok := cm.(*claim.Unstructured)
	if !ok {
		return nil
	}
	ucp, ok := cp.(*composite.Unstructured)
	if !ok {
		return nil
	}

	// TODO(negz): Consider nesting all user-specified spec values under a
	// predictable object like spec.parameters so we can propagate _only_ user
	// specified fields. Maintaining a set of keys to delete here seems error
	// prone. Note that deleting these keys isn't always necessary; the CRD will
	// drop them if preserveUnknownFields is false, but that is not the default
	// for pre-v1 CRDs.
	i, _ := fieldpath.Pave(ucm.Object).GetValue("spec")
	spec, ok := i.(map[string]interface{})
	if !ok {
		return errors.New("composite resource claim spec was not an object")
	}

	// TODO(negz): Make these filtered keys constants in the ccrds package?
	_ = fieldpath.Pave(ucp.Object).SetValue("spec", filter(spec, "resourceRef", "writeConnectionSecretToRef"))
	meta.AddAnnotations(ucp, ucm.GetAnnotations())
	meta.AddLabels(ucp, ucm.GetLabels())
	ucp.SetGenerateName(fmt.Sprintf("%s-", cm.GetName()))
	if meta.GetExternalName(cm) != "" {
		meta.SetExternalName(ucp, meta.GetExternalName(cm))
	}
	meta.AddLabels(ucp, map[string]string{
		composed.LabelKeyClaimName:      cm.GetName(),
		composed.LabelKeyClaimNamespace: cm.GetNamespace(),
	})
	return nil
}

func filter(in map[string]interface{}, keys ...string) map[string]interface{} {
	filter := map[string]bool{}
	for _, k := range keys {
		filter[k] = true
	}

	out := map[string]interface{}{}
	for k, v := range in {
		if filter[k] {
			continue
		}

		out[k] = v
	}
	return out
}

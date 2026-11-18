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

package grant

import (
	"context"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// IndexOwnerUID indexes CRDs by owner UID in the metadata cache.
const IndexOwnerUID = "metadata.ownerReferences.uid"

// CRDMetadata returns the metadata-only CRD type used by our cache and watches.
func CRDMetadata() *metav1.PartialObjectMetadata {
	o := &metav1.PartialObjectMetadata{}
	o.SetGroupVersionKind(extv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	return o
}

// OwnerUIDs extracts the owner UIDs used to index a CRD.
func OwnerUIDs(o client.Object) []string {
	uids := make([]string, 0, len(o.GetOwnerReferences()))
	for _, ref := range o.GetOwnerReferences() {
		if ref.UID != "" {
			uids = append(uids, string(ref.UID))
		}
	}
	return uids
}

// OwnedCRDs returns existing CRDs owned by owner, including non-controller
// ownership retained by inactive package revisions. The index avoids scanning
// or caching CRD schemas.
func OwnedCRDs(ctx context.Context, c client.Reader, owner metav1.Object, kind schema.GroupKind) ([]metav1.PartialObjectMetadata, error) {
	if owner.GetUID() == "" {
		return nil, nil
	}

	l := &metav1.PartialObjectMetadataList{}
	l.SetGroupVersionKind(extv1.SchemeGroupVersion.WithKind("CustomResourceDefinitionList"))
	if err := c.List(ctx, l, client.MatchingFields{IndexOwnerUID: string(owner.GetUID())}); err != nil {
		return nil, errors.Wrap(err, "cannot list owned CustomResourceDefinitions")
	}

	out := make([]metav1.PartialObjectMetadata, 0, len(l.Items))
	for _, crd := range l.Items {
		for _, ref := range crd.GetOwnerReferences() {
			if ref.UID == owner.GetUID() && ref.Name == owner.GetName() && schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind).GroupKind() == kind {
				out = append(out, crd)
				break
			}
		}
	}
	return out, nil
}

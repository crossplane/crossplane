package definition

import (
	"slices"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/internal/xcrd"
)

// IsCompositeResourceCRD accepts any CustomResourceDefinition that represents a
// Composite Resource.
func IsCompositeResourceCRD() resource.PredicateFn {
	return func(obj runtime.Object) bool {
		crd, ok := obj.(*extv1.CustomResourceDefinition)
		if !ok {
			return false
		}
		return slices.Contains(crd.Spec.Names.Categories, xcrd.CategoryComposite)
	}
}

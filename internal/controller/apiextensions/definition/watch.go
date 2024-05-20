package definition

import (
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
		for _, c := range crd.Spec.Names.Categories {
			if c == xcrd.CategoryComposite {
				return true
			}
		}
		return false
	}
}

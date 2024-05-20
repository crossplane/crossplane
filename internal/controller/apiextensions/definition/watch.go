package definition

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// OffersCompositeResource accepts any CompositeResourceDefinition or a
// CustomResourceDefinition that represents a composite.
func OffersCompositeResource() resource.PredicateFn {
	return func(obj runtime.Object) bool {
		if _, ok := obj.(*v1.CompositeResourceDefinition); ok {
			return true
		}

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

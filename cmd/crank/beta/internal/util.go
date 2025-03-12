package internal

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func DereferenceSlice[T any](slice []*T) []T {
	result := make([]T, len(slice))
	for i, element := range slice {
		result[i] = *element
	}
	return result
}

// ConvertToCRDs Helper function to convert XRDs/CRDs to CRDs
func ConvertToCRDs(extensions []*unstructured.Unstructured) ([]*extv1.CustomResourceDefinition, error) {
	crds := make([]*extv1.CustomResourceDefinition, 0)

	for _, e := range extensions {
		switch e.GroupVersionKind().GroupKind() {
		case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
			crd := &extv1.CustomResourceDefinition{}
			bytes, err := e.MarshalJSON()
			if err != nil {
				return nil, errors.Wrap(err, "cannot marshal CRD to JSON")
			}

			if err := yaml.Unmarshal(bytes, crd); err != nil {
				return nil, errors.Wrap(err, "cannot unmarshal CRD YAML")
			}

			crds = append(crds, crd)

		case schema.GroupKind{Group: "apiextensions.crossplane.io", Kind: "CompositeResourceDefinition"}:
			xrd := &v1.CompositeResourceDefinition{}
			bytes, err := e.MarshalJSON()
			if err != nil {
				return nil, errors.Wrap(err, "cannot marshal XRD to JSON")
			}

			if err := yaml.Unmarshal(bytes, xrd); err != nil {
				return nil, errors.Wrap(err, "cannot unmarshal XRD YAML")
			}

			crd, err := xcrd.ForCompositeResource(xrd)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot derive composite CRD from XRD %q", xrd.GetName())
			}
			crds = append(crds, crd)

			if xrd.Spec.ClaimNames != nil {
				claimCrd, err := xcrd.ForCompositeResourceClaim(xrd)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot derive claim CRD from XRD %q", xrd.GetName())
				}

				crds = append(crds, claimCrd)
			}

		default:
			// Process other package types as needed
			// This is where you would extract dependency information from providers, functions, etc.
			// For the standalone function, we're just focusing on extracting CRDs
			continue
		}
	}

	return crds, nil
}

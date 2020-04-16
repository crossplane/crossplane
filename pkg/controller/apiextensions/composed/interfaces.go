package composed

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Composite resource managed one or more Composable resources.
type Composite interface {
	v1.Object
	runtime.Unstructured

	resource.Conditioned
	resource.Bindable
	resource.ConnectionSecretWriterTo

	SetCompositionSelector(*v1.LabelSelector)
	GetCompositionSelector() *v1.LabelSelector

	SetCompositionReference(*corev1.ObjectReference)
	GetCompositionReference() *corev1.ObjectReference

	SetResourceReferences([]corev1.ObjectReference)
	GetResourceReferences() []corev1.ObjectReference
}

// Composable resources can be a resource in a composition.
type Composable interface {
	resource.Object

	resource.Conditioned
	resource.ConnectionSecretWriterTo
}

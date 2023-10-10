package main

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"

	v1beta1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1beta1"
)

// TODO(negz): We have similar functions in c/c composition_functions.go and in
// c/function-sdk-go. Perhaps everything should import from function-sdk-go?

// AsState builds state for a RunFunctionRequest from the XR and composed
// resources.
func AsState(xr resource.Composite, cds map[string]composed.Unstructured) (*v1beta1.State, error) {
	r, err := AsStruct(xr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert composite resource to google.proto.Struct")
	}

	oxr := &v1beta1.Resource{Resource: r}

	ocds := make(map[string]*v1beta1.Resource)
	for name, cd := range cds {
		cd := cd // Pin range variable so we can take its address.
		r, err := AsStruct(&cd)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot convert composed resource %q to google.proto.Struct", name)
		}

		ocds[name] = &v1beta1.Resource{Resource: r}
	}

	return &v1beta1.State{Composite: oxr, Resources: ocds}, nil
}

// AsStruct converts the supplied object to a protocol buffer Struct well-known
// type.
func AsStruct(o runtime.Object) (*structpb.Struct, error) {
	// If the supplied object is *Unstructured we don't need to round-trip.
	if u, ok := o.(*kunstructured.Unstructured); ok {
		s, err := structpb.NewStruct(u.Object)
		return s, errors.Wrapf(err, "cannot create google.proto.Struct from %T", u)
	}

	// If the supplied object wraps *Unstructured we don't need to round-trip.
	if w, ok := o.(unstructured.Wrapper); ok {
		s, err := structpb.NewStruct(w.GetUnstructured().Object)
		return s, errors.Wrapf(err, "cannot create google.proto.Struct from %T", w)
	}

	// Fall back to a JSON round-trip.
	b, err := json.Marshal(o)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal JSON")
	}

	s := &structpb.Struct{}
	return s, errors.Wrap(s.UnmarshalJSON(b), "cannot unmarshal JSON")
}

// FromStruct populates the supplied object with content loaded from the Struct.
func FromStruct(o client.Object, s *structpb.Struct) error {
	// If the supplied object is *Unstructured we don't need to round-trip.
	if u, ok := o.(*kunstructured.Unstructured); ok {
		u.Object = s.AsMap()
		return nil
	}

	// If the supplied object wraps *Unstructured we don't need to round-trip.
	if w, ok := o.(unstructured.Wrapper); ok {
		w.GetUnstructured().Object = s.AsMap()
		return nil
	}

	// Fall back to a JSON round-trip.
	b, err := protojson.Marshal(s)
	if err != nil {
		return errors.Wrap(err, "cannot marshal google.proto.Struct to JSON")
	}

	return errors.Wrap(json.Unmarshal(b, o), "cannot unmarshal JSON")
}

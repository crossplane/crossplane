/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	fnv1 "github.com/crossplane/crossplane/proto/fn/v1"
)

// Tag uniquely identifies a request. Two identical requests created by the
// same Crossplane binary will produce identical tags. Different builds of
// Crossplane may produce different tags for the same inputs. See the docs for
// the Deterministic protobuf MarshalOption for more details.
func Tag(req *fnv1.RunFunctionRequest) string {
	m := proto.MarshalOptions{Deterministic: true}

	b, err := m.Marshal(req)
	if err != nil {
		return ""
	}

	h := sha256.Sum256(b)

	return hex.EncodeToString(h[:])
}

// AsStruct converts the supplied object to a protocol buffer Struct well-known
// type.
func AsStruct(o runtime.Object) (*structpb.Struct, error) {
	// If the supplied object is *Unstructured we don't need to round-trip.
	if u, ok := o.(*kunstructured.Unstructured); ok {
		s, err := structpb.NewStruct(u.Object)
		return s, errors.Wrap(err, "cannot create protobuf Struct")
	}

	// If the supplied object wraps *Unstructured we don't need to round-trip.
	if w, ok := o.(unstructured.Wrapper); ok {
		s, err := structpb.NewStruct(w.GetUnstructured().Object)
		return s, errors.Wrap(err, "cannot create protobuf Struct")
	}

	// Fall back to a JSON round-trip.
	b, err := json.Marshal(o)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal object to JSON")
	}

	s := &structpb.Struct{}

	return s, errors.Wrap(s.UnmarshalJSON(b), "cannot unmarshal object from JSON")
}

// FromStruct populates the supplied object with content loaded from the Struct.
func FromStruct(o runtime.Object, s *structpb.Struct) error {
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
		return errors.Wrap(err, "cannot marshal protobuf Struct to JSON")
	}

	return errors.Wrap(json.Unmarshal(b, o), "cannot unmarshal JSON to object")
}

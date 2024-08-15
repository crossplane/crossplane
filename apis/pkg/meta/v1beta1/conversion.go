/*
Copyright 2023 The Crossplane Authors.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

const (
	errWrongConvertToFunction   = "must convert to *v1.Function"
	errWrongConvertFromFunction = "must convert from *v1.Function"
)

// A ToHubConverter converts v1beta1 types to the 'hub' v1 type.
//
// goverter:converter
// goverter:name GeneratedToHubConverter
// goverter:extend ConvertObjectMeta
// goverter:output:file ./zz_generated.conversion.go
// goverter:output:package github.com/crossplane/crossplane/apis/pkg/meta/v1beta1
// +k8s:deepcopy-gen=false
type ToHubConverter interface {
	Function(in *Function) *v1.Function
}

// A FromHubConverter converts v1beta1 types from the 'hub' v1 type.
//
// goverter:converter
// goverter:name GeneratedFromHubConverter
// goverter:extend ConvertObjectMeta
// goverter:output:file ./zz_generated.conversion.go
// goverter:output:package github.com/crossplane/crossplane/apis/pkg/meta/v1beta1
// +k8s:deepcopy-gen=false
type FromHubConverter interface {
	Function(in *v1.Function) *Function
}

// ConvertObjectMeta 'converts' ObjectMeta by producing a deepcopy. This
// is necessary because goverter can't convert metav1.Time. It also prevents
// goverter generating code that is functionally identical to deepcopygen's.
func ConvertObjectMeta(in metav1.ObjectMeta) metav1.ObjectMeta {
	out := in.DeepCopy()
	return *out
}

// ConvertTo converts this Function to the Hub version.
func (c *Function) ConvertTo(hub conversion.Hub) error {
	out, ok := hub.(*v1.Function)
	if !ok {
		return errors.New(errWrongConvertToFunction)
	}

	conv := &GeneratedToHubConverter{}
	*out = *conv.Function(c)

	return nil
}

// ConvertFrom converts this Function from the Hub version.
func (c *Function) ConvertFrom(hub conversion.Hub) error {
	in, ok := hub.(*v1.Function)
	if !ok {
		return errors.New(errWrongConvertFromFunction)
	}

	conv := &GeneratedFromHubConverter{}
	*c = *conv.Function(in)

	return nil
}

/*
Copyright 2022 The Crossplane Authors.

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

package v1alpha1

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

const (
	errWrongConvertToConfiguration   = "must convert to *v1.Configuration"
	errWrongConvertFromConfiguration = "must convert from *v1.Configuration"

	errWrongConvertToProvider   = "must convert to *v1.Provider"
	errWrongConvertFromProvider = "must convert from *v1.Provider"
)

// A ToHubConverter converts v1alpha1 types to the 'hub' v1 type.
//
// goverter:converter
// goverter:name GeneratedToHubConverter
// goverter:extend ConvertObjectMeta
// +k8s:deepcopy-gen=false
type ToHubConverter interface {
	Configuration(in *Configuration) *v1.Configuration
	Provider(in *Provider) *v1.Provider
}

// A FromHubConverter converts v1alpha1 types from the 'hub' v1 type.
//
// goverter:converter
// goverter:name GeneratedFromHubConverter
// goverter:extend ConvertObjectMeta
// +k8s:deepcopy-gen=false
type FromHubConverter interface {
	Configuration(in *v1.Configuration) *Configuration
	Provider(in *v1.Provider) *Provider
}

// ConvertObjectMeta 'converts' ObjectMeta by producing a deepcopy. This
// is necessary because goverter can't convert metav1.Time. It also prevents
// goverter generating code that is functionally identical to deepcopygen's.
func ConvertObjectMeta(in metav1.ObjectMeta) metav1.ObjectMeta {
	out := in.DeepCopy()
	return *out
}

// ConvertTo converts this Configuration to the Hub version.
func (c *Configuration) ConvertTo(hub conversion.Hub) error {
	out, ok := hub.(*v1.Configuration)
	if !ok {
		return errors.New(errWrongConvertToConfiguration)
	}

	conv := &GeneratedToHubConverter{}
	*out = *conv.Configuration(c)

	return nil
}

// ConvertFrom converts this Configuration from the Hub version.
func (c *Configuration) ConvertFrom(hub conversion.Hub) error {
	in, ok := hub.(*v1.Configuration)
	if !ok {
		return errors.New(errWrongConvertFromConfiguration)
	}

	conv := &GeneratedFromHubConverter{}
	*c = *conv.Configuration(in)

	return nil
}

// ConvertTo converts this Provider to the Hub version.
func (p *Provider) ConvertTo(hub conversion.Hub) error {
	out, ok := hub.(*v1.Provider)
	if !ok {
		return errors.New(errWrongConvertToProvider)
	}

	conv := &GeneratedToHubConverter{}
	*out = *conv.Provider(p)

	return nil
}

// ConvertFrom converts this Provider from the Hub version.
func (p *Provider) ConvertFrom(hub conversion.Hub) error {
	in, ok := hub.(*v1.Provider)
	if !ok {
		return errors.New(errWrongConvertFromProvider)
	}

	conv := &GeneratedFromHubConverter{}
	*p = *conv.Provider(in)

	return nil
}

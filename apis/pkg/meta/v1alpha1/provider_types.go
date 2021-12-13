/*
Copyright 2020 The Crossplane Authors.

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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

const (
	errWrongConvertToProvider   = "must convert to *v1.Provider"
	errWrongConvertFromProvider = "must convert from *v1.Provider"
)

// ProviderSpec specifies the configuration of a Provider.
type ProviderSpec struct {
	// Configuration for the packaged Provider's controller.
	Controller ControllerSpec `json:"controller"`

	MetaSpec `json:",inline"`
}

// ControllerSpec specifies the configuration for the packaged Provider
// controller.
type ControllerSpec struct {
	// Image is the packaged Provider controller image.
	Image *string `json:"image,omitempty"`

	// PermissionRequests for RBAC rules required for this provider's controller
	// to function. The RBAC manager is responsible for assessing the requested
	// permissions.
	// +optional
	PermissionRequests []rbacv1.PolicyRule `json:"permissionRequests,omitempty"`
}

// +kubebuilder:object:root=true

// A Provider is the description of a Crossplane Provider package.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec"`
}

// ConvertTo converts this Provider to the Hub version.
func (p *Provider) ConvertTo(hub conversion.Hub) error {
	out, ok := hub.(*v1.Provider)
	if !ok {
		return errors.New(errWrongConvertToProvider)
	}

	p.ObjectMeta.DeepCopyInto(&out.ObjectMeta)

	out.Spec = v1.ProviderSpec{
		Controller: v1.ControllerSpec{
			Image:              p.Spec.Controller.Image,
			PermissionRequests: p.Spec.Controller.PermissionRequests,
		},
	}

	if p.Spec.Crossplane != nil {
		out.Spec.Crossplane = &v1.CrossplaneConstraints{Version: p.Spec.Crossplane.Version}
	}

	if len(p.Spec.DependsOn) == 0 {
		return nil
	}

	out.Spec.DependsOn = make([]v1.Dependency, len(p.Spec.DependsOn))
	for i := range p.Spec.DependsOn {
		out.Spec.DependsOn[i] = v1.Dependency{
			Provider:      p.Spec.DependsOn[i].Provider,
			Configuration: p.Spec.DependsOn[i].Configuration,
			Version:       p.Spec.DependsOn[i].Version,
		}
	}

	return nil
}

// ConvertFrom converts this Provider from the Hub version.
func (p *Provider) ConvertFrom(hub conversion.Hub) error {
	in, ok := hub.(*v1.Provider)
	if !ok {
		return errors.New(errWrongConvertFromProvider)
	}

	in.ObjectMeta.DeepCopyInto(&p.ObjectMeta)

	p.Spec = ProviderSpec{
		Controller: ControllerSpec{
			Image:              in.Spec.Controller.Image,
			PermissionRequests: in.Spec.Controller.PermissionRequests,
		},
	}

	if in.Spec.Crossplane != nil {
		p.Spec.Crossplane = &CrossplaneConstraints{Version: in.Spec.Crossplane.Version}
	}

	if len(in.Spec.DependsOn) == 0 {
		return nil
	}

	p.Spec.DependsOn = make([]Dependency, len(in.Spec.DependsOn))
	for i := range in.Spec.DependsOn {
		p.Spec.DependsOn[i] = Dependency{
			Provider:      in.Spec.DependsOn[i].Provider,
			Configuration: in.Spec.DependsOn[i].Configuration,
			Version:       in.Spec.DependsOn[i].Version,
		}
	}

	return nil
}

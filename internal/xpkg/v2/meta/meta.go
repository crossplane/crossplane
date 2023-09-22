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

package meta

import (
	"encoding/json"
	"errors"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
)

const (
	errXPkgNameNotProvided  = "package name not provided"
	errCtrlImageNotProvided = "controller images not provided"
)

// NewConfigXPkg returns a slice of bytes containing a fully rendered
// Configuration template given the provided ConfigContext.
func NewConfigXPkg(c xpkg.InitContext) ([]byte, error) {
	// name is required
	if c.Name == "" {
		return nil, errors.New(errXPkgNameNotProvided)
	}

	cfg := metav1.Configuration{
		TypeMeta: v1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       metav1.ConfigurationKind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: c.Name,
		},
		Spec: metav1.ConfigurationSpec{
			MetaSpec: metav1.MetaSpec{
				DependsOn: c.DependsOn,
			},
		},
	}

	if c.XPVersion != "" {
		cfg.Spec.Crossplane = &metav1.CrossplaneConstraints{Version: c.XPVersion}
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return cleanNullTs(b)
}

// NewProviderXPkg returns a slice of bytes containing a fully rendered
// Provider template given the provided ProviderContext.
func NewProviderXPkg(c xpkg.InitContext) ([]byte, error) {
	// name is required
	if c.Name == "" {
		return nil, errors.New(errXPkgNameNotProvided)
	}

	// image is required
	if c.CtrlImage == "" {
		return nil, errors.New(errCtrlImageNotProvided)
	}

	p := metav1.Provider{
		TypeMeta: v1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       metav1.ProviderKind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: c.Name,
		},
		Spec: metav1.ProviderSpec{
			Controller: metav1.ControllerSpec{
				Image: pointer.String(c.CtrlImage),
			},
		},
	}

	if c.XPVersion != "" {
		p.Spec.Crossplane = &metav1.CrossplaneConstraints{Version: c.XPVersion}
	}

	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return cleanNullTs(b)
}

// cleanNullTs is a helper function for cleaning the erroneous
// `creationTimestamp: null` from the marshaled data that we're
// going to write to the meta file.
func cleanNullTs(b []byte) ([]byte, error) {
	var m map[string]any
	err := json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	// remove the erroneous creationTimestamp: null entry
	delete(m["metadata"].(map[string]any), "creationTimestamp")

	return yaml.Marshal(m)
}

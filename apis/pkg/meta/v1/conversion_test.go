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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/pkg/meta"
)

func TestConvertTo(t *testing.T) {
	name := "wishiwasaslicktop"
	version := "0.42.0"
	provider := "crossplane/provider-cool:v0.42.0"
	config := "crossplane/getting-started-with-being-cool:v0.42.0"
	ctrl := "crossplane/provider-cool-controller:v0.42.0"
	url := "/cool"
	verb := "activate"

	type want struct {
		hub conversion.Hub
		err error
	}
	cases := map[string]struct {
		reason string
		c      conversion.Convertible
		hub    conversion.Hub
		want   want
	}{
		"ErrConfigurationWrongHub": {
			reason: "It is only possible to convert a *v1.Configuration to a *meta.Configuration.",
			c:      &Configuration{},
			hub:    &meta.Provider{},
			want: want{
				hub: &meta.Provider{},
				err: errors.New(errWrongConvertToConfiguration),
			},
		},
		"MinimalConfigurationConversion": {
			reason: "It should be possible to convert a minimal *v1.Configuration to a *meta.Configuration.",
			c:      &Configuration{ObjectMeta: metav1.ObjectMeta{Name: name}},
			hub:    &meta.Configuration{},
			want: want{
				hub: &meta.Configuration{ObjectMeta: metav1.ObjectMeta{Name: name}},
			},
		},
		"FullConfigurationConversion": {
			reason: "It should be possible to convert a fully populated *v1.Configuration to a *meta.Configuration.",
			c: &Configuration{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: ConfigurationSpec{
					MetaSpec: MetaSpec{
						Crossplane: &CrossplaneConstraints{Version: version},
						DependsOn: []Dependency{
							{
								Provider: &provider,
								Version:  version,
							},
							{
								Configuration: &config,
								Version:       version,
							},
						},
					},
				},
			},
			hub: &meta.Configuration{},
			want: want{
				hub: &meta.Configuration{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: meta.ConfigurationSpec{
						MetaSpec: meta.MetaSpec{
							Crossplane: &meta.CrossplaneConstraints{Version: version},
							DependsOn: []meta.Dependency{
								{
									Provider: &provider,
									Version:  version,
								},
								{
									Configuration: &config,
									Version:       version,
								},
							},
						},
					},
				},
			},
		},
		"ErrProviderWrongHub": {
			reason: "It is only possible to convert a *v1.Provider to a *meta.Provider.",
			c:      &Provider{},
			hub:    &meta.Configuration{},
			want: want{
				hub: &meta.Configuration{},
				err: errors.New(errWrongConvertToProvider),
			},
		},
		"MinimalProviderConversion": {
			reason: "It should be possible to convert a minimal *v1.Provider to a *meta.Provider.",
			c: &Provider{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: ProviderSpec{
					Controller: ControllerSpec{Image: ctrl},
				},
			},
			hub: &meta.Provider{},
			want: want{
				hub: &meta.Provider{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: meta.ProviderSpec{
						Controller: meta.ControllerSpec{Image: ctrl},
					},
				},
			},
		},
		"FullProviderConversion": {
			reason: "It should be possible to convert a fully populated *v1.Provider to a *meta.Provider.",
			c: &Provider{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: ProviderSpec{
					Controller: ControllerSpec{
						Image: ctrl,
						PermissionRequests: []rbacv1.PolicyRule{
							{
								NonResourceURLs: []string{url},
								Verbs:           []string{verb},
							},
						},
					},
					MetaSpec: MetaSpec{
						Crossplane: &CrossplaneConstraints{Version: version},
						DependsOn: []Dependency{
							{
								Provider: &provider,
								Version:  version,
							},
							{
								Provider: &config,
								Version:  version,
							},
						},
					},
				},
			},
			hub: &meta.Provider{},
			want: want{
				hub: &meta.Provider{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: meta.ProviderSpec{
						Controller: meta.ControllerSpec{
							Image: ctrl,
							PermissionRequests: []rbacv1.PolicyRule{
								{
									NonResourceURLs: []string{url},
									Verbs:           []string{verb},
								},
							},
						},
						MetaSpec: meta.MetaSpec{
							Crossplane: &meta.CrossplaneConstraints{Version: version},
							DependsOn: []meta.Dependency{
								{
									Provider: &provider,
									Version:  version,
								},
								{
									Provider: &config,
									Version:  version,
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.c.ConvertTo(tc.hub)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nc.ConvertTo(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.hub, tc.hub); diff != "" {
				t.Errorf("\n%s\nc.ConvertTo(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConvertFrom(t *testing.T) {
	name := "wishiwasaslicktop"
	version := "0.42.0"
	provider := "crossplane/provider-cool:v0.42.0"
	config := "crossplane/getting-started-with-being-cool:v0.42.0"
	ctrl := "crossplane/provider-cool-controller:v0.42.0"
	url := "/cool"
	verb := "activate"

	type want struct {
		c   conversion.Convertible
		err error
	}
	cases := map[string]struct {
		reason string
		c      conversion.Convertible
		hub    conversion.Hub
		want   want
	}{
		"ErrConfigurationWrongHub": {
			reason: "It is only possible to convert a *v1.Configuration from a *meta.Configuration.",
			c:      &Configuration{},
			hub:    &meta.Provider{},
			want: want{
				c:   &Configuration{},
				err: errors.New(errWrongConvertFromConfiguration),
			},
		},
		"MinimalConfigurationConversion": {
			reason: "It should be possible to convert a minimal *v1.Configuration from a *meta.Configuration.",
			c:      &Configuration{},
			hub:    &meta.Configuration{ObjectMeta: metav1.ObjectMeta{Name: name}},
			want: want{
				c: &Configuration{ObjectMeta: metav1.ObjectMeta{Name: name}},
			},
		},
		"FullConfigurationConversion": {
			reason: "It should be possible to convert a fully populated *v1.Configuration from a *meta.Configuration.",
			c:      &Configuration{},
			hub: &meta.Configuration{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: meta.ConfigurationSpec{
					MetaSpec: meta.MetaSpec{
						Crossplane: &meta.CrossplaneConstraints{Version: version},
						DependsOn: []meta.Dependency{
							{
								Provider: &provider,
								Version:  version,
							},
							{
								Configuration: &config,
								Version:       version,
							},
						},
					},
				},
			},
			want: want{
				c: &Configuration{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: ConfigurationSpec{
						MetaSpec: MetaSpec{
							Crossplane: &CrossplaneConstraints{Version: version},
							DependsOn: []Dependency{
								{
									Provider: &provider,
									Version:  version,
								},
								{
									Configuration: &config,
									Version:       version,
								},
							},
						},
					},
				},
			},
		},
		"ErrProviderWrongHub": {
			reason: "It is only possible to convert a *v1.Provider from a *meta.Provider.",
			c:      &Provider{},
			hub:    &meta.Configuration{},
			want: want{
				c:   &Provider{},
				err: errors.New(errWrongConvertFromProvider),
			},
		},
		"MinimalProviderConversion": {
			reason: "It should be possible to convert a minimal *v1.Provider from a *meta.Provider.",
			c:      &Provider{},
			hub: &meta.Provider{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: meta.ProviderSpec{
					Controller: meta.ControllerSpec{Image: ctrl},
				},
			},
			want: want{
				c: &Provider{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: ProviderSpec{
						Controller: ControllerSpec{Image: ctrl},
					},
				},
			},
		},
		"FullProviderConversion": {
			reason: "It should be possible to convert a fully populated *v1.Provider from a *meta.Provider.",
			c:      &Provider{},
			hub: &meta.Provider{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: meta.ProviderSpec{
					Controller: meta.ControllerSpec{
						Image: ctrl,
						PermissionRequests: []rbacv1.PolicyRule{
							{
								NonResourceURLs: []string{url},
								Verbs:           []string{verb},
							},
						},
					},
					MetaSpec: meta.MetaSpec{
						Crossplane: &meta.CrossplaneConstraints{Version: version},
						DependsOn: []meta.Dependency{
							{
								Provider: &provider,
								Version:  version,
							},
							{
								Provider: &config,
								Version:  version,
							},
						},
					},
				},
			},
			want: want{
				c: &Provider{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Spec: ProviderSpec{
						Controller: ControllerSpec{
							Image: ctrl,
							PermissionRequests: []rbacv1.PolicyRule{
								{
									NonResourceURLs: []string{url},
									Verbs:           []string{verb},
								},
							},
						},
						MetaSpec: MetaSpec{
							Crossplane: &CrossplaneConstraints{Version: version},
							DependsOn: []Dependency{
								{
									Provider: &provider,
									Version:  version,
								},
								{
									Provider: &config,
									Version:  version,
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.c.ConvertFrom(tc.hub)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nc.ConvertFrom(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.c, tc.c); diff != "" {
				t.Errorf("\n%s\nc.ConvertFrom(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

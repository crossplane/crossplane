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

package initializer

import (
	"context"
	"reflect"

	"github.com/spf13/afero"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errApplyWebhookConfiguration = "cannot apply webhook configuration"
)

// WithWebhookConfigurationsFs is used to configure the filesystem the CRDs will
// be read from. Its default is afero.OsFs.
func WithWebhookConfigurationsFs(fs afero.Fs) WebhookConfigurationsOption {
	return func(c *WebhookConfigurations) {
		c.fs = fs
	}
}

// WebhookConfigurationsOption configures WebhookConfigurations step.
type WebhookConfigurationsOption func(*WebhookConfigurations)

// NewWebhookConfigurations returns a new *WebhookConfigurations.
func NewWebhookConfigurations(path string, s *runtime.Scheme, tlsSecretRef types.NamespacedName, svc admv1.ServiceReference, opts ...WebhookConfigurationsOption) *WebhookConfigurations {
	c := &WebhookConfigurations{
		Path:             path,
		Scheme:           s,
		TLSSecretRef:     tlsSecretRef,
		ServiceReference: svc,
		fs:               afero.NewOsFs(),
	}
	for _, f := range opts {
		f(c)
	}
	return c
}

// WebhookConfigurations makes sure the ValidatingWebhookConfigurations and
// MutatingWebhookConfiguration are installed.
type WebhookConfigurations struct {
	Path             string
	Scheme           *runtime.Scheme
	TLSSecretRef     types.NamespacedName
	ServiceReference admv1.ServiceReference

	fs afero.Fs
}

// Run applies all webhook ValidatingWebhookConfigurations and
// MutatingWebhookConfiguration in the given directory.
func (c *WebhookConfigurations) Run(ctx context.Context, kube client.Client) error { //nolint:gocyclo // Only slightly over (11).
	s := &corev1.Secret{}
	if err := kube.Get(ctx, c.TLSSecretRef, s); err != nil {
		return errors.Wrap(err, errGetWebhookSecret)
	}
	if len(s.Data["tls.crt"]) == 0 {
		return errors.Errorf(errFmtNoTLSCrtInSecret, c.TLSSecretRef.String())
	}
	caBundle := s.Data["tls.crt"]

	r, err := parser.NewFsBackend(c.fs,
		parser.FsDir(c.Path),
		parser.FsFilters(
			parser.SkipDirs(),
			parser.SkipNotYAML(),
			parser.SkipEmpty(),
		),
	).Init(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot init filesystem")
	}
	defer func() { _ = r.Close() }()
	p := parser.New(runtime.NewScheme(), c.Scheme)
	pkg, err := p.Parse(ctx, r)
	if err != nil {
		return errors.Wrap(err, "cannot parse files")
	}
	pa := resource.NewAPIPatchingApplicator(kube)
	for _, obj := range pkg.GetObjects() {
		switch conf := obj.(type) {
		case *admv1.ValidatingWebhookConfiguration:
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = caBundle
				conf.Webhooks[i].ClientConfig.Service.Name = c.ServiceReference.Name
				conf.Webhooks[i].ClientConfig.Service.Namespace = c.ServiceReference.Namespace
				conf.Webhooks[i].ClientConfig.Service.Port = c.ServiceReference.Port
			}
			// See https://github.com/kubernetes-sigs/controller-tools/issues/658
			conf.SetName("crossplane")
		case *admv1.MutatingWebhookConfiguration:
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = caBundle
				conf.Webhooks[i].ClientConfig.Service.Name = c.ServiceReference.Name
				conf.Webhooks[i].ClientConfig.Service.Namespace = c.ServiceReference.Namespace
				conf.Webhooks[i].ClientConfig.Service.Port = c.ServiceReference.Port
			}
			// See https://github.com/kubernetes-sigs/controller-tools/issues/658
			conf.SetName("crossplane")
		default:
			return errors.Errorf("only MutatingWebhookConfiguration and ValidatingWebhookConfiguration kinds are accepted, got %s", reflect.TypeOf(obj).String())
		}
		if err := pa.Apply(ctx, obj.(client.Object)); err != nil {
			return errors.Wrap(err, errApplyWebhookConfiguration)
		}
	}
	return nil
}

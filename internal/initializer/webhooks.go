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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
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
func NewWebhookConfigurations(path string, s *runtime.Scheme, webhookCertPath string, opts ...WebhookConfigurationsOption) *WebhookConfigurations {
	c := &WebhookConfigurations{
		Path:               path,
		Scheme:             s,
		WebhookTLSCertPath: webhookCertPath,
		fs:                 afero.NewOsFs(),
	}
	for _, f := range opts {
		f(c)
	}
	return c
}

// WebhookConfigurations makes sure the CRDs are installed.
type WebhookConfigurations struct {
	Path               string
	Scheme             *runtime.Scheme
	WebhookTLSCertPath string

	fs afero.Fs
}

// Run applies all CRDs in the given directory.
func (c *WebhookConfigurations) Run(ctx context.Context, kube client.Client) error { // nolint:gocyclo
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
	caBundle, err := afero.ReadFile(c.fs, c.WebhookTLSCertPath)
	if err != nil {
		return errors.Wrapf(err, errReadTLSCertFmt, c.WebhookTLSCertPath)
	}
	pa := resource.NewAPIPatchingApplicator(kube)
	for _, obj := range pkg.GetObjects() {
		switch conf := obj.(type) {
		case *admv1.ValidatingWebhookConfiguration:
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = caBundle
			}
		case *admv1.MutatingWebhookConfiguration:
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = caBundle
			}
		default:
			return errors.Errorf("only MutatingWebhookConfiguration and ValidatingWebhookConfiguration kinds are accepted, got %s", reflect.TypeOf(obj).String())
		}
		if err := pa.Apply(ctx, obj.(client.Object)); err != nil {
			return errors.Wrap(err, errApplyWebhookConfiguration)
		}
	}
	return nil
}

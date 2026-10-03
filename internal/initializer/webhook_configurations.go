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

	"github.com/spf13/afero"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xpkg/parser"
)

const (
	errApplyWebhookConfiguration = "cannot apply webhook configuration"
	errGetWebhookSecret          = "cannot get webhook secret"
)

// WithWebhookConfigurationsFs is used to configure the filesystem the CRDs will
// be read from. Its default is afero.OsFs.
func WithWebhookConfigurationsFs(fs afero.Fs) WebhookConfigurationsOption {
	return func(c *WebhookConfigurations) {
		c.fs = fs
	}
}

// WithSkippedConfigurations is used to skip applying the webhook
// configurations with the supplied metadata names.
func WithSkippedConfigurations(names ...string) WebhookConfigurationsOption {
	return func(c *WebhookConfigurations) {
		if c.skip == nil {
			c.skip = make(map[string]bool, len(names))
		}
		for _, n := range names {
			c.skip[n] = true
		}
	}
}

// WithFailurePolicyOverride is used to override the failurePolicy of every
// webhook of the ValidatingWebhookConfiguration with the supplied metadata
// name at apply time.
func WithFailurePolicyOverride(configName string, p admv1.FailurePolicyType) WebhookConfigurationsOption {
	return func(c *WebhookConfigurations) {
		if c.failurePolicyOverrides == nil {
			c.failurePolicyOverrides = make(map[string]admv1.FailurePolicyType, 1)
		}
		c.failurePolicyOverrides[configName] = p
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

	fs                     afero.Fs
	skip                   map[string]bool
	failurePolicyOverrides map[string]admv1.FailurePolicyType
}

// Run applies all webhook ValidatingWebhookConfigurations and
// MutatingWebhookConfiguration in the given directory.
func (c *WebhookConfigurations) Run(ctx context.Context, kube client.Client) error {
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
		if mo, ok := obj.(client.Object); ok && c.skip[mo.GetName()] {
			continue
		}

		switch conf := obj.(type) {
		case *admv1.ValidatingWebhookConfiguration:
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = caBundle
				conf.Webhooks[i].ClientConfig.Service.Name = c.ServiceReference.Name
				conf.Webhooks[i].ClientConfig.Service.Namespace = c.ServiceReference.Namespace
				conf.Webhooks[i].ClientConfig.Service.Port = c.ServiceReference.Port

				// Note that the applicator uses a JSON merge patch of the full
				// desired object, so an override set here (or reverted to the
				// value in the packaged YAML) always takes effect on re-apply.
				if p, ok := c.failurePolicyOverrides[conf.GetName()]; ok {
					conf.Webhooks[i].FailurePolicy = &p
				}
			}
		case *admv1.MutatingWebhookConfiguration:
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = caBundle
				conf.Webhooks[i].ClientConfig.Service.Name = c.ServiceReference.Name
				conf.Webhooks[i].ClientConfig.Service.Namespace = c.ServiceReference.Namespace
				conf.Webhooks[i].ClientConfig.Service.Port = c.ServiceReference.Port
			}
		default:
			return errors.Errorf("only MutatingWebhookConfiguration and ValidatingWebhookConfiguration kinds are accepted, got %T", obj)
		}

		if err := pa.Apply(ctx, obj.(client.Object)); err != nil { //nolint:forcetypeassert // Should always be a client.Object.
			return errors.Wrap(err, errApplyWebhookConfiguration)
		}
	}

	return nil
}

// NewValidatingWebhookRemover returns an initializer that removes webhooks from
// a validating webhook configuration.
func NewValidatingWebhookRemover(configName string, webhookNames ...string) *RemoveValidatingWebhooks {
	return &RemoveValidatingWebhooks{ConfigName: configName, WebhookNames: webhookNames}
}

// RemoveValidatingWebhooks removes webhooks from a validating webhook
// configuration. This needs to be done when we stop serving a webhook, or all
// requests for the type will fail.
type RemoveValidatingWebhooks struct {
	ConfigName   string
	WebhookNames []string
}

// Run removes the named webhooks from the named config.
func (c *RemoveValidatingWebhooks) Run(ctx context.Context, kube client.Client) error {
	cleanup := make(map[string]bool)
	for _, name := range c.WebhookNames {
		cleanup[name] = true
	}

	vwc := &admv1.ValidatingWebhookConfiguration{}

	err := kube.Get(ctx, client.ObjectKey{Name: c.ConfigName}, vwc)
	if kerrors.IsNotFound(err) {
		// If the webhook configuration doesn't exist there's nothing to cleanup.
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "cannot get ValidatingWebhookConfiguration %q", c.ConfigName)
	}

	whs := make([]admv1.ValidatingWebhook, 0, len(vwc.Webhooks))
	for _, wh := range vwc.Webhooks {
		// We want to remove this webhook - don't keep it.
		if cleanup[wh.Name] {
			continue
		}

		whs = append(whs, wh)
	}

	// We didn't cleanup any webhooks - no need to update.
	if len(whs) == len(vwc.Webhooks) {
		return nil
	}

	// The configuration is now empty - delete it.
	if len(whs) == 0 {
		return errors.Wrapf(resource.IgnoreNotFound(kube.Delete(ctx, vwc)), "cannot delete ValidatingWebhookConfiguration %q", c.ConfigName)
	}

	// TODO(negz): Retry on version conflict? I can't imagine this resource
	// changes that much, and this only happens once at install time...
	vwc.Webhooks = whs

	return errors.Wrapf(kube.Update(ctx, vwc), "cannot update ValidatingWebhookConfiguration %q", c.ConfigName)
}

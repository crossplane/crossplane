/*
Copyright 2021 The Crossplane Authors.

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
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errFmtCRDWithConversionWithoutTLS = "crds with webhook conversion strategy cannot be deployed without webhook tls secret: %s"
	errFmtNoTLSCrtInSecret            = "cannot find tls.crt key in webhook tls secret %s"
)

// WithWebhookTLSSecretRef configures CoreCRDs with the TLS Secret name so that
// it can fetch it and inject the CA bundle to CRDs with webhook conversion
// strategy.
func WithWebhookTLSSecretRef(nn types.NamespacedName) CoreCRDsOption {
	return func(c *CoreCRDs) {
		c.WebhookTLSSecretRef = &nn
	}
}

// WithFs is used to configure the filesystem the CRDs will be read from. Its
// default is afero.OsFs.
func WithFs(fs afero.Fs) CoreCRDsOption {
	return func(c *CoreCRDs) {
		c.fs = fs
	}
}

// CoreCRDsOption configures CoreCRDs step.
type CoreCRDsOption func(*CoreCRDs)

// NewCoreCRDs returns a new *CoreCRDs.
func NewCoreCRDs(path string, s *runtime.Scheme, opts ...CoreCRDsOption) *CoreCRDs {
	c := &CoreCRDs{
		Path:   path,
		Scheme: s,
		fs:     afero.NewOsFs(),
	}
	for _, f := range opts {
		f(c)
	}
	return c
}

// CoreCRDs makes sure the CRDs are installed.
type CoreCRDs struct {
	Path                string
	Scheme              *runtime.Scheme
	WebhookTLSSecretRef *types.NamespacedName

	fs afero.Fs
}

// Run applies all CRDs in the given directory.
func (c *CoreCRDs) Run(ctx context.Context, kube client.Client) error {
	var caBundle []byte
	if c.WebhookTLSSecretRef != nil {
		s := &corev1.Secret{}
		if err := kube.Get(ctx, *c.WebhookTLSSecretRef, s); err != nil {
			return errors.Wrap(err, errGetWebhookSecret)
		}
		if len(s.Data["tls.crt"]) == 0 {
			return errors.Errorf(errFmtNoTLSCrtInSecret, c.WebhookTLSSecretRef.String())
		}
		caBundle = s.Data["tls.crt"]
	}

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
		crd, ok := obj.(*extv1.CustomResourceDefinition)
		if !ok {
			return errors.New("only crds can exist in initialization directory")
		}
		if crd.Spec.Conversion != nil && crd.Spec.Conversion.Strategy == extv1.WebhookConverter {
			if len(caBundle) == 0 {
				return errors.Errorf(errFmtCRDWithConversionWithoutTLS, crd.Name)
			}
			if crd.Spec.Conversion.Webhook == nil {
				crd.Spec.Conversion.Webhook = &extv1.WebhookConversion{}
			}
			if crd.Spec.Conversion.Webhook.ClientConfig == nil {
				crd.Spec.Conversion.Webhook.ClientConfig = &extv1.WebhookClientConfig{}
			}
			crd.Spec.Conversion.Webhook.ClientConfig.CABundle = caBundle
		}
		if err := pa.Apply(ctx, crd); err != nil {
			return errors.Wrap(err, "cannot apply crd")
		}
	}
	return nil
}

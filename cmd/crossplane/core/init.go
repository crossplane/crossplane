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

package core

import (
	"context"
	"fmt"

	admv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/initializer"
)

// initCommand configuration for the initialization of core Crossplane controllers.
type initCommand struct {
	Providers      []string `help:"Pre-install a Provider by giving its image URI. This argument can be repeated."      name:"provider"`
	Configurations []string `help:"Pre-install a Configuration by giving its image URI. This argument can be repeated." name:"configuration"`
	Functions      []string `help:"Pre-install a Function by giving its image URI. This argument can be repeated."      name:"function"`
	Namespace      string   `default:"crossplane-system"                                                                env:"POD_NAMESPACE"       help:"Namespace used to set as default scope in default secret store config." short:"n"`
	ServiceAccount string   `default:"crossplane"                                                                       env:"POD_SERVICE_ACCOUNT" help:"Name of the Crossplane Service Account."`

	WebhookEnabled          bool   `default:"true"                   env:"WEBHOOK_ENABLED"                                                                         help:"Enable webhook configuration."`
	WebhookServiceName      string `env:"WEBHOOK_SERVICE_NAME"       help:"The name of the Service object that the webhook service will be run."`
	WebhookServiceNamespace string `env:"WEBHOOK_SERVICE_NAMESPACE"  help:"The namespace of the Service object that the webhook service will be run."`
	WebhookServicePort      int32  `env:"WEBHOOK_SERVICE_PORT"       help:"The port of the Service that the webhook service will be run."`
	ESSTLSServerSecretName  string `env:"ESS_TLS_SERVER_SECRET_NAME" help:"The name of the Secret that the initializer will fill with ESS TLS server certificate."`
	TLSCASecretName         string `env:"TLS_CA_SECRET_NAME"         help:"The name of the Secret that the initializer will fill with TLS CA certificate."`
	TLSServerSecretName     string `env:"TLS_SERVER_SECRET_NAME"     help:"The name of the Secret that the initializer will fill with TLS server certificates."`
	TLSClientSecretName     string `env:"TLS_CLIENT_SECRET_NAME"     help:"The name of the Secret that the initializer will fill with TLS client certificates."`
}

// Run starts the initialization process.
func (c *initCommand) Run(s *runtime.Scheme, log logging.Logger) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "cannot get config")
	}

	cl, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		return errors.Wrap(err, "cannot create new kubernetes client")
	}
	var steps []initializer.Step
	tlsGeneratorOpts := []initializer.TLSCertificateGeneratorOption{
		initializer.TLSCertificateGeneratorWithClientSecretName(c.TLSClientSecretName, []string{fmt.Sprintf("%s.%s", c.ServiceAccount, c.Namespace)}),
		initializer.TLSCertificateGeneratorWithLogger(log.WithValues("Step", "TLSCertificateGenerator")),
	}
	if c.WebhookEnabled {
		tlsGeneratorOpts = append(tlsGeneratorOpts,
			initializer.TLSCertificateGeneratorWithServerSecretName(c.TLSServerSecretName, initializer.DNSNamesForService(c.WebhookServiceName, c.WebhookServiceNamespace)))
	}
	steps = append(steps,
		initializer.NewTLSCertificateGenerator(c.Namespace, c.TLSCASecretName, tlsGeneratorOpts...),
		initializer.NewCoreCRDsMigrator("compositionrevisions.apiextensions.crossplane.io", "v1alpha1"),
		initializer.NewCoreCRDsMigrator("locks.pkg.crossplane.io", "v1alpha1"),
		initializer.NewCoreCRDsMigrator("functions.pkg.crossplane.io", "v1beta1"),
		initializer.NewCoreCRDsMigrator("functionrevisions.pkg.crossplane.io", "v1beta1"),
	)
	if c.WebhookEnabled {
		nn := types.NamespacedName{
			Name:      c.TLSServerSecretName,
			Namespace: c.Namespace,
		}
		svc := admv1.ServiceReference{
			Name:      c.WebhookServiceName,
			Namespace: c.WebhookServiceNamespace,
			Port:      &c.WebhookServicePort,
		}
		steps = append(steps,
			initializer.NewCoreCRDs("/crds", s, initializer.WithWebhookTLSSecretRef(nn)),
			initializer.NewWebhookConfigurations("/webhookconfigurations", s, nn, svc))
	} else {
		steps = append(steps,
			initializer.NewCoreCRDs("/crds", s),
		)
	}

	if c.ESSTLSServerSecretName != "" {
		steps = append(steps, initializer.NewTLSCertificateGenerator(c.Namespace, c.TLSCASecretName,
			initializer.TLSCertificateGeneratorWithServerSecretName(c.ESSTLSServerSecretName, []string{fmt.Sprintf("*.%s", c.Namespace)}),
			initializer.TLSCertificateGeneratorWithLogger(log.WithValues("Step", "ESSCertificateGenerator")),
		))
	}

	steps = append(steps, initializer.NewLockObject(),
		initializer.NewPackageInstaller(c.Providers, c.Configurations, c.Functions),
		initializer.NewStoreConfigObject(c.Namespace),
		initializer.StepFunc(initializer.DefaultDeploymentRuntimeConfig),
	)

	if err := initializer.New(cl, log, steps...).Init(context.TODO()); err != nil {
		return errors.Wrap(err, "cannot initialize core")
	}
	log.Info("Initialization has been completed")
	return nil
}

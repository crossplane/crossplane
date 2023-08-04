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
	Providers      []string `name:"provider" help:"Pre-install a Provider by giving its image URI. This argument can be repeated."`
	Configurations []string `name:"configuration" help:"Pre-install a Configuration by giving its image URI. This argument can be repeated."`
	Namespace      string   `short:"n" help:"Namespace used to set as default scope in default secret store config." default:"crossplane-system" env:"POD_NAMESPACE"`

	WebhookTLSSecretName    string `help:"The name of the Secret that the initializer will fill with webhook TLS certificate bundle." env:"WEBHOOK_TLS_SECRET_NAME"`
	WebhookServiceName      string `help:"The name of the Service object that the webhook service will be run." env:"WEBHOOK_SERVICE_NAME"`
	WebhookServiceNamespace string `help:"The namespace of the Service object that the webhook service will be run." env:"WEBHOOK_SERVICE_NAMESPACE"`
	WebhookServicePort      int32  `help:"The port of the Service that the webhook service will be run." env:"WEBHOOK_SERVICE_PORT"`
	ESSTLSClientSecretName  string `help:"The name of the Secret that the initializer will fill with ESS TLS client certificate." env:"ESS_TLS_CLIENT_SECRET_NAME"`
	ESSTLSServerSecretName  string `help:"The name of the Secret that the initializer will fill with ESS TLS server certificate." env:"ESS_TLS_SERVER_SECRET_NAME"`
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
	if c.WebhookTLSSecretName != "" {
		nn := types.NamespacedName{
			Name:      c.WebhookTLSSecretName,
			Namespace: c.Namespace,
		}
		svc := admv1.ServiceReference{
			Name:      c.WebhookServiceName,
			Namespace: c.WebhookServiceNamespace,
			Port:      &c.WebhookServicePort,
		}
		steps = append(steps,
			initializer.NewWebhookCertificateGenerator(nn, c.Namespace,
				log.WithValues("Step", "WebhookCertificateGenerator")),
			initializer.NewCoreCRDsMigrator("compositionrevisions.apiextensions.crossplane.io", "v1alpha1"),
			initializer.NewCoreCRDsMigrator("locks.pkg.crossplane.io", "v1alpha1"),
			initializer.NewCoreCRDs("/crds", s, initializer.WithWebhookTLSSecretRef(nn)),
			initializer.NewWebhookConfigurations("/webhookconfigurations", s, nn, svc))
	} else {
		steps = append(steps,
			initializer.NewCoreCRDsMigrator("compositionrevisions.apiextensions.crossplane.io", "v1alpha1"),
			initializer.NewCoreCRDsMigrator("locks.pkg.crossplane.io", "v1alpha1"),
			initializer.NewCoreCRDs("/crds", s),
		)
	}

	if c.ESSTLSClientSecretName != "" && c.ESSTLSServerSecretName != "" {
		steps = append(steps,
			initializer.NewESSCertificateGenerator(c.Namespace, c.ESSTLSClientSecretName, c.ESSTLSServerSecretName, initializer.ESSCertificateGeneratorWithLogger(log.WithValues("Step", "ESSCertificateGenerator"))),
		)
	}

	steps = append(steps, initializer.NewLockObject(),
		initializer.NewPackageInstaller(c.Providers, c.Configurations),
		initializer.NewStoreConfigObject(c.Namespace))
	if err := initializer.New(cl, log, steps...).Init(context.TODO()); err != nil {
		return errors.Wrap(err, "cannot initialize core")
	}
	log.Info("Initialization has been completed")
	return nil
}

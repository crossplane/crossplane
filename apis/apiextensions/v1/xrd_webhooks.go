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

package v1

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errConversionWebhookConfigRequired = "spec.conversion.webhook is required when spec.conversion.strategy is 'Webhook'"
)

// NOTE(negz): We only validate updates because we're only using the validation
// webhook to enforce a few immutable fields. We should look into using CEL per
// https://github.com/crossplane/crossplane/issues/4128 instead.

// +kubebuilder:webhook:verbs=create;update,path=/validate-apiextensions-crossplane-io-v1-compositeresourcedefinition,mutating=false,failurePolicy=fail,groups=apiextensions.crossplane.io,resources=compositeresourcedefinitions,versions=v1,name=compositeresourcedefinitions.apiextensions.crossplane.io,sideEffects=None,admissionReviewVersions=v1

// ValidateCreate is run for creation actions.
func (in *CompositeResourceDefinition) ValidateCreate() (admission.Warnings, error) {
	// TODO(negz): Does this code ever get exercised in reality? We don't
	// register the update verb when we generate a configuration above.
	if c := in.Spec.Conversion; c != nil && c.Strategy == extv1.WebhookConverter && c.Webhook == nil {
		return nil, errors.New(errConversionWebhookConfigRequired)
	}
	return nil, nil
}

// ValidateUpdate is run for update actions.
func (in *CompositeResourceDefinition) ValidateUpdate(_ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete is run for delete actions.
func (in *CompositeResourceDefinition) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

// SetupWebhookWithManager sets up webhook with manager.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&CompositeResourceDefinition{}).
		Complete()
}

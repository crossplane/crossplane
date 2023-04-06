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

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errUnexpectedType = "unexpected type"

	errGroupImmutable                  = "spec.group is immutable"
	errPluralImmutable                 = "spec.names.plural is immutable"
	errKindImmutable                   = "spec.names.kind is immutable"
	errClaimPluralImmutable            = "spec.claimNames.plural is immutable"
	errClaimKindImmutable              = "spec.claimNames.kind is immutable"
	errConversionWebhookConfigRequired = "spec.conversion.webhook is required when spec.conversion.strategy is 'Webhook'"
)

// +kubebuilder:webhook:verbs=update,path=/validate-apiextensions-crossplane-io-v1-compositeresourcedefinition,mutating=false,failurePolicy=fail,groups=apiextensions.crossplane.io,resources=compositeresourcedefinitions,versions=v1,name=compositeresourcedefinitions.apiextensions.crossplane.io,sideEffects=None,admissionReviewVersions=v1

// ValidateCreate is run for creation actions.
func (in *CompositeResourceDefinition) ValidateCreate() error {
	if c := in.Spec.Conversion; c != nil && c.Strategy == extv1.WebhookConverter && c.Webhook == nil {
		return errors.New(errConversionWebhookConfigRequired)
	}
	return nil
}

// ValidateUpdate is run for update actions.
func (in *CompositeResourceDefinition) ValidateUpdate(old runtime.Object) error {
	oldObj, ok := old.(*CompositeResourceDefinition)
	if !ok {
		return errors.New(errUnexpectedType)
	}
	switch {
	case in.Spec.Group != oldObj.Spec.Group:
		return errors.New(errGroupImmutable)
	case in.Spec.Names.Plural != oldObj.Spec.Names.Plural:
		return errors.New(errPluralImmutable)
	case in.Spec.Names.Kind != oldObj.Spec.Names.Kind:
		return errors.New(errKindImmutable)
	}
	if in.Spec.ClaimNames != nil && oldObj.Spec.ClaimNames != nil {
		switch {
		case in.Spec.ClaimNames.Plural != oldObj.Spec.ClaimNames.Plural:
			return errors.New(errClaimPluralImmutable)
		case in.Spec.ClaimNames.Kind != oldObj.Spec.ClaimNames.Kind:
			return errors.New(errClaimKindImmutable)
		}
	}
	return nil
}

// ValidateDelete is run for delete actions.
func (in *CompositeResourceDefinition) ValidateDelete() error {
	return nil
}

// SetupWebhookWithManager sets up webhook with manager.
func (in *CompositeResourceDefinition) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		Complete()
}

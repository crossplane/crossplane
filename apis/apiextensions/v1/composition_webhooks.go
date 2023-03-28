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

// +kubebuilder:webhook:verbs=update;create,path=/validate-apiextensions-crossplane-io-v1-composition,mutating=false,failurePolicy=fail,groups=apiextensions.crossplane.io,resources=compositions,versions=v1,name=compositions.apiextensions.crossplane.io,sideEffects=None,admissionReviewVersions=v1

package v1

const (
	// CompositionValidatingWebhookPath is the path for the Composition's validating webhook, should be kept in sync with the annotation above.
	CompositionValidatingWebhookPath = "/validate-apiextensions-crossplane-io-v1-composition"
)

// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

// +kubebuilder:webhook:verbs=create;update,path=/validate-apiextensions-crossplane-io-v1-compositeresourcedefinition,mutating=false,failurePolicy=fail,groups=apiextensions.crossplane.io,resources=compositeresourcedefinitions,versions=v1,name=compositeresourcedefinitions.apiextensions.crossplane.io,sideEffects=None,admissionReviewVersions=v1

/*
Copyright 2019 The Crossplane Authors.

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

// Package features defines Crossplane feature flags.
package features

import "github.com/crossplane/crossplane-runtime/pkg/feature"

// Alpha Feature flags.
const (
	// EnableAlphaEnvironmentConfigs enables alpha support for composition
	// environments. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/c4bcbe/design/one-pager-composition-environment.md
	EnableAlphaEnvironmentConfigs feature.Flag = "EnableAlphaEnvironmentConfigs"

	// EnableAlphaExternalSecretStores enables alpha support for
	// External Secret Stores. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/390ddd/design/design-doc-external-secret-stores.md
	EnableAlphaExternalSecretStores feature.Flag = "EnableAlphaExternalSecretStores"

	// EnableAlphaUsages enables alpha support for deletion ordering and
	// protection with Usage resource. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/19ea23/design/one-pager-generic-usage-type.md
	EnableAlphaUsages feature.Flag = "EnableAlphaUsages"

	// EnableAlphaRealtimeCompositions enables alpha support for realtime
	// compositions, i.e. watching MRs and reconciling compositions immediately
	// when any MR is updated.
	EnableAlphaRealtimeCompositions feature.Flag = "EnableAlphaRealtimeCompositions"

	// EnableAlphaClaimSSA enables alpha support for using server-side apply in
	// the claim controller. See the below issue for more details:
	// https://github.com/crossplane/crossplane/issues/4581
	EnableAlphaClaimSSA feature.Flag = "EnableAlphaClaimSSA"
)

// Beta Feature Flags.
const (
	// EnableBetaCompositionWebhookSchemaValidation enables alpha support for
	// composition webhook schema validation. See the below design for more
	// details.
	// https://github.com/crossplane/crossplane/blob/f32496/design/design-doc-composition-validating-webhook.md
	EnableBetaCompositionWebhookSchemaValidation feature.Flag = "EnableBetaCompositionWebhookSchemaValidation"

	// EnableBetaDeploymentRuntimeConfigs enables beta support for deployment
	// runtime configs. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/c2e206/design/one-pager-package-runtime-config.md
	EnableBetaDeploymentRuntimeConfigs feature.Flag = "EnableBetaDeploymentRuntimeConfigs"
)

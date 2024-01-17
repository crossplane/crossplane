// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
	// https://github.com/crossplane/crossplane/blob/19ea23e7c1fc16b20581755540f9f45afdf89338/design/one-pager-generic-usage-type.md
	EnableAlphaUsages feature.Flag = "EnableAlphaUsages"

	// EnableRealtimeCompositions enables alpha support for realtime
	// compositions, i.e. watching MRs and reconciling compositions immediately
	// when any MR is updated.
	EnableRealtimeCompositions feature.Flag = "EnableRealtimeCompositions"
)

// Beta Feature Flags
const (
	// EnableBetaCompositionFunctions enables alpha support for composition
	// functions. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/863ff6/design/design-doc-composition-functions.md
	EnableBetaCompositionFunctions feature.Flag = "EnableBetaCompositionFunctions"

	// EnableBetaCompositionWebhookSchemaValidation enables alpha support for
	// composition webhook schema validation. See the below design for more
	// details.
	// https://github.com/crossplane/crossplane/blob/f32496bed53a393c8239376fd8266ddf2ef84d61/design/design-doc-composition-validating-webhook.md
	EnableBetaCompositionWebhookSchemaValidation feature.Flag = "EnableBetaCompositionWebhookSchemaValidation"

	// EnableBetaDeploymentRuntimeConfigs enables beta support for deployment
	// runtime configs. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/c2e206/design/one-pager-package-runtime-config.md
	EnableBetaDeploymentRuntimeConfigs feature.Flag = "EnableBetaDeploymentRuntimeConfigs"
)

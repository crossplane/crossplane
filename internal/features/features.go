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

// Feature flags.
const (
	// EnableBetaCompositionRevisions enables beta support for
	// CompositionRevisions. See the below docs for more details.
	// https://github.com/crossplane/crossplane/blob/ecd9d5/design/one-pager-composition-revisions.md
	// https://github.com/crossplane/crossplane/issues/3415
	EnableBetaCompositionRevisions feature.Flag = "EnableBetaCompositionRevisions"
	// EnableAlphaEnvironmentConfigs enables alpha support for composition
	// environments. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/c4bcbe/design/one-pager-composition-environment.md
	EnableAlphaEnvironmentConfigs feature.Flag = "EnableAlphaEnvironmentConfigs"
	// EnableAlphaExternalSecretStores enables alpha support for
	// External Secret Stores. See the below design for more details.
	// https://github.com/crossplane/crossplane/blob/390ddd/design/design-doc-external-secret-stores.md
	EnableAlphaExternalSecretStores feature.Flag = "EnableAlphaExternalSecretStores"
)

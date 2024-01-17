// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package e2e

// LabelArea represents the 'area' of a feature. For example 'apiextensions',
// 'pkg', etc. Assessments roll up to features, which roll up to feature areas.
// Features within an area may be split across different test functions.
const LabelArea = "area"

// LabelModifyCrossplaneInstallation is used to mark tests that are going to
// modify Crossplane's installation, e.g. installing, uninstalling or upgrading
// it.
const LabelModifyCrossplaneInstallation = "modify-crossplane-installation"

// LabelModifyCrossplaneInstallationTrue is used to mark tests that are going to
// modify Crossplane's installation.
const LabelModifyCrossplaneInstallationTrue = "true"

// LabelStage represents the 'stage' of a feature - alpha, beta, etc. Generally
// available features have no stage label.
const LabelStage = "stage"

const (
	// LabelStageAlpha is used for tests of alpha features.
	LabelStageAlpha = "alpha"

	// LabelStageBeta is used for tests of beta features.
	LabelStageBeta = "beta"
)

// LabelSize represents the 'size' (i.e. duration) of a test.
const LabelSize = "size"

const (
	// LabelSizeSmall is used for tests that usually complete in a minute.
	LabelSizeSmall = "small"

	// LabelSizeLarge is used for test that usually complete in over a minute.
	LabelSizeLarge = "large"
)

// FieldManager is the server-side apply field manager used when applying
// manifests.
const FieldManager = "crossplane-e2e-tests"

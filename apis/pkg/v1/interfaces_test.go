// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

var _ Package = &Provider{}
var _ Package = &Configuration{}

var _ PackageRevision = &ProviderRevision{}
var _ PackageRevision = &ConfigurationRevision{}

var _ PackageRevisionList = &ProviderRevisionList{}
var _ PackageRevisionList = &ConfigurationRevisionList{}

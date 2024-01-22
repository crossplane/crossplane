// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import v1 "github.com/crossplane/crossplane/apis/pkg/v1"

var _ v1.Package = &Function{}
var _ v1.PackageRevision = &FunctionRevision{}
var _ v1.PackageRevisionList = &FunctionRevisionList{}

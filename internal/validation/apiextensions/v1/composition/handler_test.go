// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package composition

import "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

var _ admission.CustomValidator = &validator{}

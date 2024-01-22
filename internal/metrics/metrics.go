// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package metrics contains functionality for emitting Prometheus metrics.
package metrics

import "sigs.k8s.io/controller-runtime/pkg/metrics"

// Registry is a Prometheus metrics registry. All Crossplane metrics should be
// registered with it. Crossplane adds metrics to the registry created and
// served by controller-runtime.
var Registry = metrics.Registry

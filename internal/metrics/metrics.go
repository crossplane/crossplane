/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

// Package metrics contains functionality for emitting Prometheus metrics.
package metrics

import "sigs.k8s.io/controller-runtime/pkg/metrics"

// TODO(negz): Should we try to plumb the metrics registry down to all callers?
// I think this would be a good practice - similar to how we plumb the logger.
// On the other hand, using a global metrics registry is idiomatic for Prom.

// Registry is a Prometheus metrics registry. All Crossplane metrics should be
// registered with it. Crossplane adds metrics to the registry created and
// served by controller-runtime.
var Registry = metrics.Registry //nolint:gochecknoglobals // See TODO above.

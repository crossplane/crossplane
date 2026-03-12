/*
Copyright 2024 The Crossplane Authors.

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

package xmeta

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
)

// MinPollInterval is the smallest poll interval accepted via annotation.
// Anything below this is treated as invalid to prevent tight reconcile loops.
const MinPollInterval = 1 * time.Second

const (
	// AnnotationKeyPollInterval overrides the controller-level poll interval
	// for a specific resource. The value must be a valid Go duration string
	// (e.g. "1h", "30m", "24h") and at least MinPollInterval. When set, it
	// takes precedence over the global --poll-interval flag.
	AnnotationKeyPollInterval = "crossplane.io/poll-interval"

	// AnnotationKeyReconcileRequestedAt triggers an immediate reconciliation
	// when its value changes. The value is an opaque token, typically a
	// timestamp. After handling, the reconciler records the token in
	// status.lastHandledReconcileAt so users can confirm processing.
	AnnotationKeyReconcileRequestedAt = "crossplane.io/reconcile-requested-at"
)

// GetPollInterval returns the poll interval override for the given resource,
// if set via the AnnotationKeyPollInterval annotation. It returns the parsed
// duration and true if a valid annotation is present, or zero and false if the
// annotation is absent or cannot be parsed.
func GetPollInterval(o metav1.Object) (time.Duration, bool) {
	v, ok := o.GetAnnotations()[AnnotationKeyPollInterval]
	if !ok || v == "" {
		return 0, false
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, false
	}
	if d < MinPollInterval {
		return 0, false
	}
	return d, true
}

// GetReconcileRequest returns the reconcile-requested-at annotation token
// and true if present and non-empty, or empty string and false otherwise.
func GetReconcileRequest(o metav1.Object) (string, bool) {
	v, ok := o.GetAnnotations()[AnnotationKeyReconcileRequestedAt]
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// SetReconcileRequest sets the reconcile-requested-at annotation to the
// supplied token value.
func SetReconcileRequest(o metav1.Object, token string) {
	meta.AddAnnotations(o, map[string]string{AnnotationKeyReconcileRequestedAt: token})
}

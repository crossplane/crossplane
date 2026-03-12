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
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// GetLastHandledReconcileAt returns the most recently handled reconcile
// request token from the composite resource's status.
func GetLastHandledReconcileAt(c *composite.Unstructured) string {
	status := &xpv2.ObservedStatus{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status", status)
	return status.GetLastHandledReconcileAt()
}

// SetLastHandledReconcileAt sets the most recently handled reconcile request
// token in the composite resource's status.
func SetLastHandledReconcileAt(c *composite.Unstructured, token string) {
	_ = fieldpath.Pave(c.Object).SetValue("status.lastHandledReconcileAt", token)
}

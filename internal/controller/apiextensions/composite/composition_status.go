/*
Copyright 2026 The Crossplane Authors.

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

package composite

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

const statusField = "status"

// writeCompositionStatus publishes function status and final conditions with an
// optimistic lock, so a stale reconcile cannot overwrite newer status.
func (r *Reconciler) writeCompositionStatus(ctx context.Context, xr *composite.Unstructured, res CompositionResult, statusBefore any) error {
	err := r.applyCompositionStatus(ctx, xr, res, statusBefore)
	if err == nil || res.DesiredStatus == nil || isTransientStatusError(err) {
		return err
	}
	// SSA type errors can precede CRD validation. Report failures with valid
	// prior status and a stable message.
	r.record.Event(xr, event.Warning(reasonCompose, err))
	message := errUpdateStatus
	if kerrors.IsInvalid(err) {
		message = errInvalidResources
	}
	r.conditions.For(xr).MarkConditions(xpv2.ReconcileError(errors.New(message)))
	// Keep the last valid function status. Only the reconcile error, circuit
	// state, deletion state, and reconciler bookkeeping describe work
	// independently of the rejected pipeline status.
	pending := []xpv2.Condition{xr.GetCondition(xpv2.TypeSynced)}
	if c := xr.GetCondition(v1.TypeResponsive); c.Reason != "" {
		pending = append(pending, c)
	}
	if c := xr.GetCondition(xpv2.TypeReady); meta.WasDeleted(xr) && c.Reason == xpv2.ReasonDeleting {
		pending = append(pending, c)
	}
	token, hasToken, _ := kunstructured.NestedFieldCopy(xr.Object, statusField, "lastHandledReconcileAt")
	published := xr.GetConnectionDetailsLastPublishedTime()
	policy := xr.GetDeletionPolicy()
	if statusBefore != nil {
		xr.Object[statusField] = runtime.DeepCopyJSONValue(statusBefore)
	} else {
		delete(xr.Object, statusField)
	}
	if mergeStatusConditions(xr, pending...) != nil {
		return err
	}
	if hasToken {
		_ = kunstructured.SetNestedField(xr.Object, token, statusField, "lastHandledReconcileAt")
	}
	if published != nil {
		xr.SetConnectionDetailsLastPublishedTime(published)
	}
	if policy != "" {
		xr.SetDeletionPolicy(policy)
	}
	if !cmp.Equal(statusBefore, xr.Object[statusField]) {
		_ = r.client.Status().Update(ctx, xr)
	}
	return err
}

func isTransientStatusError(err error) bool {
	return kerrors.IsConflict(err) || kerrors.IsServiceUnavailable(err) ||
		kerrors.IsTimeout(err) || kerrors.IsServerTimeout(err) || kerrors.IsTooManyRequests(err) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// mergeStatusConditions replaces only the supplied types, preserving raw fields
// on other controllers' conditions and unchanged verdicts' transition times.
func mergeStatusConditions(xr *composite.Unstructured, conditions ...xpv2.Condition) error {
	stored, _, err := kunstructured.NestedSlice(xr.Object, statusField, "conditions")
	if err != nil {
		return err
	}
	for _, c := range conditions {
		i := slices.IndexFunc(stored, func(item any) bool {
			value, ok := item.(map[string]any)
			return ok && value["type"] == string(c.Type)
		})
		if i >= 0 {
			prior := xpv2.Condition{}
			value, _ := stored[i].(map[string]any)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(value, &prior); err != nil {
				return err
			}
			if prior.Equal(c) {
				c.LastTransitionTime = prior.LastTransitionTime
			}
		}
		value, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&c)
		if err != nil {
			return err
		}
		if i < 0 {
			stored = append(stored, value)
		} else {
			stored[i] = value
		}
	}
	return kunstructured.SetNestedSlice(xr.Object, stored, statusField, "conditions")
}

func (r *Reconciler) applyCompositionStatus(ctx context.Context, xr *composite.Unstructured, res CompositionResult, statusBefore any) error {
	if res.DesiredStatus == nil {
		if cmp.Equal(statusBefore, xr.Object[statusField]) {
			return nil
		}
		return errors.Wrap(r.client.Status().Update(ctx, xr), errUpdateStatus)
	}

	// Our goal here is to patch our XR's status using server-side apply. We
	// want the resulting, patched object loaded into xr. We need to pass in
	// only our "fully specified intent" - i.e. only the fields that we actually
	// care about, rather than all the fields from the cached XR.
	intent := composite.New(composite.WithSchema(xr.Schema), composite.WithGroupVersionKind(xr.GroupVersionKind()))
	intent.SetName(xr.GetName())
	intent.SetNamespace(xr.GetNamespace())
	intent.SetResourceVersion(xr.GetResourceVersion())
	desired := runtime.DeepCopyJSON(res.DesiredStatus)
	intent.Object[statusField] = desired
	// Keep the recorded deletion policy. The pipeline can't change it, and
	// leaving it out of this apply would delete it.
	intent.SetDeletionPolicy(xr.GetDeletionPolicy())

	// Keep this list in sync with the reconciler's status setters.
	for _, path := range [][]string{
		{statusField, "lastHandledReconcileAt"},
		{statusField, "connectionDetails", "lastPublishedTime"},
		{statusField, "claimConditionTypes"},
	} {
		value, found, err := kunstructured.NestedFieldCopy(xr.Object, path...)
		if err != nil {
			return errors.Wrap(err, errUpdateStatus)
		}
		if found {
			if err := kunstructured.SetNestedField(intent.Object, value, path...); err != nil {
				return errors.Wrap(kerrors.NewInvalid(xr.GroupVersionKind().GroupKind(), xr.GetName(), field.ErrorList{
					field.Invalid(field.NewPath(statusField, path[1]), desired[path[1]], err.Error()),
				}), errUpdateStatus)
			}
		}
	}

	conditions, err := compositionConditions(xr, res, statusBefore)
	if err != nil {
		return errors.Wrap(err, errUpdateStatus)
	}
	// SSA rejects a null condition list, so omit it when there are no conditions.
	delete(desired, "conditions")
	if len(conditions) > 0 {
		desired["conditions"] = conditions
	}

	// NOTE(phisco): Here we are fine using a hardcoded field owner as there is
	// no risk of conflict between different XRs.
	//nolint:staticcheck // TODO(adamwg) Stop using client.Apply after the v2.2 release.
	if err := r.client.Status().Patch(ctx, intent, client.Apply, client.ForceOwnership, client.FieldOwner(FieldOwnerXR)); err != nil {
		// Note(phisco): here we are fine with this error being terminal, as
		// there is no other resource to apply that might eventually resolve
		// this issue.
		return errors.Wrap(err, errUpdateStatus)
	}
	// Finalizer removal and error reporting need the latest persisted status.
	xr.Object[statusField] = runtime.DeepCopyJSONValue(intent.Object[statusField])
	xr.SetResourceVersion(intent.GetResourceVersion())
	xr.SetManagedFields(intent.GetManagedFields())
	return nil
}

func compositionConditions(xr *composite.Unstructured, res CompositionResult, statusBefore any) ([]any, error) {
	// Retain Crossplane's conditions without claiming other controllers' types.
	owned := appliedConditionTypes(xr.GetManagedFields())
	wanted := map[xpv2.ConditionType]bool{
		xpv2.TypeReady: true, xpv2.TypeSynced: true, v1.TypeResponsive: true,
	}
	for _, c := range res.Conditions {
		if !v1.IsSystemConditionType(c.Type) {
			wanted[c.Type] = true
		}
	}

	// Raw status may also contain conditions, including Healthy and UpToDate.
	raw := xpv2.ConditionedStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(res.DesiredStatus, &raw); err != nil {
		return nil, kerrors.NewInvalid(xr.GroupVersionKind().GroupKind(), xr.GetName(), field.ErrorList{
			field.Invalid(field.NewPath("status", "conditions"), res.DesiredStatus["conditions"], err.Error()),
		})
	}

	final := xpv2.ConditionedStatus{}
	for _, c := range xr.GetConditions() {
		if wanted[c.Type] || owned[c.Type] {
			final.Conditions = append(final.Conditions, c)
		}
	}
	for _, c := range raw.Conditions {
		if !wanted[c.Type] {
			i := slices.IndexFunc(final.Conditions, func(existing xpv2.Condition) bool { return existing.Type == c.Type })
			if i < 0 {
				final.Conditions = append(final.Conditions, c)
				continue
			}
			if final.Conditions[i].Equal(c) {
				c.LastTransitionTime = final.Conditions[i].LastTransitionTime
			}
			// Equal ignores a missing observedGeneration. Replace the value
			// anyway so an omitted or newly set generation is never dropped.
			final.Conditions[i] = c
		}
	}

	// An older status Update may still own optional fields. Include a
	// replacement value if an existing field needs to be cleared with SSA.
	previous, _, _ := kunstructured.NestedSlice(map[string]any{statusField: statusBefore}, statusField, "conditions")
	previousByType := make(map[string]map[string]any, len(previous))
	for _, item := range previous {
		if condition, ok := item.(map[string]any); ok {
			if t, ok := condition["type"].(string); ok {
				previousByType[t] = condition
			}
		}
	}

	conditions := make([]any, 0, len(final.Conditions))
	for _, c := range final.Conditions {
		value, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&c)
		if err != nil {
			return nil, err
		}
		if _, found := previousByType[string(c.Type)]["message"]; found {
			value["message"] = c.Message
		}
		if _, found := previousByType[string(c.Type)]["observedGeneration"]; found {
			value["observedGeneration"] = c.ObservedGeneration
		}
		conditions = append(conditions, value)
	}
	return conditions, nil
}

// appliedConditionTypes decodes this controller's SSA condition selectors.
func appliedConditionTypes(fields []metav1.ManagedFieldsEntry) map[xpv2.ConditionType]bool {
	for _, entry := range fields {
		if entry.Manager != FieldOwnerXR || entry.Subresource != statusField || entry.Operation != metav1.ManagedFieldsOperationApply || entry.FieldsV1 == nil {
			continue
		}
		var owned struct {
			Status struct {
				Conditions map[string]json.RawMessage `json:"f:conditions"`
			} `json:"f:status"`
		}
		if err := json.Unmarshal(entry.FieldsV1.GetRawBytes(), &owned); err != nil {
			continue
		}
		types := make(map[xpv2.ConditionType]bool, len(owned.Status.Conditions))
		for key := range owned.Status.Conditions {
			if len(key) < 2 || key[:2] != "k:" {
				continue
			}
			var selector struct {
				Type xpv2.ConditionType `json:"type"`
			}
			if err := json.Unmarshal([]byte(key[2:]), &selector); err == nil {
				types[selector.Type] = true
			}
		}
		return types
	}
	return nil
}

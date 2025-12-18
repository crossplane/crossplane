//go:build !goverter

/*
Copyright 2025 The Crossplane Authors.

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

package protection

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"

	legacy "github.com/crossplane/crossplane/v2/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/v2/apis/protection/v1beta1"
)

// InternalUsage wraps a Usage to implement the internal interface.
type InternalUsage struct {
	v1beta1.Usage
}

func (u *InternalUsage) Unwrap() conditions.ObjectWithConditions {
	return &u.Usage
}

// GetUserOf gets the resource this Usage indicates a use of.
func (u *InternalUsage) GetUserOf() Resource {
	conv := GeneratedNamespacedResourceConverter{}
	return conv.ToInternal(u.Spec.Of)
}

// SetUserOf sets the resource this Usage indicates a use of.
func (u *InternalUsage) SetUserOf(r Resource) {
	conv := GeneratedNamespacedResourceConverter{}
	u.Spec.Of = conv.FromInternal(r)
}

// GetUsedBy gets the resource this Usage indicates a use by.
func (u *InternalUsage) GetUsedBy() *Resource {
	if u.Spec.By == nil {
		return nil
	}

	conv := GeneratedResourceConverter{}
	out := conv.ToInternal(*u.Spec.By)

	return &out
}

// SetUsedBy sets the resource this Usage indicates a use by.
func (u *InternalUsage) SetUsedBy(r *Resource) {
	if r == nil {
		u.Spec.By = nil
		return
	}

	conv := GeneratedResourceConverter{}
	out := conv.FromInternal(*r)
	u.Spec.By = &out
}

// GetReason gets the reason this Usage exists.
func (u *InternalUsage) GetReason() *string {
	return u.Spec.Reason
}

// SetReason sets the reason this Usage exists.
func (u *InternalUsage) SetReason(reason *string) {
	u.Spec.Reason = reason
}

// GetReplayDeletion gets a boolean that indicates whether deletion of the used
// resource will be replayed when this Usage is deleted.
func (u *InternalUsage) GetReplayDeletion() *bool {
	return u.Spec.ReplayDeletion
}

// SetReplayDeletion specifies whether deletion of the used resource will be
// replayed when this Usage is deleted.
func (u *InternalUsage) SetReplayDeletion(replay *bool) {
	u.Spec.ReplayDeletion = replay
}

// InternalClusterUsage wraps a ClusterUsage to implement the internal interface.
type InternalClusterUsage struct {
	v1beta1.ClusterUsage
}

func (u *InternalClusterUsage) Unwrap() conditions.ObjectWithConditions {
	return &u.ClusterUsage
}

// GetUserOf gets the resource this ClusterUsage indicates a use of.
func (u *InternalClusterUsage) GetUserOf() Resource {
	conv := GeneratedResourceConverter{}
	return conv.ToInternal(u.Spec.Of)
}

// SetUserOf sets the resource this ClusterUsage indicates a use of.
func (u *InternalClusterUsage) SetUserOf(r Resource) {
	conv := GeneratedResourceConverter{}
	u.Spec.Of = conv.FromInternal(r)
}

// GetUsedBy gets the resource this ClusterUsage indicates a use by.
func (u *InternalClusterUsage) GetUsedBy() *Resource {
	if u.Spec.By == nil {
		return nil
	}

	conv := GeneratedResourceConverter{}
	out := conv.ToInternal(*u.Spec.By)

	return &out
}

// SetUsedBy sets the resource this ClusterUsage indicates a use by.
func (u *InternalClusterUsage) SetUsedBy(r *Resource) {
	if r == nil {
		u.Spec.By = nil
		return
	}

	conv := GeneratedResourceConverter{}
	out := conv.FromInternal(*r)
	u.Spec.By = &out
}

// GetReason gets the reason this ClusterUsage exists.
func (u *InternalClusterUsage) GetReason() *string {
	return u.Spec.Reason
}

// SetReason sets the reason this ClusterUsage exists.
func (u *InternalClusterUsage) SetReason(reason *string) {
	u.Spec.Reason = reason
}

// GetReplayDeletion gets a boolean that indicates whether deletion of the used
// resource will be replayed when this ClusterUsage is deleted.
func (u *InternalClusterUsage) GetReplayDeletion() *bool {
	return u.Spec.ReplayDeletion
}

// SetReplayDeletion specifies whether deletion of the used resource will be
// replayed when this ClusterUsage is deleted.
func (u *InternalClusterUsage) SetReplayDeletion(replay *bool) {
	u.Spec.ReplayDeletion = replay
}

// InternalLegacyUsage wraps a legacy Usage to implement the internal interface.
type InternalLegacyUsage struct {
	legacy.Usage
}

func (u *InternalLegacyUsage) Unwrap() conditions.ObjectWithConditions {
	return &u.Usage
}

// GetUserOf gets the resource this Usage indicates a use of.
func (u *InternalLegacyUsage) GetUserOf() Resource {
	conv := GeneratedLegacyResourceConverter{}
	return conv.ToInternal(u.Spec.Of)
}

// SetUserOf sets the resource this Usage indicates a use of.
func (u *InternalLegacyUsage) SetUserOf(r Resource) {
	conv := GeneratedLegacyResourceConverter{}
	u.Spec.Of = conv.FromInternal(r)
}

// GetUsedBy gets the resource this Usage indicates a use by.
func (u *InternalLegacyUsage) GetUsedBy() *Resource {
	if u.Spec.By == nil {
		return nil
	}

	conv := GeneratedLegacyResourceConverter{}
	out := conv.ToInternal(*u.Spec.By)

	return &out
}

// SetUsedBy sets the resource this Usage indicates a use by.
func (u *InternalLegacyUsage) SetUsedBy(r *Resource) {
	if r == nil {
		u.Spec.By = nil
		return
	}

	conv := GeneratedLegacyResourceConverter{}
	out := conv.FromInternal(*r)
	u.Spec.By = &out
}

// GetReason gets the reason this Usage exists.
func (u *InternalLegacyUsage) GetReason() *string {
	return u.Spec.Reason
}

// SetReason sets the reason this Usage exists.
func (u *InternalLegacyUsage) SetReason(reason *string) {
	u.Spec.Reason = reason
}

// GetReplayDeletion gets a boolean that indicates whether deletion of the used
// resource will be replayed when this Usage is deleted.
func (u *InternalLegacyUsage) GetReplayDeletion() *bool {
	return u.Spec.ReplayDeletion
}

// SetReplayDeletion specifies whether deletion of the used resource will be
// replayed when this Usage is deleted.
func (u *InternalLegacyUsage) SetReplayDeletion(replay *bool) {
	u.Spec.ReplayDeletion = replay
}

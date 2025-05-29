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

package v1beta1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane/internal/protection"
)

// GetUserOf gets the resource this ClusterUsage indicates a use of.
func (u *ClusterUsage) GetUserOf() protection.Resource {
	conv := GeneratedResourceConverter{}
	return conv.ToInternal(u.Spec.Of)
}

// SetUserOf sets the resource this ClusterUsage indicates a use of.
func (u *ClusterUsage) SetUserOf(r protection.Resource) {
	conv := GeneratedResourceConverter{}
	u.Spec.Of = conv.FromInternal(r)
}

// GetUsedBy gets the resource this ClusterUsage indicates a use by.
func (u *ClusterUsage) GetUsedBy() *protection.Resource {
	if u.Spec.By == nil {
		return nil
	}
	conv := GeneratedResourceConverter{}
	out := conv.ToInternal(*u.Spec.By)
	return &out
}

// SetUsedBy sets the resource this ClusterUsage indicates a use by.
func (u *ClusterUsage) SetUsedBy(r *protection.Resource) {
	if r == nil {
		u.Spec.By = nil
		return
	}
	conv := GeneratedResourceConverter{}
	out := conv.FromInternal(*r)
	u.Spec.By = &out
}

// GetReason gets the reason this ClusterUsage exists.
func (u *ClusterUsage) GetReason() *string {
	return u.Spec.Reason
}

// SetReason sets the reason this ClusterUsage exists.
func (u *ClusterUsage) SetReason(reason *string) {
	u.Spec.Reason = reason
}

// GetReplayDeletion gets a boolean that indicates whether deletion of the used
// resource will be replayed when this ClusterUsage is deleted.
func (u *ClusterUsage) GetReplayDeletion() *bool {
	return u.Spec.ReplayDeletion
}

// SetReplayDeletion specifies whether deletion of the used resource will be
// replayed when this ClusterUsage is deleted.
func (u *ClusterUsage) SetReplayDeletion(replay *bool) {
	u.Spec.ReplayDeletion = replay
}

// GetCondition of this ClusterUsage.
func (u *ClusterUsage) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return u.Status.GetCondition(ct)
}

// SetConditions of this ClusterUsage.
func (u *ClusterUsage) SetConditions(c ...xpv1.Condition) {
	u.Status.SetConditions(c...)
}

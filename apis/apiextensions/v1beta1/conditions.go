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
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// GetCondition of this Usage.
func (u *Usage) GetCondition(ct xpv2.ConditionType) xpv2.Condition {
	return u.Status.GetCondition(ct)
}

// SetConditions of this Usage.
func (u *Usage) SetConditions(c ...xpv2.Condition) {
	u.Status.SetConditions(c...)
}

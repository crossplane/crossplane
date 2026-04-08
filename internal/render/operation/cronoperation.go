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

package operation

import (
	"time"

	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/controller/ops/cronoperation"
)

// NewFromCronOperation produces the Operation a CronOperation would create. It
// calls the real CronOperation controller's NewOperation function, which sets
// the Operation's name (based on the scheduled time), labels, and owner
// references.
func NewFromCronOperation(in *CronOperationInput) *opsv1alpha1.Operation {
	scheduled := time.Now()
	if in.ScheduledTime != nil {
		scheduled = in.ScheduledTime.Time
	}

	return cronoperation.NewOperation(&in.CronOperation, scheduled)
}

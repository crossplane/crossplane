/*
Copyright 2019 The Crossplane Authors.

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

// Package leaderelection provides shared leader-election utilities.
package leaderelection

import (
	"fmt"
	"time"
)

// ValidateConfig returns an error if the leader-election timing configuration
// is not valid. It requires that all durations are positive, that the lease
// duration is greater than the renew deadline, and that the renew deadline
// is greater than 1.2 times the retry period.
func ValidateConfig(enabled bool, lease, renew, retry time.Duration) error {
	if !enabled {
		return nil
	}

	if lease <= 0 {
		return fmt.Errorf("leader election lease duration must be positive, got %s", lease)
	}

	if renew <= 0 {
		return fmt.Errorf("leader election renew deadline must be positive, got %s", renew)
	}

	if retry <= 0 {
		return fmt.Errorf("leader election retry period must be positive, got %s", retry)
	}

	if lease <= renew {
		return fmt.Errorf("leader election lease duration (%s) must be greater than renew deadline (%s)", lease, renew)
	}

	minRenewDeadline := time.Duration(1.2 * float64(retry))
	if renew <= minRenewDeadline {
		return fmt.Errorf("leader election renew deadline (%s) must be greater than 1.2 * retry period (%s)", renew, minRenewDeadline)
	}

	return nil
}

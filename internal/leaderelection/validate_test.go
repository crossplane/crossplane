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

package leaderelection

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		lease   time.Duration
		renew   time.Duration
		retry   time.Duration
		want    string
	}{
		{
			name:    "Disabled",
			enabled: false,
			lease:   0,
			renew:   0,
			retry:   0,
			want:    "",
		},
		{
			name:    "Default",
			enabled: true,
			lease:   60 * time.Second,
			renew:   50 * time.Second,
			retry:   2 * time.Second,
			want:    "",
		},
		{
			name:    "NonPositiveLease",
			enabled: true,
			lease:   0,
			renew:   50 * time.Second,
			retry:   2 * time.Second,
			want:    "leader election lease duration must be positive, got 0s",
		},
		{
			name:    "NegativeLease",
			enabled: true,
			lease:   -time.Second,
			renew:   50 * time.Second,
			retry:   2 * time.Second,
			want:    "leader election lease duration must be positive, got -1s",
		},
		{
			name:    "NonPositiveRenew",
			enabled: true,
			lease:   60 * time.Second,
			renew:   0,
			retry:   2 * time.Second,
			want:    "leader election renew deadline must be positive, got 0s",
		},
		{
			name:    "NegativeRenew",
			enabled: true,
			lease:   60 * time.Second,
			renew:   -time.Second,
			retry:   2 * time.Second,
			want:    "leader election renew deadline must be positive, got -1s",
		},
		{
			name:    "NonPositiveRetry",
			enabled: true,
			lease:   60 * time.Second,
			renew:   50 * time.Second,
			retry:   0,
			want:    "leader election retry period must be positive, got 0s",
		},
		{
			name:    "NegativeRetry",
			enabled: true,
			lease:   60 * time.Second,
			renew:   50 * time.Second,
			retry:   -time.Second,
			want:    "leader election retry period must be positive, got -1s",
		},
		{
			name:    "LeaseLessThanRenew",
			enabled: true,
			lease:   30 * time.Second,
			renew:   50 * time.Second,
			retry:   2 * time.Second,
			want:    "leader election lease duration (30s) must be greater than renew deadline (50s)",
		},
		{
			name:    "LeaseEqualToRenew",
			enabled: true,
			lease:   50 * time.Second,
			renew:   50 * time.Second,
			retry:   2 * time.Second,
			want:    "leader election lease duration (50s) must be greater than renew deadline (50s)",
		},
		{
			name:    "RenewLessThanMinRenewDeadline",
			enabled: true,
			lease:   60 * time.Second,
			renew:   5 * time.Second,
			retry:   10 * time.Second,
			want:    "leader election renew deadline (5s) must be greater than 1.2 * retry period (12s)",
		},
		{
			name:    "RenewEqualToMinRenewDeadline",
			enabled: true,
			lease:   60 * time.Second,
			renew:   12 * time.Second,
			retry:   10 * time.Second,
			want:    "leader election renew deadline (12s) must be greater than 1.2 * retry period (12s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.enabled, tt.lease, tt.renew, tt.retry)
			got := ""
			if err != nil {
				got = err.Error()
			}
			if got != tt.want {
				t.Errorf("ValidateConfig(...): -want error, +got error\n%s", cmp.Diff(tt.want, got))
			}
		})
	}
}

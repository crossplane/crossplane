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

package transaction

import (
	"context"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
)

// Validator validates a Transaction.
type Validator interface {
	Validate(ctx context.Context, tx *v1alpha1.Transaction) error
}

// ValidatorChain runs multiple validators in sequence (fail-fast).
type ValidatorChain []Validator

// Validate runs all validators in the chain, stopping at the first error.
func (c ValidatorChain) Validate(ctx context.Context, tx *v1alpha1.Transaction) error {
	for _, v := range c {
		if err := v.Validate(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

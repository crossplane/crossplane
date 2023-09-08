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

package managed

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const errSecretStoreDisabled = "cannot publish to secret store, feature is not enabled"

// A PublisherChain chains multiple ManagedPublishers.
type PublisherChain []ConnectionPublisher

// PublishConnection calls each ConnectionPublisher.PublishConnection serially. It returns the first error it
// encounters, if any.
func (pc PublisherChain) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) (bool, error) {
	published := false
	for _, p := range pc {
		pb, err := p.PublishConnection(ctx, o, c)
		if err != nil {
			return published, err
		}
		if pb {
			published = true
		}
	}
	return published, nil
}

// UnpublishConnection calls each ConnectionPublisher.UnpublishConnection serially. It returns the first error it
// encounters, if any.
func (pc PublisherChain) UnpublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) error {
	for _, p := range pc {
		if err := p.UnpublishConnection(ctx, o, c); err != nil {
			return err
		}
	}
	return nil
}

// DisabledSecretStoreManager is a connection details manager that returns a proper
// error when API used but feature not enabled.
type DisabledSecretStoreManager struct {
}

// PublishConnection returns a proper error when API used but the feature was
// not enabled.
func (m *DisabledSecretStoreManager) PublishConnection(_ context.Context, so resource.ConnectionSecretOwner, _ ConnectionDetails) (bool, error) {
	if so.GetPublishConnectionDetailsTo() != nil {
		return false, errors.New(errSecretStoreDisabled)
	}
	return false, nil
}

// UnpublishConnection returns a proper error when API used but the feature was
// not enabled.
func (m *DisabledSecretStoreManager) UnpublishConnection(_ context.Context, so resource.ConnectionSecretOwner, _ ConnectionDetails) error {
	if so.GetPublishConnectionDetailsTo() != nil {
		return errors.New(errSecretStoreDisabled)
	}
	return nil
}

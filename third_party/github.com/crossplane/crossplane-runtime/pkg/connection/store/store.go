/*
 Copyright 2022 The Crossplane Authors.

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

// Package store implements secret stores.
package store

import (
	"context"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// SecretOwner owns a Secret.
type SecretOwner interface {
	resource.Object

	resource.ConnectionDetailsPublisherTo
}

// KeyValues is a map with sensitive values.
type KeyValues map[string][]byte

// ScopedName is scoped name of a secret.
type ScopedName struct {
	Name  string
	Scope string
}

// A Secret is an entity representing a set of sensitive Key Values.
type Secret struct {
	ScopedName
	Metadata *v1.ConnectionSecretMetadata
	Data     KeyValues
}

// NewSecret returns a new Secret owned by supplied SecretOwner and with
// supplied data.
func NewSecret(so SecretOwner, data KeyValues) *Secret {
	if so.GetPublishConnectionDetailsTo() == nil {
		return nil
	}
	p := so.GetPublishConnectionDetailsTo()
	if p.Metadata == nil {
		p.Metadata = &v1.ConnectionSecretMetadata{}
	}
	p.Metadata.SetOwnerUID(so.GetUID())
	return &Secret{
		ScopedName: ScopedName{
			Name:  p.Name,
			Scope: so.GetNamespace(),
		},
		Metadata: p.Metadata,
		Data:     data,
	}
}

// GetOwner returns the UID of the owner of secret.
func (s *Secret) GetOwner() string {
	if s.Metadata == nil {
		return ""
	}
	return s.Metadata.GetOwnerUID()
}

// GetLabels returns the labels of the secret.
func (s *Secret) GetLabels() map[string]string {
	if s.Metadata == nil {
		return nil
	}
	return s.Metadata.Labels
}

// A WriteOption is called before writing the desired secret over the
// current object.
type WriteOption func(ctx context.Context, current, desired *Secret) error

// An DeleteOption is called before deleting the secret.
type DeleteOption func(ctx context.Context, secret *Secret) error

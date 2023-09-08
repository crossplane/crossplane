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

package connection

import (
	"context"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// A StoreConfig configures a connection store.
type StoreConfig interface {
	resource.Object

	GetStoreConfig() v1.SecretStoreConfig
}

// A Store stores sensitive key values in Secret.
type Store interface {
	ReadKeyValues(ctx context.Context, n store.ScopedName, s *store.Secret) error
	WriteKeyValues(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (changed bool, err error)
	DeleteKeyValues(ctx context.Context, s *store.Secret, do ...store.DeleteOption) error
}

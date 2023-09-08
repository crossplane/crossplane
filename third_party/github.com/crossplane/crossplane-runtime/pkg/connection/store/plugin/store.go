/*
Copyright 2023 The Crossplane Authors.

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

// Package plugin implements a gRPC client for external secret store plugins.
package plugin

import (
	"context"
	"crypto/tls"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	essproto "github.com/crossplane/crossplane-runtime/apis/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errGet    = "cannot get secret"
	errApply  = "cannot apply secret"
	errDelete = "cannot delete secret"

	errFmtCannotDial = "cannot dial to the endpoint: %s"
)

// SecretStore is an External Secret Store.
type SecretStore struct {
	client     essproto.ExternalSecretStorePluginServiceClient
	kubeClient client.Client
	config     *v1.Config

	defaultScope string
}

// NewSecretStore returns a new External SecretStore.
func NewSecretStore(_ context.Context, kube client.Client, tcfg *tls.Config, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	creds := credentials.NewTLS(tcfg)
	conn, err := grpc.Dial(cfg.Plugin.Endpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, errors.Wrapf(err, errFmtCannotDial, cfg.Plugin.Endpoint)
	}

	return &SecretStore{
		kubeClient:   kube,
		client:       essproto.NewExternalSecretStorePluginServiceClient(conn),
		config:       &cfg.Plugin.ConfigRef,
		defaultScope: cfg.DefaultScope,
	}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Secret.
func (ss *SecretStore) ReadKeyValues(ctx context.Context, n store.ScopedName, s *store.Secret) error {
	resp, err := ss.client.GetSecret(ctx, &essproto.GetSecretRequest{Secret: &essproto.Secret{ScopedName: ss.getScopedName(n)}, Config: ss.getConfigReference()})
	if err != nil {
		return errors.Wrap(err, errGet)
	}

	s.ScopedName = n
	s.Data = make(map[string][]byte, len(resp.Secret.Data))
	for d := range resp.Secret.Data {
		s.Data[d] = resp.Secret.Data[d]
	}
	if resp.Secret != nil && len(resp.Secret.Metadata) != 0 {
		s.Metadata = new(v1.ConnectionSecretMetadata)
		s.Metadata.Labels = make(map[string]string, len(resp.Secret.Metadata))
		for k, v := range resp.Secret.Metadata {
			s.Metadata.Labels[k] = v
		}
	}

	return nil
}

// WriteKeyValues writes key value pairs to a given Secret.
func (ss *SecretStore) WriteKeyValues(ctx context.Context, s *store.Secret, _ ...store.WriteOption) (changed bool, err error) {
	sec := &essproto.Secret{}
	sec.ScopedName = ss.getScopedName(s.ScopedName)
	sec.Data = make(map[string][]byte, len(s.Data))
	for k, v := range s.Data {
		sec.Data[k] = v
	}

	if s.Metadata != nil && len(s.Metadata.Labels) != 0 {
		sec.Metadata = make(map[string]string, len(s.Metadata.Labels))
		for k, v := range s.Metadata.Labels {
			sec.Metadata[k] = v
		}
	}

	resp, err := ss.client.ApplySecret(ctx, &essproto.ApplySecretRequest{Secret: sec, Config: ss.getConfigReference()})
	if err != nil {
		return false, errors.Wrap(err, errApply)
	}

	return resp.Changed, nil
}

// DeleteKeyValues delete key value pairs from a given Secret.
func (ss *SecretStore) DeleteKeyValues(ctx context.Context, s *store.Secret, _ ...store.DeleteOption) error {
	_, err := ss.client.DeleteKeys(ctx, &essproto.DeleteKeysRequest{Secret: &essproto.Secret{ScopedName: ss.getScopedName(s.ScopedName)}, Config: ss.getConfigReference()})

	return errors.Wrap(err, errDelete)
}

func (ss *SecretStore) getConfigReference() *essproto.ConfigReference {
	return &essproto.ConfigReference{
		ApiVersion: ss.config.APIVersion,
		Kind:       ss.config.Kind,
		Name:       ss.config.Name,
	}
}

func (ss *SecretStore) getScopedName(n store.ScopedName) string {
	if n.Scope == "" {
		n.Scope = ss.defaultScope
	}
	return filepath.Join(n.Scope, n.Name)
}

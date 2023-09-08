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

// Package kubernetes implements a secret store backed by Kubernetes Secrets.
package kubernetes

import (
	"context"
	"crypto/tls"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errGetSecret    = "cannot get secret"
	errDeleteSecret = "cannot delete secret"
	errUpdateSecret = "cannot update secret"
	errApplySecret  = "cannot apply secret"

	errExtractKubernetesAuthCreds = "cannot extract kubernetes auth credentials"
	errBuildRestConfig            = "cannot build rest config kubeconfig"
	errBuildClient                = "cannot build Kubernetes client"
)

// SecretStore is a Kubernetes Secret Store.
type SecretStore struct {
	client resource.ClientApplicator

	defaultNamespace string
}

// NewSecretStore returns a new Kubernetes SecretStore.
func NewSecretStore(ctx context.Context, local client.Client, _ *tls.Config, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	kube, err := buildClient(ctx, local, cfg)
	if err != nil {
		return nil, errors.Wrap(err, errBuildClient)
	}

	return &SecretStore{
		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewApplicatorWithRetry(resource.NewAPIPatchingApplicator(kube), resource.IsAPIErrorWrapped, nil),
		},
		defaultNamespace: cfg.DefaultScope,
	}, nil
}

func buildClient(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (client.Client, error) {
	if cfg.Kubernetes == nil {
		// No KubernetesSecretStoreConfig provided, local API Server will be
		// used as Secret Store.
		return local, nil
	}
	// Configure client for an external API server with a given Kubeconfig.
	kfg, err := resource.CommonCredentialExtractor(ctx, cfg.Kubernetes.Auth.Source, local, cfg.Kubernetes.Auth.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errExtractKubernetesAuthCreds)
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(kfg)
	if err != nil {
		return nil, errors.Wrap(err, errBuildRestConfig)
	}
	return client.New(config, client.Options{})
}

// ReadKeyValues reads and returns key value pairs for a given Kubernetes Secret.
func (ss *SecretStore) ReadKeyValues(ctx context.Context, n store.ScopedName, s *store.Secret) error {
	ks := &corev1.Secret{}
	if err := ss.client.Get(ctx, types.NamespacedName{Name: n.Name, Namespace: ss.namespaceForSecret(n)}, ks); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errGetSecret)
	}
	s.Data = ks.Data
	s.Metadata = &v1.ConnectionSecretMetadata{
		Labels:      ks.Labels,
		Annotations: ks.Annotations,
		Type:        &ks.Type,
	}
	return nil
}

// WriteKeyValues writes key value pairs to a given Kubernetes Secret.
func (ss *SecretStore) WriteKeyValues(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
	ks := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: ss.namespaceForSecret(s.ScopedName),
		},
		Type: resource.SecretTypeConnection,
		Data: s.Data,
	}

	if s.Metadata != nil {
		ks.Labels = s.Metadata.Labels
		ks.Annotations = s.Metadata.Annotations
		if s.Metadata.Type != nil {
			ks.Type = *s.Metadata.Type
		}
	}

	ao := applyOptions(wo...)
	ao = append(ao, resource.AllowUpdateIf(func(current, desired runtime.Object) bool {
		// We consider the update to be a no-op and don't allow it if the
		// current and existing secret data are identical.
		return !cmp.Equal(current.(*corev1.Secret).Data, desired.(*corev1.Secret).Data, cmpopts.EquateEmpty())
	}))

	err := ss.client.Apply(ctx, ks, ao...)
	if resource.IsNotAllowed(err) {
		// The update was not allowed because it was a no-op.
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, errApplySecret)
	}
	return true, nil
}

// DeleteKeyValues delete key value pairs from a given Kubernetes Secret.
// If no kv specified, the whole secret instance is deleted.
// If kv specified, those would be deleted and secret instance will be deleted
// only if there is no data left.
func (ss *SecretStore) DeleteKeyValues(ctx context.Context, s *store.Secret, do ...store.DeleteOption) error {
	// NOTE(turkenh): DeleteKeyValues method wouldn't need to do anything if we
	// have used owner references similar to existing implementation. However,
	// this wouldn't work if the K8s API is not the same as where ConnectionSecretOwner
	// object lives, i.e. a remote cluster.
	// Considering there is not much additional value with deletion via garbage
	// collection in this specific case other than one less API call during
	// deletion, I opted for unifying both instead of adding conditional logic
	// like add owner references if not remote and not call delete etc.
	ks := &corev1.Secret{}
	err := ss.client.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: ss.namespaceForSecret(s.ScopedName)}, ks)
	if kerrors.IsNotFound(err) {
		// Secret already deleted, nothing to do.
		return nil
	}
	if err != nil {
		return errors.Wrap(err, errGetSecret)
	}

	for _, o := range do {
		if err = o(ctx, s); err != nil {
			return err
		}
	}

	// Delete all supplied keys from secret data
	for k := range s.Data {
		delete(ks.Data, k)
	}
	if len(s.Data) == 0 || len(ks.Data) == 0 {
		// Secret is deleted only if:
		// - No kv to delete specified as input
		// - No data left in the secret
		return errors.Wrapf(ss.client.Delete(ctx, ks), errDeleteSecret)
	}
	// If there are still keys left, update the secret with the remaining.
	return errors.Wrapf(ss.client.Update(ctx, ks), errUpdateSecret)
}

func (ss *SecretStore) namespaceForSecret(n store.ScopedName) string {
	if n.Scope == "" {
		return ss.defaultNamespace
	}
	return n.Scope
}

func applyOptions(wo ...store.WriteOption) []resource.ApplyOption {
	ao := make([]resource.ApplyOption, len(wo))
	for i := range wo {
		o := wo[i]
		ao[i] = func(ctx context.Context, current, desired runtime.Object) error {
			currentSecret := current.(*corev1.Secret)
			desiredSecret := desired.(*corev1.Secret)

			cs := &store.Secret{
				ScopedName: store.ScopedName{
					Name:  currentSecret.Name,
					Scope: currentSecret.Namespace,
				},
				Metadata: &v1.ConnectionSecretMetadata{
					Labels:      currentSecret.Labels,
					Annotations: currentSecret.Annotations,
					Type:        &currentSecret.Type,
				},
				Data: currentSecret.Data,
			}

			// NOTE(turkenh): With External Secret Stores, we are using a special label/tag with key
			// "secret.crossplane.io/owner-uid" to track the owner of the connection secret. However, different from
			// other Secret Store implementations, Kubernetes Store uses metadata.OwnerReferences for this purpose and
			// we don't want it to appear in the labels of the secret additionally.
			// Here we are adding the owner label to the internal representation of the current secret as part of
			// converting store.WriteOption's to k8s resource.ApplyOption's, so that our generic store.WriteOptions
			// checking secret owner could work as expected.
			// Fixes: https://github.com/crossplane/crossplane/issues/3520
			if len(currentSecret.GetOwnerReferences()) > 0 {
				cs.Metadata.SetOwnerUID(currentSecret.GetOwnerReferences()[0].UID)
			}

			ds := &store.Secret{
				ScopedName: store.ScopedName{
					Name:  desiredSecret.Name,
					Scope: desiredSecret.Namespace,
				},
				Metadata: &v1.ConnectionSecretMetadata{
					Labels:      desiredSecret.Labels,
					Annotations: desiredSecret.Annotations,
					Type:        &desiredSecret.Type,
				},
				Data: desiredSecret.Data,
			}

			if err := o(ctx, cs, ds); err != nil {
				return err
			}

			desiredSecret.Data = ds.Data
			desiredSecret.Labels = ds.Metadata.Labels
			desiredSecret.Annotations = ds.Metadata.Annotations
			if ds.Metadata.Type != nil {
				desiredSecret.Type = *ds.Metadata.Type
			}

			return nil
		}
	}
	return ao
}

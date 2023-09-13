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

package composite

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Error strings.
const (
	errGetSecret      = "cannot get connection secret of composed resource"
	errConnDetailName = "connection detail is missing name"

	errFmtConnDetailKey  = "connection detail of type %q key is not set"
	errFmtConnDetailVal  = "connection detail of type %q value is not set"
	errFmtConnDetailPath = "connection detail of type %q fromFieldPath is not set"
)

// A ConnectionDetailsFetcherFn fetches the connection details of the supplied
// resource, if any.
type ConnectionDetailsFetcherFn func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error)

// FetchConnection calls the FetchConnectionDetailsFn.
func (f ConnectionDetailsFetcherFn) FetchConnection(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	return f(ctx, o)
}

// A ConnectionDetailsFetcherChain chains multiple ConnectionDetailsFetchers.
type ConnectionDetailsFetcherChain []managed.ConnectionDetailsFetcher

// FetchConnection details of the supplied composed resource, if any.
func (fc ConnectionDetailsFetcherChain) FetchConnection(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	all := make(managed.ConnectionDetails)
	for _, p := range fc {
		conn, err := p.FetchConnection(ctx, o)
		if err != nil {
			return nil, err
		}
		for k, v := range conn {
			all[k] = v
		}
	}
	return all, nil
}

// An SecretConnectionDetailsFetcher may use the API server to read connection
// details from a Kubernetes Secret.
type SecretConnectionDetailsFetcher struct {
	client client.Reader
}

// NewSecretConnectionDetailsFetcher returns a ConnectionDetailsFetcher that may
// use the API server to read connection details from a Kubernetes Secret.
func NewSecretConnectionDetailsFetcher(c client.Client) *SecretConnectionDetailsFetcher {
	return &SecretConnectionDetailsFetcher{client: c}
}

// FetchConnection details of the supplied composed resource from its Kubernetes
// connection secret, per its WriteConnectionSecretToRef, if any.
func (cdf *SecretConnectionDetailsFetcher) FetchConnection(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	sref := o.GetWriteConnectionSecretToReference()
	if sref == nil {
		// secret but has not yet. We presume this isn't an issue and that we'll
		// propagate any connection details during a future iteration.
		return nil, nil
	}
	s := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: sref.Namespace, Name: sref.Name}
	if err := cdf.client.Get(ctx, nn, s); client.IgnoreNotFound(err) != nil {
		return nil, errors.Wrap(err, errGetSecret)
	}
	return s.Data, nil
}

// SecretStoreConnectionPublisher is a ConnectionPublisher that stores
// connection details on the configured SecretStore.
type SecretStoreConnectionPublisher struct {
	publisher managed.ConnectionPublisher
	filter    []string
}

// NewSecretStoreConnectionPublisher returns a SecretStoreConnectionPublisher
func NewSecretStoreConnectionPublisher(p managed.ConnectionPublisher, filter []string) *SecretStoreConnectionPublisher {
	return &SecretStoreConnectionPublisher{
		publisher: p,
		filter:    filter,
	}
}

// PublishConnection details for the supplied resource.
func (p *SecretStoreConnectionPublisher) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
	// This resource does not want to expose a connection secret.
	if o.GetPublishConnectionDetailsTo() == nil {
		return false, nil
	}

	data := map[string][]byte{}
	m := map[string]bool{}
	for _, key := range p.filter {
		m[key] = true
	}

	for key, val := range c {
		// If the filter does not have any keys, we allow all given keys to be
		// published.
		if len(m) == 0 || m[key] {
			data[key] = val
		}
	}

	return p.publisher.PublishConnection(ctx, o, data)
}

// UnpublishConnection details for the supplied resource.
func (p *SecretStoreConnectionPublisher) UnpublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error {
	return p.publisher.UnpublishConnection(ctx, o, c)
}

// NewSecretStoreConnectionDetailsConfigurator returns a Configurator that
// configures a composite resource using its composition.
func NewSecretStoreConnectionDetailsConfigurator(c client.Client) *SecretStoreConnectionDetailsConfigurator {
	return &SecretStoreConnectionDetailsConfigurator{client: c}
}

// A SecretStoreConnectionDetailsConfigurator configures a composite resource
// using its composition.
type SecretStoreConnectionDetailsConfigurator struct {
	client client.Client
}

// Configure any required fields that were omitted from the composite resource
// by copying them from its composition.
func (c *SecretStoreConnectionDetailsConfigurator) Configure(ctx context.Context, cp resource.Composite, rev *v1.CompositionRevision) error {
	apiVersion, kind := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if rev.Spec.CompositeTypeRef.APIVersion != apiVersion || rev.Spec.CompositeTypeRef.Kind != kind {
		return errors.New(errCompositionNotCompatible)
	}

	if cp.GetPublishConnectionDetailsTo() != nil || rev.Spec.PublishConnectionDetailsWithStoreConfigRef == nil {
		return nil
	}

	cp.SetPublishConnectionDetailsTo(&xpv1.PublishConnectionDetailsTo{
		Name: string(cp.GetUID()),
		SecretStoreConfigRef: &xpv1.Reference{
			Name: rev.Spec.PublishConnectionDetailsWithStoreConfigRef.Name,
		},
	})

	return errors.Wrap(c.client.Update(ctx, cp), errUpdateComposite)
}

// ConnectionDetailsExtractor extracts the connection details of a resource.
type ConnectionDetailsExtractor interface {
	// ExtractConnection of the supplied resource.
	ExtractConnection(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error)
}

// A ConnectionDetailsExtractorFn is a function that satisfies
// ConnectionDetailsExtractor.
type ConnectionDetailsExtractorFn func(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error)

// ExtractConnection of the supplied resource.
func (fn ConnectionDetailsExtractorFn) ExtractConnection(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error) {
	return fn(cd, conn, cfg...)
}

// ExtractConnectionDetails extracts XR connection details from the supplied
// composed resource. If no ExtractConfigs are supplied no connection details
// will be returned.
func ExtractConnectionDetails(cd resource.Composed, data managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error) { //nolint:gocyclo // TODO(negz): Break extraction out from validation, like we do with readiness.
	out := map[string][]byte{}
	for _, cfg := range cfg {
		if cfg.Name == "" {
			return nil, errors.Errorf(errConnDetailName)
		}
		switch tp := cfg.Type; tp {
		case ConnectionDetailTypeFromValue:
			if cfg.Value == nil {
				return nil, errors.Errorf(errFmtConnDetailVal, tp)
			}
			out[cfg.Name] = []byte(*cfg.Value)
		case ConnectionDetailTypeFromConnectionSecretKey:
			if cfg.FromConnectionSecretKey == nil {
				return nil, errors.Errorf(errFmtConnDetailKey, tp)
			}
			if data[*cfg.FromConnectionSecretKey] == nil {
				// We don't consider this an error because it's possible the
				// key will still be written at some point in the future.
				continue
			}
			out[cfg.Name] = data[*cfg.FromConnectionSecretKey]
		case ConnectionDetailTypeFromFieldPath:
			if cfg.FromFieldPath == nil {
				return nil, errors.Errorf(errFmtConnDetailPath, tp)
			}
			// Note we're checking that the error _is_ nil. If we hit an error
			// we silently avoid including this connection secret. It's possible
			// the path will start existing with a valid value in future.
			if b, err := fromFieldPath(cd, *cfg.FromFieldPath); err == nil {
				out[cfg.Name] = b
			}
		}
	}
	return out, nil
}

// A ConnectionDetailType is a type of connection detail.
type ConnectionDetailType string

// ConnectionDetailType types.
const (
	ConnectionDetailTypeFromConnectionSecretKey ConnectionDetailType = "FromConnectionSecretKey"
	ConnectionDetailTypeFromFieldPath           ConnectionDetailType = "FromFieldPath"
	ConnectionDetailTypeFromValue               ConnectionDetailType = "FromValue"
)

// A ConnectionDetailExtractConfig configures how an XR connection detail should
// be extracted.
type ConnectionDetailExtractConfig struct {
	// Type sets the connection detail fetching behaviour to be used. Each
	// connection detail type may require its own fields to be set on the
	// ConnectionDetail object.
	Type ConnectionDetailType

	// Name of the connection secret key that will be propagated to the
	// connection secret of the composition instance.
	Name string

	// FromConnectionDetailKey is the key that will be used to fetch the value
	// from the given target resource's connection details.
	FromConnectionSecretKey *string

	// FromFieldPath is the path of the field on the composed resource whose
	// value to be used as input. Name must be specified if the type is
	// FromFieldPath is specified.
	FromFieldPath *string

	// Value that will be propagated to the connection secret of the composition
	// instance. Typically you should use FromConnectionSecretKey instead, but
	// an explicit value may be set to inject a fixed, non-sensitive connection
	// secret values, for example a well-known port.
	Value *string
}

// ExtractConfigsFromComposedTemplate builds extract configs for the supplied
// P&T style composed resource template.
func ExtractConfigsFromComposedTemplate(t *v1.ComposedTemplate) []ConnectionDetailExtractConfig {
	if t == nil {
		return nil
	}
	out := make([]ConnectionDetailExtractConfig, len(t.ConnectionDetails))
	for i := range t.ConnectionDetails {
		out[i] = ConnectionDetailExtractConfig{
			Type:                    connectionDetailType(t.ConnectionDetails[i]),
			Value:                   t.ConnectionDetails[i].Value,
			FromConnectionSecretKey: t.ConnectionDetails[i].FromConnectionSecretKey,
			FromFieldPath:           t.ConnectionDetails[i].FromFieldPath,
		}

		if t.ConnectionDetails[i].Name != nil {
			out[i].Name = *t.ConnectionDetails[i].Name
			continue
		}

		if out[i].Type == ConnectionDetailTypeFromConnectionSecretKey && out[i].FromConnectionSecretKey != nil {
			out[i].Name = *out[i].FromConnectionSecretKey
		}
	}
	return out
}

// Originally there was no 'type' determinator field so Crossplane would infer
// the type. We maintain this behaviour for backward compatibility when no type
// is set.
func connectionDetailType(d v1.ConnectionDetail) ConnectionDetailType {
	switch {
	case d.Type != nil:
		return ConnectionDetailType(*d.Type)
	case d.Value != nil:
		return ConnectionDetailTypeFromValue
	case d.FromConnectionSecretKey != nil:
		return ConnectionDetailTypeFromConnectionSecretKey
	case d.FromFieldPath != nil:
		return ConnectionDetailTypeFromFieldPath
	default:
		// If nothing was specified, assume FromConnectionSecretKey, which was
		// the only value we originally supported. We don't have enough
		// information (i.e. the key name) to actually fetch it, so we'll still
		// return an error eventually.
		return ConnectionDetailTypeFromConnectionSecretKey
	}
}

// fromFieldPath tries to read the value from the supplied field path first as a
// plain string. If this fails, it falls back to reading it as JSON.
func fromFieldPath(from runtime.Object, path string) ([]byte, error) {
	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return nil, err
	}

	str, err := fieldpath.Pave(fromMap).GetString(path)
	if err == nil {
		return []byte(str), nil
	}

	in, err := fieldpath.Pave(fromMap).GetValue(path)
	if err != nil {
		return nil, err
	}

	return json.Marshal(in)
}

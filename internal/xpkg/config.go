package xpkg

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	errListImageConfigs = "cannot list ImageConfigs"
)

// ConfigStore is a store for image configuration.
type ConfigStore interface {
	// PullSecretFor returns the name of the selected image config and
	// name of the pull secret for a given image.
	PullSecretFor(ctx context.Context, image string) (imageConfig, pullSecret string, err error)
}

// isValidConfig is a function that determines if an ImageConfig is valid while
// finding the best match for an image.
type isValidConfig func(c *v1beta1.ImageConfig) bool

// ImageConfigStoreOption is an option for image configuration store.
type ImageConfigStoreOption func(*ImageConfigStore)

// NewImageConfigStore creates a new image configuration store.
func NewImageConfigStore(client client.Client, opts ...ImageConfigStoreOption) ConfigStore {
	s := &ImageConfigStore{
		client: client,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// ImageConfigStore is a store for image configuration.
type ImageConfigStore struct {
	client client.Reader
}

// PullSecretFor returns the pull secret name for a given image as
// well as the name of the ImageConfig resource that contains the pull secret.
func (s *ImageConfigStore) PullSecretFor(ctx context.Context, image string) (imageConfig, pullSecret string, err error) {
	config, err := s.bestMatch(ctx, image, func(c *v1beta1.ImageConfig) bool {
		return c.Spec.Registry != nil && c.Spec.Registry.Authentication != nil && c.Spec.Registry.Authentication.PullSecretRef.Name != ""
	})
	if err != nil {
		return "", "", errors.Wrap(err, errListImageConfigs)
	}

	if config == nil {
		// No ImageConfig with a pull secret found for this image, this is not
		// an error.
		return "", "", nil
	}

	return config.GetName(), config.Spec.Registry.Authentication.PullSecretRef.Name, nil
}

// bestMatch finds the best matching ImageConfig for an image based on the
// longest prefix match.
func (s *ImageConfigStore) bestMatch(ctx context.Context, image string, valid isValidConfig) (*v1beta1.ImageConfig, error) {
	l := &v1beta1.ImageConfigList{}

	if err := s.client.List(ctx, l); err != nil {
		return nil, errors.Wrap(err, "cannot list ImageConfigs")
	}

	var config *v1beta1.ImageConfig
	var longest int

	for _, c := range l.Items {
		if !valid(&c) {
			continue
		}

		for _, m := range c.Spec.MatchImages {
			if strings.HasPrefix(image, m.Prefix) && len(m.Prefix) > longest {
				longest = len(m.Prefix)
				config = &c
			}
		}
	}

	return config, nil
}

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
	errFindBestMatch    = "cannot find best matching ImageConfig"
)

// ConfigStore is a store for image configuration.
type ConfigStore interface {
	// PullSecretFor returns the name of the selected image config and
	// name of the pull secret for a given image.
	PullSecretFor(ctx context.Context, image string) (imageConfig, pullSecret string, err error)
	// ImageVerificationConfigFor returns the ImageConfig for a given image.
	ImageVerificationConfigFor(ctx context.Context, image string) (imageConfig string, iv *v1beta1.ImageVerification, err error)
	// RewritePath returns the name of the selected image config and the
	// rewritten path of the given image based on that config.
	RewritePath(ctx context.Context, image string) (imageConfig, newPath string, err error)
}

// isValidConfig is a function that determines if an ImageConfig is valid while
// finding the best match for an image.
type isValidConfig func(c *v1beta1.ImageConfig) bool

// ImageConfigStoreOption is an option for image configuration store.
type ImageConfigStoreOption func(*ImageConfigStore)

// NewImageConfigStore creates a new image configuration store.
func NewImageConfigStore(client client.Client, namespace string, opts ...ImageConfigStoreOption) ConfigStore {
	s := &ImageConfigStore{
		client:    client,
		namespace: namespace,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// ImageConfigStore is a store for image configuration.
type ImageConfigStore struct {
	client    client.Reader
	namespace string
}

// PullSecretFor returns the pull secret name for a given image as
// well as the name of the ImageConfig resource that contains the pull secret.
func (s *ImageConfigStore) PullSecretFor(ctx context.Context, image string) (imageConfig, pullSecret string, err error) {
	config, err := s.bestMatch(ctx, image, func(c *v1beta1.ImageConfig) bool {
		return c.Spec.Registry != nil && c.Spec.Registry.Authentication != nil && c.Spec.Registry.Authentication.PullSecretRef.Name != ""
	})
	if err != nil {
		return "", "", errors.Wrap(err, errFindBestMatch)
	}

	if config == nil {
		// No ImageConfig with a pull secret found for this image, this is not
		// an error.
		return "", "", nil
	}

	return config.Name, config.Spec.Registry.Authentication.PullSecretRef.Name, nil
}

// ImageVerificationConfigFor returns the ImageConfig for a given image.
func (s *ImageConfigStore) ImageVerificationConfigFor(ctx context.Context, image string) (imageConfig string, iv *v1beta1.ImageVerification, err error) {
	config, err := s.bestMatch(ctx, image, func(c *v1beta1.ImageConfig) bool {
		return c.Spec.Verification != nil
	})
	if err != nil {
		return "", nil, errors.Wrap(err, errFindBestMatch)
	}

	if config == nil {
		// No ImageConfig with a verification config found for this image, this
		// is not an error.
		return "", nil, nil
	}

	if config.Spec.Verification.Cosign == nil {
		// Only cosign verification is supported for now.
		return config.Name, nil, errors.New("cosign verification config is missing")
	}

	return config.Name, config.Spec.Verification, nil
}

// RewritePath returns the name of the selected image config and the rewritten
// path of the given image based on that config.
func (s *ImageConfigStore) RewritePath(ctx context.Context, image string) (imageConfig, newPath string, err error) {
	config, err := s.bestMatch(ctx, image, func(c *v1beta1.ImageConfig) bool {
		return c.Spec.RewriteImage != nil
	})
	if err != nil {
		return "", "", errors.Wrap(err, errFindBestMatch)
	}

	if config == nil {
		// No ImageConfig with a rewrite found for this image, this is not an
		// error.
		return "", "", nil
	}

	rewritePrefix := config.Spec.RewriteImage.Prefix
	if rewritePrefix == "" {
		return config.Name, "", errors.New("rewrite prefix is missing")
	}

	// Find the longest prefix match in the selected image config; this is what
	// we'll replace.
	matchPrefix := ""

	for _, m := range config.Spec.MatchImages {
		if !strings.HasPrefix(image, m.Prefix) {
			continue
		}

		if len(m.Prefix) > len(matchPrefix) {
			matchPrefix = m.Prefix
		}
	}

	return config.Name, rewritePrefix + strings.TrimPrefix(image, matchPrefix), nil
}

// bestMatch finds the best matching ImageConfig for an image based on the
// longest prefix match.
func (s *ImageConfigStore) bestMatch(ctx context.Context, image string, valid isValidConfig) (*v1beta1.ImageConfig, error) {
	l := &v1beta1.ImageConfigList{}

	if err := s.client.List(ctx, l); err != nil {
		return nil, errors.Wrap(err, errListImageConfigs)
	}

	var (
		config  *v1beta1.ImageConfig
		longest int
	)

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

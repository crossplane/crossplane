package xpkg

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	cosign "github.com/sigstore/policy-controller/pkg/webhook/clusterimagepolicy"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	ImageVerificationConfigFor(ctx context.Context, image string) (imageConfig string, verificationConfig *ImageVerification, err error)
}

// ImageVerification is a struct that contains the image verification
// configuration.
type ImageVerification struct {
	// CosignConfig is image verification configuration for cosign.
	CosignConfig *cosign.ClusterImagePolicy
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
func (s *ImageConfigStore) ImageVerificationConfigFor(ctx context.Context, image string) (imageConfig string, verificationConfig *ImageVerification, err error) {
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

	cc, err := cosignPolicy(ctx, s.client, s.namespace, config.Spec.Verification.Cosign)
	if err != nil {
		return config.Name, nil, errors.Wrap(err, "cannot get cosign verification config")
	}

	return config.Name, &ImageVerification{
		CosignConfig: cc,
	}, nil
}

// bestMatch finds the best matching ImageConfig for an image based on the
// longest prefix match.
func (s *ImageConfigStore) bestMatch(ctx context.Context, image string, valid isValidConfig) (*v1beta1.ImageConfig, error) {
	l := &v1beta1.ImageConfigList{}

	if err := s.client.List(ctx, l); err != nil {
		return nil, errors.Wrap(err, errListImageConfigs)
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

// cosignPolicy converts the API type to the cosign type.
func cosignPolicy(ctx context.Context, client client.Reader, namespace string, from *v1beta1.CosignVerificationConfig) (*cosign.ClusterImagePolicy, error) {
	if from == nil {
		return nil, nil
	}

	cip := &cosign.ClusterImagePolicy{}

	// Inline secret data if any.
	for i, a := range from.Authorities {
		if a.Key != nil {
			s := &corev1.Secret{}
			if err := client.Get(ctx, types.NamespacedName{Name: a.Key.SecretRef.Name, Namespace: namespace}, s); err != nil {
				return nil, errors.Wrapf(err, "cannot get secret %q", a.Key.SecretRef.Name)
			}
			v := s.Data[a.Key.SecretRef.Key]
			if len(v) == 0 {
				return nil, errors.Errorf("no data found for key %q in secret %q", a.Key.SecretRef.Key, a.Key.SecretRef.Name)
			}
			publicKey, err := cryptoutils.UnmarshalPEMToPublicKey(v)
			if err != nil || publicKey == nil {
				return nil, errors.Errorf("secret %q contains an invalid public key: %w", a.Key.SecretRef.Key, err)
			}
			from.Authorities[i].Key.Data = string(v)
			from.Authorities[i].Key.SecretRef = nil
		}

		// TODO: Inline keyless CA cert data if any.
	}

	cip.Authorities = make([]cosign.Authority, 0, len(from.Authorities))
	// Convert Authorities field from API type to cosign type.
	if err := convertAuthorities(from.Authorities, &cip.Authorities); err != nil {
		return nil, errors.Wrap(err, "cannot convert authorities to cosign authorities")
	}

	return cip, nil
}

// convertAuthorities converts the authorities from API type to cosign type.
// Following a similar approach as policy controller to convert API types to
// internal types:https://github.com/sigstore/policy-controller/blob/dc9960d8c045d360d43c8a03401f3ad7b2357258/pkg/policy/parse.go#L105
func convertAuthorities(from []v1beta1.CosignAuthority, to *[]cosign.Authority) error {
	bs, err := json.Marshal(from)
	if err != nil {
		return errors.Wrap(err, "cannot marshal to JSON")
	}
	if err = json.Unmarshal(bs, to); err != nil {
		return errors.Wrap(err, "cannot unmarshal from JSON")
	}
	return nil
}

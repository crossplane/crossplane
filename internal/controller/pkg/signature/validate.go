package signature

import (
	"context"
	"crypto"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"github.com/sigstore/sigstore/pkg/signature"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const fetchCertTimeout = 30 * time.Second

// Validator validates image signatures.
type Validator interface {
	Validate(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, opts ...remote.Option) error
}

// NewCosignValidator returns a new CosignValidator.
func NewCosignValidator(c client.Reader, namespace string) (*CosignValidator, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchCertTimeout)
	defer cancel()

	var err error
	opts := &cosign.CheckOpts{}
	opts.RootCerts, err = fulcioroots.Get()
	if err != nil {
		return nil, errors.Errorf("cannot fetch Fulcio roots: %w", err)
	}
	opts.IntermediateCerts, err = fulcioroots.GetIntermediates()
	if err != nil {
		return nil, fmt.Errorf("cannot fetch Fulcio intermediates: %w", err)
	}
	opts.CTLogPubKeys, err = cosign.GetCTLogPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch CTLog public keys: %w", err)
	}

	opts.RekorPubKeys, err = cosign.GetRekorPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch Rekor public keys: %w", err)
	}

	return &CosignValidator{
		client:    c,
		namespace: namespace,

		checkOpts: opts,
	}, nil
}

// CosignValidator validates image signatures using cosign.
type CosignValidator struct {
	client    client.Reader
	namespace string

	checkOpts *cosign.CheckOpts
}

// Validate validates the image signature.
func (c *CosignValidator) Validate(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, opts ...remote.Option) error {
	if config.Provider != v1beta1.ImageVerificationProviderCosign {
		return errors.New("unsupported image verification provider")
	}

	var errs []error
	for _, a := range config.Cosign.Authorities {
		if err := c.buildCosignCheckOpts(ctx, a, ociremote.WithRemoteOptions(opts...)); err != nil {
			errs = append(errs, errors.Errorf("authority %q: cannot build cosign check options %v", a.Name, err))
			continue
		}
		if a.Keyless != nil {
			_, ok, err := cosign.VerifyImageSignatures(ctx, ref, c.checkOpts)
			if err != nil {
				errs = append(errs, errors.Errorf("authority %q: keyless signature verification failed with %v", a.Name, err))
				continue
			}

			if !ok {
				errs = append(errs, errors.Errorf("authority %q: keyless signature verification failed", a.Name))
				continue
			}

			// If verification is successful for at least one of the authorities,
			// return nil. Otherwise, continue with the next authority.
			return nil
		}
		if a.Key != nil {
			_, ok, err := cosign.VerifyImageSignatures(ctx, ref, c.checkOpts)
			if err != nil {
				errs = append(errs, errors.Errorf("authority %q: signature verification with provided key failed with %v", a.Name, err))
				continue
			}

			if !ok {
				errs = append(errs, errors.Errorf("authority %q: signature verification with provided key failed", a.Name))
				continue
			}

			// If verification is successful for at least one of the authorities,
			// return nil. Otherwise, continue with the next authority.
			return nil
		}
	}

	return errors.Join(errs...)
}

func (c *CosignValidator) buildCosignCheckOpts(ctx context.Context, a v1beta1.CosignAuthority, remoteOpts ...ociremote.Option) error {
	c.checkOpts.RegistryClientOpts = remoteOpts

	if kl := a.Keyless; kl != nil {
		for _, id := range kl.Identities {
			c.checkOpts.Identities = append(c.checkOpts.Identities, cosign.Identity{
				Issuer:        id.Issuer,
				Subject:       id.Subject,
				IssuerRegExp:  id.IssuerRegExp,
				SubjectRegExp: id.SubjectRegExp,
			})
		}
		if kl.InsecureIgnoreSCT != nil {
			c.checkOpts.IgnoreSCT = *kl.InsecureIgnoreSCT
		}
	}

	if kr := a.Key; kr != nil {
		s := &corev1.Secret{}
		if err := c.client.Get(ctx, types.NamespacedName{Name: kr.SecretRef.Name, Namespace: c.namespace}, s); err != nil {
			return errors.Wrap(err, "cannot get secret")
		}
		v := s.Data[kr.SecretRef.Key]
		if len(v) == 0 {
			return errors.Errorf("no data found for key %q in secret %q", kr.SecretRef.Key, kr.SecretRef.Name)
		}
		publicKey, err := cryptoutils.UnmarshalPEMToPublicKey(v)
		if err != nil || publicKey == nil {
			return errors.Errorf("secret %q contains an invalid public key: %w", kr.SecretRef.Key, err)
		}

		ha, err := hashAlgorithm(a.Key.HashAlgorithm)
		if err != nil {
			return errors.Wrap(err, "invalid hash algorithm")
		}

		c.checkOpts.SigVerifier, err = signature.LoadVerifier(publicKey, ha)
		if err != nil {
			return errors.Wrap(err, "cannot load signature verifier")
		}
	}

	return nil
}

func hashAlgorithm(algorithm string) (crypto.Hash, error) {
	switch strings.ToLower(strings.TrimSpace(algorithm)) {
	case "sha224":
		return crypto.SHA224, nil
	case "sha256":
		return crypto.SHA256, nil
	case "sha384":
		return crypto.SHA384, nil
	case "sha512":
		return crypto.SHA512, nil
	default:
		return 0, errors.Errorf("unsupported algorithm %q", algorithm)
	}
}

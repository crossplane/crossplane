package signature

import (
	"context"
	"crypto"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"github.com/sigstore/sigstore/pkg/signature"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const fetchCertTimeout = 30 * time.Second

// Validator validates image signatures.
type Validator interface {
	Validate(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, pullSecrets ...string) error
}

// NewCosignValidator returns a new CosignValidator.
func NewCosignValidator(c client.Reader, k kubernetes.Interface, namespace, serviceAccount string) (*CosignValidator, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchCertTimeout)
	defer cancel()

	var err error
	opts := cosign.CheckOpts{}
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
		client:         c,
		clientset:      k,
		namespace:      namespace,
		serviceAccount: serviceAccount,

		baseCheckOpts: opts,
	}, nil
}

// CosignValidator validates image signatures using cosign.
type CosignValidator struct {
	client         client.Reader
	clientset      kubernetes.Interface
	namespace      string
	serviceAccount string

	baseCheckOpts cosign.CheckOpts
}

// Validate validates the image signature.
func (c *CosignValidator) Validate(ctx context.Context, ref name.Reference, config *v1beta1.ImageVerification, pullSecrets ...string) error {
	if config.Provider != v1beta1.ImageVerificationProviderCosign {
		return errors.New("unsupported image verification provider")
	}

	auth, err := k8schain.New(ctx, c.clientset, k8schain.Options{
		Namespace:          c.namespace,
		ServiceAccountName: c.serviceAccount,
		ImagePullSecrets:   pullSecrets,
	})
	if err != nil {
		return errors.Wrap(err, "cannot create k8s auth chain")
	}

	var errs []error
	for _, a := range config.Cosign.Authorities {
		co, err := c.buildCosignCheckOpts(ctx, a, ociremote.WithRemoteOptions(remote.WithAuthFromKeychain(auth)))
		if err != nil {
			errs = append(errs, errors.Errorf("authority %q: cannot build cosign check options %v", a.Name, err))
			continue
		}

		verify := cosign.VerifyImageSignatures
		co.ClaimVerifier = cosign.SimpleClaimVerifier
		if len(a.Attestations) > 0 {
			verify = cosign.VerifyImageAttestations
			co.ClaimVerifier = cosign.IntotoSubjectClaimVerifier
		}

		res, ok, err := verify(ctx, ref, co)
		if err != nil {
			errs = append(errs, errors.Errorf("authority %q: signature verification failed with %v", a.Name, err))
			continue
		}

		if !ok {
			errs = append(errs, errors.Errorf("authority %q: signature verification failed", a.Name))
			continue
		}

		// If there are no attestations, return success given that the signature
		// verification was successful for this authority.
		if len(a.Attestations) == 0 {
			return nil
		}

		// If there are attestations to be verified, check if the attestation
		// is valid for at least one of the resulting/checked
		// signatures/attestations.
		for _, att := range a.Attestations {
			for _, s := range res {
				b, _, err := attestationToPayloadJSON(ctx, att.PredicateType, s)
				if err != nil {
					errs = append(errs, errors.Errorf("authority %q: cannot convert attestation %q to payload JSON: %v", a.Name, att.Name, err))
					continue
				}
				if len(b) == 0 {
					errs = append(errs, errors.Errorf("authority %q: no attestation of type %q found for %q", a.Name, att.PredicateType, att.Name))
					continue
				}
				// If the attestation is valid for at least one of the resulting
				// payloads, return nil. Otherwise, continue with the next
				// signature.
				return nil
			}
		}
	}

	// If we reach this point, none of the authorities were able to verify the
	// image signature or attestations. So, return an error with all the errors
	// encountered.
	return errors.Join(errs...)
}

func (c *CosignValidator) buildCosignCheckOpts(ctx context.Context, a v1beta1.CosignAuthority, remoteOpts ...ociremote.Option) (*cosign.CheckOpts, error) {
	opts := c.baseCheckOpts

	opts.RegistryClientOpts = remoteOpts
	if kl := a.Keyless; kl != nil {
		for _, id := range kl.Identities {
			opts.Identities = append(opts.Identities, cosign.Identity{
				Issuer:        id.Issuer,
				Subject:       id.Subject,
				IssuerRegExp:  id.IssuerRegExp,
				SubjectRegExp: id.SubjectRegExp,
			})
		}
		if kl.InsecureIgnoreSCT != nil {
			opts.IgnoreSCT = *kl.InsecureIgnoreSCT
		}
	}

	if kr := a.Key; kr != nil {
		s := &corev1.Secret{}
		if err := c.client.Get(ctx, types.NamespacedName{Name: kr.SecretRef.Name, Namespace: c.namespace}, s); err != nil {
			return nil, errors.Wrap(err, "cannot get secret")
		}
		v := s.Data[kr.SecretRef.Key]
		if len(v) == 0 {
			return nil, errors.Errorf("no data found for key %q in secret %q", kr.SecretRef.Key, kr.SecretRef.Name)
		}
		publicKey, err := cryptoutils.UnmarshalPEMToPublicKey(v)
		if err != nil || publicKey == nil {
			return nil, errors.Errorf("secret %q contains an invalid public key: %w", kr.SecretRef.Key, err)
		}

		ha, err := hashAlgorithm(a.Key.HashAlgorithm)
		if err != nil {
			return nil, errors.Wrap(err, "invalid hash algorithm")
		}

		opts.SigVerifier, err = signature.LoadVerifier(publicKey, ha)
		if err != nil {
			return nil, errors.Wrap(err, "cannot load signature verifier")
		}
	}

	return &opts, nil
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
